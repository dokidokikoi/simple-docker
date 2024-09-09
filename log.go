package main

import (
	"docker/container"
	"fmt"
	"io"
	"os"
	"path/filepath"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

func logContainer(containerName string) {
	// 找到对应文件夹的位置
	dirURL := filepath.Join(container.DefaultInfoLocation, containerName)
	logFileLocation := filepath.Join(dirURL, container.ContainerLogFile)
	file, err := os.Open(logFileLocation)
	if err != nil {
		zaplog.L().Error("log container open file error", zap.String("path", logFileLocation), zap.Error(err))
		return
	}
	// 读出所有内容
	content, err := io.ReadAll(file)
	if err != nil {
		zaplog.L().Error("log container read file error", zap.String("path", logFileLocation), zap.Error(err))
		return
	}
	// 读出到标准输出
	fmt.Fprint(os.Stdout, string(content), "\n")
}
