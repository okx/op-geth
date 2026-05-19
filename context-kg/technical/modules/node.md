---
name: "node"
description: "Module design for node: service container, lifecycle management, RPC server hosting"
---
# Node Module

## Responsibilities

- Service container: lifecycle management for registered services
- RPC server setup: HTTP, WebSocket, IPC, authenticated (JWT) endpoints
- Account manager integration
- Database opener (chaindata, lightchaindata)
- p2p.Server configuration and embedding
- Built-in admin/debug/web3 RPC APIs

## NOT Responsible For

- Implementing chain or consensus logic
- Ethereum-specific types or business logic
- Block building or transaction management

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Node` | `config`, `server`, `accman`, `databases`, `lifecycles` | Service container |
| `Config` | `DataDir`, `HTTPHost`, `WSHost`, `AuthAddr`, `P2P` | Node configuration |
| `adminAPI` | `node *Node` | Built-in admin RPC: AddPeer, RemovePeer, etc. |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Rule] Node must register built-in admin/debug/web3 APIs before any service APIs — called in `node.apis()` during `New()`.
