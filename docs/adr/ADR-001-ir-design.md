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

## Implementation Details

### Directory Structure

```
internal/ir/
├── node.go           # Core IR types (Node, Server, Transport, Security)
├── protocol.go       # Protocol constants and ownership mapping
├── examples.go       # Example nodes for 4 core protocols
├── node_test.go      # Serialization and validation tests
└── schemas/          # JSON Schema definitions
    ├── shadowsocks.json
    ├── vmess.json
    ├── vless.json
    └── trojan.json
```

### Protocol Field Mappings (P0-05)

#### Shadowsocks
- **Required**: `method`, `password`
- **Optional**: `plugin`, `plugin_opts`, `udp`, `udp_over_tcp`
- **Supported methods**: aes-128/192/256-gcm, chacha20-ietf-poly1305, xchacha20-ietf-poly1305, 2022-blake3-* variants

#### VMess
- **Required**: `uuid`, `security`
- **Optional**: `alter_id` (default 0 for AEAD), `packet_encoding`, `global_padding`, `authenticated_length`
- **Security options**: auto, none, aes-128-gcm, chacha20-poly1305, zero

#### VLESS
- **Required**: `uuid`
- **Optional**: `flow` (for XTLS: xtls-rprx-vision), `encryption` (always "none"), `packet_encoding`

#### Trojan
- **Required**: `password`
- **Optional**: `network`, `udp`

### Transport Layer Support

All 4 protocols support multiple transport types via the `Transport` struct:
- **tcp**: Direct TCP connection
- **ws**: WebSocket (with path, host, early data options)
- **grpc**: gRPC transport (with service name, idle timeout)
- **h2**: HTTP/2 transport

### Security Layer Support

- **none**: No TLS
- **tls**: Standard TLS (SNI, ALPN, fingerprint)
- **reality**: Reality protocol (public key, short ID)

### Validation Strategy

1. **Compile-time**: Typed structs for common fields (Server, Transport, Security)
2. **Runtime**: JSON Schema validation for protocol-specific `Proto` fields
3. **Test coverage**: 85.7% (P0-05), covering serialization, deserialization, and field validation

### Example Nodes

8 example nodes in `examples.go`:
- 2 Shadowsocks variants (basic, with plugin)
- 2 VMess variants (WebSocket+TLS, gRPC+TLS)
- 2 VLESS variants (Reality+XTLS, WebSocket+TLS)
- 2 Trojan variants (TLS, WebSocket+TLS)

### Compatibility Notes

**sing-box specific**:
- VMess/VLESS: `packet_encoding` field for xudp support
- Reality security type

**mihomo specific**:
- Trojan: `network` field
- Different YAML field naming (handled by generators)

**Both kernels**:
- Field name translation happens in generators, not IR
- IR uses a neutral naming convention closer to share link standards

## Implementation Notes

- IR defined in `internal/ir/node.go`
- Protocol constants and ownership map in `internal/ir/protocol.go`
- JSON schemas added in P0-05 for 4 core protocols
- Full test suite with 85.7% coverage in `node_test.go`
- Golden tests in P0-06 will verify lossless round-trip for sample nodes

---
*Decision made: 2024-10-31 (P0 phase)*  
*Implementation completed: P0-05 (4 protocol schemas, examples, tests)*
