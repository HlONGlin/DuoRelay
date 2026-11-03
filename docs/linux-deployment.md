# Linux 部署

简体中文 | [English](linux-deployment.en.md)

DuoRelay 的主要运行目标是 Linux。本文介绍构建和 systemd 安装；透明代理尚未完成各发行版真实流量及 DNS 泄漏验收，详见 [透明代理验收](transparent-proxy-acceptance.md)。GNOME 桌面可以使用系统代理开关。

## 一键部署

适用于使用 systemd、glibc 2.28+ 的 Linux amd64/arm64 系统，需 root 权限及网络连接：

```bash
# full（默认）：sing-box + mihomo
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | sudo bash -s -- --full

# lite：仅 mihomo，完整面板
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | sudo bash -s -- --lite
```

两条命令二选一；不传参数时仍安装 full，也可通过 `DUORELAY_VARIANT=full|lite` 选择，命令行参数优先。脚本将源码克隆到临时目录，安装前端依赖、下载所选版本的内核、构建发布包，并调用 `scripts/install-linux.sh` 安装服务；完成或失败后删除临时构建目录。默认程序目录为 `/opt/duorelay`，数据目录为 `/var/lib/duorelay`，服务只监听 `127.0.0.1:8080`。远程访问可使用 SSH 端口转发，例如在自己的电脑执行 `ssh -L 8080:127.0.0.1:8080 user@server`，再访问本机浏览器。

脚本读取 `/etc/os-release` 的 `ID` / `ID_LIKE`，在缺少基础依赖时选择 apt-get、dnf/yum、pacman 或 zypper，补装 Git、curl、CA 证书、tar、gzip、coreutils、findutils、awk/sed、账号管理工具及 Node.js 所需的 libstdc++。Fedora/RHEL 已有 curl-minimal 时不强装 curl，避免包冲突。Arch 使用已有软件包数据库，不执行单独的 `pacman -Sy`；若数据库过旧导致安装失败，先手动执行完整系统更新 `sudo pacman -Syu` 再重试。

Go 须满足仓库 `go.mod` 要求（当前 `1.27.0`）；Node.js 支持 18 或 20+，且 npm 必须可用。脚本优先复用兼容的现有工具；缺失或不兼容时，从 `dl.google.com/go` 下载模块要求的 Go，从 `nodejs.org` 下载 Node.js 22 LTS，并对照各自官方 HTTPS 摘要完成 SHA-256 校验后解压。临时工具链仅加入本次构建的 PATH，不写入 `/usr/local/go` 或替换系统 Node.js/npm，随临时目录一起清理。下载使用 HTTPS、重试和超时；下载、校验或构建失败会终止并指出失败阶段，不继续安装。

lite 使用锁定版本和 SHA-256 校验的 mihomo 下载器，支持 amd64/arm64，不下载 sing-box 或 Cronet，也不重写 `kernels.lock`。首次启动另行下载 Geo 缓存，网络失败会明确报错。full 继续使用原双核下载流程，生成本地 SHA-256 记录，但不验证预先锁定的散列值；Linux full 包仍不自动补入 Cronet，Naive 的额外动态库部署需单独处理。

### 一键脚本的系统范围

| 系统类型 | 当前入口的适用范围 |
|---|---|
| Debian / Ubuntu、采用 apt-get 的常规衍生系统 | 满足 systemd、glibc 2.28+、amd64/arm64 前提时，缺依赖自动走 apt-get |
| 64 位 Raspberry Pi OS | arm64、glibc 2.28+ 且运行 systemd 时走 apt-get；32 位 ARM 不支持 |
| Fedora / Rocky Linux / AlmaLinux / RHEL 等 | 缺依赖优先 dnf，未提供 dnf 时回退 yum；仍需 glibc 2.28+，不支持 CentOS/RHEL 7 的 glibc 2.17 |
| Arch / Manjaro 等 | 缺依赖走 pacman；先保证发行版已完成正常的整机更新 |
| openSUSE / SLES 等 | 缺依赖走 zypper；仍需 glibc 2.28+ |
| Alpine / OpenWrt、其他精简路由器系统 | 不作为此一键入口支持目标；不能套用常规 glibc + systemd 部署前提 |
| Docker 普通容器、无运行中 systemd 的 WSL / chroot / 精简 VPS | 脚本拒绝；容器使用 [Docker 部署](docker-deployment.md)，其他环境先满足 systemd 前提 |
| 32 位 x86 / ARM、macOS、Windows | 不支持此 Linux 一键入口 |

以上是根据脚本约束列出的适用条件，不是逐发行版实机认证。各发行版真实主机的一键安装、systemd 服务启动及透明代理流量仍为 **UNVERIFIED**。没有承诺所有发行版版本开箱即用；还要满足所选内核的运行依赖。安装器显式创建服务组，并探测 nologin 的实际位置，不再依赖发行版默认建组行为或固定 `/usr/sbin/nologin` 路径。验证范围见 [一键部署兼容性验收](linux-bootstrap-acceptance.md)。

基础面板和显式代理不要求 TUN。启用 TUN 另需 `/dev/net/tun` 与网络管理权限；redirect / tproxy 还需相应 nftables/iptables、策略路由工具和权限。桌面系统代理依赖 GNOME 会话，纯 SSH 服务器不具备该条件。

已有 `/opt/duorelay/duorelay` 或 systemd 单元时脚本会退出，不覆盖现有安装或数据。请先备份数据并按下方手动部署流程处理升级。通过环境变量可选择 Git 仓库与分支或标签：

