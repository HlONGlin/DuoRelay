//go:build linux

package systemproxy

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

const gnomeProxySchema = "org.gnome.system.proxy"

var (
	ErrUnsupported = errors.New("Linux system proxy requires gsettings and a GNOME D-Bus session")
	lookPath       = exec.LookPath
	runGSettings   = func(args ...string) ([]byte, error) {
		return exec.Command("gsettings", args...).CombinedOutput()
	}
)

var gnomeProxyKeys = []string{
	"mode",
	"autoconfig-url",
	"use-same-proxy",
	"ignore-hosts",
	"http:host",
	"http:port",
	"https:host",
	"https:port",
	"ftp:host",
	"ftp:port",
	"socks:host",
	"socks:port",
}

func Supported() bool {
	if _, err := lookPath("gsettings"); err != nil {
		return false
	}
	_, err := runGSettings("list-schemas")
	if err != nil {
		return false
	}
	value, err := runGSettings("list-recursively", gnomeProxySchema)
	if err != nil || !strings.Contains(string(value), gnomeProxySchema) {
		return false
	}
	_, err = runGSettings("get", gnomeProxySchema, "mode")
	return err == nil
}

func Capture() (Snapshot, error) {
	if !Supported() {
		return Snapshot{}, ErrUnsupported
	}
	values := make(map[string]string, len(gnomeProxyKeys))
	for _, key := range gnomeProxyKeys {
		value, err := getGSetting(key)
		if err != nil {
			return Snapshot{}, fmt.Errorf("read GNOME proxy %s: %w", key, err)
		}
		values[key] = value
	}
	return Snapshot{LinuxBackend: "gnome", LinuxValues: values}, nil
}

func Set(host string, port int) error {
	if host == "" || net.ParseIP(host) == nil {
		return fmt.Errorf("invalid Linux proxy host")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid Linux proxy port")
	}
	if !Supported() {
		return ErrUnsupported
	}
	for _, item := range []struct {
		key   string
		value string
	}{
		{"mode", "'manual'"},
		{"autoconfig-url", "''"},
		{"use-same-proxy", "true"},
		{"http:host", quoteGVariantString(host)},
		{"http:port", strconv.Itoa(port)},
		{"https:host", quoteGVariantString(host)},
		{"https:port", strconv.Itoa(port)},
		{"ftp:host", quoteGVariantString(host)},
		{"ftp:port", strconv.Itoa(port)},
		{"socks:host", quoteGVariantString(host)},
		{"socks:port", strconv.Itoa(port)},
	} {
		if err := setGSetting(item.key, item.value); err != nil {
			return fmt.Errorf("set GNOME proxy %s: %w", item.key, err)
		}
	}
	return nil
}

func Restore(snapshot Snapshot) error {
	if snapshot.LinuxBackend != "gnome" || len(snapshot.LinuxValues) == 0 {
		return ErrUnsupported
	}
	if !Supported() {
		return ErrUnsupported
	}
	for _, key := range gnomeProxyKeys {
		value, ok := snapshot.LinuxValues[key]
		if !ok {
			continue
		}
		if err := setGSetting(key, value); err != nil {
			return fmt.Errorf("restore GNOME proxy %s: %w", key, err)
		}
	}
	return nil
}

func getGSetting(key string) (string, error) {
	schema, setting := splitGSetting(key)
	args := []string{"get", schema, setting}
	output, err := runGSettings(args...)
	if err != nil {
		return "", commandError(output, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func setGSetting(key, value string) error {
	schema, setting := splitGSetting(key)
	args := []string{"set", schema, setting, value}
	output, err := runGSettings(args...)
	if err != nil {
		return commandError(output, err)
	}
	return nil
}

func splitGSetting(key string) (string, string) {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) == 2 {
		return gnomeProxySchema + "." + parts[0], parts[1]
	}
	return gnomeProxySchema, key
}

func quoteGVariantString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func commandError(output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}
