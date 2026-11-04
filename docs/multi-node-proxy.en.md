# Multi-Node Proxy, SOCKS5 Port Bindings and Inbound Restrictions

[简体中文](multi-node-proxy.md) | English

## Node Management

Select nodes and click **Load balancing** or **Proxy chain** at the top of the page. Each opens its own dialog, where nodes may also be added or removed and the composition named. **Create** saves the member order and strategy as a new composition node; it does not start or replace the running proxy. Start the new node separately from the node list.

- **Load balancing:** Select 2–64 nodes supported by mihomo. New connections are distributed using round robin, consistent hashing, sticky sessions, automatic selection (url-test), or fallback. Automatic selection chooses by health-check latency; fallback uses the first healthy node in list order. Set the health-check URL, interval, timeout and on-demand checking in the dialog; **Settings → Load balancing** saves defaults for the next opening. A single connection's bandwidth is not combined across nodes. Mihomo skips members that fail health checks; connection success cannot be guaranteed when no healthy nodes remain.
- **Proxy chain:** Select 2–8 nodes. The order is local device → first entry hop → intermediate hops → final exit → destination; move nodes up or down to reorder. The system chooses kernels according to actual protocol parameters, preferring a single kernel and bridging sing-box and mihomo when needed; alternating kernels are supported. The dialog shows each hop's kernel and connection transport and the chain's UDP capability, or the failing hop and reason.

After starting a composition node, the top of the page shows its mode and members, including chain order. Stopping stops the entire group and clears the current selection. Configuration is validated before startup; invalid parameters do not stop an existing instance. Startup failure attempts to restore the previous instance. Members referenced by a composition cannot be deleted individually; delete the composition first. If a subscription removes a referenced member, DuoRelay retains it as a regular node so the composition remains usable.

Select rows in the node table, or use the header checkbox to select the current filtered results, then choose **Delete selected**. The batch is deleted in one transaction. If any node is missing, running, or assigned to a saved or active SOCKS5 listener, the entire batch is retained and an error is shown. Stop the proxy or remove the affected listener assignment before trying again.

The kernels use `dialer-proxy` and `detour` respectively for chaining; multi-node selection uses mihomo native proxy groups and health checks. Fallback priority follows dialog list order. Strategy changes affect subsequent new connections; existing connections remain on their original node. Duplicate display names are renamed only in generated configuration, not in the database.

## Chain Compatibility

| Case | Handling |
|---|---|
| HTTP, SOCKS5, SS, VMess, VLESS, Trojan, AnyTLS, Snell, SSH, SSR, Tor SOCKS | Generated according to the actual kernel and version; HTTP / SSH / Tor exits carry TCP only. |
| Mieru, Sudoku, TrustTunnel, Naive | Supported in chains; previous-hop capability is checked against actual TCP, UDP or QUIC mode. |
| Hysteria v1 / v2, TUIC, ShadowQUIC, WireGuard, MASQUE H3 | Can be used as the first hop; later hops require the preceding path to forward UDP. |
| MASQUE H2, Mieru TCP, TrustTunnel TCP | Can follow a TCP proxy; MASQUE uses mihomo. |
| ShadowTLS | A transport wrapper that must be paired with a following Shadowsocks hop on the same kernel; cannot be the final exit. |
| OpenVPN, OpenConnect, Tailscale, EasyTier, ZeroTier | Chain generation is supported; VPN/network routes, exit permissions and authentication must be configured. Public access requires a server-provided exit; private subnets do not automatically become public proxies. |
| Rematch | A routing rematch adapter, not an independent proxy connection; the dialog explains it cannot be a proxy hop. |
| QUIC after a TCP entry | Rejected with a request to reorder if the entry cannot forward UDP; it does not silently connect directly. |
| Cross-kernel | A SOCKS5 bridge with random credentials is created on 127.0.0.1. Internal routes precede user rules and SOCKS inbound restrictions. |

UDP capability depends on the actual path. Protocols carrying datagrams over TCP can provide UDP after a TCP proxy; native SOCKS/SS datagrams still depend on previous-hop UDP capability. A chain without UDP rejects only UDP routed to the selected proxy outbound, retaining mainland bypass, blocklist and other direct rules. User rules cannot select an intermediate hop to bypass the rest of the chain.

