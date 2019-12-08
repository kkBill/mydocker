package main

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/cgroup"
	"github.com/kkBill/mydocker/cgroup/subsystem"
	"github.com/kkBill/mydocker/container"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

/* version 1
func Run(tty bool, command string) {
	parent := container.NewParentProcess(tty, command)
	if err := parent.Start(); err != nil {
		logrus.Error(err)
	}
	parent.Wait()
	os.Exit(-1)
}
*/

func Run(tty bool, comArray []string, res *subsystem.ResourceConfig, volume, containerName string) {
	// 1.generate container ID (random 10 bits number)
	containerID := generateRandomID(10)
	if containerName == ""{
		containerName = containerID
	}

	parent, writePipe := container.NewParentProcess(tty, volume, containerName)
	if parent == nil {
		logrus.Errorf("new parent process failed")
		return
	}

	if err := parent.Start(); err != nil {
		logrus.Error(err)
	}

	// 记录容器信息
	containerName, err := recordContainerInfo(parent.Process.Pid, comArray, containerName, containerID)
	if err != nil {
		logrus.Errorf("record container info error %v", err)
		return
	}

	// 资源限制 cgroup
	cgroupManager := cgroup.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Remove()
	_ = cgroupManager.Set(res)
	_ = cgroupManager.Apply(parent.Process.Pid)

	logrus.Infof("comArray is %v", comArray)
	sendInitCommand(comArray, writePipe)

	// 只有在 -ti 交互模式下才需要等待子进程，否则就是后台运行模式，即父进程就直接退出
	if tty {
		_ = parent.Wait()
		mntURL := "/root/mnt"
		rootURL := "/root"
		deleteContainerInfo(containerName)
		container.DeleteWorkSpace(rootURL, mntURL, volume)
	}

	//os.Exit(0)
}

func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	logrus.Infof("command: [%v]", command)
	_, _ = writePipe.WriteString(command)
	writePipe.Close()
}

func generateRandomID(n int) string {
	rand.Seed(time.Now().UnixNano())
	candidate := "1234567890"
	bytes := make([]byte, n)
	for i := range bytes {
		bytes[i] = candidate[rand.Intn(len(candidate))]
	}
	return string(bytes)
}

// 记录容器的信息
func recordContainerInfo(containerPid int, commandArray []string, containerName, containerID string) (string, error) {

	// 2.current time of creating container
	createTime := time.Now().Format("2006-01-02 15:04:05")
	command := strings.Join(commandArray, "")

	containerInfo := &container.ContainerInfo{
		Pid:         strconv.Itoa(containerPid),
		Id:          containerID,
		Name:        containerName,
		Command:     command,
		CreatedTime: createTime,
		Status:      container.RUNNING,
	}
	// 将容器信息的对象json序列化成字符串
	bytes, err := json.Marshal(containerInfo)
	if err != nil {
		logrus.Errorf("record container info error %v.", err)
		return "", nil
	}
	jsonStr := string(bytes)

	// 拼接存储容器信息的存储路径
	storagePath := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	if err := os.MkdirAll(storagePath, 0622); err != nil {
		logrus.Errorf("mkdir failed %s. error %v.", storagePath, err)
		return "", nil
	}
	fileName := storagePath + container.ConfigName
	// 创建配置文件
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		logrus.Errorf("create file %s failed. error %v.", file, err)
		return "", nil
	}

	// 将数据写入文件
	if _, err := file.WriteString(jsonStr); err != nil {
		logrus.Errorf("write file failed. error %v.", err)
		return "", nil
	}
	return containerName, nil
}

func deleteContainerInfo(containerName string)  {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	if err := os.RemoveAll(dirURL); err != nil {
		logrus.Errorf("Remove dir %s error %v", dirURL, err)
	}
}