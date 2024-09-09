package main

import (
	"docker/container"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "docker/nsenter"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

const ENV_EXEC_PID = "mydocker_pid"
const ENV_EXEC_CMD = "mydocker_cmd"

func ExecContainer(containerName string, comArr []string) {
	// 根据传递过来的容器名获取宿主机对应的 pid
	pid, err := getContainerPidByName(containerName)
	if err != nil {
		zap.L().Error("exec container getContainerPidByName error", zap.String("name", containerName), zap.Error(err))
		return
	}
	// 把命令以空格为分隔符拼接成一个字符串
	cmdStr := strings.Join(comArr, " ")
	zaplog.L().Info("", zap.String("container pid", pid))
	zaplog.L().Info("", zap.String("command", cmdStr))

	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	os.Setenv(ENV_EXEC_PID, pid)
	os.Setenv(ENV_EXEC_CMD, cmdStr)
	// 由于 exec 是程序发起的另一个进程，这个进程的父进程是宿主机的，并不是容器内的
	// 因为在 Cgo 里使用了 setns 系统调用才使得这个进程进入到了容器的命名空间，
	// 由于环境变量是继承于父进程，因此 exec 的环境变量来自宿主机。
	// 这里我们拿到容器内 pid 为 1 进程的环境变量赋给 exec 进程
	containerEnvs := getEnvsByPid(pid)
	cmd.Env = append(os.Environ(), containerEnvs...)

	if err := cmd.Run(); err != nil {
		zaplog.L().Error("exec container error", zap.String("container name", containerName), zap.Error(err))
	}
}

// 根据提供的容器名获取对于容器的 pid
func getContainerPidByName(containerName string) (string, error) {
	// 先拼接出存储容器信息的路径
	dirURL := filepath.Join(container.DefaultInfoLocation, containerName)
	configFilePath := filepath.Join(dirURL, container.ConfigName)
	contentBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return "", err
	}
	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return "", err
	}
	return containerInfo.Pid, nil
}

func getEnvsByPid(pid string) []string {
	// 进程存放环境变量的路径
	path := fmt.Sprintf("/proc/%s/environ", pid)
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		zaplog.L().Error("read file error", zap.String("path", path), zap.Error(err))
		return nil
	}
	// 多个环境变量的分隔符是 \u0000
	return strings.Split(string(contentBytes), "\u0000")
}
