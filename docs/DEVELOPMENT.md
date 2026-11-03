# 开发指南

本文介绍源码开发入口。部署步骤见 [Linux 部署](linux-deployment.md) 和 [Windows 部署](windows-deployment.md)，真实网络验证结果见 [文档索引](README.md)。

## 环境准备

- Git、`make`、符合根目录 `go.mod` 要求的 Go、Node.js/npm。
- 需要启动内核时，先准备 `kernels/` 中对应平台的可执行文件。

```bash
git clone https://github.com/HlONGlin/DuoRelay.git
cd DuoRelay
go mod download
cd web && npm ci && cd ..
./scripts/download-kernels.sh
```

`make dev` 依赖 `air`；可按 `Makefile` 中的 `install-tools` 目标安装开发工具。前端独立开发服务使用 `make web-dev`。两个服务的启动配置以 `.air.toml` 和 `web/vite.config.ts` 为准。

## 构建和测试

```bash
# 构建前端并将静态资源嵌入 Go 程序
make build
./build/duorelay -kernels ./kernels

# 后端测试及静态检查
go test ./...
go vet ./...

# 前端检查
cd web
npm run type-check
npm test
npm run build
```

`make test` 还会启用 race 检测并生成覆盖率文件；`make lint` 需要额外安装 `golangci-lint`。真实内核配置校验、实际代理请求以及系统网络状态恢复需另外验收，不能由单元测试代替。

## 目录结构

| 路径 | 用途 |
|------|------|
| `cmd/duorelay/` | 程序入口 |
| `internal/api/` | HTTP 与 WebSocket 接口 |
| `internal/ir/`、`internal/parser/` | 中间模型与解析 |
| `internal/generator/`、`internal/kernel/` | 双内核配置生成和进程管理 |
| `internal/routing/`、`internal/transparent/` | 路由文本与透明代理 |
| `internal/store/`、`internal/subscription/` | 数据持久化与订阅管理 |
| `web/` | Vue 3 和 TypeScript 前端 |
| `scripts/`、`deploy/` | 构建、安装、验收脚本与服务单元 |
| `docs/`、`testdata/` | 文档、架构决策与测试样本 |

## 修改与验证

- Go 代码使用 `gofmt`，前端遵循现有 Vue Composition API 和 TypeScript 风格。
- 路由或配置生成变更需覆盖已有规则顺序、旧数据兼容与内核校验回滚，不能开放原始内核配置绕过入口。
- 涉及透明代理时，在隔离环境记录启停前后的防火墙、策略路由和 TUN 状态；真实目标系统结果与容器结果分别记录。
- 涉及架构决策时参考 [`adr/`](adr/) 中现有文档；历史阶段计划和完成报告仅反映当时状态。

现有验收记录见 [透明代理](transparent-proxy-acceptance.md)、[路由](routing-acceptance.md) 和 [容器](container-acceptance.md)。
