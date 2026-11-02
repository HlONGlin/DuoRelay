//go:build windows

package systemproxy

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const internetSettingsPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func Supported() bool { return true }

func Capture() (Snapshot, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.QUERY_VALUE)
	if err != nil {
		return Snapshot{}, fmt.Errorf("open Windows Internet Settings: %w", err)
	}
	defer key.Close()
	var snapshot Snapshot
	if value, _, err := key.GetIntegerValue("ProxyEnable"); err == nil {
		snapshot.ProxyEnablePresent, snapshot.ProxyEnable = true, uint32(value)
	} else if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return Snapshot{}, fmt.Errorf("read ProxyEnable: %w", err)
	}
	if value, _, err := key.GetStringValue("ProxyServer"); err == nil {
		snapshot.ProxyServerPresent, snapshot.ProxyServer = true, value
	} else if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return Snapshot{}, fmt.Errorf("read ProxyServer: %w", err)
	}
	if value, _, err := key.GetStringValue("ProxyOverride"); err == nil {
		snapshot.ProxyOverridePresent, snapshot.ProxyOverride = true, value
	} else if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return Snapshot{}, fmt.Errorf("read ProxyOverride: %w", err)
	}
	return snapshot, nil
}

func Set(host string, port int) error {
	if host == "" || net.ParseIP(host) == nil {
		return fmt.Errorf("invalid Windows proxy host")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid Windows proxy port")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Windows Internet Settings for write: %w", err)
	}
	defer key.Close()
	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("enable Windows proxy: %w", err)
	}
	if err := key.SetStringValue("ProxyServer", net.JoinHostPort(host, strconv.Itoa(port))); err != nil {
		return fmt.Errorf("set Windows proxy: %w", err)
	}
	return notify()
}

func Restore(snapshot Snapshot) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Windows Internet Settings for restore: %w", err)
	}
	defer key.Close()
	if snapshot.ProxyEnablePresent {
		if err := key.SetDWordValue("ProxyEnable", snapshot.ProxyEnable); err != nil {
			return fmt.Errorf("restore ProxyEnable: %w", err)
		}
	} else if err := key.DeleteValue("ProxyEnable"); err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return fmt.Errorf("remove ProxyEnable: %w", err)
	}
	if snapshot.ProxyServerPresent {
		if err := key.SetStringValue("ProxyServer", snapshot.ProxyServer); err != nil {
			return fmt.Errorf("restore ProxyServer: %w", err)
		}
	} else if err := key.DeleteValue("ProxyServer"); err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return fmt.Errorf("remove ProxyServer: %w", err)
	}
	if snapshot.ProxyOverridePresent {
		if err := key.SetStringValue("ProxyOverride", snapshot.ProxyOverride); err != nil {
			return fmt.Errorf("restore ProxyOverride: %w", err)
		}
	} else if err := key.DeleteValue("ProxyOverride"); err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return fmt.Errorf("remove ProxyOverride: %w", err)
	}
	return notify()
}

func notify() error {
	proc := windows.NewLazySystemDLL("wininet.dll").NewProc("InternetSetOptionW")
	for _, option := range []uintptr{39, 37} { // SETTINGS_CHANGED, REFRESH
		if result, _, err := proc.Call(0, option, 0, 0); result == 0 && err != windows.ERROR_SUCCESS {
			return fmt.Errorf("refresh Windows proxy settings: %w", err)
		}
	}
	return nil
}
