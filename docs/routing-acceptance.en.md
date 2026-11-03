# Routing Acceptance Record

[简体中文](routing-acceptance.md) | English

Date: 2026-11-03

Routing rules retain their established order: internal routes first, bypass rules second, and user rules last. The dedicated SOCKS5 port still defaults to `10808` and uses the existing selection of a single node, load balancing group or proxy chain. `/api/config/apply` still rejects raw kernel JSON/YAML.

The list editor stores source, matcher, values, outbound, enabled state, and order. The text format is one tab-separated rule per line:

```text
enabled<TAB>source<TAB>matcher<TAB>value1,value2<TAB>outbound
```

The parser reports each malformed line with its line number. Port values are checked as 1..65535 before save and generation; sing-box receives numeric port fields, while mihomo receives `DST-PORT` rules. Invalid imports only parse and do not replace stored rules.

## Container Acceptance

| Check | Result | Evidence |
|---|---|---|
| Domain split | PASS | An `example.com` request was observed by the temporary HTTP proxy and returned the marker. |
| IP-CIDR split | PASS | `127.0.0.1/32 -> DIRECT` returned the marker without a temporary proxy CONNECT entry. |
| Port split | PASS | `port.test:18081` matched the numeric port rule and was observed by the temporary HTTP proxy. |
| Direct | PASS | The direct local HTTP server returned the marker. |
| Reject | PASS | `reject.example` returned 502 and sing-box logged the `REJECT` block outbound. |
| Export → clear → import | PASS | Exported text was saved, settings cleared, rules imported, and the second export matched the first. |
| List → text → list | PASS | API validation/import/export preserved fields and order; Vue tests and production build passed. |
| Invalid line isolation | PASS | Invalid port and malformed field count produced separate line errors; neither invalid line became a rule. |
| Kernel check rollback | PASS | A temporary unsupported SSH node failed generation/check without stopping or replacing the healthy sing-box instance. |
| Legacy rule compatibility | PASS in automated migration tests | Existing default/base rules migrated with source and stable IDs; a copied legacy database run remains necessary for the real upgrade matrix. |

These container results are recorded separately from real target-system results. Windows system proxy and macOS TUN are outside this container acceptance scope.
