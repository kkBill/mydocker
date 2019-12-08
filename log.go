package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/container"
	"io/ioutil"
	"os"
)

func logContainer(containerName string)  {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	logFileLocation := dirURL + container.ContainerLogFile
	file, err := os.Open(logFileLocation)
	//noinspection GoNilness
	defer file.Close()
	if err != nil {
		logrus.Errorf("logContainer: Log container open file %s error %v", logFileLocation, err)
		return
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		logrus.Errorf("logContainer: Log container read file %s error %v", logFileLocation, err)
		return
	}
	// 通过Fprint()把日志文件内容输出到标准控制台上
	//noinspection GoUnhandledErrorResult
	fmt.Fprint(os.Stdout, string(content))
}