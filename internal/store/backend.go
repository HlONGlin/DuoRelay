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
	 CREATE TABLE IF NOT EXISTS traffic_samples (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  kernel TEXT NOT NULL,
	  up INTEGER NOT NULL,
	  down INTEGER NOT NULL,
	  sampled_at TEXT NOT NULL
	 );
	 CREATE INDEX IF NOT EXISTS idx_traffic_samples_kernel_time ON traffic_samples(kernel, sampled_at DESC);
	 CREATE TABLE IF NOT EXISTS auth_users (
	  id TEXT PRIMARY KEY,
	  username TEXT NOT NULL UNIQUE,
	  password_hash TEXT NOT NULL,
	  created_at TEXT NOT NULL
	 );
	 CREATE TABLE IF NOT EXISTS auth_sessions (
	  token_hash TEXT PRIMARY KEY,
	  username TEXT NOT NULL REFERENCES auth_users(username) ON DELETE CASCADE,
	  expires_at TEXT NOT NULL,
	  created_at TEXT NOT NULL
	 );
	 CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions(expires_at);
	 CREATE INDEX IF NOT EXISTS idx_nodes_protocol ON nodes(protocol);
	 CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
	 CREATE INDEX IF NOT EXISTS idx_nodes_updated_at ON nodes(updated_at DESC);
	 INSERT OR IGNORE INTO schema_migrations(version) VALUES(2);
	`)
	return err
}

type TrafficSample struct {
	Kernel    string
	Up        int64
	Down      int64
	SampledAt time.Time
}

func (s *Store) SaveTrafficSample(ctx context.Context, sample TrafficSample) error {
	if sample.Kernel == "" || sample.Up < 0 || sample.Down < 0 || sample.SampledAt.IsZero() {
		return fmt.Errorf("%w: invalid traffic sample", ErrInvalid)
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO traffic_samples(kernel,up,down,sampled_at) VALUES(?,?,?,?)",
		sample.Kernel, sample.Up, sample.Down, sample.SampledAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	// Keep a bounded history per kernel. This is a rolling operational history,
	// not an accounting ledger.
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM traffic_samples
		WHERE kernel=? AND id NOT IN (
			SELECT id FROM traffic_samples WHERE kernel=? ORDER BY sampled_at DESC, id DESC LIMIT 20000
		)`, sample.Kernel, sample.Kernel)
	return err
}

