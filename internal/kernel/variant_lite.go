//go:build lite

package kernel

import (
	"fmt"
	"path/filepath"
)

type kernelProcesses struct{}

func (m *Manager) SetMode(mode RunMode) error {
	if mode != ModeSingleSwitch {
		return fmt.Errorf("精简版不支持双内核运行模式 %q", mode)
	}
	return nil
}
func (m *Manager) StartSingBox(string) error     { return fmt.Errorf("精简版不支持 sing-box") }
func (m *Manager) StopSingBox() error            { return fmt.Errorf("精简版不支持 sing-box") }
func (m *Manager) StartMihomo(path string) error { return m.start("mihomo", path) }
func (m *Manager) StopMihomo() error             { return m.stop("mihomo") }
func (m *Manager) process(name string) *Process {
	if name == "mihomo" {
		return m.mihomoProc
	}
	return nil
}
func (m *Manager) otherProcess(string) *Process    { return nil }
func (m *Manager) setProcess(_ string, p *Process) { m.mihomoProc = p }
func (m *Manager) statusSnapshot(status func(*Process) map[string]any) map[string]any {
	return map[string]any{"mode": m.mode, "mihomo": status(m.mihomoProc)}
}
func kernelArgs(_ string, config string, check bool) []string {
	args := []string{"-d", filepath.Dir(config), "-f", config}
	if check {
		args = append(args, "-t")
	}
	return args
}
