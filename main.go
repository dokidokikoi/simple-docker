package main

import (
	"os"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

const usage = `	mydocker is a simple container runtime implementation.
	The puerpose of this is to learn how docker works and how to write a docker by ourselves
	Enjoy it, just for fun.`

func main() {
	app := cli.NewApp()
	app.Name = "mydocker"
	app.Usage = usage

	app.Commands = []cli.Command{
		runCommand,
		initCommand,
		commitCommand,
		listCommand,
		logCommand,
		execCommand,
		stopCommand,
		removeCommand,
		networkCommand,
	}

	app.Before = func(ctx *cli.Context) error {
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		zaplog.L().Fatal("error", zap.Error(err))
	}
}
