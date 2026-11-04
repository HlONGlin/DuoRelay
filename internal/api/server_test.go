package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/holi/duorelay/internal/variant"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/holi/duorelay/internal/clashapi"
	"github.com/holi/duorelay/internal/configuration"
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/speedtest"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/subscription"
	"github.com/holi/duorelay/internal/task"
	"github.com/holi/duorelay/internal/update"
)

type testRuntime struct {
	running        string
	logs           []kernel.LogEntry
	logMu          sync.Mutex
	logSubscribers map[chan kernel.LogEntry]struct{}
}

func (f *testRuntime) Validate(name, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(data), "fail_validation") {
		return errors.New("invalid")
	}
	return nil
}
func (f *testRuntime) StartSingBox(string) error { f.running = "sing-box"; return nil }
func (f *testRuntime) StartMihomo(string) error  { f.running = "mihomo"; return nil }
func (f *testRuntime) StopAll() error            { f.running = ""; return nil }
func (f *testRuntime) Status() map[string]any {
	return map[string]any{"singbox": map[string]any{"running": f.running == "sing-box", "pid": 1, "start_time": time.Now()}, "mihomo": map[string]any{"running": f.running == "mihomo", "pid": 2, "start_time": time.Now()}}
}
func (f *testRuntime) GetLogs(string, int) []kernel.LogEntry {
	if f.logs != nil {
		return append([]kernel.LogEntry(nil), f.logs...)
	}
	return []kernel.LogEntry{{Timestamp: time.Now(), Source: "stdout", Line: "test log"}}
}
func apiForTest(t *testing.T) *Server {
	t.Helper()
	s, err := store.New(&store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	q := task.NewQueue(2)
	t.Cleanup(func() { q.Close(); s.Close() })
	runtime := &testRuntime{logSubscribers: make(map[chan kernel.LogEntry]struct{})}
	config := configuration.New(s, runtime, t.TempDir())
	server := NewServer(s, subscription.NewManager(s), speedtest.NewEngine(s, q), config, runtime)
	server.Auth = nil
	return server
}

func (f *testRuntime) SubscribeLogs(string) (<-chan kernel.LogEntry, func(), error) {
	ch := make(chan kernel.LogEntry, 4)
	f.logMu.Lock()
	f.logSubscribers[ch] = struct{}{}
	f.logMu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			f.logMu.Lock()
			delete(f.logSubscribers, ch)
			close(ch)
			f.logMu.Unlock()
		})
	}
	return ch, cancel, nil
}

func (f *testRuntime) publishLog(entry kernel.LogEntry) {
	f.logMu.Lock()
	defer f.logMu.Unlock()
	for ch := range f.logSubscribers {
		select {
		case ch <- entry:
		default:
		}
	}
}

