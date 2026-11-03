# DuoRelay Current Status

[简体中文](STATUS.md) | English

Updated: 2026-11-03. Phase: P3 runtime hardening. Development is ongoing.

## Current Baseline

- The Go backend, Vue 3/TypeScript frontend and dual-kernel process flow build and test.
- Nodes, subscriptions, latency tests, load balancing, proxy chains, basic routing, logs and traffic sampling are integrated.
- Config is checked by the actual kernel before application (`sing-box check`, `mihomo -t`). A failure does not replace the running instance.
- Routing preserves internal-route, bypass and user-rule precedence. The dedicated SOCKS5 port defaults to `10808`.
- Linux transparent proxy provides redirect, TPROXY and TUN modes. See [transparent proxy acceptance](docs/transparent-proxy-acceptance.en.md) for prerequisites, DNS behavior and evidence.
- Linux-container builds, panel/kernel startup, routing edits and some lifecycle cleanup are recorded. Container results do not prove real transparent traffic or DNS leak protection.

## Verification Status

| Area | Status | Record |
|---|---|---|
| Go tests and vet | Verified | [Development guide](docs/DEVELOPMENT.en.md) |
| Frontend type check, tests and build | Verified | [Container acceptance](docs/container-acceptance.en.md) |
| Routing list/text sync, import/export and rollback | Container-verified | [Routing acceptance](docs/routing-acceptance.en.md) |
| Linux TUN startup/stop and cleanup | Container-verified | [Transparent proxy acceptance](docs/transparent-proxy-acceptance.en.md) |
| Real Linux redirect/TPROXY requests | Unverified; isolated fixture cannot provide trusted end-to-end observation |
| Linux DNS capture/leak testing | Unverified; no independent DNS observer result |
| Windows system proxy and TUN | Real-system verification pending; requires target machine, driver, admin rights and firewall review |
| macOS TUN | Real-system verification pending |

## Boundaries

Parser or generator success does not mean every public node connects; capabilities depend on kernel version, remote service and configuration. Transparent-proxy support, permissions, DNS and cleanup must follow target-system acceptance. DuoRelay's license is undecided; sing-box and mihomo are used under their respective GPLv3 terms.

```bash
go test ./...
go vet ./...
cd web && npm run type-check
cd web && npm test
cd web && npm run build
```

See the [documentation index](docs/README.en.md) for more.
