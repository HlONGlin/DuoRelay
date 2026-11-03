# DuoRelay Parallel Development Plan

[简体中文](并行开发计划.md) | English

## Current Status
- ✅ P0-01 complete: the project skeleton is in place.
- 📍 Current position: preparing to start P0-02 through P0-10.

## Parallel Work Allocation

### Main thread (this session): coordination and core architecture
**Responsibilities**: project coordination, key decisions, and integration.
- Track the progress of each workstream.
- Manage dependencies between tasks.
- Integrate code and resolve conflicts.
- Consolidate documentation.

### Agent 1: kernels and protocol verification
**Tasks**:
- P0-02: download kernels and pin versions.
- P0-03: verify the protocol matrix.
- P0-07: prototype kernel process management.
- P0-09: initialize the sample library.

**Deliverables**:
- Downloaded kernels in `kernels/`.
- Updated `kernels.lock`.
- Protocol inventory in `protocol-matrix.csv`.
- Kernel start/stop management code.
- Test sample library.

### Agent 2: Clash API and network experiments
**Tasks**:
- P0-04: integrate the Clash API.
- P0-08: preliminary UDP coexistence experiment (critical risk).

**Deliverables**:
- Complete Clash API adapter.
- API differences documentation.
- UDP forwarding test report.
- ADR-002 decision record.

### Agent 3: IR model and generators
**Tasks**:
- P0-05: complete the IR model definition.
- P0-06: implement the generator skeleton.

**Deliverables**:
- Complete field mappings for 4 protocols.
- JSON Schema definitions.
- sing-box and mihomo generators.
- Golden tests.

## Dependencies

```
Start in parallel:
  Agent 1: P0-02 (independent)
  Agent 2: P0-04 (requires kernels from P0-02)
  Agent 3: P0-05 (design can start before P0-03 verification)

Sequential dependencies:
  P0-02 -> P0-03 -> P0-05 -> P0-06
  P0-02 -> P0-04 -> P0-08
  P0-03 -> P0-09

Final integration:
  All workstreams -> P0-10 technical review
```

## Execution Strategy

### Phase 1 (start immediately)
1. Agent 1 performs P0-02 (kernel downloads).
2. This session prepares P0-03 by collecting protocol documentation.

### Phase 2 (after P0-02)
1. Agent 2 performs P0-04 (Clash API).
2. Agent 3 performs P0-05 (IR model).
3. Agent 1 continues P0-03 (protocol verification).

### Phase 3 (after core tasks)
1. Agent 3 performs P0-06 (generators).
2. Agent 2 performs P0-08 (UDP experiment), a critical task.
3. Agent 1 performs P0-07 and P0-09.

### Phase 4 (integration)
1. This session integrates all deliverables.
2. Conduct the P0-10 technical review.
3. Pass the G0 gate.

## Estimated Timeline
- Phase 1: 0.5 day.
- Phase 2: 1.5 days.
- Phase 3: 2 days.
- Phase 4: 0.5 day.
- **Total**: 4.5 days in parallel versus 10 days sequentially.

## Risk Management
- 🔴 P0-08 UDP results are uncertain: run it early and reserve time for adjustments.
- 🟡 Protocol verification may find discrepancies: raise P0-03 priority if needed.
- 🟢 Tasks are relatively independent: parallelization risk is low.

---

**Created**: 2024-10-31

**Execution**: immediate