func TestLogWebSocketRejectsCrossOrigin(t *testing.T) {
	s := apiForTest(t)
	req := httptest.NewRequest(http.MethodGet, "/ws/logs?kernel=sing-box", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	s.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin websocket status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func call(t *testing.T, s *Server, method, path, body string, status int) json.RawMessage {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: status %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	var response struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   *Error          `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid response: %v %s", err, w.Body.String())
	}
	if response.Success != (status >= 200 && status < 300) {
		t.Fatal("incorrect success field")
	}
	if status >= 400 && response.Error == nil {
		t.Fatal("missing structured error")
	}
	return response.Data
}

func TestSubscriptionAPI(t *testing.T) {
	s := apiForTest(t)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "proxies: [{name: alpha, type: ss, server: example.com, port: 8388, cipher: aes-256-gcm, password: pass}]")
	}))
	defer remote.Close()
	input := fmt.Sprintf(`{"name":"api","url":%q,"group":"group"}`, remote.URL)
	var sub subscription.Subscription
	if err := json.Unmarshal(call(t, s, "POST", "/api/subscriptions", input, 201), &sub); err != nil {
		t.Fatal(err)
	}
	call(t, s, "GET", "/api/subscriptions", "", 200)
	call(t, s, "GET", "/api/subscriptions/"+sub.ID, "", 200)
	call(t, s, "GET", "/api/subscription-groups", "", 200)
	update := call(t, s, "POST", "/api/subscriptions/"+sub.ID+"/update", "", 200)
	if !strings.Contains(string(update), `"nodes_total":1`) {
		t.Fatalf("update result: %s", update)
	}
	input = fmt.Sprintf(`{"name":"changed","url":%q,"interval":4,"group":"new"}`, remote.URL)
	call(t, s, "PUT", "/api/subscriptions/"+sub.ID, input, 200)
	call(t, s, "DELETE", "/api/subscriptions/"+sub.ID, "", 200)
	call(t, s, "GET", "/api/subscriptions/"+sub.ID, "", 404)
	call(t, s, "POST", "/api/subscriptions", `{"name":"bad","url":"file:///tmp"}`, 400)
	call(t, s, "POST", "/api/subscriptions", `{"unknown":1}`, 400)
	call(t, s, "POST", "/api/subscriptions", `{} {}`, 400)
}

func TestConnectionsAPIUsesPanelFieldNames(t *testing.T) {
	s := apiForTest(t)
	kernelAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"downloadTotal":77268195,"uploadTotal":5799311,"memory":4096,"connections":[{"id":"conn-1","upload":12,"download":34,"start":"1970-01-01T00:00:01Z","metadata":{"host":"example.com"}}]}`)
	}))
	defer kernelAPI.Close()

	if err := s.Store.SaveState(context.Background(), "current_config", &configuration.State{
		ConfigVersion: configuration.CurrentConfigVersion,
		Kernel:        ir.KernelMihomo,
		Controller:    kernelAPI.URL,
		Secret:        "test-secret",
	}); err != nil {
		t.Fatal(err)
	}

	data := call(t, s, http.MethodGet, "/api/kernel/connections", "", http.StatusOK)
	var got struct {
		DownloadTotal  int64                 `json:"download_total"`
		UploadTotal    int64                 `json:"upload_total"`
		Memory         int64                 `json:"memory"`
		Connections    []clashapi.Connection `json:"connections"`
		LegacyDownload json.RawMessage       `json:"downloadTotal"`
		LegacyUpload   json.RawMessage       `json:"uploadTotal"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.DownloadTotal != 77268195 || got.UploadTotal != 5799311 {
		t.Fatalf("unexpected cumulative totals: %#v", got)
	}
	if got.Memory != 4096 || len(got.Connections) != 1 || got.Connections[0].ID != "conn-1" {
		t.Fatalf("unexpected connection payload: %#v", got)
	}
	if len(got.LegacyDownload) != 0 || len(got.LegacyUpload) != 0 {
		t.Fatalf("legacy camelCase fields leaked into panel response: %s", data)
	}
}

func TestNodeSpeedtestAndTaskAPI(t *testing.T) {
	s := apiForTest(t)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer remote.Close()
	// Use a node pointing at the local HTTP listener for deterministic TCP tests.
	var input ir.Node
	input.Name = "local"
	input.Protocol = ir.ProtocolHTTP
	input.Server.Host = "127.0.0.1"
	if _, err := fmt.Sscanf(strings.TrimPrefix(remote.URL, "http://127.0.0.1:"), "%d", &input.Server.Port); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(input)
	var node ir.Node
	if err := json.Unmarshal(call(t, s, "POST", "/api/nodes", string(data), 201), &node); err != nil {
		t.Fatal(err)
	}
	call(t, s, "GET", "/api/nodes?search=local&protocol=http", "", 200)
	call(t, s, "GET", "/api/nodes?limit=1&offset=0", "", 200)
	call(t, s, "GET", "/api/nodes?limit=0", "", 400)
	call(t, s, "GET", "/api/nodes?offset=-1", "", 400)
	result := call(t, s, "POST", "/api/nodes/"+node.ID+"/test", `{"timeout":1000}`, 200)
	if !strings.Contains(string(result), `"success":true`) {
		t.Fatalf("test result: %s", result)
	}
	batch := call(t, s, "POST", "/api/speedtest/batch", fmt.Sprintf(`{"node_ids":[%q],"type":"tcp"}`, node.ID), 202)
	var accepted struct {
		ID string `json:"task_id"`
	}
	if err := json.Unmarshal(batch, &accepted); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		value, _ := s.Speedtest.Task(accepted.ID)
		if value.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	call(t, s, "GET", "/api/speedtest/tasks/"+accepted.ID, "", 200)
	call(t, s, "GET", "/api/speedtest/tasks/missing", "", 404)
	call(t, s, "POST", "/api/speedtest/batch", `{"node_ids":[]}`, 400)
	call(t, s, "POST", "/api/nodes/missing/test", "", 404)
	deleted := call(t, s, "DELETE", "/api/nodes/"+node.ID, "", 200)
	if !strings.Contains(string(deleted), `"deleted":true`) {
		t.Fatalf("single-node delete response changed: %s", deleted)
	}
	call(t, s, "DELETE", "/api/nodes/"+node.ID, "", 404)
}

func TestNodeListDerivesRuntimeStatusFromCurrentConfig(t *testing.T) {
	s := apiForTest(t)
	first := &ir.Node{Name: "first", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "first.example", Port: 80}, Status: "active"}
	second := &ir.Node{Name: "second", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "second.example", Port: 80}, Status: "active"}
	if err := s.Store.SaveNode(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveNode(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := s.Config.Store.SaveState(context.Background(), "current_config", &configuration.State{NodeIDs: []string{first.ID}}); err != nil {
		t.Fatal(err)
	}
	var nodes []*ir.Node
	if err := json.Unmarshal(call(t, s, "GET", "/api/nodes", "", 200), &nodes); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(nodes))
	}
	statuses := map[string]string{}
	for _, node := range nodes {
		statuses[node.Name] = node.Status
	}
	if statuses["first"] != "active" || statuses["second"] != "idle" {
		t.Fatalf("runtime statuses = %#v", statuses)
	}
}

func TestNodeImportAPIAcceptsV2RayNAndModernLinks(t *testing.T) {
	s := apiForTest(t)
	content := `{"v":"2","ps":"v2rayn-alpha","add":"example.com","port":"443","id":"b8ca57c2-eb39-491a-8425-bba828fbad2d","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	data := call(t, s, "POST", "/api/nodes/import", fmt.Sprintf(`{"content":%q}`, content), 201)
	if !strings.Contains(string(data), `"nodes_total":1`) || !strings.Contains(string(data), `"protocol":"vmess"`) {
		t.Fatalf("V2RayN import result: %s", data)
	}
	modern := `{"content":"anytls://secret@example.com:443?sni=example.com#anytls\nmieru://user:pass@example.com:3000?transport=tcp#mieru"}`
	data = call(t, s, "POST", "/api/nodes/import", modern, 201)
	if !strings.Contains(string(data), `"nodes_total":2`) || !strings.Contains(string(data), `"protocol":"mieru"`) {
		t.Fatalf("modern import result: %s", data)
	}
	call(t, s, "POST", "/api/nodes/import", `{"content":"unsupported://example.com:443"}`, 400)
	call(t, s, "POST", "/api/nodes/import", `{"content":"{\"servers\":[]}"}`, 400)
	for _, content := range []string{
		`{"servers":[{"server":"ss.example","server_port":8388,"method":"aes-256-gcm","password":"pass"}]}`,
		"[Proxy]\nSS = ss, ss.example, 8388, encrypt-method=aes-256-gcm, password=pass",
		"mierus://alice:pass@mieru.example?profile=home&port=3000&protocol=TCP",
	} {
		data = call(t, s, "POST", "/api/nodes/import", fmt.Sprintf(`{"content":%q}`, content), 201)
		if !strings.Contains(string(data), `"nodes_total":1`) {
			t.Fatal("compatibility import returned incorrect count")
		}
	}
}

func TestConfigAndLogAPI(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	s := apiForTest(t)
	call(t, s, "GET", "/api/config/current", "", 404)
	settings := call(t, s, "GET", "/api/config", "", 200)
	call(t, s, "PUT", "/api/config", string(settings), 200)
	var settingsValue map[string]any
	if err := json.Unmarshal(settings, &settingsValue); err != nil {
		t.Fatal(err)
	}
	settingsValue["default_outbound"] = "DIRECT"
	settingsValue["dns_detour"] = "DIRECT"
	settingsValue["routing_mode"] = "blacklist"
	settings, _ = json.Marshal(settingsValue)
	call(t, s, "PUT", "/api/config", string(settings), 200)
	state := call(t, s, "POST", "/api/config/apply", `{"kernel":"sing-box"}`, 200)
	if strings.Contains(string(state), "secret") || strings.Contains(string(state), "path") || strings.Contains(string(state), "controller") {
		t.Fatalf("private state leaked: %s", state)
	}
	call(t, s, "GET", "/api/config/current", "", 200)
	call(t, s, "GET", "/api/kernel/status", "", 200)
	logs := call(t, s, "GET", "/api/kernel/logs?kernel=sing-box&limit=20", "", 200)
	if !strings.Contains(string(logs), "test log") {
		t.Fatal("logs missing")
	}
	call(t, s, "GET", "/api/kernel/logs?limit=-1", "", 400)
	call(t, s, "GET", "/api/kernel/logs?kernel=bad", "", 400)
	call(t, s, "GET", "/api/kernel/logs?level=unknown", "", 400)
	call(t, s, "GET", "/api/kernel/logs?source=unknown", "", 400)
	call(t, s, "POST", "/api/kernel/stop", "", 200)
	call(t, s, "POST", "/api/config/apply", `{"kernel":"bad"}`, 400)
	call(t, s, "POST", "/api/config/apply", `{"config":"{\"fail_validation\":true}"}`, 400)
	node := &ir.Node{Name: "selected", Protocol: ir.ProtocolShadowsocks, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8388}, Proto: map[string]any{"method": "aes-256-gcm", "password": "test"}}
	if err := s.Store.SaveNode(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	call(t, s, "POST", "/api/nodes/"+node.ID+"/activate", "", 200)
	call(t, s, "POST", "/api/kernel/stop", "", 200)
	call(t, s, "GET", "/api/config/current", "", 404)
	var nodes []*ir.Node
	if err := json.Unmarshal(call(t, s, "GET", "/api/nodes", "", 200), &nodes); err != nil {
		t.Fatal(err)
	}
	for _, value := range nodes {
		if value.ID == node.ID && value.Status == "active" {
			t.Fatal("stopped node remained active")
		}
	}
	call(t, s, "GET", "/api/missing", "", 404)
}

func TestLogWebSocketSendsSnapshotAndLiveEntries(t *testing.T) {
	s := apiForTest(t)
	runtime := s.Kernel.(*testRuntime)
	server := httptest.NewServer(s)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/logs?kernel=sing-box&search=live"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var snapshot struct {
		Type string           `json:"type"`
		Logs []map[string]any `json:"logs"`
	}
	if err := conn.ReadJSON(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Type != "snapshot" || len(snapshot.Logs) != 0 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}

	runtime.publishLog(kernel.LogEntry{Timestamp: time.Now(), Source: "stdout", Line: "live message"})
	var message struct {
		Type string         `json:"type"`
		Log  map[string]any `json:"log"`
	}
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	if message.Type != "log" || message.Log["message"] != "live message" {
		t.Fatalf("unexpected live log: %#v", message)
	}
}

