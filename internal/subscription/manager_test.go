package subscription

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	s, err := store.New(&store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return NewManager(s)
}

const shareLink = "ss://YWVzLTI1Ni1nY206cGFzcw@127.0.0.1:8388#alpha"
const clashOne = "proxies:\n  - {name: alpha, type: ss, server: example.com, port: 8388, cipher: aes-256-gcm, password: pass}\n"
const openVPNProfile = "client\ndev tun\nproto udp\nremote vpn.example 1194\n<ca>\ncertificate\n</ca>\n"
const v2rayNOne = `{"v":"2","ps":"v2rayn-alpha","add":"example.com","port":"443","id":"b8ca57c2-eb39-491a-8425-bba828fbad2d","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"cdn.example.com"}`

func TestParseSubscriptionFormats(t *testing.T) {
	m := newTestManager(t)
	for _, tc := range []struct{ name, body, format string }{
		{"links", shareLink, "links"},
		{"base64", base64.StdEncoding.EncodeToString([]byte(shareLink + "\n")), "base64"},
		{"raw-base64", base64.RawURLEncoding.EncodeToString([]byte(shareLink)), "base64"},
		{"clash", clashOne, "clash"},
		{"sing-box", `{"outbounds":[{"type":"direct"},{"type":"shadowsocks","tag":"alpha","server":"example.com","server_port":8388,"method":"aes-256-gcm","password":"pass"}]}`, "sing-box"},
		{"v2rayn-json", v2rayNOne, "v2rayn"},
		{"v2rayn-list", `[` + v2rayNOne + `]`, "v2rayn"},
		{"v2rayn-base64-json", base64.StdEncoding.EncodeToString([]byte(v2rayNOne)), "v2rayn-base64"},
		{"openvpn", openVPNProfile, "openvpn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			format, err := ParseFormat([]byte(tc.body))
			if err != nil || format != tc.format {
				t.Fatalf("format: %s %v", format, err)
			}
			nodes, err := m.Parse([]byte(tc.body))
			if err != nil || len(nodes) != 1 {
				t.Fatalf("nodes: %#v %v", nodes, err)
			}
			if tc.name == "openvpn" {
				if nodes[0].Protocol != ir.ProtocolOpenVPN || nodes[0].Server.Host != "vpn.example" || nodes[0].Proto["ca"] != "certificate" {
					t.Fatalf("OpenVPN node: %#v", nodes[0])
				}
			} else if tc.name == "v2rayn-json" || tc.name == "v2rayn-list" || tc.name == "v2rayn-base64-json" {
				if nodes[0].Protocol != ir.ProtocolVMess || nodes[0].Proto["uuid"] != "b8ca57c2-eb39-491a-8425-bba828fbad2d" {
					t.Fatalf("V2RayN node: %#v", nodes[0])
				}
			} else if nodes[0].Protocol != ir.ProtocolShadowsocks || nodes[0].Proto["password"] != "pass" {
				t.Fatalf("node: %#v", nodes[0])
			}
		})
	}
	for _, body := range []string{"", "garbage", "proxies: [{name: bad, type: ss, server: host, port: 0}]"} {
		if _, err := m.Parse([]byte(body)); err == nil {
			t.Fatalf("invalid input accepted: %s", body)
		}
	}
}

func TestParseModernShareLinks(t *testing.T) {
	m := newTestManager(t)
	body := "anytls://secret@example.com:443?sni=example.com#anytls\n" +
		"mieru://user:pass@example.com:3000?transport=tcp#mieru\n"
	nodes, err := m.Parse([]byte(body))
	if err != nil || len(nodes) != 2 {
		t.Fatalf("modern links: %#v %v", nodes, err)
	}
	if nodes[0].Protocol != ir.ProtocolAnyTLS || nodes[1].Protocol != ir.ProtocolMieru {
		t.Fatalf("modern protocols: %#v", nodes)
	}
}

