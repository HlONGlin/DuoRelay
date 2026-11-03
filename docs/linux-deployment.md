# Linux 部署

DuoRelay 的主要运行目标是 Linux。本文介绍构建和 systemd 安装；透明代理尚未完成各发行版真实流量及 DNS 泄漏验收，详见 [透明代理验收](transparent-proxy-acceptance.md)。GNOME 桌面可以使用系统代理开关。

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
