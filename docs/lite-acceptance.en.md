# Lite Implementation and Acceptance Report

[简体中文](lite-acceptance.md) | English

Date: 2026-11-03. Branch: `feat/lite-variant`. Baseline `master`: `0570cd74237030249f0b2bf15ecbbe3a89e2a789`. This is a local acceptance build for the existing `0.3.0` version, not a new release; existing beta.1 wording was not changed. Archives were built before commit, so build-info records the baseline HEAD. The complete implementation is represented by the single lite commit on this branch.

## Implementation

- Added `lite` / `!lite` build boundaries for kernel capabilities, process differences, generator registration and chain implementation. Full retains the dual-kernel generators and cross-kernel bridge; lite does not compile the sing-box generator or cross-kernel SOCKS5 bridge.
- Both builds embed the same production panel. It reads the backend variant and disables activation, group selection and selectors for protocols lite cannot run. Existing nodes remain visible with an explanation. The about section identifies full/lite, version and commit.
- Lite single-node, load-balancing and native-chain flows reuse the shared apply transaction, retaining real-kernel validation, transparent-proxy preparation, system-proxy restoration and rollback. Full-only chain rendering moved into `chain_full.go`.
- On first launch, lite downloads and caches Geo data from MetaCubeX's official release channel and exits with a clear error if it fails. Full does not force a pre-download. IR, parsers, node/subscription storage, database keys, subscription encryption and transparent-proxy rules were not changed.
- Linux packaging adds `--lite` and `--kernels-dir`; Windows adds `-Lite`; Make targets were added and existing scripts retained. Linux and Windows lite staging directories are separate.

## Static Checks and Builds

| Check | Windows workspace | Isolated Linux container |
|---|---|---|
| `go test ./...` | PASS | PASS |
| `go vet ./...` | PASS | PASS |
| `go test -tags lite ./...` | PASS | PASS |
| `go vet -tags lite ./...` | PASS | PASS |
| `npm ci`, `npm run type-check` | PASS | PASS |
| `npm test` | PASS, 12 files / 53 tests | PASS, 12 files / 53 tests |
| `npm run build` | PASS | PASS, production frontend step in `make build-lite` |
| `make build-lite VERSION=0.3.0` | Make not run; Windows packaging script was run | PASS |
| Linux amd64 full/lite packages | PASS, cross-build and archive inspection | amd64 panel and kernels actually ran: PASS |
| Linux arm64 full/lite packages | PASS, cross-build and ELF/contents inspection | arm64 execution UNVERIFIED |
| Windows amd64 full/lite packages | PASS, PE architecture and contents inspected | N/A |
| Lite dependency graph | `go list -tags lite -deps ./cmd/duorelay` excludes sing-box generator | Same source target |
| Lite kernel/generator registries | PASS, mihomo only | PASS, mihomo only |
| Unsupported protocols unselectable with explanation | PASS, real Vue component/group selector tests; 24/28 capability assertions | Same 53 frontend tests PASS |
| Applying raw configuration boundary | Existing full tests PASS; new lite tests PASS | HTTP API returns 400: PASS |

Toolchains: Windows Go 1.27.0; container Go 1.27.3, Node 22 and Debian 12. Kernels use the pinned sing-box 1.14.4 and mihomo 1.19.33; arm64 binaries came from the same official releases. The Windows race toolchain was not installed, so `go test -race` is not counted as passing. Full-only assertions are skipped explicitly in lite and continue to run in full. sing-box packages and generator tests depending on them are excluded by build tags. Shared subscription compatibility tests run for both targets; only tests spanning both generators moved into a full-only file.

## Isolated Container Runtime Tests

The local wrapper `C:/Users/honglin/.local/bin/orca-linux.ps1` and its Dockerfile were inspected first. It uses `orca-linux:local`, workspace mounts and a shared `/root` volume, but has no device/capability passthrough interface. A separate one-shot container from the same base image added Go, iproute2 and nftables/iptables. It did not reuse a personal home volume, inject a host proxy or mount a real host database.

The container used `--device /dev/net/tun:/dev/net/tun --cap-add NET_ADMIN --network bridge`. `docker inspect` confirmed device mapping, `CAP_NET_ADMIN` and bridge networking; `test -c /dev/net/tun` passed. A read-only artifact mount copied the source snapshot to `/tmp/duorelay-src`, where the panel was built. There was no host networking, published panel port or change to host proxy/firewall settings. Fixture interfaces, addresses, rules and data existed only in the container.

