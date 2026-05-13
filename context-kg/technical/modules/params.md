---
name: "params"
description: "Module design for params: chain parameters, fork configurations, OP Stack and XLayer overrides"
---
# Params Module

## Responsibilities

- Chain configuration (`ChainConfig`): all hardfork activation blocks and timestamps
- OP Stack fork parameters: Bedrock, Regolith, Canyon, Delta, Ecotone, Fjord, Granite, Holocene, Isthmus, Jovian, Karst, Interop
- Optimism-specific fee configuration (`OptimismConfig`)
- XLayer fork overrides (`XLayerForkConfig`): hardcoded fork times for chainID 196/1952
- Chain ID constants: XLayerMainnetChainID (196), XLayerTestnetChainID (1952), OPMainnetChainID (10), BaseMainnetChainID (8453)
- Network bootnodes and protocol version constants

## NOT Responsible For

- Fork ID computation (handled by `core/forkid`)
- Chain state management
- Consensus implementation

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `ChainConfig` | `ChainID`, all fork block/timestamp fields, `Optimism *OptimismConfig` | Master chain config |
| `OptimismConfig` | `EIP1559Elasticity`, `EIP1559Denominator`, `EIP1559DenominatorCanyon` | OP fee params |
| `XLayerForkConfig` | `JovianTime *uint64` | Hardcoded XLayer fork overrides |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] `ApplyXLayerHardcodedForks` unconditionally replaces DB `JovianTime` with hardcoded value even if DB value differs — silently re-enables a fork that was purposely deactivated. Source: `params/config_xlayer.go:63-69`.

[Warning] `XLayerHardcodedForks` only covers `JovianTime` — Karst and future forks require explicit code additions. Source: `params/config_xlayer.go`.

[Pitfall] `gatherForksXLayer` uses reflection to collect ALL `*uint64` fields with "Time" suffix — non-fork timestamp fields added in future upstreams will corrupt fork-ID checksum. Source: `core/forkid/forkid_xlayer.go:152-177`.

[Pitfall] `forkid_xlayer` fallback accepts on impossible validation — returns nil instead of `ErrLocalIncompatibleOrStale` in theoretically-impossible case. Source: `core/forkid/forkid_xlayer.go:145-146`.