func TestLogAnalysisFiltersAndClusters(t *testing.T) {
	s := apiForTest(t)
	runtime := s.Kernel.(*testRuntime)
	base := time.Date(2026, time.October, 31, 10, 0, 0, 0, time.UTC)
	runtime.logs = []kernel.LogEntry{
		{Timestamp: base, Source: "stdout", Line: "started listener on port 8080"},
		{Timestamp: base.Add(time.Second), Source: "stderr", Line: "connection failed for request 100"},
		{Timestamp: base.Add(2 * time.Second), Source: "stderr", Line: "connection failed for request 200"},
		{Timestamp: base.Add(3 * time.Second), Source: "stdout", Line: "warning: retry 3"},
		{Timestamp: base.Add(4 * time.Second), Source: "stdout", Line: "debug: retry details"},
	}
	filtered := call(t, s, "GET", "/api/kernel/logs?level=error&search=failed&limit=1", "", 200)
	var values []map[string]any
	if err := json.Unmarshal(filtered, &values); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0]["level"] != "error" || !strings.Contains(values[0]["message"].(string), "200") {
		t.Fatalf("unexpected filtered logs: %#v", values)
	}
	summary := call(t, s, "GET", "/api/kernel/logs/summary", "", 200)
	var result struct {
		Total    int            `json:"total"`
		Levels   map[string]int `json:"levels"`
		Clusters []struct {
			Count int `json:"count"`
		} `json:"clusters"`
	}
	if err := json.Unmarshal(summary, &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 5 || result.Levels["error"] != 2 || result.Levels["warn"] != 1 || len(result.Clusters) == 0 || result.Clusters[0].Count != 2 {
		t.Fatalf("unexpected log summary: %#v", result)
	}
}

