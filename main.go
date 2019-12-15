package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "mydocker"
	app.Usage = `mydocker is a simple container runtime implementation`
	app.Commands = []cli.Command{
		initCommand,
		runCommand,
		commitCommand,
		listCommand,
		logCommand,
		execCommand, // 实现了，但是有bug，还没解决
		networkCommand,
		stopCommand,
		removeCommand,
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}