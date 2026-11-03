package kernel

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/holi/duorelay/internal/variant"
	"os"
	"path/filepath"
)

const persistentLogCapacity = 10000

// SetLogDirectory enables JSONL log history for both managed kernels. The
// in-memory ring remains the fast path while this history survives panel restarts.
func (m *Manager) SetLogDirectory(directory string) error {
	if directory == "" {
		return nil
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	history := map[string]*RingBuffer{}
	for _, name := range variant.Kernels() {
		buffer := NewRingBuffer(persistentLogCapacity)
		if err := loadLogFile(filepath.Join(directory, logFileName(name)), buffer); err != nil {
			return fmt.Errorf("load %s log history: %w", name, err)
		}
		history[name] = buffer
	}
	m.mu.Lock()
	m.logDir = directory
	m.logHistory = history
	m.mu.Unlock()
	return nil
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
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		buffer.Append(entry)
	}
	return scanner.Err()
}

func (m *Manager) recordLog(name string, entry LogEntry) {
	m.mu.RLock()
	history := m.logHistory[name]
	directory := m.logDir
	m.mu.RUnlock()
	if history != nil {
		history.Append(entry)
	}
	if directory != "" {
		m.logFileMu.Lock()
		file, err := os.OpenFile(filepath.Join(directory, logFileName(name)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			_ = json.NewEncoder(file).Encode(entry)
			_ = file.Close()
		}
		m.logFileMu.Unlock()
	}
	m.publishLog(name, entry)
}
