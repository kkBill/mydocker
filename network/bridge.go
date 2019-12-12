package network

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"net"
	"os/exec"
	"strings"
)

type BridgeNetworkDriver struct {
}

func (d *BridgeNetworkDriver) Name() string {
	return "bridge"
}

func (d *BridgeNetworkDriver) Create(subnet, name string) (*Network, error) {
	ip, ipNet, err := net.ParseCIDR(subnet)
	ipNet.IP = ip

	// 初始化网络对象
	n := &Network{
		Name:    name,
		IpRange: ipNet,
		Driver:  d.Name(),
	}

	// 配置bridge
	err = d.initBridge(n)
	if err != nil {
		logrus.Errorf("error init bridge: %v", err)
	}
	return n, err
}

func (d *BridgeNetworkDriver) Delete(network Network) error {
	//
	bridgeName := network.Name

	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	return netlink.LinkDel(br)
}

func (d *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	la := netlink.NewLinkAttrs()
	la.Name = endpoint.ID[:5]
	la.MasterIndex = br.Attrs().Index

	endpoint.Device = netlink.Veth{
		LinkAttrs: la,
		PeerName:  "cif-" + endpoint.ID[:5],
	}

	if err = netlink.LinkAdd(&endpoint.Device); err != nil {
		return fmt.Errorf("Error Add Endpoint Device: %v", err)
	}

	if err = netlink.LinkSetUp(&endpoint.Device); err != nil {
		return fmt.Errorf("Error Add Endpoint Device: %v", err)
	}
	return nil
}

func (d *BridgeNetworkDriver) Disconnect(network Network, endpoint *Endpoint) error {
	return nil
}

// 初始化 bridge 设备
func (d *BridgeNetworkDriver) initBridge(n *Network) error {
	// 1.创建bridge设备
	bridgeName := n.Name
	if err := createBridgeInterface(bridgeName); err != nil {
		return fmt.Errorf("initBridge: error add bridge: %s, error: %v", bridgeName, err)
	}

	// 2.设置bridge设备的地址和路由
	gatewayIP := *n.IpRange
	gatewayIP.IP = n.IpRange.IP
	if err := setInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return fmt.Errorf("initBridge: error assigning address [%s] on bridge [%s] with an error of [%v]", gatewayIP, bridgeName, err)
	}

	// 3.启动bridge设备
	if err := setInterfaceUP(bridgeName); err != nil {
		return fmt.Errorf("initBridge: error set bridge up: %s, error: %v", bridgeName, err)
	}

	// 4.设置iptables的SNAT规则
	if err := setupIPTables(bridgeName, n.IpRange); err != nil {
		return fmt.Errorf("initBridge: error setting iptables for [%s], error: %v", bridgeName, err)
	}

	return nil
}

// 创建 bridge 设备
func createBridgeInterface(bridgeName string) error {
	// 先检查是否存在这个同名的 bridge 设备
	_, err := net.InterfaceByName(bridgeName)
	// 为什么这里是这么写的 --> 如果已经存在，err为nil
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// 初始化一个netlink的link基础对象，link 的名字就是bridge虚拟设备的名字
	// 什么是 link 对象？（这里有值得深入的点！！！https://www.linux.com/tutorials/understanding-linux-links/）
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName

	// 使用 link 对象创建 netlink 的bridge 对象
	br := &netlink.Bridge{la}
	//
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("Bridge creation failed for bridge %s: %v", bridgeName, err)
	}

	return nil
}

// 创建bridge设备的地址和路由
// 设置网络接口的IP地址，例如
func setInterfaceIP(name, ip string) error {
	// 找到需要设置的网络接口
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("setInterfaceIP: error get interface: %v", err)
	}

	ipNet, err := netlink.ParseIPNet(ip)
	if err != nil {
		return err
	}

	addr := &netlink.Addr{
		IPNet: ipNet,
		Label: "",
		Flags: 0,
		Scope: 0,
		Peer:  nil,
	}

	//
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("setInterfaceIP: error add addr: %v", err)
	}

	return nil
}

// 设置网络接口为 up 状态
func setInterfaceUP(interfaceName string) error {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		//noinspection ALL
		return fmt.Errorf("setInterfaceUP: error retrieving a link named [%s]: %v", link.Attrs().Name, err)
	}
	// 相当于执行 p124页中的 ip link set xxx up (这里的xxx 可以是veth0，也可以是br)
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("error enabling interface for %s: %v", interfaceName, err)
	}
	return nil
}

// 设置 iptables 对应的 bridge 的 MASQUERADE 规则
func setupIPTables(bridgeName string, subnet *net.IPNet) error {
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)

	// 执行 iptables 命令配置 SNAT 规则
	output, err := cmd.Output()
	if err != nil {
		logrus.Errorf("iptables output, %v", output)
	}
	return nil
}