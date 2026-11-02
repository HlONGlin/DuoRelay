package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/holi/duorelay/internal/store"
)

func TestRegisterLoginAndSession(t *testing.T) {
	db, err := store.New(&store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	status, err := s.Status(ctx, httptest.NewRequest("GET", "/api/auth/status", nil))
	if err != nil || status.Configured {
		t.Fatalf("initial status: %#v %v", status, err)
	}
	if err := s.Register(ctx, "admin", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	token, err := s.Login(ctx, "admin", "correct-horse-battery")
	if err != nil || token == "" {
		t.Fatalf("login: %v", err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: token})
	if username, err := s.Authenticate(ctx, req); err != nil || username != "admin" {
		t.Fatalf("authenticate: %q %v", username, err)
	}
}
