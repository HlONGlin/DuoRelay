# Transparent Proxy Acceptance

Date: 2026-11-03

The panel exposes `off`, `redirect`, `tproxy`, and `tun`. The old `enable_tun` field remains accepted and is normalized to `tun` when enabled. Mode cards expose prerequisites, current support, DNS behavior, and failure reasons. Unsupported combinations are disabled by capability data.

## Mode contract

| System | redirect | tproxy | tun | DNS behavior |
|---|---|---|---|---|
| Linux | Requires root or the required network capabilities, `iproute2`, and nftables or iptables. TCP interception only. | Requires root or the required network capabilities, TPROXY, fwmark, and policy routing support. | Requires readable/writable `/dev/net/tun` and `CAP_NET_ADMIN`/`CAP_NET_RAW`. | redirect does not intercept UDP DNS and is marked as leak-prone unless DNS is explicitly sent to the proxy. TPROXY also requires an explicitly configured proxy DNS. TUN uses the kernel DNS hijack and generated DNS rules. |
| Windows | Disabled. | Disabled. | Targeted, but requires a real driver/admin/firewall run. | The panel must confirm driver and firewall prerequisites before claiming DNS takeover. |
| macOS | Disabled. | Disabled. | Not claimed until a real macOS run passes. | Not claimed. |

DuoRelay owns only its nft table or `DUORELAY_*` iptables chains, the TPROXY fwmark rule, and policy route. Stop and failure cleanup is idempotent: absent chains and routes are treated as already clean. The nft rules exclude loopback from TPROXY so the panel health endpoint cannot be intercepted by its own TPROXY path.

## Acceptance matrix

| Target | redirect | tproxy | tun | DNS leak check | Cleanup |
|---|---|---|---|---|---|
| Orca Debian 12 container | Setup/cleanup PASS; real transparent request UNVERIFIED due local bridge fixture loop | Setup/cleanup PASS; real transparent request UNVERIFIED due fixture limits | PASS for sing-box and mihomo integration start/stop and route/TUN restoration | UNVERIFIED: no independent observer | PASS for nft, policy route, interface, and TUN state checked |
| Linux Debian/Ubuntu real host | UNVERIFIED | UNVERIFIED | UNVERIFIED | UNVERIFIED | UNVERIFIED |
| Linux Fedora/RHEL real host | UNVERIFIED | UNVERIFIED | UNVERIFIED | UNVERIFIED | UNVERIFIED |
| Linux Arch real host | UNVERIFIED | UNVERIFIED | UNVERIFIED | UNVERIFIED | UNVERIFIED |
| Windows 11 real target | N/A | N/A | UNVERIFIED: driver/admin/firewall run required | UNVERIFIED | UNVERIFIED |
| macOS real target | N/A | N/A | UNVERIFIED: real TUN run required | UNVERIFIED | UNVERIFIED |

The container's `scripts/acceptance-linux.sh` was run without a test URL and correctly reported all three modes as `UNVERIFIED`; it did not convert capability detection into a real-flow pass.
