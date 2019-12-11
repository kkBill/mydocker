package main

import (
	"fmt"
	"net"
)

func main() {
	//fmt.Println(net.ParseIP("192.0.2.1"))
	//fmt.Println(net.ParseIP("2001:db8::68"))
	//fmt.Println(net.ParseIP("192.0.2"))

	ip := net.ParseIP("172.16.0.0")
	ipv4Mask := net.CIDRMask(12, 32)
	ip.Mask(ipv4Mask)
	fmt.Println(ip)
	ip = ip.To4() // 将IP地址转换成以4个字节表示的方式
	fmt.Println(len(ip))

	var c uint32 = 65555
	for t := uint(4); t > 0; t-- {
		[]byte(ip)[4-t] += uint8(c >> ((t - 1) * 8))
	}
	ip[3] += 1
	fmt.Println(net.ParseIP(ip.String()))
	//ipv4Mask := net.CIDRMask(24, 32)
	//fmt.Println(ip.Mask(ipv4Mask))
	//fmt.Println(ip.To4())


	//ipv4Addr, ipv4Net, err := net.ParseCIDR("192.0.2.1/24")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//fmt.Println(ipv4Addr)
	//fmt.Println(ipv4Net)
}
