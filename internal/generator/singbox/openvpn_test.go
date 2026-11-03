//go:build !lite

package singbox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	ovpnparser "github.com/holi/duorelay/internal/parser/openvpn"
)

func testPKI(t *testing.T) (ca, cert, key string) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	encode := func(kind string, der []byte) string {
		return strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der})))
	}
	return encode("CERTIFICATE", caDER), encode("CERTIFICATE", clientDER), encode("PRIVATE KEY", keyDER)
}

func testStaticKey(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 256)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	encoded := hex.EncodeToString(raw)
	var lines []string
	lines = append(lines, "-----BEGIN OpenVPN Static key V1-----")
	for i := 0; i < len(encoded); i += 32 {
		lines = append(lines, encoded[i:i+32])
	}
	lines = append(lines, "-----END OpenVPN Static key V1-----")
	return strings.Join(lines, "\n")
}

func parseOpenVPN(t *testing.T, profile string) *ir.Node {
	t.Helper()
	parsed, err := ovpnparser.New().Parse(profile)
	if err != nil {
		t.Fatalf("parse profile: %v", err)
	}
	return parsed.(*ir.Node)
}

func generateOpenVPN(t *testing.T, node *ir.Node) (map[string]any, []byte) {
	t.Helper()
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	for _, item := range config["outbounds"].([]any) {
		if item.(map[string]any)["tag"] == node.Name {
			t.Fatalf("OpenVPN node leaked into outbounds: %v", item)
		}
	}
	endpoints, ok := config["endpoints"].([]any)
	if !ok || len(endpoints) != 1 {
		t.Fatalf("want exactly one root endpoint, got %v", config["endpoints"])
	}
	return endpoints[0].(map[string]any), generated
}

func singBoxCheck(t *testing.T, generated []byte) {
	t.Helper()
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box check failed: %v\n%s\n%s", err, output, generated)
	}
}

