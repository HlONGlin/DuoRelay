# ADR-001: Intermediate Representation (IR) Design

## Status
Proposed (P0-05)

## Context
DuoRelay needs to support 28 protocols across two kernels (sing-box and mihomo). Each kernel has its own configuration format (JSON vs YAML) and field naming conventions. We need a unified internal representation that can:

1. Parse various share link formats
2. Store nodes in database
3. Generate configs for both kernels
4. Handle protocol-specific fields without tight coupling

## Decision

We adopt a **four-layer IR structure**:

```go
type Node struct {
    Server    Server            // Host, Port
    Transport Transport         // tcp/ws/grpc/h2/quic + options
    Security  Security          // none/tls/reality + SNI/ALPN/fingerprint
    Proto     map[string]any    // Protocol-specific validated fields
    Extra     map[string]any    // Passthrough for unknown fields
}
```

### Key Principles

1. **Separation of concerns**: Common fields (server, transport, security) are typed structs; protocol-specific fields go into `Proto` map
2. **Lossless passthrough**: Fields the panel doesn't understand go into `Extra` and are preserved when generating configs
3. **Ownership tracking**: Each node knows which kernel(s) can run it via `Owner KernelSet`
4. **Schema validation**: Each protocol has a JSON Schema to validate its `Proto` fields

### Protocol Ownership

- **Both kernels** (16): Shadowsocks, VMess, VLESS, Trojan, Hysteria, Hysteria2, TUIC, AnyTLS, Snell, SOCKS5, HTTP, SSH, WireGuard, OpenVPN, MASQUE, Tailscale
- **sing-box only** (4): ShadowTLS, Naive, Tor, OpenConnect  
- **mihomo only** (8): ShadowsocksR, Mieru, Sudoku, ShadowQUIC, TrustTunnel, Rematch, EasyTier, ZeroTier

## Consequences

### Positive
- Single source of truth for node data
- Easy to add new protocols without modifying core IR
- Generators can focus on format conversion, not semantics
- Database schema remains stable

### Negative
- `Proto` and `Extra` lose compile-time type safety (mitigated by schema validation)
- Need to maintain JSON schemas for 28 protocols

### Risks
- If a protocol's fields vary significantly between kernels, may need kernel-specific `Proto` variants

## Alternatives Considered

1. **Separate IR per kernel**: More duplication, harder to switch nodes between kernels
2. **Fully typed structs per protocol**: 28 struct types * 2 for each generator = high maintenance
3. **Protocol interface**: Requires all protocols implement same interface, limits flexibility

## Implementation Notes

- IR defined in `internal/ir/node.go`
- Protocol constants and ownership map in `internal/ir/protocol.go`
- JSON schemas will be added in P1-05~P1-07 during parser implementation
- Golden tests in P0-06 verify lossless round-trip for sample nodes

---
*Decision made: 2024-10-31 (P0 phase)*
