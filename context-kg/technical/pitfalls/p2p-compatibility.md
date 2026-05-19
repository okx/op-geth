---
name: "p2p-compatibility"
description: "Pitfalls related to P2P protocol compatibility, ETH69 filtering, and handshake ordering"
---
# P2P Compatibility Pitfalls

[Pitfall] **ETH69 stripped only by client-name heuristic**: `transport_xlayer.go` filters ETH69 for peers whose handshake `Name` contains "Geth" or "geth" (case-sensitive). Custom Geth forks that don't include "Geth" in their name will negotiate ETH69 and may break. **Correct approach**: Use protocol-version capability negotiation rather than name heuristic. Source: `isGeth()` in `p2p/transport_xlayer.go`. Affected module: p2p.

[Pitfall] **ETH69 two-phase trim design — outgoing trim is global, not per-peer**: `TrimOurHandshakeCaps` removes eth/69 from `srv.ourHandshake.Caps` at startup for ALL peers (not just Geth peers). This means non-Geth peers also receive our handshake without eth/69 when trim is enabled. Acceptable tradeoff for a temporary compat shim, but unexpected if you assume per-peer outgoing filtering. **Correct approach**: Understand the two-phase split — Phase A (outgoing) is global at startup; Phase B (incoming via `doProtoHandshakeWithXLayerFilter`) is per-peer based on `isGeth`. Source: `TrimOurHandshakeCaps` + `doProtoHandshakeWithXLayerFilter` in `p2p/transport_xlayer.go`. Affected module: p2p.

[Pitfall] **Sequential read-then-send handshake causes protocol-level deadlock**: If both peers use a "read remote handshake first, then send ours" ordering, both block waiting for the other to send — a protocol-level deadlock that race detectors cannot catch. The original `doProtoHandshakeLegacy` had this bug. **Correct approach**: Always delegate to the upstream concurrent `doProtoHandshake` which sends and reads in parallel (goroutine for send, blocking read). Post-process the result after the concurrent exchange completes. Source: TD v1→v2 evolution, adversarial review Round 1 blocker. Affected module: p2p.

**Trigger**: Implementing a custom handshake wrapper that changes the send/read ordering of the upstream `doProtoHandshake`.

**Correct approach**:
```go
// CORRECT: delegate to upstream concurrent handshake, then post-filter
func doProtoHandshakeWithXLayerFilter(c transport, our *protoHandshake) (*protoHandshake, error) {
    their, err := c.doProtoHandshake(our) // upstream: concurrent send+read
    if err != nil {
        return nil, err
    }
    // Post-filter: safe because handshake already exchanged
    if eth69CompatEnabled.Load() && isGeth(their.Name) {
        filtered, removed := trimETH69Caps(their.Caps)
        if removed {
            their.Caps = filtered
            eth69TrimCounter.Inc(1)
        }
    }
    return their, nil
}
```

**Date**: 2026-05-19
**Hit count**: 1

[Warning] **ETH69 compatibility filter is fragile**: Name-based filtering is inherently brittle — any fork or custom build not matching the substring will bypass the filter. Source: `p2p/transport_xlayer.go`. Affected module: p2p.
