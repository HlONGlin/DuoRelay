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

	"gopkg.in/yaml.v3"
)

// Manager serializes lifecycle operations while allowing concurrent status/log reads.
type Manager struct {
	opMu              sync.Mutex
	mu                sync.RWMutex
	singboxProc       *Process
	mihomoProc        *Process
	mode              RunMode
	kernelsDir        string
	command           func(context.Context, string, ...string) *exec.Cmd
	validationTimeout time.Duration
	healthTimeout     time.Duration
	stopTimeout       time.Duration
	stableUptime      time.Duration
	restartPolicy     func() *RestartPolicy
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
		validationTimeout: 10 * time.Second, healthTimeout: 5 * time.Second,
		stopTimeout: 5 * time.Second, stableUptime: 30 * time.Second,
		restartPolicy: NewRestartPolicy,
	}
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
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.RLock()
	busy := m.active(m.process(name))
	mode := m.mode
	other := m.singboxProc
	if name == "sing-box" {
		other = m.mihomoProc
	}
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
	p := &Process{Name: name, Binary: binary, Config: config, Ctx: ctx, Cancel: cancel,
		LogBuffer: NewRingBuffer(10000), RestartPolicy: m.restartPolicy(),
		controller: controller, secret: secret, done: make(chan struct{})}
	m.mu.Lock()
	a, err := m.launch(p)
	if err == nil {
		if name == "sing-box" {
			m.singboxProc = p
		} else {
			m.mihomoProc = p
		}
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
	cmd.Cancel = func() error {
		if runtime.GOOS != "windows" {
			if err := cmd.Process.Signal(syscall.SIGTERM); err == nil || errors.Is(err, os.ErrProcessDone) {
				return err
			}
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = m.stopTimeout
	stdout := newLogWriter(p.LogBuffer, "stdout")
	stderr := newLogWriter(p.LogBuffer, "stderr")
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
	a, b := m.singboxProc, m.mihomoProc
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
		return nil
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
		return nil
	case <-timer.C:
		return fmt.Errorf("timed out stopping %s", p.Name)
	}
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
			return fmt.Errorf("process exited during startup: %v", a.err)
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
			return fmt.Errorf("process exited during startup: %v", a.err)
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
				return fmt.Errorf("process exited during startup: %v", a.err)
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
			return fmt.Errorf("process exited during startup: %v", a.err)
		case <-timer.C:
		}
	}
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
	if name != "sing-box" && name != "mihomo" {
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
	if name != "sing-box" && name != "mihomo" {
		return []LogEntry{}
	}
	p := m.process(name)
	if p == nil {
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
	return map[string]any{"mode": m.mode, "singbox": status(m.singboxProc), "mihomo": status(m.mihomoProc)}
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
