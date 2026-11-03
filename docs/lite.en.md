# Full and Lite

[简体中文](lite.md) | English

Both variants share a repository and version number; builds without a tag remain full. They share IR, parsers, subscriptions, storage, the complete Vue panel, routing and transparent proxy rule management. No new data format or encryption was introduced, and no v2rayA code was copied.

| Capability | duorelay / full | duorelay-lite / lite |
|---|---|---|
| Go build tag | Default `!lite` | `lite` |
| Kernels | sing-box + mihomo | mihomo |
| Protocol/endpoint model | 28 | 24 supported by mihomo |
| Nodes, subscriptions, load balancing, panel | Existing functionality | All retained |
| Proxy chains | Native and cross-kernel SOCKS5 bridge | Native mihomo; cross-kernel bridge not compiled |
| Routing list/text editor | Retained | Retained |
| Transparent proxy | off / redirect / tproxy / tun, subject to system capabilities | Same; see acceptance report for verification scope |
| Validation and rollback | Existing flow | Shared apply transaction and real mihomo validation |
| Raw JSON/YAML at `/api/config/apply` | Rejected | Rejected |
| Database, key, encrypted subscriptions | Original format | Same; no new persisted fields |
| Windows kernel dependencies | sing-box.exe, mihomo.exe, libcronet.dll | mihomo.exe only |
| Settings → About | full, version, commit | lite, version, commit |

## Protocol Comparison

Support means the current model, parsing, generation and kernel capability boundaries; it does not imply end-to-end verification of every remote protocol, authentication method or platform. OpenVPN and other protocols still undergo real-kernel validation according to their parameters. Rematch is a routing adapter; network capabilities are not equivalent to a general remote proxy. MASQUE belongs to mihomo under the current ownership model.

| Protocol / capability | Full | Lite |
|---|---|---|
| Shadowsocks, VMess, VLESS, Trojan, Hysteria, Hysteria2, TUIC, AnyTLS, Snell, SOCKS5, HTTP / HTTPS, SSH, WireGuard, Tailscale | Both kernels | mihomo |
| OpenVPN | Both kernels, subject to existing parameter limits | mihomo, subject to the same limits |
| MASQUE, ShadowsocksR, Mieru, Sudoku, ShadowQUIC, TrustTunnel, Rematch, EasyTier, ZeroTier | mihomo | mihomo |
| ShadowTLS, Naive, Tor, OpenConnect | sing-box | Unsupported |

Lite retains imported unsupported nodes and subscription data; they remain searchable and removable. Start buttons, group selection and chain/load balancing selectors disable them and explain the limitation; the backend also rejects startup. The panel reads the backend variant before mounting and disables node startup if that fails; refresh to retry. Data remains available when switching back to full.

## Build and Package

For direct deployment, use [Docker full / lite](docker-deployment.en.md) or [Linux full / lite installation](linux-deployment.en.md#one-command-installation). Full remains the default; lite must be specified explicitly.

Use the versions required by `go.mod` and the existing frontend workflow. Run `cd web && npm ci`, then return to the repository root:

```bash
make build                       # Full, existing command
make build-lite                  # build/duorelay-lite
# With an existing production frontend:
go build -tags lite -o build/duorelay-lite ./cmd/duorelay

bash scripts/package-linux.sh 0.3.0 amd64
bash scripts/package-linux.sh 0.3.0 amd64 --lite
bash scripts/package-linux.sh 0.3.0 arm64 --kernels-dir /path/to/linux-arm64-kernels
bash scripts/package-linux.sh 0.3.0 arm64 --lite --kernels-dir /path/to/linux-arm64-kernels
# Panel package explicitly excluding kernels:
bash scripts/package-linux.sh 0.3.0 amd64 --lite --without-kernels
make package-linux-lite VERSION=0.3.0 ARCH=amd64
make package-windows-lite VERSION=0.3.0
```

Windows PowerShell:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 -Version 0.3.0
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 -Version 0.3.0 -Lite
```

Full filenames remain unchanged. Lite packages are `duorelay-lite-0.3.0-amd64.tar.gz`, `duorelay-lite-0.3.0-arm64.tar.gz` (Linux), and `duorelay-lite-0.3.0-amd64.zip` (Windows). The executable inside remains `duorelay` / `duorelay.exe` for installer and service compatibility. Linux and Windows lite staging directories are separate to prevent concurrent builds overwriting each other. Both variants use the same injected version parameters; this does not indicate a separate release.

Lite copies only mihomo, not sing-box, Cronet or Geo. Linux `--kernels-dir` must contain Linux binaries for the target architecture; the script does not convert architectures. Inspect the contents, ELF/PE architecture, versions and byte sizes of six packages containing kernels:

```bash
python3 scripts/measure-variants.py --version 0.3.0
```

## First Geo Download

On startup lite downloads missing `geoip.metadb` and `GeoSite.dat` to `-config-dir` (default `data/configs`) from mihomo's official sources:

- `https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb`
- `https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat`

The directory matches mihomo's `-d`; validation and runtime reuse the cache. Each download has a three-minute timeout and a 128 MiB limit; it writes a temporary file before renaming. HTTP errors, empty files, interruptions, size limits and write failures stop startup with the file, source and cache path, rather than silently using defaults. Existing nonempty cache files are reused; mihomo still validates their contents.

The first panel startup waits for downloads. For offline deployment, place the official files in the cache beforehand. Geo is not distributed in the package. Full's Geo behavior remains unchanged.

## Data Compatibility and Reinstallation

Stop the existing panel/service and back up the entire data directory, including database and matching `.key`, before switching. Replace the panel and appropriate kernels while retaining the same `-db`, `-config-dir` and service arguments, then start the other variant. On Linux continue using `scripts/install-linux.sh --package <extracted-directory>` and systemd; on Windows stop the process, replace application files and retain the data directory. Do not let both variants manage the same data directory concurrently.

Node, subscription and rule fields, the database key and subscription encryption are unchanged. A sing-box preference saved by full is projected as mihomo when read by lite without rewriting the original preference. Saving settings in lite explicitly saves its current mihomo preference, which full also recognizes.

If the last configuration used sing-box or a cross-kernel chain, lite retains its records and files, logs the limitation and waits for selection of a mihomo node. It does not automatically delete nodes, change protocols or weaken validation. Reapplying replaces the runtime record through the normal flow. After returning to full, sing-box can be selected again. Generated absolute file paths require reapplication when moving between machines; keep directories unchanged for a same-machine switch.

## Verification

```bash
go test ./...
go vet ./...
go test -tags lite ./...
go vet -tags lite ./...
cd web
npm run type-check
npm test
npm run build
```

`scripts/acceptance-lite.py --help` describes isolated container acceptance for bidirectional data, initial setup, official Geo download/cache, real local proxy and routing, load balancing, native chains and transparent proxy lifecycle. It requires Python 3, curl, iproute2, nftables and both built panels, and permits only an independent temporary directory inside Docker.

The container explicitly uses `--device /dev/net/tun:/dev/net/tun --cap-add NET_ADMIN --network bridge`; host networking is prohibited. The script uses only temporary accounts, nodes and container network namespaces, without modifying host system proxy or data. Transparent proxy rule lifecycle results do not establish real transparent traffic or DNS leak protection. See the [acceptance report](lite-acceptance.en.md) for measurements, baseline failures and unverified real-system items.