func TestGenerateOpenVPNTLSProfile(t *testing.T) {
	ca, cert, key := testPKI(t)
	profile := `client
dev tun
proto udp
remote vpn1.example.com 1194
remote vpn2.example.com 443 tcp
remote-random
cipher AES-256-CBC
data-ciphers AES-256-GCM:CHACHA20-POLY1305
auth SHA256
verify-x509-name vpn1.example.com name
remote-cert-tls server
auth-user-pass
route-nopull
route 10.8.0.0 255.255.255.0
route 192.168.0.0 255.255.0.0 vpn_gateway
route-ipv6 fd00::/64
redirect-gateway def1
pull-filter ignore "dhcp-option DNS"
reneg-sec 0
tls-version-min 1.2
compress lz4-v2
mssfix 1300
explicit-exit-notify 2
key-direction 1
<ca>
` + ca + `
</ca>
<cert>
` + cert + `
</cert>
<key>
` + key + `
</key>
<tls-auth>
` + testStaticKey(t) + `
</tls-auth>`
	node := parseOpenVPN(t, profile)
	node.Name = "ovpn-tls"
	// auth-user-pass credentials come from the subscription/user, not the profile.
	node.Proto["username"] = "alice"
	node.Proto["password"] = "s3cret"

	endpoint, generated := generateOpenVPN(t, node)

	if endpoint["type"] != "openvpn-client" || endpoint["tag"] != "ovpn-tls" || endpoint["mode"] != "tls" {
		t.Fatalf("unexpected header: %v", endpoint)
	}
	servers := endpoint["servers"].([]any)
	if len(servers) != 2 {
		t.Fatalf("servers = %v", servers)
	}
	first, second := servers[0].(map[string]any), servers[1].(map[string]any)
	if first["server"] != "vpn1.example.com" || first["server_port"] != float64(1194) || first["network"] != "udp" {
		t.Errorf("first remote = %v", first)
	}
	if second["server"] != "vpn2.example.com" || second["server_port"] != float64(443) || second["network"] != "tcp" {
		t.Errorf("second remote = %v", second)
	}
	if endpoint["remote_random"] != true || endpoint["network"] != "udp" {
		t.Errorf("remote_random/network = %v/%v", endpoint["remote_random"], endpoint["network"])
	}
	if endpoint["username"] != "alice" || endpoint["password"] != "s3cret" {
		t.Errorf("credentials missing: %v", endpoint)
	}
	if _, ok := endpoint["cipher"]; ok {
		t.Errorf("cipher must not be emitted in TLS mode")
	}
	if endpoint["data_ciphers_fallback"] != "AES-256-CBC" || endpoint["auth"] != "SHA256" {
		t.Errorf("cipher fallback/auth = %v/%v", endpoint["data_ciphers_fallback"], endpoint["auth"])
	}
	if endpoint["route_no_pull"] != true || endpoint["redirect_gateway"] != true {
		t.Errorf("route flags = %v", endpoint)
	}
	if flags := endpoint["redirect_gateway_flags"].([]any); len(flags) != 1 || flags[0] != "def1" {
		t.Errorf("redirect_gateway_flags = %v", flags)
	}
	routes := endpoint["routes"].([]any)
	if len(routes) != 3 || routes[0] != "10.8.0.0/24" || routes[1] != "192.168.0.0/16" || routes[2] != "fd00::/64" {
		t.Errorf("routes = %v", routes)
	}
	filters := endpoint["pull_filters"].([]any)
	if len(filters) != 1 || filters[0].(map[string]any)["text"] != "dhcp-option DNS" {
		t.Errorf("pull_filters = %v", filters)
	}
	if endpoint["renegotiate_disabled"] != true || endpoint["compression"] != "lz4-v2" ||
		endpoint["mss_fix"] != float64(1300) || endpoint["explicit_exit_notify"] != float64(2) {
		t.Errorf("tuning fields = %v", endpoint)
	}

	tls := endpoint["tls"].(map[string]any)
	if tls["certificate"] != ca || tls["client_certificate"] != cert || tls["client_key"] != key {
		t.Errorf("inline certificates not preserved: %v", tls)
	}
	if tls["server_name"] != "vpn1.example.com" || tls["server_name_type"] != "name" ||
		tls["remote_certificate_tls"] != "server" || tls["version_min"] != "1.2" {
		t.Errorf("tls verification fields = %v", tls)
	}
	wrap := tls["control_wrap"].(map[string]any)
	if wrap["type"] != "tls_auth" || wrap["direction"] != "client" || wrap["key"] == "" {
		t.Errorf("control_wrap = %v", wrap)
	}

	singBoxCheck(t, generated)
}

func TestGenerateOpenVPNTLSCryptAndPaths(t *testing.T) {
	profile := `client
dev tun
remote vpn.example.com 1194
ca /etc/openvpn/ca.crt
cert /etc/openvpn/client.crt
key /etc/openvpn/client.key
tls-crypt /etc/openvpn/tc.key
`
	node := parseOpenVPN(t, profile)
	endpoint, _ := generateOpenVPN(t, node)
	tls := endpoint["tls"].(map[string]any)
	if tls["certificate_path"] != "/etc/openvpn/ca.crt" || tls["client_certificate_path"] != "/etc/openvpn/client.crt" ||
		tls["client_key_path"] != "/etc/openvpn/client.key" {
		t.Errorf("paths not preserved: %v", tls)
	}
	wrap := tls["control_wrap"].(map[string]any)
	if wrap["type"] != "tls_crypt" || wrap["key_path"] != "/etc/openvpn/tc.key" {
		t.Errorf("control_wrap = %v", wrap)
	}
	if tls["remote_certificate_tls"] != "none" {
		t.Errorf("absent remote-cert-tls must be made explicit as none, got %v", tls["remote_certificate_tls"])
	}
}

