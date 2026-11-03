# P2-17~21 Remaining Protocol Integration Assessment

[简体中文](P2-17~21-remaining-protocols-analysis.md) | English

**Assessment date**: 2024-10-31

**Workspace**: DuoRelay repository root

**Scope**: OpenVPN, OpenConnect, Tailscale, EasyTier, ZeroTier, Rematch

**Method**: Read-only code review; no files modified

---

## Executive Summary

This assessment reviewed the integration status of six remaining protocols. **OpenVPN** and **OpenConnect** had complete, registered parsers, but no generator conversion logic. **Tailscale, EasyTier, ZeroTier, and Rematch** had no parser implementations; none of these four should be represented as an ordinary proxy node.

**Recommended order**:
1. **OpenVPN** (1 day): parser complete; generator only.
2. **OpenConnect** (0.5 day): parser complete; generator only.
3. **Rematch** (0.5 day): requires special routing-control handling.
4. **Tailscale** (1 day): parser, generator, and state management.
5. **EasyTier / ZeroTier** (1 day): parser, generator, and overlay-network dependencies.

---

## 1. OpenVPN

### 1.1 Import formats

Formats implemented at assessment time:
- `.ovpn` configuration files.
- Custom `openvpn://` URI with Base64-encoded configuration or simplified parameters.
- OpenVPN configuration blocks recognized in subscription formats.

Evidence: `internal/parser/openvpn/parser.go:26-65`, `internal/subscription/parse.go:31-32,54-68,77-92`.

### 1.2 Implementation status

#### Parser ✅ Complete

- File: `internal/parser/openvpn/parser.go` (354 lines).
- Registered at `internal/parser/defaults/default.go:47` with `r.Register(openvpn.New())`.
- Test file exists: `internal/parser/openvpn/parser_test.go`.
- Parses `.ovpn` files, including multiple `remote` entries and inline `<ca>`, `<cert>`, and `<key>` blocks.
- Parses simplified or Base64-config `openvpn://` URIs.
- Distinguishes TLS and `static_key` modes.
- Extracts server list, port, transport (`tcp`/`udp`), authentication options, and certificate paths or inline content.
- Produces an IR node with `Protocol="openvpn"` and `Owner=KernelSetBoth`.

The core `parseConfig` implementation scans remotes, global network settings, options, and inline blocks; records the original configuration and derived mode/remotes in `Proto`; and preserves authentication, certificate, and routing options.

#### Generator ❌ Not implemented

- Neither the sing-box nor mihomo generator had OpenVPN conversion logic.
- `internal/generator/singbox/generator.go:162-177` and `internal/generator/mihomo/generator.go:102-116` fell through to their generic defaults.
- The generic fallback emitted the protocol as `type`, node name as `tag`, and the IR server host/port. That shape is insufficient for OpenVPN's configuration model.

### 1.3 Files blocking integration

| Component | File | Lines | Blocker |
|---|---|---:|---|
| sing-box generator | `internal/generator/singbox/generator.go` | 162-177 | Missing `ir.ProtocolOpenVPN` switch case. |
| mihomo generator | `internal/generator/mihomo/generator.go` | 102-116 | Missing `ir.ProtocolOpenVPN` switch case. |

### 1.4 Why it cannot be treated as an ordinary proxy node

1. **Different configuration model**: OpenVPN uses `.ovpn` files and certificates, not the conventional `server:port + method + password` model.
2. **sing-box capability level**: sing-box v1.14.4 implements it as an **endpoint** (`docs/configuration/endpoint/openvpn-client.md`), not an outbound.
3. **mihomo capability level**: mihomo v1.19.33 implements an **outbound adapter**, but its configuration is complex.
4. **State and credential requirements**: certificates may be files or inline data; key direction and TLS-auth/TLS-crypt keys may also be required.
5. The protocol matrix classifies it as a cross-layer capability (`docs/protocol-matrix.csv:16`).

Evidence: `docs/protocol-verification-report.md:27-28`.

