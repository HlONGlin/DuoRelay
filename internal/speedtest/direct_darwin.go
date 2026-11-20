package speedtest

import (
	"net"

	"golang.org/x/sys/unix"
)

func bindInterface(fd uintptr, iface *net.Interface, ipv6 bool, _ uint32) error {
	if ipv6 {
		return unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, iface.Index)
	}
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, iface.Index)
}
