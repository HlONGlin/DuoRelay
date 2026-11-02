package zerotier

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/internal/overlayutil"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolZeroTier }

func (Parser) CanParse(input string) bool {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return false
	}
	value, present, err := overlayutil.String(object, "type")
	return err == nil && present && strings.EqualFold(value, ir.ProtocolZeroTier)
}

func (Parser) Parse(input string) (any, error) {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return nil, err
	}
	node, err := overlayutil.NewNode(object, ir.ProtocolZeroTier)
	if err != nil {
		return nil, err
	}
	network, err := overlayutil.RequiredString(object, "network")
	if err != nil {
		return nil, err
	}
	node.Proto["network"] = network
	for _, key := range []string{"state_dir", "identity_secret", "planet", "tcp_fallback_mode", "tcp_fallback_relay", "remote_trace_target", "ip_stack"} {
		if err := overlayutil.OptionalString(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"udp", "remote_dns_resolve", "low_bandwidth", "encrypted_hello"} {
		if err := overlayutil.OptionalBool(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"mtu", "physical_mtu", "primary_port", "secondary_port", "remote_trace_level"} {
		if err := overlayutil.OptionalInt(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"dns"} {
		if err := overlayutil.OptionalStrings(object, key); err != nil {
			return nil, err
		}
	}
	if value, present := object["orbit"]; present {
		items, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("%w: orbit", parser.ErrInvalidField)
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%w: orbit", parser.ErrInvalidField)
			}
			if _, err := overlayutil.RequiredString(entry, "world"); err != nil {
				return nil, err
			}
			if _, err := overlayutil.RequiredString(entry, "seed"); err != nil {
				return nil, err
			}
		}
	}
	return node, nil
}
