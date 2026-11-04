package subscription

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/defaults"
	"github.com/holi/duorelay/internal/store"
)

var ErrUnavailable = errors.New("subscription store is not configured")
var ErrUpdate = errors.New("subscription update failed")
var ErrFetchURL = errors.New("subscription URL error")
var ErrFetchNetwork = errors.New("subscription network error")
var ErrFetchDNS = errors.New("subscription DNS error")
var ErrFetchTimeout = errors.New("subscription timeout")
var ErrFetchTLS = errors.New("subscription TLS error")
var ErrFetchHTTP = errors.New("subscription HTTP error")
var ErrFetchBody = errors.New("subscription response error")
var ErrFetchParse = errors.New("subscription parse error")

type Diff struct {
	Added      int `json:"added"`
	Removed    int `json:"removed"`
	Changed    int `json:"changed"`
	Duplicates int `json:"duplicates"`
}

type Subscription struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	URL         string      `json:"url"`
	UserAgent   string      `json:"user_agent"`
	Interval    int         `json:"interval"` // hours; zero on create means 24 hours
	Group       string      `json:"group"`
	Disabled    bool        `json:"disabled"`
	LastUpdate  time.Time   `json:"last_update"`
	LastAttempt time.Time   `json:"last_attempt"`
	LastStatus  string      `json:"last_status"`
	LastError   string      `json:"last_error,omitempty"`
	NextUpdate  time.Time   `json:"next_update"`
	Failures    int         `json:"failures"`
	Diff        Diff        `json:"diff"`
	Skipped     []ParseSkip `json:"skipped,omitempty"`
	Used        int64       `json:"used"`
	Total       int64       `json:"total"`
	Expire      time.Time   `json:"expire"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Manager borrows Store; its owner closes the database after Run has stopped.
// Client and Registry may be configured before requests/scheduling begin.
type Manager struct {
	Store    *store.Store
	Client   *http.Client
	Registry *parser.Registry
	locks    sync.Map
}

func NewManager(stores ...*store.Store) *Manager {
	m := &Manager{Client: &http.Client{Timeout: 30 * time.Second}, Registry: defaults.New()}
	if len(stores) > 0 {
		m.Store = stores[0]
	}
	return m
}
func (m *Manager) lock(id string) func() {
	value, _ := m.locks.LoadOrStore(id, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
func (m *Manager) ready() error {
	if m.Store == nil {
		return ErrUnavailable
	}
	return nil
}
func validate(sub *Subscription) error {
	if sub == nil || strings.TrimSpace(sub.Name) == "" {
		return fmt.Errorf("%w: name is required", store.ErrInvalid)
	}
	u, err := url.Parse(sub.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("%w: URL must use HTTP(S), without credentials or fragment", store.ErrInvalid)
	}
	if sub.Interval < 1 || sub.Interval > 8760 {
		return fmt.Errorf("%w: interval must be 1..8760 hours", store.ErrInvalid)
	}
	if strings.ContainsAny(sub.UserAgent, "\r\n") {
		return fmt.Errorf("%w: invalid user agent", store.ErrInvalid)
	}
	return nil
}
func payload(sub *Subscription) ([]byte, error) {
	copy := *sub
	copy.URL = ""
	return json.Marshal(copy)
}

func (m *Manager) Add(ctx context.Context, sub *Subscription) error {
	if err := m.ready(); err != nil {
		return err
	}
	if sub == nil {
		return fmt.Errorf("%w: missing subscription", store.ErrInvalid)
	}
	if sub.Interval == 0 {
		sub.Interval = 24
	}
	if err := validate(sub); err != nil {
		return err
	}
	// IDs and operational metadata are always assigned by the service.
	sub.ID = uuid.NewString()
	now := time.Now().UTC()
	*sub = Subscription{ID: sub.ID, Name: sub.Name, URL: sub.URL, UserAgent: sub.UserAgent, Interval: sub.Interval, Group: sub.Group, Disabled: sub.Disabled, CreatedAt: now, UpdatedAt: now, NextUpdate: now, LastStatus: "pending"}
	encrypted, err := m.Store.Encrypt(sub.URL)
	if err != nil {
		return err
	}
	data, err := payload(sub)
	if err != nil {
		return err
	}
	_, err = m.Store.DB().ExecContext(ctx, "INSERT INTO subscriptions(id,name,url_enc,payload) VALUES(?,?,?,?)", sub.ID, sub.Name, encrypted, string(data))
	return err
}

func (m *Manager) Get(ctx context.Context, id string) (*Subscription, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	var name, encrypted, data string
	err := m.Store.DB().QueryRowContext(ctx, "SELECT name,url_enc,payload FROM subscriptions WHERE id=?", id).Scan(&name, &encrypted, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var sub Subscription
	if err := json.Unmarshal([]byte(data), &sub); err != nil {
		return nil, err
	}
	sub.ID, sub.Name = id, name
	sub.URL, err = m.Store.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}
func (m *Manager) List(ctx context.Context, group string) ([]*Subscription, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	rows, err := m.Store.DB().QueryContext(ctx, "SELECT id,name,url_enc,payload FROM subscriptions ORDER BY name,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*Subscription{}
	for rows.Next() {
		var id, name, encrypted, data string
		if err := rows.Scan(&id, &name, &encrypted, &data); err != nil {
			return nil, err
		}
		var sub Subscription
		if err := json.Unmarshal([]byte(data), &sub); err != nil {
			return nil, err
		}
		sub.ID, sub.Name = id, name
		sub.URL, err = m.Store.Decrypt(encrypted)
		if err != nil {
			return nil, err
		}
		if group == "" || sub.Group == group {
			result = append(result, &sub)
		}
	}
	return result, rows.Err()
}
func (m *Manager) Groups(ctx context.Context) ([]string, error) {
	subs, err := m.List(ctx, "")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, sub := range subs {
		if sub.Group != "" {
			set[sub.Group] = true
		}
	}
	groups := []string{}
	for group := range set {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups, nil
}
func (m *Manager) save(ctx context.Context, sub *Subscription) error {
	encrypted, err := m.Store.Encrypt(sub.URL)
	if err != nil {
		return err
	}
	data, err := payload(sub)
	if err != nil {
		return err
	}
	result, err := m.Store.DB().ExecContext(ctx, "UPDATE subscriptions SET name=?,url_enc=?,payload=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", sub.Name, encrypted, string(data), sub.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return store.ErrNotFound
	}
	return err
}
func (m *Manager) Edit(ctx context.Context, id string, input *Subscription) (*Subscription, error) {
	unlock := m.lock(id)
	defer unlock()
	sub, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if input == nil {
		return nil, fmt.Errorf("%w: missing subscription", store.ErrInvalid)
	}
	sub.Name, sub.URL, sub.UserAgent, sub.Interval, sub.Group, sub.Disabled = input.Name, input.URL, input.UserAgent, input.Interval, input.Group, input.Disabled
	if err := validate(sub); err != nil {
		return nil, err
	}
	sub.UpdatedAt = time.Now().UTC()
	sub.NextUpdate = sub.UpdatedAt
	sub.Failures = 0
	if err := m.save(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}
func (m *Manager) Delete(ctx context.Context, id string) error {
	unlock := m.lock(id)
	defer unlock()
	if _, err := m.Get(ctx, id); err != nil {
		return err
	}
	nodes, err := m.Store.Nodes(ctx, id)
	if err != nil {
		return err
	}
	tx, err := m.Store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	assigned, err := assignedSOCKSNodes(ctx, tx)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if _, keep := assigned[node.ID]; keep {
			if err := detachSubscriptionNode(ctx, tx, node, id); err != nil {
				return err
			}
			continue
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM nodes WHERE id=?", node.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM subscriptions WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// assignedSOCKSNodes reads persisted and currently running assignments in the
// same transaction that removes subscription-owned nodes.
func assignedSOCKSNodes(ctx context.Context, tx *sql.Tx) (map[string]struct{}, error) {
	rows, err := tx.QueryContext(ctx, "SELECT value FROM backend_state WHERE key IN ('settings','current_config')")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assigned := make(map[string]struct{})
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var state struct {
			SOCKSListeners []struct {
				NodeID string `json:"node_id"`
			} `json:"socks_listeners"`
		}
		if err := json.Unmarshal([]byte(data), &state); err != nil {
			return nil, err
		}
		for _, listener := range state.SOCKSListeners {
			if listener.NodeID != "" {
				assigned[listener.NodeID] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return assigned, nil
}

// detachSubscriptionNode keeps an assigned node usable after it disappears from
// its subscription or the subscription itself is deleted.
func detachSubscriptionNode(ctx context.Context, tx *sql.Tx, node *ir.Node, subscriptionID string) error {
	node.Source = ir.Source{Type: "import"}
	node.GroupID = ""
	node.Status = "idle"
	data, err := json.Marshal(node)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE nodes SET ir_json=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", string(data), node.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM subscription_nodes WHERE subscription_id=? AND node_id=?", subscriptionID, node.ID)
	return err
}

// Update only replaces subscription-owned nodes after a full successful parse.
func (m *Manager) Update(ctx context.Context, id string) ([]*ir.Node, error) {
	unlock := m.lock(id)
	defer unlock()
	sub, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	sub.LastAttempt = time.Now().UTC()
	nodes, headers, err := m.fetch(ctx, sub)
	if err != nil {
		sub.Failures++
		sub.LastStatus = "error"
		sub.LastError = err.Error()
		sub.UpdatedAt = time.Now().UTC()
		delay := time.Minute
		for i := 1; i < sub.Failures && delay < 6*time.Hour; i++ {
			delay *= 2
		}
		if delay > 6*time.Hour {
			delay = 6 * time.Hour
		}
		sub.NextUpdate = sub.UpdatedAt.Add(delay)
		// Preserve failure metadata even when the caller timed out or disconnected.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if saveErr := m.save(saveCtx, sub); saveErr != nil {
			return nil, errors.Join(err, saveErr)
		}
		return nil, err
	}
	oldNodes, err := m.Store.Nodes(ctx, id)
	if err != nil {
		return nil, err
	}
	old := map[string]*ir.Node{}
	for _, node := range oldNodes {
		old[node.ID] = node
	}
	now := time.Now().UTC()
	diff := Diff{}
	seen := map[string]bool{}
	fingerprints := map[string]bool{}
	oldByFingerprint := map[string]*ir.Node{}
	oldByEndpoint := map[string][]*ir.Node{}
	endpointKey := func(n *ir.Node) string {
		return fmt.Sprintf("%s\x00%s\x00%s\x00%d", n.Protocol, n.Name, n.Server.Host, n.Server.Port)
	}
	for _, n := range oldNodes {
		oldByFingerprint[fingerprint(n)] = n
		key := endpointKey(n)
		oldByEndpoint[key] = append(oldByEndpoint[key], n)
	}
	unique := make([]*ir.Node, 0, len(nodes))
	for _, node := range nodes {
		// A subscription node's identity is its connection parameters, not its
		// display name. Providers commonly rename the same endpoint between
		// refreshes and may repeat it with different labels in one response.
		fp := fingerprint(node)
		if fingerprints[fp] {
			diff.Duplicates++
			continue
		}
		fingerprints[fp] = true
		identity := id + "\x00" + fp
		digest := sha256.Sum256([]byte(identity))
		node.ID = hex.EncodeToString(digest[:16])
		if previous := oldByFingerprint[fp]; previous != nil && !seen[previous.ID] {
			node.ID = previous.ID
		} else if candidates := oldByEndpoint[endpointKey(node)]; len(candidates) == 1 && !seen[candidates[0].ID] {
			// Preserve identity on a credential change only when the previous
			// endpoint is unambiguous. Duplicate names never identify a node.
			node.ID = candidates[0].ID
		}
		seen[node.ID] = true
		unique = append(unique, node)
		node.Source = ir.Source{Type: "subscription", SubscriptionID: id}
		node.CreatedAt = now
		node.UpdatedAt = now
		node.GroupID = sub.Group
		node.Status = "active"
		if previous, ok := old[node.ID]; ok {
			node.CreatedAt = previous.CreatedAt
			node.GroupID = previous.GroupID
			node.Tags = previous.Tags
			node.Favorite = previous.Favorite
			node.SortOrder = previous.SortOrder
			if fingerprint(previous) != fingerprint(node) {
				diff.Changed++
			} else {
				node.LastDelayMs = previous.LastDelayMs
				node.LastTestAt = previous.LastTestAt
			}
		} else {
			diff.Added++
		}
	}
	nodes = unique
	for id := range old {
		if !seen[id] {
			diff.Removed++
		}
	}
	sub.Used, sub.Total, sub.Expire = ExtractUsageInfo(headers)
	sub.LastUpdate, sub.UpdatedAt = now, now
	sub.LastStatus = "ok"
	sub.LastError = ""
	sub.Failures = 0
	sub.Diff = diff
	sub.NextUpdate = now.Add(time.Duration(sub.Interval) * time.Hour)
	tx, err := m.Store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	assigned, err := assignedSOCKSNodes(ctx, tx)
	if err != nil {
		return nil, err
	}
	for oldID, node := range old {
		if !seen[oldID] {
			if _, keep := assigned[oldID]; keep {
				if err := detachSubscriptionNode(ctx, tx, node, id); err != nil {
					return nil, err
				}
				continue
			}
			if _, err = tx.ExecContext(ctx, "DELETE FROM nodes WHERE id=?", oldID); err != nil {
				return nil, err
			}
		}
	}
	for _, node := range nodes {
		data, err := json.Marshal(node)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO nodes(id,name,protocol,ir_json) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,protocol=excluded.protocol,ir_json=excluded.ir_json,updated_at=CURRENT_TIMESTAMP`, node.ID, node.Name, node.Protocol, string(data)); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO subscription_nodes(subscription_id,node_id) VALUES(?,?)", sub.ID, node.ID); err != nil {
			return nil, err
		}
	}
	data, err := payload(sub)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE subscriptions SET payload=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", string(data), id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return nodes, nil
}
func fingerprint(n *ir.Node) string {
	return ir.ConnectionFingerprint(n)
}
func (m *Manager) fetch(ctx context.Context, sub *Subscription) ([]*ir.Node, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sub.URL, nil)
	if err != nil {
		return nil, nil, &fetchError{kind: ErrFetchURL, cause: err}
	}
	ua := sub.UserAgent
	if ua == "" {
		ua = "DuoRelay/0.1"
	}
	req.Header.Set("User-Agent", ua)
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, &fetchError{kind: classifyFetchError(err), cause: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("%w: %w: HTTP %d", ErrUpdate, ErrFetchHTTP, resp.StatusCode)
	}
	const limit = 16 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, &fetchError{kind: ErrFetchBody, cause: err}
	}
	if len(body) > limit {
		return nil, nil, fmt.Errorf("%w: %w: response exceeds 16 MiB", ErrUpdate, ErrFetchBody)
	}
	nodes, skipped, err := m.ParseDetailed(body)
	sub.Skipped = skipped
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w: %w", ErrUpdate, ErrFetchParse, err)
	}
	if len(nodes) == 0 {
		return nil, nil, fmt.Errorf("%w: %w: subscription contains no usable nodes", ErrUpdate, ErrFetchParse)
	}
	return nodes, resp.Header, nil
}

