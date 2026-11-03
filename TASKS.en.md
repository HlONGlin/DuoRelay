# DuoRelay Project Task Tracking

[简体中文](TASKS.md) | English

This file is a historical task plan. Its early progress table reported 1/69 tasks complete on 2024-10-31; later phase acceptance reports supersede it.

## Phase Overview in the Original Plan

| Phase | Planned tasks | Original status |
|---|---:|---|
| P0 Technical validation | 10 | 1 complete, 9 pending |
| P1 MVP | 22 | Pending |
| P2 Protocols and coexistence | 13 | Pending |
| P3 Feature completion | 16 | Pending |
| P4 Acceptance/release | 8 | Pending |
| Total | 69 | 1.4% in that snapshot |

## P0 Plan

The plan listed project skeleton, pinned kernel downloads, protocol comparison, Clash API, IR model, generators, kernel process manager, UDP/SOCKS5 experiment, sample library and technical/G0 review. Dependencies were documented between tasks: for example, protocol verification needed pinned kernels, generators needed IR, and the UDP experiment depended on Clash API.

G0 criteria were both kernel APIs integrated, a UDP/coexistence decision, approved IR and verified 28-entry protocol inventory. The source plan's “current status” is stale; use the dated [P0 review](docs/P0-10-technical-review.en.md) and current acceptance docs instead.

The remainder of the Chinese file contains detailed P1–P4 task estimates and checklists. Their implementation history is preserved in the corresponding phase reports and the [documentation index](docs/README.en.md).

## Original P0 Checklist

The original plan assigned these estimates and dependencies:

| Task | Estimate | Scope / dependency |
|---|---:|---|
| P0-01 Project skeleton | 1 day | Go module and directories, Vue 3 scaffold, IR/protocol map, tools, docs and ADR-001. Marked complete on 2024-10-31. |
| P0-02 Kernel acquisition and version pinning | 0.5 day | Finish download script, obtain sing-box/mihomo, calculate SHA-256 and update `kernels.lock`. |
| P0-03 Protocol and comparison verification | 1 day; depends on P0-02 | Verify 28 protocols, determine Rematch's nature, review v2rayA comparison and produce `protocol-matrix.csv`. |
| P0-04 Clash API integration | 1 day; depends on P0-02 | Test both APIs, record differences and implement an adapter prototype. |
| P0-05 IR definition | 1.5 days; depends on P0-03 | Validate four protocols' field maps, write JSON Schemas and serialization tests, review ADR-001. |
| P0-06 Generator skeleton | 1.5 days; depends on P0-05 | Generate SS/VMess configs for both kernels, add golden tests and validate with the kernels. |
| P0-07 Kernel process-manager prototype | 1 day; depends on P0-02 | Start/stop, capture logs, observe exit codes and invoke config checks. |
| P0-08 UDP coexistence pre-experiment | 1.5 days; depends on P0-04 | Test SOCKS5 chaining, TUIC/Hysteria2 and DNS; record data and decide ADR-002. Marked a critical risk. |
| P0-09 Sample-library initialization | 0.5 day; depends on P0-03 | Collect at least three samples per A-tier protocol, sanitize them and document the layout. |
| P0-10 Technical review and decision | 0.5 day; depends on P0-04 through P0-08 | Select libraries (ADR-003), define API/WebSocket and database schema, and review the G0 gate. |

### G0 gate criteria in the original plan

- [ ] Clash APIs integrated for both kernels.
- [ ] UDP coexistence conclusion recorded and P2 approach selected.
- [ ] IR review passed.
- [ ] 28-protocol inventory verified.

The source marked this gate not started at the time it was written.

## Original P1–P4 Plan

| Phase | Planned window | Goal | Original gate/status |
|---|---|---|---|
| P1 MVP | Weeks 3–9 | 16 shared protocols, subscriptions, single-kernel switching and a basic panel; 22 tasks. | Locked pending G0. |
| P2 exclusive protocols and coexistence | Weeks 10–13 | Complete the 28-protocol inventory and master/worker coexistence; 13 tasks. | Locked pending G1. |
| P3 feature completion and polish | Weeks 14–18 | Deliver all §7 features and freeze functionality; 16 tasks. | Locked pending G2. |
| P4 acceptance, packaging and release | Weeks 19–20 | Prepare the v1.0 release; 8 tasks. | Locked pending G3. |

The plan described P1 as an MVP with parsers, subscriptions, kernel switching, frontend work and speed testing. Later phase reports supersede these estimates and gate statuses.

## Planned Critical Path and Parallel Work

```text
P0-01 -> P0-03 -> P0-05 -> P0-06 -> P1-05~07 -> P1-10/11 -> P1-12 -> P1-13 -> G1
                                                                        |
                    P2-01~04 -> P2-12 -> G2 -> P3-01~16 -> G3 -> P4-01~08 -> G5
```

- Coexistence verification: P0-08 -> P2-05~11; the result determines the implementation approach.
- Frontend work: run in parallel with backend work starting at P1-16.
- Documentation: maintain continuously throughout all phases.

## Original Weekly Targets and Risk Board

**Week 1**: P0-01 complete; finish P0-02 and begin P0-03.

**Week 2**: complete P0-04 through P0-10, pass G0 and prepare to enter P1.

| Risk | Impact | Planned response | Original status |
|---|---|---|---|
| R-04 UDP coexistence compatibility | Medium | Verify early in P0-08. | Pending verification. |
| R-06 Lack of test nodes | High | Identify sources in P0-09. | Pending. |
| R-07 Protocol inventory discrepancies | Medium | Verify in P0-03. | Pending. |

The source plan was dated 2024-10-31, listed Week 1 of 20 and estimated 99 of 100 person-days remaining, with a target completion date of 2027-03-21 under its full-time assumption. These are original planning figures, not current estimates.
