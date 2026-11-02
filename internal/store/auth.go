package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) AuthUserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM auth_users").Scan(&count)
	return count, err
}

func (s *Store) CreateAuthUser(ctx context.Context, id, username, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM auth_users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: account already configured", ErrConflict)
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO auth_users(id,username,password_hash,created_at) VALUES(?,?,?,?)",
		id, username, passwordHash, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("%w: username already exists", ErrConflict)
		}
		return err
	}
	return tx.Commit()
}

func (s *Store) AuthPasswordHash(ctx context.Context, username string) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, "SELECT password_hash FROM auth_users WHERE username=?", username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

func (s *Store) CreateAuthSession(ctx context.Context, tokenHash, username string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO auth_sessions(token_hash,username,expires_at,created_at) VALUES(?,?,?,?)",
		tokenHash, username, expiresAt.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) AuthSessionUser(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	var username, expiresText string
	err := s.db.QueryRowContext(ctx, "SELECT username,expires_at FROM auth_sessions WHERE token_hash=?", tokenHash).Scan(&username, &expiresText)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expiresText)
	if err != nil || !expiresAt.After(now) {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM auth_sessions WHERE token_hash=?", tokenHash)
		return "", ErrNotFound
	}
	return username, nil
}

func (s *Store) DeleteAuthSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM auth_sessions WHERE token_hash=?", tokenHash)
	return err
}

func (s *Store) DeleteExpiredAuthSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM auth_sessions WHERE expires_at<=?", now.UTC().Format(time.RFC3339Nano))
	return err
}