func TestParseBase64NativeDocuments(t *testing.T) {
	m := newTestManager(t)
	clash := base64.StdEncoding.EncodeToString([]byte("proxies:\n  - {name: anytls, type: anytls, server: example.com, port: 443, password: secret}\n"))
	format, err := ParseFormat([]byte(clash))
	if err != nil || format != "clash-base64" {
		t.Fatalf("Clash Base64 format: %s %v", format, err)
	}
	nodes, err := m.Parse([]byte(clash))
	if err != nil || len(nodes) != 1 || nodes[0].Protocol != ir.ProtocolAnyTLS {
		t.Fatalf("Clash Base64 nodes: %#v %v", nodes, err)
	}

	singBox := base64.RawStdEncoding.EncodeToString([]byte(`{"outbounds":[{"type":"anytls","tag":"anytls","server":"example.com","server_port":443,"password":"secret","tls":{"enabled":true}}]}`))
	format, err = ParseFormat([]byte(singBox))
	if err != nil || format != "sing-box-base64" {
		t.Fatalf("sing-box Base64 format: %s %v", format, err)
	}
	nodes, err = m.Parse([]byte(singBox))
	if err != nil || len(nodes) != 1 || nodes[0].Protocol != ir.ProtocolAnyTLS {
		t.Fatalf("sing-box Base64 nodes: %#v %v", nodes, err)
	}
}

func TestParseV2RayNModernProtocols(t *testing.T) {
	m := newTestManager(t)
	content := `[{"protocol":"anytls","ps":"AnyTLS JSON","address":"anytls.example.com","port":443,"password":"secret","sni":"anytls.example.com"},{"protocol":"mieru","ps":"Mieru JSON","address":"mieru.example.com","port":3000,"username":"user","password":"pass","transport":"TCP"}]`
	nodes, err := m.Parse([]byte(content))
	if err != nil || len(nodes) != 2 {
		t.Fatalf("V2RayN modern protocols: %#v %v", nodes, err)
	}
	if nodes[0].Protocol != ir.ProtocolAnyTLS || nodes[0].Proto["password"] != "secret" || nodes[0].Security.Type != "tls" {
		t.Fatalf("unexpected AnyTLS V2RayN node: %#v", nodes[0])
	}
	if nodes[1].Protocol != ir.ProtocolMieru || nodes[1].Proto["username"] != "user" || nodes[1].Proto["transport"] != "TCP" {
		t.Fatalf("unexpected Mieru V2RayN node: %#v", nodes[1])
	}
}

func TestParseClashModernProtocols(t *testing.T) {
	m := newTestManager(t)
	content := `proxies:
  - name: anytls
    type: anytls
    server: anytls.example.com
    port: 443
    password: secret
    sni: anytls.example.com
  - name: mieru
    type: mieru
    server: mieru.example.com
    port: 3000
    username: user
    password: pass
    transport: TCP`
	nodes, err := m.Parse([]byte(content))
	if err != nil || len(nodes) != 2 {
		t.Fatalf("Clash modern protocols: %#v %v", nodes, err)
	}
	if nodes[0].Protocol != ir.ProtocolAnyTLS || nodes[1].Protocol != ir.ProtocolMieru || nodes[1].Proto["transport"] != "TCP" {
		t.Fatalf("unexpected Clash modern nodes: protocols=%s/%s transport=%#v nodes=%#v", nodes[0].Protocol, nodes[1].Protocol, nodes[1].Proto["transport"], nodes)
	}
}

