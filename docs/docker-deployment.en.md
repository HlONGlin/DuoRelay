# Docker Deployment and Updates

[简体中文](docker-deployment.md) | English

This setup builds a single-container image from this repository, sharing the panel and embedded frontend. Full is the default. Building requires Docker, the Compose plugin, and access to base images, npm, Go dependencies and kernel downloads.

| Variant | Kernels and dependencies | Architecture | Compose command |
|---|---|---|---|
| Full | sing-box, mihomo, Cronet | Linux amd64 | `docker compose up -d --build` |
| Lite | mihomo only, no sing-box/Cronet | Linux amd64 / arm64 | `docker compose -f compose.yaml -f compose.lite.yaml up -d --build` |

Both variants verify kernel SHA-256 values in `kernels.lock`. Other full architectures lack complete pinned kernel and Cronet data and are not claimed as supported. Lite downloads the Geo cache to `/data/configs` on first start and reports failures; `compose.lite.yaml` allows seven minutes for the initial health check.

The published private full image `ghcr.io/hlonglin/duorelay:v0.3.0-beta.2` can be pulled without a local build. Access requires a GitHub account with package read access and a personal access token with `read:packages`:

```bash
docker login ghcr.io -u YOUR_GITHUB_USERNAME
docker pull ghcr.io/hlonglin/duorelay:v0.3.0-beta.2
```

Enter the token at the password prompt. To use it with the existing Compose deployment, create `compose.override.yaml` in the project directory (requires Compose support for `!reset`):

```yaml
services:
  duorelay:
    image: ghcr.io/hlonglin/duorelay:v0.3.0-beta.2
    build: !reset null
```

Run `docker compose up -d`. To update, back up data, change the image tag and run `docker compose pull && docker compose up -d`. Alternatively build from source using the default Compose configuration.

The existing `v0.3.0-beta.2` image is full. Build lite from this repository; no existing lite image is assumed. Future release workflows can select `variant=full` or `variant=lite`: full uses `<release_tag>`, lite uses `<release_tag>-lite`. Lite requires a release tag containing lite code; old tags are rejected. This merge does not create a release tag or publish an image.

## Install Docker

If Docker Engine or Docker Desktop and the Compose plugin are installed, check `docker version` and `docker compose version`, then proceed to first installation. The following is for Docker-supported 64-bit Debian or Ubuntu, with sudo and internet access. Check for conflicts with existing Docker, containerd or Podman packages using the official [Ubuntu](https://docs.docker.com/engine/install/ubuntu/) or [Debian](https://docs.docker.com/engine/install/debian/) installation guide; do not blindly uninstall an existing environment.

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings

# Only original Debian / Ubuntu, not other distributions or derivatives.
. /etc/os-release
case "$ID" in
  ubuntu) docker_suite="${UBUNTU_CODENAME:-$VERSION_CODENAME}" ;;
  debian) docker_suite="$VERSION_CODENAME" ;;
  *) echo "Consult Docker's official installation guide for this system"; return 1 2>/dev/null || exit 1 ;;
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

If an ordinary user receives a Docker permission error, prefix the commands below with `sudo`. Alternatively follow Docker's [Linux post-installation steps](https://docs.docker.com/engine/install/linux-postinstall/) to configure a group; membership in the `docker` group grants root-equivalent privileges. On Windows and macOS install and start [Docker Desktop](https://docs.docker.com/get-started/get-docker/). The full image requires Linux amd64; lite supports Linux amd64/arm64. Other host architectures require appropriate container emulation.

## First Installation

Clone the project and run from its root:

```bash
git clone https://github.com/HlONGlin/DuoRelay.git
cd DuoRelay
# Full (default)
docker compose up -d --build

# Or lite; do not run both variants simultaneously
docker compose -f compose.yaml -f compose.lite.yaml up -d --build
docker compose ps
docker compose logs -f duorelay
```