func classifyFetchError(err error) error {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrFetchDNS
	}
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return ErrFetchTimeout
	}
	var tlsErr *tls.CertificateVerificationError
	if errors.As(err, &tlsErr) || strings.Contains(strings.ToLower(err.Error()), "tls") || strings.Contains(strings.ToLower(err.Error()), "certificate") {
		return ErrFetchTLS
	}
	return ErrFetchNetwork
}

// Run performs due updates until cancellation. Retry deadlines survive restarts.
func (m *Manager) Run(ctx context.Context, poll time.Duration) error {
	if err := m.ready(); err != nil {
		return err
	}
	if poll <= 0 {
		poll = time.Minute
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		subs, err := m.List(ctx, "")
		if err != nil {
			return err
		}
		for _, sub := range subs {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !sub.Disabled && !sub.NextUpdate.After(time.Now()) {
				_, _ = m.Update(ctx, sub.ID)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func ExtractUsageInfo(headers map[string][]string) (used, total int64, expire time.Time) {
	values := map[string]int64{}
	for key, lines := range headers {
		if !strings.EqualFold(key, "subscription-userinfo") {
			continue
		}
		for _, line := range lines {
			for _, part := range strings.Split(line, ";") {
				pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
				if len(pair) != 2 {
					continue
				}
				v, err := strconv.ParseInt(strings.TrimSpace(pair[1]), 10, 64)
				if err == nil && v >= 0 {
					values[strings.TrimSpace(strings.ToLower(pair[0]))] = v
				}
			}
		}
	}
	upload, download := values["upload"], values["download"]
	if upload > math.MaxInt64-download {
		used = math.MaxInt64
	} else {
		used = upload + download
	}
	total = values["total"]
	if values["expire"] > 0 {
		expire = time.Unix(values["expire"], 0).UTC()
	}
	return
}