func TestUpdateDiffFailureAndCRUD(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	var content atomic.Value
	content.Store(clashOne)
	var status atomic.Int32
	status.Store(200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "custom-agent" {
			t.Error("user agent not forwarded")
		}
		w.Header().Set("subscription-userinfo", "upload=10; download=20; total=100; expire=2000000000")
		w.WriteHeader(int(status.Load()))
		fmt.Fprint(w, content.Load().(string))
	}))
	defer server.Close()
	sub := &Subscription{Name: "test", URL: server.URL + "/?token=private", UserAgent: "custom-agent", Group: "group"}
	if err := m.Add(ctx, sub); err != nil {
		t.Fatal(err)
	}
	var encrypted, payload string
	if err := m.Store.DB().QueryRow("SELECT url_enc,payload FROM subscriptions WHERE id=?", sub.ID).Scan(&encrypted, &payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "private") || strings.Contains(payload, "private") {
		t.Fatal("URL token leaked to stored payload")
	}
	manual := &ir.Node{Name: "manual", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 80}, Source: ir.Source{Type: "manual"}}
	if err := m.Store.SaveNode(ctx, manual); err != nil {
		t.Fatal(err)
	}
	nodes, err := m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("update: %v %v", nodes, err)
	}
	firstID := nodes[0].ID
	nodes[0].Favorite = true
	nodes[0].Tags = []string{"keep"}
	if err := m.Store.SaveNode(ctx, nodes[0]); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(ctx, sub.ID)
	if err != nil || got.Used != 30 || got.Total != 100 || got.Diff.Added != 1 || got.Expire.Unix() != 2000000000 {
		t.Fatalf("bad metadata: %#v %v", got, err)
	}
	content.Store(strings.Replace(clashOne, "password: pass", "password: changed", 1) + "  - {name: beta, type: http, server: example.net, port: 80}\n")
	nodes, err = m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 2 {
		t.Fatalf("second update: %v", err)
	}
	preserved, err := m.Store.Node(ctx, firstID)
	if err != nil || !preserved.Favorite || len(preserved.Tags) != 1 {
		t.Fatal("lost node identity/metadata")
	}
	got, _ = m.Get(ctx, sub.ID)
	if got.Diff.Changed != 1 || got.Diff.Added != 1 {
		t.Fatalf("bad diff: %v", got.Diff)
	}
	content.Store(clashOne)
	if _, err := m.Update(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = m.Get(ctx, sub.ID)
	if got.Diff.Removed != 1 {
		t.Fatal("removal not counted")
	}
	lastUpdate := got.LastUpdate
	status.Store(503)
	if _, err := m.Update(ctx, sub.ID); !errors.Is(err, ErrUpdate) {
		t.Fatalf("expected fetch failure: %v", err)
	}
	got, _ = m.Get(ctx, sub.ID)
	if got.LastStatus != "error" || got.Failures != 1 || !got.LastUpdate.Equal(lastUpdate) || !got.NextUpdate.After(time.Now()) || strings.Contains(got.LastError, "private") {
		t.Fatalf("bad failure metadata: %#v", got)
	}
	if nodes, _ := m.Store.Nodes(ctx, sub.ID); len(nodes) != 1 {
		t.Fatal("failure removed old nodes")
	}
	groups, err := m.Groups(ctx)
	if err != nil || len(groups) != 1 || groups[0] != "group" {
		t.Fatalf("groups: %v %v", groups, err)
	}
	if list, err := m.List(ctx, "other"); err != nil || len(list) != 0 {
		t.Fatal("group filter failed")
	}
	input := &Subscription{Name: "renamed", URL: server.URL, UserAgent: "custom-agent", Interval: 2, Group: "other"}
	if _, err := m.Edit(ctx, sub.ID, input); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(ctx, sub.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if nodes, _ := m.Store.Nodes(ctx, ""); len(nodes) != 1 || nodes[0].ID != manual.ID {
		t.Fatal("delete touched manual node or retained owned node")
	}
}

func TestSchedulerAndCancellation(t *testing.T) {
	m := newTestManager(t)
	updated := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, clashOne)
		select {
		case updated <- struct{}{}:
		default:
		}
	}))
	defer server.Close()
	sub := &Subscription{Name: "scheduled", URL: server.URL}
	if err := m.Add(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx, 10*time.Millisecond) }()
	select {
	case <-updated:
	case <-time.After(3 * time.Second):
		t.Fatal("scheduler did not update")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("scheduler did not stop")
	}
}

