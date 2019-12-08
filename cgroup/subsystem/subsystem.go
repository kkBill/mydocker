package subsystem

// 用于传递资源限制配置的结构体
// subsystem 作为资源控制模块，可以限制的资源类型可以通过 lssubsys -a 命令进行查看
// 这里只限制以下 3 种资源类型
type ResourceConfig struct {
	MemoryLimit string //内存限制
	CpuShare string    //cpu 时间片权重
	CpuSet string      //cpu 核心数
}

// 这里将 cgroup 抽象成 path
type Subsystem interface {
	// 返回 subsystem 的名字
	Name() string
	//
	Set(path string, res * ResourceConfig) error
	// 将进程添加到某个 cgroup 中
	Apply(path string, pid int) error
	// 移除 cgroup
	Remove(path string) error
}

// 这样声明变量是什么意思？
var (
	SubsystemsItems = []Subsystem{
		&CpusetSubSystem{},
		&MemorySubSystem{},
		&CpuSubSystem{},
	}
)