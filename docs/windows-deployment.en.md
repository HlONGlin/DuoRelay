# Windows Deployment and Migration

[简体中文](windows-deployment.md) | English

This guide covers packaging and migration. Full real-system acceptance of Windows system proxy restoration and TUN remains outstanding; see the [transparent proxy acceptance matrix](transparent-proxy-acceptance.en.md).

## Build a Release Package

Run from the repository root:

```powershell
.\scripts\package-windows.ps1 -Version 0.3.0
```

Install the Go version required by `go.mod`, Node.js/npm, and the Windows kernels under `kernels/`. Run `npm ci` in `web/` first. The script builds the frontend and Go backend and creates:

```text
dist/duorelay-windows-amd64-0.3.0.zip
```

The package contains:

- `duorelay.exe`
- `kernels/sing-box.exe`
- `kernels/mihomo.exe`
- `kernels/libcronet.dll`
- `kernels.lock`
- `build-info.json`

Frontend assets are embedded in the executable. The first launch creates `data/`. Start the program from the package root so relative paths resolve to `kernels` and `data`.

## Migrate Configuration

Stop the old version and back up the entire `data/` directory before migrating:

```powershell
Copy-Item .\data .\backup\data-$(Get-Date -Format yyyyMMdd-HHmmss) -Recurse
```

Keep at least:

- `data/duorelay.db`
- `data/duorelay.db.key`
- `data/configs/`

On startup, the new version migrates the SQLite schema and brings old settings and runtime state up to the current `config_version`. Subscription URLs are encrypted with the database key. Migrate the database and `.key` file together; saved subscription URLs cannot be recovered without the key.

## Windows System Proxy

When Windows system proxy is enabled in settings, DuoRelay is designed to save the current user's Internet Settings and switch to the local mixed proxy port after the kernel starts successfully. It restores the previous settings when the kernel stops, startup fails or changes are rolled back. These operations still require acceptance testing on real Windows systems.

Only the current user is changed; global policies do not need to be modified. Windows TUN is created by sing-box or mihomo and may require administrator privileges, driver permissions and firewall confirmation on first use.

## Live Logs

The backend provides:

```text
GET /ws/logs?kernel=sing-box
```

After connecting, the client receives a `snapshot` of history followed by live `log` messages and periodic `ping` heartbeats. The `level`, `source` and `search` parameters match the HTTP log API. The kernel status page prefers WebSocket and falls back to HTTP refresh when the connection closes.

## Startup Example

```powershell
.\duorelay.exe -listen 127.0.0.1:8080 -db data\duorelay.db -kernels kernels -config-dir data\configs
```
