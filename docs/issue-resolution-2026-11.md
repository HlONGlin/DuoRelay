# 2026-11 问题修复与验收

2026-11-04，对照用户提供的 `duorelay-issues.csv` 共 23 项问题。下面的“修复”表示已实现并完成所列验证，不表示所有操作系统的真实网络接管都已经验收。实现同时覆盖 full/lite；协议支持仍按所选内核判断。

## 对照记录

| 编号 | 修复结果 | 验证依据 |
|---|---|---|
| P0-01 | sing-box TUN 增加 sniff → hijack-dns 顺序；区分 bootstrap 和业务 DNS，支持 DoH 与代理出口，避免代理域名解析循环；直连用户目标明确使用业务 DNS | 双内核真实配置校验及启动；三个 full/lite 组合通过指定 DNS 解析新域名；TUN DNS 流量和泄漏仍待特权环境验收 |
| P0-02 | 分享链接、Clash/sing-box、v2rayN、Xray、SIP008、INI 条目部分成功；返回 skipped 行号/条目序号及原因；零可用节点才整体失败 | 解析、HTTP 更新及 API 回归；失败保留原节点 |
| P0-03 | 生成时复制节点并分配唯一运行名，保护 DIRECT/REJECT/PROXY；数据库显示名不变，运行探测按 ID 对应运行名 | 两真实内核配置检查；三种 full/lite 组合的同名节点多端口实际流量 |
| P0-04 | VLESS Reality 缺少 fingerprint 时补 chrome，两生成器一致处理 | 含缺省指纹的 Reality 配置交给锁定双内核检查 |
| P0-05 | 基本 VLESS XHTTP 由 mihomo 生成；sing-box 明确拒绝；pqv 及未支持的高级选项明确诊断并跳过，避免静默改写 | 锁定 mihomo XHTTP 校验；原生格式、编辑和高级参数回归。完整 Xray 高级 XHTTP/pqv 未实现 |
| P0-06 | 识别 naive+quic，映射为 sing-box Naive QUIC；lite/mihomo 继续按能力禁用 | 锁定 sing-box 配置检查；未进行真实 Naive 服务端握手 |
| P1-07 | PUT 节点部分编辑，保护身份字段；前端编辑名称、端点、标签及高级 JSON | API 非法修改拒绝/保留旧字段测试；Vue 编辑保存测试 |
| P1-08 | Linux capabilities/设备/防火墙锁与访问预检；Windows 管理员和 macOS root 检查；预检在停止旧内核前执行 | 权限失败保持当前代理回归；容器限制能力测试；跨平台编译 |
| P1-09 | 启动持有原始 HTTP listener；占用端口在初始化前报错；systemd 增加退避及启动频率限制 | full/lite 占用端口实际启动失败且不创建数据库；安装器渲染测试 |
| P1-10 | 普通服务使用 StateDirectory/STATE_DIRECTORY；自定义数据目录生成覆盖并维护属主 | 安装器隔离回归；systemd 实机启动仍待验证 |
| P1-11 | 显式区分 duorelay 用户普通服务与 --transparent root 覆盖；私有 xtables 锁路径与必要能力 | 安装器两种模式切换和权限拒绝回归；实际特权网络仍待验证 |
| P1-12 | 分类 DNS、超时、TLS、HTTP 状态及解析错误，保留底层 cause 与上次成功节点，隐藏订阅 URL | errors.Is/As、凭据隐藏、订阅更新失败保留节点测试 |
| P1-13 | 导入按连接指纹去重，串行执行检查和保存，返回 added/duplicates；订阅重命名保留节点 ID | 重复导入、订阅重命名、管理元数据保持测试与实际 API 验收 |
| P2-14 | 默认 loopback 监听；补 HTTPS、WebSocket 反代和 cookie-secure 示例 | 源码默认值、启动验收及[运维指南](operations.md)；未部署真实证书 |
| P2-15 | 界面及文档说明 allow_lan 与 TUN 接管互相独立 | 设置页类型检查、组件测试及文案核对 |
| P2-16 | 区分所选/生效模式，生效状态结合运行内核判断，提供待应用或权限原因 | 持久状态存在但内核停止的回归测试 |
| P2-17 | 历史配置最多保留最近十份及当前引用，清理只匹配程序生成的文件 | 旧配置、当前配置、手写文件保留测试 |
| P2-18 | 中文索引使用正常 UTF-8；CI 检测所有受跟踪 Markdown 编码和替换字符 | scripts/check-docs.py |
| P2-19 | 根目录十二份受跟踪历史文档移入 docs/archive，调整可解析的相对链接并新增索引 | 归档检查；未跟踪的本地笔记不发布，构建使用的 DESIGN 文件保留 |
| OPS-01 | 自动分配 loopback 控制器端口，保留高级配置显式地址 | 已占用端口回归与多次实际启动 |
| OPS-02 | SQLite 在线快照、配套 key、SHA-256 manifest、每日备份模板和安装前备份 | full/lite 运行中备份并启动恢复库；加密内容与原库不被修改的回归 |
| OPS-03 | stdin 密码重置，撤销旧会话，防并发旧密码创建会话；缺少 key 时拒绝维护 | 运行中 CLI 重置与旧 Cookie 失效实际验收；认证回归 |
| OPS-04 | 增加 full/lite CI：前后端检查、锁定内核、真实生成校验、API/双 SOCKS 流量、跨平台编译 | 新 validate.yml；对应命令已在本地 Linux 容器执行。尚未触发远端 CI |

