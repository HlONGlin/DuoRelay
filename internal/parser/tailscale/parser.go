package tailscale

import (
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/internal/overlayutil"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolTailscale }

func (Parser) CanParse(input string) bool {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return false
	}
	value, present, err := overlayutil.String(object, "type")
	return err == nil && present && strings.EqualFold(value, ir.ProtocolTailscale)
}

func (Parser) Parse(input string) (any, error) {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return nil, err
	}
	node, err := overlayutil.NewNode(object, ir.ProtocolTailscale)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{
		"hostname", "auth_key", "control_url", "state_directory", "exit_node",
		"system_interface_name", "taildrop_directory",
	} {
		if err := overlayutil.OptionalString(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{
		"ephemeral", "udp", "accept_routes", "exit_node_allow_lan_access",
		"advertise_exit_node", "system_interface",
	} {
		if err := overlayutil.OptionalBool(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"advertise_routes", "advertise_tags", "relay_server_static_endpoints"} {
		if err := overlayutil.OptionalStrings(object, key); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"listen_port", "relay_server_port", "system_interface_mtu"} {
		if err := overlayutil.OptionalInt(object, key); err != nil {
			return nil, err
		}
	}
	return node, nil
}
