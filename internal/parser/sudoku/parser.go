package sudoku

import (
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolSudoku }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "sudoku") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "sudoku", ir.ProtocolSudoku, 0)
	if err != nil {
		return nil, err
	}
	o.Password("key")
	o.Enum("aead_method", "chacha20-poly1305", "aes-128-gcm", "chacha20-poly1305", "none")
	o.Enum("table_type", "prefer_entropy", "prefer_ascii", "prefer_entropy", "up_ascii_down_entropy", "up_entropy_down_ascii")
	min, hasMin := o.Int("padding_min", 0, 100)
	max, hasMax := o.Int("padding_max", 0, 100)
	if hasMin && hasMax && min > max {
		o.Invalid("padding_min > padding_max")
	}
	o.Bool("enable_pure_downlink")
	o.Bool("http_mask")
	o.Enum("http_mask_mode", "", "legacy", "stream", "poll", "auto", "ws")
	o.Enum("multiplex", "", "off", "auto", "on")
	o.String("http_mask_host")
	if path := o.String("path_root"); path != "" {
		segment := strings.Trim(path, "/")
		if segment == "" || strings.ContainsAny(segment, "/?#") || strings.ContainsFunc(segment, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
		}) {
			o.Invalid("path_root")
		}
	}
	if o.Bool("http_mask_tls") {
		o.TLS()
	}
	o.Node.Proto["udp"] = true
	return o.Finish()
}
