package kernel

import (
	"fmt"
	"sync"
	"time"
)

// RestartPolicy defines the restart behavior for a kernel process
type RestartPolicy struct {
	mu sync.Mutex

	enabled          bool
	maxRetries       int
	consecutiveFails int
	backoffBase      time.Duration
	maxBackoff       time.Duration

	restartHistory []RestartEvent
}

// RestartEvent records a restart attempt
type RestartEvent struct {
	Timestamp time.Time
	Reason    string
	ExitCode  int
	Success   bool
}

// NewRestartPolicy creates a new restart policy with default settings
func NewRestartPolicy() *RestartPolicy {
	return &RestartPolicy{
		enabled:        true,
		maxRetries:     5,
		backoffBase:    1 * time.Second,
		maxBackoff:     16 * time.Second,
		restartHistory: make([]RestartEvent, 0),
	}
}

// Enable enables automatic restart
func (rp *RestartPolicy) Enable() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.enabled = true
	rp.consecutiveFails = 0
}

// Disable disables automatic restart
func (rp *RestartPolicy) Disable() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.enabled = false
}

// IsEnabled returns whether automatic restart is enabled
func (rp *RestartPolicy) IsEnabled() bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.enabled
}

// ShouldRestart determines if a restart should be attempted
func (rp *RestartPolicy) ShouldRestart(exitCode int, reason string) (bool, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if !rp.enabled {
		return false, fmt.Errorf("restart disabled")
	}

	if rp.consecutiveFails >= rp.maxRetries {
		return false, fmt.Errorf("max consecutive failures (%d) reached", rp.maxRetries)
	}

	return true, nil
}

// GetBackoffDuration returns the wait time before next restart attempt
// Uses exponential backoff: 1s, 2s, 4s, 8s, 16s (capped at maxBackoff)
func (rp *RestartPolicy) GetBackoffDuration() time.Duration {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.consecutiveFails == 0 {
		return rp.backoffBase
	}

	// Exponential backoff: base * 2^attempts, capped before multiplication.
	duration := rp.backoffBase
	for i := 0; i < rp.consecutiveFails && duration < rp.maxBackoff; i++ {
		if duration > rp.maxBackoff/2 {
			return rp.maxBackoff
		}
		duration *= 2
	}

	if duration > rp.maxBackoff {
		duration = rp.maxBackoff
	}

	return duration
}

// RecordRestart records a restart attempt
func (rp *RestartPolicy) RecordRestart(exitCode int, reason string, success bool) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	event := RestartEvent{
		Timestamp: time.Now(),
		Reason:    reason,
		ExitCode:  exitCode,
		Success:   success,
	}

	rp.restartHistory = append(rp.restartHistory, event)

	// Keep only the last 100 restart events
	if len(rp.restartHistory) > 100 {
		rp.restartHistory = rp.restartHistory[len(rp.restartHistory)-100:]
	}

	// Starting a process does not prove recovery: it may crash immediately.
	// The manager resets this budget only after stable uptime.
	rp.consecutiveFails++
}

// GetRestartHistory returns the restart history
func (rp *RestartPolicy) GetRestartHistory() []RestartEvent {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Return a copy to avoid concurrent modification
	history := make([]RestartEvent, len(rp.restartHistory))
	copy(history, rp.restartHistory)
	return history
}

// GetConsecutiveFailures returns the number of consecutive failures
func (rp *RestartPolicy) GetConsecutiveFailures() int {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.consecutiveFails
}

// Reset resets the restart policy state
func (rp *RestartPolicy) Reset() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.consecutiveFails = 0
}

// monitorProcess owns one managed lifetime. Each attempt's waiter drains logs
// before publishing its exit, and only this loop decides whether to restart.
func (m *Manager) monitorProcess(p *Process, a *attempt) {
	defer func() {
		p.Cancel()
		close(p.done)
	}()
	for {
		<-a.done
		code := a.cmd.ProcessState.ExitCode()
		reason := "process exited"
		if a.err != nil {
			reason = a.err.Error()
		}
		m.mu.Lock()
		p.Running, p.Healthy = false, false
		p.ExitCode, p.ExitReason = code, reason
		m.mu.Unlock()
		if p.Ctx.Err() != nil {
			return
		}
		if time.Since(a.started) >= m.stableUptime {
			p.RestartPolicy.Reset()
		}
		for {
			if ok, err := p.RestartPolicy.ShouldRestart(code, reason); !ok {
				m.mu.Lock()
				p.ExitReason = fmt.Sprintf("%s; %v", reason, err)
				m.mu.Unlock()
				return
			}
			timer := time.NewTimer(p.RestartPolicy.GetBackoffDuration())
			select {
			case <-p.Ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			m.mu.Lock()
			if p.Ctx.Err() != nil {
				m.mu.Unlock()
				return
			}
			next, err := m.launch(p)
			m.mu.Unlock()
			p.RestartPolicy.RecordRestart(code, reason, err == nil)
			if err != nil {
				code, reason = -1, err.Error()
				m.mu.Lock()
				p.ExitCode, p.ExitReason = code, reason
				m.mu.Unlock()
				continue
			}
			a = next
			if err := m.waitHealthy(p, a); err != nil && p.Ctx.Err() == nil {
				m.mu.Lock()
				p.HealthError = err.Error()
				m.mu.Unlock()
				// The attempt's sole waiter still owns Wait and log draining.
				_ = a.cmd.Process.Kill()
			}
			break
		}
	}
}
