package uriopts

import (
	"errors"
	"reflect"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

func TestEnvelope(t *testing.T) {
	for _, tc := range []struct {
		input string
		port  int
		want  error
	}{
		{"", 443, parser.ErrInvalidFormat},
		{"other://host:443", 443, parser.ErrInvalidFormat},
		{"demo://:443", 443, parser.ErrInvalidFormat},
		{"demo:opaque", 443, parser.ErrInvalidFormat},
		{"demo://host", 0, parser.ErrMissingField},
		{"demo://host:", 443, parser.ErrInvalidField},
		{"demo://host:0", 443, parser.ErrInvalidField},
		{"demo://host:65536", 443, parser.ErrInvalidField},
		{"demo://host", 70000, parser.ErrInvalidField},
		{"demo://[bad-ip]:443", 443, parser.ErrInvalidFormat},
		{"demo://2001:db8::1:443", 443, parser.ErrInvalidField},
		{"demo://a:b:c:443", 443, parser.ErrInvalidField},
		{"demo://host:443/path", 443, parser.ErrInvalidField},
		{"demo://host:443?x=%GG", 443, parser.ErrInvalidFormat},
		{"demo://host:443?x=a;b", 443, parser.ErrInvalidFormat},
		{"demo://host:443?x=1&x=1", 443, parser.ErrInvalidField},
		{"demo://host:443?=empty", 443, parser.ErrInvalidField},
	} {
		o, err := Open(tc.input, "demo", "demo", tc.port)
		if o != nil || !errors.Is(err, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.input, err, tc.want)
		}
	}
	o, err := Open("  DEMO://u:p@[2001:db8::1]/#A+B%2520  ", "demo", "demo", 443)
	if err != nil {
		t.Fatal(err)
	}
	if o.Node.Name != "A+B%20" || o.Node.Server.Host != "2001:db8::1" || o.Node.Server.Port != 443 {
		t.Fatal("name/IPv6/default port was corrupted", o.Node)
	}
	if !CanParse("  DEMO://x ", "demo") || CanParse("demo+tls://x", "demo") {
		t.Fatal("scheme detection")
	}
}

func open(t *testing.T, query string) *Options {
	t.Helper()
	o, err := Open("demo://example.com:443?"+query, "demo", "demo", 0)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestTypedOptions(t *testing.T) {
	o := open(t, "value=a%2Bb&flag=0&count=2&mode=B&list=one,%20two&vendor=keep&same_name=x&same-name=x")
	if o.String("value") != "a+b" || o.String("absent") != "" {
		t.Fatal("string")
	}
	if o.Bool("flag") || o.Bool("missing") {
		t.Fatal("bool")
	}
	if n, ok := o.Int("count", 1, 5); n != 2 || !ok {
		t.Fatal("int")
	}
	if _, ok := o.Int("missing", 0, 1); ok {
		t.Fatal("missing int")
	}
	if o.Enum("mode", "a", "a", "b") != "b" || o.Enum("default", "a", "a", "b") != "a" || o.Enum("absent", "") != "" {
		t.Fatal("enum")
	}
	if !reflect.DeepEqual(o.List("list"), []string{"one", "two"}) || o.List("missing") != nil {
		t.Fatal("list")
	}
	if o.Value("same_name") != "x" {
		t.Fatal("equivalent aliases")
	}
	result, err := o.Finish()
	if err != nil {
		t.Fatal(err)
	}
	n := result.(*ir.Node)
	if !reflect.DeepEqual(n.Extra["uri_query"], map[string]string{"vendor": "keep"}) {
		t.Fatal("extension lost", n.Extra)
	}
	if n.Proto["flag"] != false || n.Proto["count"] != 2 {
		t.Fatal("types", n.Proto)
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, query := range []string{"flag=maybe", "flag=", "count=", "count=-1", "count=99999999999999999999999", "mode=z", "list=a,,b", "same_name=a&same-name=b"} {
		o := open(t, query)
		o.Bool("flag")
		o.Int("count", 0, 10)
		o.Enum("mode", "", "a")
		o.List("list")
		o.Value("same_name")
		result, err := o.Finish()
		if result != nil || !errors.Is(err, parser.ErrInvalidField) {
			t.Errorf("%q: %v", query, err)
		}
	}
	o := open(t, "")
	o.Required("", "first")
	o.Invalid("second")
	o.Required("", "third")
	if _, err := o.Finish(); !errors.Is(err, parser.ErrMissingField) {
		t.Fatal("first error lost", err)
	}
}

func TestTLS(t *testing.T) {
	o := open(t, "security=tls&tls=1&sni=tls.example.com&alpn=h2,http%2F1.1&client-fingerprint=chrome&allowInsecure=0")
	o.TLS()
	result, err := o.Finish()
	if err != nil {
		t.Fatal(err)
	}
	n := result.(*ir.Node)
	if n.Security.Type != "tls" || n.Security.SNI != "tls.example.com" || n.Security.Fingerprint != "chrome" || n.Security.Opts["insecure"] != false || len(n.Security.ALPN) != 2 {
		t.Fatal(n.Security)
	}
	if _, exists := n.Proto["alpn"]; exists {
		t.Fatal("ALPN should only be stored in Security")
	}
	o = open(t, "")
	o.TLS("h3")
	if o.Node.Security.SNI != "example.com" || !reflect.DeepEqual(o.Node.Security.ALPN, []string{"h3"}) {
		t.Fatal(o.Node.Security)
	}
	if _, err := o.Finish(); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"security=none", "tls=false", "tls=invalid", "insecure=maybe", "sni=a&peer=b", "alpn=h2,"} {
		o := open(t, query)
		o.TLS()
		if _, err := o.Finish(); !errors.Is(err, parser.ErrInvalidField) {
			t.Errorf("%q: %v", query, err)
		}
	}
}

func TestAuthentication(t *testing.T) {
	for _, tc := range []struct {
		input                  string
		required, passwordOnly bool
		want                   error
	}{
		{"demo://host:443", false, false, nil},
		{"demo://host:443", true, false, parser.ErrMissingField},
		{"demo://u:p@host:443", true, false, nil},
		{"demo://:p@host:443", true, false, parser.ErrMissingField},
		{"demo://u@host:443", false, false, parser.ErrMissingField},
		{"demo://p%40ss@host:443", false, true, nil},
		{"demo://host:443?password=demo", false, true, nil},
		{"demo://demo@host:443?password=demo", false, true, nil},
		{"demo://demo@host:443?password=other", false, true, parser.ErrInvalidField},
		{"demo://u:p@host:443", false, true, parser.ErrInvalidField},
		{"demo://host:443", false, true, parser.ErrMissingField},
	} {
		o, err := Open(tc.input, "demo", "demo", 0)
		if err != nil {
			t.Fatal(err)
		}
		if tc.passwordOnly {
			o.Password("password")
		} else {
			o.Credentials(tc.required)
		}
		_, err = o.Finish()
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", tc.input, err, tc.want)
		}
	}
}
