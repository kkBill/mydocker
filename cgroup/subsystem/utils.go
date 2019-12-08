package subsystem

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
)

// cat /proc/self/mountinfo 的结果内容如下：
// 33 28 0:30 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:12 - cgroup cgroup rw,seclabel,memory
//
func FindCgroupMountpoint(subsystem string) string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close() //在defer后指定的函数会在函数退出前调用

	scanner := bufio.NewScanner(f)
	for scanner.Scan() { // scanner.Scan() return bool type
		text := scanner.Text()
		fields := strings.Split(text, " ")
		for _, opt := range strings.Split(fields[len(fields)-1], ",") {
			if opt == subsystem {
				return fields[4]; // 下标为 4 是根据下面计算出的
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ""
	}
	return ""
}

// 该函数的作用是获取当前 subsystem 在虚拟文件系统中的路径
func GetCgroupPath(subsystem string, cgroupPath string, autoCreate bool) (string, error)  {
	cgroupRoot := FindCgroupMountpoint(subsystem)
	// Stat returns a FileInfo describing the named file.
	if _, err := os.Stat(path.Join(cgroupRoot, cgroupPath)); err == nil || (autoCreate && os.IsNotExist(err)) {
		if os.IsNotExist(err) {
			if err := os.Mkdir(path.Join(cgroupRoot, cgroupPath), 0755); err == nil {
			} else {
				return "", fmt.Errorf("error create cgroup %v", err)
			}
		}
		return path.Join(cgroupRoot, cgroupPath), nil
	} else {
		return "", fmt.Errorf("cgroup path error %v", err)
	}
}