package main

import (
	"docker/container"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/tabwriter"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

func ListContainers() {
	// 找到存储容器信息的路径 /var/run/mydocker
	// 遍历该文件夹下所有文件
	var containers []*container.ContainerInfo
	err := filepath.Walk(container.DefaultInfoLocation, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() || info.Name() != "config.json" {
			return err
		}
		// 根据容器配置文件获取对应信息，然后转换成容器信息的对象
		tmpContainer, err := getContainerInfo(path)
		if err != nil {
			zaplog.L().Error("get container error", zap.Error(err))
			return err
		}
		containers = append(containers, tmpContainer)
		return nil
	})
	if err != nil {
		zaplog.L().Error("list container error", zap.Error(err))
	}

	// 使用 tabwriter.NewWriter 在控制台打印出容器信息
	// tabwriter 是引用的 text/tabwriter 类库，用于在控制台打印对齐的表格
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	// 控制台输出的信息列
	fmt.Fprint(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATES\n")
	for _, item := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.Id, item.Name, item.Pid, item.Status, item.Command, item.CreatedTime)
	}
	// 刷新标准输出流缓存区，将容器列表打印出来
	if err := w.Flush(); err != nil {
		zaplog.L().Error("flush error", zap.Error(err))
		return
	}
}

func getContainerInfo(path string) (*container.ContainerInfo, error) {
	// 读取 config.json 文件内的容器信息
	content, err := os.ReadFile(path)
	if err != nil {
		zaplog.L().Error("read file error", zap.String("path", path), zap.Error(err))
		return nil, err
	}
	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(content, &containerInfo); err != nil {
		zaplog.L().Error("json parse error", zap.String("path", path), zap.Error(err))
		return nil, err
	}

	return &containerInfo, nil
}
