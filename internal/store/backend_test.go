package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
)

func TestTrafficSamplesRoundTrip(t *testing.T) {
	s, err := New(&Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := s.SaveTrafficSample(context.Background(), TrafficSample{Kernel: "sing-box", Up: 12, Down: 34, SampledAt: now}); err != nil {
		t.Fatal(err)
	}
	values, err := s.TrafficSamples(context.Background(), "sing-box", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Up != 12 || values[0].Down != 34 || !values[0].SampledAt.Equal(now) {
		t.Fatalf("unexpected traffic samples: %#v", values)
	}
}

func TestEncryptionPersistenceAndMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec("CREATE TABLE subscriptions(id TEXT PRIMARY KEY,name TEXT NOT NULL,url_enc TEXT NOT NULL,created_at DATETIME,updated_at DATETIME)"); err != nil {
		t.Fatal(err)
	}
	legacy.Close()
	s, err := New(&Config{Path: path, WAL: true})
	if err != nil {
		t.Fatal(err)
	}
	plain := "https://subscription.invalid/token?secret=private"
	value, err := s.Encrypt(plain)
	if err != nil || strings.Contains(value, "private") {
		t.Fatalf("not encrypted: %s %v", value, err)
	}
	if _, err := s.DB().Exec("INSERT INTO subscriptions(id,name,url_enc) VALUES('legacy','legacy',?)", value); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveState(context.Background(), "test", map[string]int{"value": 7}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteState(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	var deleted map[string]int
	if err := s.State(context.Background(), "test", &deleted); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted state still exists: %v", err)
	}
	if err := s.DeleteState(context.Background(), "test"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing state delete returned %v", err)
	}
	if err := s.SaveState(context.Background(), "test", map[string]int{"value": 7}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = New(&Config{Path: path, WAL: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Decrypt(value)
	if err != nil || got != plain {
		t.Fatalf("key did not persist: %s %v", got, err)
	}
	var state map[string]int
	if err := s.State(context.Background(), "test", &state); err != nil || state["value"] != 7 {
		t.Fatalf("state not persisted: %v %v", state, err)
	}
	if key, err := os.ReadFile(path + ".key"); err != nil || len(key) != 32 {
		t.Fatalf("invalid key file: %v", err)
	}
	if _, err := s.Decrypt("v1:broken"); err == nil {
		t.Fatal("invalid ciphertext accepted")
	}
	if _, err := s.Decrypt("plaintext"); err == nil {
		t.Fatal("plaintext accepted")
	}
}
func TestNodesAndMemoryStore(t *testing.T) {
	s, err := New(&Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	n := &ir.Node{Name: "manual", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 80}, Source: ir.Source{Type: "manual"}}
	if err := s.SaveNode(ctx, n); err != nil {
		t.Fatal(err)
	}
	n.Name = "edited"
	if err := s.SaveNode(ctx, n); err != nil {
		t.Fatal(err)
	}
	got, err := s.Node(ctx, n.ID)
	if err != nil || got.Name != "edited" {
		t.Fatalf("bad node: %v %v", got, err)
	}
	if nodes, err := s.Nodes(ctx, ""); err != nil || len(nodes) != 1 {
		t.Fatalf("bad list: %v %v", nodes, err)
	}
	if _, err := s.Node(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.SaveNode(ctx, &ir.Node{}); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid node accepted")
	}
	if err := s.State(ctx, "missing", &got); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestMeshNodesDoNotRequireRemoteSocket(t *testing.T) {
	s, err := New(&Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, p := range []string{ir.ProtocolTailscale, ir.ProtocolEasyTier, ir.ProtocolZeroTier, ir.ProtocolRematch} {
		n := &ir.Node{Name: p, Protocol: p, Owner: ir.GetProtocolOwner(p)}
		if err := s.SaveNode(context.Background(), n); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if stored, err := s.Node(context.Background(), n.ID); err != nil || stored.Protocol != p {
			t.Fatal("mesh endpoint lost")
		}
	}
	for _, n := range []*ir.Node{
		{Name: "HTTP", Protocol: ir.ProtocolHTTP},
		{Name: "incomplete mesh address", Protocol: ir.ProtocolTailscale, Server: ir.Server{Host: "127.0.0.1"}},
	} {
		if err := s.SaveNode(context.Background(), n); !errors.Is(err, ErrInvalid) {
			t.Fatal("incomplete socket accepted")
		}
	}
}

func TestNodesQueryPushesFiltersAndPagination(t *testing.T) {
	s, err := New(&Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, node := range []*ir.Node{
		{Name: "alpha", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "alpha.example", Port: 80}},
		{Name: "beta", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "beta.example", Port: 81}},
		{Name: "gamma", Protocol: ir.ProtocolSOCKS5, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "gamma.example", Port: 1080}},
	} {
		if err := s.SaveNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	values, err := s.NodesQuery(ctx, NodeListQuery{Protocol: ir.ProtocolHTTP, Limit: 1, Offset: 1})
	if err != nil || len(values) != 1 || values[0].Protocol != ir.ProtocolHTTP {
		t.Fatalf("unexpected paged protocol query: %v %v", values, err)
	}
	values, err = s.NodesQuery(ctx, NodeListQuery{Search: "beta.example"})
	if err != nil || len(values) != 1 || values[0].Name != "beta" {
		t.Fatalf("unexpected search query: %v %v", values, err)
	}
	values, err = s.NodesQuery(ctx, NodeListQuery{Search: "%"})
	if err != nil || len(values) != 0 {
		t.Fatalf("wildcard search was not escaped: %v %v", values, err)
	}
}

func TestInvalidEncryptionKey(t *testing.T) {
	if s, err := New(&Config{Path: ":memory:", EncryptionKey: []byte("short")}); err == nil {
		s.Close()
		t.Fatal("short key accepted")
	}
}
