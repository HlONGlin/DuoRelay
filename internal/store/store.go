package store

import (
	"crypto/cipher"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Store represents the database store
type Store struct {
	db            *sql.DB
	aead          cipher.AEAD
	importMu      sync.Mutex
	encryptionKey []byte
}

// Config holds database configuration
type Config struct {
	Path string
	// WAL mode for better concurrency
	WAL           bool
	EncryptionKey []byte
}

// New creates a new store instance
func New(cfg *Config) (*Store, error) {
	if cfg == nil {
		cfg = &Config{
			Path: "duorelay.db",
			WAL:  true,
		}
	}

	dsn := cfg.Path
	if dsn == "" {
		dsn = "duorelay.db"
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	dsn += separator + "_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	if cfg.WAL {
		dsn += "&_pragma=journal_mode(WAL)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	// Serialize SQLite writes and keep :memory: on its single owning connection.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}

	// Run migrations
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}
	if err := store.initEncryption(cfg); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// migrate runs database migrations
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Placeholder tables - will be created properly in P1-01
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		protocol TEXT NOT NULL,
		ir_json TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS subscriptions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		url_enc TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	INSERT OR IGNORE INTO schema_migrations(version) VALUES(1);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	return s.migrateBackend()
}

// DB returns the underlying database connection
func (s *Store) DB() *sql.DB {
	return s.db
}