Open `http://127.0.0.1:8080/` on the Docker host and create an administrator on first access. The panel port binds only to host loopback by default; use SSH forwarding or an HTTPS reverse proxy for remote access. Behind HTTPS, retain all Dockerfile default arguments in the Compose `command` and append `-cookie-secure`. If changing the container's listen port, update the image health check as well.

Data is stored in the Compose named volume `duorelay-data`, including the database, its `.key` file, configuration and logs. Do not use `docker compose down -v` unless all data should be deleted. The container runs as UID/GID `10001:10001`; ensure any bind-mounted host directory is writable by these IDs.

For lite, include `-f compose.yaml -f compose.lite.yaml` in subsequent `ps`, `logs`, `build`, `up` and other commands, for example `docker compose -f compose.yaml -f compose.lite.yaml logs -f duorelay`. Full uses default commands. Keep the Compose project name and directory fixed for both configurations to share the same data volume.

### Switch Full / Lite

Back up data and stop the current service, then start the desired variant using the existing volume:

```bash
docker compose stop
# Switch to lite
docker compose -f compose.yaml -f compose.lite.yaml up -d --build
# To switch back to full later, stop lite first
docker compose -f compose.yaml -f compose.lite.yaml stop
docker compose up -d --build
```

Both configurations use the same service and volume, so database copying is unnecessary. Lite retains unsupported nodes and subscription data but does not restore sing-box or cross-kernel runtime configurations; select compatible nodes again. See [full / lite data compatibility](lite.en.md#data-compatibility-and-in-place-upgrades) for migration boundaries.

When using the Dockerfile directly, select `--build-arg DUORELAY_VARIANT=full|lite` (default full):

```bash
docker build --build-arg DUORELAY_VARIANT=full -t duorelay:local .
docker build --build-arg DUORELAY_VARIANT=lite -t duorelay-lite:local .
# Build arm64 lite with cross-architecture buildx support
docker buildx build --platform linux/arm64 --build-arg DUORELAY_VARIANT=lite --load -t duorelay-lite:arm64 .
```

## Proxy Ports

Default Compose maps only the panel. Add proxy ports as needed. For example, to use the sing-box inbound on host-local port 1080, add `127.0.0.1:1080:1080/tcp` to `ports`, set the panel inbound address to `0.0.0.0` and inbound port to `1080`, then apply the configuration. UDP also requires a corresponding UDP mapping. Run `docker compose up -d` after changing Compose.

Ports and LAN options vary by kernel and inbound mode; consult the panel and verify connectivity before exposing them to a LAN. Mapping to a non-loopback address may allow others to use the proxy; configure access controls and a firewall first.

The default container has no TUN device or network administration privileges. System proxy and network rules inside the container do not automatically capture host traffic. TUN, redirect and TPROXY require separate configuration and acceptance for the specific network, permissions and traffic path. See [container acceptance](container-acceptance.en.md).

## Backup and Update

Record the current Git commit and stop the service for a consistent SQLite backup. Run these commands from the project root on a Linux host. The backup contains sensitive data and must be protected.

```bash
git rev-parse HEAD
docker compose stop
docker run --rm --volumes-from "$(docker compose ps -aq duorelay)" \
  -v "$(pwd):/backup" debian:bookworm-slim \
  tar -C /data -czf /backup/duorelay-data-backup.tar.gz .
docker compose start
```

After verifying the backup, obtain the target code version and rebuild:

```bash
git pull --ff-only
docker compose build --pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 duorelay
```

Check the panel, nodes and proxy connection. Kernel versions and hashes come from the updated `kernels.lock`; image builds verify both extracted kernels and Cronet and do not automatically track upstream latest versions. If the build fails, the existing container and volume remain; `docker compose start` can start the old container.

To roll back, rebuild the old image from the recorded Git version. If the new version migrated the database, stop the container and restore the pre-update backup to the volume. Restoration overwrites data, so back up the current volume separately first. Do not restore `.db` without `.key`; SQLite WAL files may also contain data, so stop the service for backup and restore.

To uninstall the container while retaining data, run `docker compose down`. Reinstall by running `docker compose up -d` from the same Compose project directory.
