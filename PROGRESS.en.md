# DuoRelay Live Progress Board

[简体中文](PROGRESS.md) | English

**Updated**: 2024-10-31 (historical snapshot)

## Parallel Tasks at the Time

### Main thread (this session)

**Status**: In progress

**Task**: Prepare P0-03 protocol-matrix verification
- Created the verification-plan document.
- Organized the protocol-list template.
- Awaited P0-02 completion to obtain kernel versions.
- Prepared comparison and verification materials.

### Agent 1: IR model

**Status**: Running in the background

**Task**: Define and validate the P0-05 IR model
- Define fields for Shadowsocks, VMess, VLESS and Trojan.
- Create JSON Schemas.
- Implement serialization tests.
- Completion estimate: awaiting notification.

### Agent 2: Kernel downloads

**Status**: Running in the background

**Task**: Obtain kernels and pin versions for P0-02
- Download sing-box and mihomo.
- Generate checksums.
- Update `kernels.lock`.
- Completion estimate: awaiting notification.

### Agent 3: Clash API integration

**Status**: Running in the background

**Task**: Integrate the Clash APIs for P0-04
- Test the Clash API endpoints on both kernels.
- Record API differences.
- Implement a unified adapter.
- Completion estimate: awaiting notification.

## Task Progress in the Snapshot

| Phase | Total tasks | Complete | In progress | Reported completion |
|---|---:|---:|---:|---:|
| P0 | 10 | 1 | 4 | 10% to 50% (in progress) |
| P1 | 22 | 0 | 0 | 0% |
| P2 | 13 | 0 | 0 | 0% |
| P3 | 16 | 0 | 0 | 0% |
| P4 | 8 | 0 | 0 | 0% |

## Completed

- [x] P0-01: Project skeleton (2024-10-31)
  - 40 files and 3,200+ lines of code.
  - Commits: `ccf6b9a`, `41c262f`.

## In Progress

- [ ] P0-02: Kernel downloads (Agent 2).
- [ ] P0-03: Protocol verification (main thread preparation).
- [ ] P0-04: Clash API integration (Agent 3).
- [ ] P0-05: IR model (Agent 1).

## Not Yet Started

- [ ] P0-06: Generator skeleton.
- [ ] P0-07: Kernel process management.
- [ ] P0-08: UDP coexistence experiment (critical).
- [ ] P0-09: Initialize sample library.
- [ ] P0-10: Technical review.

## Next Milestone

**G0 gate**, targeted for the end of week two:
- [ ] Integrate the Clash APIs for both kernels.
- [ ] Reach a conclusion on UDP coexistence.
- [ ] Pass IR review.
- [ ] Verify the protocol inventory.

## Burn-Down Snapshot

```text
Person-days used: 1 / 100
Remaining: 99 person-days
Parallel tasks: 4
Estimated time for this batch: 0.5-1 day
```

## Pending Notifications

Awaiting completion notices from three background agents.

---

**Project**: DuoRelay

**Phase**: P0 technical validation

**Week**: Week 1 of 20

This is a historical snapshot, not a live task feed. Later completion reports supersede these statuses and estimates.
