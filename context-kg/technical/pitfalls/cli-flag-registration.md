---
name: cli-flag-registration
description: urfave/cli flags must be registered in the command's Flags slice to be readable at runtime
---

# CLI Flag Registration

[Pitfall] Defining a `cli.StringFlag` (or any flag) in `cmd/utils/flags.go` is NOT sufficient to make it work. The flag MUST also be appended to the relevant command's `Flags` slice (e.g., `nodeFlags`, `rpcFlags`) in `cmd/geth/main.go`. Without registration, `ctx.String(flagName)` silently returns the zero value (empty string), not an error.

**Trigger**: Adding a new CLI flag to op-geth — defining the flag variable but forgetting to add it to the appropriate `Flags` slice in `main.go`.

**Correct**:
```go
// 1. Define the flag in cmd/utils/flags.go
KMSNodeKeyNameFlag = &cli.StringFlag{
    Name:  "kms.nodekey-name",
    Value: kms.DefaultNodeKeyHexKMSKey,
}

// 2. Register in the appropriate slice in cmd/geth/main.go
var nodeFlags = []cli.Flag{
    // ... existing flags ...
    utils.KMSNodeKeyNameFlag,  // <-- MUST be here
}
```

[Rule] Every new CLI flag definition MUST have a corresponding registration in the command Flags slice. Verify by checking that `ctx.String(flag.Name)` returns the expected default value in a test.

**Module**: `cmd/geth/main.go`, `cmd/utils/flags.go`
**Source**: Code Review Stage 3.1 — Major finding #1 (XLOP-1113)
**Date**: 2026-06-17
**Hit count**: 1
