package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"gopkg.in/yaml.v3"
)

// Opt-in: this starts real kernels, but only uses temporary configurations,
// ephemeral loopback ports and deterministic local proxy/target servers.
func TestRealChainTraffic(t *testing.T) {
	if testing.Short() || os.Getenv("DUORELAY_CHAIN_INTEGRATION") != "1" {
		t.Skip("set DUORELAY_CHAIN_INTEGRATION=1")
	}
	kernelDir, err := filepath.Abs(filepath.Join("..", "..", "kernels"))
	if err != nil {
		t.Fatal(err)
	}
	s, _ := testService(t)
	runtime := kernel.NewManager(kernelDir)
	s.Runtime = runtime
	t.Cleanup(func() { _ = s.Stop(context.Background()) })
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "chain-traffic-ok") }))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)
	var ledgerMu sync.Mutex
	var ledger []string
	const count = 8
	var servers []*httptest.Server
	for hop := 0; hop < count; hop++ {
		hop := hop
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "CONNECT" {
				http.Error(w, "CONNECT required", 405)
				return
			}
			upstream, err := net.DialTimeout("tcp", r.Host, 5*time.Second)
			if err != nil {
				http.Error(w, "dial failed", 502)
				return
			}
			defer upstream.Close()
			client, buffer, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer client.Close()
			_ = client.SetDeadline(time.Now().Add(10 * time.Second))
			_ = upstream.SetDeadline(time.Now().Add(10 * time.Second))
			ledgerMu.Lock()
			ledger = append(ledger, fmt.Sprintf("%d:%s", hop, r.Host))
			ledgerMu.Unlock()
			_, _ = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
			_ = buffer.Flush()
			done := make(chan struct{})
			go func() { _, _ = io.Copy(upstream, buffer); close(done) }()
			_, _ = io.Copy(client, upstream)
			client.Close()
			<-done
		}))
		servers = append(servers, server)
		defer server.Close()
	}
	port := func() int {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	settings := defaults()
	settings.InboundPort = port()
	settings.MixedPort = port()
	settings.SOCKSPort = port()
	settings.RoutingMode = "global"
	for _, tc := range []struct {
		name   string
		owners []ir.KernelSet
		scoped bool
	}{
		{"sing-box", []ir.KernelSet{1, 1}, false},
		{"mihomo", []ir.KernelSet{2, 2}, false},
		{"sing-box-to-mihomo", []ir.KernelSet{1, 2}, false},
		{"mihomo-to-sing-box", []ir.KernelSet{2, 1}, false},
		{"eight-alternating-hops", []ir.KernelSet{1, 2, 1, 2, 1, 2, 1, 2}, false},
		{"scoped-with-both-directions", []ir.KernelSet{2, 1, 2}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings.SOCKSOnly = tc.scoped
			if err := s.SaveSettings(context.Background(), settings); err != nil {
				t.Fatal(err)
			}
			var ids []string
			for i, owner := range tc.owners {
				u, _ := url.Parse(servers[i].URL)
				_, p, _ := net.SplitHostPort(u.Host)
				nport, _ := strconv.Atoi(p)
				n := &ir.Node{Name: "PROXY", Protocol: "http", Owner: owner, Server: ir.Server{Host: "127.0.0.1", Port: nport}}
				if err := s.Store.SaveNode(context.Background(), n); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, n.ID)
			}
			state, err := s.Apply(context.Background(), ApplyRequest{Mode: "chain", NodeIDs: ids})
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.HealthCheck(context.Background(), string(state.Kernel)); err != nil {
				t.Fatal(err)
			}
			for _, aux := range state.Auxiliary {
				if err := runtime.HealthCheck(context.Background(), string(aux.Kernel)); err != nil {
					t.Fatal(err)
				}
			}
			for _, config := range append([]RuntimeConfig{{Kernel: state.Kernel, Path: state.Path}}, state.Auxiliary...) {
				data, err := os.ReadFile(config.Path)
				if err != nil {
					t.Fatal(err)
				}
				root := map[string]any{}
				if config.Kernel == ir.KernelSingBox {
					err = json.Unmarshal(data, &root)
				} else {
					err = yaml.Unmarshal(data, &root)
				}
				if err != nil {
					t.Fatal(err)
				}
				key := "inbounds"
				if config.Kernel == ir.KernelMihomo {
					key = "listeners"
				}
				for _, in := range chainEntries(root, key) {
					tag := fmt.Sprint(in["tag"])
					if config.Kernel == ir.KernelMihomo {
						tag = fmt.Sprint(in["name"])
					}
					if !strings.HasPrefix(tag, "DR-BRIDGE-") {
						continue
					}
					p := in["listen_port"]
					if config.Kernel == ir.KernelMihomo {
						p = in["port"]
					}
					connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%v", p), time.Second)
					if err != nil {
						t.Fatal(err)
					}
					_ = connection.SetDeadline(time.Now().Add(time.Second))
					_, _ = connection.Write([]byte{5, 1, 0})
					var response [2]byte
					_, err = io.ReadFull(connection, response[:])
					connection.Close()
					if err == nil && response == [2]byte{5, 0} {
						t.Fatal("bridge accepted anonymous clients")
					}
				}
			}
			ledgerMu.Lock()
			ledger = nil
			ledgerMu.Unlock()
			mixed := settings.InboundPort
			if state.Kernel == ir.KernelMihomo {
				mixed = settings.MixedPort
			}
			proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", mixed))
			if tc.scoped {
				proxyURL, _ = url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", settings.SOCKSPort))
			}
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
			client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
			response, err := client.Get(target.URL)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			transport.CloseIdleConnections()
			if err != nil || string(body) != "chain-traffic-ok" {
				t.Fatalf("unexpected target response: %v", err)
			}
			ledgerMu.Lock()
			events := append([]string{}, ledger...)
			ledgerMu.Unlock()
			if len(events) != len(ids) {
				t.Fatalf("expected %d hops, got %v", len(ids), events)
			}
			for i, event := range events {
				destination := targetURL.Host
				if i+1 < len(ids) {
					u, _ := url.Parse(servers[i+1].URL)
					destination = u.Host
				}
				if event != fmt.Sprintf("%d:%s", i, destination) {
					t.Fatalf("wrong traversal: %v", events)
				}
			}
			if tc.scoped {
				ledgerMu.Lock()
				ledger = nil
				ledgerMu.Unlock()
				proxyURL, _ = url.Parse(fmt.Sprintf("http://127.0.0.1:%d", mixed))
				transport = &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
				client = &http.Client{Transport: transport, Timeout: 5 * time.Second}
				response, err = client.Get(target.URL)
				if err != nil {
					t.Fatal(err)
				}
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
				transport.CloseIdleConnections()
				ledgerMu.Lock()
				events = append([]string{}, ledger...)
				ledgerMu.Unlock()
				if len(events) != 0 {
					t.Fatal("non-SOCKS traffic entered chain")
				}
			}
		})
	}
}
