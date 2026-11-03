# 容器验收记录

Date: 2026-11-03

本次 Linux 运行使用仓库的 `docker-ssh` Orca recipe。recipe 基于 `duorelay-orca-ssh:base` 启动 Debian 12 容器，使用普通 bridge networking，将仓库挂载到 `/workspace`，显式添加 `--cap-add=NET_ADMIN` 并映射 `/dev/net/tun`，未使用 host networking。容器内 `/dev/net/tun` 可写，有效 capability 包含 `CAP_NET_ADMIN` 和 `CAP_NET_RAW`。

The installed Orca CLI has no standalone `orca-linux` command. The local recipe help and `vm recipe doctor docker-ssh --provision --json` were used to confirm the supported invocation; the recipe returned `/workspace` as its project root.

| 检查项 | 容器结果 | 证据 |
|---|---|---|
| 构建 Go 后端和嵌入式前端 | PASS | 容器内 `go build ./cmd/duorelay`、`npm ci`、`npm run type-check`、45 个前端测试和 `npm run build` 均通过。 |
| `127.0.0.1:8080` 面板 | PASS | 嵌入式前端返回 HTTP 200。 |
| 首次设置 | PASS | 使用临时 `temp` 账号通过面板 API 完成注册和登录。 |
| 内核启动 | PASS | 创建临时本地 HTTP 节点；sing-box 校验、启动并报告健康。 |
| 真实代理请求 | PASS | 临时 HTTP 代理记录到 CONNECT 请求，并通过 sing-box 返回标记响应。 |
| 路由列表/文本校验和导入 | PASS | 5 条规则带行号完成校验和导入；清空后导出与重新导入结果逐字节比较一致。 |
| 非法规则隔离 | PASS | 端口 `70000` 和格式错误分别返回行级错误；非法内核选择失败时，原实例和 `current_config` 仍保持有效。 |
| 内核校验回滚 | PASS | 临时不支持的 SSH 节点在替换前校验失败；原 sing-box 实例保持运行。 |
| TUN 内核启动/清理 | PASS | `DUORELAY_TUN_INTEGRATION=1 go test ./internal/configuration -run TestLinuxTUNCaptureAndCleanup` 对 sing-box、mihomo 均通过，包含 route/rule 和 TUN 地址清理。 |
| redirect 设置/清理 | PASS | 面板应用创建自有 nft table，停止后删除；接口、路由和防火墙快照恢复干净。 |
| redirect 真实透明请求 | UNVERIFIED | bridge sidecar 可到达容器，但当前本地 fixture 将请求重定向成回环，不计为真实透明流量通过。 |
| tproxy 设置/清理 | PASS | 面板应用创建 nft TPROXY chain、`0xd0` fwmark 规则和 table 100 本地路由；停止后全部删除。 |
| tproxy 真实透明请求 | UNVERIFIED | 容器验证了规则安装和清理，但隔离 bridge fixture 无法提供可信的端到端 TPROXY 流量，因此不声明通过。 |
| DNS 接管/泄漏检查 | UNVERIFIED | 容器未提供独立 DNS 观察器，不声明 DNS 泄漏检查通过。 |
| Windows 系统代理 | 不在本次范围 | 必须在真实 Windows 目标机保存并恢复当前用户 Internet Settings。 |
| macOS TUN | 不在本次范围 | 必须在真实 macOS 目标机验证；容器结果不能代替。 |

redirect 和 TPROXY 分开记录规则生命周期与真实流量。当前 fixture 会形成回环，无法可信观察原始目的地址，因此真实流量仍为未验证。以上结果与 Windows、macOS 真实系统矩阵分开计算。
