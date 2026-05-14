---
name: "p2p-compatibility"
description: "Pitfalls related to P2P protocol compatibility and ETH69 filtering"
---
# P2P Compatibility Pitfalls

[Pitfall] **ETH69 stripped only by client-name heuristic**: `transport_xlayer.go` filters ETH69 for peers whose handshake `Name` contains "Geth" or "geth" (case-sensitive). Custom Geth forks that don't include "Geth" in their name will negotiate ETH69 and may break. **Correct approach**: Use protocol-version capability negotiation rather than name heuristic. Source: `isGeth()` in `p2p/transport_xlayer.go`. Affected module: p2p.

[Pitfall] **ETH69 filter modifies remote handshake copy**: `trimETH69` is called on both outgoing caps AND received remote handshake. Modifying the parsed remote handshake means recorded peer capabilities won't match what was actually sent. **Correct approach**: Only filter outgoing handshake; accept remote as-is. Source: `p2p/transport_xlayer.go:25,37`. Affected module: p2p.

[Warning] **ETH69 compatibility filter is fragile**: Name-based filtering is inherently brittle — any fork or custom build not matching the substring will bypass the filter. Source: `p2p/transport_xlayer.go`. Affected module: p2p.
