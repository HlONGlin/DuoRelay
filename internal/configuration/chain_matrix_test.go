package configuration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	parserdefaults "github.com/holi/duorelay/internal/parser/defaults"
	"gopkg.in/yaml.v3"
)

type chainCheckRuntime struct {
	chainRuntime
	validator *kernel.Manager
}

func (r *chainCheckRuntime) Validate(k, path string) error { return r.validator.Validate(k, path) }

func chainFixture(t *testing.T, protocol string) *ir.Node {
	t.Helper()
	n := &ir.Node{Name: protocol, Protocol: protocol, Owner: ir.GetProtocolOwner(protocol), Server: ir.Server{Host: "127.0.0.1", Port: 14443}, Transport: ir.Transport{Type: "tcp"}, Proto: map[string]any{"udp": true}, Extra: map[string]any{}}
	registry := parserdefaults.New()
	if data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "samples", protocol+"_samples.json")); err == nil {
		var fixtures []struct {
			Link string `json:"link"`
		}
		if json.Unmarshal(data, &fixtures) == nil && len(fixtures) > 0 && fixtures[0].Link != "" {
			value, err := registry.ParseAny(fixtures[0].Link)
			if err != nil {
				t.Fatal(err)
			}
			n = value.(*ir.Node)
		}
	}
	switch protocol {
	case "http":
	case "socks5":
	case "shadowsocks":
		n.Proto["method"] = "aes-128-gcm"
		n.Proto["password"] = "chain-test"
	case "vmess":
		n.Proto["uuid"] = "418048af-a293-4b99-9b0c-98ca3580dd24"
		n.Proto["security"] = "auto"
	case "vless":
		n.Proto["uuid"] = "418048af-a293-4b99-9b0c-98ca3580dd24"
	case "trojan", "hysteria2":
		n.Proto["password"] = "chain-test"
		n.Security = ir.Security{Type: "tls", SNI: "example.com"}
	case "tuic":
		n.Proto["uuid"] = "418048af-a293-4b99-9b0c-98ca3580dd24"
		n.Proto["password"] = "chain-test"
		n.Security = ir.Security{Type: "tls", SNI: "example.com"}
	case "hysteria":
		n.Proto["auth"] = "chain-test"
		n.Proto["up_mbps"] = "20"
		n.Proto["down_mbps"] = "100"
		n.Security = ir.Security{Type: "tls", SNI: "example.com"}
	case "snell":
		n.Proto["version"] = 4
		n.Proto["psk"] = "chain-test"
	case "ssh":
		n.Proto["username"] = "test"
		n.Proto["password"] = "test"
	case "wireguard":
		n.Proto = map[string]any{"private_key": "YJX5N5XkMrzxmVzqzpXqbreUjLIuBv6E9Aobdrc4z1o=", "public_key": "12SmDlc4k7EwUTXU0MaBnwJI6afMwbVT8/qnkejc7Eg=", "address": []string{"10.0.0.2/32"}, "allowed_ips": []string{"0.0.0.0/0"}}
	case "zerotier":
		// The documentation identity is deliberately redacted; let the core
		// create a temporary identity instead of treating it as a real secret.
		delete(n.Proto, "identity_secret")
		n.Proto["state_dir"] = "zerotier-chain-test"
	case "tailscale":
		n.Proto["state_directory"] = "tailscale-chain-test"
	case "shadowsocksr":
		n.Proto = map[string]any{"password": "chain-test", "method": "aes-256-cfb", "protocol": "origin", "obfs": "plain", "udp": true}
	case "shadowtls":
		n.Proto["password"] = "chain-test"
		n.Proto["version"] = 3
		n.Security = ir.Security{Type: "tls", SNI: "example.com"}
	case "openconnect":
		n.Proto["flavor"] = "anyconnect"
		n.Proto["username"] = "test"
		n.Proto["password"] = "test"
		n.Security = ir.Security{Type: "tls", SNI: "example.com"}
	case "openvpn":
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "chain-test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
		der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
		if err != nil {
			t.Fatal(err)
		}
		n.Proto = map[string]any{"mode": "tls", "network": "udp", "username": "chain-test", "password": "chain-test", "ca": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))}
		n.Transport.Type = "udp"
	}
	n.ID = ""
	n.Name = protocol
	return n
}

