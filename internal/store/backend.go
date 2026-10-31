package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/ir"
)

var ErrNotFound = errors.New("record not found")
var ErrInvalid = errors.New("invalid input")
var ErrConflict = errors.New("record conflict")

func (s *Store) migrateBackend() error {
	rows, err := s.db.Query("PRAGMA table_info(subscriptions)")
	if err != nil {
		return err
	}
	hasPayload := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "payload" {
			hasPayload = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !hasPayload {
		if _, err := s.db.Exec("ALTER TABLE subscriptions ADD COLUMN payload TEXT NOT NULL DEFAULT '{}'"); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(`
	 CREATE TABLE IF NOT EXISTS subscription_nodes (
	  subscription_id TEXT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
	  node_id TEXT NOT NULL UNIQUE REFERENCES nodes(id) ON DELETE CASCADE,
	  PRIMARY KEY(subscription_id,node_id)
	 );
	 CREATE TABLE IF NOT EXISTS speedtest_results (
	  id TEXT PRIMARY KEY, node_id TEXT NOT NULL, result_json TEXT NOT NULL,
	  created_at TEXT NOT NULL
	 );
	 CREATE INDEX IF NOT EXISTS idx_speedtest_node ON speedtest_results(node_id,created_at);
	 CREATE TABLE IF NOT EXISTS backend_state (key TEXT PRIMARY KEY,value TEXT NOT NULL);
	 INSERT OR IGNORE INTO schema_migrations(version) VALUES(2);
	`)
	return err
}

func (s *Store) initEncryption(cfg *Config) error {
	key := append([]byte(nil), cfg.EncryptionKey...)
	if len(key) == 0 {
		if cfg.Path == ":memory:" || strings.Contains(cfg.Path, "mode=memory") {
			key = make([]byte, 32)
			if _, err := rand.Read(key); err != nil {
				return err
			}
		} else {
			path := cfg.Path
			if path == "" {
				path = "duorelay.db"
			}
			// URI DSNs require an explicit key: they do not name an ordinary file.
			if strings.HasPrefix(path, "file:") || strings.Contains(path, "?") {
				return fmt.Errorf("database URI requires EncryptionKey")
			}
			var err error
			key, err = os.ReadFile(path + ".key")
			if errors.Is(err, os.ErrNotExist) {
				key = make([]byte, 32)
				if _, err = rand.Read(key); err != nil {
					return err
				}
				file, createErr := os.OpenFile(path+".key", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if errors.Is(createErr, os.ErrExist) {
					key, err = os.ReadFile(path + ".key")
				} else if createErr != nil {
					return createErr
				} else {
					_, err = file.Write(key)
					closeErr := file.Close()
					if err == nil {
						err = closeErr
					}
				}
			}
			if err != nil {
				return fmt.Errorf("subscription encryption key: %w", err)
			}
		}
	}
	if len(key) != 32 {
		return fmt.Errorf("EncryptionKey must contain 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	s.aead, err = cipher.NewGCM(block)
	return err
}

func (s *Store) Encrypt(plain string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return "v1:" + base64.RawStdEncoding.EncodeToString(s.aead.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func (s *Store) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "v1:") {
		return "", fmt.Errorf("unsupported encrypted URL format")
	}
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, "v1:"))
	if err != nil || len(data) < s.aead.NonceSize() {
		return "", fmt.Errorf("invalid encrypted URL")
	}
	n := s.aead.NonceSize()
	plain, err := s.aead.Open(nil, data[:n], data[n:], nil)
	return string(plain), err
}

func (s *Store) SaveNode(ctx context.Context, n *ir.Node) error {
	if n == nil || strings.TrimSpace(n.Name) == "" || !ir.IsValidProtocol(n.Protocol) || strings.TrimSpace(n.Server.Host) == "" || n.Server.Port < 1 || n.Server.Port > 65535 {
		return fmt.Errorf("%w: invalid node", ErrInvalid)
	}
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	n.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO nodes(id,name,protocol,ir_json) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,protocol=excluded.protocol,ir_json=excluded.ir_json,updated_at=CURRENT_TIMESTAMP`, n.ID, n.Name, n.Protocol, string(data))
	return err
}

func (s *Store) Node(ctx context.Context, id string) (*ir.Node, error) {
	var data string
	err := s.db.QueryRowContext(ctx, "SELECT ir_json FROM nodes WHERE id=?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var node ir.Node
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return nil, err
	}
	return &node, nil
}

func (s *Store) Nodes(ctx context.Context, subscriptionID string) ([]*ir.Node, error) {
	query := "SELECT n.ir_json FROM nodes n"
	var args []any
	if subscriptionID != "" {
		query += " JOIN subscription_nodes sn ON sn.node_id=n.id WHERE sn.subscription_id=?"
		args = append(args, subscriptionID)
	}
	query += " ORDER BY n.id"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []*ir.Node{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var node ir.Node
		if err := json.Unmarshal([]byte(data), &node); err != nil {
			return nil, err
		}
		nodes = append(nodes, &node)
	}
	return nodes, rows.Err()
}

func (s *Store) SaveState(ctx context.Context, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO backend_state(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, string(data))
	return err
}

func (s *Store) State(ctx context.Context, key string, value any) error {
	var data string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM backend_state WHERE key=?", key).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), value)
}
