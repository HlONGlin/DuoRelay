package parser

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

var (
	ErrInvalidFormat      = errors.New("invalid format")
	ErrUnsupportedProto   = errors.New("unsupported protocol")
	ErrMissingField       = errors.New("missing required field")
	ErrInvalidField       = errors.New("invalid field value")
	ErrUnsupportedFeature = errors.New("unsupported feature")
)

// Parser defines the interface for protocol parsers
type Parser interface {
	// Parse parses a share link or config into IR
	Parse(input string) (any, error)

	// CanParse checks if this parser can handle the input
	CanParse(input string) bool

	// Protocol returns the protocol name this parser handles
	Protocol() string
}

// Format identifies the outer representation of an imported node or subscription.
type Format string

const (
	FormatURI     Format = "uri"
	FormatBase64  Format = "base64"
	FormatJSON    Format = "json"
	FormatYAML    Format = "yaml"
	FormatUnknown Format = "unknown"
)

// DetectFormat detects the common import envelope without parsing protocol fields.
func DetectFormat(input string) Format {
	s := strings.TrimSpace(input)
	if s == "" {
		return FormatUnknown
	}
	if strings.Contains(s, "://") {
		return FormatURI
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return FormatJSON
	}
	if strings.HasPrefix(s, "---") || strings.Contains(s, "\n") && strings.Contains(s, ":") {
		return FormatYAML
	}
	if _, err := DecodeBase64(s); err == nil {
		return FormatBase64
	}
	return FormatUnknown
}

// DecodeBase64 accepts standard, raw and URL-safe Base64 encodings.
func DecodeBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	decoders := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	for _, decoder := range decoders {
		if decoded, err := decoder.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, ErrInvalidFormat
}

// ParseShareURI parses and validates the common URI envelope.
func ParseShareURI(input, scheme string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil || !strings.EqualFold(u.Scheme, scheme) || u.Host == "" {
		return nil, fmt.Errorf("%w: expected %s URI", ErrInvalidFormat, scheme)
	}
	if u.Port() == "" {
		return nil, fmt.Errorf("%w: missing port", ErrMissingField)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: invalid port", ErrInvalidField)
	}
	return u, nil
}

// ParseEndpointURI parses a URI whose port may be omitted when a default is known.
func ParseEndpointURI(input string, scheme string, defaultPort int) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil || !strings.EqualFold(u.Scheme, scheme) || u.Hostname() == "" {
		return nil, fmt.Errorf("%w: expected %s URI", ErrInvalidFormat, scheme)
	}
	if u.Port() == "" && defaultPort > 0 {
		u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(defaultPort))
	}
	if u.Port() == "" {
		return nil, fmt.Errorf("%w: missing port", ErrMissingField)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: invalid port", ErrInvalidField)
	}
	return u, nil
}

func NewNode(protocol, name string, u *url.URL) *ir.Node {
	return &ir.Node{
		Name: name, Protocol: protocol, Owner: ir.GetProtocolOwner(protocol),
		Server:    ir.Server{Host: u.Hostname(), Port: mustPort(u.Port())},
		Transport: ir.Transport{Type: "tcp"}, Security: ir.Security{Type: "none"},
		Proto: map[string]any{}, Extra: map[string]any{}, Source: ir.Source{Type: "import", URL: SanitizedSourceURL(u)},
	}
}

// SanitizedSourceURL keeps only the source endpoint. Share-link credentials can
// appear in userinfo, queries, fragments, paths, or opaque payloads, so none of
// those components belong in source metadata. The original URL is not mutated.
func SanitizedSourceURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
}

func mustPort(value string) int { n, _ := strconv.Atoi(value); return n }

func SetName(node *ir.Node, fragment string) {
	node.Name = fragment
	if node.Name == "" {
		node.Name = node.Protocol
	}
}

