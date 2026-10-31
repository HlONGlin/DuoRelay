# IR Protocol JSON Schemas

This directory contains JSON Schema definitions for protocol-specific fields in DuoRelay's intermediate representation (IR).

## Purpose

Each protocol has unique configuration fields that go into the `Node.Proto` map. These schemas:
- Document required and optional fields for each protocol
- Specify field types and validation rules
- Support both sing-box and mihomo kernel variants
- Enable runtime validation of parsed nodes

## Schemas

### shadowsocks.json
Shadowsocks protocol with support for multiple cipher methods and plugins.

**Required fields:**
- `method`: Encryption method (aes-128-gcm, chacha20-ietf-poly1305, etc.)
- `password`: Authentication password

**Optional fields:**
- `plugin`: Plugin name (obfs-local, v2ray-plugin, simple-obfs)
- `plugin_opts`: Plugin options string
- `udp`: Enable UDP relay (default: true)
- `udp_over_tcp`: Enable UDP over TCP

### vmess.json
VMess protocol with AEAD encryption support.

**Required fields:**
- `uuid`: User UUID (RFC 4122 format)
- `security`: Encryption method (auto, none, aes-128-gcm, chacha20-poly1305, zero)

**Optional fields:**
- `alter_id`: Alter ID for legacy VMess (deprecated, use 0 for AEAD)
- `packet_encoding`: Packet encoding (packet, xudp) - sing-box specific
- `global_padding`: Enable global padding
- `authenticated_length`: Enable authenticated length

### vless.json
VLESS protocol with XTLS flow control support.

**Required fields:**
- `uuid`: User UUID (RFC 4122 format)

**Optional fields:**
- `flow`: Flow control mode (xtls-rprx-vision, xtls-rprx-vision-udp443)
- `encryption`: Encryption method (always "none" for VLESS)
- `packet_encoding`: Packet encoding (packet, xudp) - sing-box specific

### trojan.json
Trojan protocol with TLS masquerading.

**Required fields:**
- `password`: Authentication password

**Optional fields:**
- `network`: Network type (tcp, udp, ws, grpc) - mihomo specific
- `udp`: Enable UDP relay (default: true)

## Usage

These schemas are used for:

1. **Documentation**: Reference for parser and generator implementations
2. **Validation**: Runtime validation of node configurations (future)
3. **Testing**: Verify example nodes conform to protocol specifications

## Validation Notes

- Current implementation (P0-05) uses manual field checks in tests
- Future implementation may integrate a JSON Schema validator library
- Fields not defined in schemas but present in `Proto` are preserved in `Node.Extra`

## Kernel Compatibility

- **Shared fields**: Work identically in both sing-box and mihomo
- **Kernel-specific fields**: Noted in field descriptions
- **Field translation**: Happens in generators, not at IR level

Example:
```json
{
  "protocol": "shadowsocks",
  "proto": {
    "method": "aes-256-gcm",
    "password": "example-password",
    "udp": true
  }
}
```

## References

- sing-box configuration: https://sing-box.sagernet.org/configuration/outbound/
- mihomo configuration: https://wiki.metacubex.one/config/proxies/
- ADR-001: IR Design Decision Record
