# 文档索引

- [DuoRelay 图文介绍、界面截图与体积实测](showcase.md)
- [full / lite 构建、协议对照与数据互通](lite.md)
- [lite 构建、体积与隔离容器验收](lite-acceptance.md)

这里汇总常用文档。历史计划、阶段报告和设计稿记录的是对应时间的情况；当前运行能力与验证范围请结合代码及下列验收记录判断。

## 使用与部署

| 文档 | 内容 |
|------|------|
| [项目 README](../README.md) | 功能概览、快速开始和能力边界 |
| [Linux 部署](linux-deployment.md) | 打包、systemd 服务、权限和验收 |
| [Docker 部署](docker-deployment.md) | 单容器安装、数据备份和更新 |
| [Windows 部署](windows-deployment.md) | 打包、启动、数据迁移和系统代理 |
| [订阅格式](subscription-formats.md) | 支持的输入格式及限制 |
| [多节点代理](multi-node-proxy.md) | 负载均衡与链式代理说明 |

## 验证记录

| 文档 | 内容 |
|------|------|
| [透明代理验收](transparent-proxy-acceptance.md) | 各系统模式、DNS 行为和验证矩阵 |
| [路由验收](routing-acceptance.md) | 规则格式、优先级、导入导出与验证结果 |
| [容器验收](container-acceptance.md) | Orca Linux 容器环境和逐项结果 |
| [协议验证报告](protocol-verification-report.md) | 协议覆盖和验证资料 |

容器中的通过项不能代替真实宿主机或 Windows/macOS 验收；未完成的真实流量和 DNS 泄漏检查仍标为未验证。

## 开发与设计

| 文档 | 内容 |
|------|------|
| [开发指南](DEVELOPMENT.md) | 环境准备、构建、测试和目录结构 |
| [IR 架构决策](adr/ADR-001-ir-design.md) | 统一中间模型 |
| [双内核架构决策](adr/ADR-002-dual-kernel-coexistence.md) | 双内核共存设计 |
| [Clash API 差异](clashapi-diff.md) | 内核 API 差异记录 |

`P0-*`、`P1-*`、`P2-*`、`P3-*` 等文件为阶段性计划或报告，保留历史上下文，不代表当前全部事项的最终验收结论。
