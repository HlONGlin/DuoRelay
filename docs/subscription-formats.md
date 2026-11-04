# 订阅导入格式

简体中文 | [English](subscription-formats.en.md)

订阅管理中填写 HTTP(S) 地址；节点管理的「导入节点」可粘贴响应内容或链接合集。
两者使用同一个解析入口，不依赖服务商的 Content-Type。

| 格式 | 支持范围 |
| --- | --- |
| v2rayN / V2RayNG 链接合集 | 每行一个分享链接，可混用协议；支持空行和独立注释行 |
| Base64 | 标准、URL-safe、有/无 padding、换行分段；最多三层包装，适用于下列格式 |
| Clash / Mihomo | YAML、JSON 的 `proxies` 和 JSON 节点数组 |
| sing-box | JSON `outbounds` 和出站数组；跳过 direct、selector 等控制项 |
| Xray / V2Ray | JSON `outbounds` 或单个 outbound；VMess、VLESS、Trojan、SS、SOCKS、HTTP；TCP、WS、gRPC、基本 VLESS XHTTP、TLS、Reality |
| v2rayN JSON | VMess 分享对象、现有项目兼容的节点对象、数组和 `configs` 包装 |
| SIP008 | `servers` 对象或服务器数组，保留 Shadowsocks 插件参数 |
| SSD | `ssd://` 编码服务器列表，支持全局默认参数和各服务器覆盖 |
| Surge / Loon | `[Proxy]` 或裸节点行；常用 SS、SSR、VMess、VLESS、Trojan、HTTP、SOCKS 等参数 |
| Quantumult X | `[server_local]` 或裸节点行；支持常用认证、TLS、WS、SS 混淆参数 |
| OpenVPN | 保留已有的原生配置导入 |

## 分享链接补齐

- SS：SIP002/SIP003 的 Base64 userinfo、明文 method/password，以及旧版整段 authority Base64。
- VMess：v2rayN Base64 JSON，以及 endpoint 形式的凭据链接。
- Hysteria2：`hysteria2://` 与 `hy2://`；保留带冒号的认证密码。
- AnyTLS：保留现有 `anytls://` 支持。
- Naive：识别 `naive+https://`、`naive+quic://`；由 full 的 sing-box 运行，QUIC 需要兼容的服务端和 Cronet 动态库。lite/mihomo 不提供此协议。
- VLESS XHTTP：由 mihomo 运行；支持 path、host、mode、scMaxEachPostBytes、scMinPostsIntervalMs、xPaddingBytes 和 xPaddingKey。sing-box 不生成 XHTTP；pqv、extra、xmux 等未实现选项会明确报错并跳过，不静默丢弃。
- Mieru：保留项目的 endpoint 链接；新增原生 `mieru://` protobuf 配置和 `mierus://` 简化链接。按服务器和端口绑定拆成节点；保留 TCP/UDP、端口范围、multiplexing、handshake-mode、traffic-pattern。
- URL 名称片段只解码一次，保留加号和转义百分号；gRPC serviceName、TLS 指纹、验证开关和 WS 额外请求头传递给生成器。

## 边界

- 导入的是代理节点，不迁移其他客户端的监听、规则、策略组、DNS、脚本和模块。
- INI 格式只读取节点段；不支持的协议/关键选项报行号，不静默改成另一种连接方式。
- Xray 的 mKCP、复杂链式拨号和超出上述范围的 XHTTP 传输控制尚未完整转换。
- Mieru 原生明文凭据可导入；仅有密码哈希、自定义 MTU（除上游默认 1400）或内置 dialer 的配置会报错。节点交由 Mihomo 生成。
- 可独立解析的条目允许部分成功，返回 `skipped[{line,reason}]`；链接/INI 使用行号，JSON/原生列表使用条目序号。零可用条目或下载失败时保留原有节点；部分成功时按可用节点更新，需查看跳过报告。错误信息不会回显完整链接或凭据。
- 导入返回新增和重复数量，同一连接改显示名不会再生成重复节点；订阅刷新会尽量保留节点 ID、收藏、分组和标签。手动创建仍允许用户刻意保存重复连接。
- 格式导入测试不等于真实服务器连通性测试。连接能力还受内核版本和服务端参数影响。

## 验证

`internal/subscription/compat_test.go` 覆盖混合链接、三种包装、BOM/UTF-16、各新增格式、畸形输入、真实 HTTP 下载到入库、失败保留原节点，以及 IR 到双内核生成器的关键参数保留。

`internal/api/server_test.go` 同时验证 SIP008、Surge、Mieru 的导入接口和空节点拒绝。
