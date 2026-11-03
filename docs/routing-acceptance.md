# 路由验收记录

Date: 2026-11-03

路由规则保留既定顺序：内部路由优先，绕过规则其次，用户规则最后。独立 SOCKS5 端口默认仍为 `10808`，通过现有入口选择单节点、负载均衡组或代理链。`/api/config/apply` 仍拒绝原始内核 JSON/YAML。

The list editor stores source, matcher, values, outbound, enabled state, and order. The text format is one tab-separated rule per line:

```text
enabled<TAB>source<TAB>matcher<TAB>value1,value2<TAB>outbound
```

The parser reports every malformed line with its line number. Port values are checked as 1..65535 before save and before generation; sing-box receives numeric port fields, while mihomo receives `DST-PORT` rules. Invalid imports are parse-only and do not replace stored rules.

## 容器验收

| 检查项 | 结果 | 证据 |
|---|---|---|
| Domain split | PASS | `example.com` request was observed by the temporary HTTP proxy and returned the marker. |
| IP-CIDR split | PASS | `127.0.0.1/32 -> DIRECT` returned the marker without a temporary proxy CONNECT entry. |
| Port split | PASS | `port.test:18081` matched the numeric port rule and was observed by the temporary HTTP proxy. |
| Direct | PASS | The direct local HTTP server returned the marker. |
| Reject | PASS | `reject.example` returned 502 and sing-box logged the `REJECT` block outbound. |
| Export -> clear -> import | PASS | Exported text was saved, settings were cleared, rules were imported, and the second export matched the first. |
| List -> text -> list | PASS | API validation/import/export preserved rule fields and order; Vue tests and production build passed. |
| Invalid line isolation | PASS | Invalid port and malformed field count produced separate line errors; no invalid line was returned as a rule. |
| Kernel check rollback | PASS | A temporary unsupported SSH node failed generation/check and did not stop or replace the healthy existing sing-box instance. |
| Legacy rule compatibility | PASS in automated migration tests | Existing default/base rules migrated with source and stable IDs; a copied legacy database run is still required for the real upgrade matrix. |

以上容器结果与真实目标系统结果分开统计。Windows 系统代理和 macOS TUN 不属于容器验收范围。
