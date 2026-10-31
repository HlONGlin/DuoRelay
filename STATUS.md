# DuoRelay Project Status

## Current Phase: P0 - Technical Validation

**Week**: 1 of 20  
**Status**: ✅ P0-01 Complete

### Completed Tasks

#### P0-01: Project Skeleton ✅
- [x] Go module initialization
- [x] Directory structure created
- [x] Makefile with build targets
- [x] Frontend scaffolding (Vue 3 + TypeScript)
- [x] Development tooling (.air.toml, .golangci.yml)
- [x] Core module stubs (ir, generator, parser, kernel, store)
- [x] Git configuration (.gitignore, README)
- [x] Documentation structure

**Deliverables**:
- ✅ Single binary builds successfully (`make build`)
- ✅ Project structure follows specification
- ✅ Development tools configured

### Next Tasks (P0-02 through P0-10)

#### P0-02: Kernel Download & Version Lock
- [ ] Complete kernel download script
- [ ] Verify sing-box and mihomo downloads
- [ ] Generate checksums
- [ ] Test on Linux amd64/arm64

#### P0-03: Protocol Matrix Verification
- [ ] Lock kernel versions
- [ ] Review official documentation
- [ ] Verify all 28 protocols
- [ ] Document protocol ownership
- [ ] Cross-check v2rayA claims

#### P0-04: Clash API Integration
- [ ] Test sing-box Clash API endpoints
- [ ] Test mihomo Clash API endpoints
- [ ] Document API differences
- [ ] Create adapter layer

#### P0-05: IR Model Finalization
- [ ] Validate IR design with 4 protocols (SS/VMess/VLESS/Trojan)
- [ ] Create JSON schemas
- [ ] Test serialization/deserialization
- [ ] Complete ADR-001

#### P0-06: Generator Skeleton
- [ ] Implement sing-box generator for SS + VMess
- [ ] Implement mihomo generator for SS + VMess
- [ ] Create golden tests
- [ ] Verify with kernel validation commands

#### P0-07: Kernel Process Manager
- [ ] Process lifecycle management
- [ ] stdout/stderr capture
- [ ] Exit code monitoring
- [ ] Crash detection

#### P0-08: UDP Coexistence Experiment ⚠️ CRITICAL
- [ ] Setup test environment
- [ ] Test TUIC over SOCKS5 chain
- [ ] Test Hysteria2 over SOCKS5 chain
- [ ] Test DNS UDP forwarding
- [ ] Document findings in ADR-002
- [ ] Decision: Full chain vs Degraded chain

#### P0-09: Sample Library
- [ ] Collect 3+ samples per A-tier protocol
- [ ] Sanitize all data
- [ ] Document sample format
- [ ] Create test fixtures

#### P0-10: Technical Review
- [ ] Finalize library choices (ADR-003)
- [ ] Define API/WebSocket contracts
- [ ] Review database schema
- [ ] Gate G0 checkpoint

## Gate G0 Criteria (End of Week 2)

- [ ] Both kernels' Clash API integrated
- [ ] UDP coexistence conclusion documented
- [ ] IR model validated with 4 protocols
- [ ] Protocol matrix verified (28 protocols confirmed)

**Status**: 🟡 In Progress

## Development Statistics

- **Lines of Code**: ~2,000 (skeleton)
- **Test Coverage**: 0% (tests to be added in P0-05+)
- **Protocols Implemented**: 0/28
- **Tasks Complete**: 1/69

## Known Issues

None yet - skeleton phase

## Risk Tracking

| ID | Risk | Status | Mitigation |
|----|------|--------|------------|
| R-06 | Lack of real test nodes for 28 protocols | 🟡 Open | P0-09 will identify sources; P2-12 will build test range |
| R-04 | UDP forwarding compatibility | 🟡 Open | P0-08 experiment will determine feasibility |
| R-07 | Protocol matrix may differ from spec | 🟡 Open | P0-03 will verify with locked kernel versions |

## Next Milestone

**G0 - Technical Validation Complete**  
Target: End of Week 2  
Confidence: 🟢 High

---

*Last updated: 2024-10-31*  
*Project started: 2024-10-31*
