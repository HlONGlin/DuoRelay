# 单内核发行与升级

`duorelay-mihomo` 接续 `duorelay-lite`。`duorelay-singbox` 随包携带 sing-box 和 Cronet。过渡期可用 `make build-legacy` 构建旧双核面板。发行包内的程序名继续是 `duorelay`（Windows 为 `duorelay.exe`），方便沿用服务配置。

升级前备份 SQLite 数据库和对应的 `.key`。使用同一数据目录启动新版本时，DuoRelay 会优先用保存的节点重新生成与新内核匹配的配置。原始 JSON 或 YAML 无法直接转换，组合里包含不兼容成员时也会保留原配置，在面板中选择兼容节点后重新启动。Mieru 在本项目锁定的 mihomo 构建中受支持，sing-box 版会保留这些节点并禁用启动。VLESS 的 `mlkem768x25519plus` 加密参数会写入 mihomo 配置。已用 mihomo 1.19.33 验证配置语法。真实握手还取决于远端配置。

在节点页勾选两个或更多兼容节点，点击“启动手动分组”。使用成员的“切换到此节点”按钮可以切换实际出口，重启后恢复所选成员。原有负载均衡继续由 mihomo 提供。

从节点生成的配置会在保存局域网访问、监听端口或内核设置时自动重新应用。应用失败时，面板会报告错误并尝试恢复之前的配置。控制器端口设为 `0` 会自动分配，可在设置中指定固定的本地端口。自动分配的控制器未就绪时，普通节点配置会换端口重试一次。配置目录保留当前配置和最近七天内最多五组历史，手工备份文件不在自动清理范围内。Linux 安装器更新主程序时根据旧程序自身报告的版本与提交号命名二进制备份。

```bash
bash scripts/download-kernels.sh --variant mihomo
make package-linux-mihomo VERSION=0.3.2
bash scripts/download-kernels.sh --variant singbox
make package-linux-singbox VERSION=0.3.2
```

Windows 对应命令为 `scripts/package-windows.ps1 -Version 0.3.2 -Variant mihomo` 和 `scripts/package-windows.ps1 -Version 0.3.2 -Variant singbox`。运行 `duorelay --version` 可查看二进制版本、提交和构建时间，`/api/version` 同时返回发行版、内核列表和平台。
