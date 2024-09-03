package cgroups

import (
	"docker/cgroups/subsystems"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

type CgroupManager struct {
	// cgroup 在 hierarchy 中的路径，相当于创建的 cgroup 目录相对于各 root cgroup 目录的路径
	Path string
	// 资源配置
	Resource *subsystems.ResourceConfig
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{
		Path: path,
	}
}

// 将进程 Pid 加入到每个 cgroup 中
func (c *CgroupManager) Apply(pid int) error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		subSysIns.Apply(c.Path, pid)
	}
	return nil
}

// 设置各个 subsystem 挂载中的 cgroup 资源限制
func (c *CgroupManager) Set(res *subsystems.ResourceConfig) error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		subSysIns.Set(c.Path, res)
	}
	return nil
}

// 释放各个 subsystem 挂载中的 cgroup
func (c *CgroupManager) Destory() error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		if err := subSysIns.Remove(c.Path); err != nil {
			zaplog.L().Warn("remove cgroup fail", zap.Error(err))
		}
	}
	return nil
}
