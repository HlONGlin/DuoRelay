# full 与 lite

同一仓库、同一版本号，默认不带 tag 的构建仍为 full。共用 IR、解析器、订阅、存储、完整 Vue 面板、路由与透明代理规则管理；没有引入新数据格式或加密方式，没有复制 v2rayA 代码。

| 能力 | duorelay / full | duorelay-lite / lite |
|---|---|---|
| Go build tag | 默认 `!lite` | `lite` |
| 内核 | sing-box + mihomo | mihomo |
| 协议/端点模型 | 28 项 | mihomo 的 24 项 |
| 节点、订阅、负载均衡、面板 | 保持现状 | 全部保留 |
| 链式代理 | 原生与跨核 SOCKS5 桥接 | mihomo 原生，不编译跨核桥接 |
| 路由列表/文本编辑 | 保留 | 保留 |
| 透明代理 | off / redirect / tproxy / tun，受系统能力限制 | 相同，实际验证范围见验收报告 |
| 校验与回滚 | 原有流程 | 共享应用事务与 mihomo 真核校验 |
| `/api/config/apply` 原始 JSON/YAML | 拒绝 | 拒绝 |
| 数据库、密钥、加密订阅 | 原格式 | 相同，无新增持久化字段 |
| Windows 内核依赖 | sing-box.exe、mihomo.exe、libcronet.dll | 仅 mihomo.exe |
| 设置 → 关于 | full、版本、提交 | lite、版本、提交 |

## 协议对照

“支持”指现有模型、解析、生成与内核能力边界，不表示所有远端协议、认证方式或平台均已端到端验证。OpenVPN 等协议仍按具体参数进行真核校验。Rematch 是路由适配器，组网能力不等于通用远端代理。MASQUE 按当前所有权模型归属 mihomo。

| 协议/能力 | full 内核 | lite |
|---|---|---|
| Shadowsocks | 两核 | mihomo |
| VMess | 两核 | mihomo |
| VLESS | 两核 | mihomo |
| Trojan | 两核 | mihomo |
| Hysteria | 两核 | mihomo |
| Hysteria2 | 两核 | mihomo |
| TUIC | 两核 | mihomo |
| AnyTLS | 两核 | mihomo |
| Snell | 两核 | mihomo |
| SOCKS5 | 两核 | mihomo |
| HTTP / HTTPS | 两核 | mihomo |
| SSH | 两核 | mihomo |
| WireGuard | 两核 | mihomo |
| OpenVPN | 两核，参数限制不变 | mihomo，参数限制不变 |
| MASQUE | mihomo | mihomo |
| Tailscale | 两核 | mihomo |
| ShadowsocksR | mihomo | mihomo |
| Mieru | mihomo | mihomo |
| Sudoku | mihomo | mihomo |
| ShadowQUIC | mihomo | mihomo |
| TrustTunnel | mihomo | mihomo |
| Rematch | mihomo | mihomo |
| EasyTier | mihomo | mihomo |
| ZeroTier | mihomo | mihomo |
| ShadowTLS | sing-box | 精简版不支持 |
| Naive | sing-box | 精简版不支持 |
| Tor | sing-box | 精简版不支持 |
| OpenConnect | sing-box | 精简版不支持 |

lite 保留已导入的不支持节点与订阅内容，列表可检索、删除。启动按钮、组合勾选、链式/负载均衡选择器禁用对应节点并显示“精简版不支持”；后端同样拒绝启动。面板挂载前读取后端变体，失败时禁用节点启动，刷新重试。切回 full 后数据仍在。

## 构建与打包

工具版本沿用 `go.mod` 和现有前端流程。先 `cd web && npm ci`，再返回仓库根目录：

