package network

import (
	"docker/container"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"go.uber.org/zap"
)

var (
	defaultNetworkPath = "/var/run/mydocker/network/network/"
	drivers            = map[string]NetworkDriver{}
	networks           = map[string]*Network{}
)

type Network struct {
	// 网络名
	Name string
	// 地址段
	IpRange *net.IPNet
	// 网络驱动名
	Driver string
}

func (nw *Network) dump(dumpPath string) error {
	// 检查保存的目录是否存在，不存在则创建
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dumpPath, 0644)
		} else {
			return err
		}
	}
	// 保存的文件名是网络名
	nwPath := path.Join(dumpPath, nw.Name)
	// 清空写入、只写、不存在则创建
	newFile, err := os.OpenFile(nwPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		zaplog.L().Error("open file error", zap.String("path", nwPath), zap.Error(err))
		return err
	}
	defer newFile.Close()

	nwJson, err := json.Marshal(nw)
	if err != nil {
		return err
	}
	_, err = newFile.Write(nwJson)
	if err != nil {
		return err
	}
	return nil
}

func (nw *Network) load(dumpPath string) error {
	nwConfigFile, err := os.Open(dumpPath)
	if err != nil {
		return err
	}
	defer nwConfigFile.Close()

	nwJson := make([]byte, 2000)
	n, err := nwConfigFile.Read(nwJson)
	if err != nil {
		return err
	}
	err = json.Unmarshal(nwJson[:n], nw)
	if err != nil {
		return err
	}
	return nil
}

func (nw *Network) remove(dumpPath string) error {
	if _, err := os.Stat(path.Join(dumpPath, nw.Name)); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	} else {
		// 删除这个网络的配置文件
		return os.Remove(path.Join(dumpPath, nw.Name))
	}
}

