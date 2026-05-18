---
name: "p2p-compatibility"
description: "Pitfalls related to P2P protocol compatibility and ETH69 filtering"
---
# P2P Compatibility Pitfalls

[Pitfall] **ETH69 stripped only by client-name heuristic**: `transport_xlayer.go` filters ETH69 for peers whose handshake `Name` contains "Geth" or "geth" (case-sensitive). Custom Geth forks that don't include "Geth" in their name will negotiate ETH69 and may break. **Correct approach**: Use protocol-version capability negotiation rather than name heuristic. Source: `isGeth()` in `p2p/transport_xlayer.go`. Affected module: p2p.

[Pitfall] **ETH69 filter modifies remote handshake copy** — **FIXED (XLOP-1045)**: `trimETH69Counted` now returns a new struct copy with eth/69 removed; the original `*protoHandshake` is never mutated. The caller reassigns the local variable `their` to the returned copy. Both outgoing and incoming caps are trimmed (by design, so local record reflects negotiated protocol), but immutability of the original struct is guaranteed. **Correct approach (now implemented)**: Return a struct copy from the filter function; never mutate the input. Source: `p2p/transport_xlayer.go:66-82`. Affected module: p2p.

[Warning] **ETH69 compatibility filter is fragile**: Name-based filtering is inherently brittle — any fork or custom build not matching the substring will bypass the filter. Now controlled by `--p2p.eth69-compat` flag (default: true); set to false to disable the trim once upstream eth/69 interop is verified. Source: `p2p/transport_xlayer.go`. Affected module: p2p.