func SetTLS(node *ir.Node, q url.Values) {
	security := strings.ToLower(q.Get("security"))
	if security == "tls" || security == "reality" || q.Get("sni") != "" || q.Get("allowInsecure") != "" {
		node.Security.Type = security
		if node.Security.Type == "" {
			node.Security.Type = "tls"
		}
		node.Security.SNI = q.Get("sni")
		if q.Get("alpn") != "" {
			node.Security.ALPN = strings.Split(q.Get("alpn"), ",")
		}
		if security == "reality" {
			node.Security.Opts = map[string]any{"public_key": q.Get("pbk"), "short_id": q.Get("sid")}
		}
	}
	if node.Security.Opts == nil {
		node.Security.Opts = map[string]any{}
	}
	node.Security.Fingerprint = firstNonEmpty(q.Get("fp"), q.Get("fingerprint"), q.Get("client-fingerprint"))
	if sni := firstNonEmpty(q.Get("sni"), q.Get("peer"), q.Get("serverName")); sni != "" {
		node.Security.SNI = sni
	}
	if alpn := q.Get("alpn"); alpn != "" {
		node.Security.ALPN = strings.Split(alpn, ",")
	}
	for _, key := range []string{"allowInsecure", "insecure", "skip-cert-verify"} {
		if value := q.Get(key); value != "" {
			if skip, err := strconv.ParseBool(value); err == nil {
				node.Security.Opts["insecure"] = skip
			}
		}
	}
}

func SetTransport(node *ir.Node, q url.Values) error {
	t := strings.ToLower(firstNonEmpty(q.Get("type"), q.Get("net")))
	if t == "" {
		return nil
	}
	switch t {
	case "tcp", "ws", "grpc", "http", "h2", "quic", "xhttp":
	default:
		return fmt.Errorf("%w: unsupported transport %q", ErrInvalidField, t)
	}
	node.Transport.Type = t
	node.Transport.Path = q.Get("path")
	node.Transport.Host = q.Get("host")
	if t == "xhttp" {
		node.Transport.Opts = map[string]any{}
		for _, key := range []string{"extra", "scStreamUpServerSecs", "xmux", "xPaddingObfsMode", "xPaddingVerify"} {
			if q.Get(key) != "" {
				return fmt.Errorf("%w: XHTTP option %s cannot be represented by the pinned kernels", ErrUnsupportedFeature, key)
			}
		}
		mode := q.Get("mode")
		if mode != "" && mode != "auto" && mode != "packet-up" && mode != "stream-up" && mode != "stream-one" {
			return fmt.Errorf("%w: unsupported XHTTP mode", ErrUnsupportedFeature)
		}
		for _, key := range []string{"mode", "scMaxEachPostBytes", "scMinPostsIntervalMs", "xPaddingBytes", "xPaddingKey"} {
			if value := q.Get(key); value != "" {
				node.Transport.Opts[key] = value
			}
		}
	}
	if t == "grpc" {
		node.Transport.Path = firstNonEmpty(q.Get("serviceName"), q.Get("service_name"))
		node.Transport.Opts = map[string]any{"service_name": node.Transport.Path}
	}
	return nil
}

func Require(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s", ErrMissingField, field)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Registry holds all protocol parsers
type Registry struct {
	parsers map[string]Parser
}

// NewRegistry creates a new parser registry
func NewRegistry() *Registry {
	return &Registry{
		parsers: make(map[string]Parser),
	}
}

// Register registers a parser
func (r *Registry) Register(p Parser) {
	r.parsers[p.Protocol()] = p
}

// Get returns a parser by protocol name
func (r *Registry) Get(protocol string) (Parser, error) {
	p, ok := r.parsers[protocol]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProto, protocol)
	}
	return p, nil
}

// ParseAny tries to parse with all registered parsers
func (r *Registry) ParseAny(input string) (any, error) {
	for _, p := range r.parsers {
		if p.CanParse(input) {
			return p.Parse(input)
		}
	}
	return nil, ErrUnsupportedProto
}

// GlobalRegistry is the default parser registry
var GlobalRegistry = NewRegistry()
