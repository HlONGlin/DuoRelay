# DuoRelay Development Guide

## Project Structure

```
duorelay/
├── cmd/duorelay/          # Main application entry
├── internal/              # Internal packages
│   ├── api/              # REST & WebSocket API
│   ├── clashapi/         # Clash API adapter
│   ├── diagnose/         # System diagnostics
│   ├── generator/        # Config generators
│   │   ├── singbox/     # sing-box generator
│   │   └── mihomo/      # mihomo generator
│   ├── ir/              # Intermediate representation
│   ├── kernel/          # Kernel process management
│   ├── parser/          # Protocol parsers (28 protocols)
│   ├── store/           # Database layer
│   ├── subscription/    # Subscription management
│   └── task/            # Background task queue
├── web/                  # Vue 3 frontend
├── testdata/            # Test samples
├── testrange/           # Testing infrastructure (docker compose)
├── deploy/              # Deployment configs
├── docs/                # Documentation
│   └── adr/            # Architecture Decision Records
└── scripts/             # Build & utility scripts
```

## Development Workflow

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/holi/duorelay.git
cd duorelay

# Install Go dependencies
go mod download

# Install frontend dependencies
cd web && npm install && cd ..

# Install development tools
make install-tools

# Download kernels
./scripts/download-kernels.sh
```

### Running in Development Mode

```bash
# Terminal 1: Backend with hot reload
make dev

# Terminal 2: Frontend dev server
make web-dev
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linters
make lint
```

### Building

```bash
# Build for current platform
make build

# Build for multiple platforms
make build-all
```

## Code Style

### Go
- Follow standard Go conventions
- Use `gofmt` and `goimports`
- Keep functions focused and small
- Write table-driven tests
- Document exported types and functions

### TypeScript/Vue
- Use TypeScript strict mode
- Follow Vue 3 Composition API style
- Component names in PascalCase
- Use `<script setup>` syntax

## Architecture Decisions

All significant technical decisions are documented in `docs/adr/`. When making a major choice:

1. Copy `docs/adr/TEMPLATE.md`
2. Fill in the sections
3. Number it sequentially (ADR-XXX)
4. Commit with the implementation

## Git Workflow

### Branches
- `main`: stable, always deployable
- `feature/xxx`: feature branches
- `fix/xxx`: bug fix branches

### Commits
Use Conventional Commits:
```
feat: add Shadowsocks parser
fix: correct VMess link parsing
docs: update API documentation
test: add parser test cases
```

### Before Committing
```bash
make lint
make test
```

## Phase Tracking

Current phase: **P0 - Technical Validation**

See `企划书_v2.0.md` for the full 20-week plan.

### P0 Checklist (Week 1-2)
- [x] P0-01: Project skeleton
- [ ] P0-02: Kernel download script
- [ ] P0-03: Protocol matrix verification
- [ ] P0-04: Clash API integration
- [ ] P0-05: IR model definition
- [ ] P0-06: Generator skeleton
- [ ] P0-07: Kernel process manager
- [ ] P0-08: UDP coexistence experiment
- [ ] P0-09: Sample library initialization
- [ ] P0-10: Technical review & ADR

## Testing Strategy

### Unit Tests
- All parsers must have table-driven tests
- Generators must have golden tests
- Core IR logic must be covered

### Integration Tests
- Config generation → kernel validation → connectivity
- Use docker-compose for multi-protocol testing

### E2E Tests
- Playwright for browser testing
- Cover core user journeys

## Performance Budget

Frontend:
- Bundle size: < 1.5MB compressed
- First paint: < 800ms
- 10K nodes list: ≥ 55fps scrolling

Backend:
- API response: < 50ms (dashboard)
- Search 10K nodes: < 100ms
- Memory: < 512MB (excluding kernels)

## Security Checklist

Before any release:
- [ ] No default passwords
- [ ] Sensitive data encrypted at rest
- [ ] CSRF protection enabled
- [ ] Input validation on all endpoints
- [ ] No command injection vectors
- [ ] Dependencies scanned for vulnerabilities

## Getting Help

- Read the enterprise plan: `企划书_v2.0.md`
- Check existing ADRs in `docs/adr/`
- Review test samples in `testdata/`

---

*Last updated: 2024-10-31 (P0 phase)*