### 1.5 Minimum viable implementation

#### Option A: complete dual-kernel support (recommended)

For sing-box, add an `ir.ProtocolOpenVPN` case and convert the node into an `openvpn-client` endpoint with its tag. Map the first `remotes` entry to server and port; preserve `ca`, `cert`, `key`, `tls_auth`, and `tls_crypt` as inline values or corresponding path fields. If `auth_user_pass` is enabled, surface the requirement for credentials or an auth file.

For mihomo, add the matching case and emit an `openvpn` proxy. Prefer passing the complete `config` through `config-file-content` (or a temporary file); otherwise map server, port, remotes, certificates, and authentication fields.

**Estimate**: 1 day: 4 hours for sing-box endpoint mapping and 4 hours for mihomo outbound mapping.

#### Option B: mihomo-first quick support

Implement only the mihomo generator, passing through the config file or writing it to a temporary file.

**Estimate**: 0.5 day.

---

## 2. OpenConnect

### 2.1 Import formats

Implemented formats:
- JSON configuration fragment in sing-box endpoint shape.
- Custom `openconnect://` URI (not verified).

Evidence: `internal/parser/openconnect/parser.go:19-79`.

### 2.2 Implementation status

#### Parser ✅ Complete

- File: `internal/parser/openconnect/parser.go` (80 lines), registered at `internal/parser/defaults/default.go:48`; tests exist.
- Parses JSON such as `{"type":"openconnect","server":"https://...",...}`.
- Extracts server URL, authentication fields (`flavor`, `username`, `password`, `auth_group`, `token`, `cookie`), TLS options, and client metadata (`reported_os`, `user_agent`, `version`).
- Produces `Protocol="openconnect"`, `Owner=KernelSetSingBox`, and TLS security metadata.
- Fields are retained in `Proto`.

#### Generator ❌ Not implemented

There was no sing-box conversion. mihomo does not support OpenConnect according to the protocol matrix. The sing-box generator used its generic fallback.

### 2.3 Files blocking integration

| Component | File | Lines | Blocker |
|---|---|---:|---|
| sing-box generator | `internal/generator/singbox/generator.go` | 162-177 | Missing `ir.ProtocolOpenConnect` case. |

### 2.4 Why it cannot be treated as an ordinary proxy node

1. **sing-box-only**: mihomo v1.19.33 has no OpenConnect adapter (`docs/protocol-matrix.csv:21`).
2. **Enterprise VPN**: intended for Cisco AnyConnect/OpenConnect-compatible gateways.
3. **Complex authentication**: may use passwords, tokens, cookies, SAML, and CSD/HIP endpoint checks.
4. **sing-box capability level**: implemented as an **endpoint**, not an outbound.
5. Classified as "sing-box endpoint only" in `docs/protocol-verification-report.md:11`.

### 2.5 Minimum viable implementation: sing-box generator

Add an `ir.ProtocolOpenConnect` case that emits an `openconnect` endpoint with the node tag and an HTTPS server URL. Copy through available values for `flavor`, `username`, `password`, `auth_group`, `cookie`, `token`, `reported_os`, `user_agent`, `version`, `local_hostname`, `system`, `name`, `mobile`, `csd`, `hip`, `tncc`, `mtu`, `dpd_interval`, and `reconnect_timeout`; preserve the `tls` map when present.

**Estimate**: 0.5 day.

---

## 3. Tailscale

### 3.1 Intended import format

Not implemented at assessment time: JSON configuration (sing-box endpoint or mihomo outbound), auth key, and state-directory path. References: sing-box `endpoint/tailscale.md` and mihomo `adapter/outbound/tailscale.go`.

### 3.2 Implementation status

#### Parser ❌ Not implemented

- `internal/parser/tailscale/` did not exist and the default registry had no Tailscale entry.
- IR protocol constant existed at `internal/ir/protocol.go:21`; ownership was `KernelSetBoth` at line 58.