```bash
make build                       # full，原命令
make build-lite                  # build/duorelay-lite
# 已有生产前端时：
go build -tags lite -o build/duorelay-lite ./cmd/duorelay

bash scripts/package-linux.sh 0.3.0 amd64
bash scripts/package-linux.sh 0.3.0 amd64 --lite
bash scripts/package-linux.sh 0.3.0 arm64 --kernels-dir /path/to/linux-arm64-kernels
bash scripts/package-linux.sh 0.3.0 arm64 --lite --kernels-dir /path/to/linux-arm64-kernels
# 明确不含内核的面板包：
bash scripts/package-linux.sh 0.3.0 amd64 --lite --without-kernels
make package-linux-lite VERSION=0.3.0 ARCH=amd64
make package-windows-lite VERSION=0.3.0
```

Windows PowerShell：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 -Version 0.3.0
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 -Version 0.3.0 -Lite
```

full 文件名保持原状。lite 为 `duorelay-lite-0.3.0-amd64.tar.gz`、`duorelay-lite-0.3.0-arm64.tar.gz`（Linux），`duorelay-lite-0.3.0-amd64.zip`（Windows）。包内面板仍叫 `duorelay` / `duorelay.exe`，兼容原有安装器与服务单元。Linux、Windows 的 lite 暂存目录分开，避免同时打包相互覆盖。两种变体共用版本注入参数，不代表另行发版。

lite 仅复制 mihomo，不复制 sing-box、Cronet 或 Geo。Linux `--kernels-dir` 必须包含目标架构的 Linux 二进制；脚本不会转换内核架构。检查六个含内核包的组成、ELF/PE 架构、版本和实际字节数：

```bash
python3 scripts/measure-variants.py --version 0.3.0
```

## Geo 首次下载

lite 启动时向 `-config-dir`（默认 `data/configs`）下载缺失的 `geoip.metadb`、`GeoSite.dat`，使用 mihomo 自身的官方发布来源：

- `https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb`
- `https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat`

目录与 mihomo 的 `-d` 一致，校验和运行复用缓存。每个文件超时 3 分钟，大小上限 128 MiB，先写临时文件再改名。HTTP 错误、空文件、下载中断、超限或写入失败均报错退出，说明文件、来源和缓存路径，不静默使用缺省数据。已有非空缓存直接复用；实际内容仍由 mihomo 校验。

首次启动面板需要等待下载；离线部署可预先把上述官方文件放入缓存目录。Geo 不随包分发。full 的 Geo 行为保持原状。

## 数据互通与覆盖安装

切换前停止现有面板/服务，备份整个数据目录（含数据库与配套 `.key`）。以相同 `-db`、`-config-dir` 和服务参数替换面板及对应内核，再启动另一变体。Linux 继续使用原 `scripts/install-linux.sh --package <解包目录>` 与 systemd 单元；Windows 停止进程后覆盖应用文件，保留数据目录。不要让两个变体同时管理同一数据目录。

未更改节点、订阅、规则字段，未触碰数据库密钥或订阅加解密。full 保存的 sing-box 首选项在 lite 读取时仅投影为 mihomo，不因读取而改写原偏好；主动在 lite 保存设置会保存当前 mihomo 偏好，full 也认识该值。

上次配置为 sing-box 或跨核链时，lite 保留记录和文件，启动日志说明不支持，等待选择 mihomo 节点。它不会自动删除节点、变更协议或降低校验。重新应用后按原流程替换运行记录。返回 full 可重新选择 sing-box。生成文件的绝对路径跨机器搬迁时需重新应用，同机切换保持相同目录。

## 验证

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

`scripts/acceptance-lite.py --help` 提供隔离容器验收：双向数据、首次设置、官方 Geo 下载/缓存、真实本地代理与路由、负载均衡、原生链式、透明代理生命周期。需要 Python 3、curl、iproute2、nftables 和已构建的两种面板；仅允许 Docker 内独立临时目录。

容器显式使用 `--device /dev/net/tun:/dev/net/tun --cap-add NET_ADMIN --network bridge`，禁止 host networking。脚本只使用临时账号、节点与容器网络空间，不操作宿主机系统代理或数据。透明代理规则生命周期结果不等于真实透明流量、DNS 防泄漏验收。实测、基线失败及真实系统 UNVERIFIED 项见 [验收报告](lite-acceptance.md)。
