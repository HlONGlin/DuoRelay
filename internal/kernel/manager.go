package kernel

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
)

// Manager manages kernel processes (sing-box and mihomo)
type Manager struct {
	mu sync.RWMutex

	singboxProc *Process
	mihomoProc  *Process

	mode RunMode
}

// RunMode defines how kernels run
type RunMode string

const (
	// ModeSingleSwitch - only one kernel runs at a time
	ModeSingleSwitch RunMode = "single_switch"

	// ModeCoexist - both kernels can run together (main + slave)
	ModeCoexist RunMode = "coexist"
)

// Process represents a kernel process
type Process struct {
	Name    string
	Binary  string
	Config  string
	Cmd     *exec.Cmd
	Ctx     context.Context
	Cancel  context.CancelFunc
	Running bool
}

// NewManager creates a new kernel manager
func NewManager() *Manager {
	return &Manager{
		mode: ModeSingleSwitch, // Start with single switch mode
	}
}

// SetMode sets the run mode
func (m *Manager) SetMode(mode RunMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mode == mode {
		return nil
	}

	// TODO: Implement mode switching logic
	m.mode = mode
	return nil
}

// StartSingBox starts the sing-box kernel
func (m *Manager) StartSingBox(configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mode == ModeSingleSwitch && m.mihomoProc != nil && m.mihomoProc.Running {
		// Stop mihomo first in single switch mode
		if err := m.stopMihomo(); err != nil {
			return fmt.Errorf("failed to stop mihomo: %w", err)
		}
	}

	return m.startSingBox(configPath)
}

// StartMihomo starts the mihomo kernel
func (m *Manager) StartMihomo(configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mode == ModeSingleSwitch && m.singboxProc != nil && m.singboxProc.Running {
		// Stop sing-box first in single switch mode
		if err := m.stopSingBox(); err != nil {
			return fmt.Errorf("failed to stop sing-box: %w", err)
		}
	}

	return m.startMihomo(configPath)
}

// StopAll stops all running kernels
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	if err := m.stopSingBox(); err != nil {
		errs = append(errs, err)
	}
	if err := m.stopMihomo(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to stop kernels: %v", errs)
	}
	return nil
}

func (m *Manager) startSingBox(configPath string) error {
	// TODO: Implement in P0-07
	return fmt.Errorf("not implemented")
}

func (m *Manager) startMihomo(configPath string) error {
	// TODO: Implement in P0-07
	return fmt.Errorf("not implemented")
}

func (m *Manager) stopSingBox() error {
	if m.singboxProc == nil || !m.singboxProc.Running {
		return nil
	}
	// TODO: Implement graceful shutdown
	return fmt.Errorf("not implemented")
}

func (m *Manager) stopMihomo() error {
	if m.mihomoProc == nil || !m.mihomoProc.Running {
		return nil
	}
	// TODO: Implement graceful shutdown
	return fmt.Errorf("not implemented")
}

// Status returns the current status of all kernels
func (m *Manager) Status() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		"mode": m.mode,
		"singbox": map[string]any{
			"running": m.singboxProc != nil && m.singboxProc.Running,
		},
		"mihomo": map[string]any{
			"running": m.mihomoProc != nil && m.mihomoProc.Running,
		},
	}
}
