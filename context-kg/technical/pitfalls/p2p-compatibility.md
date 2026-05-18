---
name: "p2p-compatibility"
description: "Pitfalls related to P2P protocol compatibility, ETH69 filtering, and p2p metric correctness"
---
# P2P Compatibility Pitfalls

[Pitfall] **ETH69 stripped only by client-name heuristic**: `transport_xlayer.go` filters ETH69 for peers whose handshake `Name` contains "Geth" or "geth" (case-sensitive). Custom Geth forks that don't include "Geth" in their name will negotiate ETH69 and may break. **Correct approach**: Use protocol-version capability negotiation rather than name heuristic. Source: `isGeth()` in `p2p/transport_xlayer.go`. Affected module: p2p.

[Pitfall] **ETH69 filter creates copy — never mutate original handshake**: `trimETH69` is called on both outgoing caps AND received remote handshake. The function returns a shallow-copied `*protoHandshake` with filtered `Caps` slice; the original is never mutated. When eth/69 is not present, returns same pointer (zero allocation). **Trigger**: Adding new cap-filtering logic that modifies the handshake in place instead of copying. **Correct approach**: Always use copy semantics (`filteredHandshake := *phs`) and return new pointer; use two-pass scan-before-allocate to avoid heap pressure when no filtering is needed. Source: `p2p/transport_xlayer.go:66-82`. Affected module: p2p. **Date**: 2026-05-18. **Hit count**: 1

[Pitfall] **P2P metric counter must increment only after successful Send**: `eth69TrimCounter.Inc(1)` must be placed AFTER `Send()` returns nil. If placed before Send or unconditionally, network failures inflate operational dashboards with false-positive trim counts. **Trigger**: Adding new p2p metrics that count operations involving network I/O. **Correct approach**: Always gate counter increment on successful I/O completion: `if err := Send(...); err != nil { return err } counter.Inc(1)`. Source: `p2p/transport_xlayer.go:48-54`. Affected module: p2p. **Date**: 2026-05-18. **Hit count**: 1

[Pitfall] **Atomic feature flags need init() safe defaults**: Go `atomic.Bool` flags used as runtime feature toggles (e.g., `eth69CompatEnabled`) must set their production-safe default in an `init()` function. Test binaries and integration tests may not call the configuration setup path (`SetETH69CompatEnabled`), leaving the zero value (false) active — which silently disables the feature. **Trigger**: Adding new `atomic.Bool`/`atomic.Value` feature flags in packages called before CLI config wiring. **Correct approach**: Always define `func init() { flag.Store(productionDefault) }` in the same file as the flag declaration. Source: `p2p/transport_xlayer.go:18-20`. Affected module: p2p. **Date**: 2026-05-18. **Hit count**: 1

[Warning] **ETH69 compatibility filter is fragile**: Name-based filtering is inherently brittle — any fork or custom build not matching the substring will bypass the filter. The feature is runtime-disableable via `--p2p.eth69-compat=false` flag. Source: `p2p/transport_xlayer.go`. Affected module: p2p.
