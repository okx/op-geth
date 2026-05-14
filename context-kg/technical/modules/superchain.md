---
name: "superchain"
description: "Module design for superchain: OP Stack superchain network configuration loading"
---
# Superchain Module

## Responsibilities

- Loads and caches per-network superchain config from embedded TOML
- Defines superchain types: `Superchain`, `ChainConfig`, `HardforkConfig`, `SystemConfig`, `RolesConfig`
- Protocol version addresses, superchain config addresses, OP contracts manager
- Chain-specific: `BatchInboxAddr`, `SeqWindowSize`, `MaxSequencerDrift`, `DataAvailabilityType`
- Alternative DA config (`AltDAConfig`)

## NOT Responsible For

- Chain state or consensus logic
- Runtime chain operations
- Block building or validation

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Superchain` | `ProtocolVersionsAddr`, `SuperchainConfigAddr`, `Hardforks`, `L1` | Superchain network |
| `ChainConfig` | `BatchInboxAddr`, `SeqWindowSize`, `MaxSequencerDrift`, `DataAvailabilityType` | Per-chain config |
| `HardforkConfig` | `CanyonTime` through `InteropTime` | Fork timestamps |
| `SystemConfig` | `BatcherAddr`, `Overhead`, `Scalar`, `GasLimit` | On-chain system config |
| `RolesConfig` | `SystemConfigOwner`, `Guardian`, `Challenger`, `Proposer`, `BatchSubmitter` | Chain roles |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

None identified.
