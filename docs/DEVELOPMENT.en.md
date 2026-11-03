# Development Guide

[简体中文](DEVELOPMENT.md) | English

This guide covers source development. See [Linux deployment](linux-deployment.en.md) and [Windows deployment](windows-deployment.en.md) for deployment, and the [documentation index](README.en.md) for real-network verification results.

## Prerequisites

- Git, `make`, the Go version required by the root `go.mod`, and Node.js/npm.
- To start kernels, prepare the platform-specific executables under `kernels/`.

```bash
git clone https://github.com/HlONGlin/DuoRelay.git
cd DuoRelay
go mod download
cd web && npm ci && cd ..
./scripts/download-kernels.sh
```

`make dev` requires `air`; the `install-tools` target in the `Makefile` installs development tools. Use `make web-dev` for the standalone frontend development server. See `.air.toml` and `web/vite.config.ts` for the two server configurations.

## Build and Test

```bash
# Build the frontend and embed its assets in the Go program
make build
./build/duorelay -kernels ./kernels

# Backend tests and static analysis
go test ./...
go vet ./...

# Frontend checks
cd web
npm run type-check
npm test
npm run build
```

`make test` also enables race detection and generates coverage output; `make lint` requires `golangci-lint`. Real kernel configuration validation, actual proxy requests and restoration of system network state need separate acceptance testing and cannot be replaced by unit tests.

## Repository Layout

| Path | Purpose |
|---|---|
| `cmd/duorelay/` | Program entry point |
| `internal/api/` | HTTP and WebSocket APIs |
| `internal/ir/`, `internal/parser/` | Intermediate model and parsers |
| `internal/generator/`, `internal/kernel/` | Dual-kernel configuration generation and process management |
| `internal/routing/`, `internal/transparent/` | Routing text and transparent proxy |
| `internal/store/`, `internal/subscription/` | Persistence and subscription management |
| `web/` | Vue 3 and TypeScript frontend |
| `scripts/`, `deploy/` | Build, installation and acceptance scripts and service units |
| `docs/`, `testdata/` | Documentation, architecture decisions and test samples |

## Changes and Verification

- Format Go with `gofmt`; follow the existing Vue Composition API and TypeScript style for frontend code.
- Routing and configuration generation changes must cover existing rule order, compatibility with old data, and rollback after kernel validation failure. Do not add a raw kernel configuration entry point that bypasses validation.
- For transparent proxy changes, record firewall, policy routing and TUN state before and after startup in an isolated environment. Record real target-system and container results separately.
- Consult existing documents in [`adr/`](adr/) for architecture decisions; historical plans and completion reports reflect their original dates.

Existing acceptance records cover [transparent proxy](transparent-proxy-acceptance.en.md), [routing](routing-acceptance.en.md) and [containers](container-acceptance.en.md).
