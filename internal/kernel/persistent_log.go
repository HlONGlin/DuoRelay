package kernel

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/holi/duorelay/internal/variant"
)

const persistentLogCapacity = 10000
const persistentLogMaxBytes = 16 * 1024 * 1024

type LogPolicy struct {
	RetentionDays int       `json:"retention_days"`
	CleanupHours  int       `json:"cleanup_hours"`
	LastCleanup   time.Time `json:"last_cleanup,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
}

func DefaultLogPolicy() LogPolicy { return LogPolicy{RetentionDays: 7, CleanupHours: 24} }

func (m *Manager) LogPolicy() LogPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.logPolicy
}

// ConfigureLogPolicy changes the next cleanup deadline without creating a new
// worker. Zero cleanup_hours disables scheduled age cleanup; disk caps remain.
func (m *Manager) ConfigureLogPolicy(policy LogPolicy) error {
	if policy.RetentionDays < 1 || policy.RetentionDays > 365 || policy.CleanupHours < 0 || policy.CleanupHours > 720 {
		return fmt.Errorf("retention_days must be 1..365 and cleanup_hours must be 0..720")
	}
	m.mu.Lock()
	policy.LastCleanup, policy.LastError = m.logPolicy.LastCleanup, m.logPolicy.LastError
	m.logPolicy = policy
	if m.logCleanupWake != nil {
		select {
		case m.logCleanupWake <- struct{}{}:
		default:
		}
	}
	m.mu.Unlock()
	return nil
}

// SetLogDirectory enables bounded JSONL history and one owned cleanup worker.
func (m *Manager) SetLogDirectory(directory string) error {
	m.CloseLogs()
	if directory == "" {
		return nil
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	m.logFileMu.Lock()
	defer m.logFileMu.Unlock()
	policy := m.LogPolicy()
	if policy.RetentionDays == 0 {
		policy = DefaultLogPolicy()
	}
	history := map[string]*RingBuffer{}
	for _, name := range variant.Kernels() {
		path := filepath.Join(directory, logFileName(name))
		if err := pruneLogFile(path, time.Now().UTC().AddDate(0, 0, -policy.RetentionDays)); err != nil {
			return fmt.Errorf("prune %s log history: %w", name, err)
		}
		buffer := NewRingBuffer(persistentLogCapacity)
		if err := loadLogFile(path, buffer); err != nil {
			return fmt.Errorf("load %s log history: %w", name, err)
		}
		history[name] = buffer
	}
	m.mu.Lock()
	m.logDir, m.logHistory, m.logPolicy = directory, history, policy
	m.logCleanupStop, m.logCleanupDone, m.logCleanupWake = make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
	stop, done, wake := m.logCleanupStop, m.logCleanupDone, m.logCleanupWake
	m.mu.Unlock()
	go m.periodicLogCleanup(stop, done, wake)
	return nil
}

func (m *Manager) CloseLogs() {
	m.mu.Lock()
	stop, done := m.logCleanupStop, m.logCleanupDone
	if stop != nil {
		close(stop)
		m.logCleanupStop = nil
	}
	m.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (m *Manager) periodicLogCleanup(stop <-chan struct{}, done chan<- struct{}, wake <-chan struct{}) {
	defer close(done)
	for {
		policy := m.LogPolicy()
		var deadline <-chan time.Time
		var timer *time.Timer
		if policy.CleanupHours > 0 {
			timer = time.NewTimer(time.Duration(policy.CleanupHours) * time.Hour)
			deadline = timer.C
		}
		select {
		case <-deadline:
			_ = m.PruneLogs(time.Now().UTC())
		case <-wake:
		case <-stop:
			if timer != nil {
				timer.Stop()
			}
			return
		}
		if timer != nil {
			timer.Stop()
		}
	}
}

func (m *Manager) PruneLogs(now time.Time) error {
	m.logFileMu.Lock()
	defer m.logFileMu.Unlock()
	m.mu.RLock()
	directory, policy := m.logDir, m.logPolicy
	m.mu.RUnlock()
	if directory == "" {
		return nil
	}
	cutoff := now.AddDate(0, 0, -policy.RetentionDays)
	var result error
	for _, name := range variant.Kernels() {
		result = errors.Join(result, pruneLogFile(filepath.Join(directory, logFileName(name)), cutoff))
	}
	m.mu.Lock()
	for _, buffer := range m.logHistory {
		buffer.PruneBefore(cutoff)
	}
	for _, name := range variant.Kernels() {
		if process := m.process(name); process != nil {
			process.LogBuffer.PruneBefore(cutoff)
		}
	}
	m.logPolicy.LastCleanup = now
	m.logPolicy.LastError = ""
	if result != nil {
		m.logPolicy.LastError = result.Error()
	}
	m.mu.Unlock()
	return result
}

func pruneLogFile(path string, cutoff time.Time) error {
	buffer := NewRingBuffer(persistentLogCapacity)
	if err := loadLogFile(path, buffer); err != nil {
		return err
	}
	entries := buffer.GetAll()
	kept := make([]LogEntry, 0, len(entries))
	bytes := 0
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.Timestamp.Before(cutoff) {
			continue
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if bytes+len(data)+1 > persistentLogMaxBytes/2 {
			break
		}
		bytes += len(data) + 1
		kept = append(kept, entry)
	}
	out, err := os.CreateTemp(filepath.Dir(path), ".kernel-log-*")
	if err != nil {
		return err
	}
	tmp := out.Name()
	defer os.Remove(tmp)
	encoder := json.NewEncoder(out)
	for i := len(kept) - 1; i >= 0; i-- {
		if err := encoder.Encode(kept[i]); err != nil {
			out.Close()
			return err
		}
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func logFileName(name string) string {
	if name == "sing-box" {
		return "sing-box.jsonl"
	}
	return "mihomo.jsonl"
}

func loadLogFile(path string, buffer *RingBuffer) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var entry LogEntry
		if json.Unmarshal(scanner.Bytes(), &entry) == nil {
			buffer.Append(entry)
		}
	}
	return scanner.Err()
}

func (m *Manager) ClearLogs(name string) error {
	if !variant.SupportsKernel(name) {
		return fmt.Errorf("unsupported kernel")
	}
	m.logFileMu.Lock()
	defer m.logFileMu.Unlock()
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.logDir != "" {
		if err := os.WriteFile(filepath.Join(m.logDir, logFileName(name)), nil, 0600); err != nil {
			return err
		}
	}
	if buffer := m.logHistory[name]; buffer != nil {
		buffer.Clear()
	}
	if process := m.process(name); process != nil {
		process.LogBuffer.Clear()
	}
	return nil
}

func (m *Manager) recordLog(name string, entry LogEntry) {
	m.logFileMu.Lock()
	m.mu.RLock()
	history, directory := m.logHistory[name], m.logDir
	m.mu.RUnlock()
	if history != nil {
		history.Append(entry)
	}
	var writeErr error
	if directory != "" {
		path := filepath.Join(directory, logFileName(name))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		writeErr = err
		if err == nil {
			writeErr = json.NewEncoder(file).Encode(entry)
			info, statErr := file.Stat()
			writeErr = errors.Join(writeErr, statErr, file.Close())
			if writeErr == nil && info.Size() > persistentLogMaxBytes {
				writeErr = pruneLogFile(path, time.Time{})
			}
		}
	}
	if writeErr != nil {
		m.mu.Lock()
		m.logPolicy.LastError = writeErr.Error()
		m.mu.Unlock()
	}
	m.logFileMu.Unlock()
	m.publishLog(name, entry)
}
