package overlayutil

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

// NewNode creates an IR node from a mihomo/sing-box style JSON object.
// Hyphenated option names are normalized to snake_case for generators.
func NewNode(object map[string]any, protocol string) (*ir.Node, error) {
	typ, present, err := String(object, "type")
	if err != nil {
		return nil, fmt.Errorf("%w: type", parser.ErrInvalidField)
	}
	if !present || !strings.EqualFold(typ, protocol) {
		return nil, fmt.Errorf("%w: expected %s config", parser.ErrInvalidFormat, protocol)
	}

	name := protocol
	for _, key := range []string{"name", "tag"} {
		value, present, err := String(object, key)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", parser.ErrInvalidField, key)
		}
		if present {
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("%w: %s", parser.ErrMissingField, key)
			}
			name = value
			break
		}
	}

	node := &ir.Node{
		Name:     name,
		Protocol: protocol,
		Owner:    ir.GetProtocolOwner(protocol),
		Security: ir.Security{Type: "none"},
		Proto:    map[string]any{},
		Extra:    map[string]any{"config": object},
		Source:   ir.Source{Type: "import"},
	}
	for key, value := range object {
		canonical := CanonicalKey(key)
		switch canonical {
		case "type", "name", "tag":
			continue
		}
		node.Proto[canonical] = value
	}
	return node, nil
}

func CanonicalKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	key = strings.ReplaceAll(key, "-", "_")
	return strings.ReplaceAll(key, " ", "_")
}

func RequiredString(object map[string]any, keys ...string) (string, error) {
	value, present, err := String(object, optionKeys(keys)...)
	if err != nil {
		return "", fmt.Errorf("%w: %s", parser.ErrInvalidField, keys[0])
	}
	if !present || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: %s", parser.ErrMissingField, keys[0])
	}
	return value, nil
}

func OptionalString(object map[string]any, keys ...string) error {
	if _, present, err := String(object, optionKeys(keys)...); err != nil {
		return fmt.Errorf("%w: %s", parser.ErrInvalidField, keys[0])
	} else if present {
		return nil
	}
	return nil
}

func OptionalBool(object map[string]any, keys ...string) error {
	if _, present, err := Bool(object, optionKeys(keys)...); err != nil {
		return fmt.Errorf("%w: %s", parser.ErrInvalidField, keys[0])
	} else if present {
		return nil
	}
	return nil
}

func OptionalInt(object map[string]any, keys ...string) error {
	if _, present, err := Int(object, optionKeys(keys)...); err != nil {
		return fmt.Errorf("%w: %s", parser.ErrInvalidField, keys[0])
	} else if present {
		return nil
	}
	return nil
}

func OptionalStrings(object map[string]any, keys ...string) error {
	if _, present, err := Strings(object, optionKeys(keys)...); err != nil {
		return fmt.Errorf("%w: %s", parser.ErrInvalidField, keys[0])
	} else if present {
		return nil
	}
	return nil
}

func optionKeys(keys []string) []string {
	result := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		result = append(result, key)
		if strings.Contains(key, "_") {
			result = append(result, strings.ReplaceAll(key, "_", "-"))
		}
	}
	return result
}
