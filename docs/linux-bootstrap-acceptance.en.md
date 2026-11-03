# Linux One-Click Installation Compatibility Acceptance

[简体中文](linux-bootstrap-acceptance.md) | English

Date: 2026-11-03. This change covers the one-click entry point, service-account creation and deployment documentation. Full/lite selection, existing-installation protection and the data directory remain in place. Business code, databases, keys and subscription encryption were not changed, and no release tag was created.

## Changes

- Selects apt-get, dnf/yum, pacman or zypper using `/etc/os-release` fields `ID` and `ID_LIKE`, and installs required commands, CA certificates and libstdc++. It does not force-install curl when `curl-minimal` is present; Arch does not run a separate database refresh.
- If Go does not meet `go.mod`, Node.js is incompatible or npm is unavailable, downloads a temporary official toolchain, verifies SHA-256 and uses it without replacing system versions. Go follows the module version; Node.js uses the official Node 22 archive.
- Checks for glibc 2.28+, amd64/arm64, running systemd, root privileges and an executable temporary directory. Downloads use retries and timeouts, and errors identify the failing stage.
- Reads distribution metadata in a subshell so variables such as `VERSION` cannot overwrite build arguments. Creates the service group explicitly and detects the nologin path dynamically.
- Documents root-without-sudo, wget entry points, `TMPDIR` and supported systems in the README and Linux guide.

## Isolated Tests

| Check | Result | Scope |
|---|---|---|
| `bash -n scripts/bootstrap-linux.sh` | PASS | Syntax check in a Linux container |
| `bash -n scripts/install-linux.sh` | PASS | Syntax check in a Linux container |
| `python3 scripts/test-bootstrap-linux.py` | PASS, 45 cases | Temporary commands, source and archives; not a real distribution installation |
| Debian 12 apt dependency commands | PASS | Accessed repositories and installed dependencies in a temporary `orca-linux:local` container |
| Fedora 43 dnf dependency commands | PASS | Accessed repositories and installed dependencies in a temporary official `fedora:43` container |
| Actual `/tmp` noexec mount | PASS | Execution was rejected and the temporary directory cleaned up |
| `TMPDIR=/var/tmp` | PASS | Passed noexec and reached the source stage; Git was deliberately failed to verify error reporting and cleanup |

The 45 branch tests cover variant defaults and explicit choices, amd64/arm64, environment variables and argument precedence, help and invalid options, non-root execution, stopped systemd, existing units, musl, old glibc, missing or outdated Go/Node.js, npm failure, download and checksum errors, five package managers, install failures, missing curl/libstdc++, unknown distributions and `ID_LIKE` derivatives. A simulated `os-release` containing `VERSION="12 (bookworm)"` also verifies packaging still uses `bootstrap`.

Toolchain tests perform real tar extraction and SHA-256 calculation. Network responses and executable toolchains are provided by temporary test doubles; archives with invalid hashes must not execute. The test PATH does not fall back to host package managers or toolchains.

```bash
bash -n scripts/bootstrap-linux.sh
bash -n scripts/install-linux.sh
python3 scripts/test-bootstrap-linux.py
```

## Orca Linux Container Installation

Used the local `orca-linux:local` image (Debian 12), with `/dev/net/tun` and `NET_ADMIN` explicitly passed through Docker CLI. The container used bridge networking, had no published ports or host networking, and did not mount host business data. The repository was mounted read-only at `/workspace`; installation files, account and data remained inside the temporary container.

**systemctl was a test double; real systemd was not started.** Git cloned existing project code from a read-only local repository with the installer changes included. Temporary doubles reported outdated system Go/Node versions to trigger actual toolchain downloads. Other download, checksum, npm, Go build, account and file installation steps ran for real.

| Check | Result |
|---|---|
| Official Go 1.27.0 download, SHA-256 verification and execution | PASS |
| Official Node.js v22.23.4 / npm 10.9.9 download, verification and execution | PASS |
| `npm ci` and lite production frontend build | PASS |
| Pinned mihomo download and lite Go release build | PASS |
| `/opt/duorelay`, service account/group, nologin and data directory installation | PASS |
| Lite install directory contains mihomo and no sing-box | PASS |
| Temporary build directory cleanup | PASS |
| Repeated full install refused before download when an installation already existed | PASS |
| Service account started the program and requested the embedded home page | PASS, HTTP 200 |
| Actual systemd daemon-reload, enable and start | **UNVERIFIED**; calls were handled by a test double |

The run found and fixed a packaging link failure caused by a distribution `VERSION` variable overriding the package version. Metadata reading was isolated and a regression assertion added before rebuilding and reinstalling.

## Unverified Scope

- **UNVERIFIED:** Full one-click installation, systemd lifecycle and transparent traffic on real hosts across distributions.
- **UNVERIFIED:** Real package-repository installation through dnf/yum on Rocky/Alma/RHEL, pacman on Arch and zypper on openSUSE. These paths received isolated branch tests only. Fedora dnf success does not prove every RPM distribution works.
- **UNVERIFIED:** Temporary toolchain and full installation on arm64 hardware. Argument handling and archive selection passed isolated tests.
- This run did not change Go/Vue business code or repeat the full Go/vet suite, frontend type checks and tests, all-platform builds or full/lite migration. See [deployment variant acceptance](deployment-variants-acceptance.en.md) and [lite acceptance](lite-acceptance.en.md). The production build actually run here was Linux amd64 lite.
- `npm ci` continues to report existing dependency audit findings (15 items); frontend dependencies were not upgraded as part of deployment compatibility work.

Container results do not replace real-system acceptance. This entry point does not support Alpine/musl, OpenWrt, glibc below 2.28, 32-bit systems or ordinary containers without running systemd.
