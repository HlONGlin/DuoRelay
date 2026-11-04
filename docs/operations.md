# 日常运维与故障恢复

适用于 full 与 lite。两种构建使用相同的数据格式和管理 API；lite 只运行 mihomo。协议能力以 `kernels.lock` 锁定的版本及节点参数为准。

## 透明代理权限

普通系统服务以 `duorelay` 用户运行，适合面板和显式 HTTP/SOCKS5 代理。Linux 透明代理使用单独的安装选项：

```bash
sudo ./scripts/install-linux.sh --package /path/to/unpacked-package --transparent --backup
sudo systemctl restart duorelay
sudo journalctl -u duorelay -n 100 --no-pager
```

`--transparent` 安装 root 服务覆盖，限制网络能力为 `CAP_NET_ADMIN`、`CAP_NET_RAW` 和 `CAP_NET_BIND_SERVICE`，并为 iptables 使用 `/run/duorelay/xtables.lock`。需要安装 iproute2；redirect/TPROXY 还需要可用的 nftables 或 iptables。TUN 需要可读写的 `/dev/net/tun`。不要设置空的 `CapabilityBoundingSet=` 来尝试授权；容器里的 root 也可能缺少网络能力。

Linux 权限预检会实际读取进程有效 capabilities、检查设备，并探测防火墙访问。保存或应用不满足条件的模式会返回原因；应用时先预检，再停止旧内核。Windows 检测当前进程是否提升为管理员，macOS 检测 root；这两种系统只提供 TUN，仍需要目标系统的驱动、系统网络和防火墙验收。程序不会自行弹出提权窗口。

重新运行安装器且不带 `--transparent`，可切回普通服务。安装器维护 `StateDirectory=duorelay` 和数据属主；自定义 `--data-dir` 使用服务覆盖传入对应路径。程序和内核采用同目录临时文件与原子替换，避免升级正在运行的 ELF 时发生 `Text file busy`。普通服务的程序目录由 root 保护；透明服务覆盖允许 root 写程序目录，因此一键更新还取决于匹配的发布资产和更新检查结果。

`allow_lan` 控制显式代理监听是否接受局域网连接，关闭它不会关闭 TUN 的本机接管。Linux redirect/TPROXY 的防火墙链作用于 PREROUTING；本机整机接管应使用 TUN。redirect 不接管 UDP DNS；TPROXY 也不能仅凭启动成功宣称 DNS 已接管。启用路由前应确认目标主机的网络环境，验收矩阵见[透明代理验收](transparent-proxy-acceptance.md)。

## DNS 与模式状态

新安装默认业务 DNS 为 `https://1.1.1.1/dns-query`，经 `PROXY`，bootstrap 为 `223.5.5.5`，用于解析代理及 DNS 服务器域名。已有自定义配置继续保留。bootstrap 必须填写 IP，业务 DNS 支持 IP、UDP、TCP、TLS 和 HTTPS；直连网络下可将 DNS 出口设为 `DIRECT`。选择 `PROXY` 时必须有可用代理节点，避免隐式回退系统 DNS。

sing-box TUN 先 sniff 再 hijack-dns，随后处理端口绑定和用户路由。设置页分别展示所选模式和当前生效模式：保存设置不会立即改变现有连接；需要重新应用节点。内核未运行、启动恢复失败或权限不满足时，不会仅凭数据库旧状态显示已经生效。

`dns_fake_ip` 是可选功能，并非完整的大陆域名数据库策略。显式代理模式不会自动修改 `/etc/resolv.conf`；不要把系统 DNS 改为 `127.0.0.1`，除非已经部署并确认有本地 DNS 监听服务。

控制器默认绑定自动分配的 loopback 端口，以减少与其他内核服务冲突。高级配置保留显式控制器地址。面板 HTTP 监听在启动时直接绑定；端口占用会在初始化前退出并给出地址，systemd 在五分钟内最多启动三次，每次失败间隔十五秒。

## 日志清理

日志页默认显示最近 **100 条不同消息**。相同来源的重复错误合并显示次数；时间、颜色码及连接编号不影响合并，远端 IP 和端口差异仍保留。WebSocket 每 250ms 聚合发送，界面最多保留 100 行，避免错误刷屏持续占用显示框。

