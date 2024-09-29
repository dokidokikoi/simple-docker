package network

import (
	"encoding/json"
	"net"
	"os"
	"path"
	"strings"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

const ipamDefaultAllocatorPath = "/var/run/mydocker/network/ipam/subnet.json"

// ip 地址分配信息
type IPAM struct {
	// 分配文件存放位置
	SubnetAllocatorPath string
	// 网段和位图算法的数组 map,key 是网段，value 是分配的位图数组
	Subnets *map[string]string
}

// 默认的 IPAM 对象
var ipAllocator = &IPAM{
	SubnetAllocatorPath: ipamDefaultAllocatorPath,
}

func (ipam *IPAM) load() error {
	if _, err := os.Stat(ipam.SubnetAllocatorPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}
	subnetConfigFile, err := os.Open(ipam.SubnetAllocatorPath)
	if err != nil {
		return err
	}
	defer subnetConfigFile.Close()

	subnetJson := make([]byte, 2000)
	n, err := subnetConfigFile.Read(subnetJson)
	if err != nil {
		return err
	}
	err = json.Unmarshal(subnetJson[:n], ipam.Subnets)
	if err != nil {
		return err
	}
	return nil
}

func (ipam *IPAM) dump() error {
	ipamConfigFileDir, _ := path.Split(ipam.SubnetAllocatorPath)
	if _, err := os.Stat(ipamConfigFileDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(ipamConfigFileDir, 0644)
		} else {
			return err
		}
	}

	subnetConfigFile, err := os.OpenFile(ipam.SubnetAllocatorPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer subnetConfigFile.Close()

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

func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	// 存在网段中地址分配信息的数组
	ipam.Subnets = &map[string]string{}

	// 从文件中加载已经分配的网段信息
	err = ipam.load()
	if err != nil {
		zaplog.L().Error("load ipam subnets error", zap.Error(err))
		return
	}

	_, subnet, _ = net.ParseCIDR(subnet.String())
	one, size := subnet.Mask.Size()
	// 如果没有分配过这个网段，则初始化网段的分配配置
	if _, exist := (*ipam.Subnets)[subnet.String()]; !exist {
		// 用 ‘0’ 填满这个网段的配置， 1 << uint(size - one) 表示这个网段中有多少个可用地址
		// size - one 是子网掩码后面的网络位数，2^(size-one) 表示网段中的可用 ip 数
		(*ipam.Subnets)[subnet.String()] = strings.Repeat("0", 1<<uint8(size-one))
	}

	// 遍历网段的位图数组
	for i := range (*ipam.Subnets)[subnet.String()] {
		// 找到数组中为 ‘0’ 的序号，即为可分配的 ip
		if (*ipam.Subnets)[subnet.String()][i] == '0' {
			// 设置这个 ‘0’ 为 ‘1’，即分配这个 ip
			ipalloc := []byte((*ipam.Subnets)[subnet.String()])
			ipalloc[i] = '1'
			(*ipam.Subnets)[subnet.String()] = string(ipalloc)
			ip = subnet.IP

			// 通过网段的 ip 与上面的偏移相加计算出分配的 ip 地址，由于 ip 地址是 byte 的数组，
			// 需要通过数组中的每一项加所需要的值，比如网段是 172.16.0.0/12, 数组号是 65555,
			// 那么在 [172,16,0,0] 上依次加 uint8(65555 >> 24)、uint8(65555 >> 16)...
			for t := uint(4); t > 0; t-- {
				[]byte(ip)[4-t] += uint8(i >> ((t - 1) * 8))
			}
			// 由于 ip 是从 1 开始分配的，所以最后一位需要加一
			ip[3] += 1
			break
		}
	}

	// 保存到文件
	ipam.dump()
	return
}

func (ipam *IPAM) Release(subnet *net.IPNet, ipaddr *net.IP) error {
	ipam.Subnets = &map[string]string{}

	_, subnet, _ = net.ParseCIDR(subnet.String())
	// 从文件中加载网段的分配信息
	err := ipam.load()
	if err != nil {
		return err
	}

	// 计算 ip 在位图中的索引
	c := 0
	releaseIP := ipaddr.To4()
	releaseIP[3] -= 1
	for t := uint(4); t > 0; t-- {
		sub := releaseIP[t-1] - subnet.IP[t-1]
		c += int(sub << ((4 - t) * 8))
	}

	ipalloc := []byte((*ipam.Subnets)[subnet.String()])
	ipalloc[c] = '0'
	(*ipam.Subnets)[subnet.String()] = string(ipalloc)

	ipam.dump()
	return nil
}
