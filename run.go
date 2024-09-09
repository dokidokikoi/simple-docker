package main

import (
	"docker/cgroups"
	"docker/cgroups/subsystems"
	"docker/container"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

/*
这里的 Start 方法是真正开始前面创建好的 command 的调用，它首先会 clone 出来一个 namespace 隔离
的进程，然后在子进程中，调用 /proc/self/exe，也就是调用自己，发送 init 参数，调用我们写的 init
方法，去初始化容器的一些资源。
*/
func Run(tty bool, comArray []string, imageName, volume, containerName string, envSlice []string, res *subsystems.ResourceConfig) {
	// 首先生成 10 位数字的容器 ID
	id := randStringBytes(10)
	// 如果用户不指定容器名，那么就以容器 id 作容器名
	if containerName == "" {
		containerName = id
	}

	parent, writePipe := container.NewParentProcess(tty, containerName, volume, imageName, envSlice)
	if parent == nil {
		zaplog.L().Error("new parent process error")
		return
	}
	if err := parent.Start(); err != nil {
		zaplog.L().Error("error", zap.Error(err))
	}

	// 记录文件信息
	containerName, err := recordContainerInfo(parent.Process.Pid, comArray, id, containerName, volume)
	if err != nil {
		zaplog.L().Error("record container info error", zap.Error(err))
		return
	}

	// use mydocker-cgroup as cgroup name
	// 创建 cgroup manager,并通过调用 set 和 apply 设置资源限制并使限制在容器上生效
	cgroupManager := cgroups.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Destory()

	// 设置资源限制
	cgroupManager.Set(res)
	// 将容器进程加入到各个 subsystem 挂载的 cgroup 中
	cgroupManager.Apply(parent.Process.Pid)
	//  对容器设置完限制后，初始化容器
	sendInitCommand(comArray, writePipe)
	if tty {
		parent.Wait()
		deleteContainerInfo(containerName)
		container.DeleteWorkSpace(volume, containerName)
	}
}

func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	zaplog.L().Info("command", zap.String("all command", command))
	writePipe.WriteString(command)
	writePipe.Close()
}

func recordContainerInfo(containerPID int, commandArray []string, containerID, containerName, volume string) (string, error) {
	// 以当前时间为容器创建时间
	createTime := time.Now().Format("2006-01-02 15:04:05")
	command := strings.Join(commandArray, "")
	// 生成容器信息的结构体实例
	containerInfo := &container.ContainerInfo{
		Id:          containerID,
		Pid:         strconv.Itoa(containerPID),
		Command:     command,
		CreatedTime: createTime,
		Status:      container.RUNNING,
		Name:        containerName,
		Volume:      volume,
	}

	// 将容器信息序列化
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		zaplog.L().Error("record container info error", zap.Error(err))
		return "", err
	}
	jsonStr := string(jsonBytes)

	// 拼凑一下存储容器信息的路径
	dirUri := filepath.Join(container.DefaultInfoLocation, containerName)
	// 创建文件夹
	if err := os.MkdirAll(dirUri, 0622); err != nil {
		zaplog.L().Error("container name duplicated", zap.Error(err))
		return "", err
	}
	fileName := filepath.Join(dirUri, container.ConfigName)
	// 创建最终的配置文件
	file, err := os.Create(fileName)
	if err != nil {
		zaplog.L().Error("create config.json error", zap.Error(err))
		return "", err
	}
	defer file.Close()
	// 将 json 化之后的数据写入文件中
	if _, err := file.WriteString(jsonStr); err != nil {
		zaplog.L().Error("write config.json error", zap.Error(err))
		return "", err
	}

	return containerName, nil
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rand.Intn(10) + '0')
	}
	return string(b)
}

func deleteContainerInfo(containerID string) {
	dirURL := filepath.Join(container.DefaultInfoLocation, containerID)
	if err := os.RemoveAll(dirURL); err != nil {
		zaplog.L().Error("remove info dir error", zap.Error(err))
	}
}