func TestClassifyLogLevelTreatsInfoOnStderrAsInfo(t *testing.T) {
	entry := kernel.LogEntry{Source: "stderr", Line: "\x1b[36mINFO\x1b[0m router: clash-api listening"}
	if got := classifyLogLevel(entry); got != "info" {
		t.Fatalf("classifyLogLevel = %q, want info", got)
	}
}

func TestTrafficAPIUsesDeterministicClashMock(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer traffic-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade traffic mock: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.WriteJSON(clashapi.TrafficData{Up: 321, Down: 654}); err != nil {
			t.Errorf("write traffic mock: %v", err)
		}
	}))
	defer server.Close()

	s := apiForTest(t)
	if err := s.Config.Store.SaveState(context.Background(), "current_config", &configuration.State{
		Kernel:     ir.KernelSingBox,
		Controller: server.URL,
		Secret:     "traffic-secret",
	}); err != nil {
		t.Fatalf("save current config: %v", err)
	}

	data := call(t, s, "GET", "/api/kernel/traffic", "", 200)
	var sample clashapi.TrafficSnapshot
	if err := json.Unmarshal(data, &sample); err != nil {
		t.Fatalf("decode traffic response: %v", err)
	}
	if sample.Up != 321 || sample.Down != 654 || sample.SampledAt.IsZero() {
		t.Fatalf("unexpected traffic response: %#v", sample)
	}
	historyData := call(t, s, "GET", "/api/kernel/traffic/history?limit=10", "", 200)
	var history []clashapi.TrafficSnapshot
	if err := json.Unmarshal(historyData, &history); err != nil {
		t.Fatalf("decode traffic history: %v", err)
	}
	if len(history) != 1 || history[0].Up != 321 || history[0].Down != 654 || history[0].SampledAt.IsZero() {
		t.Fatalf("unexpected traffic history: %#v", history)
	}
	call(t, s, "GET", "/api/kernel/traffic/history?limit=0", "", 400)
}

