# Linux 一键部署兼容性验收

日期：2026-11-03。本次只改一键入口、服务账号创建及部署文档，保留 full/lite 选择、既有安装保护和原数据目录。未修改业务代码、数据库、密钥或订阅加密，也未创建发行标签。

## 变更

- 根据 `/etc/os-release` 的 `ID` / `ID_LIKE` 选择 apt-get、dnf/yum、pacman、zypper，补装基础命令、CA 和 libstdc++。已有 curl-minimal 时不强装 curl；Arch 不执行单独的数据库刷新。
- Go 不满足 `go.mod`、Node.js 不兼容或 npm 不可用时，下载官方临时工具链，校验 SHA-256 后使用，不替换系统版本。Go 使用模块要求的版本，Node.js 使用 22 分支的官方归档。
- 明确检查 glibc 2.28+、amd64/arm64、运行中的 systemd、root 和临时目录执行权限。下载有重试和超时，错误提示包含失败阶段。
- 发行版元数据在子 shell 读取，防止其中的 `VERSION` 等变量覆盖构建参数；服务组显式创建，nologin 位置动态探测。
- README 与 Linux 文档补充 root 无 sudo、wget 入口、TMPDIR 及系统范围。

## 隔离测试

| 项目 | 结果 | 边界 |
|---|---|---|
| `bash -n scripts/bootstrap-linux.sh` | PASS | Linux 容器内语法检查 |
| `bash -n scripts/install-linux.sh` | PASS | Linux 容器内语法检查 |
| `python3 scripts/test-bootstrap-linux.py` | PASS，45 项 | 临时命令、源码及归档，不是真实发行版安装 |
| Debian 12 apt-get 依赖安装命令 | PASS | orca-linux:local 临时容器内，实际访问软件源并安装依赖 |
| Fedora 43 dnf 依赖安装命令 | PASS | 官方 fedora:43 临时容器内，实际访问软件源并安装依赖 |
| 实际 `/tmp` noexec 挂载 | PASS | 明确拒绝执行并清理临时目录 |
| `TMPDIR=/var/tmp` | PASS | 绕过 noexec 后进入源码阶段；该用例主动令 Git 失败，确认错误阶段及清理 |

45 项分支测试覆盖默认 full、显式 full/lite、amd64/arm64、环境变量及参数优先级、帮助、无效参数、非 root、无 systemd、已有单元、musl、旧 glibc、旧/缺失 Go 与 Node、npm 故障、下载失败、摘要格式和内容错误、五种包管理器、补装失败、缺少 curl/libstdc++、未知发行版及 `ID_LIKE` 衍生系统。模拟 `os-release` 包含 `VERSION="12 (bookworm)"`，同时断言打包仍使用 `bootstrap`。

工具链测试使用真实 tar 解压及 SHA-256 计算，但网络和工具链可执行文件由临时测试替身提供，错误摘要的归档不得执行。测试 PATH 不回退到宿主包管理器或工具链。

可重复运行：

```bash
bash -n scripts/bootstrap-linux.sh
bash -n scripts/install-linux.sh
python3 scripts/test-bootstrap-linux.py
```

## Orca Linux 容器实装

使用本机 orca-linux 的 `orca-linux:local` 镜像（Debian 12），直接通过 Docker CLI 显式传入 `/dev/net/tun` 和 `NET_ADMIN`，使用 bridge 网络，没有发布端口、使用 host networking 或挂载宿主业务数据目录。仓库只读挂载到 `/workspace`，安装目录、账号和数据全部位于临时容器内。

**systemctl 使用测试替身，未启动真正的 systemd。** Git 从只读本地仓库克隆已有业务代码，并带入本次修改的安装器；临时替身将系统 Go/Node 版本报告为过旧，以触发真实工具链下载。其余下载、校验、npm、Go 编译、账号及文件安装均实际执行。

| 项目 | 结果 |
|---|---|
| Go 1.27.0 官方下载、SHA-256 校验及运行 | PASS |
| Node.js v22.23.4 / npm 10.9.9 官方下载、SHA-256 校验及运行 | PASS |
| `npm ci` 与 lite 前端生产构建 | PASS |
| 锁定 mihomo 下载与 lite Go 发布包构建 | PASS |
| `/opt/duorelay` 文件、服务账号、同名组、nologin 及数据目录安装 | PASS |
| lite 安装目录只有 mihomo，没有 sing-box | PASS |
| 临时构建目录在完成后清理 | PASS |
| 重复以 full 一键安装，已有安装在下载前被拒绝 | PASS |
| 以 duorelay 服务账号启动程序并请求内嵌前端首页 | PASS，HTTP 200 |
| systemd daemon-reload、enable/start 的真实效果 | **UNVERIFIED**，调用由测试替身接收 |

实装发现并修复了发行版 `VERSION` 覆盖打包版本导致的链接失败；隔离读取元数据并加入回归断言后，重新完成构建和安装。

## 未验证与范围

- **UNVERIFIED**：各发行版真实主机上的完整一键安装、systemd 启停及透明代理流量。
- **UNVERIFIED**：Rocky/Alma/RHEL 的 dnf/yum、Arch 的 pacman、openSUSE 的 zypper 实际软件源安装；本次只完成这些路径的隔离分支测试。Fedora 的 dnf 命令通过不等于所有 RPM 系统通过。
- **UNVERIFIED**：arm64 主机上的本轮临时工具链和完整安装。参数及下载归档选择通过隔离测试。
- 本轮没有改 Go/Vue 业务代码，未重复全部 Go/vet、前端类型与单测、全平台构建或 full/lite 数据迁移；既有记录见 [双版本部署验收](deployment-variants-acceptance.md) 和 [lite 验收](lite-acceptance.md)。本轮实际生产构建为 Linux amd64 lite。
- npm ci 仍报告既有依赖审计结果（15 项）；未在部署兼容性改动中自动升级前端依赖。

容器结果不能代替真实系统验收。此入口仍不支持 Alpine/musl、OpenWrt、glibc 低于 2.28、32 位系统或没有运行中 systemd 的普通容器。
