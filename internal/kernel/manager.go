package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/holi/duorelay/internal/variant"
	"gopkg.in/yaml.v3"
)

// Manager serializes lifecycle operations while allowing concurrent status/log reads.
type Manager struct {
	opMu sync.Mutex
	mu   sync.RWMutex
	kernelProcesses
	mihomoProc        *Process
	mode              RunMode
	kernelsDir        string
	command           func(context.Context, string, ...string) *exec.Cmd
	validationTimeout time.Duration
	healthTimeout     time.Duration
	stopTimeout       time.Duration
	stableUptime      time.Duration
	restartPolicy     func() *RestartPolicy
	logSubs           map[string]map[chan LogEntry]struct{}
	logHistory        map[string]*RingBuffer
	logDir            string
	logFileMu         sync.Mutex
}

type RunMode string

const (
	ModeSingleSwitch RunMode = "single_switch"
	ModeCoexist      RunMode = "coexist"
)

// Process holds a managed lifetime, including all restart attempts.
// Mutable fields are protected by Manager.mu; callers use Status and GetLogs.
type Process struct {
	Name          string
	Binary        string
	Config        string
	Cmd           *exec.Cmd
	Ctx           context.Context
	Cancel        context.CancelFunc
	Running       bool
	LogBuffer     *RingBuffer
	RestartPolicy *RestartPolicy
	ExitCode      int
	ExitReason    string
	StartTime     time.Time
	Healthy       bool
	HealthError   string
	controller    string
	secret        string
	done          chan struct{}
}

type attempt struct {
	cmd     *exec.Cmd
	done    chan struct{}
	err     error
	started time.Time
}

// NewManager accepts an optional kernels directory; the default is ./kernels.
func NewManager(kernelsDir ...string) *Manager {
	dir := "kernels"
	if len(kernelsDir) > 0 && kernelsDir[0] != "" {
		dir = kernelsDir[0]
	}
	return &Manager{
		mode: ModeSingleSwitch, kernelsDir: dir, command: exec.CommandContext,
		validationTimeout: 10 * time.Second, healthTimeout: 30 * time.Second,
		stopTimeout: 5 * time.Second, stableUptime: 30 * time.Second,
		restartPolicy: NewRestartPolicy,
		logSubs:       map[string]map[chan LogEntry]struct{}{},
		logHistory:    map[string]*RingBuffer{},
	}
}

