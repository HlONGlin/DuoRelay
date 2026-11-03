# ADR-002: Dual-Kernel Coexistence Decision

[简体中文](ADR-002-dual-kernel-coexistence.md) | English

## Status

Decided

## Context

DuoRelay supports 28 proxy protocols across sing-box and mihomo: 14 shared protocols, 4 sing-box-only and 10 mihomo-only according to the original proposal's grouping. The system needed a policy for running nodes supported by only one kernel.

## Decision

**Use single-kernel switching; do not run both kernels in parallel.** When a selected node requires another kernel:

1. Stop the currently running kernel.
2. Generate configuration for the target kernel.
3. Start the target kernel.
4. Notify the user that the kernel has switched.

## Rationale

### Master/Worker Coexistence with UDP over SOCKS5

The proposed design would run shared protocols in a master kernel and unique protocols in a worker, connecting them with a SOCKS5 proxy chain. It has several issues:

1. SOCKS5 UDP ASSOCIATE requires explicit client support. TUIC and Hysteria2 use QUIC/UDP, requiring special handling through a SOCKS5 relay; QUIC connection migration also conflicts with proxy assumptions.
2. Carrying UDP through a TCP tunnel adds latency and encapsulation overhead.
3. NAT traversal is difficult and two running kernels create more failure points.

### Single-Kernel Switching

Only one kernel runs at a time. The node protocol determines the kernel; switching stops the old process and starts the new one.

Benefits: simpler and more reliable implementation, lower resource use (about 150 MB for one kernel), no UDP relay requirement, no additional proxy layer, and a single failure domain that is easier to debug.

Costs: switching takes about 2–3 seconds for validation and process restart, and unique protocols from both kernels cannot be used simultaneously.

### Expected Usage

Most users use common protocols such as SS, VMess and Trojan and do not switch kernels. Users of a unique protocol are unlikely to switch often; switching occurs when choosing a node, not per request. A 2–3 second delay is acceptable alongside connection establishment and can be communicated in the UI. The original comparison noted that v2rayA uses one kernel (Xray), while DuoRelay supports two kernels with automatic selection.

## Decision Consequences

### P2 Implementation Plan

1. Kernel selection retains the current kernel for nodes owned by both kernels and otherwise uses the node owner's preferred kernel:

   ```go
   func SelectKernel(node *ir.Node) ir.KernelType {
       if node.Owner == ir.KernelSetBoth {
           return currentKernel
       }
       return node.Owner.PreferredKernel()
   }
   ```

2. Switching detects the need, stops the current process, generates configuration, starts the target kernel, checks health and updates frontend status.
3. The UI shows switching progress, a loading state, a completion toast and each node's required kernel.

If there is strong demand for coexistence, a future V2.0 could offer a limited mode: TCP protocols coexist while UDP still triggers switching. It would be an advanced feature, disabled by default.

### Impact

- Generator creates configuration for one kernel at a time.
- Kernel manager uses single-kernel switching.
- API exposes kernel-switching operations.
- Frontend displays switching state and active kernel.
- Expected memory is about 150 MB for one kernel versus about 300 MB for both; CPU impact is negligible and switching takes 2–3 seconds.

Common-protocol use is unaffected. Unique protocols incur the switch delay, considered acceptable overall.

## Alternatives

- **Run both kernels fully in parallel:** rejected because UDP forwarding cannot be handled cleanly; separate ports do not resolve the relay issue.
- **Hybrid mode (TCP coexistence, UDP switching):** rejected because it adds complexity and is difficult to explain.

## References

RFC 1928, sing-box outbound detour documentation, mihomo proxy-groups documentation and QUIC protocol analysis.

Decision date: 2024-10-31. Decision owner: DuoRelay team. Review after the V1.0 release and user feedback.
