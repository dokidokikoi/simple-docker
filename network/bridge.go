package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

type BridgeNetworkDriver struct {
}

func (BridgeNetworkDriver) Name() string {
	return "bridge"
}

func (d *BridgeNetworkDriver) initBridge(n *Network) error {
	// 创建 bridge 虚拟设备
	bridgeName := n.Name
	if err := createBridgeInterface(bridgeName); err != nil {
		return fmt.Errorf("error add bridge: %ss, error: %w", bridgeName, err)
	}

	// 设置 bridge 设置的地址和路由
	gatewayIP := *n.IpRange
	gatewayIP.IP = n.IpRange.IP
	if err := setInterfaceIp(bridgeName, gatewayIP.String()); err != nil {
		return fmt.Errorf("error assigning address: %s on bridge: %s with an error of: %w", &gatewayIP, bridgeName, err)
	}

	// 启动 bridge 设备
	if err := setInterfaceUp(bridgeName); err != nil {
		return fmt.Errorf("error set bridge up: %s, error %w", bridgeName, err)
	}

	// 设置 iptables 的 SNAT 规则
	if err := setupIPTables(bridgeName, n.IpRange); err != nil {
		return fmt.Errorf("error setting iptables for %s: %w", bridgeName, err)
	}

	return nil
}

func (d *BridgeNetworkDriver) Create(subnet, name string) (*Network, error) {
	ip, ipRange, _ := net.ParseCIDR(subnet)
	ipRange.IP = ip
	n := &Network{
		Name:    name,
		IpRange: ipRange,
		Driver:  d.Name(),
	}
	// 配置 linux bridge
	err := d.initBridge(n)
	if err != nil {
		zaplog.L().Error("error init bridge", zap.Error(err))
	}

	return n, err
}

func (d *BridgeNetworkDriver) Delete(network *Network) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	return netlink.LinkDel(br)
}

// 连接容器到之前创建的网络
func (d *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	// 创建 veth 接口的配置
	la := netlink.NewLinkAttrs()
	la.Name = endpoint.ID[:5]
	// 通过设置 veth 接口的 master 属性，设置这个 veth 的一端挂载到网络对应的 linux bridge 上
	la.MasterIndex = br.Attrs().Index

	// 创建 veth 对象，通过 PeerName 配置 veth 另外一端的接口名
	// 配置 veth 另外一端的名字 cif-<endpoint ID 的前5位>
	endpoint.Device = netlink.Veth{
		LinkAttrs: la,
		PeerName:  "cif-" + endpoint.ID[:5],
	}

	// 创建 veth 接口
	// 因为上面指定了 link 的 MasterIndex 是网络对应的 linux bridge
	// 所以 veth 的一端就已经挂载到了网络对应的 linux bridge 上
	if err = netlink.LinkAdd(&endpoint.Device); err != nil {
		return fmt.Errorf("error add endpoint device: %w", err)
	}
	// 设置 veth 启动
	if err = netlink.LinkSetUp(&endpoint.Device); err != nil {
		return fmt.Errorf("error add endpoint device: %w", err)
	}
	return nil
}

func (d *BridgeNetworkDriver) DisConnect(network *Network, endpoint *Endpoint) error {
	return nil
}

func createBridgeInterface(bridgeName string) error {
	// 先检查是否已经创建了同名的 bridge 设备
	_, err := net.InterfaceByName(bridgeName)
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// 初始化一个 netlink 的 link 基础对象
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName

	// 使用刚才创建 link 的属性创建 netlink 的 bridge 对象
	br := &netlink.Bridge{LinkAttrs: la}
	// 创建 bridge 的 虚拟网络设备，相当于 ip link add xxx
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("bridge creation failed for bridge %s: %w", bridgeName, err)
	}
	return nil
}

func setInterfaceUp(interfaceName string) error {
	iface, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("error retrieving a link named [ %s ]: %w", iface.Attrs().Name, err)
	}

	// 设置接口状态为 “UP” 状态
	// 相当于 ip link set xxx up
	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("error enabling interface for %s: %w", interfaceName, err)
	}
	return nil
}

func setInterfaceIp(name, rawIP string) error {
	retries := 2
	var iface netlink.Link
	var err error

	for i := 0; i < retries; i++ {
		iface, err = netlink.LinkByName(name)
		if err == nil {
			break
		}
		zaplog.L().Sugar().Debugf("error retrieving new bridge netlink link [ %s ]... retrying", name)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("abandoning retrieving the new bridge link from netlink, Run [ ip link ] to troubleshoot the error: %w", err)
	}

	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}
	// netlink.AddrAdd 相当于 ip addr add xxx
	// 如果配置了地址所在网段的信息，例如 192.168.0.0/24
	// 会配置路由表 192.168.0.0/24 转发到这个网络接口上
	addr := &netlink.Addr{IPNet: ipNet}
	return netlink.AddrAdd(iface, addr)
}

// 设置 iptables 对应 bridge 的 MASQUERADE 规则
func setupIPTables(bridgeName string, subnet *net.IPNet) error {
	// 允许 IP forwarding
	forwardCmd := "net.ipv4.conf.all.forwarding=1"
	cmd := exec.Command("sysctl", strings.Split(forwardCmd, " ")...)
	output, err := cmd.Output()
	if err != nil {
		zaplog.L().Error("", zap.String("sysctl output", string(output)), zap.Error(err))
		return err
	}
	// iptables -t nat -A POSTROUTING -s <subnet> ! -o <bridgeName> -j MASQUERADE
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd = exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	output, err = cmd.Output()
	if err != nil {
		zaplog.L().Error("", zap.String("iptables output", string(output)), zap.Error(err))
	}
	return err
}