// Validate checks a configuration without disrupting any running process.
func (m *Manager) Validate(name, configPath string) error {
	if !variant.SupportsKernel(name) {
		return fmt.Errorf("unknown kernel %q", name)
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()
	binary, err := m.binaryPath(name)
	if err != nil {
		return err
	}
	config, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	return m.validate(name, binary, config)
}

func (m *Manager) active(p *Process) bool {
	if p == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (m *Manager) start(name, configPath string) error {
	if !variant.SupportsKernel(name) {
		return fmt.Errorf("unknown kernel %q", name)
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.RLock()
	busy := m.active(m.process(name))
	mode := m.mode
	other := m.otherProcess(name)
	m.mu.RUnlock()
	if busy {
		return fmt.Errorf("%s is already running or restarting", name)
	}
	config, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	binary, err := m.binaryPath(name)
	if err != nil {
		return err
	}
	// Validate the replacement before disrupting the current kernel.
	if err := m.validate(name, binary, config); err != nil {
		return err
	}
	controller, secret, err := readController(name, config)
	if err != nil {
		return err
	}
	if mode == ModeSingleSwitch {
		if err := m.stopProcess(other); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	buffer := NewRingBuffer(10000)
	m.mu.RLock()
	if history := m.logHistory[name]; history != nil {
		for _, entry := range history.GetAll() {
			buffer.Append(entry)
		}
	}
	m.mu.RUnlock()
	p := &Process{Name: name, Binary: binary, Config: config, Ctx: ctx, Cancel: cancel,
		LogBuffer: buffer, RestartPolicy: m.restartPolicy(),
		controller: controller, secret: secret, done: make(chan struct{})}
	m.mu.Lock()
	a, err := m.launch(p)
	if err == nil {
		m.setProcess(name, p)
	}
	m.mu.Unlock()
	if err != nil {
		cancel()
		return err
	}
	go m.monitorProcess(p, a)
	if err := m.waitHealthy(p, a); err != nil {
		return errors.Join(fmt.Errorf("%s startup: %w", name, err), m.stopProcess(p))
	}
	return nil
}

func (m *Manager) binaryPath(name string) (string, error) {
	file := name
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	path, err := filepath.Abs(filepath.Join(m.kernelsDir, file))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s binary not found: %w", name, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s binary is a directory", path)
	}
	return path, nil
}

func (m *Manager) validate(name, binary, config string) error {
	if info, err := os.Stat(config); err != nil {
		return fmt.Errorf("config: %w", err)
	} else if info.IsDir() {
		return fmt.Errorf("config is a directory")
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.validationTimeout)
	defer cancel()
	cmd := m.command(ctx, binary, kernelArgs(name, config, true)...)
	cmd.WaitDelay = m.stopTimeout
	output := &limitedOutput{}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("%s config validation: %w", name, ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("%s config validation: %w: %s", name, err, output.String())
	}
	return nil
}

// launch is called with m.mu held. Each command has exactly one Wait caller.
func (m *Manager) launch(p *Process) (*attempt, error) {
	cmd := m.command(p.Ctx, p.Binary, kernelArgs(p.Name, p.Config, false)...)
	if p.Name == "mihomo" && runtime.GOOS == "linux" && mihomoTUNEnabled(p.Config) {
		// Keep the configured IPv6 TUN route active on IPv4-only Linux hosts.
		cmd.Env = withEnvironment(os.Environ(), "SKIP_SYSTEM_IPV6_CHECK", "1")
	}
	cmd.Cancel = func() error {
		if runtime.GOOS != "windows" {
			if err := cmd.Process.Signal(syscall.SIGTERM); err == nil || errors.Is(err, os.ErrProcessDone) {
				return err
			}
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = m.stopTimeout
	stdout := newLogWriterWithCallback(p.LogBuffer, "stdout", func(entry LogEntry) { m.recordLog(p.Name, entry) })
	stderr := newLogWriterWithCallback(p.LogBuffer, "stderr", func(entry LogEntry) { m.recordLog(p.Name, entry) })
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", p.Name, err)
	}
	a := &attempt{cmd: cmd, done: make(chan struct{}), started: time.Now()}
	p.Cmd, p.StartTime, p.Running, p.Healthy = cmd, a.started, true, false
	go func() {
		a.err = cmd.Wait()
		stdout.Flush()
		stderr.Flush()
		close(a.done)
	}()
	return a, nil
}

func mihomoTUNEnabled(configPath string) bool {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	var config struct {
		TUN struct {
			Enable bool `yaml:"enable"`
		} `yaml:"tun"`
	}
	return yaml.Unmarshal(data, &config) == nil && config.TUN.Enable
}

func withEnvironment(environment []string, key, value string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name != key {
			result = append(result, entry)
		}
	}
	return append(result, key+"="+value)
}

// SubscribeLogs returns a live stream of log entries for one kernel. The
// cancel function removes the subscriber and closes its channel.
func (m *Manager) SubscribeLogs(name string) (<-chan LogEntry, func(), error) {
	if !variant.SupportsKernel(name) {
		return nil, nil, fmt.Errorf("unknown kernel %q", name)
	}
	ch := make(chan LogEntry, 128)
	m.mu.Lock()
	if m.logSubs == nil {
		m.logSubs = map[string]map[chan LogEntry]struct{}{}
	}
	if m.logSubs[name] == nil {
		m.logSubs[name] = map[chan LogEntry]struct{}{}
	}
	m.logSubs[name][ch] = struct{}{}
	m.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			m.mu.Lock()
			if subscribers := m.logSubs[name]; subscribers != nil {
				delete(subscribers, ch)
			}
			close(ch)
			m.mu.Unlock()
		})
	}
	return ch, cancel, nil
}

