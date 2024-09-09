package main

import (
	"docker/container"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

func stopContainer(containerName string) {
	pid, err := getContainerPidByName(containerName)
	if err != nil {
		zaplog.L().Error("get container pid by name error", zap.String("container name", containerName), zap.Error(err))
		return
	}
	// 将 string 类型的 pid 转换成 int 类型
	pidInt, err := strconv.Atoi(pid)
	if err != nil {
		zaplog.L().Error("conver pid from string to int error", zap.String("pid", pid), zap.Error(err))
		return
	}
	// 系统调用 kill 可以发送信号给进程，通过传递 syscall.SIGTERM 信号，去杀掉容器主进程
	if err := syscall.Kill(pidInt, syscall.SIGTERM); err != nil {
		zaplog.L().Error("stop container error", zap.String("container name", containerName), zap.Error(err))
		return
	}
	// 至此，容器进程已经被杀死，所以下面需要修改容器状态，pid 可以置空
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		zaplog.L().Error("get container info error", zap.String("container name", containerName), zap.Error(err))
		return
	}
	containerInfo.Status = container.STOP
	containerInfo.Pid = ""
	contentBytes, err := json.Marshal(containerInfo)
	if err != nil {
		zaplog.L().Error("marshal container info error", zap.String("container name", containerName), zap.Error(err))
		return
	}
	err = os.WriteFile(filepath.Join(container.DefaultInfoLocation, containerName, container.ConfigName), contentBytes, 0622)
	if err != nil {
		zaplog.L().Error("write container info error", zap.String("container name", containerName), zap.Error(err))
	}
}

func getContainerInfoByName(containerName string) (*container.ContainerInfo, error) {
	dirURL := filepath.Join(container.DefaultInfoLocation, containerName)
	configFilePath := filepath.Join(dirURL, container.ConfigName)
	return getContainerInfo(configFilePath)
}
