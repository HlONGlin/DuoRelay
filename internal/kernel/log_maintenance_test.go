package kernel

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/variant"
)

func TestLogFloodCollapsesWithoutHidingDifferentErrors(t *testing.T) {
	if LogMessageKey("\x1b[31mERROR\x1b[0m[0000] [123 1ms] remote port 443 failed") != LogMessageKey("ERROR[0123] [987 5.5s] remote port 443 failed") {
		t.Fatal("sing-box elapsed uptime or connection metadata prevented repeat detection")
	}
	buffer := NewRingBuffer(10000)
	for i := 0; i < 1000; i++ {
		buffer.Append(LogEntry{Source: "stderr", Timestamp: time.Now(), Line: fmt.Sprintf("2026-11-04 10:11:12 ERROR [%d 1.5s] connection failed", i)})
	}
	buffer.Append(LogEntry{Source: "stderr", Timestamp: time.Now(), Line: "ERROR remote port 443 failed"})
	buffer.Append(LogEntry{Source: "stderr", Timestamp: time.Now(), Line: "ERROR remote port 444 failed"})
	entries := buffer.SnapshotRecent(100)
	if len(entries) != 3 || entries[0].Count != 1000 {
		t.Fatalf("bad collapse: %#v", entries)
	}
}

func TestLogCollapseKeepsRecentlyRepeatedMessage(t *testing.T) {
	entries := []LogEntry{{Line: "first"}, {Line: "second"}, {Line: "third"}, {Line: "first"}, {Line: "fourth"}}
	got := CollapseLogs(entries, 2)
	if len(got) != 2 || got[0].Line != "first" || got[0].Count != 2 || got[1].Line != "fourth" {
		t.Fatal("recent repeat evicted because its first occurrence was old", got)
	}
}

func TestLogMaintenanceRetainsConcurrentWritesAndSurvivesRestart(t *testing.T) {
	m := NewManager("")
	defer m.CloseLogs()
	dir := t.TempDir()
	if err := m.SetLogDirectory(dir); err != nil {
		t.Fatal(err)
	}
	name := variant.DefaultKernel
	m.recordLog(name, LogEntry{Timestamp: time.Now().AddDate(0, 0, -9), Line: "expired"})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			m.recordLog(name, LogEntry{Timestamp: time.Now(), Line: fmt.Sprint(i)})
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 5; i++ {
			if err := m.PruneLogs(time.Now()); err != nil {
				t.Error(err)
			}
		}
	}()
	workers.Wait()
	loaded := NewRingBuffer(10000)
	if err := loadLogFile(filepath.Join(dir, logFileName(name)), loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Size() != 100 {
		t.Fatalf("lost writes or retained expired lines: %d", loaded.Size())
	}
	if err := m.ClearLogs(name); err != nil {
		t.Fatal(err)
	}
	if got := m.GetLogs(name, 0); len(got) != 0 {
		t.Fatal("clear did not remove in-memory history")
	}
	m.CloseLogs()
	if err := m.SetLogDirectory(dir); err != nil {
		t.Fatal(err)
	}
	if got := m.GetLogs(name, 0); len(got) != 0 {
		t.Fatal("cleared logs returned after restart")
	}
}
