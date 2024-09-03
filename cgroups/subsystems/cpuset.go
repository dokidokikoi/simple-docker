package subsystems

import (
	"fmt"
	"os"
	"path"
	"strconv"
)

type CpusetSubSystem struct {
}

func (s *CpusetSubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err != nil {
		return err
	} else {
		if res.CpuSet != "" {
			if err := os.WriteFile(path.Join(subsysCgroupPath, "cpuset.cpus"), []byte(res.CpuSet), 0644); err != nil {
				return fmt.Errorf("set cgroup cpuset fail %w", err)
			}
		}
		return nil
	}
}

func (s *CpusetSubSystem) Remove(cgroupPath string) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err != nil {
		return err
	} else {
		return os.RemoveAll(subsysCgroupPath)
	}
}

func (s *CpusetSubSystem) Apply(cgroupPath string, pid int) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err != nil {
		return fmt.Errorf("get cgroup %s error: %w", cgroupPath, err)
	} else {
		if err := os.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("set cgroup proc fail %w", err)
		}
		return nil
	}
}

func (s *CpusetSubSystem) Name() string {
	return "cpuset"
}