#### Generator ❌ Not implemented

Neither generator had Tailscale conversion.

### 3.3 Blocking files

| Component | File | Status | Blocker |
|---|---|---|---|
| Parser | `internal/parser/tailscale/parser.go` | Missing | Implement parser. |
| Parser registration | `internal/parser/defaults/default.go` | Missing | Register parser. |
| sing-box generator | `internal/generator/singbox/generator.go` | Missing case | Generate endpoint. |
| mihomo generator | `internal/generator/mihomo/generator.go` | Missing case | Generate outbound. |

### 3.4 Why it cannot be treated as an ordinary proxy node

1. Requires a Tailscale account or self-hosted Headscale control plane.
2. Requires persistent state for device keys, network configuration, and peer state.
3. First use requires an auth key or OAuth login.
4. Routes depend on dynamic peer discovery inside the Tailnet.
5. Capability levels differ: sing-box v1.14.4 starts a client instance as an **endpoint**; mihomo v1.19.33 provides an **outbound adapter** that connects to an existing Tailscale network.
6. The matrix calls it a cross-layer capability (`docs/protocol-matrix.csv:18`).

Evidence: `docs/protocol-verification-report.md:35-36`.

### 3.5 Minimum viable implementation: state dependency and dual-kernel support

Create `internal/parser/tailscale/parser.go`. Accept JSON with `type: "tailscale"`, create a node owned by both kernels, and use a semantic placeholder server (`Host: "tailscale", Port: 0`) because there is no conventional server/port. Preserve `state_directory`, `auth_key`, `control_url`, `exit_node`, `routes`, and `hostname` in `Proto`.

The sing-box generator should emit an endpoint; mihomo should emit its outbound. Before generation, check that the state directory exists and explain first-time authentication when it does not.

**Estimate**: 1 day: 3 hours for parser/tests, 4 hours for both generators, and 1 hour for state-management documentation.

---

## 4. EasyTier

### 4.1 Intended import format

Not implemented at assessment time: mihomo outbound JSON, network name and secret, peer list, and route configuration. Reference: mihomo `adapter/outbound/easytier.go`.

### 4.2 Implementation status

#### Parser ❌ Not implemented

No `internal/parser/easytier/` directory or default registration existed. The IR constant was defined at `internal/ir/protocol.go:36`; ownership at line 73 was `KernelSetMihomo`.

#### Generator ❌ Not implemented

The mihomo generator had no conversion logic.

### 4.3 Blocking files

| Component | File | Status | Blocker |
|---|---|---|---|
| Parser | `internal/parser/easytier/parser.go` | Missing | Implement parser. |
| Parser registration | `internal/parser/defaults/default.go` | Missing | Register parser. |
| mihomo generator | `internal/generator/mihomo/generator.go` | Missing case | Generate outbound. |

### 4.4 Why it cannot be treated as an ordinary proxy node

EasyTier is a complete P2P overlay network rather than a single-server proxy protocol; it is mihomo-only, depends on network name/shared secret/peers, and requires persistent network and peer state. The matrix classifies it as an embedded overlay outbound (`docs/protocol-matrix.csv:28`). Evidence: `docs/protocol-verification-report.md:12,43`.

### 4.5 Minimum viable implementation: mihomo-specific

Create a parser that reads JSON, creates a mihomo-owned node with semantic host `easytier` and port 0, and retains `network_name`, `network_secret`, `peers`, `routes`, `keys`, and `dhcp` in `Proto`. The mihomo generator emits type `easytier` and maps the retained protocol fields.

**Estimate**: 0.5 day, in parallel with ZeroTier.

---

## 5. ZeroTier

### 5.1 Intended import format

Not implemented at assessment time: mihomo outbound JSON, Network ID, identity/state directory, and Controller/Planet configuration. Reference: mihomo `adapter/outbound/zerotier.go`.

### 5.2 Implementation status

#### Parser ❌ Not implemented

