# Linux 部署

DuoRelay 的主要运行目标是 Linux。服务器和无桌面环境优先使用 TUN；GNOME 桌面可以使用系统代理开关。

## 构建发布包

```bash
./scripts/download-kernels.sh
./scripts/package-linux.sh 0.3.0 amd64
```

发布脚本支持 `amd64` 和 `arm64`。前端会在构建时嵌入 `duorelay`，发布包不需要额外携带 Node.js 或 `web/` 目录。如果只需要构建管理面板、不打包内核，可以传入 `--without-kernels`。

## 安装

带透明接管的系统服务需要 root 安装。服务账户只获得 `CAP_NET_ADMIN`、`CAP_NET_RAW` 和 `/dev/net/tun` 访问权限；TUN 的路由和 DNS hijack 由内核管理，redirect/tproxy 的自有 nftables/iptables 链与策略路由由 DuoRelay 管理。停止、启动失败或回滚时必须清理这些状态；是否完成真实接管以 [透明代理验收记录](transparent-proxy-acceptance.md) 为准。

```bash
sudo ./scripts/install-linux.sh --package ./dist/duorelay-linux-amd64-0.3.0
sudo ./scripts/acceptance-linux.sh
```

数据默认放在 `/var/lib/duorelay`。停止服务不会删除数据库；卸载时增加 `--purge` 才会删除数据。

## 首次登录

启动后访问 `http://127.0.0.1:8080/`。如果数据库中还没有账号，页面会自动进入首次设置，创建管理员账号；之后必须使用该账号登录才能访问节点、订阅、配置和日志接口。密码使用 bcrypt 保存，数据库迁移会自动创建认证表。

桌面用户如果只需要 GNOME 系统代理，可以安装用户服务：

```bash
./scripts/install-linux.sh --user-mode --package ./dist/duorelay-linux-amd64-0.3.0
```

系统代理依赖当前用户的 `gsettings` 和 GNOME D-Bus 会话。KDE、纯 SSH 服务器和容器环境应使用 TUN 或手动设置应用代理；应用进程无法被 DuoRelay 通过环境变量修改其父进程环境。

## 验收

```bash
./scripts/acceptance-linux.sh
DUORELAY_ACCEPTANCE_REAL=1 DUORELAY_TEST_URL=https://example.com/ ./scripts/acceptance-linux.sh
```

脚本按发行版和模式输出矩阵；只有在提供临时节点、真实测试 URL、权限和 `DUORELAY_ACCEPTANCE_REAL=1` 后，才会执行开启、真实请求、关闭和状态恢复检查。缺少条件会明确输出 `UNVERIFIED`。Docker/容器测试需要显式提供 `/dev/net/tun` 和 `NET_ADMIN`；不要使用 host networking，避免测试路由影响宿主机。启用 TUN 后，“仅 SOCKS5 端口使用代理”会自动关闭，因为该模式会让其他入口直连。严格路由会阻止无法按当前规则转发的流量回落直连；本地网络、组播、门户认证和应用自带 VPN 仍受各自路由规则与操作系统限制。
