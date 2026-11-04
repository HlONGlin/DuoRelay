package kernel

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const DefaultLogViewLimit = 100

var logANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var logPrefix = regexp.MustCompile(`^(?:\+\d{4}\s+)?\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?\s*`)
var logTimeField = regexp.MustCompile(`\btime="[^"]+"\s*`)
var logElapsedPrefix = regexp.MustCompile(`^((?i:trace|debug|info|warn|warning|error|fatal|panic))\s*\[[0-9.]+\]\s*`)
var logConnection = regexp.MustCompile(`\[\d+\s+[0-9.]+(?:ms|s)\]\s*`)

func LogMessageKey(line string) string {
	line = logANSI.ReplaceAllString(line, "")
	line = logPrefix.ReplaceAllString(strings.TrimSpace(line), "")
	line = logTimeField.ReplaceAllString(line, "")
	line = logElapsedPrefix.ReplaceAllString(line, "$1 ")
	return logConnection.ReplaceAllString(line, "")
}

// LogEntry represents a single log line with metadata
type LogEntry struct {
	Timestamp time.Time
	Source    string // "stdout" or "stderr"
	Line      string
	Count     int `json:",omitempty"`
}

// RingBuffer implements a fixed-size circular buffer for log entries
type RingBuffer struct {
	mu       sync.RWMutex
	buffer   []LogEntry
	capacity int
	head     int // next write position
	size     int // current number of entries
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 10000
	}
	return &RingBuffer{
		buffer:   make([]LogEntry, capacity),
		capacity: capacity,
		head:     0,
		size:     0,
	}
}

// Append adds a new log entry to the buffer
func (rb *RingBuffer) Append(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.head] = entry
	rb.head = (rb.head + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetRecent returns the most recent n log entries (or all if n > size)
func (rb *RingBuffer) GetRecent(n int) []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.size == 0 {
		return []LogEntry{}
	}

	if n > rb.size {
		n = rb.size
	}

	result := make([]LogEntry, n)

	// Calculate start position
	start := (rb.head - n + rb.capacity) % rb.capacity

	for i := 0; i < n; i++ {
		idx := (start + i) % rb.capacity
		result[i] = rb.buffer[idx]
	}

	return result
}

// GetAll returns all log entries in chronological order
func (rb *RingBuffer) GetAll() []LogEntry {
	return rb.GetRecent(rb.capacity)
}

// Size returns the current number of entries in the buffer
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// Clear removes all entries from the buffer
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.size = 0
	clear(rb.buffer)
}

func (rb *RingBuffer) PruneBefore(cutoff time.Time) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	kept := make([]LogEntry, 0, rb.size)
	for i := 0; i < rb.size; i++ {
		entry := rb.buffer[(rb.head-rb.size+i+rb.capacity)%rb.capacity]
		if !entry.Timestamp.Before(cutoff) {
			kept = append(kept, entry)
		}
	}
	clear(rb.buffer)
	copy(rb.buffer, kept)
	rb.size, rb.head = len(kept), len(kept)%rb.capacity
}

// SnapshotRecent returns the latest distinct messages with occurrence counts.
func (rb *RingBuffer) SnapshotRecent(n int) []LogEntry {
	return CollapseLogs(rb.GetAll(), n)
}

// CollapseLogs preserves the original text, so every repeated occurrence can
// be counted, including the third and subsequent occurrences.
func CollapseLogs(entries []LogEntry, n int) []LogEntry {
	if n <= 0 {
		n = DefaultLogViewLimit
	}
	if len(entries) == 0 {
		return entries
	}
	type recent struct {
		entry LogEntry
		index int
	}
	messages := make(map[string]recent)
	for index, entry := range entries {
		key := entry.Source + "\x00" + LogMessageKey(entry.Line)
		entry.Count = messages[key].entry.Count + max(1, entry.Count)
		messages[key] = recent{entry, index}
	}
	ordered := make([]recent, 0, len(messages))
	for _, message := range messages {
		ordered = append(ordered, message)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].index < ordered[j].index })
	if len(ordered) > n {
		ordered = ordered[len(ordered)-n:]
	}
	result := make([]LogEntry, len(ordered))
	for i, message := range ordered {
		result[i] = message.entry
	}
	return result
}

// LogCapture captures output from a reader and stores it in a ring buffer
type LogCapture struct {
	buffer *RingBuffer
	source string
	done   chan struct{}
	once   sync.Once
}

// NewLogCapture creates a new log capture for the given source
func NewLogCapture(buffer *RingBuffer, source string) *LogCapture {
	return &LogCapture{
		buffer: buffer,
		source: source,
		done:   make(chan struct{}),
	}
}

// Start begins capturing logs from the reader
func (lc *LogCapture) Start(reader io.Reader) {
	lc.once.Do(func() {
		go func() {
			defer close(lc.done)
			writer := newLogWriter(lc.buffer, lc.source)
			_, err := io.Copy(writer, bufio.NewReader(reader))
			writer.Flush()
			if err != nil {
				lc.buffer.Append(LogEntry{Timestamp: time.Now(), Source: lc.source, Line: "log capture: " + err.Error()})
			}
		}()
	})
}

// Wait waits for the capture to complete
func (lc *LogCapture) Wait() {
	<-lc.done
}

// logWriter streams complete lines to the ring without Scanner's token limit.
// Very long lines are split into bounded chunks so a noisy child cannot block
// pipe draining or grow the pending line indefinitely.
type logWriter struct {
	mu       sync.Mutex
	buffer   *RingBuffer
	source   string
	pending  []byte
	chunked  bool
	onAppend func(LogEntry)
}

func newLogWriter(buffer *RingBuffer, source string) *logWriter {
	return &logWriter{buffer: buffer, source: source}
}

func newLogWriterWithCallback(buffer *RingBuffer, source string, onAppend func(LogEntry)) *logWriter {
	return &logWriter{buffer: buffer, source: source, onAppend: onAppend}
}

func (w *logWriter) append(line []byte) {
	entry := LogEntry{Timestamp: time.Now(), Source: w.source, Line: string(bytes.TrimSuffix(line, []byte{'\r'}))}
	w.buffer.Append(entry)
	if w.onAppend != nil {
		w.onAppend(entry)
	}
}

func (w *logWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(data)
	const maxLine = 512 * 1024
	for len(data) > 0 {
		length := len(data)
		if length > maxLine-len(w.pending) {
			length = maxLine - len(w.pending)
		}
		if index := bytes.IndexByte(data[:length], '\n'); index >= 0 {
			w.pending = append(w.pending, data[:index]...)
			if len(w.pending) > 0 || !w.chunked {
				w.append(w.pending)
			}
			w.pending = w.pending[:0]
			w.chunked = false
			data = data[index+1:]
			continue
		}
		w.pending = append(w.pending, data[:length]...)
		data = data[length:]
		if len(w.pending) == maxLine {
			w.append(w.pending)
			w.pending = w.pending[:0]
			w.chunked = true
		}
	}
	return n, nil
}

func (w *logWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		w.append(w.pending)
		w.pending = nil
	}
}