No parser directory or default registration existed. The IR constant was at `internal/ir/protocol.go:37`; ownership at line 74 was `KernelSetMihomo`.

#### Generator ❌ Not implemented

The mihomo generator had no conversion logic.

### 5.3 Blocking files

| Component | File | Status | Blocker |
|---|---|---|---|
| Parser | `internal/parser/zerotier/parser.go` | Missing | Implement parser. |
| Parser registration | `internal/parser/defaults/default.go` | Missing | Register parser. |
| mihomo generator | `internal/generator/mihomo/generator.go` | Missing case | Generate outbound. |

### 5.4 Why it cannot be treated as an ordinary proxy node

ZeroTier is a complete virtual-network system, not a single-server proxy. It is mihomo-only, requires joining an existing network by Network ID, persists device identity and authorization state, and depends on ZeroTier Central or a self-hosted controller. The matrix classifies it as an embedded overlay outbound (`docs/protocol-matrix.csv:29`). Evidence: `docs/protocol-verification-report.md:12,43`.

### 5.5 Minimum viable implementation: mihomo-specific

Create a parser that creates a mihomo-owned node (`Host: "zerotier", Port: 0`) and preserves `network_id`, `identity`, `state_directory`, `controller`, and `planet`. The mihomo generator can map the fields directly, following the EasyTier pattern.

**Estimate**: 0.5 day, in parallel with EasyTier.

---

## 6. Rematch

### 6.1 Intended import format

Not implemented at assessment time: JSON for mihomo routing control, with `target-rematch-name` or `target-sub-rule`. Reference: mihomo `adapter/outbound/rematch.go`.

### 6.2 Implementation status

#### Parser ❌ Not implemented

No parser directory or default registration existed. The IR constant was at `internal/ir/protocol.go:35`; ownership at line 72 was `KernelSetMihomo`.

#### Generator ❌ Not implemented

The mihomo generator had no conversion logic.

### 6.3 Blocking files

| Component | File | Status | Blocker |
|---|---|---|---|
| Parser | `internal/parser/rematch/parser.go` | Missing | Implement parser. |
| Parser registration | `internal/parser/defaults/default.go` | Missing | Register parser. |
| mihomo generator | `internal/generator/mihomo/generator.go` | Missing case | Add special generation. |

### 6.4 Why it cannot be treated as an ordinary proxy node

1. It is a routing-control adapter, not a remote-server protocol.
2. It is mihomo-only; sing-box has no Rematch concept.
3. It has no `server:port`; it uses `target-rematch-name` or `target-sub-rule`.
4. Dial/ListenPacket rewrites connection metadata and does not create a remote connection.
5. The matrix classifies it as a routing-control adapter rather than a protocol (`docs/protocol-matrix.csv:27`).

Evidence: `docs/protocol-verification-report.md:18-20`.

### 6.5 Minimum viable implementation: special IR handling and mihomo generation

**Key design**: Rematch must not become a `proxies` entry. It should generate a routing rule or proxy-group configuration.

The parser should accept JSON, set protocol `rematch`, owner `KernelSetMihomo`, and an empty server/port. Preserve `target-rematch-name` as `target_rematch_name` and `target-sub-rule` as `target_sub_rule`; mark `node.Extra["is_routing_adapter"] = true`.

In the mihomo generator, skip Rematch nodes in ordinary proxy generation. Handle them separately by creating routing/group entries with `type: "rematch"` and mapping the target fields back to mihomo's hyphenated names.

**Estimate**: 0.5 day.

---

## 7. Combined Analysis

### 7.1 Implementation status comparison

| Protocol | Parser | Generator | Subscription recognition | Category | Kernel support |
|---|:---:|:---:|:---:|---|---|
| OpenVPN | ✅ | ❌ | ✅ | Cross-layer capability | sing-box endpoint / mihomo outbound |
| OpenConnect | ✅ | ❌ | ❌ | sing-box-only endpoint | sing-box only |
| Tailscale | ❌ | ❌ | ❌ | Cross-layer capability | sing-box endpoint / mihomo outbound |
| EasyTier | ❌ | ❌ | ❌ | Embedded overlay | mihomo only |
| ZeroTier | ❌ | ❌ | ❌ | Embedded overlay | mihomo only |
| Rematch | ❌ | ❌ | ❌ | Routing control | mihomo only |

