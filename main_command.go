package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/cgroup/subsystem"
	"github.com/kkBill/mydocker/container"
	"github.com/urfave/cli"
	"os"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: `Create a container with namespace and cgroups limit ie: mydocker run -ti [image] [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "ti",
			Usage: "enable tty",
		},
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach container",
		},
		cli.StringFlag{
			Name:  "m",
			Usage: "memory limit",
		},
		cli.StringFlag{
			Name:  "cpushare",
			Usage: "cpushare limit",
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit",
		},
		cli.StringFlag{
			Name:  "v",
			Usage: "vloume",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
	},

	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container command")
		}
		var cmdArray []string
		for _, arg := range context.Args() {
			cmdArray = append(cmdArray, arg)
		}
		// 比如shell中输出的是$ ./mydocker run -d sh
		// 那么cmdArray 就是 [sh]
		cmdArray = cmdArray[0:]

		tty := context.Bool("ti")
		detach := context.Bool("d")

		if tty && detach {
			return fmt.Errorf("ti and d parameter can not both provided.")
		}
		resconfig := &subsystem.ResourceConfig{
			MemoryLimit: context.String("m"),
			CpuShare:    context.String("cpushare"),
			CpuSet:      context.String("cpuset"),
		}
		volume := context.String("v")
		containerName := context.String("name")

		logrus.Infof("tty %v", tty)
		Run(tty, cmdArray, resconfig, volume, containerName)
		return nil
	},
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: "Init container process run user's process in container. Do not call it outside",
	Action: func(context *cli.Context) error {
		logrus.Infof("initCommand: init...") // 当输入 ./mydocker run -d top 时不会执行到这里，为什么？
		err := container.RunContainerInitProcess()
		return err
	},
}

// 用法： mydocker commit xxximage
var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit a container into image",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("commitCommand: missing container name...")
		}
		imageName := context.Args().Get(0)
		commitContainer(imageName)
		return nil
	},
}

// docker ps
var listCommand = cli.Command{
	Name:  "ps",
	Usage: "list all containers",
	Action: func(context *cli.Context) error {
		ListContainers()
		return nil
	},
}

var logCommand = cli.Command{
	Name:  "logs",
	Usage: "print logs of a container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("logCommand: please input container name...")
		}
		containerName := context.Args().Get(0)
		logContainer(containerName)
		return nil
	},
}

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a command into coontainer",
	Action: func(context *cli.Context) error {
		if os.Getenv(ENV_EXEC_PID) != ""{
			logrus.Infof("execCommand: pid callback, pid is: %v", os.Getpid())
			return nil
		}
		// 命令格式为：mydocker exec 容器名 命令
		if len(context.Args()) < 2 {
			return fmt.Errorf("execCommand: missing container name or command")
		}
		containerName := context.Args().Get(0)
		var commandArray []string
		for _, arg := range context.Args().Tail() {
			commandArray = append(commandArray, arg)
		}
		// 执行命令
		ExecContainer(containerName, commandArray)
		return nil
	},
}