// Checks every declared protocol in entry and exit position against the pinned
// binaries. Starting is mocked: a schema check cannot establish remote reachability.
func TestChainProtocolKernelChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("real kernel config validation")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "kernels"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sing-box.exe")); err != nil {
		t.Skip("pinned kernels unavailable")
	}
	for _, protocol := range ir.GetAllProtocols() {
		if protocol == ir.ProtocolRematch {
			continue
		} // non-network routing adapter, covered by planner tests
		for _, k := range []ir.KernelType{ir.KernelSingBox, ir.KernelMihomo} {
			fixture := chainFixture(t, protocol)
			if !fixture.CanRunOn(k) {
				continue
			}
			for _, position := range []string{"entry", "exit"} {
				if protocol == ir.ProtocolShadowTLS && position == "exit" {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/%s", protocol, k, position), func(t *testing.T) {
					s, _ := testService(t)
					s.Runtime = &chainCheckRuntime{validator: kernel.NewManager(dir)}
					n := chainFixture(t, protocol)
					peer := chainFixture(t, ir.ProtocolAnyTLS)
					if protocol == ir.ProtocolAnyTLS {
						peer = chainFixture(t, ir.ProtocolVMess)
					}
					nodes := []*ir.Node{peer, n}
					if position == "entry" {
						nodes = []*ir.Node{n, peer}
					}
					if protocol == ir.ProtocolShadowTLS {
						nodes[1] = chainFixture(t, ir.ProtocolShadowsocks)
					}
					var ids []string
					for _, node := range nodes {
						if err := s.Store.SaveNode(context.Background(), node); err != nil {
							t.Fatal(err)
						}
						ids = append(ids, node.ID)
					}
					state, err := s.Apply(context.Background(), ApplyRequest{Mode: "chain", NodeIDs: ids, Kernel: k})
					if err != nil {
						t.Fatal(err)
					}
					if state.Chain.Kernel != k {
						t.Fatal("explicit kernel ignored")
					}
				})
			}
		}
	}
}

func TestBlacklistChainKeepsDirectDefault(t *testing.T) {
	for _, k := range []ir.KernelType{ir.KernelSingBox, ir.KernelMihomo} {
		s, _ := testService(t)
		settings := defaults()
		settings.Kernel = k
		settings.RoutingMode = "blacklist"
		settings.RoutingRules = []generator.RoutingRule{{Match: "domain", Values: []string{"proxy.example"}, Outbound: "PROXY"}}
		if err := s.SaveSettings(context.Background(), settings); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for i := 0; i < 2; i++ {
			n := chainFixture(t, "http")
			n.Name = fmt.Sprintf("hop-%d", i)
			if err := s.Store.SaveNode(context.Background(), n); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, n.ID)
		}
		state, err := s.Apply(context.Background(), ApplyRequest{NodeIDs: ids, Mode: "chain"})
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(state.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "DIRECT") || !strings.Contains(string(data), "proxy.example") {
			t.Fatal("blacklist rules lost")
		}
		root := map[string]any{}
		if k == ir.KernelSingBox {
			if err := json.Unmarshal(data, &root); err != nil {
				t.Fatal(err)
			}
			if root["route"].(map[string]any)["final"] != "DIRECT" {
				t.Fatal("blacklist default must remain DIRECT")
			}
			var proxy map[string]any
			for _, entry := range chainEntries(root, "outbounds") {
				if entry["tag"] == "PROXY" {
					proxy = entry
				}
			}
			if proxy == nil || fmt.Sprint(proxy["outbounds"]) != "[hop-1]" {
				t.Fatal("blacklist proxy selector must use the final hop")
			}
		} else {
			if err := yaml.Unmarshal(data, &root); err != nil {
				t.Fatal(err)
			}
			rules := chainArray(root, "rules")
			if len(rules) == 0 || rules[len(rules)-1] != "MATCH,DIRECT" {
				t.Fatal("blacklist default must remain DIRECT")
			}
			groups := chainEntries(root, "proxy-groups")
			if len(groups) != 1 || groups[0]["name"] != "PROXY" || fmt.Sprint(groups[0]["proxies"]) != "[hop-1]" {
				t.Fatal("blacklist proxy group must use the final hop")
			}
		}
	}
}
