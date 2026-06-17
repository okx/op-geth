---
name: app-before-global-init
description: Global preconditions (e.g. KMS init) must use app.Before, not a single subcommand function
---

# Global Initialization Must Use app.Before

[Pitfall] Placing global initialization logic (e.g., `kms.InitFromEnv()`) inside a specific subcommand function like `geth()` means other subcommands (`console`, `attach`, `dumpconfig`, etc.) silently bypass it. This creates a security gap where some entry points lack required preconditions.

**Trigger**: Adding startup-time initialization that should apply to ALL subcommands but placing it in only one subcommand's handler function.

**Correct**:
```go
// In cmd/geth/main.go — use app.Before for global preconditions
app.Before = func(ctx *cli.Context) error {
    // This runs before ANY subcommand action
    if err := kms.InitFromEnv(); err != nil {
        utils.Fatalf("KMS initialization failed: %v", err)
    }
    // ... other global setup ...
    return nil
}
```

[Rule] Any initialization that is a security precondition or must apply to all CLI entry points MUST be placed in `app.Before`, NOT in individual subcommand handlers. Test by verifying the init runs when invoking non-default subcommands (e.g., `geth console`).

**Module**: `cmd/geth/main.go`
**Source**: Code Review Stage 3.1 — Major finding #2 (XLOP-1113)
**Date**: 2026-06-17
**Hit count**: 1
