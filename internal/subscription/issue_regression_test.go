package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIssuePartialStructuredImports(t *testing.T) {
	m := newTestManager(t)
	for name, content := range map[string]string{
		"v2rayn": `{"configs":[{"protocol":"socks","address":"127.0.0.1","port":1080,"name":"good"},{"protocol":"unknown-secret","address":"bad"}]}`,
		"sip008": `{"servers":[{"server":"127.0.0.1","server_port":8388,"method":"aes-256-gcm","password":"kept"},{"server":"bad"}]}`,
		"ini":    "[Proxy]\ngood = socks5, 127.0.0.1, 1080\nbad = unknown, host, 443, password=hidden\n[Rule]\nFINAL,DIRECT",
		"xray":   `{"outbounds":[{"protocol":"socks","settings":{"servers":[{"address":"127.0.0.1","port":1080}]}},{"protocol":"unknown-secret"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			nodes, skipped, err := m.ParseDetailed([]byte(content))
			if err != nil || len(nodes) != 1 || len(skipped) != 1 || strings.Contains(skipped[0].Reason, "secret") || strings.Contains(skipped[0].Reason, "hidden") {
				t.Fatalf("partial parse failed or exposed credentials: %v %v %v", nodes, skipped, err)
			}
		})
	}
}

func TestIssueNativeXHTTPOptionsAndUnsupportedPQV(t *testing.T) {
	m := newTestManager(t)
	entry := map[string]any{"type": "vless", "name": "xhttp", "server": "127.0.0.1", "port": 443, "uuid": "b8ca57c2-eb39-491a-8425-bba828fbad2d", "network": "xhttp", "xhttp-opts": map[string]any{"path": "/hello", "mode": "auto", "sc-max-each-post-bytes": 1000000}}
	data, _ := json.Marshal(map[string]any{"proxies": []any{entry}})
	nodes, skipped, err := m.ParseDetailed(data)
	if err != nil || len(nodes) != 1 || len(skipped) != 0 || nodes[0].Transport.Path != "/hello" || nodes[0].Transport.Opts["scMaxEachPostBytes"] == nil {
		t.Fatal("native XHTTP lost parameters", err, skipped)
	}
	entry["reality-opts"] = map[string]any{"pqv": "hidden"}
	data, _ = json.Marshal(map[string]any{"proxies": []any{entry, map[string]any{"type": "socks5", "name": "valid", "server": "127.0.0.1", "port": 1080}}})
	nodes, skipped, err = m.ParseDetailed(data)
	if err != nil || len(nodes) != 1 || len(skipped) != 1 || !strings.Contains(skipped[0].Reason, "pqv") || strings.Contains(skipped[0].Reason, "hidden") {
		t.Fatal("unsupported Reality option was silently accepted", err, skipped)
	}
}

func TestIssueRefreshDeduplicatesAndRetainsIDOnRename(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	var body atomic.Value
	body.Store(shareLink + "\n" + strings.Replace(shareLink, "#alpha", "#other", 1) + "\nunknown://secret@bad:443")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body.Load().(string))) }))
	defer server.Close()
	sub := &Subscription{Name: "partial", URL: server.URL}
	if err := m.Add(ctx, sub); err != nil {
		t.Fatal(err)
	}
	nodes, err := m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 1 {
		t.Fatal("duplicate entries should not fail refresh", err)
	}
	firstID := nodes[0].ID
	nodes[0].Favorite = true
	if err := m.Store.SaveNode(ctx, nodes[0]); err != nil {
		t.Fatal(err)
	}
	saved, _ := m.Get(ctx, sub.ID)
	if len(saved.Skipped) != 1 || saved.Diff.Duplicates != 1 {
		t.Fatal("missing partial import report", saved)
	}
	body.Store(strings.Replace(shareLink, "#alpha", "#renamed", 1))
	nodes, err = m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 1 || nodes[0].ID != firstID || !nodes[0].Favorite {
		t.Fatal("rename lost identity/metadata", err)
	}
}

func TestIssueFetchErrorPreservesCauseAndRedactsToken(t *testing.T) {
	cause := &net.DNSError{Name: "provider.example", Err: "no such host"}
	err := &fetchError{kind: ErrFetchDNS, cause: &url.Error{Op: "Get", URL: "https://provider.example/private-secret?token=hidden", Err: cause}}
	var dns *net.DNSError
	if !errors.As(err, &dns) || !errors.Is(err, ErrFetchDNS) || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "hidden") {
		t.Fatal("cause lost or subscription credentials leaked", err)
	}
}
