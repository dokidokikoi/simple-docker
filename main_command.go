package main

import (
	"docker/cgroups/subsystems"
	"docker/container"
	"errors"
	"fmt"
	"os"
	"strconv"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

var runCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroup limit
	mydocker run -ti [command]`,
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
			Usage: "volume",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "set environment",
		},
	},
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			return fmt.Errorf("missing container command")
		}

		var cmdArray []string
		for _, arg := range ctx.Args() {
			cmdArray = append(cmdArray, arg)
		}
		zaplog.L().Info("cmArray", zap.Any("array", cmdArray))
		tty := ctx.Bool("ti")
		detach := ctx.Bool("d")
		if tty && detach {
			return errors.New("ti and d paramter can not both provided")
		}
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: ctx.String("m"),
			CpuSet:      ctx.String("cpuset"),
			CpuShare:    ctx.String("cpushare"),
		}
		containerName := ctx.String("name")
		volume := ctx.String("v")

		imageName := cmdArray[0]
		cmdArray = cmdArray[1:]

		envSlice := ctx.StringSlice("e")
		Run(tty, cmdArray, imageName, volume, containerName, envSlice, resConf)
		return nil
	},
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: `Init container process run user's process in container. Do not call it outside`,
	Action: func(ctx *cli.Context) error {
		zaplog.L().Info("init come on")
		return container.RunContainerInitProcess()
	},
}

var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit a container into image",
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 2 {
			return errors.New("missing container name and image name")
		}
		imageName := ctx.Args().Get(1)
		containerName := ctx.Args().Get(0)
		commitContainer(containerName, imageName)
		return nil
	},
}

var listCommand = cli.Command{
	Name:  "ps",
	Usage: "list all the containers",
	Action: func(ctx *cli.Context) error {
		ListContainers()
		return nil
	},
}

var logCommand = cli.Command{
	Name:  "logs",
	Usage: "print logs of a containers",
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			return errors.New("please input your container name")
		}
		containerName := ctx.Args().Get(0)
		logContainer(containerName)
		return nil
	},
}

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a command into container",
	Action: func(ctx *cli.Context) error {
		if os.Getenv(ENV_EXEC_PID) != "" {
			zaplog.L().Info("pid callback", zap.String("pid", strconv.Itoa(os.Getpid())))
			return nil
		}
		if len(ctx.Args()) < 2 {
			return errors.New("missing container name or command")
		}
		containerName := ctx.Args().Get(0)
		var commdArr []string
		// 将除了容器名之外的参数当作需要执行的命令处理
		commdArr = append(commdArr, ctx.Args().Tail()...)
		ExecContainer(containerName, commdArr)
		return nil
	},
}

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a container",
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			return errors.New("missing container name")
		}
		containerName := ctx.Args().Get(0)
		stopContainer(containerName)
		return nil
	},
}

var removeCommand = cli.Command{
	Name:  "rm",
	Usage: "remove unused container",
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			return errors.New("missing container name")
		}
		containerName := ctx.Args().Get(0)
		removeContainer(containerName)
		return nil
	},
}
