---
name: kms-init-entry-point-coverage
description: Package-level Init() must be called from the most upstream common function to prevent silent fallback in forgotten entry points
---

[Pitfall] When a package requires explicit initialization before its feature-gated functions work (e.g., `kms.Init()` sets `enabled=true`), placing the Init call in only one entry point leaves other entry points silently falling back to "disabled" behavior — even when the feature is configured.

**Trigger**: Adding `kms.Init()` only to the `geth()` function while the `localConsole` entry point also calls `makeFullNode()` → `setNodeKey()`. With KMS env vars set but Init not run, `kms.Enabled()` returns `false` and secrets silently use file/flag paths — violating the "no silent fallback" requirement (PRD G-3).

**Correct**: Place `kms.Init()` in the most upstream common function that all secret-consuming paths share. In op-geth, this is `makeFullNode()` (cmd/geth/config.go), which is called by `geth`, `localConsole`, `ephemeralConsole`, and all future full-node entry points.

```go
// cmd/geth/config.go — top of makeFullNode()
func makeFullNode(ctx *cli.Context) *node.Node {
    if err := kms.Init(); err != nil {
        utils.Fatalf("KMS initialization failed: %v", err)
    }
    // ... rest of node creation
}
```

[Rule] When adding package-level initialization that gates feature behavior via a bool/flag, always place the Init call at the most upstream common ancestor in the call graph — never at individual CLI entry points.

**Module**: `cmd/geth`, `internal/kms`
**Source**: Adversarial Review F-01 (A-15), XLOP-1113
**Date**: 2026-06-17
**Hit count**: 1
