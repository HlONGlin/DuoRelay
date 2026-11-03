# full / lite 部署入口验证

简体中文 | [English](deployment-variants-acceptance.en.md)

日期：2026-11-03。lite 功能与图文提交快进合并到 master，在此基础上补齐 Docker 与 Linux 一键部署的变体选择。未创建新发行标签，既有 beta 版本表述与已发布镜像标签保持原样。

## 变更

- Dockerfile 默认 full，接受 `DUORELAY_VARIANT=lite` 构建参数；lite 编译 Go tag 且仅下载 mihomo。
- 默认 `compose.yaml` 保持 full；新增 `compose.lite.yaml` 覆盖镜像及构建参数，服务、UID 和数据卷共用，lite 首次健康检查宽限 7 分钟。
- 下载器仍验证二进制 SHA-256。full Docker 保持 amd64 双核与 Cronet；lite 支持 amd64/arm64，补入此前从官方渠道下载的 mihomo arm64 二进制散列，并在本次 Docker 构建重新下载校验通过。
- Linux 一键脚本默认 full，新增 `--full` / `--lite` 和 `DUORELAY_VARIANT`；lite 使用锁定的单核下载器及正确的 lite 暂存路径。首次安装保护、数据目录和 systemd 服务保持原状。
- 发布工作流新增 full/lite 选择；full 继续使用原版本镜像标签，lite 后缀为 `-lite`，拒绝不包含 lite 代码的旧发行标签。
- README 快速开始分别列出两种版本的 Docker 与 Linux 一键命令，Linux 文档解释发行版前提。

## 本次结果

| 检查 | 结果 |
|---|---|
| `go test ./...`、`go test -tags lite ./...` | PASS，包含新增下载器测试；未修改包复用 Go 测试缓存 |
| 两种目标的 `go vet` | PASS |
| 前端类型检查、53 项现有测试 | PASS |
| Docker 生产前端构建 | PASS |
| full linux/amd64 Docker 镜像 | PASS，真实构建与启动 |
| lite linux/amd64 Docker 镜像 | PASS，真实构建与启动 |
| lite linux/arm64 Docker 镜像 | PASS，真实构建；前端与运行层的构建命令由仿真环境执行 |
| arm64 面板 `-h`、mihomo `-v` | PASS，Docker Desktop 仿真运行；不等于 arm64 实机验收 |
| 两种 Compose 配置解析 | PASS，服务/卷一致，lite 参数正确 |
| 镜像内容 | PASS，full 为 sing-box、mihomo、libcronet.so；lite 仅 mihomo |
| Compose 面板与身份验证 | PASS，生产 HTML 返回成功，API 确认编译变体，UID/GID 为 10001 |
| full → lite → full 共用卷 | PASS，临时账号可登录，节点和规则逐项一致，原 sing-box 首选项保留 |
| lite 首次 Geo 下载 | PASS，两个缓存文件非空且可复用 |
| `bash -n scripts/bootstrap-linux.sh` | PASS |
| `python3 scripts/test-bootstrap-linux.py` | PASS，14 项隔离分支测试 |
| 下载器单测 | PASS，full/amd64、lite/amd64、lite/arm64 的请求/文件组成与散列校验；非法目标及缺失锁定散列被拒绝 |

一键脚本测试替换了 Git、Go、npm、systemctl 等命令，以临时源码和目录验证参数、打包路径及安装器调用。覆盖默认 full、显式 full/lite、两种 CPU 架构、环境变量、参数优先级、帮助、错误选项、错误变体、32 位拒绝、非 root、systemd 未运行及过旧 Node.js。**没有将这些模拟分支测试记作真实系统安装通过。**

Compose 验证使用独立项目 `duorelay-deployment-acceptance`，临时数据卷与账号，通过测试覆盖文件显式映射 `/dev/net/tun` 并添加 NET_ADMIN，使用 bridge networking；无宿主端口发布、无真实数据挂载、未操作宿主代理/防火墙。结束后仅删除此测试项目及其临时卷。本次仅验证部署和数据切换，未追加透明流量通过声明。

## UNVERIFIED

- Debian、Ubuntu、Fedora、Rocky/AlmaLinux、Arch、openSUSE 等发行版逐项实机一键安装及 systemd 服务验收。
- apt-get 依赖补装在各发行版版本上的实际结果；软件源可能无法提供满足要求的 Go/Node.js。
- arm64 真实主机面板、代理流量与安装流程；本次只构建镜像并仿真运行命令入口。
- 新发布工作流的 GitHub Actions / GHCR 实际发布；本次未创建发行标签或触发发布。
- 原 [lite 验收报告](lite-acceptance.md) 中真实系统、透明转发、DNS 等未验证项目继续保留；其中两个 full opt-in HTTP 链式用例的基线失败不在本次部署入口改动中修复。

Linux 支持范围的表格是脚本适用条件，详见 [一键脚本的系统范围](linux-deployment.md#一键脚本的系统范围)。后续可在满足条件的测试主机执行真实安装验收；当前不承诺所有发行版开箱即用。