| Case | Result and evidence |
|---|---|
| Lite panel and first setup | PASS; embedded HTML HTTP 200, temporary admin created in a new isolated DB and logged in, default kernel mihomo |
| First Geo download and cache | PASS; both official files downloaded into an empty cache; second start left mtimes unchanged |
| Geo download failure | PASS; unit tests cover HTTP 403/404 and empty responses, report errors and leave no success cache or temp file |
| mihomo startup | PASS; panel API reported running/mihomo and runtime directory contained only mihomo |
| At least one real proxy request | PASS; curl → panel-managed mihomo → temporary HTTP upstream → temporary origin; response and upstream counters checked |
| Routing | PASS; PROXY, DIRECT and REJECT verified in one configuration; DIRECT did not increment upstream counter and REJECT failed |
| Load balancing | PASS; panel applied two nodes and a real request was made |
| Native chain | PASS; panel applied two hops; two temporary upstream forwards observed, with no cross-kernel helper |
| Invalid kernel replacement | PASS; HTTP 400, `current_config` ID unchanged and prior proxy remained usable |
| Real-kernel validation failure | PASS; mihomo rejected an invalid Shadowsocks cipher with 422 / `KERNEL_CHECK_FAILED`; old config ID and proxy remained usable |
| Startup rollback | PASS; lite single-node/load-balance/chain unit tests injected startup failure and restored paths and persisted state; this is not represented as a remote-failure test |
| Full → lite → full | PASS; API content matched for six nodes (four unsupported by lite), one encrypted subscription and three rules; DB records and key hash unchanged; full sing-box preference retained |
| Lite-created data → full | PASS; full reopened two nodes, one subscription and three rules created by lite and compared each item |
| redirect, single and chain | PASS; panel started, owned nft rules existed, and rules/routes were fully restored after stop |
| tproxy, single and chain | PASS; nft TPROXY and policy-route lifecycle verified and snapshots restored after stop |
| tun, single and chain | PASS; panel applied config and kernel started; rules/routes restored on stop |
| Full TUN setup/cleanup | PASS; existing Linux integration tests passed for both sing-box and mihomo, including IPv4/IPv6 addresses and route cleanup |
| Lite TUN setup/cleanup | PASS; same integration tests for mihomo, including address and route cleanup |
| Real kernel crash recovery | PASS; existing tests killed/restarted full's two kernels and lite's single kernel, checked health and rejected invalid config |
| Lite protocol-chain matrix | PASS; existing fixtures placed each mihomo-supported protocol at entry/exit and passed real-kernel config checks; Rematch was excluded as a non-network adapter |
| Full real configuration application | PASS; existing test enabled with `DUORELAY_BACKEND_KERNEL_INTEGRATION=1` |

The single-node fixture retained the existing PROXY manual-selection group, whose initial member is DIRECT. It explicitly selected `entry` through the temporary instance's Clash control API before observing the proxy path; no raw configuration or default behavior was changed. Dummy-interface addresses `10.203.0.2/3/4` avoided mistaking the kernel's inherent loopback bypass for proxy success.

Geo cache measured for this run (archives contain zero Geo bytes):

| File | Downloaded bytes | SHA256 |
|---|---:|---|
| GeoSite.dat | 4,356,022 | `3fee9b84efce1ef142974a18b48b9b12d2cf8ccb8017dd4ec16e3906bcb7da69` |
| geoip.metadb | 8,424,898 | `f74ad728d3d3e1a7ca51886a147a18d82406733f3bc0db8cf9117f66de437239` |
| Total | 12,780,920 | Official latest data changes; these are this run's downloads |

## Existing Failure Found During Additional Acceptance

An additional run of `DUORELAY_CHAIN_INTEGRATION=1 go test -v ./internal/configuration -run '^TestReal' -count=1` found `TestRealChainTraffic/sing-box` and `mihomo-to-sing-box` failing with connection refused on the mixed port. The same two failures reproduced in the original `master` baseline `0570cd7` using the same binaries and libraries in the same container. The other mihomo, sing-box-to-mihomo, alternating eight-hop and scoped bidirectional HTTP cases passed. All four full native/bidirectional cross-kernel UDP combinations passed. This opt-in group is not reported as fully passing; full chain behavior was not changed in the lite task.

Initial full default tests in the container failed because Cronet was absent. After adding the repository's existing `libcronet.so` and setting container `LD_LIBRARY_PATH`, full/lite default tests and vet passed on rerun. This was an acceptance-environment fix; Cronet was not added to lite packages. The first two proxy fixtures did not traverse the upstream because of loopback bypass and the group's default DIRECT member, so they were not counted. The final fixture used distinct addresses and explicitly selected a group member, then verified upstream counters.

## Release Archive Measurements

