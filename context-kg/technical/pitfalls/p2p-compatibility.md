---
name: "p2p-compatibility"
description: "Pitfalls related to P2P protocol compatibility and ETH69 filtering"
---
# P2P Compatibility Pitfalls

[Pitfall] **ETH69 stripped only by client-name heuristic**: `transport_xlayer.go` filters ETH69 for peers whose handshake `Name` starts with the `Geth/` prefix (`strings.HasPrefix(name, "Geth/")`). This correctly excludes fork nodes (`l2-geth`, `op-geth`) but any upstream Geth build that changes its `NodeName` prefix away from `Geth/` will bypass the filter. **Residual risk**: Non-`Geth/`-prefixed ETH68-only peers will not be filtered. **Correct approach**: Use protocol-version capability negotiation rather than name heuristic. **History**: Previously used substring matching (`strings.Contains`) which false-positived on fork names containing "geth", causing fork-to-fork peering failures (XLOP-1116, 2026-06-15). Source: `isGeth()` in `p2p/transport_xlayer.go`. Affected module: p2p. **Hit count**: 1

[Pitfall] **ETH69 filter modifies remote handshake copy**: `trimETH69` is called on both outgoing caps AND received remote handshake. Modifying the parsed remote handshake means recorded peer capabilities won't match what was actually sent. **Correct approach**: Only filter outgoing handshake; accept remote as-is. Source: `p2p/transport_xlayer.go:25,37`. Affected module: p2p.

[Warning] **ETH69 compatibility filter is fragile**: Name-based filtering is inherently brittle — any client not using the `Geth/` prefix will bypass the filter. The prefix match is more precise than the prior substring match but remains a heuristic. Source: `p2p/transport_xlayer.go`. Affected module: p2p.