Naive on Windows amd64 requires `kernels/libcronet.dll`. Run `powershell -File scripts/install-cronet-windows.ps1`; the script reads the pinned version from `kernels.lock` and checks SHA256. An existing different version is not overwritten.

## Multiple SOCKS5 Ports Pinned to Nodes

Under **Settings → General → Bind nodes to SOCKS5 ports**, add a port and select its node. You can configure up to 32 additional listeners. Ports must be in the range `1–65535`, be unique, and not conflict with DuoRelay's configured proxy or transparent-proxy ports.

Each added port connects directly to its assigned node, independently of the active proxy node or group, load-balancing strategy and routing rules. It does not change the mixed ports or the **Proxy only through the SOCKS5 port** option below. All listeners use the configured inbound listen address, which defaults to localhost. SOCKS listeners do not configure username/password authentication; expose them only to a trusted LAN and configure firewall access as needed.

Full uses sing-box or mihomo according to the active kernel; lite uses mihomo. The assigned node must be supported by the kernel currently running. After saving settings, reapply the active node or group from the Nodes page. Example client address: `socks5://127.0.0.1:18081`; replace it with the actual listen address and port.

## Proxy Only Through the SOCKS5 Port

Enable the switch under **Settings → General**, set the dedicated SOCKS5 port (default 10808), save, and restart the node or group.

- The dedicated SOCKS5 port selects the current single node, load balancing group or chain using existing routing rules. Bypass rules still apply; for all traffic on this inbound to use the proxy, select the global preset and proxy outbound.
- Mixed ports and the TUN inbound prefer direct connections; custom proxy rules do not override this restriction.
- The SOCKS5 port must differ from both kernels' mixed ports. While enabled, the API rejects direct application of raw configuration to prevent bypassing the inbound restriction.
- Applications must explicitly connect to `socks5://127.0.0.1:10808` (or the configured port). This controls traffic entering DuoRelay and does not change other VPN, system proxy or application network settings.
- Disabling the switch removes the dedicated port and restores existing inbound behavior.

## API

Example `POST /api/config/apply` request:

```json
{
  "mode": "load_balance",
  "node_ids": ["node-a", "node-b"],
  "load_balance": {
    "strategy": "round-robin",
    "url": "https://www.gstatic.com/generate_204",
    "interval": 300,
    "timeout": 5000,
    "lazy": true
  }
}
```

For chains use `mode: "chain"` and order `node_ids` from entry to exit. Omitting `kernel` automatically chooses one or both kernels; specifying it restricts the chain to that kernel. Omitting `load_balance` uses saved server defaults. `GET /api/config/current` returns mode, member order and the `chain` plan, without bridge passwords, controller secrets or internal paths.

`POST /api/config/chain/preview` accepts `{ "node_ids": ["entry ID", "exit ID"] }` and returns `supported`, `reason`, `cross_kernel`, `udp`, each hop's kernel and warnings. Preview checks protocol parameters and path capabilities; startup also validates all actual kernels. If the second kernel fails to start, the new instance is stopped and the previous complete configuration restored.

## Verification

Tests cover protocol capabilities, inbound/exit configuration matrices, preserving the prior running state on invalid parameters, dual-kernel rollback, bridge authentication, and dialog preview and ordering. Pinned-kernel configuration checks cover valid positions for 27 network/transport protocols; Rematch is verified as a routing adapter and rejected from the chain.

Real-traffic tests use a temporary database, local HTTP CONNECT service, two encrypted Shadowsocks services and a local UDP echo. They cover same-kernel chains, both cross-kernel directions, eight alternating cross-kernel hops, rejection of anonymous internal bridge authentication, and isolation of dedicated SOCKS and mixed ports. They demonstrate reproducible local paths only, not connectivity to every user's remote node; network authentication and remote exits remain dependent on user service configuration. TUN rules were generated and checked but the user's system network was not captured.

Reproduce with PowerShell:

```powershell
go test ./internal/configuration -run TestChainProtocolKernelChecks -count=1
$env:DUORELAY_CHAIN_INTEGRATION = '1'
go test ./internal/configuration -run 'TestRealChainTraffic|TestRealChainUDP' -count=1 -v
```
