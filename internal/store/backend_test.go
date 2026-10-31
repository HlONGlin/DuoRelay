package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/ir"
)

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
func TestInvalidEncryptionKey(t *testing.T) {
	if s, err := New(&Config{Path: ":memory:", EncryptionKey: []byte("short")}); err == nil {
		s.Close()
		t.Fatal("short key accepted")
	}
}
