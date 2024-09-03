package main

import (
	"os/exec"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

func commitContainer(imageName string) {
	mntURL := "/root/mnt"
	imageTar := "/root/" + imageName + ".tar"
	zaplog.L().Info("image tar", zap.String("path", imageTar))
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput(); err != nil {
		zaplog.L().Error("tar folder error", zap.String("path", mntURL), zap.Error(err))
	}
}
