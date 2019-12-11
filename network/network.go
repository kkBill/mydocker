package network

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/kkBill/mydocker/container"
	"github.com/vishvananda/netlink"
	"net"
	"os"
	"path"
	"path/filepath"
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

	// 通过IPAM分配网关IP
	ip, err := ipAllocator.Allocate(ipNet)
	if err != nil {
		return err
	}
	ipNet.IP = ip

	// 通过调用指定的网络驱动创建网络，本项目中以Bridge驱动实现
	network, err := drivers[driver].Create(ipNet.String(), name);
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
		logrus.Errorf("dump: OpenFile error: ", err)
		return err
	}
	defer file.Close()

	// 通过json将网络对象转化成json字符串
	bytes, err := json.Marshal(network)
	if err != nil {
		logrus.Errorf("dump: json.Marshal error: ", err)
		return err
	}
	// 将json字符串写入到文件中
	_, err = file.Write(bytes)
	if err != nil {
		logrus.Errorf("dump: file.Write error: ", err)
		return err
	}
	return nil
}

// 从文件中读取网络配置信息
func (network *Network) load(dumpPath string) error {
	// 打开配置文件
	file, err := os.Open(dumpPath)
	if err != nil {
		logrus.Errorf("load: os.Open error: ", err)
		return err
	}
	defer file.Close()

	// 从配置文件中读取json串
	nwjson := make([]byte, 2000)
	n, err := file.Read(nwjson)
	if err != nil {
		logrus.Errorf("load: file.Read error: ", err)
		return err
	}
	// 将json串解码成网络对象
	if err := json.Unmarshal(nwjson[:n], network); err != nil {
		logrus.Errorf("load: json.Unmarshal error: ", err)
		return err
	}
	return nil
}

// 创建容器并连接网络
func Connect(networkName string, cinfo *container.ContainerInfo) error {
	// 从networks字典中取到容器连接的网络信息，networks 保存了当前已经创建的网络
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("Connect: No such network: %s", networkName)
	}

	// 通过调用 IPAM 从网络的网段中获取可用的IP作为容器IP地址
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}

	// 创建网络端点
	endpoint := &Endpoint{
		ID: fmt.Sprintf("%s-%s", cinfo.Id, networkName),
		//Device:      netlink.Veth{},
		IPAddress: ip,
		//MacAddress:  nil,
		PortMapping: cinfo.PortMapping,
		Network:     network,
	}

	// 调用网络驱动的Connect()方法，连接和配置网络端点
	if err := drivers[network.Driver].Connect(network, endpoint); err != nil {
		return err
	}

	// 进入容器的network namespace 配置容器网络设备的IP地址和路由
	if err := configEndpointIpAddressAndRoute(endpoint, cinfo); err != nil {
		return err
	}

	// 配置容器到宿主机的端口映射
	if err := configPortMapping(endpoint, cinfo); err != nil {
		return err
	}

	return nil
}

func configEndpointIpAddressAndRoute(endpoint *Endpoint, cinfo *container.ContainerInfo) error {

	return nil
}

func configPortMapping(endpoint *Endpoint, cinfo *container.ContainerInfo) error {

	return nil
}

// 展示网络列表
// 通过 mydocker network list 显示当前创建了哪些网络
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

func ListNetwork()  {
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
		return fmt.Errorf("DeleteNetwork: No such network: %s",networkName)
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