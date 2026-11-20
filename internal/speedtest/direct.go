package speedtest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
)

var ErrDirectRoute = errors.New("direct route unavailable for TCP test")

// DirectRoute holds the default interfaces from before the managed TUN starts.
// An absent interface is kept as such so a probe cannot fall back to the TUN.
type DirectRoute struct {
	ipv4, ipv6 *net.Interface
	dns        string
	mark       uint32
}

func CaptureDirectRoute(dns string, mark uint32) *DirectRoute {
	return &DirectRoute{
		ipv4: routeInterface("udp4", "192.0.2.1:53"),
		ipv6: routeInterface("udp6", "[2001:db8::1]:53"),
		dns:  dns,
		mark: mark,
	}
}

func routeInterface(network, address string) *net.Interface {
	// Connecting a UDP socket selects a route without sending a datagram.
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil
	}
	ip := conn.LocalAddr().(*net.UDPAddr).IP
	_ = conn.Close()
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			candidate, _, _ := net.ParseCIDR(address.String())
			if ip.Equal(candidate) {
				return &iface
			}
		}
	}
	return nil
}

func (r *DirectRoute) TCPDelay(ctx context.Context, address string) (int64, error) {
	dialer := &net.Dialer{ControlContext: r.bind}
	dnsDialer := &net.Dialer{ControlContext: r.bind}
	dialer.Resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			if net.ParseIP(r.dns) == nil {
				return nil, ErrDirectRoute
			}
			return dnsDialer.DialContext(ctx, network, net.JoinHostPort(r.dns, "53"))
		},
	}
	return tcpDelay(ctx, dialer, address)
}

func (r *DirectRoute) bind(_ context.Context, network, address string, raw syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	iface := r.ipv4
	ipv6 := usesIPv6(network, host)
	if ipv6 {
		iface = r.ipv6
	}
	if iface == nil {
		return fmt.Errorf("%w: no interface for %s", ErrDirectRoute, network)
	}
	var bindErr error
	if err := raw.Control(func(fd uintptr) {
		bindErr = bindInterface(fd, iface, ipv6, r.mark)
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrDirectRoute, err)
	}
	if bindErr != nil {
		return fmt.Errorf("%w: bind %s: %v", ErrDirectRoute, iface.Name, bindErr)
	}
	return nil
}

func usesIPv6(network, host string) bool {
	if network == "tcp6" || network == "udp6" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() == nil
}
