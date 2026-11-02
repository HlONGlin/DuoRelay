package mihomo

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	openvpnparser "github.com/holi/duorelay/internal/parser/openvpn"
	"gopkg.in/yaml.v3"
)

func openVPNTestPEM(t *testing.T) (string, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "OpenVPN test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))
}

func parseOpenVPNTestNode(t *testing.T, input string) *ir.Node {
	t.Helper()
	parsed, err := openvpnparser.New().Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.(*ir.Node)
}

func openVPNTestNode(t *testing.T) *ir.Node {
	t.Helper()
	ca, _ := openVPNTestPEM(t)
	node := parseOpenVPNTestNode(t, "openvpn://vpn.example:1194?username=user&password=secret#VPN")
	node.Proto["ca"] = ca
	return node
}

func generatedOpenVPNProxy(t *testing.T, node *ir.Node) (map[string]any, []byte) {
	t.Helper()
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Proxies []map[string]any `yaml:"proxies"`
		Groups  []struct {
			Proxies []string `yaml:"proxies"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Proxies) != 1 || len(config.Groups) != 1 || !reflect.DeepEqual(config.Groups[0].Proxies, []string{"DIRECT", node.Name}) {
		t.Fatalf("unexpected proxies or group membership: %s", generated)
	}
	proxy := config.Proxies[0]
	// Only fields present in v1.19.33's OpenVPNOption may escape to YAML.
	allowed := map[string]bool{}
	for _, field := range strings.Fields("name type server port proto dev cipher auth data-ciphers data-ciphers-fallback comp-lzo ca cert key tls-auth key-direction tls-crypt tls-crypt-v2 username password ping ping-restart mtu") {
		allowed[field] = true
	}
	for key := range proxy {
		if !allowed[key] {
			t.Errorf("unexpected OpenVPNOption field %q", key)
		}
	}
	if proxy["type"] != "openvpn" {
		t.Errorf("type = %v", proxy["type"])
	}
	return proxy, generated
}

func TestGenerateOpenVPNProfile(t *testing.T) {
	ca, key := openVPNTestPEM(t)
	staticKey := "-----BEGIN OpenVPN Static key V1-----\n" + strings.Repeat("ab", 256) + "\n-----END OpenVPN Static key V1-----"
	profile := fmt.Sprintf(`client
dev tun
proto tcp-client
remote vpn.example 443 tcp
cipher AES-256-GCM
auth SHA256
data-ciphers AES-256-GCM:AES-128-GCM
data-ciphers-fallback AES-128-CBC
comp-lzo adaptive
ping 10
ping-restart 60
tun-mtu 1400
key-direction 1
remote-cert-tls server
auth-user-pass
nobind
persist-key
persist-tun
verb 3
mute 20
<ca>
%s</ca>
<cert>
%s</cert>
<key>
%s</key>
<tls-auth>
%s
</tls-auth>
<auth-user-pass>
user
secret
</auth-user-pass>`, ca, ca, key, staticKey)
	node := parseOpenVPNTestNode(t, profile)
	proxy, _ := generatedOpenVPNProxy(t, node)
	if proxy["server"] != "vpn.example" || proxy["port"] != 443 || proxy["mtu"] != 1400 {
		t.Fatalf("incorrect endpoint or MTU: %#v", proxy)
	}
	want := map[string]any{
		"name": node.Name, "type": "openvpn", "server": "vpn.example", "port": 443, "mtu": 1400,
		"proto": "tcp", "dev": "tun", "ca": strings.TrimSpace(ca),
		"cert": strings.TrimSpace(ca), "key": strings.TrimSpace(key), "tls-auth": staticKey,
		"cipher": "AES-256-GCM", "auth": "SHA256", "data-ciphers": []any{"AES-256-GCM", "AES-128-GCM"},
		"data-ciphers-fallback": "AES-128-CBC", "comp-lzo": "adaptive", "ping": 10, "ping-restart": 60,
		"key-direction": "1", "username": "user", "password": "secret",
	}
	if !reflect.DeepEqual(proxy, want) {
		t.Errorf("flat OpenVPN config mismatch\ngot: %#v\nwant: %#v", proxy, want)
	}
}

func TestGenerateOpenVPNURIAndJSONRoundTrip(t *testing.T) {
	for _, roundTrip := range []bool{false, true} {
		t.Run(fmt.Sprint(roundTrip), func(t *testing.T) {
			node := openVPNTestNode(t)
			node.Proto["data_ciphers"] = "AES-256-GCM:AES-128-GCM"
			node.Proto["ping"] = 0
			if roundTrip {
				encoded, err := json.Marshal(node)
				if err != nil {
					t.Fatal(err)
				}
				var decoded ir.Node
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					t.Fatal(err)
				}
				node = &decoded
			}
			proxy, _ := generatedOpenVPNProxy(t, node)
			config := proxy
			if config["username"] != "user" || config["password"] != "secret" || config["proto"] != "udp" || config["ping"] != 0 {
				t.Fatalf("credentials, transport or explicit zero lost: %#v", config)
			}
			if !reflect.DeepEqual(config["data-ciphers"], []any{"AES-256-GCM", "AES-128-GCM"}) {
				t.Errorf("data-ciphers = %#v", config["data-ciphers"])
			}
		})
	}
}

func TestGenerateOpenVPNRejectsUnsupportedInputs(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ir.Node)
		want   string
	}{
		{"static mode", func(n *ir.Node) { n.Proto["mode"] = "static_key" }, "static-key"},
		{"secret", func(n *ir.Node) { n.Proto["secret"] = "sensitive" }, "secret"},
		{"multiple remotes", func(n *ir.Node) {
			n.Proto["remotes"] = []map[string]any{{"host": "vpn.example", "port": 1194}, {"host": "backup.example", "port": 1194}}
		}, "one remote"},
		{"CA path", func(n *ir.Node) { n.Proto["ca_path"] = "ca.pem" }, "ca_path"},
		{"certificate path", func(n *ir.Node) { n.Proto["cert_path"] = "sensitive" }, "cert_path"},
		{"private key path", func(n *ir.Node) { n.Proto["key_path"] = "sensitive" }, "key_path"},
		{"auth file", func(n *ir.Node) { n.Proto["auth_user_pass_file"] = "sensitive" }, "auth_user_pass_file"},
		{"TLS auth file", func(n *ir.Node) { n.Proto["tls_auth_path"] = "sensitive" }, "tls_auth_path"},
		{"remote random", func(n *ir.Node) { n.Proto["remote_random"] = true }, "remote_random"},
		{"route nopull", func(n *ir.Node) { n.Proto["route_nopull"] = true }, "route_nopull"},
		{"x509 identity", func(n *ir.Node) { n.Proto["verify_x509_name"] = "vpn.example name" }, "verify_x509_name"},
		{"compress", func(n *ir.Node) { n.Proto["compress"] = "lz4" }, "compress"},
		{"topology", func(n *ir.Node) { n.Proto["topology"] = "subnet" }, "topology"},
		{"unsupported cipher", func(n *ir.Node) { n.Proto["cipher"] = "BF-CBC" }, "cipher"},
		{"unsupported fallback", func(n *ir.Node) { n.Proto["data_ciphers_fallback"] = "BF-CBC" }, "data-ciphers-fallback"},
		{"unsupported negotiated cipher", func(n *ir.Node) { n.Proto["data_ciphers"] = []string{"BF-CBC"} }, "data-ciphers"},
		{"invalid cipher list", func(n *ir.Node) { n.Proto["data_ciphers"] = []any{42} }, "strings"},
		{"unsupported auth", func(n *ir.Node) { n.Proto["auth"] = "none" }, "auth digest"},
		{"unsupported LZO mode", func(n *ir.Node) { n.Proto["comp_lzo"] = "sensitive" }, "comp-lzo"},
		{"bad static TLS auth key", func(n *ir.Node) { n.Proto["tls_auth"] = "sensitive" }, "256 bytes"},
		{"bad TLS crypt v2 key", func(n *ir.Node) { n.Proto["tls_crypt_v2"] = "sensitive" }, "client PEM"},
		{"unsupported retry flag", func(n *ir.Node) { n.Proto["config"] = "resolv-retry infinite" }, "resolv-retry"},
		{"profile invalid ping", func(n *ir.Node) { n.Proto["config"] = "ping sensitive" }, "integer"},
		{"profile server transport", func(n *ir.Node) { n.Proto["config"] = "proto tcp-server" }, "tcp-server"},
		{"inline credential conflict", func(n *ir.Node) { n.Extra["inline"] = map[string]string{"auth-user-pass": "other\nsensitive"} }, "credentials disagree"},
		{"inline incomplete credentials", func(n *ir.Node) { n.Extra["inline"] = map[string]string{"auth-user-pass": "sensitive"} }, "password lines"},
		{"ignored profile directive", func(n *ir.Node) {
			n.Proto["config"] = "client\nremote vpn.example 1194\nscript-security 2\nup sensitive"
		}, "script-security, up"},
		{"ignored inline block", func(n *ir.Node) { n.Proto["config"] = "<pkcs12>\nsensitive\n</pkcs12>" }, "<pkcs12>"},
		{"tap device", func(n *ir.Node) { n.Proto["dev"] = "tap" }, "dev tun"},
		{"generic TLS option", func(n *ir.Node) { n.Security.SNI = "vpn.example" }, "generic TLS options"},
		{"generic transport option", func(n *ir.Node) { n.Transport.Path = "/ws" }, "transport paths"},
		{"remote mismatch", func(n *ir.Node) { n.Server.Host = "different.example" }, "disagree"},
		{"network mismatch", func(n *ir.Node) { n.Transport.Type = "tcp" }, "disagree"},
		{"missing CA", func(n *ir.Node) { delete(n.Proto, "ca") }, "inline PEM ca"},
		{"missing credentials", func(n *ir.Node) { delete(n.Proto, "username"); delete(n.Proto, "password") }, "requires username"},
		{"incomplete certificate", func(n *ir.Node) { n.Proto["cert"] = n.Proto["ca"] }, "both be set"},
		{"negative ping", func(n *ir.Node) { n.Proto["ping"] = -1 }, "non-negative"},
		{"fractional ping", func(n *ir.Node) { n.Proto["ping"] = 1.5 }, "invalid"},
		{"invalid MTU", func(n *ir.Node) { n.Proto["tun_mtu"] = 0 }, "positive"},
		{"MTU conflict", func(n *ir.Node) { n.Proto["tun_mtu"] = 1400; n.Proto["mtu"] = 1500 }, "disagree"},
		{"control keys conflict", func(n *ir.Node) { n.Proto["tls_auth"] = "sensitive"; n.Proto["tls_crypt"] = "sensitive" }, "mutually exclusive"},
		{"auth prompt", func(n *ir.Node) { n.Proto["auth_user_pass"] = true; delete(n.Proto, "username") }, "prompting"},
		{"extra fields", func(n *ir.Node) { n.Extra["unsupported"] = "sensitive" }, "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := openVPNTestNode(t)
			tt.change(node)
			generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNode(), node}, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if len(generated) != 0 {
				t.Errorf("returned a partial/fake config for rejected OpenVPN input")
			}
			if strings.Contains(err.Error(), "sensitive") {
				t.Errorf("error leaked credential/key contents: %v", err)
			}
		})
	}
}

func TestGeneratedOpenVPNConfigPassesPinnedKernelCheck(t *testing.T) {
	binary, err := filepath.Abs(filepath.Join("..", "..", "..", "kernels", kernelBinary()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Skip("local pinned mihomo binary not available")
	}
	version, err := exec.Command(binary, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("mihomo version: %v\n%s", err, version)
	}
	if !strings.Contains(string(version), "v1.19.33 ") {
		t.Skipf("requires pinned v1.19.33; found %s", version)
	}
	t.Log(strings.TrimSpace(string(version)))
	for _, variant := range []string{"udp-password", "tcp-client-cert", "tls-auth", "tls-crypt", "tls-crypt-v2", "encoded-profile"} {
		t.Run(variant, func(t *testing.T) {
			node := openVPNTestNode(t)
			ca, key := openVPNTestPEM(t)
			staticKey := "-----BEGIN OpenVPN Static key V1-----\n" + strings.Repeat("ab", 256) + "\n-----END OpenVPN Static key V1-----"
			switch variant {
			case "tcp-client-cert":
				node.Transport.Type = "tcp"
				node.Proto["network"] = "tcp"
				node.Proto["remotes"] = []map[string]any{{"host": "vpn.example", "port": 1194, "network": "tcp"}}
				node.Proto["cert"], node.Proto["key"] = ca, key
				delete(node.Proto, "username")
				delete(node.Proto, "password")
			case "tls-auth", "tls-crypt":
				node.Proto[strings.ReplaceAll(variant, "-", "_")] = staticKey
			case "tls-crypt-v2":
				// Pinned decoder: 256-byte client key followed by nonempty wrapped-key bytes.
				node.Proto["tls_crypt_v2"] = "-----BEGIN OpenVPN tls-crypt-v2 client key-----\n" + base64.StdEncoding.EncodeToString(make([]byte, 288)) + "\n-----END OpenVPN tls-crypt-v2 client key-----"
			case "encoded-profile":
				profile := "client\ndev tun\nproto udp\nremote vpn.example 1194\n<ca>\n" + ca + "</ca>\n<auth-user-pass>\nuser\nsecret\n</auth-user-pass>"
				node = parseOpenVPNTestNode(t, "openvpn://"+base64.StdEncoding.EncodeToString([]byte(profile))+"#Encoded")
			}
			node.Proto["ping"], node.Proto["ping_restart"], node.Proto["tun_mtu"] = 10, 60, 1400
			node.Proto["data_ciphers"] = []string{"AES-256-GCM", "AES-128-GCM"}
			node.Proto["data_ciphers_fallback"] = "AES-128-CBC"
			node.Proto["comp_lzo"] = "adaptive"
			_, generated := generatedOpenVPNProxy(t, node)
			directory := t.TempDir()
			path := filepath.Join(directory, "config.yaml")
			if err := os.WriteFile(path, generated, 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "-t", "-d", directory, "-f", path)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("mihomo v1.19.33 -t failed: %v\n%s", err, output)
			} else {
				t.Logf("%s", output)
			}
		})
	}
}
