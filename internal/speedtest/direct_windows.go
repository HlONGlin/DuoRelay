package speedtest

import (
	"encoding/binary"
	"net"

	"golang.org/x/sys/windows"
)

func bindInterface(fd uintptr, iface *net.Interface, ipv6 bool, _ uint32) error {
	const unicastInterface = 31 // IP_UNICAST_IF and IPV6_UNICAST_IF
	if ipv6 {
		return windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, unicastInterface, iface.Index)
	}
	// Winsock expects the IPv4 interface index in network byte order.
	var index [4]byte
	binary.BigEndian.PutUint32(index[:], uint32(iface.Index))
	return windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, unicastInterface, int(binary.NativeEndian.Uint32(index[:])))
}