func TestDeleteNodeRequiresRemovingSOCKSListenerAssignment(t *testing.T) {
	s := apiForTest(t)
	ctx := context.Background()
	node := &ir.Node{Name: "assigned", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
	if err := s.Store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	settings, err := s.Config.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.SOCKSListeners = []configuration.SOCKSListener{{Port: 18081, NodeID: node.ID}}
	if err := s.Config.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/nodes/"+node.ID, nil)
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("delete assigned node status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	var payload struct {
		Error *Error `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error == nil || payload.Error.Code != "SOCKS_NODE_ASSIGNED" {
		t.Fatalf("unexpected delete conflict: %#v", payload.Error)
	}
	if _, err := s.Store.Node(ctx, node.ID); err != nil {
		t.Fatalf("node was deleted despite its active port assignment: %v", err)
	}
	settings.SOCKSListeners = []configuration.SOCKSListener{}
	if err := s.Config.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveState(ctx, "current_config", &configuration.State{
		SOCKSListeners: []configuration.SOCKSListener{{Port: 18081, NodeID: node.ID}},
	}); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodDelete, "/api/nodes/"+node.ID, nil)
	response = httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("delete node with active SOCKS assignment status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	if _, err := s.Store.Node(ctx, node.ID); err != nil {
		t.Fatalf("active assignment node was deleted: %v", err)
	}

}

func TestBatchDeleteNodesIsAtomicAndValidatesIDs(t *testing.T) {
	s := apiForTest(t)
	ctx := context.Background()
	makeNode := func(name string) string {
		t.Helper()
		node := &ir.Node{Name: name, Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
		if err := s.Store.SaveNode(ctx, node); err != nil {
			t.Fatal(err)
		}
		return node.ID
	}
	first, second := makeNode("batch-first"), makeNode("batch-second")

	data := call(t, s, http.MethodPost, "/api/nodes/batch-delete", fmt.Sprintf(`{"node_ids":[%q,%q]}`, first, second), http.StatusOK)
	if !strings.Contains(string(data), `"deleted":2`) {
		t.Fatalf("unexpected batch delete response: %s", data)
	}
	if _, err := s.Store.Node(ctx, first); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("first node still exists after batch delete: %v", err)
	}
	if _, err := s.Store.Node(ctx, second); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second node still exists after batch delete: %v", err)
	}

	third, fourth := makeNode("atomic-third"), makeNode("atomic-fourth")
	call(t, s, http.MethodPost, "/api/nodes/batch-delete", fmt.Sprintf(`{"node_ids":[%q,"missing"]}`, third), http.StatusNotFound)
	if _, err := s.Store.Node(ctx, third); err != nil {
		t.Fatalf("existing node was deleted by an atomicity failure: %v", err)
	}
	call(t, s, http.MethodPost, "/api/nodes/batch-delete", fmt.Sprintf(`{"node_ids":[%q,%q]}`, fourth, fourth), http.StatusBadRequest)
	if _, err := s.Store.Node(ctx, fourth); err != nil {
		t.Fatalf("node was deleted after duplicate-ID validation failure: %v", err)
	}
	call(t, s, http.MethodPost, "/api/nodes/batch-delete", fmt.Sprintf(`{"node_ids":[%q,""]}`, third), http.StatusBadRequest)
	if _, err := s.Store.Node(ctx, third); err != nil {
		t.Fatalf("node was deleted after empty-ID validation failure: %v", err)
	}
}

func TestBatchDeleteRejectsActiveAndSOCKSAssignedNodes(t *testing.T) {
	s := apiForTest(t)
	ctx := context.Background()
	makeNode := func(name string) *ir.Node {
		t.Helper()
		node := &ir.Node{Name: name, Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
		if err := s.Store.SaveNode(ctx, node); err != nil {
			t.Fatal(err)
		}
		return node
	}
	active, assigned := makeNode("active"), makeNode("socks-assigned")
	settings, err := s.Config.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.SOCKSListeners = []configuration.SOCKSListener{{Port: 18081, NodeID: assigned.ID}}
	if err := s.Config.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveState(ctx, "current_config", &configuration.State{NodeIDs: []string{active.ID}}); err != nil {
		t.Fatal(err)
	}
	call(t, s, http.MethodPost, "/api/nodes/batch-delete", fmt.Sprintf(`{"node_ids":[%q]}`, active.ID), http.StatusConflict)
	call(t, s, http.MethodPost, "/api/nodes/batch-delete", fmt.Sprintf(`{"node_ids":[%q,%q]}`, active.ID, assigned.ID), http.StatusConflict)
	for _, node := range []*ir.Node{active, assigned} {
		if _, err := s.Store.Node(ctx, node.ID); err != nil {
			t.Fatalf("protected node %q was deleted: %v", node.ID, err)
		}
	}
}

func TestUpdateEndpointsExposeCheckAndBackgroundStatus(t *testing.T) {
	s := apiForTest(t)
	releases := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"tag_name":"v1.0.0","assets":[]}`)
	}))
	defer releases.Close()
	service := update.New("1.0.0", "full")
	service.APIBaseURL = releases.URL
	service.Client = releases.Client()
	s.Updates = service

	checked := call(t, s, http.MethodGet, "/api/update/check", "", http.StatusOK)
	if !strings.Contains(string(checked), `"latest_version":"1.0.0"`) || !strings.Contains(string(checked), `"update_available":false`) {
		t.Fatalf("unexpected update check response: %s", checked)
	}
	status := call(t, s, http.MethodGet, "/api/update/status", "", http.StatusOK)
	if !strings.Contains(string(status), `"state":"idle"`) {
		t.Fatalf("unexpected initial update status: %s", status)
	}
}