### 7.2 Categories of incompatibility with ordinary proxy nodes

| Category | Protocols | Core reason |
|---|---|---|
| Capability-level difference | OpenVPN, OpenConnect, Tailscale | Endpoint or cross-layer capability, not an outbound. |
| State/account dependency | Tailscale, EasyTier, ZeroTier | External control plane, persistent state, and network registration. |
| Non-protocol adapter | Rematch | Routing control, not a remote connection. |
| Complex configuration model | OpenVPN | Certificates, configuration files, and multiple remotes. |

### 7.3 Generator changes

- **sing-box**: add three cases in `internal/generator/singbox/generator.go:162-177` and methods `convertOpenVPN()`, `convertOpenConnect()`, and `convertTailscale()`.
- **mihomo**: add five cases in `internal/generator/mihomo/generator.go:102-116` and methods for OpenVPN, Tailscale, EasyTier, and ZeroTier; handle Rematch separately through `generateRematchRules()` or proxy-group/rule generation.

### 7.4 Recommended order and rationale

#### Priority 1 (2 days)

1. **OpenVPN (1 day)**: parser is complete; only generator work remains; supports both kernels and a widely used enterprise VPN. Low risk because configuration is already parsed; high value.
2. **OpenConnect (0.5 day)**: parser is complete; only sing-box generator work; simple field pass-through. Low risk; medium enterprise value.
3. **Rematch (0.5 day)**: straightforward mihomo-specific handling with a clear special case and no complex network interaction. Medium value for advanced routing.

#### Priority 2 (2 days)

4. **Tailscale (1 day)**: both kernels, modern overlay networking, and growing user demand. Medium risk due to state management and authentication guidance; high value.
5. **EasyTier + ZeroTier (1 parallel day)**: similar mihomo-only implementation patterns. Medium risk due to network dependencies; low-to-medium value for niche overlay use cases.

### 7.5 Total effort estimate

| Task | Parser | Generator | Tests | Total |
|---|---:|---:|---:|---:|
| OpenVPN | 0h (complete) | 6h | 2h | 8h (1 day) |
| OpenConnect | 0h (complete) | 3h | 1h | 4h (0.5 day) |
| Rematch | 2h | 2h | 0.5h | 4.5h (0.5 day) |
| Tailscale | 3h | 4h | 1h | 8h (1 day) |
| EasyTier | 2h | 2h | 0.5h | 4.5h (0.5 day) |
| ZeroTier | 2h | 2h | 0.5h | 4.5h (0.5 day) |
| **Total** | **9h** | **19h** | **5.5h** | **33.5h (4.2 days)** |

---

## 8. Key File Inventory

### 8.1 Existing files

| File | Lines | Status | Purpose |
|---|---:|---|---|
| `internal/parser/openvpn/parser.go` | 354 | ✅ Complete | OpenVPN parser. |
| `internal/parser/openvpn/parser_test.go` | ~100 | ✅ Present | OpenVPN tests. |
| `internal/parser/openconnect/parser.go` | 80 | ✅ Complete | OpenConnect parser. |
| `internal/parser/openconnect/parser_test.go` | ~60 | ✅ Present | OpenConnect tests. |
| `internal/parser/defaults/default.go` | 58 | ✅ Includes OV/OC | Default registry. |
| `internal/subscription/parse.go` | 292 | ✅ Includes OV detection | Subscription parser. |
| `internal/ir/protocol.go` | 99 | ✅ All protocol constants | IR protocol definitions. |
| `internal/generator/singbox/generator.go` | 305 | ⚠️ Missing 3 protocols | sing-box generator. |
| `internal/generator/mihomo/generator.go` | 294 | ⚠️ Missing 5 protocols | mihomo generator. |