All sizes are bytes. The panel includes the embedded production frontend.

| Platform | Variant | Archive | Panel | mihomo | sing-box | Cronet | Geo | Other files |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| linux/amd64 | full | 62,342,500 | 21,745,491 | 62,017,662 | 81,416,421 | 0 | 0 | 14,042 |
| linux/amd64 | lite | 34,663,941 | 21,516,546 | 62,017,662 | 0 | 0 | 0 | 14,042 |
| linux/arm64 | full | 56,908,686 | 20,672,027 | 57,475,198 | 75,956,453 | 0 | 0 | 14,042 |
| linux/arm64 | lite | 31,832,481 | 20,489,042 | 57,475,198 | 0 | 0 | 0 | 14,042 |
| windows/amd64 | full | 66,558,506 | 21,812,224 | 61,247,488 | 82,074,624 | 9,529,344 | 0 | 9,126 |
| windows/amd64 | lite | 34,336,092 | 21,584,384 | 61,247,488 | 0 | 0 | 0 | 9,126 |

The frontend is embedded in the panel and is not counted as a separate web directory. Archive metadata supplied the archive sizes and component sizes; ELF/PE architecture and build-info platform/version/variant were checked. Lite contains no sing-box, `libcronet.dll` or Geo files. Full retains its prior kernel composition (both Linux kernels; Windows kernels plus Cronet). The existing Linux full script does not package `libcronet.so`, and this behavior was not changed; deployment of that extra library for Naive is not counted as verified.

For context, Docker Registry manifest reads for v2rayA returned these compressed layer sums (not uncompressed image sizes and excluding protocol/manifest transfer overhead):

| Image | Compressed layer bytes | Decimal MB |
|---|---:|---:|
| `mzz2017/v2raya@sha256:255c166e59b3b889d00b35db37c7e4b12a7a9570e188cbbb8bfa1b9ad6b203d8` (linux/amd64) | 42,061,749 | 42.061749 |
| `mzz2017/v2raya@sha256:ae77951d6f3ef496f1b637872ae6ea581e19fb693bd17f032a7ec9f8b0fe17ca` (linux/arm64) | 40,297,837 | 40.297837 |

This is only an approximate size comparison: DuoRelay values are compressed release archives, while v2rayA values are compressed image layers with different base systems and Geo contents. No v2rayA code was copied and no panel features were removed to reach a target size.

## Unverified Items and Scope Limits

| Item | Status | Reason |
|---|---|---|
| Windows real-machine system proxy, TUN and service in-place upgrade | UNVERIFIED | This machine ran Go/frontend tests, packaging and file inspection only; its real proxy/firewall was untouched |
| Linux real-host systemd/root/desktop behavior | UNVERIFIED | Tested in an isolated container only; real `install-linux.sh` service replacement was not run |
| macOS build, real TUN and system behavior | UNVERIFIED | No macOS environment |
| Linux arm64 execution | UNVERIFIED | Cross-build and archive ELF checks only |
| End-to-end transparent forwarding through redirect/TPROXY/TUN | UNVERIFIED | Rule/interface/route lifecycles were checked; explicit proxy requests do not replace transparent-traffic tests |
| DNS interception and leak testing | UNVERIFIED | No independent DNS observer; no leak-free claim is made |
| Real remote connectivity for all 24 lite protocols | UNVERIFIED | Kernel config matrix is not external-service acceptance; real traffic used only a temporary HTTP upstream |
| Public internet proxy exit | UNVERIFIED | Used local temporary upstream and origin, not a user's real node |
| Real public-network outage/timeout injection for Geo downloads | UNVERIFIED | HTTP and empty-file failures are unit-tested; no host network fault was injected |

## Files and Evidence

The change covered 59 files, including `Makefile`, `README.md`, `cmd/duorelay/main.go`, `docs/README.md`, lite docs and package measurements, API/configuration/kernel/variant code and tests, packaging scripts, and the node-management/settings frontend. The source report's full path-by-path list is retained in the Chinese version.

Original logs and archives are under ignored `build/lite-work/` and `dist/`. Key evidence includes `full-go-test.log`, `lite-go-test.log`, `web-test.log`, `container-final.log`, `container-go-test-{full,lite}.log`, `container-lite-protocol-matrix.log`, `container-tun-{full,lite}.log`, `container-kernel-{full,lite}.log`, `container-chain-full.log`, `container-chain-baseline.log` and `package-measurements.json`. The adjacent `lite-package-sizes.json` contains per-file archive composition and can be regenerated with `scripts/measure-variants.py`. The acceptance container was removed; its image remains locally for inspection and is not a release image.