func TestGenerateOpenVPNShareURI(t *testing.T) {
	// A bare share URI carries no CA/fingerprint, so a runnable TLS endpoint
	// cannot be produced and the generator must say so.
	node := parseOpenVPN(t, "openvpn://vpn.example.com:443?proto=tcp&username=u&password=p&cipher=AES-256-GCM#Office")
	if _, err := NewGenerator().Generate([]*ir.Node{node}, nil); err == nil {
		t.Fatalf("expected an error for TLS mode without CA or peer fingerprint")
	}

	ca, _, _ := testPKI(t)
	node.Proto["ca"] = ca
	endpoint, generated := generateOpenVPN(t, node)
	if endpoint["tag"] != "Office" || endpoint["username"] != "u" || endpoint["password"] != "p" {
		t.Errorf("endpoint = %v", endpoint)
	}
	servers := endpoint["servers"].([]any)
	if len(servers) != 1 || servers[0].(map[string]any)["network"] != "tcp" {
		t.Errorf("servers = %v", servers)
	}
	singBoxCheck(t, generated)
}

func TestGenerateOpenVPNStaticKey(t *testing.T) {
	secret := testStaticKey(t)
	profile := `dev tun
proto udp
remote vpn.example.com 1194
ifconfig 10.8.0.2 10.8.0.1
cipher AES-256-CBC
auth SHA256
key-direction 1
<secret>
` + secret + `
</secret>`
	node := parseOpenVPN(t, profile)
	node.Name = "ovpn-static"
	endpoint, generated := generateOpenVPN(t, node)
	if endpoint["mode"] != "static_key" || endpoint["static_key"] != secret || endpoint["key_direction"] != "client" {
		t.Errorf("static key mapping = %v", endpoint)
	}
	if endpoint["cipher"] != "AES-256-CBC" || endpoint["peer_address"] != "10.8.0.1" {
		t.Errorf("cipher/peer = %v", endpoint)
	}
	if addr := endpoint["address"].([]any); len(addr) != 1 || addr[0] != "10.8.0.2/30" {
		t.Errorf("address = %v", addr)
	}
	if _, ok := endpoint["tls"]; ok {
		t.Errorf("tls must be absent in static_key mode")
	}
	singBoxCheck(t, generated)
}

func TestGenerateOpenVPNRejectsUnsupported(t *testing.T) {
	ca, _, _ := testPKI(t)
	base := "client\ndev tun\nremote vpn.example.com 1194\n<ca>\n" + ca + "\n</ca>\n"
	cases := map[string]string{
		"tap device":         strings.Replace(base, "dev tun", "dev tap", 1),
		"http proxy":         base + "http-proxy proxy.local 8080\n",
		"pkcs12":             base + "pkcs12 client.p12\n",
		"block-local":        base + "redirect-gateway def1 block-local\n",
		"route gateway":      base + "route 10.0.0.0 255.0.0.0 192.168.1.1\n",
		"user-pass no creds": base + "auth-user-pass\n",
		"tcp-server":         strings.Replace(base, "dev tun", "dev tun\nproto tcp-server", 1),
		"bad fingerprint":    base + "peer-fingerprint zz\n",
		"static without if":  "dev tun\nremote vpn.example.com 1194\n<secret>\n" + testStaticKey(t) + "\n</secret>\n",
	}
	for name, profile := range cases {
		t.Run(name, func(t *testing.T) {
			node := parseOpenVPN(t, profile)
			if _, err := NewGenerator().Generate([]*ir.Node{node}, nil); err == nil {
				t.Fatalf("expected error, got none")
			}
		})
	}

	insecure := parseOpenVPN(t, base)
	insecure.Security.Opts = map[string]any{"insecure": true}
	if _, err := NewGenerator().Generate([]*ir.Node{insecure}, nil); err == nil {
		t.Fatalf("expected error for insecure TLS")
	}
}

func TestOpenVPNNeverEmittedAsOutbound(t *testing.T) {
	node := &ir.Node{Name: "bare", Protocol: ir.ProtocolOpenVPN, Owner: ir.KernelSetBoth,
		Server: ir.Server{Host: "vpn.example.com", Port: 1194}, Transport: ir.Transport{Type: "udp"},
		Proto: map[string]any{"peer_fingerprint": "x"}}
	if g := NewGenerator(); g.convertNode(node) != nil {
		t.Fatalf("convertNode must not produce an outbound for OpenVPN")
	}
}
