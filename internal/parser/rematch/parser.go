package rematch

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/internal/overlayutil"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolRematch }

func (Parser) CanParse(input string) bool {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return false
	}
	value, present, err := overlayutil.String(object, "type")
	return err == nil && present && strings.EqualFold(value, ir.ProtocolRematch)
}

func (Parser) Parse(input string) (any, error) {
	object, err := overlayutil.DecodeObject(input)
	if err != nil {
		return nil, err
	}
	node, err := overlayutil.NewNode(object, ir.ProtocolRematch)
	if err != nil {
		return nil, err
	}
	targetName, namePresent, err := overlayutil.String(object, "target_rematch_name", "target-rematch-name")
	if err != nil {
		return nil, fmt.Errorf("%w: target-rematch-name", parser.ErrInvalidField)
	}
	targetRule, rulePresent, err := overlayutil.String(object, "target_sub_rule", "target-sub-rule")
	if err != nil {
		return nil, fmt.Errorf("%w: target-sub-rule", parser.ErrInvalidField)
	}
	if (!namePresent || strings.TrimSpace(targetName) == "") && (!rulePresent || strings.TrimSpace(targetRule) == "") {
		return nil, fmt.Errorf("%w: target-rematch-name or target-sub-rule", parser.ErrMissingField)
	}
	return node, nil
}
