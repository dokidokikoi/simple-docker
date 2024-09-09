package main

import (
	"docker/container"
	"os"
	"path/filepath"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

func removeContainer(containerName string) {
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		zaplog.L().Error("get container info error", zap.String("container name", containerName), zap.Error(err))
		return
	}
	if containerInfo.Status != container.STOP {
		zaplog.L().Error("could not remove running container")
		return
	}
	dirURL := filepath.Join(container.DefaultInfoLocation, containerName)
	if err := os.RemoveAll(dirURL); err != nil {
		zaplog.L().Error("remove file error", zap.String("container name", containerName), zap.Error(err))
		return
	}
	container.DeleteWorkSpace(containerInfo.Volume, containerName)
}
