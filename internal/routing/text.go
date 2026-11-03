package routing

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/generator"
)

type LineError struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

type ParseResult struct {
	Rules  []generator.RoutingRule `json:"rules"`
	Errors []LineError             `json:"errors,omitempty"`
}

// Text uses one tab-separated rule per line:
// enabled<TAB>source<TAB>matcher<TAB>value1,value2<TAB>outbound
func Format(rules []generator.RoutingRule) string {
	var lines []string
	for _, rule := range rules {
		enabled := "1"
		if !rule.Enabled {
			enabled = "0"
		}
		source := rule.Source
		if source == "" {
			source = "user"
		}
		lines = append(lines, strings.Join([]string{enabled, source, rule.Match, strings.Join(rule.Values, ","), rule.Outbound}, "\t"))
	}
	return strings.Join(lines, "\n") + func() string {
		if len(lines) > 0 {
			return "\n"
		}
		return ""
	}()
}

func Parse(input string) ParseResult {
	result := ParseResult{}
	for lineNo, raw := range strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 5 {
			result.Errors = append(result.Errors, LineError{lineNo + 1, "expected 5 tab-separated fields"})
			continue
		}
		enabled, err := strconv.ParseBool(parts[0])
		if err != nil {
			if parts[0] == "1" {
				enabled = true
			} else if parts[0] == "0" {
				enabled = false
			} else {
				result.Errors = append(result.Errors, LineError{lineNo + 1, "enabled must be 1, 0, true, or false"})
				continue
			}
		}
		rule := generator.RoutingRule{ID: fmt.Sprintf("import-%d", lineNo+1), Enabled: enabled, Source: strings.TrimSpace(parts[1]), Match: strings.TrimSpace(parts[2]), Values: splitValues(parts[3]), Outbound: strings.TrimSpace(parts[4])}
		if rule.Source == "" {
			rule.Source = "user"
		}
		if err := generator.ValidateRoutingRules([]generator.RoutingRule{rule}); err != nil {
			result.Errors = append(result.Errors, LineError{lineNo + 1, err.Error()})
			continue
		}
		result.Rules = append(result.Rules, rule)
	}
	if err := generator.ValidateRoutingRules(result.Rules); err != nil {
		result.Errors = append(result.Errors, LineError{0, err.Error()})
		result.Rules = nil
	}
	return result
}

func splitValues(value string) []string {
	values := strings.Split(value, ",")
	result := make([]string, 0, len(values))
	for _, item := range values {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
