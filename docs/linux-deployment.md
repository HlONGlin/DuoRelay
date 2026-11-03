# Linux 部署

DuoRelay 的主要运行目标是 Linux。本文介绍构建和 systemd 安装；透明代理尚未完成各发行版真实流量及 DNS 泄漏验收，详见 [透明代理验收](transparent-proxy-acceptance.md)。GNOME 桌面可以使用系统代理开关。

## 一键部署

适用于使用 systemd 的 Linux amd64/arm64 系统，需 root 权限及网络连接：

```bash
# full（默认）：sing-box + mihomo
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | sudo bash -s -- --full

# lite：仅 mihomo，完整面板
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh | sudo bash -s -- --lite
```

两条命令二选一；不传参数时仍安装 full，也可通过 `DUORELAY_VARIANT=full|lite` 选择，命令行参数优先。脚本将源码克隆到临时目录，安装前端依赖、下载所选版本的内核、构建发布包，并调用 `scripts/install-linux.sh` 安装服务；完成或失败后删除临时构建目录。默认程序目录为 `/opt/duorelay`，数据目录为 `/var/lib/duorelay`，服务只监听 `127.0.0.1:8080`。远程访问可使用 SSH 端口转发，例如在自己的电脑执行 `ssh -L 8080:127.0.0.1:8080 user@server`，再访问本机浏览器。

构建需要 Git、curl、Go、Node.js/npm、tar、gzip 和 coreutils。缺少依赖时，脚本在检测到 `apt-get` 的系统上尝试安装；其他系统需手动安装。Go 须满足仓库 `go.mod` 要求（当前 `1.27.0`），或者能够自动下载所需工具链；Node.js 支持 18 或 20+，推荐 22。发行版软件源中的 Go/Node.js 可能过旧，自动安装软件包不等于工具链版本一定满足。脚本在构建前检查 Go 能否加载模块及 Node.js 主版本。

lite 使用锁定版本和 SHA-256 校验的 mihomo 下载器，支持 amd64/arm64，不下载 sing-box 或 Cronet，也不重写 `kernels.lock`。首次启动另行下载 Geo 缓存，网络失败会明确报错。full 继续使用原双核下载流程，生成本地 SHA-256 记录，但不验证预先锁定的散列值；Linux full 包仍不自动补入 Cronet，Naive 的额外动态库部署需单独处理。

### 一键脚本的系统范围

| 系统类型 | 当前入口的适用范围 |
|---|---|
| Debian / Ubuntu、采用 apt-get 的常规衍生系统 | systemd 正常运行、64 位 amd64/arm64、root 和网络可用时可使用；缺依赖时尝试 apt-get 补装 |
| 64 位 Raspberry Pi OS | arm64 且运行 systemd 时满足同类前提；32 位 ARM 不支持 |
| Fedora / Rocky Linux / AlmaLinux / RHEL / Arch / openSUSE 等 | 按脚本条件可用，但必须自行安装兼容工具链与基础命令；没有 dnf/yum/pacman/zypper 自动安装分支 |
| Alpine / OpenWrt、其他精简路由器系统 | 不作为此一键入口支持目标；不能套用常规 glibc + systemd 部署前提 |
| Docker 普通容器、无运行中 systemd 的 WSL / chroot / 精简 VPS | 脚本拒绝；容器使用 [Docker 部署](docker-deployment.md)，其他环境先满足 systemd 前提 |
| 32 位 x86 / ARM、macOS、Windows | 不支持此 Linux 一键入口 |

以上是根据脚本约束列出的适用条件，不是逐发行版实机认证。各发行版真实主机的一键安装、systemd 服务启动及透明代理流量仍为 **UNVERIFIED**。没有承诺具体发行版版本开箱即用；还要满足所选内核的运行依赖、`useradd` 等系统命令及 `/usr/sbin/nologin` 路径。

基础面板和显式代理不要求 TUN。启用 TUN 另需 `/dev/net/tun` 与网络管理权限；redirect / tproxy 还需相应 nftables/iptables、策略路由工具和权限。桌面系统代理依赖 GNOME 会话，纯 SSH 服务器不具备该条件。

已有 `/opt/duorelay/duorelay` 或 systemd 单元时脚本会退出，不覆盖现有安装或数据。请先备份数据并按下方手动部署流程处理升级。通过环境变量可选择 Git 仓库与分支或标签：

```bash
curl -fsSL https://raw.githubusercontent.com/HlONGlin/DuoRelay/master/scripts/bootstrap-linux.sh -o /tmp/duorelay-bootstrap.sh
sudo env DUORELAY_REPO_URL=https://github.com/HlONGlin/DuoRelay.git DUORELAY_REF=master bash /tmp/duorelay-bootstrap.sh
# 同一入口选择 lite
sudo env DUORELAY_REF=master bash /tmp/duorelay-bootstrap.sh --lite
```

`DUORELAY_REPO_URL` 指向私有仓库时，需要预先为 root 配置 Git 访问凭据。上面 `curl` 入口本身也必须能够读取该脚本；私有仓库应使用已授权的下载方式。

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

打包脚本同时生成上述目录及同名 `.tar.gz`；安装脚本的 `--package` 接受解压后的目录，不接受压缩包。默认程序放在 `/opt/duorelay`，数据放在 `/var/lib/duorelay`。

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
