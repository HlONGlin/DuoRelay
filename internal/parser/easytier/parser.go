package easytier

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/internal/overlayutil"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolEasyTier }

func (Parser) CanParse(input string) bool {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return false
	}
	value, present, err := overlayutil.String(object, "type")
	return err == nil && present && strings.EqualFold(value, ir.ProtocolEasyTier)
}

func (Parser) Parse(input string) (any, error) {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return nil, err
	}
	node, err := overlayutil.NewNode(object, ir.ProtocolEasyTier)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{
		"network_name", "network_secret", "hostname", "ipv4", "instance_name",
		"state_dir", "encryption_algorithm", "tld_dns_zone",
		"local_private_key", "local_public_key",
	} {
		if err := overlayutil.OptionalString(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{
		"dhcp", "no_listener", "udp", "accept_dns", "enable_exit_node",
		"enable_encryption", "private_mode", "latency_first", "disable_p2p",
		"enable_kcp_proxy", "disable_kcp_input", "enable_quic_proxy",
		"disable_quic_input", "secure_mode",
	} {
		if err := overlayutil.OptionalBool(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{
		"peers", "listeners", "mapped_listeners", "exit_nodes", "proxy_networks",
	} {
		if err := overlayutil.OptionalStrings(object, key); err != nil {
			return nil, err
		}
	}
	peers, _, err := overlayutil.Strings(object, "peers")
	if err != nil {
		return nil, fmt.Errorf("%w: peers", parser.ErrInvalidField)
	}
	listeners, _, err := overlayutil.Strings(object, "listeners")
	if err != nil {
		return nil, fmt.Errorf("%w: listeners", parser.ErrInvalidField)
	}
	if len(peers) == 0 && len(listeners) == 0 {
		return nil, fmt.Errorf("%w: peers or listeners", parser.ErrMissingField)
	}
	if err := overlayutil.OptionalInt(object, "mtu"); err != nil {
		return nil, err
	}
	return node, nil
}
