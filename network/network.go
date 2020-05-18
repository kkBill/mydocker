package network

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/container"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
)

var (
	defaultNetworkPath = "/var/run/mydocker/network/network/"
	drivers            = map[string]NetworkDriver{}
	networks           = map[string]*Network{}
)

type Network struct {
	Name    string     // 网络名称
	IpRange *net.IPNet // 地址段
	Driver  string     // 网络驱动名
}

type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"dev"`
	IPAddress   net.IP           `json:"ip"`
	MacAddress  net.HardwareAddr `json:"mac"`
	PortMapping []string         `json:"portmapping"`
	Network     *Network
}

type NetworkDriver interface {
	// 驱动名
	Name() string
	// 创建网络
	Create(subnet, name string) (*Network, error)
	// 删除网络
	Delete(network Network) error
	// 连接容器网络端点到网络
	Connect(network *Network, endpoint *Endpoint) error
	// 从网络上移除容器网络端点
	Disconnect(network Network, endpoint *Endpoint) error
}

// 创建网络
func CreateNetwork(driver, subnet, name string) error {
	// 将网段的字符串转换成net.IPNet的对象
	_, ipNet, _ := net.ParseCIDR(subnet)

	// 通过 IPAM 分配网关IP
	ip, err := ipAllocator.Allocate(ipNet)
	if err != nil {
		return err
	}
	ipNet.IP = ip
	// logrus.Infof("ip: %s", ip.To4())  // 可正常分配
	// 通过调用指定的网络驱动创建网络，本项目中以Bridge驱动实现
	network, err := drivers[driver].Create(ipNet.String(), name);
	logrus.Infof("network: %v", network)
	if err != nil {
		return err
	}

	// 保存网络信息
	if err := network.dump(defaultNetworkPath); err != nil {
		return err
	}

	return nil
}

// 将网络的配置信息保存在文件系统中
func (network *Network) dump(dumpPath string) error {
	// 检查保存的目录是否存在，不存在则创建
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dumpPath, 0644)
		} else {
			return err
		}
	}

	// 保存的文件是网络的名字
	nwPath := path.Join(dumpPath, network.Name) // Join是专门拼接路径
	// 打开文件用于写入
	file, err := os.OpenFile(nwPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logrus.Errorf("dump: OpenFile error: %v", err)
		return err
	}
	defer file.Close()

	// 通过json将网络对象转化成json字符串
	bytes, err := json.Marshal(network)
	if err != nil {
		logrus.Errorf("dump: json.Marshal error: %v", err)
		return err
	}
	// 将json字符串写入到文件中
	_, err = file.Write(bytes)
	if err != nil {
		logrus.Errorf("dump: file.Write error: %v", err)
		return err
	}
	return nil
}

// 从文件中读取网络配置信息
func (network *Network) load(dumpPath string) error {
	// 打开配置文件
	file, err := os.Open(dumpPath)
	if err != nil {
		logrus.Errorf("load: os.Open error: %v", err)
		return err
	}
	defer file.Close()

	// 从配置文件中读取json串
	nwjson := make([]byte, 2000)
	n, err := file.Read(nwjson)
	if err != nil {
		logrus.Errorf("load: file.Read error: %v", err)
		return err
	}
	// 将json串解码成网络对象
	if err := json.Unmarshal(nwjson[:n], network); err != nil {
		logrus.Errorf("load: json.Unmarshal error: %v", err)
		return err
	}
	return nil
}

// 创建容器并连接网络
func Connect(networkName string, cinfo *container.ContainerInfo) error {
	// 从networks字典中取到容器连接的网络信息，networks 保存了当前已经创建的网络
	network, ok := networks[networkName]
	logrus.Infof("network.IpRange: %v", network.IpRange) // 这里是正确的！

	if !ok {
		return fmt.Errorf("Connect: No such network: %s", networkName)
	}

	// 通过调用 IPAM 从网络的网段中获取可用的IP作为容器IP地址
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}
	logrus.Infof("Connect: ip: %v",ip.To4()) // 这里的 ip 是空的，问题一定出在 Allocate()

	// 创建网络端点
	endpoint := &Endpoint{
		ID: fmt.Sprintf("%s-%s", cinfo.Id, networkName),
		//Device:      netlink.Veth{},
		IPAddress: ip,
		//MacAddress:  nil,
		PortMapping: cinfo.PortMapping,
		Network:     network,
	}
	//logrus.Infof("Connect: endpoint: %v",endpoint)

	// 调用网络驱动的Connect()方法，连接和配置网络端点
	if err := drivers[network.Driver].Connect(network, endpoint); err != nil {
		logrus.Errorf("drivers[network.Driver].Connect: error %v", err)
		return err
	}

	// 进入容器的network namespace 配置容器网络设备的IP地址和路由
	if err := configEndpointIpAddressAndRoute(endpoint, cinfo); err != nil {
		logrus.Errorf("configEndpointIpAddressAndRoute: error %v", err)
		return err
	}

	// 配置容器到宿主机的端口映射
	if err := configPortMapping(endpoint, cinfo); err != nil {
		logrus.Errorf("configPortMapping: error %v", err)
		return err
	}

	return nil
}

// 配置容器网络端点的地址和路由
func configEndpointIpAddressAndRoute(endpoint *Endpoint, cinfo *container.ContainerInfo) error {
	//peerLink, err := netlink.LinkByName(endpoint.Device.Name) // 难道这里就是bug所在？
	// veth 的另一端
	peerLink, err := netlink.LinkByName(endpoint.Device.PeerName)
	if err != nil {
		return fmt.Errorf("configEndpointIpAddressAndRoute: fail config endpoint: %v", err)
	}
	// 将容器的网络端点加入到容器的network namespace
	// 并使该函数之后的操作都在这个network namespace中进行
	// 执行完 configEndpointIpAddressAndRoute() 之后，再恢复为默认的 network namespace
	defer enterContainerNetns(&peerLink, cinfo)()

	interfaceIP := *endpoint.Network.IpRange
	interfaceIP.IP = endpoint.IPAddress

	// 设置容器内 Veth 端的ip
	if err = setInterfaceIP(endpoint.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", endpoint.Network, err)
	}

	// 启动容器内的 Veth 端点
	if err = setInterfaceUP(endpoint.Device.PeerName); err != nil {
		logrus.Errorf("configEndpointIpAddressAndRoute: setInterfaceUP error: %v",err)
		return err
	}

	// net namespace 中默认本地地址127.0.0.1的“lo”网卡是关闭状态
	// 启动它以保证容器访问自己的请求
	if err = setInterfaceUP("lo"); err != nil {
		logrus.Errorf("configEndpointIpAddressAndRoute: setInterfaceUP \"lo\" error: %v",err)
		return err
	}

	// 设置容器内的外部请求都通过容器内的Veth端点进行访问
	// 0.0.0.0/0 的网段，表示所有的ip地址端
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")
	// 构建要添加的路由数据，包括网络设备、网关IP和目的网段
	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        endpoint.Network.IpRange.IP,
		Dst:       cidr,
	}
	// 添加路由到容器的网络空间
	// 相当于执行 route add 命令
	if err = netlink.RouteAdd(defaultRoute); err != nil {
		logrus.Errorf("configEndpointIpAddressAndRoute: route add error: %v",err)
		return err
	}

	return nil
}

// 锁定当前程序所执行的线程，使当前线程进入到容器的网络空间
// 返回值是一个函数指针，执行这个返回函数才会退出容器的网络空间，回归宿主机的网络空间
func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func() {
	// 找到容器的net namespace
	// /proc/[pid]/ns/net 打开这个文件的文件描述符就可以来操作net namespace
	// ContainerInfo中的PID，即容器再宿主机上映射的PID
	// 它对应的 /proc/[pid]/ns/net 就是容器内部的 net namespace
	file, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", cinfo.Pid), os.O_RDONLY, 0)
	if err != nil {
		logrus.Errorf("error get container net namespace, %v", err)
	}

	// 取到文件的文件描述符 file descriptor
	nsFD := file.Fd()

	// 锁定当前程序执行的线程，以保证一直在所需要的net namespace 中
	//
	runtime.LockOSThread()

	// 修改veth peer 另外一端，将其移到容器的 net namespace中
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		logrus.Errorf("error set link netns , %v", err)
	}

	// 获取当前的网络namespace
	origins, err := netns.Get()
	if err != nil {
		logrus.Errorf("error get current netns, %v", err)
	}

	// 设置当前进程到新的网络namespace，并在函数执行完成之后再恢复到之前的namespace
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		logrus.Errorf("error set netns, %v", err)
	}

	// 调用此函数，将程序恢复到原来的 net namespace 中
	return func() {
		// 恢复到之前的 net namespace 中
		netns.Set(origins)
		origins.Close()
		runtime.UnlockOSThread()
		file.Close()
	}
}

// 配置端口映射
func configPortMapping(endpoint *Endpoint, cinfo *container.ContainerInfo) error {
	for _, pm := range endpoint.PortMapping {
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			logrus.Errorf("port mapping format error, %v", pm)
			continue
		}
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], endpoint.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		//err := cmd.Run()
		output, err := cmd.Output()
		if err != nil {
			logrus.Errorf("iptables Output, %v", output)
			continue
		}
	}
	return nil
}

// 初始化网络（也就是把网络配置信息从文件中读取到内存相应的数据结构中以供调用）
func Init() error {
	// 加载网络驱动
	bridgeDriver := BridgeNetworkDriver{}
	drivers[bridgeDriver.Name()] = &bridgeDriver

	// 判断网络的配置目录是否存在，不存在则创建
	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
		} else {
			return err
		}
	}

	// 检查网络配置目录中的所有文件
	filepath.Walk(defaultNetworkPath, func(nwPath string, info os.FileInfo, err error) error {
		// 如果是目录，则跳过
		if info.IsDir() {
			return nil
		}
		// 加载文件名作为网络名
		_, nwName := path.Split(nwPath) // 分解目录，分别返回目录和文件名
		nw := &Network{
			Name: nwName,
		}
		// 调用network.load方法加载网络的配置信息
		if err := nw.load(nwPath); err != nil {
			logrus.Errorf("error load network: %s", err)
		}
		// 将网络配置的信息加入到networks字典中
		networks[nwName] = nw
		return nil
	})
	return nil
}

// 展示网络列表
// 通过 mydocker network list 显示当前创建了哪些网络
func ListNetwork() {
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "NAME\tIP-RANGE\tDRIVER\n")
	// 遍历网络信息
	for _, nw := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n", nw.Name, nw.IpRange, nw.Driver)
	}
	// 输出到标准输出
	if err := w.Flush(); err != nil {
		logrus.Errorf("Flush error %v", err)
		return
	}
}

// 删除网络
func DeleteNetwork(networkName string) error {
	// 查找网络是否存在
	nw, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("DeleteNetwork: No such network: %s", networkName)
	}

	// 调用 IPAM 的实例 ipAllocator 释放网络网关的IP
	if err := ipAllocator.Release(nw.IpRange, &nw.IpRange.IP); err != nil {
		return fmt.Errorf("DeleteNetwork: remove network gateway ip: %s", err)
	}

	// 调用网络驱动删除网络创建的设备与配置
	if err := drivers[nw.Driver].Delete(*nw); err != nil {
		return fmt.Errorf("DeleteNetwork: remove network driver: %s", err)
	}

	// 从网络的配置目录中删除该网络对应的配置文件
	if err := nw.remove(defaultNetworkPath); err != nil {
		return fmt.Errorf("DeleteNetwork: remove network path: %s", err)
	}

	return nil
}

// 从网络配置目录中删除网络的配置文件
func (nw *Network) remove(dumpPath string) error {
	// 网络对应的配置文件
	// 检查文件状态，如果文件已经不存在则直接返回
	if _, err := os.Stat(path.Join(dumpPath, nw.Name)); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	} else {
		return os.Remove(path.Join(dumpPath, nw.Name))
	}
}
