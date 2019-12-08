package cgroup

import (
	"github.com/kkBill/mydocker/cgroup/subsystem"
)

// 通过 CgroupManager 把不同的资源限制模块(subsystem)给管理起来
type CgroupManager struct {
	Path     string
	Resource *subsystem.ResourceConfig
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{
		Path: path, // 记得加 ","
	}
}

func (c *CgroupManager) Apply(pid int) error {
	for _, subSys := range subsystem.SubsystemsItems {
		_ = subSys.Apply(c.Path, pid)
	}
	return nil
}

func (c *CgroupManager) Set(res *subsystem.ResourceConfig) error {
	for _, subSys := range subsystem.SubsystemsItems {
		_ = subSys.Set(c.Path, res)
	}
	return nil
}

func (c *CgroupManager) Remove() error  {
	for _, subSys := range subsystem.SubsystemsItems {
		_ = subSys.Remove(c.Path)
	}
	return nil
}