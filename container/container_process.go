package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

var (
	RUNNING string = "running"
	STOP    string = "stopped"
	EXIT    string = "exited"
)
var (
	DefaultInfoLocation string = "/var/run/mydocker/"
	ConfigName          string = "config.json"
	ContainerLogFile    string = "container.log"
	RootUrl             string = "/root"
	MntUrl              string = "/root/mnt/"
	WriteLayerUrl       string = "/root/writeLayer/"
)

type ContainerInfo struct {
	Pid         string   `json:"pid"`         //容器的init进程在宿主机上的 PID
	Id          string   `json:"id"`          //容器Id
	Name        string   `json:"name"`        //容器名
	Command     string   `json:"command"`     //容器内init运行命令
	CreatedTime string   `json:"createTime"`  //创建时间
	Status      string   `json:"status"`      //容器的状态
	Volume      string   `json:"volume"`      // 数据卷
	PortMapping []string `json:"portmapping"` //端口映射
}

/*
这里是父进程，也就是当前进程执行的内容
1. 这里的 /proc/self/exe 调用中，/proc/self 指的是当前运行进程自己的环境，exec 其实就是自己调用自己，使用这种方式对创建出来的进程进行初始化
2. 后面的 args 是参数，其中 init 是传递给本进程的第一个参数
3. 下面的 clone 参数就是去 fork 出来一个新进程，并且使用了 namespace 隔离新创建的进程和外部环境。
4. 如果用户指定了 -ti 参数，就需要把当前进程的输入输出导入到标准输入输出上
*/
func NewParentProcess(tty bool, containerName, volume, imageName string, envSlice []string) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := NewPipe()
	if err != nil {
		zaplog.L().Error("New pipe error", zap.Error(err))
		return nil, nil
	}
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// 生成容器对应目录的 container.log 文件
		dirURL := filepath.Join(DefaultInfoLocation, containerName)
		if err := os.MkdirAll(dirURL, 0622); err != nil {
			zaplog.L().Warn("NewParentProcess mkdir error", zap.String("path", dirURL), zap.Error(err))
			return nil, nil
		}
		stdLogFilePath := filepath.Join(dirURL, ContainerLogFile)
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			zaplog.L().Error("NewParentProcess create log file error", zap.String("path", stdLogFilePath), zap.Error(err))
			return nil, nil
		}
		// 把生成好的文件赋值给 stdout，这样能把容器内的标准输出重定向到这个文件中
		cmd.Stdout = stdLogFile
	}
	cmd.ExtraFiles = []*os.File{readPipe}
	cmd.Env = append(os.Environ(), envSlice...)
	NewWorkSpace(volume, imageName, containerName)
	cmd.Dir = filepath.Join(MntUrl, containerName)
	return cmd, writePipe
}

func NewPipe() (*os.File, *os.File, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return read, write, nil
}
