# OpenVPN sing-box Generator Report

Task: task_25774b4d4a13. Target: `internal/generator/singbox`. Pinned kernel: sing-box 1.14.4 (`kernels/sing-box.exe`, built with `with_openvpn`).

## Placement

OpenVPN nodes are emitted under root `endpoints` as `type: "openvpn-client"` (`constant/proxy.go` `TypeOpenVPNClient`), never under `outbounds`. `convertNode` returns nil for OpenVPN, so no generic fallback outbound can be produced. Nodes that cannot be represented make `Generate` return an error.

Schema source: `option/openvpn.go` and `docs/configuration/endpoint/openvpn-client.md` at tag v1.14.4. Validation rules were cross-checked against `protocol/openvpn/client.go` and `endpoint.go`.

## Files

- `internal/generator/singbox/openvpn.go` (new): IR to endpoint mapping.
- `internal/generator/singbox/generator.go`: classifies OpenVPN with OpenConnect; `generateOutboundsAndEndpoints` now returns an error.
- `internal/generator/singbox/openvpn_test.go` (new): tests.

## Mapping

| IR / profile | sing-box field |
| --- | --- |
| `Proto["remotes"]` (multi-remote, per-remote network) | `servers[]` (`server`, `server_port`, `network`) |
| `Proto["network"]`, `remote_random` | `network`, `remote_random` |
| `mode` (`tls` / `static_key`) | `mode` |
| `username`, `password`, `auth-retry`, `static-challenge` | same-named fields |
| `ca`/`cert`/`key` inline or `_path` | `tls.certificate`, `tls.client_certificate`, `tls.client_key` (and `_path`) |
| `tls_auth`/`tls_crypt`/`tls_crypt_v2` + `key-direction` | `tls.control_wrap` (`type`, `key`/`key_path`, `direction`) |
| `verify_x509_name`, `peer-fingerprint`, `crl-verify`, `remote-cert-*`, `ns-cert-type`, `tls-version-*`, `tls-cipher`, `tls-groups`, `tls-cert-profile` | `tls.*` |
| `data_ciphers`, `data_ciphers_fallback`, `cipher`, `auth` | `data_ciphers`, `data_ciphers_fallback`, `cipher` (static_key only), `auth` |
| `comp-lzo`, `compress`, `mssfix`, `fragment`, `replay-window`, `explicit-exit-notify`, `tun_mtu` | `compression_lzo`, `compression`, `mss_fix*`, `fragment`, `replay_window*`, `explicit_exit_notify`, `mtu` |
| `ping`, `ping_restart`, `reneg-*`, `tls-timeout`, `hand-window` | `ping_interval`, `ping_restart*`, `renegotiate_*`, `tls_timeout`, `handshake_window` |
| `route_nopull`, `pull-filter`, `route`, `route-ipv6`, `route-gateway`, `route-metric`, `redirect-gateway [flags]`, `redirect-private`, `block-ipv6` | `route_no_pull`, `pull_filters`, `routes`, `route_gateway`, `route_metric`, `redirect_gateway(_flags)`, `redirect_private`, `block_ipv6` |
| `secret` inline / path, `ifconfig`, `ifconfig-ipv6` | `static_key` / `static_key_path`, `address`, `peer_address(_ipv6)` |
| `local` | `inet4_bind_address` / `inet6_bind_address` |

Semantic adjustments:
- In TLS mode sing-box rejects `cipher`. A profile `cipher` is mapped to `data_ciphers_fallback` (OpenVPN 2.5+ behavior) unless an explicit fallback exists.
- sing-box defaults `tls.remote_certificate_tls` to `server`, OpenVPN checks nothing by default. Absent `remote-cert-tls` is emitted as `none` to preserve semantics.

## Rejected (error, nothing emitted)

- `Security.Opts.insecure=true` (no OpenVPN equivalent).
- `dev tap` / non-tun, `proto tcp-server`.
- `pkcs12`, `http-proxy`, `socks-proxy`, `tls-ciphersuites`, `connection`, `tls-server`, `cryptoapicert`, inline `<pkcs12>`, `<connection>`, `<crl-verify>`.
- `redirect-gateway ... block-local`; `route` with an explicit gateway or per-route metric; `crl-verify ... dir`.
- `auth-user-pass` with no username/password in the IR (credential files and prompts are not resolved).
- TLS mode with no CA and no `peer-fingerprint` (sing-box requires one; bare `openvpn://host` share URIs therefore fail until a CA is supplied).
- `static_key` without `ifconfig`; TLS-only options in `static_key` mode.
- Invalid fingerprints (must be SHA-256), unknown enum values, more than one of tls-auth/tls-crypt/tls-crypt-v2.

## Mapping gaps

- The parser lifts only a subset of directives into `Proto`. Everything else is recovered by re-scanning `Proto["config"]`. Share URIs without a full profile expose only the parser's fields, so `remote_cert_tls`, `ifconfig` and similar are unavailable there.
- Parser stores `auth-user-pass <file>` as `auth_user_pass_file` but does not read it; the generator errors unless credentials are in the IR.
- `tls_auth` path form with the direction argument is read from the raw profile; the parser's `tls_auth_path` keeps only the first argument.
- `system`, `name`, `udp_*` NAT fields and server-side options are not generated (no IR source).
- The tun-only schema means `dev tap` and bridged setups are unsupported by the kernel.

## Evidence

- `go test -count=1 ./internal/generator/singbox ./internal/generator/mihomo ./internal/configuration` passes.
- `kernels/sing-box.exe check -c` (v1.14.4) is run by `singBoxCheck` in `TestGenerateOpenVPNTLSProfile` (multi-remote, inline CA/cert/key, tls-auth, routes, pull filters), `TestGenerateOpenVPNShareURI` and `TestGenerateOpenVPNStaticKey`. Tests skip if the binary is absent; it is present locally.
- `TestGenerateOpenVPNRejectsUnsupported` covers 10 rejection paths.
- Not verified: an actual tunnel connection to an OpenVPN server (only config validation).
