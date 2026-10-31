package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/configuration"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/speedtest"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/subscription"
	"github.com/holi/duorelay/internal/task"
)

type testRuntime struct{ running string }

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
	runtime := &testRuntime{}
	config := configuration.New(s, runtime, t.TempDir())
	return NewServer(s, subscription.NewManager(s), speedtest.NewEngine(s, q), config, runtime)
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
	call(t, s, "DELETE", "/api/nodes/"+node.ID, "", 200)
	call(t, s, "DELETE", "/api/nodes/"+node.ID, "", 404)
}

func TestConfigAndLogAPI(t *testing.T) {
	s := apiForTest(t)
	call(t, s, "GET", "/api/config/current", "", 404)
	settings := call(t, s, "GET", "/api/config", "", 200)
	call(t, s, "PUT", "/api/config", string(settings), 200)
	state := call(t, s, "POST", "/api/config/apply", `{"kernel":"sing-box","config":"{\"outbounds\":[{\"type\":\"direct\"}]}"}`, 200)
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
	call(t, s, "POST", "/api/kernel/stop", "", 200)
	call(t, s, "POST", "/api/config/apply", `{"kernel":"bad"}`, 400)
	call(t, s, "POST", "/api/config/apply", `{"config":"{\"fail_validation\":true}"}`, 422)
	node := &ir.Node{Name: "selected", Protocol: ir.ProtocolShadowsocks, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8388}, Proto: map[string]any{"method": "aes-256-gcm", "password": "test"}}
	if err := s.Store.SaveNode(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	call(t, s, "POST", "/api/nodes/"+node.ID+"/activate", "", 200)
	call(t, s, "GET", "/api/missing", "", 404)
}
