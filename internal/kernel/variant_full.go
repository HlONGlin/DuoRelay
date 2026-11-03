//go:build !lite

package kernel

import (
	"context"
	"fmt"
	"path/filepath"
)

type kernelProcesses struct{ singboxProc *Process }

func PrepareData(context.Context, string) error { return nil }
func (m *Manager) otherProcess(name string) *Process {
	if name == "sing-box" {
		return m.mihomoProc
	}
	return m.singboxProc
}
func (m *Manager) setProcess(name string, p *Process) {
	if name == "sing-box" {
		m.singboxProc = p
	} else {
		m.mihomoProc = p
	}
}
func (m *Manager) statusSnapshot(status func(*Process) map[string]any) map[string]any {
	return map[string]any{"mode": m.mode, "singbox": status(m.singboxProc), "mihomo": status(m.mihomoProc)}
}

// SetMode retains sing-box when switching from coexistence to single mode.
func (m *Manager) SetMode(mode RunMode) error {
	if mode != ModeSingleSwitch && mode != ModeCoexist {
		return fmt.Errorf("unknown run mode %q", mode)
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.RLock()
	both := m.active(m.singboxProc) && m.active(m.mihomoProc)
	other := m.mihomoProc
	m.mu.RUnlock()
	if mode == ModeSingleSwitch && both {
		if err := m.stopProcess(other); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.mode = mode
	m.mu.Unlock()
	return nil
}

func (m *Manager) StartSingBox(configPath string) error { return m.start("sing-box", configPath) }
func (m *Manager) StartMihomo(configPath string) error  { return m.start("mihomo", configPath) }
func (m *Manager) StopSingBox() error                   { return m.stop("sing-box") }
func (m *Manager) StopMihomo() error                    { return m.stop("mihomo") }

func (m *Manager) process(name string) *Process {
	if name == "sing-box" {
		return m.singboxProc
	}
	return m.mihomoProc
}

func kernelArgs(name, config string, check bool) []string {
	if name == "sing-box" {
		verb := "run"
		if check {
			verb = "check"
		}
		return []string{verb, "-c", config}
	}
	args := []string{"-d", filepath.Dir(config), "-f", config}
	if check {
		args = append(args, "-t")
	}
	return args
}
