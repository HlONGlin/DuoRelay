package auth

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/holi/duorelay/internal/store"
)

func TestBackupRestoreAndPasswordReset(t *testing.T) {
	ctx := context.Background()
	db, err := store.New(&store.Config{Path: filepath.Join(t.TempDir(), "app.db"), WAL: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	auth := New(db)
	if err := auth.Register(ctx, "admin", "old-password"); err != nil {
		t.Fatal(err)
	}
	token, err := auth.Login(ctx, "admin", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := db.Encrypt("subscription-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(ctx, "encrypted-test", cipher); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(ctx, "current_config", map[string]string{"path": "/original-host/config.json"}); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "backup")
	if err := db.Backup(ctx, destination); err != nil {
		t.Fatal(err)
	}
	if err := db.Backup(ctx, destination); err == nil {
		t.Fatal("backup overwrote an existing directory")
	}
	restored, err := store.New(&store.Config{Path: filepath.Join(destination, "duorelay.db"), WAL: true})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var runtimeState map[string]string
	if err := restored.State(ctx, "current_config", &runtimeState); err == nil {
		t.Fatal("backup retained runtime paths from the original host")
	}
	if err := db.State(ctx, "current_config", &runtimeState); err != nil {
		t.Fatal("backup changed the live runtime state", err)
	}
	var encrypted string
	if err := restored.State(ctx, "encrypted-test", &encrypted); err != nil {
		t.Fatal(err)
	}
	if plain, err := restored.Decrypt(encrypted); err != nil || plain != "subscription-secret" {
		t.Fatal("backup lost database/key consistency")
	}
	if err := auth.ResetPassword(ctx, "admin", "new-password"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if _, err := auth.Authenticate(ctx, request); err == nil {
		t.Fatal("old session survived password reset")
	}
	if _, err := auth.Login(ctx, "admin", "old-password"); err == nil {
		t.Fatal("old password accepted")
	}
	if _, err := auth.Login(ctx, "admin", "new-password"); err != nil {
		t.Fatal(err)
	}
}
