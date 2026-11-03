# Subscription Import Formats

[简体中文](subscription-formats.md) | English

Enter an HTTP(S) URL in subscription management, or paste a response body or collection of links into the node import dialog. Both use the same parser and do not depend on the provider's Content-Type.

| Format | Supported input |
|---|---|
| v2rayN / V2RayNG link list | One share link per line; protocols may be mixed; blank lines and standalone comments are supported. |
| Base64 | Standard or URL-safe, with or without padding, split across lines; up to three wrapping layers for the formats below. |
| Clash / Mihomo | YAML or JSON `proxies` and JSON node arrays. |
| sing-box | JSON `outbounds` and outbound arrays; skips control entries such as direct and selector. |
| Xray / V2Ray | JSON `outbounds` or a single outbound; VMess, VLESS, Trojan, SS, SOCKS and HTTP; TCP, WS, gRPC, TLS and Reality. |
| v2rayN JSON | VMess share objects, existing project-compatible node objects, arrays and `configs` wrappers. |
| SIP008 | `servers` object or server array, retaining Shadowsocks plugin parameters. |
| SSD | `ssd://` encoded server lists with global defaults and per-server overrides. |
| Surge / Loon | `[Proxy]` sections or bare node lines; common SS, SSR, VMess, VLESS, Trojan, HTTP and SOCKS parameters. |
| Quantumult X | `[server_local]` sections or bare node lines; common authentication, TLS, WS and SS obfuscation parameters. |
| OpenVPN | Existing native configuration import. |

## Share Links

- SS: SIP002/SIP003 Base64 userinfo, plaintext method/password, and legacy whole-authority Base64.
- VMess: v2rayN Base64 JSON and endpoint-style credential links.
- Hysteria2: `hysteria2://` and `hy2://`; authentication passwords containing colons are preserved.
- AnyTLS: existing `anytls://` support.
- Mieru: project endpoint links, native `mieru://` protobuf configuration and simplified `mierus://` links. Nodes are split by server and port binding; TCP/UDP, port ranges, multiplexing, handshake mode and traffic pattern are preserved.
- URL name fragments are decoded only once, preserving plus signs and escaped percent signs. gRPC serviceName, TLS fingerprint, verification options and extra WS request headers are passed to the generator.

## Limitations

- Import includes proxy nodes only. It does not migrate other clients' listeners, rules, policy groups, DNS, scripts or modules.
- INI input reads node sections only. Unsupported protocols or critical options produce line-numbered errors rather than silently switching connection methods.
- Xray transports without completed generator mappings, including XHTTP and mKCP, are not supported. Complex chained dialing and transport controls are not fully converted.
- Mieru native plaintext credentials can be imported. Configurations containing only password hashes, a custom MTU other than the upstream default of 1400, or an embedded dialer produce errors. Mihomo generates the nodes.
- If the subscription fails to parse as a whole, existing nodes are retained so partial success cannot mistakenly delete them. Errors do not echo complete links or credentials.
- Import tests do not establish real server connectivity. Kernel version and server settings also affect connectivity.

## Verification

`internal/subscription/compat_test.go` covers mixed links, three wrapping formats, BOM/UTF-16, added formats, malformed input, actual HTTP download through database insertion, preservation of existing nodes on failure, and retention of key parameters from IR to both kernel generators.

`internal/api/server_test.go` also verifies SIP008, Surge and Mieru import APIs and rejection of empty node sets.
