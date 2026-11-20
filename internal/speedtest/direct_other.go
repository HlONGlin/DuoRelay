//go:build !linux && !windows && !darwin

package speedtest

import (
	"errors"
	"net"
)

func bindInterface(uintptr, *net.Interface, bool, uint32) error {
	return errors.New("binding a network interface is unsupported on this platform")
}