func TestTrafficAPIWithoutAppliedConfigReturnsUnavailable(t *testing.T) {
	s := apiForTest(t)
	call(t, s, "GET", "/api/kernel/traffic", "", 503)
	data := call(t, s, "GET", "/api/kernel/traffic/history", "", 200)
	var history []clashapi.TrafficSnapshot
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatalf("decode empty traffic history: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("traffic history without config = %#v, want empty", history)
	}
}

func TestChainPreviewAPI(t *testing.T) {
	if variant.Name == "lite" {
		t.Skip("full-only sing-box / cross-kernel assertions")
	}
	s := apiForTest(t)
	ctx := context.Background()
	var ids []string
	for i, owner := range []ir.KernelSet{ir.KernelSetSingBox, ir.KernelSetMihomo} {
		n := &ir.Node{Name: fmt.Sprintf("hop-%d", i), Protocol: ir.ProtocolHTTP, Owner: owner,
			Server: ir.Server{Host: "private-server.example", Port: 8088},
			Proto:  map[string]any{"username": "private-user", "password": "private-password"}}
		if err := s.Store.SaveNode(ctx, n); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, n.ID)
	}
	body, _ := json.Marshal(configuration.ApplyRequest{NodeIDs: ids})
	data := call(t, s, "POST", "/api/config/chain/preview", string(body), 200)
	var plan generator.ChainPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	if !plan.Supported || !plan.CrossKernel || len(plan.Hops) != 2 || plan.Hops[0].Kernel != ir.KernelSingBox || plan.Hops[1].Kernel != ir.KernelMihomo {
		t.Fatalf("incorrect cross-kernel preview: %s", data)
	}
	for _, secret := range []string{"private-server.example", "private-user", "private-password", "controller", "auxiliary"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("preview leaked private field %q", secret)
		}
	}
	if s.activeKernel() != "" {
		t.Fatal("preview started a kernel")
	}
	call(t, s, "GET", "/api/config/current", "", 404)
	n := &ir.Node{Name: "QUIC-exit", Protocol: ir.ProtocolHysteria2, Owner: ir.KernelSetBoth,
		Server: ir.Server{Host: "example.com", Port: 443}, Proto: map[string]any{"password": "test"},
		Security: ir.Security{Type: "tls"}}
	if err := s.Store.SaveNode(ctx, n); err != nil {
		t.Fatal(err)
	}
	body, _ = json.Marshal(configuration.ApplyRequest{NodeIDs: []string{ids[0], n.ID}})
	data = call(t, s, "POST", "/api/config/chain/preview", string(body), 200)
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Supported || !strings.Contains(plan.Reason, "UDP") {
		t.Fatalf("missing transport incompatibility reason: %s", data)
	}
	call(t, s, "POST", "/api/config/chain/preview", `{"node_ids":[]}`, 400)
}

type chainStatusRuntime struct {
	*testRuntime
	primaryRunning bool
}

func (r *chainStatusRuntime) Status() map[string]any {
	return map[string]any{
		"singbox": map[string]any{"running": true},
		"mihomo":  map[string]any{"running": r.primaryRunning},
	}
}

func TestActiveKernelUsesChainPrimary(t *testing.T) {
	s := apiForTest(t)
	runtime := &chainStatusRuntime{testRuntime: s.Kernel.(*testRuntime), primaryRunning: true}
	s.Kernel = runtime
	if err := s.Store.SaveState(context.Background(), "current_config", &configuration.State{
		Kernel: ir.KernelMihomo, Chain: &generator.ChainPlan{CrossKernel: true},
	}); err != nil {
		t.Fatal(err)
	}
	if got := s.activeKernel(); got != "mihomo" {
		t.Fatalf("active kernel = %q, want primary mihomo", got)
	}
	runtime.primaryRunning = false
	if got := s.activeKernel(); got != "" {
		t.Fatalf("auxiliary kernel reported as primary: %q", got)
	}
}
