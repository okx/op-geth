---
name: "p2p"
description: "Module design for p2p: networking, peer discovery, RLPx transport, XLayer protocol compat"
---
# P2P Module

## Responsibilities

- TCP/UDP peer connections
- RLPx handshake and protocol multiplexing
- Discovery v4 and v5
- Dial scheduling and peer management
- XLayer transport compatibility: ETH69 capability stripping for Geth peers

## NOT Responsible For

- Ethereum block/transaction types
- Chain logic or state management
- RPC serving

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Server` | `Config`, `listener`, `discmix`, `peers` | Main P2P server |
| `Config` | `MaxPeers`, `Protocols`, `BootstrapNodes`, `StaticNodes` | Server configuration |
| `Peer` | `rw`, `caps`, `log` | Connected peer |
| `transport_xlayer` | `eth69CompatEnabled`, `eth69TrimmedCounter`, `SetETH69Compat`, `trimETH69Counted`, `doProtoHandshakeLegacy`, `isGeth` | XLayer ETH69 compat layer: flag-gated trim + counter metric |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] ETH69 stripped only by client-name heuristic: `isGeth()` checks for "Geth"/"geth" substring in peer name — custom forks not matching will negotiate ETH69 and may break. Source: `p2p/transport_xlayer.go`.

[Pitfall] **FIXED (XLOP-1045)**: `trimETH69Counted` now returns a copy of the handshake struct — original `protoHandshake` is never mutated. The caller reassigns the `their` local variable. Previously `trimETH69` modified the struct in place. Source: `p2p/transport_xlayer.go:66-82`.

[Warning] ETH69 compatibility filter is name-based heuristic — forks or custom builds will not be filtered. Behavior is now flag-gated via `--p2p.eth69-compat` (default: true = trim active). Source: `p2p/transport_xlayer.go`.
