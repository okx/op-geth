---
name: "cmd-geth"
description: "Module design for cmd/geth: main binary entry point, CLI flags, and utility commands"
---
# Cmd/Geth Module

## Responsibilities

- Main geth binary entry point
- CLI flag definitions for all node, txpool, sync, and OP-Stack hardfork override flags
- Configuration loading and merging (TOML config + CLI flags)
- XLayer-specific flags: migration config, monitor config, hybrid filter API registration
- Additional cmd utilities: abigen, abidump, clef, devp2p, evm, ethkey, rlpdump, era, keeper, workload

## NOT Responsible For

- Implementing any service logic
- Chain operations or consensus
- RPC method implementations

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `main()` | `app *cli.App` | CLI entry point |
| `gethConfig` | `Eth ethconfig.Config`, `Node node.Config` | Combined config struct |
| `flags_xlayer.go` | XLayer-specific flags | Migration, monitor, hybrid filter flags |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] `RegisterXlayerHybridFilterAPI` panics on Erigon connection failure — should return error. Source: `cmd/utils/flags_xlayer.go:88`.

[Rule] All OP-Stack hardfork override flags (Canyon, Ecotone, Fjord, Granite, Holocene, Isthmus, Jovian, Karst, Interop) must be registered as top-level CLI flags.
