---
name: "cmd-geth-flag-wiring"
description: "Pitfall: defining CLI flags and wiring functions without connecting them to the application startup path"
---
# cmd/geth Flag Wiring Pitfalls

[Pitfall] **CLI flag defined but never registered or called from startup path**: Defining a flag in `cmd/utils/flags_xlayer.go` with a `SetXLayer*Config` wiring function does NOT make it functional. The flag must be: (1) appended to the appropriate flag slice in `cmd/geth/main.go` (or `main_xlayer.go` via `init()`), AND (2) the wiring function must be called from `makeConfigNode` in `cmd/geth/config.go` (or `config_xlayer.go`). Missing either step means the flag exists in `--help` output but has zero runtime effect.

**Trigger**: Adding a new XLayer CLI flag to `cmd/utils/flags_xlayer.go` without verifying the full activation path through `cmd/geth`.

**Correct approach**:
```go
// Step 1: cmd/geth/main_xlayer.go — register flag
func init() {
    nodeFlags = append(nodeFlags, utils.P2PETH69CompatFlag)
}

// Step 2: cmd/geth/config_xlayer.go — apply during node config
func applyXLayerP2PConfig(ctx *cli.Context) {
    var cfg ethconfig.XLayerP2PConfig
    utils.SetXLayerP2PConfig(ctx, &cfg)
    p2p.SetETH69CompatEnabled(cfg.ETH69Compat)
}

// Step 3: cmd/geth/config.go — call from makeConfigNode
func makeConfigNode(ctx *cli.Context) (*node.Node, gethConfig) {
    // ... existing code ...
    applyXLayerP2PConfig(ctx)
    return stack, cfg
}
```

[Rule] Every new XLayer CLI flag MUST have a unit test that verifies the flag value reaches the target subsystem. Test pattern: create a `cli.Context` with the flag set, call the wiring function, assert the subsystem state changed.

**Module**: cmd/geth, cmd/utils
**Source**: TDD rework — `P2PETH69CompatFlag` defined but FR-2 (disable via `--p2p.eth69-compat=false`) was non-functional until rework added `main_xlayer.go` + `config_xlayer.go`.
**Date**: 2026-05-19
**Hit count**: 1
