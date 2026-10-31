package speedtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/task"
)

type Options struct {
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}
type Result struct {
	ID       string    `json:"id"`
	NodeID   string    `json:"node_id"`
	Type     string    `json:"type"`
	DelayMS  int64     `json:"delay_ms"`
	Success  bool      `json:"success"`
	Error    string    `json:"error,omitempty"`
	TestedAt time.Time `json:"tested_at"`
}

// HTTPProbe measures an actual outbound through a configured kernel, never an
// unrelated direct connection. It is injected by the application composition.
type HTTPProbe func(context.Context, *ir.Node, string, time.Duration) (int64, error)
type Engine struct {
	Store     *store.Store
	Queue     *task.Queue
	ProbeHTTP HTTPProbe
}

func NewEngine(s *store.Store, q *task.Queue) *Engine {
	return &Engine{Store: s, Queue: q}
}
func Normalize(options Options) (Options, error) {
	if options.Type == "" {
		options.Type = "tcp"
	}
	if options.Type != "tcp" && options.Type != "http" {
		return options, fmt.Errorf("%w: type must be tcp or http", store.ErrInvalid)
	}
	if options.TimeoutMS == 0 {
		options.TimeoutMS = 5000
	}
	if options.TimeoutMS < 1 || options.TimeoutMS > 60000 {
		return options, fmt.Errorf("%w: timeout_ms must be 1..60000", store.ErrInvalid)
	}
	if options.Type == "http" {
		if options.URL == "" {
			options.URL = "https://www.gstatic.com/generate_204"
		}
		u, err := url.Parse(options.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return options, fmt.Errorf("%w: invalid HTTP test URL", store.ErrInvalid)
		}
	}
	return options, nil
}
func TCPDelay(ctx context.Context, address string) (int64, error) {
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return time.Since(start).Milliseconds(), nil
}

// HTTPDelay measures a request using an explicit HTTP(S) proxy. Callers choose
// the proxy; it is not silently replaced with a direct connection.
func HTTPDelay(ctx context.Context, target, proxyAddress string) (int64, error) {
	proxy, err := url.Parse(proxyAddress)
	if err != nil || (proxy.Scheme != "http" && proxy.Scheme != "https") || proxy.Host == "" {
		return 0, fmt.Errorf("HTTP proxy is required")
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	response, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP proxy request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return 0, fmt.Errorf("HTTP probe returned %d", response.StatusCode)
	}
	return time.Since(start).Milliseconds(), nil
}

func (e *Engine) Test(ctx context.Context, id string, options Options) (*Result, error) {
	options, err := Normalize(options)
	if err != nil {
		return nil, err
	}
	if e.Store == nil {
		return nil, fmt.Errorf("speedtest store is not configured")
	}
	node, err := e.Store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(options.TimeoutMS)*time.Millisecond)
	defer cancel()
	result := &Result{ID: uuid.NewString(), NodeID: id, Type: options.Type, TestedAt: time.Now().UTC()}
	if options.Type == "tcp" {
		result.DelayMS, err = TCPDelay(probeCtx, net.JoinHostPort(node.Server.Host, strconv.Itoa(node.Server.Port)))
	} else if e.ProbeHTTP != nil {
		result.DelayMS, err = e.ProbeHTTP(probeCtx, node, options.URL, time.Duration(options.TimeoutMS)*time.Millisecond)
	} else if node.Protocol == ir.ProtocolHTTP {
		proxy := &url.URL{Scheme: "http", Host: net.JoinHostPort(node.Server.Host, strconv.Itoa(node.Server.Port))}
		if node.Security.Type == "tls" {
			proxy.Scheme = "https"
		}
		if user, ok := node.Proto["username"].(string); ok {
			password, _ := node.Proto["password"].(string)
			proxy.User = url.UserPassword(user, password)
		}
		result.DelayMS, err = HTTPDelay(probeCtx, options.URL, proxy.String())
	} else {
		err = fmt.Errorf("HTTP testing requires an active kernel containing this node")
	}
	result.Success = err == nil
	if err != nil {
		result.DelayMS = -1
		switch {
		case errors.Is(probeCtx.Err(), context.DeadlineExceeded):
			result.Error = "test timed out"
		case errors.Is(probeCtx.Err(), context.Canceled):
			result.Error = "test cancelled"
		default:
			result.Error = "connection or outbound probe failed"
		}
	}
	if result.Success {
		node.LastDelayMs = int(result.DelayMS)
	} else {
		node.LastDelayMs = -1
	}
	node.LastTestAt = result.TestedAt
	// Write the result and node measurement atomically, re-reading the current IR
	// so an update during a long probe is not overwritten by a stale node copy.
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer persistCancel()
	tx, dbErr := e.Store.DB().BeginTx(persistCtx, nil)
	if dbErr != nil {
		return nil, dbErr
	}
	defer tx.Rollback()
	var currentJSON string
	if dbErr = tx.QueryRowContext(persistCtx, "SELECT ir_json FROM nodes WHERE id=?", id).Scan(&currentJSON); dbErr != nil {
		return nil, dbErr
	}
	var current ir.Node
	if dbErr = json.Unmarshal([]byte(currentJSON), &current); dbErr != nil {
		return nil, dbErr
	}
	current.LastDelayMs, current.LastTestAt = node.LastDelayMs, node.LastTestAt
	data, dbErr := json.Marshal(&current)
	if dbErr != nil {
		return nil, dbErr
	}
	if _, dbErr = tx.ExecContext(persistCtx, "UPDATE nodes SET ir_json=? WHERE id=?", string(data), id); dbErr != nil {
		return nil, dbErr
	}
	data, dbErr = json.Marshal(result)
	if dbErr != nil {
		return nil, dbErr
	}
	if _, dbErr = tx.ExecContext(persistCtx, "INSERT INTO speedtest_results(id,node_id,result_json,created_at) VALUES(?,?,?,?)", result.ID, id, string(data), result.TestedAt.Format(time.RFC3339Nano)); dbErr != nil {
		return nil, dbErr
	}
	if dbErr = tx.Commit(); dbErr != nil {
		return nil, dbErr
	}
	return result, nil
}

