// Package uriopts implements the common, strict URI envelope used by the P2
// parsers. Protocol fields remain in their own packages.
package uriopts

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

type Options struct {
	Node *ir.Node
	URL  *url.URL
	q    url.Values
	used map[string]bool
	err  error
}

func CanParse(input, scheme string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), scheme+"://")
}

// Open rejects malformed queries instead of url.URL.Query's silent partial parse.
// A zero defaultPort means the import must contain an explicit port.
func Open(input, scheme, protocol string, defaultPort int) (*Options, error) {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil || !strings.EqualFold(u.Scheme, scheme) || u.Opaque != "" || u.Hostname() == "" {
		return nil, fmt.Errorf("%w: expected %s endpoint URI", parser.ErrInvalidFormat, scheme)
	}
	host := u.Hostname()
	if strings.ContainsAny(host, "\\/@?#") || strings.ContainsFunc(host, unicode.IsSpace) || strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return nil, fmt.Errorf("%w: server host", parser.ErrInvalidField)
	}
	if strings.HasPrefix(u.Host, "[") && net.ParseIP(host) == nil || strings.Contains(host, ":") && !strings.HasPrefix(u.Host, "[") {
		return nil, fmt.Errorf("%w: IPv6 address", parser.ErrInvalidField)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("%w: endpoint path (use a query option)", parser.ErrInvalidField)
	}
	if strings.HasSuffix(u.Host, ":") {
		return nil, fmt.Errorf("%w: empty port", parser.ErrInvalidField)
	}
	if u.Port() == "" {
		if defaultPort == 0 {
			return nil, fmt.Errorf("%w: port", parser.ErrMissingField)
		}
		u.Host = net.JoinHostPort(host, strconv.Itoa(defaultPort))
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: port", parser.ErrInvalidField)
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("%w: query encoding", parser.ErrInvalidFormat)
	}
	for key, values := range q {
		if key == "" || len(values) != 1 {
			return nil, fmt.Errorf("%w: duplicate or empty query key", parser.ErrInvalidField)
		}
	}
	n := parser.NewNode(protocol, protocol, u)
	// URL.Fragment has already been decoded. QueryUnescape would corrupt '+'
	// and double-decode percent escapes in the display name.
	if u.Fragment != "" {
		n.Name = u.Fragment
	}
	return &Options{Node: n, URL: u, q: q, used: map[string]bool{}}, nil
}

// Value accepts snake_case and kebab-case plus explicitly listed URI aliases.
// Multiple aliases must agree; ambiguous input is never silently selected.
func (o *Options) Value(key string, aliases ...string) string {
	keys := append([]string{key, strings.ReplaceAll(key, "_", "-")}, aliases...)
	var value string
	found := false
	for _, k := range keys {
		if values, ok := o.q[k]; ok {
			o.used[k] = true
			if found && value != values[0] {
				o.Invalid(key)
			}
			value, found = values[0], true
		}
	}
	return value
}

func (o *Options) Invalid(field string) {
	if o.err == nil {
		o.err = fmt.Errorf("%w: %s", parser.ErrInvalidField, field)
	}
}

func (o *Options) Required(value, field string) {
	if strings.TrimSpace(value) == "" && o.err == nil {
		o.err = fmt.Errorf("%w: %s", parser.ErrMissingField, field)
	}
}

func (o *Options) String(key string, aliases ...string) string {
	v := o.Value(key, aliases...)
	if v != "" {
		o.Node.Proto[key] = v
	}
	return v
}

func (o *Options) BoolValue(key string, aliases ...string) (bool, bool) {
	v := o.Value(key, aliases...)
	if v == "" {
		if o.has(key, aliases...) {
			o.Invalid(key)
			return false, true
		}
		return false, false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		o.Invalid(key)
	}
	return b, true
}

func (o *Options) Bool(key string, aliases ...string) bool {
	v, present := o.BoolValue(key, aliases...)
	if present {
		o.Node.Proto[key] = v
	}
	return v
}

func (o *Options) Int(key string, min, max int, aliases ...string) (int, bool) {
	v := o.Value(key, aliases...)
	if v == "" {
		if o.has(key, aliases...) {
			o.Invalid(key)
		}
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min || n > max {
		o.Invalid(key)
	}
	o.Node.Proto[key] = n
	return n, true
}

func (o *Options) has(key string, aliases ...string) bool {
	for _, k := range append([]string{key, strings.ReplaceAll(key, "_", "-")}, aliases...) {
		if _, ok := o.q[k]; ok {
			return true
		}
	}
	return false
}

func (o *Options) Enum(key, fallback string, allowed ...string) string {
	v := strings.ToLower(o.Value(key))
	if v == "" {
		v = fallback
	}
	if v != "" {
		if !contains(allowed, v) {
			o.Invalid(key)
		}
		o.Node.Proto[key] = v
	}
	return v
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func (o *Options) List(key string, aliases ...string) []string {
	v := o.Value(key, aliases...)
	if v == "" {
		return nil
	}
	list := strings.Split(v, ",")
	for i := range list {
		list[i] = strings.TrimSpace(list[i])
		if list[i] == "" {
			o.Invalid(key)
		}
	}
	o.Node.Proto[key] = list
	return list
}

// Credentials reads user:password authentication. Optional credentials must be
// either entirely absent or complete, to catch truncated authenticated links.
func (o *Options) Credentials(required bool) {
	if o.URL.User == nil {
		if required {
			o.Required("", "username/password")
		}
		return
	}
	user := o.URL.User.Username()
	pass, _ := o.URL.User.Password()
	o.Required(user, "username")
	o.Required(pass, "password")
	o.Node.Proto["username"], o.Node.Proto["password"] = user, pass
}

// Password reads password-only userinfo, used by AnyTLS and Sudoku's key.
func (o *Options) Password(field string) {
	value := o.Value(field)
	if o.URL.User != nil {
		if _, hasPassword := o.URL.User.Password(); hasPassword {
			o.Invalid("password-only userinfo")
		}
		if value != "" && value != o.URL.User.Username() {
			o.Invalid(field)
		}
		value = o.URL.User.Username()
	}
	o.Required(value, field)
	o.Node.Proto[field] = value
}

// TLS enforces TLS for protocols where cleartext is not an alternative.
func (o *Options) TLS(defaultALPN ...string) {
	if v := o.Value("security"); v != "" && v != "tls" {
		o.Invalid("security")
	}
	if enabled, present := o.BoolValue("tls"); present && !enabled {
		o.Invalid("tls")
	}
	n := o.Node
	n.Security.Type = "tls"
	n.Security.SNI = o.Value("sni", "servername", "peer")
	if n.Security.SNI == "" {
		n.Security.SNI = n.Server.Host
	}
	n.Security.ALPN = o.List("alpn")
	delete(n.Proto, "alpn")
	if n.Security.ALPN == nil {
		n.Security.ALPN = defaultALPN
	}
	n.Security.Fingerprint = o.Value("client_fingerprint", "fp", "clientFingerprint")
	if insecure, present := o.BoolValue("skip_cert_verify", "insecure", "allowInsecure"); present {
		n.Security.Opts = map[string]any{"insecure": insecure}
	}
}

func (o *Options) Finish() (any, error) {
	if o.err != nil {
		return nil, o.err
	}
	// Keep unrecognised extensions separate from validated protocol options.
	extra := map[string]string{}
	for k, values := range o.q {
		if !o.used[k] {
			extra[k] = values[0]
		}
	}
	if len(extra) != 0 {
		o.Node.Extra["uri_query"] = extra
	}
	return o.Node, nil
}
