package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/cgroup/subsystem"
	"github.com/kkBill/mydocker/container"
	"github.com/kkBill/mydocker/network"
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
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "set environment",
		},
		cli.StringFlag{
			Name:  "net",
			Usage: "container network",
		},
		cli.StringSliceFlag{
			Name:  "p",
			Usage: "port mapping",
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

		//for _, e := range cmdArray {
		//	fmt.Printf("%v ", e)
		//}
		//fmt.Println()
		// 比如shell中输出的是$ ./mydocker run -d sh
		// 那么cmdArray 就是 [sh]
		imageName := cmdArray[0]
		cmdArray = cmdArray[1:]

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
		network := context.String("net")

		//envSlice := context.StringSlice("e")
		portmapping := context.StringSlice("p")

		logrus.Infof("tty %v", tty)
		//Run(tty, cmdArray, resconfig, volume, containerName)
		Run(tty, cmdArray, resconfig, volume, containerName, imageName, network, portmapping)
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

// 用法： mydocker commit containerName imageName
var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit a container into image",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 2 {
			return fmt.Errorf("commitCommand: missing container name & imageName...")
		}
		containerName := context.Args().Get(0)
		imageName := context.Args().Get(1)
		commitContainer(containerName, imageName)
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

// 命令格式为：mydocker stop 容器名
var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		stopContainer(containerName)
		return nil
	},
}

// 命令格式为：mydocker rm 容器名
var removeCommand = cli.Command{
	Name:  "rm",
	Usage: "remove unused containers",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		removeContainer(containerName)
		return nil
	},
}

// 命令格式为：mydocker exec 容器名 命令
var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a command into coontainer",
	Action: func(context *cli.Context) error {
		if os.Getenv(ENV_EXEC_PID) != "" {
			logrus.Infof("execCommand: pid callback, pid is: %v", os.Getpid())
			return nil
		}
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

// 这部分代码没有问题，可以正常创建、展示和删除网络对象
var networkCommand = cli.Command{
	Name:  "network",
	Usage: "container network commands",
	Subcommands: []cli.Command{
		{
			Name:  "create",
			Usage: "create a container network",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver",
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet",
					Usage: "subnet cidr",
				},
			},
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.Init()

				bridgeName := context.Args()[0]
				driverName := context.String("driver")
				subnetName := context.String("subnet")
				logrus.Infof("driverName: %s, subnetName: %s, bridgeName: %s\n", driverName, subnetName, bridgeName)
				err := network.CreateNetwork(driverName, subnetName, bridgeName)
				if err != nil {
					return fmt.Errorf("create network error: %+v", err)
				}
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "list container network",
			Action: func(context *cli.Context) error {
				network.Init()
				network.ListNetwork()
				return nil
			},
		},
		{
			Name:  "remove",
			Usage: "remove container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.Init()
				err := network.DeleteNetwork(context.Args()[0])
				if err != nil {
					return fmt.Errorf("remove network error: %+v", err)
				}
				return nil
			},
		},
	},
}
