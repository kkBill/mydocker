package container

import (
	"github.com/Sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

// 创建容器的文件系统，其过程如下：
// 1.创建只读层
// 2.创建容器读写层
// 3.创建挂载点，并把只读层和读写层挂载到挂载点上
// 4.1.首先判断volume是否为空，如果为空，则用户并没有使用挂载标签，创建过程结束
// 4.2.如果不为空，则解析volume字符串
// 5.如果返回的字符串长度为2，且数据元素均不为空，则执行MountVolume()进行挂载（该函数的具体流程见后）
// 6.否则提示用户创建数据卷的输入值不对
// 执行数据卷挂载的命令：./mydocker run -ti -v /root/volume:/containerVolume sh
func NewWorkSpace(rootURL string, mntURL string, volume string) {
	CreateReadOnlyLayer(rootURL)
	CreateWriteLayer(rootURL)
	CreateMountPoint(rootURL, mntURL)

	// 根据volume是否为空判断是否执行挂载数据卷操作
	if volume != ""{
		volumeURLs := strings.Split(volume, ":")
		if len(volumeURLs) ==2 && volumeURLs[0] != "" && volumeURLs[1] != ""{
			MountVolume(rootURL,mntURL,volumeURLs)
			logrus.Infof("%q", volumeURLs)
		}else{
			logrus.Infof("volume parameters input is invalid.")
		}
	}
}

// 将 busybox.tar 解压到 busybox 目录下，作为容器的只读层
func CreateReadOnlyLayer(rootURL string) {
	busyboxURL := rootURL + "busybox/"
	busyboxTarURL := rootURL + "busybox.tar"

	exists, err := PathExists(busyboxURL)
	if err != nil {
		logrus.Infof("fail to judge whether dir %s exists or not. %v", busyboxURL, err)
	}
	if exists == false {
		if err := os.Mkdir(busyboxURL, 0777); err != nil {
			logrus.Errorf("mkdir %s error. %v", busyboxURL, err)
		}
		// 这一句不懂
		if _, err := exec.Command("tar", "-xvf", busyboxTarURL, "-C", busyboxURL).CombinedOutput();
			err != nil {
			logrus.Errorf("ubTar dir %s error %v", busyboxTarURL, err)
		}
	}
}

// 创建一个名为writeLayer的文件夹作为容器唯一的可写层
func CreateWriteLayer(rootURL string) {
	writeURL := rootURL + "writeLayer/"
	if err := os.Mkdir(writeURL, 0777); err != nil {
		logrus.Infof("Mkdir %s error. %v", writeURL, err)
	}
}

// 创建mnt文件夹作为挂载点
func CreateMountPoint(rootURL string, mntURL string) {
	if err := os.Mkdir(mntURL, 0777); err != nil {
		logrus.Infof("Mkdir %s error. %v", mntURL, err)
	}
	// 把writeLayer目录和busybox目录mount到mnt目录下
	dirs := "dirs=" + rootURL + "writeLayer:" + rootURL + "busybox"
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("run error. %v", err)
	}
}

// 挂载数据卷，其基本过程如下：
// 1.读取宿主机文件目录hostURL，创建宿主机文件目录/root/${hostURL}
// 2.读取容器挂载点containerURL，在容器文件系统里创建挂载点/root/mnt/${containerURL}
// 3.把宿主机文件目录挂载到容器挂载点
// 通过以上3步，在启动容器的时候，对数据卷的处理也完成了
func MountVolume(rootURL, mntURL string, volumeURLs []string)  {
	// 创建宿主机文件目录
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		logrus.Infof("Mkdir parent dir %s error. %v", parentUrl, err)
	}
	// 在容器的文件系统里创建挂载点目录
	containerUrl := volumeURLs[1]
	containerVolumeUrl := mntURL + containerUrl
	if err := os.Mkdir(containerVolumeUrl, 0777); err != nil {
		logrus.Infof("Mkdir container volume dir %s error. %v", containerVolumeUrl, err)
	}

	// 把宿主机文件目录挂载到容器挂载点上
	dirs := "dirs=" + parentUrl
	cmd := exec.Command("mount","-t","aufs","-o",dirs,"none",containerVolumeUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("mount volume failed. %v", err)
	}
}


// 判断文件路径是否存在
func PathExists(path string) (bool, error) {
	//
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	// IsNotExist returns a boolean indicating whether the error is known to
	// report that a file or directory does not exist.
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// 解析挂载数据卷时传入的参数
func volumeUrlExtract(volume string) []string {
	var volumes []string
	volumes = strings.Split(volume, ":")
	return volumes
}

// 删除容器文件系统，其基本过程如下：
// 1.umount挂载点(/root/mnt/)的文件系统
// 2.删除挂载点  --> 对于挂载点，不能直接remove，而是要先umount (切记：踩过一次坑了
// 3.删除读写层
// 补充：
// 如果volume不为空，且解析出来的字符串数组长度为2，数据元素均不为空，就需要执行DeleteMountPointWithVolume()
// 否则就执行DeleteMountPoint()
func DeleteWorkSpace(rootURL, mntURL, volume string) {
	// 删除挂载点
	if volume != "" {
		volumeUrls := volumeUrlExtract(volume)
		if len(volumeUrls) == 2 && volumeUrls[0] != "" && volumeUrls[1] != "" {
			DeleteMountPointWithVolume(rootURL, mntURL, volumeUrls)
		}else{
			DeleteMountPoint(rootURL, mntURL)
		}
	}else{
		DeleteMountPoint(rootURL, mntURL)
	}
	// 删除读写层
	DeleteWriteLayer(rootURL)
}

// 1.umount挂载点的文件系统（/root/mnt/${containerUrl}）
// 2.umount整个容器文件系统的挂载点（/root/mnt）
// 3.删除容器文件系统的挂载点
func DeleteMountPointWithVolume(rootURL, mntURL string, volumeURLs []string) {
	// 1: mntURL = "/root/mnt"; volumeURLs[1] = "/containerVolume"
	containerUrl := mntURL + volumeURLs[1]
	cmd := exec.Command("umount", containerUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("umount volume failed. %v", err)
	}
	// 2
	cmd = exec.Command("umount", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("umount mountpoint failed. %v", err)
	}
	// 3: 等价于 rm -rf xxx
	if err := os.RemoveAll(mntURL); err != nil {
		logrus.Errorf("remove mountpoint dir %s error. %v", mntURL, err)
	}
}

func DeleteMountPoint(rootURL string, mntURL string) {
	cmd := exec.Command("umount", mntURL)
	// 这一步是为什么？
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("%v", err)
	}
	if err := os.RemoveAll(mntURL); err != nil {
		logrus.Errorf("remove dir %s error. %v", mntURL, err)
	}
}

func DeleteWriteLayer(rootURL string) {
	writeURL := rootURL + "/writeLayer"
	if err := os.RemoveAll(writeURL); err != nil {
		logrus.Errorf("remove dir %s error. %v", writeURL, err)
	}
}