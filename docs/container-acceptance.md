# Container Acceptance

Date: 2026-11-03

The Linux run used the repository's `docker-ssh` Orca recipe. The recipe started a Debian 12 container from `duorelay-orca-ssh:base` with ordinary bridge networking, `/workspace` mounted from the repository, `--cap-add=NET_ADMIN`, and an explicit `/dev/net/tun` device mapping. Host networking was not used. Inside the container, `/dev/net/tun` was a writable character device and the effective capability set contained `CAP_NET_ADMIN` and `CAP_NET_RAW`.

The installed Orca CLI has no standalone `orca-linux` command. The local recipe help and `vm recipe doctor docker-ssh --provision --json` were used to confirm the supported invocation; the recipe returned `/workspace` as its project root.

| Check | Container result | Evidence |
|---|---|---|
| Build Go backend and embedded frontend | PASS | `go build ./cmd/duorelay`; `npm ci`, `npm run type-check`, 45 frontend tests, and `npm run build` passed in the container. |
| Panel at `127.0.0.1:8080` | PASS | Embedded frontend returned HTTP 200. |
| First setup | PASS | Temporary `temp` account registered and logged in using the panel API. |
| Kernel start | PASS | Temporary local HTTP node was created; sing-box was validated, started, and reported healthy. |
| Real proxy request | PASS | A temporary local HTTP proxy recorded a CONNECT request and returned the marker response through sing-box. |
| Routing list/text validation and import | PASS | Five rules validated/imported with line numbers; export and re-import were compared byte-for-byte after a clear. |
| Invalid rule isolation | PASS | Port `70000` and malformed lines returned separate line errors; an invalid kernel selection failed while the previous instance and `current_config` remained active. |
| Kernel check rollback | PASS | An unsupported temporary SSH node failed before replacement; the previous sing-box state stayed running. |
| TUN kernel start/cleanup | PASS | `DUORELAY_TUN_INTEGRATION=1 go test ./internal/configuration -run TestLinuxTUNCaptureAndCleanup` passed for sing-box and mihomo, including route/rule and TUN address cleanup. |
| redirect setup/cleanup | PASS | Panel apply created the owned nft table and stop removed it; interface, route, and firewall snapshots were clean afterward. |
| redirect real transparent request | UNVERIFIED | A bridge sidecar reached the container, but the test request was redirected into a loop in the current local fixture; this is not counted as a real transparent-flow pass. |
| tproxy setup/cleanup | PASS | Panel apply created the nft TPROXY chain, fwmark rule `0xd0`, and table 100 local route; stop removed all three. |
| tproxy real transparent request | UNVERIFIED | The container verified rule installation and cleanup, but no request is claimed because the isolated bridge fixture did not provide a trustworthy end-to-end TPROXY flow. |
| DNS interception/leak check | UNVERIFIED | The container did not provide an independent DNS observer; no DNS leak pass is claimed. |
| Windows system proxy | OUT OF SCOPE | Must run on a real Windows target with the current-user Internet Settings snapshot. |
| macOS TUN | OUT OF SCOPE | Must run on a real macOS target; container results cannot stand in for it. |

The redirect and TPROXY rows distinguish rule lifecycle verification from real traffic verification. The latter remain unverified until a fixture can observe the original destination without creating a loop. These results are separate from the real Windows and macOS matrix.
