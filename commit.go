package main

import (
	"docker/container"
	"os/exec"
	"path/filepath"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

// 用子目录集合制作镜像
func commitContainer(containerName, imageName string) {
	mntURL := filepath.Join(container.MntUrl, containerName)
	imageTar := filepath.Join(container.RootUrl, imageName+".tar")
	zaplog.L().Info("image tar", zap.String("path", imageTar))
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput(); err != nil {
		zaplog.L().Error("tar folder error", zap.String("path", mntURL), zap.Error(err))
	}
}