```bash
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh -o /tmp/duorelay-bootstrap.sh
sudo env DUORELAY_REPO_URL=https://github.com/HlONGlin/DuoRelay.git DUORELAY_REF=master bash /tmp/duorelay-bootstrap.sh
# 同一入口选择 lite
sudo env DUORELAY_REF=master bash /tmp/duorelay-bootstrap.sh --lite
```

`DUORELAY_REPO_URL` 指向私有仓库时，需要预先为 root 配置 Git 访问凭据。上面 `curl` 入口本身也必须能够读取该脚本；私有仓库应使用已授权的下载方式。

### root、缺少 curl 与临时目录受限

已经通过 root 登录的服务器不需要 sudo，使用下面命令（另一变体改为 `--full`）：

```bash
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | bash -s -- --lite
```

没有 curl 但有 wget 时，可先用 wget 获取入口；脚本会补装后续下载需要的 curl：

```bash
# root 执行；普通用户将 bash 换成 sudo bash
wget -qO- https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | bash -s -- --lite
```

若 curl、wget 都没有，先通过系统包管理器安装其中之一。遇到软件源锁、DNS/证书错误、GitHub 或官方工具链下载不可达，须先恢复对应网络或软件源再重试；此入口没有绕过 TLS 校验，也没有隐式切换第三方镜像。

脚本会检查临时目录能否执行程序。如果 `/tmp` 挂载了 `noexec`，请选择允许执行且空间充足的目录，例如先下载入口，再运行：

```bash
sudo env TMPDIR=/var/tmp bash /tmp/duorelay-bootstrap.sh --lite
```

`TMPDIR` 必须是已存在、root 可写的目录；源码、工具链及构建产物需要额外磁盘空间。失败时查看脚本报告的阶段；如果已经进入服务安装阶段，请检查 `systemctl status duorelay` 和日志，并按手动部署流程处理，不要反复覆盖已有安装。

## 构建发布包

```bash
cd web && npm ci && cd ..
./scripts/download-kernels.sh
./scripts/package-linux.sh 0.3.0 amd64
```

发布脚本支持 `amd64` 和 `arm64`。前端会在构建时嵌入 `duorelay`，发布包不需要额外携带 Node.js 或 `web/` 目录。如果只需要构建管理面板、不打包内核，可以传入 `--without-kernels`。

## 安装

系统服务安装需要 root 和 systemd。服务单元将网络 capabilities 限定为 `CAP_NET_ADMIN`、`CAP_NET_RAW`，并允许访问 `/dev/net/tun`；设备本身还需具备相应访问权限。TUN 的路由和 DNS hijack 由内核管理，redirect/tproxy 的自有 nftables/iptables 链与策略路由由 DuoRelay 管理。停止、启动失败或回滚后的清理仍需在目标系统验证。

```bash
sudo ./scripts/install-linux.sh --package ./dist/duorelay-linux-amd64-0.3.0
```

打包脚本同时生成上述目录及同名 `.tar.gz`；安装脚本的 `--package` 接受解压后的目录，不接受压缩包。默认程序放在 `/opt/duorelay`，数据放在 `/var/lib/duorelay`。关于页可检查 GitHub 正式版；默认系统服务的程序目录受保护，因此不会提供原地一键更新，请按本节停止服务、备份数据并重新安装目标版本。其他可写安装目录的一键更新条件见[应用更新说明](app-update.md)。

```bash
sudo systemctl status duorelay
sudo journalctl -u duorelay -n 100 --no-pager
sudo systemctl restart duorelay

# 卸载程序和服务，默认保留数据
sudo ./scripts/uninstall-linux.sh
# 确认不再需要数据后，才使用 --purge
```

`--purge` 会删除默认安装目录和数据目录；自定义路径的清理需自行确认。

## 首次登录

启动后访问 `http://127.0.0.1:8080/`。如果数据库中还没有账号，页面会自动进入首次设置，创建管理员账号；之后必须使用该账号登录才能访问节点、订阅、配置和日志接口。密码使用 bcrypt 保存，数据库迁移会自动创建认证表。

桌面用户如果只需要 GNOME 系统代理，可以安装用户服务：

```bash
./scripts/install-linux.sh --user-mode --package ./dist/duorelay-linux-amd64-0.3.0
```

系统代理依赖当前用户的 `gsettings` 和 GNOME D-Bus 会话。KDE、纯 SSH 服务器和容器环境可手动设置应用代理，或在确认前置条件及验收状态后评估 TUN；DuoRelay 无法通过环境变量修改其父进程环境。

## 验收

```bash
# 对运行中的面板做基础检查；按需提供认证 Cookie
DUORELAY_AUTH_COOKIE='会话 Cookie' ./scripts/acceptance-linux.sh
```

脚本需要 `curl`、`jq` 和可访问的面板 API，输出按模式区分的结果。缺少真实测试条件时输出 `UNVERIFIED`。设置 `DUORELAY_ACCEPTANCE_REAL=1` 和 `DUORELAY_TEST_URL` 会尝试修改配置、启停内核、发起请求并比较网络状态；请只在隔离测试环境中使用，并准备临时节点和必要权限。脚本中的普通请求成功本身不能证明流量经过代理，也不能证明 DNS 无泄漏；必须结合独立出口和 DNS 观察证据判断。

容器测试需要显式提供 `/dev/net/tun` 和 `NET_ADMIN`，不使用 host networking。已记录结果见 [容器验收](container-acceptance.md)，不能代替真实宿主机验收。