在日志页设置保留天数和清理间隔：保留天数为 1–365，间隔为 0–720 小时，0 关闭周期清理。默认保留 7 天、每 24 小时清理；策略存入数据库并在重启后恢复。启动时也清理过期磁盘记录。单个内核磁盘日志达到 16MiB 后裁剪为约 8MiB 尾部，最多 10000 条；关闭周期清理仍保留该容量保护。

“清空日志”同时清空所选内核的内存及磁盘记录，运行中的内核可继续产生新日志。历史生成配置只保留最近十份及当前引用的配置；清理只匹配程序生成的 UUID 文件，不删除手写配置和 Geo 数据。

## 备份与恢复

对正在运行的 WAL 数据库，使用 SQLite 快照备份，避免直接复制一个仍在写入的 `.db`：

```bash
sudo /opt/duorelay/duorelay --db /var/lib/duorelay/duorelay.db \
  --backup /var/lib/duorelay/backups/manual-20261104
```

目标必须是尚不存在的目录，父目录须预先存在且可写。结果包含 `duorelay.db`、`duorelay.db.key` 和带 SHA-256 的 `manifest.json`；目录为 0700，文件为 0600。数据库与配套密钥必须一起保管，否则加密订阅无法恢复。快照保留账号、节点、订阅、设置和日志策略，但清除原机器的运行配置引用；恢复后需在面板重新应用节点，避免自动使用旧机器的绝对路径和系统代理快照。原数据库不会被修改。

恢复前停止服务，将备份的数据库和 `.key` 成对放回目标数据目录，移开原数据库及其 `-wal`/`-shm` 文件，然后按服务运行用户恢复文件属主和权限。不要将备份的 `.db` 与其他时间点的 WAL 混用。启动后登录并重新应用节点。备份不包含磁盘日志、内核、Geo 缓存及历史配置；需要完整灾备时同时保存这些文件。

安装器的 `--backup` 在升级前备份已有数据库，并安装每日运行的 `duorelay-backup.timer`：

```bash
sudo systemctl list-timers duorelay-backup.timer
sudo systemctl start duorelay-backup.service
sudo journalctl -u duorelay-backup.service -n 50 --no-pager
```

自动备份位于数据目录的 `backups/`。备份不会自动删除；请按磁盘容量和恢复需求另行制定保留策略。用户服务可直接使用备份 CLI，并由用户自己的计划任务调度。

## 重置管理员密码

在服务运行用户或 root 的终端执行，新密码从标准输入读取，不放在命令行参数中：

```bash
read -rsp 'New password: ' DUORELAY_NEW_PASSWORD; echo
printf '%s\n' "$DUORELAY_NEW_PASSWORD" | sudo /opt/duorelay/duorelay \
  --db /var/lib/duorelay/duorelay.db --reset-password --username admin
unset DUORELAY_NEW_PASSWORD
```

密码长度为 8–72 字节，账号必须已经存在。重置会撤销该账号全部会话，运行中的面板无需重启；用户应重新登录。备份和重置都要求原数据库及对应的 32 字节 `.key` 已存在，不会在密钥丢失时静默创建替代密钥。

## HTTPS 访问

默认面板只监听 `127.0.0.1:8080`。远程管理可使用 SSH 隧道；公网域名使用 HTTPS 反向代理，并为 DuoRelay 启动参数增加 `--cookie-secure`，然后重启服务。在 systemd 中覆盖 `ExecStart` 时需先写一个空的 `ExecStart=`，再填写包含原数据库、配置和内核目录的完整命令。

Caddy 示例：

```caddyfile
panel.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

已有证书的 Nginx 示例，`map` 放在 `http` 段中：

```nginx
map $http_upgrade $duorelay_connection {
    default upgrade;
    '' close;
}
server {
    listen 443 ssl;
    server_name panel.example.com;
    ssl_certificate /etc/ssl/panel/fullchain.pem;
    ssl_certificate_key /etc/ssl/panel/privkey.pem;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $duorelay_connection;
        proxy_read_timeout 3600s;
    }
}
```

证书签发、域名解析和防火墙由部署环境管理。通过纯 HTTP 访问时启用 Secure Cookie 会导致浏览器不发送会话 Cookie，应先完成 HTTPS。
