package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// cleanupMihomoTUNNetwork removes the policy-routing objects created by
// mihomo's Linux auto-redirect mode only when table 2022 contains exclusively
// routes through its Meta interface. It leaves nftables and link cleanup to
// mihomo, since their global names cannot distinguish another instance.
func cleanupMihomoTUNNetwork() error {
	routesOutput, err := exec.Command("ip", "-j", "route", "show", "table", "2022").Output()
	if err != nil {
		if isNetworkObjectMissing([]byte(err.Error())) {
			return nil
		}
		return fmt.Errorf("list table 2022 routes: %w", err)
	}
	owned, err := exclusivelyUsesMeta(routesOutput)
	if err != nil || !owned {
		return err
	}
	rulesOutput, err := exec.Command("ip", "-j", "rule", "list").Output()
	if err != nil {
		return fmt.Errorf("list ip rules: %w", err)
	}
	rules, err := metaOwnedRuleDeleteArgs(rulesOutput)
	if err != nil {
		return err
	}
	var errs []error
	for _, args := range rules {
		if output, err := exec.Command("ip", args...).CombinedOutput(); err != nil && !isNetworkObjectMissing(output) {
			errs = append(errs, fmt.Errorf("ip %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output))))
		}
	}
	if output, err := exec.Command("ip", "route", "flush", "table", "2022", "dev", "Meta").CombinedOutput(); err != nil && !isNetworkObjectMissing(output) {
		errs = append(errs, fmt.Errorf("ip route flush table 2022: %w: %s", err, strings.TrimSpace(string(output))))
	}
	return errors.Join(errs...)
}

type linuxRoute struct {
	Device string `json:"dev"`
}

type linuxRule struct {
	Priority int             `json:"priority"`
	Table    json.RawMessage `json:"table"`
	From     string          `json:"src"`
	To       string          `json:"dst"`
	TOS      string          `json:"tos"`
	Mark     string          `json:"fwmark"`
	IIF      string          `json:"iif"`
	OIF      string          `json:"oif"`
	UIDRange string          `json:"uidrange"`
	IPProto  string          `json:"ipproto"`
	Sport    string          `json:"sport"`
	Dport    string          `json:"dport"`
}

func exclusivelyUsesMeta(data []byte) (bool, error) {
	var routes []linuxRoute
	if err := json.Unmarshal(data, &routes); err != nil {
		return false, fmt.Errorf("decode table 2022 routes: %w", err)
	}
	if len(routes) == 0 {
		return false, nil
	}
	for _, route := range routes {
		if route.Device != "Meta" {
			return false, nil
		}
	}
	return true, nil
}

func metaOwnedRuleDeleteArgs(data []byte) ([][]string, error) {
	var rules []linuxRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("decode ip rules: %w", err)
	}
	var result [][]string
	for _, rule := range rules {
		if !isMihomoRulePriority(rule.Priority) || tableNumber(rule.Table) != "2022" {
			continue
		}
		args := []string{"rule", "del", "priority", strconv.Itoa(rule.Priority)}
		for _, selector := range []struct{ key, value string }{
			{"from", rule.From}, {"to", rule.To}, {"tos", rule.TOS},
			{"fwmark", rule.Mark}, {"iif", rule.IIF},
			{"oif", rule.OIF}, {"uidrange", rule.UIDRange}, {"ipproto", rule.IPProto},
			{"sport", rule.Sport}, {"dport", rule.Dport},
		} {
			if selector.value != "" && selector.value != "all" {
				args = append(args, selector.key, selector.value)
			}
		}
		args = append(args, "table", "2022")
		result = append(result, args)
	}
	return result, nil
}

func isMihomoRulePriority(priority int) bool {
	switch priority {
	case 9000, 9001, 9002, 9010:
		return true
	default:
		return false
	}
}

func tableNumber(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var name string
	if json.Unmarshal(raw, &name) == nil {
		return name
	}
	return strings.TrimSpace(string(raw))
}

func isNetworkObjectMissing(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "no such process") ||
		strings.Contains(message, "no such file") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "not found")
}