## 追加需求

日志页默认最近 100 条不同消息，重复错误合并计数，WebSocket 有界聚合；增加定时保留策略和手动清空，持久化与并发清理同锁。后端覆盖 1000 条重复错误及不同端口错误保留；前端覆盖 500 次重复事件、有界历史和来源区分。

多 SOCKS5 端口、批量删除及关于页一键更新已有对应实现；本次继续验证 full/lite 兼容、同名节点与端口绑定。批量删除对正在使用或被端口绑定的节点给出冲突提示，非法批次不会部分删除。更新包匹配和替换逻辑有自动回归，真实在线安装更新未在 Windows 上执行。

## 本次执行环境与结果

所有运行、编译和测试都通过 **orca-linux 的 Linux 容器**完成，项目挂载 `/workspace`。未在 Windows 本机运行测试。前端源码复制到容器内部目录安装 Linux 依赖，避免复用宿主 Windows node_modules。

- Go full/lite：`go test ./...`、`go test -tags lite ./...` 与两个变体的 vet。
- 并发：kernel、configuration、subscription、store、api、auth 的 race 检查。
- 前端：类型检查、14 个测试文件 / 65 项测试、生产构建。
- 内核：`kernels.lock` 中 sing-box 1.14.4、mihomo 1.19.33，实际配置检查与应用启动。
- 流量：full/sing-box、full/mihomo、lite/mihomo 各启动两个 SOCKS5 端口，请求分别到达节点 A/B；节点故意同名。三个组合另通过指定 UDP DNS 解析新域名并访问临时 HTTP 服务。采用容器内临时上游，不依赖公网代理或公网 DNS。
- 运维：full/lite 在线备份恢复、重置密码及旧会话拒绝、日志策略恢复、批量删除保护、端口冲突；45 个隔离 bootstrap 分支与安装器升级/数据保留回归；systemd-analyze 检查生成的服务、覆盖和备份定时器语法，未启动真实 systemd 服务。
- 交叉编译：Windows amd64、macOS amd64/arm64、Linux arm64 的 full/lite。交叉编译仅验证编译兼容性。

未验证范围：当前容器没有特权 TUN/路由环境；真实 Linux 发行版的 systemd 启停、redirect/TPROXY/TUN 流量、DNS 泄漏，以及 Windows/macOS 的驱动、网络接管和系统代理恢复仍需目标环境实测。XHTTP 高级选项与 pqv 的限制已明确报错，不宣称完整支持。

复现主流程：

```bash
go build -o /tmp/duorelay-full ./cmd/duorelay
go build -tags lite -o /tmp/duorelay-lite ./cmd/duorelay
DUORELAY_REQUIRE_KERNELS=1 go test ./...
go test -tags lite ./...
python3 scripts/acceptance-api.py --full /tmp/duorelay-full \
  --lite /tmp/duorelay-lite --kernels ./kernels --work /tmp/duorelay-acceptance
```

`--work` 必须是新目录，验收日志和 JSON 结果保留在该目录。lite 启动需要可访问官方 Geo 数据下载地址，也可以通过 `--geodata` 提供已经下载的真实 Geo 文件目录。验收只使用新建的临时数据库，不操作现有服务数据。
