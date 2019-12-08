package container

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"os"
	"os/exec"
	"syscall"
)

// version 1 2019-11-29
//func NewParentProcess(tty bool, command string) *exec.Cmd {
//	args := []string{"init",command}
//
//	cmd := exec.Command("/proc/self/exe",args...)
//
//	cmd.SysProcAttr = &syscall.SysProcAttr{
//		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
//	}
//
//	if tty {
//		cmd.Stdin = os.Stdin
//		cmd.Stdout = os.Stdout
//		cmd.Stderr = os.Stderr
//	}
//	return cmd
//}

// version 3 2019-12-04
var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	Exit                string = "exited"
	DefaultInfoLocation string = "/var/run/mydocker/%s/"
	ConfigName          string = "config.json"
	ContainerLogFile    string = "container.log"
	RootUrl             string = "/root"
	MntUrl              string = "/root/mnt/%s"
	WriteLayerUrl       string = "/root/writeLayer/%s"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`        //容器的init进程在宿主机上的 PID
	Id          string `json:"id"`         //容器Id
	Name        string `json:"name"`       //容器名
	Command     string `json:"command"`    //容器内init运行命令
	CreatedTime string `json:"createdTime"` //创建时间
	Status      string `json:"status"`     //容器的状态
	//Volume      string `json:"volume"`     //容器的数据卷
	//PortMapping []string `json:"portmapping"` //端口映射
}

// version 2 2019-12-02
// 创建匿名管道，返回两个变量，一个读一个写，都是文件类型
func NewPipe() (*os.File, *os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return r, w, nil
}

// 这个函数不太理解(2019-12-05)
func NewParentProcess(tty bool, volume, containerName string) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := NewPipe()
	if err != nil {
		logrus.Errorf("New pipe error %v", err)
		return nil, nil
	}

	// 初始化容器，执行自己定义的 init 命令
	cmd := exec.Command("/proc/self/exe", "init")

	//noinspection ALL
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		// 关键！！！
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: syscall.Getuid(), Size: 1,},},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: syscall.Getgid(), Size: 1,},},
	}
	//cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(1), Gid: uint32(1)}
	// 如果开启终端，读入终端的输入
	cmd.Stdin = os.Stdin
	if tty {
		//cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}else{
		// 生成容器对应目录的container.log文件
		path := fmt.Sprintf(DefaultInfoLocation, containerName)
		if err := os.MkdirAll(path, 0622); err != nil {
			logrus.Errorf("NewParentProcess: mkdir %s error %v.", path, err)
			return nil, nil
		}
		logFilePath := path + ContainerLogFile
		logrus.Infof("container.log path: %v", logFilePath)
		logFile, err := os.Create(logFilePath)
		if err != nil {
			logrus.Errorf("NewParentProcess: create %s error %v.", logFilePath, err)
			return nil, nil
		}
		// 把生成好的文件赋值给stdout，把容器内的标准输出重定向到该文件中
		cmd.Stdout = logFile
	}

	// ExtraFiles specifies additional open files to be inherited by the
	// new process. (only linux)
	// 就是通过cmd的这个属性把readPipe这个文件传给子进程
	cmd.ExtraFiles = []*os.File{readPipe}

	mntURL := "/root/mnt/"
	rootURL := "/root/"
	NewWorkSpace(rootURL, mntURL, volume)
	// Dir specifies the working directory of the command.
	cmd.Dir = mntURL
	return cmd, writePipe
}
