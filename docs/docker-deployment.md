# Docker 部署与更新

本方案从本仓库源码构建单容器镜像，包含面板、嵌入式前端、sing-box、mihomo 和 Linux amd64 Cronet。当前只支持 Linux amd64；其他架构需要先补齐 `kernels.lock` 中的内核及 Cronet 哈希。构建需要 Docker、Compose 插件及访问基础镜像、npm、Go 依赖和内核下载地址的网络。

也可以拉取已发布的私有镜像 `ghcr.io/hlonglin/duorelay:v0.3.0-beta.2`，无需本地构建。镜像访问需要有包读取权限的 GitHub 账号及带 `read:packages` 权限的个人访问令牌：

```bash
docker login ghcr.io -u YOUR_GITHUB_USERNAME
docker pull ghcr.io/hlonglin/duorelay:v0.3.0-beta.2
```

在密码提示时输入令牌。使用现有 Compose 部署时，可在项目目录创建 `compose.override.yaml`（需要支持 `!reset` 的 Docker Compose）：

```yaml
services:
  duorelay:
    image: ghcr.io/hlonglin/duorelay:v0.3.0-beta.2
    build: !reset null
```

然后执行 `docker compose up -d`；更新时先备份数据，再将镜像标签改为目标版本并执行 `docker compose pull && docker compose up -d`。也可保持默认 Compose 配置，按下文从源码构建。

## 安装 Docker

已安装 Docker Engine 或 Docker Desktop 及 Compose 插件的用户，可运行 `docker version` 和 `docker compose version` 检查，然后跳到首次安装。以下步骤适用于 Docker 官方支持的 Debian 或 Ubuntu amd64 系统，需要具备 sudo 权限并连接互联网。安装前如已存在其他 Docker、containerd 或 Podman 软件包，请先参照 [Ubuntu 官方安装文档](https://docs.docker.com/engine/install/ubuntu/) 或 [Debian 官方安装文档](https://docs.docker.com/engine/install/debian/) 检查冲突；不要盲目卸载现有环境。

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings

# 仅支持原版 Debian / Ubuntu，不适用于其他发行版或衍生版。
. /etc/os-release
case "$ID" in
  ubuntu) docker_suite="${UBUNTU_CODENAME:-$VERSION_CODENAME}" ;;
  debian) docker_suite="$VERSION_CODENAME" ;;
  *) echo "请参照 Docker 官方文档安装此系统的 Docker"; return 1 2>/dev/null || exit 1 ;;
esac
sudo curl -fsSL "https://download.docker.com/linux/$ID/gpg" -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
printf 'Types: deb\nURIs: https://download.docker.com/linux/%s\nSuites: %s\nComponents: stable\nArchitectures: %s\nSigned-By: /etc/apt/keyrings/docker.asc\n' \
  "$ID" "$docker_suite" "$(dpkg --print-architecture)" | sudo tee /etc/apt/sources.list.d/docker.sources >/dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
sudo docker run --rm hello-world
sudo docker compose version
```

如果普通用户运行 `docker` 时出现权限错误，以下文档中的 Docker 命令均可加 `sudo`。也可以按 [Docker Linux 安装后步骤](https://docs.docker.com/engine/install/linux-postinstall/) 配置用户组；`docker` 组成员相当于拥有 root 权限。Windows 和 macOS 请按 [Docker Desktop 官方安装说明](https://docs.docker.com/get-started/get-docker/) 安装并启动 Desktop；本项目容器镜像仍要求 Linux amd64。

## 首次安装

克隆项目并在项目根目录执行：

```bash
git clone https://github.com/HlONGlin/DuoRelay.git
cd DuoRelay
docker compose up -d --build
docker compose ps
docker compose logs -f duorelay
```

在 Docker 主机访问 `http://127.0.0.1:8080/`，首次访问时创建管理员。默认面板端口只绑定宿主机回环地址；远程访问可使用 SSH 端口转发，或配置 HTTPS 反向代理。HTTPS 代理后需在 Compose 的 `command` 中保留 Dockerfile 的全部默认参数并追加 `-cookie-secure`。如果修改容器内监听端口，也需同步修改镜像的健康检查。

数据保存在 Compose 命名卷 `duorelay-data` 中，包括数据库、对应 `.key` 文件、配置和日志。不要使用 `docker compose down -v`，除非确定要删除全部数据。容器以 UID/GID `10001:10001` 运行；改用宿主机目录挂载时，需先确保该目录对这两个 ID 可写。

## 代理端口

默认 Compose 只映射面板，代理端口需要按需求添加。例如，要通过宿主机本地的 1080 使用 sing-box 入站，在 `ports` 中增加 `127.0.0.1:1080:1080/tcp`，并在面板中将入站地址设为 `0.0.0.0`、入站端口设为 `1080`，然后应用配置。UDP 场景还需要对应的 UDP 映射。更改 Compose 后执行 `docker compose up -d`。

不同内核和入站模式的端口、LAN 选项不同；请以面板实际配置为准，验证连接后再对局域网开放。映射到非回环地址可能使代理被其他人使用，应先配置访问控制及防火墙。

默认容器没有 TUN 或网络管理权限。容器中的系统代理和网络规则不自动接管宿主机；TUN、redirect、TPROXY 需针对具体网络、权限和流量路径单独配置与验收。现有验证边界见 [容器验收](container-acceptance.md)。

## 备份和更新

更新前先记录当前 Git 提交，并停止服务以获得一致的 SQLite 备份。以下命令在 Linux 主机项目根目录执行；备份包含敏感数据，妥善保管。

```bash
git rev-parse HEAD
docker compose stop
docker run --rm --volumes-from "$(docker compose ps -aq duorelay)" \
  -v "$(pwd):/backup" debian:bookworm-slim \
  tar -C /data -czf /backup/duorelay-data-backup.tar.gz .
docker compose start
```

确认备份成功后，获取要更新到的代码版本并重建：

```bash
git pull --ff-only
docker compose build --pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 duorelay
```

检查面板、节点和代理连接。内核版本及其哈希来自更新后的 `kernels.lock`；镜像构建会校验解压后的两个内核及 Cronet 文件，不会自动跟随上游最新版本。如果构建失败，已有容器和数据卷仍在，可以使用 `docker compose start` 启动旧容器。

回滚时应使用更新前记录的 Git 版本重新构建旧镜像；如果新版本迁移过数据库，先停止容器，再将更新前备份恢复到数据卷。恢复会覆盖数据，先另行备份当前卷。不要只恢复 `.db` 而遗漏 `.key`；SQLite 的 WAL 文件也可能包含数据，因此备份和恢复应在服务停止时操作。

要卸载容器但保留数据，执行 `docker compose down`。需要重新安装时，在同一 Compose 项目目录运行 `docker compose up -d`。
