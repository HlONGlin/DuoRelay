package speedtest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/task"
)

func engineForTest(t *testing.T) *Engine {
	t.Helper()
	s, err := store.New(&store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	q := task.NewQueue(2)
	t.Cleanup(func() { q.Close(); s.Close() })
	return NewEngine(s, q)
}
func nodeAt(t *testing.T, e *Engine, address string) *ir.Node {
	t.Helper()
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := strconv.Atoi(port)
	node := &ir.Node{Name: "test", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: host, Port: p}, Proto: map[string]any{}}
	if err := e.Store.SaveNode(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	return node
}
func TestTCPAndPersistentResults(t *testing.T) {
	e := engineForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	node := nodeAt(t, e, strings.TrimPrefix(server.URL, "http://"))
	result, err := e.Test(context.Background(), node.ID, Options{})
	if err != nil || !result.Success || result.DelayMS < 0 {
		t.Fatalf("TCP test: %#v %v", result, err)
	}
	got, err := e.Store.Node(context.Background(), node.ID)
	if err != nil || got.LastTestAt.IsZero() {
		t.Fatal("measurement not stored")
	}
	results, err := e.Results(context.Background(), node.ID)
	if err != nil || len(results) != 1 || results[0].ID != result.ID {
		t.Fatalf("result history: %v %v", results, err)
	}
	server.Close()
	result, err = e.Test(context.Background(), node.ID, Options{TimeoutMS: 100})
	if err != nil || result.Success || result.DelayMS != -1 || result.Error == "" {
		t.Fatalf("failure not persisted: %#v %v", result, err)
	}
	if _, err := e.Test(context.Background(), "missing", Options{}); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestHTTPProxyProbe(t *testing.T) {
	e := engineForTest(t)
	requests := make(chan string, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests <- r.URL.String(); w.WriteHeader(204) }))
	defer proxy.Close()
	node := nodeAt(t, e, strings.TrimPrefix(proxy.URL, "http://"))
	result, err := e.Test(context.Background(), node.ID, Options{Type: "http", URL: "http://target.invalid/probe"})
	if err != nil || !result.Success {
		t.Fatalf("HTTP probe: %#v %v", result, err)
	}
	select {
	case target := <-requests:
		if target != "http://target.invalid/probe" {
			t.Fatal(target)
		}
	default:
		t.Fatal("HTTP request did not go through proxy")
	}
	e.ProbeHTTP = func(ctx context.Context, _ *ir.Node, _ string, _ time.Duration) (int64, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	result, err = e.Test(context.Background(), node.ID, Options{Type: "http", TimeoutMS: 20})
	if err != nil || result.Success || result.Error != "test timed out" {
		t.Fatalf("timeout: %#v %v", result, err)
	}
}
func TestBatchProgressAndCancellation(t *testing.T) {
	e := engineForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	node := nodeAt(t, e, strings.TrimPrefix(server.URL, "http://"))
	id, err := e.Batch(context.Background(), []string{node.ID, node.ID}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		value, err := e.Task(id)
		if err != nil {
			t.Fatal(err)
		}
		if value.Status == "completed" {
			if value.Progress != 1 || value.Total != 1 {
				t.Fatalf("bad progress: %#v", value)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("batch stuck: %#v", value)
		}
		time.Sleep(time.Millisecond)
	}
	e.ProbeHTTP = func(ctx context.Context, _ *ir.Node, _ string, _ time.Duration) (int64, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	id, err = e.Batch(context.Background(), []string{node.ID}, Options{Type: "http"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Queue.Cancel(id); err != nil {
		t.Fatal(err)
	}
	if value, _ := e.Task(id); value.Status != "cancelled" {
		t.Fatal("not cancelled")
	}
	if _, err := e.Batch(context.Background(), []string{"missing"}, Options{}); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	for _, options := range []Options{{Type: "bad"}, {TimeoutMS: -1}, {Type: "http", URL: "file:///test"}} {
		if _, err := Normalize(options); !errors.Is(err, store.ErrInvalid) {
			t.Fatal(fmt.Sprintf("invalid options accepted: %v", options))
		}
	}
}
