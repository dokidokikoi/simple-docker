package subsystems

import (
	"fmt"
	"os"
	"path"
	"strconv"
)

// memory subsystem 的实现
type MemorySubSystem struct {
}

func (*MemorySubSystem) Name() string {
	return "memory"
}

// 设置 cgroupPath 对应的 cgroup 的内存资源限制
func (s *MemorySubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err != nil {
		return err
	} else {
		if res.MemoryLimit != "" {
			// 设置这个 cgroup 的内存限制，即将限制写入到 cgroup 对应目录的 memory.limit_in_bytes 文件中。
			if err := os.WriteFile(path.Join(subsysCgroupPath, "memory.limit_in_bytes"), []byte(res.MemoryLimit), 0644); err != nil {
				return fmt.Errorf("set cgroup memory fail %w", err)
			}
		}
		return nil
	}
}

// 删除 cgroupPath 对应的 cgroup
func (s *MemorySubSystem) Remove(cgroupPath string) error {
	if SubsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err != nil {
		return err
	} else {
		// 删除 cgroup 便是删除对应的 cgroupPath 目录
		return os.Remove(SubsysCgroupPath)
	}
}

// 将一个进程加入到 cgroupPath 对应的 cgroup 中
func (s *MemorySubSystem) Apply(cgroupPath string, pid int) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err != nil {
		return fmt.Errorf("get cgroup %s error: %w", cgroupPath, err)
	} else {
		// 把进程的 PID 写到 cgroup 的虚拟文件系统对应目录下的 “task” 文件中
		if err := os.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("set cgroup proc fail %w", err)
		}
		return nil
	}
}
