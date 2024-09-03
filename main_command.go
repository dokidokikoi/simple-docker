package main

import (
	"docker/cgroups/subsystems"
	"docker/container"
	"errors"
	"fmt"

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
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: ctx.String("m"),
			CpuSet:      ctx.String("cpuset"),
			CpuShare:    ctx.String("cpushare"),
		}
		volume := ctx.String("v")
		Run(tty, cmdArray, volume, resConf)
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
		if len(ctx.Args()) < 1 {
			return errors.New("missing container name")
		}
		imageName := ctx.Args().Get(0)
		commitContainer(imageName)
		return nil
	},
}
