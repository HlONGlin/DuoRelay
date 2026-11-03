# Full / Lite Deployment Entry Point Acceptance

[简体中文](deployment-variants-acceptance.md) | English

Date: 2026-11-03. The lite feature and showcase changes were fast-forwarded to `master`; Docker and Linux one-click deployment were then updated to select a variant. No release tag was created, and existing beta wording and published image tags were left unchanged.

## Changes

- Docker defaults to full and accepts `DUORELAY_VARIANT=lite`; lite uses the Go build tag and downloads only mihomo.
- `compose.yaml` remains full by default. `compose.lite.yaml` overrides the image and build arguments while sharing the service, UID and data volume; lite receives a seven-minute first-start health-check grace period.
- The downloader continues to verify binary SHA-256 hashes. Full Docker retains both kernels and Cronet on amd64. Lite supports amd64 and arm64; the mihomo arm64 hash was added and verified during this Docker build.
- The Linux installer defaults to full and accepts `--full`, `--lite` and `DUORELAY_VARIANT`. Lite uses the pinned single-kernel downloader and its own staging path. First-install protection, data directory and systemd service behavior are unchanged.
- The release workflow supports both variants. Full retains the existing version tag; lite uses the `-lite` suffix and rejects old release tags that do not contain lite code.
- The README lists Docker and Linux one-click commands for both variants and documents distribution prerequisites.

## Results

| Check | Result |
|---|---|
| `go test ./...` and `go test -tags lite ./...` | PASS, including downloader tests; unchanged packages reused Go's test cache |
| `go vet` for both targets | PASS |
| Frontend type check and 53 existing tests | PASS |
| Docker production frontend build | PASS |
| Full and lite linux/amd64 Docker images | PASS, built and started |
| Lite linux/arm64 Docker image | PASS, built; frontend and runtime commands executed under emulation |
| arm64 panel `-h` and mihomo `-v` | PASS under Docker Desktop emulation; not arm64 hardware acceptance |
| Both Compose configurations | PASS; services and volumes match and lite arguments are correct |
| Image contents | PASS; full has sing-box, mihomo and `libcronet.so`; lite has mihomo only |
| Compose panel and authentication | PASS; production HTML returned successfully, API reported the compiled variant, UID/GID were 10001 |
| Shared-volume full → lite → full migration | PASS; temporary account logged in, nodes and rules matched, sing-box preference was retained |
| Lite first-start Geo download | PASS; both cache files were non-empty and reusable |
| `bash -n scripts/bootstrap-linux.sh` | PASS |
| `python3 scripts/test-bootstrap-linux.py` | PASS, 14 isolated branch tests |
| Downloader unit tests | PASS for full/amd64, lite/amd64 and lite/arm64 requests, file composition and hashes; invalid targets and missing pinned hashes were rejected |

Installer tests substitute Git, Go, npm and systemctl and use temporary source trees and directories to check arguments, package paths and installer calls. They cover defaults and explicit variants, both CPU architectures, environment variables and precedence, help, invalid values, 32-bit rejection, non-root execution, stopped systemd and outdated Node.js. Simulated branches are not real system installations.

Compose was tested in isolated project `duorelay-deployment-acceptance`, with a temporary volume and account. A test override mapped `/dev/net/tun`, added `NET_ADMIN`, and used bridge networking. No host port was published, no real data was mounted, and host proxy or firewall settings were not changed. Only the test project and temporary volume were removed afterward. This work accepted deployment and data switching; it did not claim successful transparent traffic forwarding.

## Unverified

- One-click installation and systemd acceptance on real Debian, Ubuntu, Fedora, Rocky/AlmaLinux, Arch and openSUSE hosts.
- Actual apt dependency installation across distribution versions; repositories may not provide sufficiently recent Go or Node.js.
- Real arm64 host behavior for the panel, proxy traffic and installation; this run built an image and emulated command entry points only.
- Publishing through GitHub Actions and GHCR; no release tag was created and no release was triggered.
- Real-system, transparent-forwarding and DNS items listed as unverified in the [lite acceptance report](lite-acceptance.en.md) remain unverified. Two baseline failures in opt-in full HTTP chain tests were outside this deployment change.

The supported Linux distribution matrix describes installer prerequisites; see [Linux deployment](linux-deployment.en.md#one-click-installation). Real installation acceptance can be run on a suitable test host. This report does not claim out-of-the-box support for every distribution.
