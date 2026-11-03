# P0-03 Protocol Matrix Verification Report

[简体中文](protocol-verification-report.md) | English

## Conclusion

Verification used the project-pinned sing-box v1.14.4 and mihomo v1.19.33, local Windows amd64 binary version output, tagged source and configuration documentation.

The 28 entries in the proposal can remain DuoRelay product-coverage items, but they are not all native outbound protocols at the same layer. The verified capability groups are:

- 13 outbound types shared by both kernels: Shadowsocks, VMess, VLESS, Trojan, Hysteria, Hysteria2, TUIC, AnyTLS, Snell, SOCKS5, HTTP/HTTPS, SSH and WireGuard.
- 2 cross-layer capabilities: OpenVPN and Tailscale. sing-box provides endpoints; mihomo provides outbounds.
- 4 sing-box-only capabilities: ShadowTLS, Naive, Tor and OpenConnect. OpenConnect is an endpoint. ShadowTLS is exclusive only as a standalone outbound; mihomo still supports embedded ShadowTLS options in other outbounds.
- 9 mihomo-only capabilities: MASQUE, ShadowsocksR, Mieru, Sudoku, ShadowQUIC, TrustTunnel, Rematch, EasyTier and ZeroTier.

The total remains 28, with corrected implementation semantics. An outbound column means an independently configurable type; plugins or obfuscation options embedded in another protocol do not count as separate outbounds.

## Key Findings

### ShadowTLS

sing-box v1.14.4 provides a standalone `type: shadowtls` outbound. mihomo v1.19.33 supports ShadowTLS only as an option embedded in Shadowsocks and Snell outbounds; a standalone ShadowTLS node is not shared by both kernels. Evidence below is pinned to the versions in `kernels.lock`, rather than inferred from rolling documentation:

- sing-box's [outbound configuration](https://github.com/SagerNet/sing-box/blob/v1.14.4/docs/configuration/outbound/shadowtls.md#L1-L52) specifies `type: shadowtls`, server, port, version, password and TLS fields. `server`, `server_port` and `tls` are required; password is used for v2/v3 and the default version is v1. Its [registration and constructor](https://github.com/SagerNet/sing-box/blob/v1.14.4/protocol/shadowtls/outbound.go#L21-L35) register `option.ShadowTLSOutboundOptions` as `C.TypeShadowTLS`, construct an independent outbound and reject missing or disabled TLS.
- mihomo's [proxy-type dispatcher](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/parser.go#L29-L228) has no standalone `shadowtls` or `shadow-tls` branch; unknown types return `unsupport proxy type`. Having a `transport/shadowtls` implementation does not establish a standalone outbound type.
- mihomo's [Shadowsocks options](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/shadowsocks.go#L37-L53) include `plugin` and `plugin-opts`. The [plugin branch](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/shadowsocks.go#L396-L421) decodes `plugin: shadow-tls` into ShadowTLS options; the [connection path](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/shadowsocks.go#L163-L208) wraps the connection with ShadowTLS before creating the Shadowsocks cipher connection. It remains a Shadowsocks outbound.
- mihomo's [Snell connection path](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/snell.go#L29-L69) wraps the connection according to its obfuscation mode before calling `snell.StreamConn`; its [options branch](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/snell.go#L169-L207) decodes `mode: shadow-tls` from `obfs-opts`. It remains a Snell outbound.
- Both mihomo adapters use the same [mode constant](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/transport/shadowtls/client.go#L20), whose value is `shadow-tls`, not a new proxy type.

Therefore the ShadowTLS row in `protocol-matrix.csv` should read `sing_box_outbound=yes`, `mihomo_outbound=no`, and `classification=sing-box outbound only`. `internal/ir/protocol.go` already assigns `ProtocolShadowTLS: KernelSetSingBox`, so production code needs no change. Ownership regression tests should assert that sing-box can run the node and mihomo cannot. This verifies standalone configuration capability only, not connectivity to a real ShadowTLS server.

### Rematch

Rematch is not a standalone remote protocol. In mihomo v1.19.33, `adapter/outbound/rematch.go` requires `target-rematch-name` or `target-sub-rule`; its Dial/ListenPacket paths only write rematch information into connection metadata. DuoRelay should model it as a mihomo routing-control adapter, without remote-server fields, share links or standalone connectivity acceptance.

### Other Capabilities

- **AnyTLS:** Both kernels expose an AnyTLS outbound configuration. It requires at least server, port, password and TLS/dial fields. Configuration support alone does not prove a real node works; connectivity requires later lab or real-node testing.
- **OpenVPN:** sing-box v1.14.4 provides an OpenVPN client endpoint; mihomo v1.19.33 provides an outbound adapter. Their configuration models and lifecycles differ, so they cannot share the ordinary outbound generation path.
- **MASQUE:** mihomo v1.19.33 contains `adapter/outbound/masque.go`; MASQUE is absent from sing-box v1.14.4 outbound and endpoint listings.
- **Tailscale:** sing-box v1.14.4 provides a Tailscale endpoint and mihomo v1.19.33 a Tailscale outbound. Both depend on a state directory, control-plane registration and route/exit-node configuration, so they require external account or state.

## Implementation Impact

- Outbounds generate the corresponding kernel outbound/proxy configuration.
- Standalone ShadowTLS belongs only to sing-box. ShadowTLS options in mihomo `ss` plugins or Snell obfuscation remain part of their parent outbound and do not grant mihomo ownership to a standalone ShadowTLS node.
- Endpoints generate sing-box endpoint configuration; mihomo handles the corresponding capability as an outbound adapter.
- Rematch generates mihomo routing/proxy-control configuration, not remote server connection settings.
- EasyTier and ZeroTier are embedded networking capabilities and require state-directory, identity and network dependency checks.

## Evidence

- `kernels.lock`, `kernels/sing-box.exe version` and `kernels/mihomo.exe -v`
- sing-box v1.14.4 outbound index and OpenVPN, OpenConnect and Tailscale endpoint documentation
- mihomo v1.19.33 `constant/adapters.go` and `adapter/outbound/*.go`
- Official source tags SagerNet/sing-box v1.14.4 and MetaCubeX/mihomo v1.19.33

## ShadowTLS Regression Verification (2026-10-31)

- Local version output confirmed sing-box 1.14.4 (revision `c005f35300ce76427dc285481ec5695f5b179cd2`) and Mihomo Meta v1.19.33, matching `kernels.lock`.
- `go test ./internal/ir -count=1` passed. `TestShadowTLSStandaloneKernelEligibility` constructs a node through `GetProtocolOwner` and checks that sing-box can run it and mihomo cannot; the existing `TestProtocolOwnership/shadowtls` continues to verify exact ownership.
- `git diff --check -- docs/protocol-matrix.csv docs/protocol-verification-report.md internal/ir/protocol_test.go` passed. That change touched only those two documents and the IR ownership test, preserving existing parser/generator work.
