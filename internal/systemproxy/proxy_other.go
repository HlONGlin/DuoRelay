//go:build !windows && !linux

package systemproxy

import "errors"

var ErrUnsupported = errors.New("system proxy integration is supported on Windows and GNOME Linux only")

func Supported() bool            { return false }
func Capture() (Snapshot, error) { return Snapshot{}, ErrUnsupported }
func Set(string, int) error      { return ErrUnsupported }
func Restore(Snapshot) error     { return ErrUnsupported }
