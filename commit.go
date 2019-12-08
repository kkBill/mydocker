package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"os/exec"
)

func commitContainer(imageName string) {
	mntURL := "/root/mnt"
	imageTar := "/root/" + imageName + ".tar"
	fmt.Printf("%s", imageTar)
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput();
		err != nil {
		logrus.Errorf("tar folder %s error. %v", mntURL, err)
	}
}
