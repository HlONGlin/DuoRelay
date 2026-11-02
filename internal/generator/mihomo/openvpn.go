package mihomo

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/holi/duorelay/internal/ir"
)

// This is deliberately the v1.19.33 subset, not a generic .ovpn passthrough.
// See adapter/outbound/openvpn.go and transport/openvpn/config.go at the pinned
// binary's revision 3f27ef39f8c96bbfbc8b0b9bb10f5c9ef60149d8. Its schema is
// flat: there is no config/profile passthrough or certificate file loader.
// Only one remote and TLS client mode are implemented by this kernel.
type openVPNFields struct {
	Config              string `json:"config"`
	Mode                string `json:"mode"`
	Network             string `json:"network"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	Dev                 string `json:"dev"`
	Cipher              string `json:"cipher"`
	Auth                string `json:"auth"`
	DataCiphers         any    `json:"data_ciphers"`
	DataCiphersFallback string `json:"data_ciphers_fallback"`
	CompLZO             string `json:"comp_lzo"`
	CA                  string `json:"ca"`
	Cert                string `json:"cert"`
	Key                 string `json:"key"`
	TLSAuth             string `json:"tls_auth"`
	TLSCrypt            string `json:"tls_crypt"`
	TLSCryptV2          string `json:"tls_crypt_v2"`
	KeyDirection        string `json:"key_direction"`
	Ping                *int   `json:"ping"`
	PingRestart         *int   `json:"ping_restart"`
	MTU                 *int   `json:"mtu"`
	TunMTU              *int   `json:"tun_mtu"`
	AuthUserPass        bool   `json:"auth_user_pass"`
	RemoteCertTLS       string `json:"remote_cert_tls"`
	Remotes             []struct {
		Host    string `json:"host"`
		Port    int    `json:"port"`
		Network string `json:"network"`
	} `json:"remotes"`
}

func (g *Generator) convertOpenVPN(node *ir.Node) (map[string]any, error) {
	encoded, err := json.Marshal(node.Proto)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenVPN IR fields: %w", err)
	}
	var fields openVPNFields
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("unsupported or invalid mihomo v1.19.33 OpenVPN input: %w", err)
	}
	if fields.Mode != "" && fields.Mode != "tls" {
		return nil, fmt.Errorf("mihomo v1.19.33 supports TLS client mode only (static-key tunnels are unsupported)")
	}
	if node.Security.Type != "" && node.Security.Type != "tls" {
		return nil, fmt.Errorf("unsupported OpenVPN security mode")
	}
	if node.Security.SNI != "" || len(node.Security.ALPN) != 0 || node.Security.Fingerprint != "" || len(node.Security.Opts) != 0 {
		return nil, fmt.Errorf("mihomo v1.19.33 OpenVPN does not support generic TLS options (SNI, ALPN, fingerprint or security opts)")
	}
	if node.Transport.Path != "" || node.Transport.Host != "" || len(node.Transport.Opts) != 0 {
		return nil, fmt.Errorf("mihomo v1.19.33 OpenVPN does not support transport paths, hosts or opts")
	}
	if err := validateOpenVPNProfile(fields.Config); err != nil {
		return nil, err
	}
	if len(fields.Remotes) > 1 {
		return nil, fmt.Errorf("mihomo v1.19.33 OpenVPN supports one remote; got %d", len(fields.Remotes))
	}
	host, port, network := node.Server.Host, node.Server.Port, fields.Network
	if len(fields.Remotes) == 1 {
		remote := fields.Remotes[0]
		if host != "" && host != remote.Host || port != 0 && port != remote.Port {
			return nil, fmt.Errorf("OpenVPN server and primary remote disagree")
		}
		host, port = remote.Host, remote.Port
		if remote.Network != "" {
			network = remote.Network
		}
	}
	if node.Transport.Type != "" {
		if network != "" && network != node.Transport.Type {
			return nil, fmt.Errorf("OpenVPN transport and primary remote disagree")
		}
		network = node.Transport.Type
	}
	if network == "" {
		network = "udp"
	}
	if network != "udp" && network != "tcp" {
		return nil, fmt.Errorf("OpenVPN transport must be udp or tcp")
	}
	if strings.TrimSpace(host) == "" || port < 1 || port > 65535 {
		return nil, fmt.Errorf("OpenVPN requires a valid remote host and port")
	}
	if fields.Dev != "" && fields.Dev != "tun" {
		return nil, fmt.Errorf("mihomo v1.19.33 OpenVPN supports dev tun only")
	}
	if fields.RemoteCertTLS != "" && fields.RemoteCertTLS != "server" {
		return nil, fmt.Errorf("OpenVPN remote-cert-tls must be server")
	}
	if fields.KeyDirection != "" && fields.KeyDirection != "0" && fields.KeyDirection != "1" {
		return nil, fmt.Errorf("OpenVPN key-direction must be 0 or 1")
	}
	for name, value := range map[string]string{"cipher": fields.Cipher, "data-ciphers-fallback": fields.DataCiphersFallback} {
		if value != "" && !openVPNCipherSupported(value) {
			return nil, fmt.Errorf("unsupported mihomo v1.19.33 OpenVPN %s", name)
		}
	}
	switch strings.ToUpper(strings.TrimSpace(fields.Auth)) {
	case "", "MD5", "SHA1", "SHA-1", "SHA256", "SHA384", "SHA512":
	default:
		return nil, fmt.Errorf("unsupported mihomo v1.19.33 OpenVPN auth digest")
	}
	switch strings.ToLower(strings.TrimSpace(fields.CompLZO)) {
	case "", "yes", "adaptive", "no":
	default:
		return nil, fmt.Errorf("unsupported mihomo v1.19.33 OpenVPN comp-lzo mode")
	}
	if err := openVPNInlineCredentials(node.Extra, &fields); err != nil {
		return nil, err
	}
	if block, _ := pem.Decode([]byte(fields.CA)); block == nil {
		return nil, fmt.Errorf("OpenVPN requires an inline PEM ca; CA file paths are unsupported")
	}
	hasCert := fields.Cert != ""
	hasKey := fields.Key != ""
	if hasCert != hasKey {
		return nil, fmt.Errorf("OpenVPN client certificate and key must both be set")
	}
	if !hasCert && strings.TrimSpace(fields.Username) == "" {
		return nil, fmt.Errorf("OpenVPN requires username credentials or a client certificate/key pair")
	}
	for name, value := range map[string]string{"cert": fields.Cert, "key": fields.Key} {
		if value != "" {
			if block, _ := pem.Decode([]byte(value)); block == nil {
				return nil, fmt.Errorf("OpenVPN inline %s must be PEM", name)
			}
		}
	}
	controlKeys := 0
	for _, value := range []string{fields.TLSAuth, fields.TLSCrypt, fields.TLSCryptV2} {
		if value != "" {
			controlKeys++
		}
	}
	if controlKeys > 1 {
		return nil, fmt.Errorf("OpenVPN tls-auth, tls-crypt and tls-crypt-v2 are mutually exclusive")
	}
	for name, value := range map[string]string{"tls-auth": fields.TLSAuth, "tls-crypt": fields.TLSCrypt} {
		if value != "" {
			var lines []string
			for _, line := range strings.Split(value, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-----BEGIN OpenVPN Static key") || strings.HasPrefix(line, "-----END OpenVPN Static key") {
					continue
				}
				lines = append(lines, line)
			}
			key, err := hex.DecodeString(strings.Join(lines, ""))
			if err != nil || len(key) != 256 {
				return nil, fmt.Errorf("OpenVPN %s requires 256 bytes of hex static key material", name)
			}
		}
	}
	if fields.TLSCryptV2 != "" {
		block, _ := pem.Decode([]byte(fields.TLSCryptV2))
		if block == nil || block.Type != "OpenVPN tls-crypt-v2 client key" || len(block.Bytes) <= 256 {
			return nil, fmt.Errorf("OpenVPN tls-crypt-v2 requires client PEM with a wrapped key")
		}
	}
	proxy := map[string]any{"name": node.Name, "type": "openvpn", "server": host, "port": port, "proto": network, "ca": fields.CA}
	for key, value := range map[string]string{
		"dev": fields.Dev, "cipher": fields.Cipher, "auth": fields.Auth,
		"data-ciphers-fallback": fields.DataCiphersFallback, "comp-lzo": fields.CompLZO,
		"cert": fields.Cert, "key": fields.Key, "tls-auth": fields.TLSAuth,
		"tls-crypt": fields.TLSCrypt, "tls-crypt-v2": fields.TLSCryptV2,
		"key-direction": fields.KeyDirection, "username": fields.Username, "password": fields.Password,
	} {
		if value != "" {
			proxy[key] = value
		}
	}
	if fields.DataCiphers != nil {
		ciphers, err := openVPNDataCiphers(fields.DataCiphers)
		if err != nil {
			return nil, err
		}
		proxy["data-ciphers"] = ciphers
	}
	for key, value := range map[string]*int{"ping": fields.Ping, "ping-restart": fields.PingRestart} {
		if value != nil {
			if *value < 0 || int64(*value) > int64((time.Duration(1<<63-1))/time.Second) {
				return nil, fmt.Errorf("OpenVPN %s must be non-negative and fit a duration in seconds", key)
			}
			proxy[key] = *value
		}
	}
	if fields.MTU != nil && fields.TunMTU != nil && *fields.MTU != *fields.TunMTU {
		return nil, fmt.Errorf("OpenVPN mtu and tun-mtu disagree")
	}
	mtu := fields.MTU
	if mtu == nil {
		mtu = fields.TunMTU
	}
	if mtu != nil {
		if *mtu < 1 {
			return nil, fmt.Errorf("OpenVPN mtu must be positive")
		}
		proxy["mtu"] = *mtu
	}
	return proxy, nil
}

func openVPNDataCiphers(value any) ([]string, error) {
	var result []string
	switch value := value.(type) {
	case string:
		result = strings.FieldsFunc(value, func(r rune) bool { return r == ':' || r == ',' })
	case []any:
		for _, item := range value {
			cipher, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("OpenVPN data-ciphers must contain strings")
			}
			result = append(result, cipher)
		}
	default:
		return nil, fmt.Errorf("OpenVPN data-ciphers must be a list or colon-separated string")
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("OpenVPN data-ciphers must not be empty")
	}
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
		if result[i] == "" {
			return nil, fmt.Errorf("OpenVPN data-ciphers must not contain empty entries")
		}
		if !openVPNCipherSupported(result[i]) {
			return nil, fmt.Errorf("unsupported mihomo v1.19.33 OpenVPN data-ciphers entry")
		}
	}
	return result, nil
}

func openVPNCipherSupported(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "AES-CBC", "AES-128-CBC", "AES-192-CBC", "AES-256-CBC", "AES-128-GCM", "AES-192-GCM", "AES-256-GCM", "CHACHA20-POLY1305":
		return true
	default:
		return false
	}
}

func openVPNInlineCredentials(extra map[string]any, fields *openVPNFields) error {
	for key := range extra {
		if key != "inline" {
			return fmt.Errorf("unsupported OpenVPN extra field %q", key)
		}
	}
	if value, ok := extra["inline"]; ok {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("invalid OpenVPN inline blocks")
		}
		var blocks map[string]string
		if err := json.Unmarshal(data, &blocks); err != nil {
			return fmt.Errorf("invalid OpenVPN inline blocks")
		}
		for tag, content := range blocks {
			switch tag {
			case "ca", "cert", "key", "tls-auth", "tls-crypt", "tls-crypt-v2":
				// The parser also exposes these in Proto; do not duplicate them.
			case "auth-user-pass":
				lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
				if len(lines) != 2 || lines[0] == "" {
					return fmt.Errorf("OpenVPN inline auth-user-pass requires username and password lines")
				}
				if fields.Username != "" && fields.Username != lines[0] || fields.Password != "" && fields.Password != lines[1] {
					return fmt.Errorf("OpenVPN inline and explicit credentials disagree")
				}
				fields.Username, fields.Password = lines[0], lines[1]
			default:
				return fmt.Errorf("unsupported OpenVPN inline block <%s>", tag)
			}
		}
	}
	if fields.AuthUserPass && fields.Username == "" {
		return fmt.Errorf("OpenVPN auth-user-pass requires explicit or inline username credentials; prompting is unsupported")
	}
	return nil
}

// The IR retains the original profile, including directives it does not model.
// Inspect it so unsupported security/routing options cannot disappear silently.
func validateOpenVPNProfile(profile string) error {
	allowed := map[string]bool{}
	// client/tls-client, nobind and persistence flags describe behavior inherent
	// to this client outbound; verb/mute are profile-local logging preferences.
	for _, key := range strings.Fields("client tls-client dev proto remote cipher auth data-ciphers data-ciphers-fallback comp-lzo ca cert key tls-auth tls-crypt tls-crypt-v2 key-direction ping ping-restart tun-mtu mtu auth-user-pass remote-cert-tls nobind persist-key persist-tun verb mute") {
		allowed[key] = true
	}
	var unsupported []string
	scanner := bufio.NewScanner(strings.NewReader(profile))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	block := ""
	remotes := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if block != "" {
			if strings.EqualFold(line, "</"+block+">") {
				block = ""
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
			block = strings.ToLower(strings.Trim(line, "<>"))
			switch block {
			case "ca", "cert", "key", "tls-auth", "tls-crypt", "tls-crypt-v2", "auth-user-pass":
			default:
				unsupported = append(unsupported, "<"+block+">")
			}
			continue
		}
		tokens := strings.Fields(line)
		key := strings.ToLower(tokens[0])
		if !allowed[key] {
			unsupported = append(unsupported, key)
		}
		if key == "remote" {
			remotes++
		}
		switch key {
		case "dev", "proto", "cipher", "auth", "data-ciphers", "data-ciphers-fallback", "comp-lzo", "key-direction", "remote-cert-tls":
			if len(tokens) < 2 {
				return fmt.Errorf("OpenVPN profile directive %s requires an explicit value", key)
			}
		case "ping", "ping-restart", "tun-mtu", "mtu":
			if len(tokens) < 2 {
				return fmt.Errorf("OpenVPN profile directive %s requires an integer", key)
			}
			if _, err := strconv.Atoi(tokens[1]); err != nil {
				return fmt.Errorf("OpenVPN profile directive %s requires an integer", key)
			}
		}
		if (key == "proto" && len(tokens) > 1 && strings.EqualFold(tokens[1], "tcp-server")) ||
			(key == "remote" && len(tokens) > 3 && strings.EqualFold(tokens[3], "tcp-server")) {
			return fmt.Errorf("OpenVPN tcp-server transport is unsupported for a client outbound")
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read OpenVPN profile: %w", err)
	}
	if block != "" {
		return fmt.Errorf("unterminated OpenVPN inline block")
	}
	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return fmt.Errorf("unsupported mihomo v1.19.33 OpenVPN profile directives: %s", strings.Join(unsupported, ", "))
	}
	if remotes > 1 {
		return fmt.Errorf("mihomo v1.19.33 OpenVPN supports one remote; profile has %d", remotes)
	}
	return nil
}
