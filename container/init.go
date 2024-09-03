package container

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

/*
这里的 init 函数在容器内部执行的，也就是说，代码执行到这里后，容器所在的进场其实就已经创建出来了，
这是本容器执行的第一个进程。
使用 mount 先去挂载 proc 文件系统，以便后面通过 ps 等系统命令去查看当前进程资源的情况。
*/
func RunContainerInitProcess() error {
	cmdArray := readUserCommand()
	if len(cmdArray) == 0 {
		return errors.New("run container get user command error, cmdArray is nil")
	}

	setUpMount()

	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		zaplog.L().Error("Exec loop path error", zap.Error(err))
		return err
	}
	zaplog.L().Info("command path", zap.String("path", path))
	if err := syscall.Exec(path, cmdArray[0:], os.Environ()); err != nil {
		zaplog.L().Error("command exec error", zap.Error(err))
	}
	return nil
}

func readUserCommand() []string {
	pipe := os.NewFile(uintptr(3), "pipe")
	msg, err := io.ReadAll(pipe)
	if err != nil {
		zaplog.L().Error("init read pipe error", zap.Error(err))
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}

/*
*
Init 挂载点
*/
func setUpMount() {
	// 获取当前工作路径
	pwd, err := os.Getwd()
	if err != nil {
		zaplog.L().Error("Get current location error", zap.Error(err))
		return
	}
	zaplog.L().Info("Current location", zap.String("location", pwd))
	pivotRoot(pwd)

	//mount proc
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}

func pivotRoot(root string) error {
	/**
	  为了使当前 root 的老 root 和新 root 不在同一个文件系统下，我们把 root 重新 mount 了一次
	  bind mount 是把相同的内容换了一个挂载点的挂载方法
	*/
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount rootfs to itself error: %w", err)
	}
	// 创建 rootfs/.pivot_root 存储 old_root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}
	// pivot_root 到新的rootfs, 现在老的 old_root 是挂载在 rootfs/.pivot_root
	// 挂载点现在依然可以在mount命令中看到
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root failed %w", err)
	}
	// 修改当前的工作目录到根目录
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / failed %w", err)
	}

	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.pivot_root
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir failed %w", err)
	}
	// 删除临时文件夹
	return os.Remove(pivotDir)
}