func (s *Store) TrafficSamples(ctx context.Context, kernel string, limit int) ([]TrafficSample, error) {
	if kernel == "" {
		return nil, fmt.Errorf("%w: kernel is required", ErrInvalid)
	}
	if limit < 1 || limit > 20000 {
		return nil, fmt.Errorf("%w: limit must be 1..20000", ErrInvalid)
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT kernel,up,down,sampled_at FROM traffic_samples WHERE kernel=? ORDER BY sampled_at DESC, id DESC LIMIT ?",
		kernel, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]TrafficSample, 0, limit)
	for rows.Next() {
		var sample TrafficSample
		var sampledAt string
		if err := rows.Scan(&sample.Kernel, &sample.Up, &sample.Down, &sampledAt); err != nil {
			return nil, err
		}
		sample.SampledAt, err = time.Parse(time.RFC3339Nano, sampledAt)
		if err != nil {
			return nil, err
		}
		result = append(result, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result, nil
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
	if err == nil {
		s.encryptionKey = append([]byte(nil), key...)
	}
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
	if n == nil || strings.TrimSpace(n.Name) == "" || (!ir.IsValidProtocol(n.Protocol) && !n.IsComposition()) {
		return fmt.Errorf("%w: invalid node", ErrInvalid)
	}
	if n.IsComposition() {
		if n.Composition == nil || n.Server.Host != "" || n.Server.Port != 0 || len(n.Composition.NodeIDs) < 2 {
			return fmt.Errorf("%w: invalid composition node", ErrInvalid)
		}
		if (n.Protocol == ir.ProtocolProxyChain && n.Composition.Mode != "chain") || (n.Protocol == ir.ProtocolLoadBalance && n.Composition.Mode != "load_balance") {
			return fmt.Errorf("%w: composition type does not match its mode", ErrInvalid)
		}
		limit := 64
		if n.Composition.Mode == "chain" {
			limit = 8
		}
		if len(n.Composition.NodeIDs) > limit || n.Owner == 0 {
			return fmt.Errorf("%w: invalid composition node", ErrInvalid)
		}
		if n.Composition.Kernel != ir.KernelSingBox && n.Composition.Kernel != ir.KernelMihomo {
			return fmt.Errorf("%w: invalid composition kernel", ErrInvalid)
		}
		if n.Protocol == ir.ProtocolLoadBalance && n.Composition.Kernel != ir.KernelMihomo {
			return fmt.Errorf("%w: load balancing requires mihomo", ErrInvalid)
		}
		if n.Composition.Kernel == ir.KernelSingBox && n.Owner != ir.KernelSetSingBox || n.Composition.Kernel == ir.KernelMihomo && n.Owner != ir.KernelSetMihomo {
			return fmt.Errorf("%w: composition kernel does not match node ownership", ErrInvalid)
		}
		seen := make(map[string]bool, len(n.Composition.NodeIDs))
		for _, id := range n.Composition.NodeIDs {
			if strings.TrimSpace(id) == "" || seen[id] {
				return fmt.Errorf("%w: invalid composition node members", ErrInvalid)
			}
			seen[id] = true
		}
		if n.Protocol == ir.ProtocolLoadBalance && n.Composition.LoadBalance == nil || n.Protocol == ir.ProtocolProxyChain && n.Composition.LoadBalance != nil {
			return fmt.Errorf("%w: invalid composition options", ErrInvalid)
		}
	} else if n.Composition != nil {
		return fmt.Errorf("%w: native nodes cannot contain a composition", ErrInvalid)
	}
	// Mesh endpoints are identified by network/auth configuration rather than
	// a single remote socket. They legitimately have no server host or port.
	virtual := n.IsComposition() || n.Protocol == ir.ProtocolTailscale || n.Protocol == ir.ProtocolEasyTier || n.Protocol == ir.ProtocolZeroTier || n.Protocol == ir.ProtocolRematch
	if !(virtual && n.Server.Host == "" && n.Server.Port == 0) && (strings.TrimSpace(n.Server.Host) == "" || n.Server.Port < 1 || n.Server.Port > 65535) {
		return fmt.Errorf("%w: invalid node", ErrInvalid)
	}
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.Protocol == ir.ProtocolVLESS && n.Transport.Type == "xhttp" {
		n.Owner = ir.KernelSetMihomo
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

// FindNodeByFingerprint returns an existing node with the same connection
// parameters, ignoring its display and management fields.
func (s *Store) FindNodeByFingerprint(ctx context.Context, candidate *ir.Node) (*ir.Node, error) {
	want := nodeFingerprint(candidate)
	rows, err := s.db.QueryContext(ctx, "SELECT ir_json FROM nodes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var node ir.Node
		if err := json.Unmarshal([]byte(data), &node); err != nil {
			continue
		}
		if nodeFingerprint(&node) == want {
			return &node, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func nodeFingerprint(n *ir.Node) string {
	return ir.ConnectionFingerprint(n)
}

// ImportNode serializes the duplicate check and insert for concurrent imports.
// Manual creation remains free to create independent nodes intentionally.
func (s *Store) ImportNode(ctx context.Context, node *ir.Node) (bool, error) {
	s.importMu.Lock()
	defer s.importMu.Unlock()
	if existing, err := s.FindNodeByFingerprint(ctx, node); err != nil {
		return false, err
	} else if existing != nil {
		return false, nil
	}
	return true, s.SaveNode(ctx, node)
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
	return s.NodesQuery(ctx, NodeListQuery{SubscriptionID: subscriptionID})
}

// NodeListQuery describes the filters pushed down to SQLite for node listing.
// A zero Limit keeps the historical unbounded behavior.
type NodeListQuery struct {
	SubscriptionID string
	Protocol       string
	Search         string
	Limit          int
	Offset         int
}

// NodesQuery avoids loading and filtering the entire node table in Go.
func (s *Store) NodesQuery(ctx context.Context, filter NodeListQuery) ([]*ir.Node, error) {
	query := "SELECT n.ir_json FROM nodes n"
	var args []any
	conditions := []string{}
	if filter.SubscriptionID != "" {
		query += " JOIN subscription_nodes sn ON sn.node_id=n.id"
		conditions = append(conditions, "sn.subscription_id=?")
		args = append(args, filter.SubscriptionID)
	}
	if filter.Protocol != "" {
		conditions = append(conditions, "n.protocol=?")
		args = append(args, filter.Protocol)
	}
	if filter.Search != "" {
		pattern := "%" + escapeLike(strings.ToLower(filter.Search)) + "%"
		conditions = append(conditions, "(lower(n.name) LIKE ? ESCAPE '\\' OR lower(n.ir_json) LIKE ? ESCAPE '\\')")
		args = append(args, pattern, pattern)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY n.updated_at DESC, n.id"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}
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

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "%", `\%`)
	return strings.ReplaceAll(value, "_", `\_`)
}

func (s *Store) SaveState(ctx context.Context, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO backend_state(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, string(data))
	return err
}

func (s *Store) DeleteState(ctx context.Context, key string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM backend_state WHERE key=?", key)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return ErrNotFound
	}
	return nil
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
