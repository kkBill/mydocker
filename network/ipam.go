package network

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"net"
	"os"
	"path"
	"strings"
)

const ipamDefaultAllocatorPath = "/var/run/mydocker/network/ipam/subnet.json"

// 存放IP地址分配信息
type IPAM struct {
	// 分配文件存放位置
	SubnetAllocatorPath string
	// 网段和位图算法的数组map，key是网段，value 是分配的位图数组
	Subnets *map[string]string
}

// 初始化一个IPAM的对象
var ipAllocator = &IPAM{
	SubnetAllocatorPath: ipamDefaultAllocatorPath,
}

// 加载网段地址分配信息
func (ipam *IPAM) load() error {
	if _, err := os.Stat(ipam.SubnetAllocatorPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}
	subnetConfigFile, err := os.Open(ipam.SubnetAllocatorPath)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}
	subnetJson := make([]byte, 2000)
	n, err := subnetConfigFile.Read(subnetJson)
	if err != nil {
		return err
	}

	err = json.Unmarshal(subnetJson[:n], ipam.Subnets)
	if err != nil {
		logrus.Errorf("Error dump allocation info, %v", err)
		return err
	}
	return nil
}

// 存储网段地址分配信息
func (ipam *IPAM) dump() error {
	ipamConfigFileDir, _ := path.Split(ipam.SubnetAllocatorPath)
	if _, err := os.Stat(ipamConfigFileDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(ipamConfigFileDir, 0644)
		} else {
			return err
		}
	}
	subnetConfigFile, err := os.OpenFile(ipam.SubnetAllocatorPath, os.O_TRUNC | os.O_WRONLY | os.O_CREATE, 0644)
	defer subnetConfigFile.Close()

	if err != nil {
		return err
	}

	ipamConfigJson, err := json.Marshal(ipam.Subnets)
	if err != nil {
		return err
	}

	_, err = subnetConfigFile.Write(ipamConfigJson)
	if err != nil {
		return err
	}
	return nil
}

// 注意这个函数的写法，返回值的写法是 (ip net.IP, err error)，而不是(net.IP, error)
// 注意这两者的区别，在return的时候没有像往常一样返回这两个返回值，而是在函数体内直接用了
// 在网段中分配一个可用的IP地址，并将IP地址分配信息记录到文件中
func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	ipam.Subnets = &map[string]string{}
	// 从文件中加载已经分配的网段信息
	err = ipam.load()
	if err != nil {
		logrus.Errorf("Allocate: load allocation info error, %v", err)
	}

	// 比如网段是 127.0.0.0/8，其子网掩码是 255.0.0.0
	// 那么该函数返回的 ones=8, bits=24
	ones, bits := subnet.Mask.Size()
	logrus.Infof("ones: %v, bits: %v", ones, bits)

	// 如果之前没有分配过这个网段，则初始化网段的分配配置
	if _, exist := (*ipam.Subnets)[subnet.String()]; !exist {
		(*ipam.Subnets)[subnet.String()] = strings.Repeat("0", 1<<uint8(bits-ones))
	}

	// 遍历网段的位图数组
	for c := range ((*ipam.Subnets)[subnet.String()]) {
		// 找到数组中为0的项和数组序号（即可以分配的ip）
		if (*ipam.Subnets)[subnet.String()][c] == '0' {
			// 设置这个为0的序号值为1，表示分配这个ip
			ipalloc := []byte((*ipam.Subnets)[subnet.String()])
			//
			ipalloc[c] = '1'
			(*ipam.Subnets)[subnet.String()] = string(ipalloc)
			// 这里的IP为初始IP，比如对于网段192.168.0.0/16，则IP就是192.168.0.0
			ip = subnet.IP // IP 的数据类型是 []byte

			// 通过网段的IP与上面的偏移相加，计算出分配的IP地址
			for t := uint(4); t > 0; t-- {
				[]byte(ip)[4-t] += uint8(c >> ((t - 1) * 8))
			}
			ip[3] += 1
			break
		}
	}
	ipam.dump()
	return
}

// 释放IP地址
func (ipam *IPAM) Release(subnet *net.IPNet, ipaddr *net.IP) error {
	ipam.Subnets = &map[string]string{}

	// ?
	_, subnet, _ = net.ParseCIDR(subnet.String())

	// 从文件中加载网段的分配信息
	err := ipam.load()
	if err != nil {
		logrus.Errorf("Release: load allocation info error, %v", err)
	}
	// 计算IP地址在网段位图中的索引位置
	c := 0
	// 将IP地址转换成 4 个字节的表示方式
	releaseIP := ipaddr.To4()
	// 细节！
	releaseIP[3] -= 1

	for t := uint(4); t > 0; t-- {
		c += int(releaseIP[t-1]-subnet.IP[t-1]) << ((4 - t) * 8)
	}

	// 将索引位置的值置为0 (必须先转化成byte数组才可以)
	bytes := []byte((*ipam.Subnets)[subnet.String()])
	bytes[c] = '0'
	(*ipam.Subnets)[subnet.String()] = string(bytes)
	// 保存更新后的信息
	ipam.dump()
	return nil
}
