# Documentation Index

[简体中文](README.md) | English

- [DuoRelay showcase, screenshots and package measurements](showcase.en.md)
- [Full / lite builds, protocol comparison and data compatibility](lite.en.md)
- [Lite build, size and isolated container acceptance](lite-acceptance.en.md)
- [Full / lite Docker and Linux installation entry point verification](deployment-variants-acceptance.en.md)
- [Linux installation compatibility acceptance](linux-bootstrap-acceptance.en.md)
- [Showcase, screenshots and package measurements](showcase.en.md)
- [Protocol matrix verification report](protocol-verification-report.en.md)

This index collects commonly used documentation. Historical plans, progress reports and designs describe their respective dates; consult the code and acceptance records for current capabilities and verification limits.

## Usage and Deployment

| Document | Contents |
|---|---|
| [Project README](../README.en.md) | Features, quick start and capability boundaries |
| [Linux deployment](linux-deployment.en.md) | Packaging, systemd services, permissions and acceptance |
| [Docker deployment](docker-deployment.en.md) | Single-container installation, backups and updates |
| [Windows deployment](windows-deployment.en.md) | Packaging, startup, migration and system proxy |
| [Subscription formats](subscription-formats.en.md) | Supported input formats and limitations |
| [Node management and multi-node proxy](multi-node-proxy.en.md) | Batch deletion, load balancing, proxy chains and SOCKS5 port bindings |
| [Application updates](app-update.en.md) | About-page checks, one-click update flow, supported platforms and install permissions |

## Verification Records

| Document | Contents |
|---|---|
| [Transparent proxy acceptance](transparent-proxy-acceptance.en.md) | System modes, DNS behavior and verification matrix |
| [Routing acceptance](routing-acceptance.en.md) | Rule format, priority, import/export and results |
| [Container acceptance](container-acceptance.en.md) | Orca Linux container environment and results |
| [Protocol verification report](protocol-verification-report.en.md) | Protocol coverage and verification evidence |

Passing checks in a container does not replace acceptance on a real host or Windows/macOS. Outstanding real-traffic and DNS leak checks remain unverified.

## Development and Design

| Document | Contents |
|---|---|
| [Development guide](DEVELOPMENT.en.md) | Environment, build, tests and repository layout |
| [IR architecture decision](adr/ADR-001-ir-design.md) | Unified intermediate model |
| [Dual-kernel architecture decision](adr/ADR-002-dual-kernel-coexistence.en.md) | Dual-kernel coexistence design |
| [Clash API differences](clashapi-diff.md) | Kernel API differences |

Files named `P0-*`, `P1-*`, `P2-*`, `P3-*` and similar are historical plans or reports, not final acceptance conclusions for all current work.
