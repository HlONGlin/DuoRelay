# Linux Deployment

[简体中文](linux-deployment.md) | English

Linux is DuoRelay's primary target. This guide covers building and systemd installation. Real traffic and DNS leak acceptance across distributions remains incomplete; see [transparent proxy acceptance](transparent-proxy-acceptance.en.md). GNOME desktops can use the system proxy switch.

## One-Command Installation

For Linux amd64/arm64 with systemd and glibc 2.28 or newer; requires root and network access:

```bash
# Full (default): sing-box + mihomo
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | sudo bash -s -- --full

# Lite: mihomo only, complete panel
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | sudo bash -s -- --lite
```

Choose one command. Without an argument, full is installed; `DUORELAY_VARIANT=full|lite` also selects the variant, with command-line arguments taking precedence. The script clones source into a temporary directory, installs frontend dependencies, downloads the selected kernels, builds a package and invokes `scripts/install-linux.sh`. Temporary build files are removed after success or failure. The defaults are `/opt/duorelay` for the application, `/var/lib/duorelay` for data and `127.0.0.1:8080` for the panel. For remote access use SSH port forwarding, for example `ssh -L 8080:127.0.0.1:8080 user@server`, then open the local browser.

The script reads `ID` / `ID_LIKE` from `/etc/os-release` and uses apt-get, dnf/yum, pacman or zypper to install missing basic dependencies: Git, curl, CA certificates, tar, gzip, coreutils, findutils, awk/sed, account management tools and libstdc++ for Node.js. On Fedora/RHEL, an existing curl-minimal avoids a conflicting curl installation. On Arch the script uses the existing package database without a separate `pacman -Sy`; if it is stale, run a full system update with `sudo pacman -Syu` before retrying.

Go must meet `go.mod` (currently `1.27.0`); Node.js 18 or 20+ and npm are required. Compatible existing tools are reused. Otherwise the script downloads the required Go from `dl.google.com/go` and Node.js 22 LTS from `nodejs.org`, verifies SHA-256 against official HTTPS checksums, and extracts them. These temporary toolchains are added only to the build PATH, do not replace system Go or Node.js/npm, and are removed with the temporary directory. Downloads use HTTPS, retries and timeouts; download, verification or build failure stops installation and identifies the stage.

Lite uses a pinned, SHA-256-verified mihomo downloader for amd64/arm64, does not download sing-box or Cronet, and does not rewrite `kernels.lock`. It downloads a Geo cache on first start and reports network failures. Full retains the dual-kernel download flow and records local SHA-256 hashes without verifying pinned hashes. The Linux full package does not automatically include Cronet; Naive's extra dynamic library must be deployed separately.

### Supported Environment for the Installer

| System | Scope |
|---|---|
| Debian / Ubuntu and ordinary apt-get derivatives | Missing dependencies use apt-get when systemd, glibc 2.28+ and amd64/arm64 requirements are met. |
| 64-bit Raspberry Pi OS | apt-get on arm64 with glibc 2.28+ and running systemd; 32-bit ARM is unsupported. |
| Fedora / Rocky Linux / AlmaLinux / RHEL | Missing dependencies use dnf, falling back to yum. glibc 2.28+ is required; CentOS/RHEL 7 with glibc 2.17 is unsupported. |
| Arch / Manjaro | Missing dependencies use pacman; keep the distribution fully updated. |
| openSUSE / SLES | Missing dependencies use zypper; glibc 2.28+ remains required. |
| Alpine / OpenWrt and other minimal router systems | Not targeted by this installer; normal glibc + systemd assumptions do not apply. |
| Ordinary Docker containers, WSL/chroot/minimal VPS without running systemd | Rejected. Use [Docker deployment](docker-deployment.en.md) for containers; other environments must meet the systemd requirement. |
| 32-bit x86 / ARM, macOS, Windows | Unsupported by this Linux installer. |

These are script requirements, not per-distribution real-machine certification. Installation, systemd startup and transparent proxy traffic on real distribution hosts remain **UNVERIFIED**. No promise is made that every distribution version works out of the box; kernel runtime dependencies must also be met. The installer explicitly creates the service group and locates nologin rather than relying on distribution defaults or a fixed path. See [installer compatibility acceptance](linux-bootstrap-acceptance.en.md).

The panel and explicit proxy do not require TUN. TUN additionally requires `/dev/net/tun` and network administration privileges; redirect/tproxy also require appropriate nftables/iptables, policy routing tools and privileges. Desktop system proxy requires a GNOME session, which an SSH-only server does not provide.

The installer exits if `/opt/duorelay/duorelay` or a systemd unit already exists; it does not overwrite an installation or data. Back up data and use the manual deployment procedure below to upgrade. Environment variables select the Git repository and branch or tag:

```bash
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh -o /tmp/duorelay-bootstrap.sh
sudo env DUORELAY_REPO_URL=https://github.com/HlONGlin/DuoRelay.git DUORELAY_REF=master bash /tmp/duorelay-bootstrap.sh
# Select lite using the same entry point
sudo env DUORELAY_REF=master bash /tmp/duorelay-bootstrap.sh --lite
```

If `DUORELAY_REPO_URL` points to a private repository, configure Git credentials for root first. The `curl` entry point must itself be accessible; use an authorized download method for private repositories.

### Root, Missing curl and Restricted Temporary Directories

When already logged in as root, omit sudo (use `--full` for the other variant):

```bash
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | bash -s -- --lite
```

If curl is unavailable but wget exists, fetch the entry point with wget; the script installs curl for later downloads:

```bash
# Run as root; ordinary users can replace bash with sudo bash
wget -qO- https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | bash -s -- --lite
```

If neither is installed, install one through the system package manager. Resolve package locks, DNS/certificate errors, or connectivity to GitHub and official toolchain downloads before retrying. This entry point does not bypass TLS verification or silently switch to third-party mirrors.

The script checks whether executables can run in its temporary directory. If `/tmp` is mounted `noexec`, choose a writable executable directory with sufficient space, for example after downloading the entry point:

```bash
sudo env TMPDIR=/var/tmp bash /tmp/duorelay-bootstrap.sh --lite
```

`TMPDIR` must already exist and be writable by root. Source, toolchains and build artifacts require additional disk space. Check the reported stage on failure. If service installation has begun, inspect `systemctl status duorelay` and its logs and follow the manual deployment procedure; do not repeatedly overwrite an existing installation.

## Build a Release Package

```bash
cd web && npm ci && cd ..
./scripts/download-kernels.sh
./scripts/package-linux.sh 0.3.0 amd64
```

The packaging script supports `amd64` and `arm64`. The frontend is embedded in `duorelay`, so Node.js and `web/` are not needed in the package. Pass `--without-kernels` to build the panel without packaging kernels.

## Installation

System service installation requires root and systemd. The service unit limits network capabilities to `CAP_NET_ADMIN` and `CAP_NET_RAW` and permits access to `/dev/net/tun`; the device also needs appropriate permissions. The kernel manages TUN routing and DNS hijacking; DuoRelay manages its redirect/tproxy nftables/iptables chains and policy routes. Cleanup after stop, startup failure or rollback still needs verification on target systems.

```bash
sudo ./scripts/install-linux.sh --package ./dist/duorelay-linux-amd64-0.3.0
```

The packaging script creates both the directory above and a matching `.tar.gz`. The installer's `--package` accepts the extracted directory, not the archive. Defaults are `/opt/duorelay` for the program and `/var/lib/duorelay` for data.

```bash
sudo systemctl status duorelay
sudo journalctl -u duorelay -n 100 --no-pager
sudo systemctl restart duorelay

# Remove the program and service, retaining data by default
sudo ./scripts/uninstall-linux.sh
# Use --purge only after confirming data is no longer needed
```

`--purge` deletes the default installation and data directories; verify cleanup of custom paths separately.

## First Login

Open `http://127.0.0.1:8080/` after startup. If no account exists, the panel opens initial setup to create an administrator. Thereafter authentication is required for nodes, subscriptions, configuration and log APIs. Passwords are stored with bcrypt; database migration creates authentication tables automatically.

Desktop users who only need GNOME system proxy can install a user service:

```bash
./scripts/install-linux.sh --user-mode --package ./dist/duorelay-linux-amd64-0.3.0
```

System proxy depends on the current user's `gsettings` and GNOME D-Bus session. KDE, SSH-only servers and containers can configure applications' proxies manually or evaluate TUN after checking prerequisites and verification status. DuoRelay cannot change its parent process environment using environment variables.

## Acceptance

```bash
# Basic checks against the running panel; provide an authentication cookie as needed
DUORELAY_AUTH_COOKIE='session cookie' ./scripts/acceptance-linux.sh
```

The script requires `curl`, `jq` and an accessible panel API and reports results by mode. Missing real test conditions produce `UNVERIFIED`. Setting `DUORELAY_ACCEPTANCE_REAL=1` and `DUORELAY_TEST_URL` attempts configuration changes, kernel start/stop, requests and network state comparison. Use only an isolated test environment with temporary nodes and required privileges. A successful ordinary request alone does not prove proxy traversal or absence of DNS leaks; use independent exit and DNS observation.

Container tests require explicit `/dev/net/tun` and `NET_ADMIN`, without host networking. Recorded results are in [container acceptance](container-acceptance.en.md) and do not replace real-host acceptance.