func TestValidationAndUsage(t *testing.T) {
	m := newTestManager(t)
	for _, sub := range []*Subscription{nil, {Name: "bad", URL: "file:///etc/test"}, {Name: "bad", URL: "https://example.com", Interval: -1}, {Name: "bad", URL: "https://example.com", UserAgent: "bad\r\nheader"}} {
		if err := m.Add(context.Background(), sub); !errors.Is(err, store.ErrInvalid) {
			t.Fatalf("bad subscription accepted: %v", err)
		}
	}
	used, total, expire := ExtractUsageInfo(map[string][]string{"Subscription-Userinfo": {"upload=bad;download=30;total=40;expire=2000000000"}})
	if used != 30 || total != 40 || expire.Unix() != 2000000000 {
		t.Fatalf("bad usage: %d %d %v", used, total, expire)
	}
}

func TestSubscriptionUpdateRetainsAssignedNodeAfterUpstreamRemoval(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	content := clashOne
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, content)
	}))
	defer server.Close()

	sub := &Subscription{Name: "pinned", URL: server.URL}
	if err := m.Add(ctx, sub); err != nil {
		t.Fatal(err)
	}
	nodes, err := m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("initial update: nodes=%d err=%v", len(nodes), err)
	}
	assignedID := nodes[0].ID
	settings := map[string]any{
		"socks_listeners": []map[string]any{{"port": 18081, "node_id": assignedID}},
	}
	if err := m.Store.SaveState(ctx, "settings", settings); err != nil {
		t.Fatal(err)
	}

	content = "proxies:\n  - {name: beta, type: http, server: beta.example, port: 8080}\n"
	nodes, err = m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("replacement update: nodes=%d err=%v", len(nodes), err)
	}
	pinned, err := m.Store.Node(ctx, assignedID)
	if err != nil {
		t.Fatalf("assigned node was deleted: %v", err)
	}
	if pinned.Source.Type != "import" || pinned.Source.SubscriptionID != "" || pinned.GroupID != "" {
		t.Fatalf("assigned node was not detached cleanly: %#v", pinned)
	}
	subscriptionNodes, err := m.Store.Nodes(ctx, sub.ID)
	if err != nil || len(subscriptionNodes) != 1 || subscriptionNodes[0].ID == assignedID {
		t.Fatalf("removed node still belongs to subscription: %#v, %v", subscriptionNodes, err)
	}

	if err := m.Delete(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Store.Node(ctx, assignedID); err != nil {
		t.Fatalf("deleting the subscription removed its assigned node: %v", err)
	}
}

func TestDeleteSubscriptionDetachesAssignedNodes(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, clashOne)
	}))
	defer server.Close()

	sub := &Subscription{Name: "pinned", URL: server.URL}
	if err := m.Add(ctx, sub); err != nil {
		t.Fatal(err)
	}
	nodes, err := m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("initial update: nodes=%d err=%v", len(nodes), err)
	}
	assignedID := nodes[0].ID
	if err := m.Store.SaveState(ctx, "settings", map[string]any{"socks_listeners": []any{}}); err != nil {
		t.Fatal(err)
	}
	current := map[string]any{
		"socks_listeners": []map[string]any{{"port": 18081, "node_id": assignedID}},
	}
	if err := m.Store.SaveState(ctx, "current_config", current); err != nil {
		t.Fatal(err)
	}

	if err := m.Delete(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	pinned, err := m.Store.Node(ctx, assignedID)
	if err != nil || pinned.Source.Type != "import" || pinned.Source.SubscriptionID != "" {
		t.Fatalf("subscription deletion did not preserve and detach the assigned node: %#v, %v", pinned, err)
	}
	if _, err := m.Get(ctx, sub.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("subscription still exists after deletion: %v", err)
	}
}
