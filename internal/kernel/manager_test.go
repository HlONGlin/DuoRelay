package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/holi/duorelay/internal/variant"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestKernelHelper is a real child process speaking the kernels' command-line
// contracts. Tests exercise OS process exit/kill and HTTP, without shell scripts.
func TestKernelHelper(t *testing.T) {
	if os.Getenv("DUORELAY_KERNEL_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 3 {
		os.Exit(90)
	}
	name, args := args[1], args[2:]
	check, config := false, ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "check", "-t":
			check = true
		case "run":
		case "-c", "-f":
			i++
			config = args[i]
		case "-d":
			i++
		default:
			os.Exit(91)
		}
	}
	if name == "sing-box" && (len(args) == 0 || (args[0] != "check" && args[0] != "run")) {
		os.Exit(92)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		os.Exit(93)
	}
	var options struct {
		Invalid         bool   `yaml:"invalid"`
		ValidationDelay int    `yaml:"validation_delay"`
		CrashAfter      int    `yaml:"crash_after"`
		ExitCode        int    `yaml:"exit_code"`
		NoAPI           bool   `yaml:"no_api"`
		Marker          string `yaml:"marker"`
		OutputLines     int    `yaml:"output_lines"`
	}
	if yaml.Unmarshal(data, &options) != nil {
		os.Exit(94)
	}
	if check {
		time.Sleep(time.Duration(options.ValidationDelay) * time.Millisecond)
		if options.Invalid {
			fmt.Fprintln(os.Stderr, "invalid test configuration")
			os.Exit(2)
		}
		os.Exit(0)
	}
	if options.Marker != "" {
		if _, err := os.Stat(options.Marker); err == nil {
			options.CrashAfter = 0
		}
		if err := os.WriteFile(options.Marker, []byte("started"), 0600); err != nil {
			os.Exit(95)
		}
	}
	controller, secret, err := readController(name, config)
	if err != nil {
		os.Exit(96)
	}
	if controller != "" && !options.NoAPI {
		listener, err := net.Listen("tcp", strings.TrimPrefix(controller, "http://"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(97)
		}
		go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/version" {
				http.NotFound(w, r)
				return
			}
			if secret != "" && r.Header.Get("Authorization") != "Bearer "+secret {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"version":"test-kernel"}`)
		}))
	}
	fmt.Fprintln(os.Stdout, "kernel started")
	fmt.Fprintln(os.Stderr, "stderr captured")
	for i := 0; i < options.OutputLines; i++ {
		fmt.Fprintln(os.Stdout, "line-"+strconv.Itoa(i))
	}
	if options.CrashAfter > 0 {
		time.Sleep(time.Duration(options.CrashAfter) * time.Millisecond)
		fmt.Fprint(os.Stdout, "final partial line")
		os.Exit(options.ExitCode)
	}
	// Block without Go's deadlock detector: a process with no API is valid too.
	for {
		time.Sleep(time.Hour)
	}
}

func TestMihomoTUNEnvironmentDetection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("tun:\n  enable: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !mihomoTUNEnabled(path) {
		t.Fatal("enabled mihomo TUN was not detected")
	}
	if err := os.WriteFile(path, []byte("tun:\n  enable: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if mihomoTUNEnabled(path) {
		t.Fatal("disabled mihomo TUN was detected as enabled")
	}
}

func TestWithEnvironmentReplacesExistingValue(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	got := withEnvironment([]string{"PATH=/bin", "SKIP_SYSTEM_IPV6_CHECK=0", "OTHER=x"}, "SKIP_SYSTEM_IPV6_CHECK", "1")
	count := 0
	for _, entry := range got {
		if entry == "SKIP_SYSTEM_IPV6_CHECK=1" {
			count++
		}
		if entry == "SKIP_SYSTEM_IPV6_CHECK=0" {
			t.Fatal("old environment value was retained")
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one forced value, got %v", got)
	}
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"sing-box", "mihomo"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0700); err != nil {
			t.Fatal(err)
		}
	}
	m := NewManager(dir)
	m.command = func(ctx context.Context, binary string, args ...string) *exec.Cmd {
		name := strings.TrimSuffix(filepath.Base(binary), ".exe")
		childArgs := append([]string{"-test.run=^TestKernelHelper$", "--", name}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], childArgs...)
		cmd.Env = append(os.Environ(), "DUORELAY_KERNEL_HELPER=1")
		return cmd
	}
	m.healthTimeout = 3 * time.Second
	m.stopTimeout = time.Second
	m.restartPolicy = func() *RestartPolicy {
		p := NewRestartPolicy()
		p.backoffBase, p.maxBackoff = 20*time.Millisecond, 80*time.Millisecond
		return p
	}
	t.Cleanup(func() {
		if err := m.StopAll(); err != nil {
			t.Error(err)
		}
	})
	return m
}

func freeAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	return address
}

func writeConfig(t *testing.T, name string, extra map[string]any) string {
	t.Helper()
	config := map[string]any{}
	if name == "sing-box" {
		config["experimental"] = map[string]any{"clash_api": map[string]any{"external_controller": freeAddress(t), "secret": "test-secret"}}
	} else {
		config["external-controller"], config["secret"] = freeAddress(t), "test-secret"
	}
	for key, value := range extra {
		config[key] = value
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "kernel config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func kernelStatus(m *Manager, name string) map[string]any {
	if name == "sing-box" {
		name = "singbox"
	}
	return m.Status()[name].(map[string]any)
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition did not become true")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSubscribeLogsReceivesAndCloses(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	m := testManager(t)
	entries, cancel, err := m.SubscribeLogs("sing-box")
	if err != nil {
		t.Fatal(err)
	}
	config := writeConfig(t, "sing-box", map[string]any{"output_lines": 1})
	if err := m.start("sing-box", config); err != nil {
		t.Fatal(err)
	}
	select {
	case entry := <-entries:
		if entry.Line == "" {
			t.Fatal("received an empty log entry")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a live log entry")
	}
	cancel()
	deadline := time.After(time.Second)
	for {
		select {
		case _, open := <-entries:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for log subscription close")
		}
	}
}

func TestManagerLifecycle(t *testing.T) {
	for _, name := range variant.Kernels() {
		t.Run(name, func(t *testing.T) {
			m := testManager(t)
			config := writeConfig(t, name, nil)
			stop := m.StopSingBox
			if name == "mihomo" {
				stop = m.StopMihomo
			}
			if err := m.start(name, config); err != nil {
				t.Fatal(err)
			}
			state := kernelStatus(m, name)
			if state["running"] != true || state["healthy"] != true || state["pid"].(int) <= 0 {
				t.Fatalf("bad status: %v", state)
			}
			if err := m.HealthCheck(context.Background(), name); err != nil {
				t.Fatal(err)
			}
			if err := m.start(name, config); err == nil {
				t.Fatal("duplicate start accepted")
			}
			eventually(t, func() bool { return len(m.GetLogs(name, 0)) >= 2 })
			seen := map[string]bool{}
			for _, entry := range m.GetLogs(name, 0) {
				seen[entry.Source] = true
			}
			if !seen["stdout"] || !seen["stderr"] {
				t.Fatalf("missing output stream: %v", seen)
			}
			if err := stop(); err != nil {
				t.Fatal(err)
			}
			if kernelStatus(m, name)["running"] != false {
				t.Fatal("still running")
			}
			if err := stop(); err != nil {
				t.Fatal(err)
			}
			if err := m.start(name, config); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestManagerModesAndInvalidReplacement(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	m := testManager(t)
	singbox := writeConfig(t, "sing-box", nil)
	mihomo := writeConfig(t, "mihomo", nil)
	if err := m.StartSingBox(singbox); err != nil {
		t.Fatal(err)
	}
	if err := m.StartMihomo(writeConfig(t, "mihomo", map[string]any{"invalid": true})); err == nil || !strings.Contains(err.Error(), "invalid test configuration") {
		t.Fatalf("bad validation error: %v", err)
	}
	if kernelStatus(m, "sing-box")["running"] != true {
		t.Fatal("invalid replacement stopped current kernel")
	}
	if err := m.StartMihomo(mihomo); err != nil {
		t.Fatal(err)
	}
	if kernelStatus(m, "sing-box")["running"] != false {
		t.Fatal("single mode left previous process running")
	}
	if err := m.SetMode(ModeCoexist); err != nil {
		t.Fatal(err)
	}
	if err := m.StartSingBox(singbox); err != nil {
		t.Fatal(err)
	}
	if kernelStatus(m, "mihomo")["running"] != true {
		t.Fatal("coexist mode stopped mihomo")
	}
	if err := m.SetMode(ModeSingleSwitch); err != nil {
		t.Fatal(err)
	}
	if kernelStatus(m, "mihomo")["running"] != false || kernelStatus(m, "sing-box")["running"] != true {
		t.Fatal("mode transition did not enforce a single process")
	}
	if err := m.SetMode("invalid"); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestManagerCrashRecovery(t *testing.T) {
	for _, name := range variant.Kernels() {
		t.Run(name, func(t *testing.T) {
			m := testManager(t)
			config := writeConfig(t, name, map[string]any{"crash_after": 250, "exit_code": 23, "marker": filepath.Join(t.TempDir(), "once")})
			if err := m.start(name, config); err != nil {
				t.Fatal(err)
			}
			pid := kernelStatus(m, name)["pid"]
			eventually(t, func() bool {
				s := kernelStatus(m, name)
				return s["running"] == true && s["healthy"] == true && s["pid"] != pid && s["restart_count"].(int) == 1
			})
			if got := kernelStatus(m, name)["exit_code"]; got != 23 {
				t.Fatalf("lost exit code: %v", got)
			}
			found := false
			for _, entry := range m.GetLogs(name, 0) {
				if entry.Line == "final partial line" {
					found = true
				}
			}
			if !found {
				t.Fatal("lost trailing output across restart")
			}
		})
	}
}

func TestManagerExternalKillRecovery(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	m := testManager(t)
	if err := m.StartSingBox(writeConfig(t, "sing-box", nil)); err != nil {
		t.Fatal(err)
	}
	pid := kernelStatus(m, "sing-box")["pid"].(int)
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Kill(); err != nil {
		t.Fatal(err)
	}
	process.Release()
	eventually(t, func() bool {
		s := kernelStatus(m, "sing-box")
		return s["healthy"] == true && s["pid"] != pid
	})
}

func TestManagerCleanUnexpectedExitRecovery(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	m := testManager(t)
	config := writeConfig(t, "sing-box", map[string]any{"crash_after": 200, "exit_code": 0, "marker": filepath.Join(t.TempDir(), "once")})
	if err := m.StartSingBox(config); err != nil {
		t.Fatal(err)
	}
	pid := kernelStatus(m, "sing-box")["pid"]
	eventually(t, func() bool {
		s := kernelStatus(m, "sing-box")
		return s["healthy"] == true && s["pid"] != pid && s["exit_code"] == 0
	})
}

func TestManagerStopDuringBackoff(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	m := testManager(t)
	m.restartPolicy = func() *RestartPolicy { p := NewRestartPolicy(); p.backoffBase = time.Hour; return p }
	if err := m.StartSingBox(writeConfig(t, "sing-box", map[string]any{"crash_after": 250, "exit_code": 4})); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return kernelStatus(m, "sing-box")["restarting"] == true })
	started := time.Now()
	if err := m.StopAll(); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("stop waited through backoff")
	}
	if s := kernelStatus(m, "sing-box"); s["restarting"] != false || s["restart_count"] != 0 {
		t.Fatalf("restart after stop: %v", s)
	}
}

func TestManagerRestartBudget(t *testing.T) {
	m := testManager(t)
	m.restartPolicy = func() *RestartPolicy {
		p := NewRestartPolicy()
		p.maxRetries = 2
		p.backoffBase = 10 * time.Millisecond
		return p
	}
	if err := m.StartMihomo(writeConfig(t, "mihomo", map[string]any{"crash_after": 180, "exit_code": 9})); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		s := kernelStatus(m, "mihomo")
		return s["running"] == false && strings.Contains(fmt.Sprint(s["exit_reason"]), "max consecutive failures")
	})
	if s := kernelStatus(m, "mihomo"); s["restart_count"] != 2 || s["consecutive_failures"] != 2 {
		t.Fatalf("incorrect restart budget: %v", s)
	}
	// A settled lifetime can be started manually with a fresh restart budget.
	if err := m.StartMihomo(writeConfig(t, "mihomo", nil)); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRestartLaunchFailure(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	m := testManager(t)
	base := m.command
	launches := 0
	m.command = func(ctx context.Context, binary string, args ...string) *exec.Cmd {
		if filepath.Base(binary) == "sing-box.exe" || filepath.Base(binary) == "sing-box" {
			if len(args) > 0 && args[0] == "run" {
				launches++
				if launches > 1 {
					return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-binary"))
				}
			}
		}
		return base(ctx, binary, args...)
	}
	if err := m.StartSingBox(writeConfig(t, "sing-box", map[string]any{"crash_after": 200, "exit_code": 3})); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		return strings.Contains(fmt.Sprint(kernelStatus(m, "sing-box")["exit_reason"]), "max consecutive failures")
	})
	if s := kernelStatus(m, "sing-box"); s["restart_count"] != 5 || s["exit_code"] != -1 {
		t.Fatalf("launch failures not budgeted: %v", s)
	}
}

func TestManagerValidationAndHealthFailure(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	t.Run("missing binary", func(t *testing.T) {
		m := NewManager(t.TempDir())
		if err := m.StartSingBox(writeConfig(t, "sing-box", nil)); err == nil {
			t.Fatal("missing binary accepted")
		}
	})
	t.Run("missing config", func(t *testing.T) {
		m := testManager(t)
		if err := m.StartMihomo(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("missing config accepted")
		}
	})
	t.Run("validation timeout", func(t *testing.T) {
		m := testManager(t)
		m.validationTimeout = 100 * time.Millisecond
		err := m.StartSingBox(writeConfig(t, "sing-box", map[string]any{"validation_delay": 3000}))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected timeout, got %v", err)
		}
	})
	t.Run("unhealthy startup", func(t *testing.T) {
		m := testManager(t)
		m.healthTimeout = 250 * time.Millisecond
		if err := m.StartMihomo(writeConfig(t, "mihomo", map[string]any{"no_api": true})); err == nil {
			t.Fatal("unhealthy startup accepted")
		}
		if s := kernelStatus(m, "mihomo"); s["running"] != false || s["restarting"] != false {
			t.Fatalf("failed startup leaked: %v", s)
		}
	})
	t.Run("API disabled", func(t *testing.T) {
		m := testManager(t)
		if err := m.StartMihomo(writeConfig(t, "mihomo", map[string]any{"external-controller": ""})); err != nil {
			t.Fatal(err)
		}
		if s := kernelStatus(m, "mihomo"); s["healthy"] != false || s["health_check_enabled"] != false {
			t.Fatalf("disabled API marked healthy: %v", s)
		}
		if err := m.HealthCheck(context.Background(), "mihomo"); err == nil {
			t.Fatal("missing API accepted")
		}
	})
}

func TestManagerConcurrentObservation(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	m := testManager(t)
	if err := m.StartSingBox(writeConfig(t, "sing-box", nil)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				m.Status()
				m.GetLogs("sing-box", 0)
				_ = m.HealthCheck(context.Background(), "sing-box")
			}
		}()
	}
	if err := m.StopAll(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

func TestProbeVersion(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		good       bool
	}{
		{"valid", `{"version":"1.0"}`, 200, true},
		{"unauthorized", `{"message":"unauthorized"}`, 401, false},
		{"invalid JSON", `not-json`, 200, false},
		{"missing version", `{}`, 200, false},
		{"empty version", `{"version":" "}`, 200, false},
		{"redirect", ``, 302, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/version" || r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("incorrect probe request")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			if err := probeVersion(context.Background(), server.URL, "secret"); (err == nil) != tc.good {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
}

func TestRingBufferAndLogCapture(t *testing.T) {
	b := NewRingBuffer(10000)
	for i := 0; i < 10010; i++ {
		b.Append(LogEntry{Line: strconv.Itoa(i)})
	}
	all := b.GetAll()
	if len(all) != 10000 || all[0].Line != "10" || all[9999].Line != "10009" {
		t.Fatal("incorrect ring ordering/eviction")
	}
	all[0].Line = "mutated"
	if b.GetAll()[0].Line != "10" {
		t.Fatal("snapshot aliases storage")
	}
	if len(b.GetRecent(-1)) != 0 || len(b.GetRecent(2)) != 2 {
		t.Fatal("incorrect recent selection")
	}
	b.Clear()
	if b.Size() != 0 {
		t.Fatal("clear failed")
	}
	for _, capacity := range []int{0, -1} {
		fallback := NewRingBuffer(capacity)
		fallback.Append(LogEntry{Line: "safe"})
		if fallback.Size() != 1 {
			t.Fatal("invalid capacity panic/failure")
		}
	}
	long := strings.Repeat("x", 600000)
	capture := NewLogCapture(b, "stderr")
	capture.Start(strings.NewReader("one\r\n\n" + long + "\ntrailing"))
	capture.Wait()
	entries := b.GetAll()
	if len(entries) != 5 || entries[0].Line != "one" || entries[1].Line != "" || entries[2].Line+entries[3].Line != long || entries[4].Line != "trailing" {
		t.Fatal("log splitting or trailing line lost")
	}
	for _, entry := range entries {
		if entry.Source != "stderr" || entry.Timestamp.IsZero() {
			t.Fatal("missing metadata")
		}
	}
	// A newline exactly after a full chunk must not create a phantom blank line.
	b.Clear()
	writer := newLogWriter(b, "stdout")
	_, _ = writer.Write([]byte(strings.Repeat("x", 512*1024)))
	_, _ = writer.Write([]byte("\n\n"))
	writer.Flush()
	if got := b.GetAll(); len(got) != 2 || got[1].Line != "" {
		t.Fatalf("incorrect chunk boundary: %d entries", len(got))
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				b.Append(LogEntry{})
				b.GetAll()
				b.Clear()
			}
		}()
	}
	wg.Wait()
}

func TestRestartPolicy(t *testing.T) {
	p := NewRestartPolicy()
	for i, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second} {
		if ok, err := p.ShouldRestart(1, "crash"); !ok || err != nil {
			t.Fatalf("attempt %d denied: %v", i, err)
		}
		if got := p.GetBackoffDuration(); got != want {
			t.Fatalf("attempt %d: got %s want %s", i, got, want)
		}
		p.RecordRestart(1, "crash", true)
	}
	if ok, _ := p.ShouldRestart(1, "crash"); ok {
		t.Fatal("rapid successful launches reset the retry budget")
	}
	p.Reset()
	if p.GetConsecutiveFailures() != 0 {
		t.Fatal("reset failed")
	}
	p.Disable()
	if ok, _ := p.ShouldRestart(1, "crash"); ok || p.IsEnabled() {
		t.Fatal("disabled restart allowed")
	}
	p.Enable()
	if !p.IsEnabled() {
		t.Fatal("enable failed")
	}
	for i := 0; i < 110; i++ {
		p.RecordRestart(i, "crash", false)
	}
	history := p.GetRestartHistory()
	if len(history) != 100 || history[0].ExitCode != 10 {
		t.Fatal("history bound failed")
	}
	history[0].Reason = "mutated"
	if p.GetRestartHistory()[0].Reason != "crash" {
		t.Fatal("history aliases storage")
	}
}

func TestReadController(t *testing.T) {
	for _, tc := range []struct {
		address, want string
		valid         bool
	}{
		{"127.0.0.1:9090", "http://127.0.0.1:9090", true},
		{"0.0.0.0:9090", "http://127.0.0.1:9090", true},
		{":9090", "http://127.0.0.1:9090", true},
		{"[::]:9090", "http://[::1]:9090", true},
		{"[::1]:9090", "http://[::1]:9090", true},
		{"invalid", "", false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			config := writeConfig(t, "mihomo", map[string]any{"external-controller": tc.address})
			url, secret, err := readController("mihomo", config)
			if (err == nil) != tc.valid || url != tc.want {
				t.Fatalf("unexpected controller: %s %v", url, err)
			}
			if tc.valid && secret != "test-secret" {
				t.Fatal("lost secret")
			}
		})
	}
}

// Real-kernel coverage is explicit so ordinary unit tests work without binaries.
func TestRealKernels(t *testing.T) {
	if os.Getenv("DUORELAY_KERNEL_INTEGRATION") != "1" {
		t.Skip("set DUORELAY_KERNEL_INTEGRATION=1 to run bundled kernels")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "kernels"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range variant.Kernels() {
		t.Run(name, func(t *testing.T) {
			m := NewManager(dir)
			t.Cleanup(func() {
				if err := m.StopAll(); err != nil {
					t.Error(err)
				}
			})
			extra := map[string]any{}
			if name == "sing-box" {
				extra["inbounds"] = []any{}
				extra["outbounds"] = []any{map[string]any{"type": "direct", "tag": "DIRECT"}}
			} else {
				extra["mode"], extra["mixed-port"], extra["dns"] = "direct", 0, map[string]any{"enable": false}
			}
			if err := m.start(name, writeConfig(t, name, extra)); err != nil {
				t.Fatalf("%v; logs: %v", err, m.GetLogs(name, 20))
			}
			if err := m.HealthCheck(context.Background(), name); err != nil {
				t.Fatal(err)
			}
			pid := kernelStatus(m, name)["pid"].(int)
			process, err := os.FindProcess(pid)
			if err != nil {
				t.Fatal(err)
			}
			if err := process.Kill(); err != nil {
				t.Fatal(err)
			}
			process.Release()
			eventually(t, func() bool { s := kernelStatus(m, name); return s["healthy"] == true && s["pid"] != pid })
			if len(m.GetLogs(name, 0)) == 0 {
				t.Fatal("no real kernel output captured")
			}
			if err := m.stop(name); err != nil {
				t.Fatal(err)
			}
			invalid := map[string]any{"proxies": []any{map[string]any{"name": "bad", "type": "not-a-protocol"}}}
			if name == "sing-box" {
				invalid = map[string]any{"outbounds": []any{map[string]any{"type": "not-a-protocol"}}}
			}
			if err := m.start(name, writeConfig(t, name, invalid)); err == nil || !strings.Contains(err.Error(), "config validation") {
				t.Fatalf("native validator did not reject invalid config: %v", err)
			}
		})
	}
}
