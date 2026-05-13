---
name: "consensus"
description: "Module design for consensus: Engine interface, beacon, clique, ethash implementations"
---
# Consensus Module

## Responsibilities

- Defines the `Engine` interface: `Author`, `VerifyHeader`, `VerifyUncles`, `Prepare`, `Finalize`, `FinalizeAndAssemble`, `Seal`, `SealHash`, `CalcDifficulty`
- Beacon consensus engine (post-merge, OP Stack default)
- Clique consensus engine (PoA for testnets/dev chains)
- Ethash consensus engine (legacy PoW)
- EIP-1559 misc utilities including Optimism-specific base fee calculation

## NOT Responsible For

- Block import or chain management (delegated to `core.BlockChain`)
- P2P networking or sync
- Database access (only receives necessary data through function parameters)

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Engine` | interface | Core consensus algorithm interface |
| `beacon.Beacon` | `ethone Engine` | Beacon consensus wrapping pre-merge engine |
| `clique.Clique` | `config`, `signatures` | PoA consensus engine |
| `ethash.Ethash` | `config`, `caches`, `datasets` | PoW consensus engine (legacy) |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Rule] Consensus engine implementations must never call `eth`, `miner`, or `rpc` packages — only `core/state`, `core/types`, `core/vm`, `params` imports allowed.
