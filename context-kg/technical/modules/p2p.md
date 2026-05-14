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
| `transport_xlayer` | `doProtoHandshakeLegacy`, `trimETH69` | XLayer ETH69 compat layer |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] ETH69 stripped only by client-name heuristic: `isGeth()` checks for "Geth"/"geth" substring in peer name — custom forks not matching will negotiate ETH69 and may break. Source: `p2p/transport_xlayer.go`.

[Pitfall] `trimETH69` modifies both outgoing caps AND received remote handshake — recorded peer capabilities will not match what was actually sent. Source: `p2p/transport_xlayer.go:25,37`.

[Warning] ETH69 compatibility filter is name-based heuristic — forks or custom builds will not be filtered. Source: `p2p/transport_xlayer.go`.
