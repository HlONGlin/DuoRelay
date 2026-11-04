package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/variant"
)

func TestIssueNodePartialEditAndImportReport(t *testing.T) {
	s := apiForTest(t)
	ctx := context.Background()
	node := &ir.Node{Name: "before", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 5000}, Proto: map[string]any{"password": "kept"}}
	if err := s.Store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	call(t, s, "PUT", "/api/nodes/"+node.ID, `{"name":"after","server":{"port":5001}}`, 200)
	saved, err := s.Store.Node(ctx, node.ID)
	if err != nil || saved.Server.Host != "127.0.0.1" || saved.Server.Port != 5001 || saved.Proto["password"] != "kept" || saved.Name != "after" {
		t.Fatal("partial update lost existing fields", err)
	}
	call(t, s, "PUT", "/api/nodes/"+node.ID, `{"server":{"port":-1}}`, 400)
	call(t, s, "PUT", "/api/nodes/"+node.ID, `{"protocol":"vless"}`, 400)
	call(t, s, "PUT", "/api/nodes/"+node.ID, `{"name":null}`, 400)
	content := `{"content":"socks5://127.0.0.1:40000#one\nsocks5://127.0.0.1:40000#two\nunknown://secret@host:443"}`
	var report struct {
		Added      int   `json:"added"`
		Duplicates int   `json:"duplicates"`
		Skipped    []any `json:"skipped"`
	}
	json.Unmarshal(call(t, s, "POST", "/api/nodes/import", content, 201), &report)
	if report.Added != 1 || report.Duplicates != 1 || len(report.Skipped) != 1 {
		t.Fatal("bad import report", report)
	}
	json.Unmarshal(call(t, s, "POST", "/api/nodes/import", content, 201), &report)
	if report.Added != 0 || report.Duplicates != 2 {
		t.Fatal("reimport duplicated nodes", report)
	}
}

func TestIssueLogDefaultLimitAndPersistentPolicy(t *testing.T) {
	s := apiForTest(t)
	runtime := s.Kernel.(*testRuntime)
	for i := 0; i < 150; i++ {
		runtime.logs = append(runtime.logs, kernel.LogEntry{Timestamp: time.Now(), Source: "stdout", Line: fmt.Sprintf("INFO entry %d", i)})
	}
	var logs []map[string]any
	if err := json.Unmarshal(call(t, s, "GET", "/api/kernel/logs", "", 200), &logs); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 100 {
		t.Fatal("default log limit is not 100", len(logs))
	}
	manager := kernel.NewManager("")
	defer manager.CloseLogs()
	if err := manager.SetLogDirectory(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	s.Kernel = manager
	call(t, s, "PUT", "/api/kernel/logs/policy", `{"retention_days":3,"cleanup_hours":2}`, 200)
	policy := call(t, s, "GET", "/api/kernel/logs/policy", "", 200)
	if !strings.Contains(string(policy), `"cleanup_hours":2`) {
		t.Fatal("policy did not apply")
	}
	var persisted kernel.LogPolicy
	if err := s.Store.State(context.Background(), "kernel_log_policy", &persisted); err != nil || persisted.RetentionDays != 3 {
		t.Fatal("policy not persisted", err)
	}
	call(t, s, "PUT", "/api/kernel/logs/policy", `{"retention_days":0,"cleanup_hours":-1}`, 400)
	call(t, s, "DELETE", "/api/kernel/logs?kernel="+variant.DefaultKernel, "", 200)
}