func (m *Manager) publishLog(name string, entry LogEntry) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for ch := range m.logSubs[name] {
		select {
		case ch <- entry:
		default:
		}
	}
}

func (m *Manager) stop(name string) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.RLock()
	p := m.process(name)
	m.mu.RUnlock()
	return m.stopProcess(p)
}

// StopAll also cancels restarts which are currently waiting in backoff.
func (m *Manager) StopAll() error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.RLock()
	a, b := m.otherProcess("mihomo"), m.mihomoProc
	m.mu.RUnlock()
	return errors.Join(m.stopProcess(a), m.stopProcess(b))
}

func (m *Manager) stopProcess(p *Process) error {
	if p == nil {
		return nil
	}
	m.mu.Lock()
	p.RestartPolicy.Disable()
	p.Cancel()
	m.mu.Unlock()
	timer := time.NewTimer(m.stopTimeout + time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		return m.cleanupStoppedTUN(p)
	case <-timer.C:
		m.mu.RLock()
		cmd := p.Cmd
		m.mu.RUnlock()
		if cmd != nil && cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return fmt.Errorf("stop %s: %w", p.Name, err)
			}
		}
	}
	timer.Reset(m.stopTimeout)
	select {
	case <-p.done:
		return m.cleanupStoppedTUN(p)
	case <-timer.C:
		return fmt.Errorf("timed out stopping %s", p.Name)
	}
}

func (m *Manager) cleanupStoppedTUN(p *Process) error {
	if p == nil || p.Name != "mihomo" || runtime.GOOS != "linux" || !mihomoTUNEnabled(p.Config) {
		return nil
	}
	return cleanupMihomoTUNNetwork()
}

func readController(name, config string) (string, string, error) {
	data, err := os.ReadFile(config)
	if err != nil {
		return "", "", err
	}
	var c struct {
		Controller   string `yaml:"external-controller"`
		Secret       string `yaml:"secret"`
		Experimental struct {
			Clash struct {
				Controller string `yaml:"external_controller"`
				Secret     string `yaml:"secret"`
			} `yaml:"clash_api"`
		} `yaml:"experimental"`
	}
	// YAML also decodes the JSON used by sing-box.
	if err := yaml.Unmarshal(data, &c); err != nil {
		return "", "", fmt.Errorf("read controller config: %w", err)
	}
	address, secret := c.Controller, c.Secret
	if name == "sing-box" {
		address, secret = c.Experimental.Clash.Controller, c.Experimental.Clash.Secret
	}
	if address == "" {
		return "", secret, nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return "", "", fmt.Errorf("invalid external controller %q", address)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::" {
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, port), secret, nil
}

func (m *Manager) waitHealthy(p *Process, a *attempt) error {
	if p.controller == "" {
		select {
		case <-a.done:
			return m.startupExitError(p, a)
		default:
			return nil
		}
	}
	ctx, cancel := context.WithTimeout(p.Ctx, m.healthTimeout)
	defer cancel()
	var lastErr error
	for {
		select {
		case <-a.done:
			return m.startupExitError(p, a)
		default:
		}
		lastErr = probeVersion(ctx, p.controller, p.secret)
		m.mu.Lock()
		if p.Cmd == a.cmd {
			p.Healthy = lastErr == nil && p.Running && p.Ctx.Err() == nil
			p.HealthError = ""
			if lastErr != nil {
				p.HealthError = lastErr.Error()
			}
		}
		m.mu.Unlock()
		if lastErr == nil {
			select {
			case <-a.done:
				return m.startupExitError(p, a)
			default:
			}
			return p.Ctx.Err()
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("health check: %w (last probe: %v)", ctx.Err(), lastErr)
		case <-a.done:
			timer.Stop()
			return m.startupExitError(p, a)
		case <-timer.C:
		}
	}
}

