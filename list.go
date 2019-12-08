package main

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/container"
	"io/ioutil"
	"os"
	"text/tabwriter"
)

func ListContainers()  {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, "")
	dirURL = dirURL[:len(dirURL)-1]
	files, err := ioutil.ReadDir(dirURL)
	if err != nil {
		logrus.Errorf("Read dir %s error %v", dirURL, err)
		return
	}

	var containers []*container.ContainerInfo
	for _, file := range files {
		if file.Name() == "network" {
			continue
		}
		tmpContainer, err := getContainerInfo(file)
		if err != nil {
			logrus.Errorf("Get container info error %v", err)
			continue
		}
		containers = append(containers, tmpContainer)
	}
	// 引用 "text/tabwriter" 类库，用于控制台打印对齐的表格
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	_, _ = fmt.Fprint(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATED\n")
	for _, item := range containers {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.Id,
			item.Name,
			item.Pid,
			item.Status,
			item.Command,
			item.CreatedTime)
	}
	if err := w.Flush(); err != nil {
		logrus.Errorf("Flush error %v", err)
		return
	}
}

func getContainerInfo(file os.FileInfo) (*container.ContainerInfo, error)  {
	// 读取文件名，也就是containerName
	containerName := file.Name()
	// 根据文件名生成文件的绝对路径
	containerPath := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	containerPath = containerPath + container.ConfigName
	// 读取config.json文件内的容器信息
	bytes, err := ioutil.ReadFile(containerPath)
	if err != nil {
		logrus.Errorf("read file failed %v.", err)
		return nil, err
	}
	var containerInfo container.ContainerInfo
	// 将json文件信息反序列化位容器信息对象
	if err := json.Unmarshal(bytes, &containerInfo); err != nil {
		logrus.Errorf("json unmarshal failed %v.", err)
		return nil, err
	}
	return &containerInfo, nil
}