type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"dev"`
	IPAddress   net.IP           `json:"ip"`
	MacAddreess net.HardwareAddr `json:"mac"`
	PortMapping []string         `json:"portmapping"`
	Network     *Network
}

type NetworkDriver interface {
	// 驱动名
	Name() string
	// 创建网络
	Create(subnet, name string) (*Network, error)
	// 删除网络
	Delete(network *Network) error
	// 连接容器网络端点到网络
	Connect(network *Network, endpoint *Endpoint) error
	// 从网络上移除容器网络端点
	DisConnect(network *Network, endpoint *Endpoint) error
}

func Init() error {
	// 加载网络驱动
	var bridgeDriver = BridgeNetworkDriver{}
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
	return filepath.Walk(defaultNetworkPath, func(nwPath string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return err
		}
		// 加载文件名作为网络名
		_, nwName := path.Split(nwPath)
		nw := &Network{
			Name: nwName,
		}

		// 加载网络信息
		if err := nw.load(nwPath); err != nil {
			zaplog.L().Error("error load network", zap.Error(err))
		}

		// 将网络的配置信息加入到 networks 字典中
		networks[nwName] = nw
		return nil
	})
}

func CreateNetwork(driver, subnet, name string) error {
	// PareseCIDR 是 Golang net 包的函数，将网段的字符串转换成 net.IPNet 的对象
	_, cidr, _ := net.ParseCIDR(subnet)
	// 透过 IPAM 分配网关，获取到网段中的第一个 IP 作为网关的 IP
	gatewayIP, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return err
	}
	cidr.IP = gatewayIP

	// 调用指定的网络驱动器创建网络，这里的 drivers 字典是各个网络驱动的实例字典，
	// 通过调用网络驱动的 Create 方法创建网络
	nw, err := drivers[driver].Create(cidr.String(), name)
	if err != nil {
		return err
	}
	// 保留网络信息，将网络的信息保存在文件系统中，以便查询和在网络上连接网络端点
	return nw.dump(defaultNetworkPath)

}

func ListNetwork() {
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "NAME\tIpRange\tDriver\n")

	// 遍历网络信息
	for _, nw := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n", nw.Name, nw.IpRange.String(), nw.Driver)
	}
	// 输出到标准输出
	if err := w.Flush(); err != nil {
		zaplog.L().Error("flush error", zap.Error(err))
		return
	}
}

func DeleteNetwork(networkName string) error {
	// 查找网络是否存在
	nw, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("no such network: %s", networkName)
	}

	// 调用 IPAM 的实例 ipAllcocator 释放网络网关的 IP
	if err := ipAllocator.Release(nw.IpRange, &nw.IpRange.IP); err != nil {
		return fmt.Errorf("error remove network gateway ip: %w", err)
	}

	// 调用网络驱动删除网络创建的设备与配置
	if err := drivers[nw.Driver].Delete(nw); err != nil {
		return fmt.Errorf("error remove network dirver error: %w", err)
	}

	// 从网络的配置目录中删除该网络对应的配置文件
	return nw.remove(defaultNetworkPath)
}

// 将容器的网络端点加入到容器的网络空间中
// 并锁定当前程序所执行的线程，使当前线程进入到容器是网络空间
// 返回值是一个函数指针，执行这个返回函才会退出容器的网络空间，回归到宿主机的网络空间
func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func() {
	// 找到容器的 Net Namespace
	// /proc/[pid]/ns/net 打开这个文件的文件描述符就可以来操作 Net NameSpace
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", cinfo.Pid), os.O_RDONLY, 0)
	if err != nil {
		zaplog.L().Error("error get  container net namespace", zap.Error(err))
		return nil
	}
	// 取到文件的描述符
	nsFd := f.Fd()

	// 锁定当前程序所执行的线程，如果不锁定的话
	// go 语言的 goroutine 可能会被调度到别的线程上
	// 就不能保证一直到所需的网络空间中了
	runtime.LockOSThread()

	// 修改 veth peer 另外一端移到容器的 namespace 中
	if err = netlink.LinkSetNsFd(*enLink, int(nsFd)); err != nil {
		zaplog.L().Error("error set link netns", zap.Error(err))
		return nil
	}

	// 获取当前的网络 namespace
	// 以便之后从容器的 Net Namespace 中退出，回到原本的 Net Namespace 中
	origns, err := netns.Get()
	if err != nil {
		zaplog.L().Error("error get current netns", zap.Error(err))
		return nil
	}
	// 设置当前进程到新的网络 namespace
	if err = netns.Set(netns.NsHandle(nsFd)); err != nil {
		zaplog.L().Error("error set netns", zap.Error(err))
		return nil
	}
	// 返回恢复到之前的 namespace 的函数
	return func() {
		// 恢复到上面获取到的之前的 Net Namespace
		netns.Set(origns)
		// 关闭 Namespace 文件
		origns.Close()
		// 取消对当前程序的线程锁定
		runtime.UnlockOSThread()
		// 关闭 Namespace 文件
		f.Close()
	}
}

func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error {
	// 拿到网络端点中 veth 的另一端
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %w", err)
	}
	// 将容器的网络端点加入到容器的网络空间中
	// 并使这个函数下面的操作都在这个网络空间中进行
	// 执行完这个函数后，恢复到默认的网络空间
	defer enterContainerNetns(&peerLink, cinfo)()

	// 获取容器的 ip 及网段，用于配置容器内部的接口地址
	interfaceIP := *ep.Network.IpRange
	interfaceIP.IP = ep.IPAddress
	// 设置容器内 veth 端点的 ip
	if err = setInterfaceIp(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%w", ep.Network, err)
	}

	if err = setInterfaceUp(ep.Device.PeerName); err != nil {
		return err
	}
	// Net Namespace 中默认本地地址 127.0.0.1 的 “lo” 网卡是关闭状态
	// 启动它以保证容器访问自己的请求
	if err = setInterfaceUp("lo"); err != nil {
		return err
	}

	// 设置容器内的外部请求都通过容器内的 veth 端点访问
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")
	// 构建要添加的路由数据，包括网络设备、网关 ip 及目的网段
	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.IpRange.IP,
		Dst:       cidr,
	}
	// 相当于 route add -net 0.0.0.0/0 gw <bridge 网桥地址> dev <容器内的 veth 端点设备>
	if err = netlink.RouteAdd(defaultRoute); err != nil {
		return err
	}

	return nil
}

func configPortMapping(ep *Endpoint, cinfo *container.ContainerInfo) error {
	for _, pm := range ep.PortMapping {
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			zaplog.L().Error("port mapping format error", zap.Any("prot mapping", pm))
			continue
		}
		// 在 iptables 的 PREROUTING 中添加 DNAT 规则
		// 将宿主机的端口请求转发到容器的地址和端口上
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING ! -i %s -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			ep.Network.Name, portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		output, err := cmd.Output()
		if err != nil {
			zaplog.L().Error("", zap.String("iptables output", string(output)), zap.Error(err))
			continue
		}
		// 因为访问本地服务不会走 PREROUTING、INPUT 链，直接走的是 OUTPUT 链
		// 因此要在 OUTPUT 链也增加 DNAT。
		iptablesCmd = fmt.Sprintf("-t nat -A OUTPUT -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd = exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		output, err = cmd.Output()
		if err != nil {
			zaplog.L().Error("", zap.String("iptables output", string(output)), zap.Error(err))
			continue
		}
	}
	return nil
}

func Connect(networkName string, cinfo *container.ContainerInfo) error {
	// 从 networks 字典中取到容器连接的网络信息，networks 字典中保存了当前已经创建的网络
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("no such network: %s", networkName)
	}
	// 通过调用 IPAM 从网络的网段中获取可用的 IP 作为容器 IP 地址
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}

	// 创建网络端点
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.Id, networkName),
		IPAddress:   ip,
		Network:     network,
		PortMapping: cinfo.PortMapping,
	}

	// 调用网络驱动挂载和配置网络端点
	if err := drivers[network.Driver].Connect(network, ep); err != nil {
		return err
	}
	// 到容器的 namespace 配置容器网络设备 ip 地址
	if err := configEndpointIpAddressAndRoute(ep, cinfo); err != nil {
		return err
	}
	// 配置端口映射信息
	return configPortMapping(ep, cinfo)
}
