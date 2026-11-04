package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Backup uses SQLite's snapshot mechanism, so a running WAL database need not
// be stopped or copied unsafely. The associated encryption key is immutable.
func (s *Store) Backup(ctx context.Context, destination string) (err error) {
	if len(s.encryptionKey) != 32 {
		return fmt.Errorf("encryption key is unavailable")
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	if err = os.Mkdir(destination, 0700); err != nil {
		return fmt.Errorf("backup requires a new destination directory: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(destination)
		}
	}()
	database := filepath.Join(destination, "duorelay.db")
	if _, err = s.db.ExecContext(ctx, "VACUUM INTO ?", database); err != nil {
		return err
	}
	if err = os.Chmod(database, 0600); err != nil {
		return err
	}
	// Runtime state contains absolute config paths and host system-proxy
	// snapshots. A restored backup must apply fresh configs on its target host.
	snapshot, err := sql.Open("sqlite", database)
	if err != nil {
		return err
	}
	_, clearErr := snapshot.ExecContext(ctx, "DELETE FROM backend_state WHERE key='current_config'")
	closeErr := snapshot.Close()
	if clearErr != nil {
		return clearErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.WriteFile(database+".key", s.encryptionKey, 0600); err != nil {
		return err
	}
	manifest := map[string]any{"created_at": time.Now().UTC(), "format": 1, "requires_apply": true}
	for _, name := range []string{"duorelay.db", "duorelay.db.key"} {
		data, readErr := os.ReadFile(filepath.Join(destination, name))
		if readErr != nil {
			return readErr
		}
		digest := sha256.Sum256(data)
		manifest[name+".sha256"] = hex.EncodeToString(digest[:])
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(destination, "manifest.json"), data, 0600); err != nil {
		return err
	}
	complete = true
	return nil
}

func (s *Store) ResetAuthPassword(ctx context.Context, username, hash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE auth_users SET password_hash=? WHERE username=?", hash, username)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM auth_sessions WHERE username=?", username); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateAuthSessionForPassword(ctx context.Context, tokenHash, username, passwordHash string, expiresAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `INSERT INTO auth_sessions(token_hash,username,expires_at,created_at)
		SELECT ?,username,?,? FROM auth_users WHERE username=? AND password_hash=?`, tokenHash, expiresAt.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), username, passwordHash)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrConflict
	}
	return nil
}