func (m *Manager) startupExitError(p *Process, a *attempt) error {
	err := fmt.Errorf("process exited during startup: %v", a.err)
	entries := p.LogBuffer.GetRecent(20)
	lines := make([]string, 0, 8)
	for i := len(entries) - 1; i >= 0 && len(lines) < 8; i-- {
		entry := entries[i]
		if entry.Source != "stderr" || strings.TrimSpace(entry.Line) == "" {
			continue
		}
		lines = append(lines, strings.TrimSpace(entry.Line))
	}
	if len(lines) == 0 {
		return err
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return fmt.Errorf("%w; recent stderr: %s", err, strings.Join(lines, " | "))
}

func probeVersion(ctx context.Context, baseURL, secret string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/version", nil)
	if err != nil {
		return err
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	// Local control requests must bypass environment proxy settings.
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("/version returned HTTP %d", resp.StatusCode)
	}
	var version struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&version); err != nil {
		return fmt.Errorf("decode /version: %w", err)
	}
	if strings.TrimSpace(version.Version) == "" {
		return fmt.Errorf("/version returned an empty version")
	}
	return nil
}

// HealthCheck probes a running kernel's configured Clash API on demand.
func (m *Manager) HealthCheck(ctx context.Context, name string) error {
	if !variant.SupportsKernel(name) {
		return fmt.Errorf("unknown kernel %q", name)
	}
	m.mu.RLock()
	p := m.process(name)
	if p == nil || !p.Running {
		m.mu.RUnlock()
		return fmt.Errorf("%s is not running", name)
	}
	cmd := p.Cmd
	controller, secret := p.controller, p.secret
	m.mu.RUnlock()
	if controller == "" {
		return fmt.Errorf("%s Clash API is not configured", name)
	}
	err := probeVersion(ctx, controller, secret)
	m.mu.Lock()
	if p.Cmd == cmd {
		p.Healthy = err == nil && p.Running && p.Ctx.Err() == nil
		p.HealthError = ""
		if err != nil {
			p.HealthError = err.Error()
		}
	}
	m.mu.Unlock()
	return err
}

// GetLogs returns a chronological snapshot; n <= 0 returns all retained lines.
func (m *Manager) GetLogs(name string, n int) []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !variant.SupportsKernel(name) {
		return []LogEntry{}
	}
	p := m.process(name)
	if p == nil {
		if history := m.logHistory[name]; history != nil {
			if n <= 0 {
				return history.GetAll()
			}
			return history.GetRecent(n)
		}
		return []LogEntry{}
	}
	if n <= 0 {
		return p.LogBuffer.GetAll()
	}
	return p.LogBuffer.GetRecent(n)
}

// Status returns independent snapshots, safe to inspect during starts/stops.
func (m *Manager) Status() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := func(p *Process) map[string]any {
		s := map[string]any{"running": false, "restarting": false, "healthy": false, "health_check_enabled": false}
		if p == nil {
			return s
		}
		s["running"], s["restarting"] = p.Running, m.active(p) && !p.Running && p.Ctx.Err() == nil
		s["healthy"], s["health_error"], s["health_check_enabled"] = p.Healthy, p.HealthError, p.controller != ""
		s["start_time"], s["exit_code"], s["exit_reason"] = p.StartTime, p.ExitCode, p.ExitReason
		s["restart_count"] = len(p.RestartPolicy.GetRestartHistory())
		s["consecutive_failures"] = p.RestartPolicy.GetConsecutiveFailures()
		if p.Running && p.Cmd != nil {
			s["pid"] = p.Cmd.Process.Pid
		}
		return s
	}
	return m.statusSnapshot(status)
}

// limitedOutput retains bounded validator diagnostics while draining all output.
type limitedOutput struct {
	mu   sync.Mutex
	data []byte
}

func (b *limitedOutput) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(data)
	if remaining := 64*1024 - len(b.data); remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		b.data = append(b.data, data...)
	}
	return n, nil
}
func (b *limitedOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}
