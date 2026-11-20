# Transparent Proxy Acceptance Record

[简体中文](transparent-proxy-acceptance.md) | English

Date: 2026-11-03

Update on 2026-11-04: this run used an ordinary orca-linux container for permission failure protection, duplicate node names, actual kernel startup, separate SOCKS ports and configured-DNS requests. Windows/macOS full/lite were cross compiled. This container lacked TUN/NET_ADMIN, so privileged setup/cleanup from the earlier record below was not rerun. Real transparent traffic and DNS leak checks remain unverified. See the [November issue record](issue-resolution-2026-11.md).

The panel offers `off`, `redirect`, `tproxy` and `tun`. The legacy `enable_tun` field remains compatible and is normalized to `tun` when enabled. Settings show current environment support, prerequisites, DNS behavior and the active mode. Capability detection and active status do not verify real traffic or DNS leak protection. Windows TUN, Windows system proxy restoration and macOS TUN remain unverified on real systems.

## Mode Constraints

| System | redirect | tproxy | tun | DNS behavior |
|---|---|---|---|---|
| Linux | Requires root or the required network capabilities, `iproute2`, and nftables or iptables. TCP interception only. | Requires root or the required network capabilities, TPROXY, fwmark and policy routing support. | Requires readable/writable `/dev/net/tun` and `CAP_NET_ADMIN`/`CAP_NET_RAW`. | redirect does not intercept UDP DNS; system DNS queries may bypass the proxy. TPROXY also requires explicitly configured proxy DNS. TUN relies on kernel DNS interception configuration; actual behavior needs verification. |
| Windows | N/A. | N/A. | Requires administrator privileges, a Wintun driver and firewall confirmation; unverified on a real system. | Unverified on a real system. |
| macOS | N/A. | N/A. | Requires root privileges to create utun and manage routes; unverified on a real system. | Unverified on a real system. |

DuoRelay manages only its own nft table or `DUORELAY_*` iptables chains, TPROXY fwmark rules and policy routes. Stop and failure cleanup are idempotent: absent chains and routes count as already cleaned up. nft rules exclude loopback so the panel's health checks are not intercepted by its own TPROXY path.

## Acceptance Matrix

| Target | redirect | tproxy | tun | DNS leak check | Cleanup |
|---|---|---|---|---|---|
| Orca Debian 12 container | Setup/cleanup passed; real transparent requests unverified due to local bridge fixture loopback | Setup/cleanup passed; real transparent requests unverified due to fixture limits | sing-box and mihomo start/stop and route/TUN restoration passed | Unverified: no independent observer | nft, policy routes, interfaces and TUN state passed |
| Real Debian/Ubuntu Linux host | Unverified | Unverified | Unverified | Unverified | Unverified |
| Real Fedora/RHEL Linux host | Unverified | Unverified | Unverified | Unverified | Unverified |
| Real Arch Linux host | Unverified | Unverified | Unverified | Unverified | Unverified |
| Real Windows 11 target | N/A | N/A | Unverified: driver, administrator privileges and firewall confirmation required | Unverified | Unverified |
| Real macOS target | N/A | N/A | Unverified: real TUN test required | Unverified | Unverified |

When `scripts/acceptance-linux.sh` ran in the container without a test URL, it correctly reported all three modes as `UNVERIFIED`; capability detection was not mistaken for a successful real traffic test.
