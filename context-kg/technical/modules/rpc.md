---
name: "rpc"
description: "Module design for rpc: JSON-RPC framework, service registry, reflection-based dispatch"
---
# RPC Module

## Responsibilities

- JSON-RPC 2.0 framework implementation
- Reflection-based service registration and method dispatch
- Connection management (HTTP, WebSocket, IPC)
- Subscription support (eth_subscribe)
- Error code definitions and error interface contracts
- `BlockNumber` type with sentinel constants (Latest, Pending, Safe, Finalized, Earliest)

## NOT Responsible For

- Authentication (handled at node level via JWT)
- Ethereum-specific types or business logic
- Chain data access

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `serviceRegistry` | `services` map | Method → callback mapping |
| `Client` | `conn`, `handler` | JSON-RPC client for outbound calls |
| `BlockNumber` | int64 | Block number type with sentinels |
| `Error` interface | `Error()`, `ErrorCode()` | Custom RPC error contract |
| `DataError` interface | `ErrorData()` | Error with additional data field |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Rule] All custom JSON-RPC errors must implement `rpc.Error` interface (`Error() string` + `ErrorCode() int`); errors with extra data also implement `rpc.DataError`. Use standard JSON-RPC error codes (-32600 series) for protocol errors, -32000 for application errors.

[Rule] Use `rpc.BlockNumber` type for block-number RPC parameters; use named sentinel constants (`LatestBlockNumber`, `PendingBlockNumber`, etc.) instead of raw integers.
