# Container Acceptance Record

[简体中文](container-acceptance.md) | English

Date: 2026-11-03

This Linux run used the repository's `docker-ssh` Orca recipe. Based on `duorelay-orca-ssh:base`, it starts a Debian 12 container, uses ordinary bridge networking, mounts the repository at `/workspace`, explicitly adds `--cap-add=NET_ADMIN` and maps `/dev/net/tun`; host networking was not used. `/dev/net/tun` was writable inside the container, and effective capabilities included `CAP_NET_ADMIN` and `CAP_NET_RAW`.

The installed Orca CLI has no standalone `orca-linux` command. The supported invocation was confirmed using local recipe help and `vm recipe doctor docker-ssh --provision --json`; the recipe reported `/workspace` as its project root.

| Check | Container result | Evidence |
|---|---|---|
| Build Go backend and embedded frontend | PASS | In-container `go build ./cmd/duorelay`, `npm ci`, `npm run type-check`, 45 frontend tests and `npm run build` all passed. |
| Panel at `127.0.0.1:8080` | PASS | Embedded frontend returned HTTP 200. |
| First setup | PASS | Registered and logged in through the panel API with a temporary `temp` account. |
| Kernel startup | PASS | Created a temporary local HTTP node; sing-box validated, started and reported healthy. |
| Real proxy request | PASS | Temporary HTTP proxy recorded a CONNECT request and sing-box returned the marked response. |
| Routing list/text validation and import | PASS | Validated and imported five numbered rules; after clearing, exported and re-imported content matched byte for byte. |
| Invalid-rule isolation | PASS | Port `70000` and malformed syntax returned line-level errors. Failed kernel selection left the existing instance and `current_config` valid. |
| Kernel validation rollback | PASS | A temporary unsupported SSH node failed validation before replacement; the original sing-box process kept running. |
| TUN kernel startup/cleanup | PASS | `DUORELAY_TUN_INTEGRATION=1 go test ./internal/configuration -run TestLinuxTUNCaptureAndCleanup` passed for sing-box and mihomo, including route/rule and TUN-address cleanup. |
| redirect setup/cleanup | PASS | Panel created its own nft table and removed it on stop; interface, route and firewall snapshots were cleanly restored. |
| Real transparent redirect request | UNVERIFIED | A bridge sidecar could reach the container, but the local fixture redirected the request to loopback; this does not count as real transparent-traffic success. |
| tproxy setup/cleanup | PASS | Panel created the nft TPROXY chain, `0xd0` fwmark rule and table 100 local route; all were removed on stop. |
| Real transparent tproxy request | UNVERIFIED | Rules were installed and cleaned up, but the isolated bridge fixture cannot provide trustworthy end-to-end TPROXY traffic evidence. |
| DNS interception/leak test | UNVERIFIED | No independent DNS observer was available; no DNS leak-test success is claimed. |
| Windows system proxy | Out of scope | Must be tested on a real Windows target while saving and restoring the current user's Internet Settings. |
| macOS TUN | Out of scope | Must be tested on a real macOS target; container results cannot substitute. |

redirect and TPROXY rule lifecycles are reported separately from real traffic. The current fixture creates a loop and cannot reliably observe the original destination, so real transparent traffic remains unverified. These results are tracked separately from the Windows/macOS real-system matrix.