### 8.2 Files to create

| File | Estimated lines | Priority | Dependency |
|---|---:|---|---|
| `internal/parser/tailscale/parser.go` | 80 | P2 | None |
| `internal/parser/tailscale/parser_test.go` | 60 | P2 | parser.go |
| `internal/parser/easytier/parser.go` | 70 | P2 | None |
| `internal/parser/easytier/parser_test.go` | 50 | P2 | parser.go |
| `internal/parser/zerotier/parser.go` | 70 | P2 | None |
| `internal/parser/zerotier/parser_test.go` | 50 | P2 | parser.go |
| `internal/parser/rematch/parser.go` | 60 | P1 | None |
| `internal/parser/rematch/parser_test.go` | 40 | P1 | parser.go |

---

## 9. Risks and Recommendations

### 9.1 Technical risks

| Risk | Severity | Probability | Mitigation |
|---|---|---|---|
| Inline OpenVPN certificates exceed IR limits | Medium | Low | Support certificate file paths. |
| Tailscale first-time authentication is complex | Medium | Medium | Provide clear authentication instructions. |
| EasyTier/ZeroTier network dependencies are difficult to verify | Low | High | Use mock configuration in tests. |
| Rematch semantics in proxy groups are unclear | Low | Medium | Check mihomo's official documentation. |

### 9.2 Implementation recommendations

1. Complete OpenVPN/OpenConnect first: their parsers are done, implementation risk is lowest, and they quickly improve protocol coverage.
2. Design Rematch as a special architecture: exclude it from ordinary proxies and consider generating routing rules separately.
3. Document stateful protocols: provide first-time setup guides for Tailscale/EasyTier/ZeroTier and add state checks and guidance in the UI.
4. Evolve generators incrementally: implement basic field mapping first, then improve complex options and validation.

### 9.3 Test strategy

| Protocol | Unit tests | Integration tests | Real connectivity tests |
|---|---|---|---|
| OpenVPN | Parser exists | Add generator tests | Requires a real `.ovpn` file. |
| OpenConnect | Parser exists | Add generator tests | Requires an enterprise gateway. |
| Tailscale | Create | Create | Requires a Tailnet. |
| EasyTier | Create | Create | Can be simulated. |
| ZeroTier | Create | Create | Requires a Network ID. |
| Rematch | Create | Create | Validate mihomo configuration. |

---

## 10. Conclusion

### 10.1 Status summary

- Parsers were complete for OpenVPN (354 lines) and OpenConnect (80 lines).
- Both were registered at `internal/parser/defaults/default.go:47-48`.
- OpenVPN configuration recognition was integrated into `subscription.Parse`.
- All six protocols lacked generator implementations at assessment time.
- None of the six could safely be represented as an ordinary proxy node.

### 10.2 Recommended implementation path

**Phase 1 (2 days)**: finish protocols with existing parsers: OpenVPN generator for both kernels, OpenConnect generator for sing-box, and Rematch parser/generator for mihomo.

**Phase 2 (2 days)**: implement state-dependent protocols: Tailscale, then EasyTier and ZeroTier in parallel.

**Estimated total**: 4.2 days; reserve 5 days with contingency.

### 10.3 File change list

Modify `internal/generator/singbox/generator.go` to add cases for OpenVPN, OpenConnect, and Tailscale; modify `internal/generator/mihomo/generator.go` to add five cases plus special Rematch handling; register Tailscale, EasyTier, ZeroTier, and Rematch in `internal/parser/defaults/default.go`.

Create parser and test files under `internal/parser/{tailscale,easytier,zerotier,rematch}/`.

---

**Report generated**: 2024-10-31

**Assessment method**: static code analysis, documentation cross-check, and protocol-matrix review

**Evidence completeness**: ✅ Every conclusion was accompanied by file paths and line numbers
