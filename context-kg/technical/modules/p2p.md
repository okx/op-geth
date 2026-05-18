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
| `transport_xlayer` | `eth69CompatEnabled`, `eth69TrimCounter`, `doProtoHandshakeLegacy`, `trimETH69`, `SetETH69CompatEnabled`, `isGeth` | XLayer ETH69 compat layer with runtime-disableable flag and operational metrics |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] ETH69 stripped only by client-name heuristic: `isGeth()` checks for "Geth"/"geth" substring in peer name — custom forks not matching will negotiate ETH69 and may break. Feature is runtime-disableable via `--p2p.eth69-compat=false`. Source: `p2p/transport_xlayer.go`.

[Pitfall] `trimETH69` filters both outgoing caps AND received remote handshake — uses copy semantics (shallow copy of `*protoHandshake`), never mutates original. Two-pass: scan for eth/69 presence first, allocate only if found. Returns same pointer on no-op. Source: `p2p/transport_xlayer.go:66-82`.

[Pitfall] P2P metric counter (`eth69TrimCounter`) must increment only after successful `Send()` — placing it before or unconditionally inflates dashboards. Source: `p2p/transport_xlayer.go:48-54`.

[Warning] ETH69 compatibility filter is name-based heuristic — forks or custom builds will not be filtered. Runtime-disableable via `--p2p.eth69-compat` flag (default true). Source: `p2p/transport_xlayer.go`.
