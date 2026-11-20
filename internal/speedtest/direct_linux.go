package speedtest

import (
	"net"

	"golang.org/x/sys/unix"
)

func bindInterface(fd uintptr, iface *net.Interface, _ bool, mark uint32) error {
	if mark != 0 {
		// The auto-redirect output chain sees even interface-bound sockets.
		if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, int(mark)); err != nil {
			return err
		}
	}
	return unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, iface.Index)
}
