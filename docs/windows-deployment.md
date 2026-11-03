# Windows 部署与迁移

本文提供打包与迁移步骤；Windows 系统代理恢复及 TUN 的完整真实系统验收仍待完成，见 [透明代理验收矩阵](transparent-proxy-acceptance.md)。

## 生成发布包

在仓库根目录运行：

```powershell
.\scripts\package-windows.ps1 -Version 0.3.0
```

需要满足 `go.mod` 要求的 Go、Node.js/npm，以及 `kernels/` 下的 Windows 内核文件。先在 `web/` 执行 `npm ci`；脚本会构建前端和 Go 后端，并生成：

```text
dist/duorelay-windows-amd64-0.3.0.zip
```

发布包包含：

- `duorelay.exe`
- `kernels/sing-box.exe`
- `kernels/mihomo.exe`
- `kernels/libcronet.dll`
- `kernels.lock`
- `build-info.json`

前端资源嵌入可执行文件。运行时首次启动会创建 `data/` 目录；建议从发布包根目录启动程序，以便相对路径能够找到 `kernels` 和 `data`。

## 配置迁移

迁移前先停止旧版本，并备份整个 `data/` 目录：

```powershell
Copy-Item .\data .\backup\data-$(Get-Date -Format yyyyMMdd-HHmmss) -Recurse
```

至少需要保留：

- `data/duorelay.db`
- `data/duorelay.db.key`
- `data/configs/`

启动新版本时会自动迁移 SQLite schema，并将旧设置和运行状态补齐到当前 `config_version`。订阅 URL 使用数据库密钥加密，数据库和 `.key` 文件必须一起迁移；丢失密钥后无法恢复已保存的订阅地址。

## Windows 系统代理

在设置中启用“Windows 系统代理”后，设计上 DuoRelay 会保存当前用户的 Internet Settings，并在内核成功启动后切换到本地混合代理端口；停止内核、启动失败或回滚时恢复原设置。这些操作仍需要真实 Windows 环境验收。

该功能只修改当前用户，不要求修改全局策略。Windows TUN 仍由 sing-box 或 mihomo 创建，首次使用可能需要管理员权限、驱动权限和防火墙确认。

## 实时日志

后端提供：

```text
GET /ws/logs?kernel=sing-box
```

连接后先收到 `snapshot` 历史快照，随后收到 `log` 实时日志和周期性的 `ping` 心跳。`level`、`source`、`search` 参数与 HTTP 日志接口一致。前端内核状态页优先使用 WebSocket，连接断开时会回退到 HTTP 刷新。

## 启动示例

```powershell
.\duorelay.exe -listen 127.0.0.1:8080 -db data\duorelay.db -kernels kernels -config-dir data\configs
```
