# Proxy Chain Compatibility Acceptance

[简体中文](chain-compatibility-report.md) | English

Date: 2026-11-01

## Implemented

- Removed the frontend protocol allowlist. The backend now plans the chain from node parameters, kernel support and per-hop TCP/UDP capabilities, for 2–8 distinct nodes.
- Prefers one kernel and, when needed, bridges sing-box and mihomo through a local SOCKS5 listener with random authentication. Multiple kernel alternations are supported.
- The dialog shows each hop's kernel and transport, whole-chain UDP capability and incompatibility reason. Starting is disabled during preview and whenever the chain cannot run.
- Generates and validates all configuration before switching the running instance. Startup failure restores the previous complete configuration; stopping removes primary and auxiliary configurations.
- Public APIs hide controller credentials, bridge authentication and internal paths. Cross-kernel status is reported using the primary kernel.
- Retains China bypass, default blacklist-direct behavior and the rule that only the dedicated SOCKS5 entry follows proxy routing, preventing user rules from bypassing an intermediate hop.
- Corrected MASQUE kernel ownership, ShadowQUIC password authentication, VMess defaults, Hysteria v1 authentication fields and storage validation for overlay nodes.
- Added a pinned Cronet DLL installation script for Windows Naive support, with version lock and SHA-256 verification.

## Verification

- `go test ./...` and `go vet ./...` passed. After final changes, API, configuration and blacklist-default-outbound tests passed.
- A configuration matrix using pinned real kernels verified usable entry/exit positions for 27 network/transport protocols.
- Real local TCP traffic passed for same-kernel chains, both cross-kernel directions, eight alternating hops and SOCKS5 entry isolation. The bridge rejects anonymous authentication.
- Two local encrypted Shadowsocks servers and UDP echo tests passed for all four kernel combinations of real UDP chains.
- 42 frontend tests, type checking and production build passed.
- Browser interaction verified a Naive → Mieru cross-kernel preview can start. HTTP → Hysteria2 showed that hop 2 lacks UDP and disabled start.
- API tests verified preview has no runtime side effects, responses expose no credentials, and stopping the primary kernel does not report the auxiliary kernel as the active exit.
- The updated backend ran with the normal development server. The page on 5173, API on 8080 and API forwarded through 5173 returned HTTP 200; the new preview endpoint was reachable. Temporary test services were stopped.

## Practical Limits

Rematch is a routing adapter and cannot be an independent proxy hop. ShadowTLS must be paired with a following Shadowsocks hop. If a later hop requires UDP, earlier hops must carry UDP. VPN/overlay exit permissions depend on remote configuration. Local tests do not prove all remote nodes work and did not capture the user's system traffic through TUN.

See [multi-node proxy documentation](multi-node-proxy.en.md) for usage and reproduction commands.
