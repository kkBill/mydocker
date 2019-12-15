package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/container"
	"os/exec"
)

func commitContainer(containerName, imageName string) {
	//mntURL := "/root/mnt"
	//imageTar := "/root/" + imageName + ".tar"
	mntURL := fmt.Sprintf(container.MntUrl, containerName)
	imageTar := container.RootUrl + "/" + imageName + ".tar"
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput();
		err != nil {
		logrus.Errorf("tar folder %s error. %v", mntURL, err)
	}
}