// Batch validates all IDs before enqueueing one bounded-concurrency queue job.
func (e *Engine) Batch(ctx context.Context, ids []string, options Options) (string, error) {
	options, err := Normalize(options)
	if err != nil {
		return "", err
	}
	if e.Queue == nil || e.Store == nil {
		return "", fmt.Errorf("speedtest dependencies are not configured")
	}
	if len(ids) == 0 || len(ids) > 10000 {
		return "", fmt.Errorf("%w: node_ids must contain 1..10000 entries", store.ErrInvalid)
	}
	seen := map[string]bool{}
	copyIDs := []string{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		if _, err := e.Store.Node(ctx, id); err != nil {
			return "", err
		}
		seen[id] = true
		copyIDs = append(copyIDs, id)
	}
	id, err := e.Queue.Submit(ctx, "speedtest", func(ctx context.Context, t *task.Task) error {
		e.Queue.UpdateProgress(t, 0, len(copyIDs))
		results := make([]*Result, len(copyIDs))
		jobs := make(chan int)
		var wg sync.WaitGroup
		var mu sync.Mutex
		completed := 0
		var failures []string
		workers := 4
		if len(copyIDs) < workers {
			workers = len(copyIDs)
		}
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for index := range jobs {
					result, err := e.Test(ctx, copyIDs[index], options)
					mu.Lock()
					results[index] = result
					if err != nil {
						failures = append(failures, "failed to persist test for "+copyIDs[index])
					}
					completed++
					e.Queue.UpdateProgress(t, completed, len(copyIDs))
					mu.Unlock()
				}
			}()
		}
	loop:
		for index := range copyIDs {
			select {
			case <-ctx.Done():
				break loop
			case jobs <- index:
			}
		}
		close(jobs)
		wg.Wait()
		t.Result = results
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if len(failures) > 0 {
			return fmt.Errorf("%s", strings.Join(failures, "; "))
		}
		return nil
	})
	return id, err
}
func (e *Engine) Task(id string) (*task.Task, error) {
	if e.Queue == nil {
		return nil, task.ErrNotFound
	}
	value, err := e.Queue.Get(id)
	if err != nil {
		return nil, err
	}
	if value.Type != "speedtest" {
		return nil, task.ErrNotFound
	}
	return value, nil
}
func (e *Engine) Results(ctx context.Context, id string) ([]*Result, error) {
	rows, err := e.Store.DB().QueryContext(ctx, "SELECT result_json FROM speedtest_results WHERE node_id=? ORDER BY created_at DESC LIMIT 100", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []*Result{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var result Result
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return nil, err
		}
		results = append(results, &result)
	}
	return results, rows.Err()
}
