package main

import (
	"fmt"
	"net"
	"testing"
)

func TestNet(t *testing.T) {
	ip, _, _ := net.ParseCIDR("192.168.1.3/24")
	fmt.Println(ip.String())
}
