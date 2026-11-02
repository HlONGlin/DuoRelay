package generator

import (
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// RestrictChainUDP rejects UDP only where routing would select the proxy
// chain. Existing DIRECT/REJECT routes (including the SOCKS-only guard) retain
// their order and behavior. Internal bridge routes are added afterwards.
func RestrictChainUDP(root map[string]any, kernel ir.KernelType) {
	if kernel == ir.KernelSingBox {
		route := root["route"].(map[string]any)
		var rules []map[string]any
		appendRule := func(rule map[string]any) {
			outbound, _ := rule["outbound"].(string)
			if outbound != "" && outbound != "DIRECT" && outbound != "REJECT" {
				match := map[string]any{}
				for k, v := range rule {
					if k != "outbound" && k != "action" {
						match[k] = v
					}
				}
				reject := map[string]any{"type": "logical", "mode": "and", "rules": []map[string]any{{"network": "udp"}, match}, "action": "reject"}
				rules = append(rules, reject)
			}
			rules = append(rules, rule)
		}
		switch original := route["rules"].(type) {
		case []map[string]any:
			for _, r := range original {
				appendRule(r)
			}
		case []any:
			for _, r := range original {
				appendRule(r.(map[string]any))
			}
		}
		if final, _ := route["final"].(string); final != "" && final != "DIRECT" && final != "REJECT" {
			rules = append(rules, map[string]any{"network": "udp", "action": "reject"})
		}
		route["rules"] = rules
	} else {
		var rules []string
		appendRule := func(rule string) {
			idx := strings.LastIndex(rule, ",")
			if idx < 0 {
				rules = append(rules, rule)
				return
			}
			match, outbound := rule[:idx], rule[idx+1:]
			if outbound != "DIRECT" && outbound != "REJECT" {
				if match == "MATCH" {
					rules = append(rules, "NETWORK,udp,REJECT")
				} else {
					rules = append(rules, "AND,((NETWORK,udp),("+match+")),REJECT")
				}
			}
			rules = append(rules, rule)
		}
		switch original := root["rules"].(type) {
		case []string:
			for _, r := range original {
				appendRule(r)
			}
		case []any:
			for _, r := range original {
				appendRule(r.(string))
			}
		}
		root["rules"] = rules
	}
}
