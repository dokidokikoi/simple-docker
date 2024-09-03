package subsystems

import (
	"fmt"
	"os"
	"path"
	"strconv"
)

type CpuSubSystem struct {
}

func (*CpuSubSystem) Name() string {
	return "cpu"
}

func (s *CpuSubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	if subsystemCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err != nil {
		return err
	} else {
		if res.CpuShare != "" {
			if err := os.WriteFile(path.Join(subsystemCgroupPath, "cpu.shares"), []byte(res.CpuShare), 0644); err != nil {
				return fmt.Errorf("set cgroup cpu share fail %v", err)
			}
		}
		return nil
	}
}

func (s *CpuSubSystem) Remove(cgroupPath string) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		return os.RemoveAll(subsysCgroupPath)
	} else {
		return err
	}
}

func (s *CpuSubSystem) Apply(cgroupPath string, pid int) error {
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		if err := os.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("set cgroup proc fail %v", err)
		}
		return nil
	} else {
		return fmt.Errorf("get cgroup %s error: %v", cgroupPath, err)
	}
}
