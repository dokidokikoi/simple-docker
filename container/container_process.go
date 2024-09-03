package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

/*
这里是父进程，也就是当前进程执行的内容
1. 这里的 /proc/self/exe 调用中，/proc/self 指的是当前运行进程自己的环境，exec 其实就是自己调用自己，使用这种方式对创建出来的进程进行初始化
2. 后面的 args 是参数，其中 init 是传递给本进程的第一个参数
3. 下面的 clone 参数就是去 fork 出来一个新进程，并且使用了 namespace 隔离新创建的进程和外部环境。
4. 如果用户指定了 -ti 参数，就需要把当前进程的输入输出导入到标准输入输出上
*/
func NewParentProcess(tty bool, volume string) (*exec.Cmd, *os.File) {
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
	}
	cmd.ExtraFiles = []*os.File{readPipe}
	mntURL := "/root/mnt/"
	rootURL := "/root/"
	NewWorkSpace(rootURL, mntURL, volume)
	cmd.Dir = mntURL
	return cmd, writePipe
}

func NewPipe() (*os.File, *os.File, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return read, write, nil
}

// Create a AUFS filesystem as container root workspace
func NewWorkSpace(rootURL, mntURL, volume string) {
	CreateReadOnlyLayer(rootURL)
	CreateWriteLayer(rootURL)
	CreateMountPoint(rootURL, mntURL)

	// 根据 volume 判断是否执行挂载数据卷操作
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		if len(volumeURLs) == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(rootURL, mntURL, volumeURLs)
		} else {
			zaplog.L().Error("volume parameter input is not correnct.", zap.String("volume", volume))
		}
	}
}

// 将 busybox.tar 解压到 busybox 目录下，作为容器只读层
func CreateReadOnlyLayer(rootURL string) {
	busyboxURL := filepath.Join(rootURL, "busybox/")
	busyboxTarURL := filepath.Join(rootURL, "busybox.tar")
	exist, err := PathExists(busyboxURL)
	if err != nil {
		zaplog.L().Sugar().Errorf("fail to judge whether dir %s exist. error: %v", busyboxURL, err)
	}
	if !exist {
		if err := os.Mkdir(busyboxURL, 0777); err != nil {
			zaplog.L().Sugar().Errorf("mkdir %s fail. error: %v", busyboxURL, err)
		}
		if _, err := exec.Command("tar", "-xvf", busyboxTarURL, "-C", busyboxURL).CombinedOutput(); err != nil {
			zaplog.L().Error("untar error", zap.String("dir", busyboxTarURL), zap.Error(err))
		}
	}
}

// 创建一个名为 writeLayer 的文件夹作为容器唯一的可写层
func CreateWriteLayer(rootDir string) {
	writeURL := filepath.Join(rootDir, "writeLayer/")
	if err := os.Mkdir(writeURL, 0777); err != nil {
		zaplog.L().Sugar().Errorf("mkdir %s fail. error: %v", writeURL, err)
	}
}

func CreateMountPoint(rootURL, mntURL string) {
	// 创建 mnt 文件夹作为挂载点
	if err := os.Mkdir(mntURL, 0777); err != nil {
		zaplog.L().Sugar().Errorf("mkdir %s fail. error: %v", mntURL, err)
	}
	// 把 writeLayer 目录和 busybox 目录 mount 到 mnt 目录下
	dirs := "dirs=" + rootURL + "writeLayer:" + rootURL + "busybox"
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		zaplog.L().Error("mount layer cmd error", zap.Error(err))
	}
}

// Delete the AUFS filesystem while container exit
func DeleteWorkSpace(rootURL, mntURL, volume string) {
	volumeUrls := volumeUrlExtract(volume)
	if len(volumeUrls) == 2 && volumeUrls[0] != "" && volumeUrls[1] != "" {
		DeleteMountPointWithVolume(rootURL, mntURL, volumeUrls)
	} else {
		DeleteMountPoint(rootURL, mntURL)
	}

	DeleteWriteLayer(rootURL)
}

func DeleteMountPoint(rootURL string, mntURL string) {
	cmd := exec.Command("umount", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		zaplog.L().Error("umount cmd error", zap.Error(err))
	}
	if err := os.RemoveAll(mntURL); err != nil {
		zaplog.L().Error("Remove dir error", zap.String("path", mntURL), zap.Error(err))
	}
}

func DeleteMountPointWithVolume(rootURL, mntURL string, volumeUrls []string) {
	// 卸载容器里 volume 挂载点的文件系统
	containerUrl := filepath.Join(mntURL, volumeUrls[1])
	cmd := exec.Command("umount", containerUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		zaplog.L().Error("umount volume failed.", zap.Error(err))
	}
	// 卸载整个容器文件系统的挂载点
	DeleteMountPoint(rootURL, mntURL)
}

func DeleteWriteLayer(rootURL string) {
	writeURL := filepath.Join(rootURL, "writeLayer/")
	if err := os.RemoveAll(writeURL); err != nil {
		zaplog.L().Error("Remove dir error", zap.String("path", writeURL), zap.Error(err))
	}
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func volumeUrlExtract(volume string) []string {
	return strings.Split(volume, ":")
}

func MountVolume(rootURL, mntURL string, volumeURLs []string) {
	// 创建宿主机文件目录
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		zaplog.L().Error("mkdir parent dir error", zap.String("path", parentUrl), zap.Error(err))
	}
	// 在容器文件系统里创建挂载点
	containerUrl := volumeURLs[1]
	containerVolumeUrl := filepath.Join(mntURL, containerUrl)
	if err := os.Mkdir(containerVolumeUrl, 0777); err != nil {
		zaplog.L().Error("mkdir container dir error", zap.String("path", containerVolumeUrl), zap.Error(err))
	}
	// 把宿主机文件目录挂载到容器挂载点
	dirs := "dirs=" + parentUrl
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		zaplog.L().Error("mount volume failed.", zap.Error(err))
	}
}
