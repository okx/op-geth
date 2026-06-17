---
name: ADR-002-kms-init-placement-make-full-node
description: kms.Init() placed in makeFullNode() rather than individual entry points to cover all full-node paths
---

# ADR-002: KMS Init Placement in makeFullNode()

**Status**: Accepted
**Date**: 2026-06-17

## Context

op-geth has multiple CLI entry points that create a full node: `geth` (main), `localConsole`, `ephemeralConsole`, and potentially future commands. Each calls `makeFullNode()` which triggers config loading and secret resolution. The KMS SDK must be initialized before any `kms.Enabled()` / `kms.GetSecret()` call, otherwise secrets silently fall back to file/flag paths.

The initial TD design placed `kms.Init()` in `geth()` only. Adversarial review (A-15 Major #1) identified that `localConsole` also creates a full node with all secrets, but would never call Init — violating the "no silent fallback" requirement.

## Decision

Place `kms.Init()` at the top of `makeFullNode()` (cmd/geth/config.go) with `Fatalf` on failure:

```go
func makeFullNode(ctx *cli.Context) *node.Node {
    if err := kms.Init(); err != nil {
        utils.Fatalf("KMS initialization failed: %v", err)
    }
    // ...
}
```

This ensures all current and future entry points that build a full node automatically get KMS initialization.

## Consequences

- **Positive**: Single initialization point — no entry point can accidentally skip KMS
- **Positive**: Future entry points calling `makeFullNode()` automatically get KMS support
- **Positive**: `Init()` is idempotent (ADR-001), so duplicate calls are harmless
- **Negative**: Lightweight commands that don't need KMS but hypothetically call `makeFullNode()` would also trigger Init (currently none exist)

**Source**: Adversarial Review F-01 (A-15) + TDD Summary (A-06), XLOP-1113
