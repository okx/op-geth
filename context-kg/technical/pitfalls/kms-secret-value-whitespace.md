---
name: kms-secret-value-whitespace
description: KMS-returned secret values may contain trailing whitespace/newlines that break hex parsing
---

[Pitfall] KMS secret stores may return values with trailing whitespace or newlines. If not trimmed, `crypto.HexToECDSA` fails with a cryptic "invalid hex character" error. The JWT path (`obtainJWTSecretFromKMS`) trims via `common.FromHex` which handles whitespace, but the nodekey path (`setNodeKey`) passes raw KMS output directly to `crypto.HexToECDSA`.

**Trigger**: A KMS store value for `nodekeyhex` has a trailing `\n` (common in key-value stores). `strings.TrimPrefix(hexVal, "0x")` does not strip trailing whitespace. `crypto.HexToECDSA` fails.

**Correct**: Always trim KMS-returned values at the point of consumption, before any hex parsing:

```go
hexVal, err := kms.GetSecret(keyName)
if err != nil {
    Fatalf("KMS: failed to obtain nodekeyhex (key=%q): %v", keyName, err)
}
hexVal = strings.TrimSpace(hexVal)
hexVal = strings.TrimPrefix(hexVal, "0x")
key, err := crypto.HexToECDSA(hexVal)
```

[Rule] All KMS secret consumers must `strings.TrimSpace()` before parsing. Apply consistently across all `kms.GetSecret()` call sites.

**Module**: `cmd/utils/flags.go`, `node/node.go`, `internal/kms`
**Source**: Code Review R3-1 (A-08), XLOP-1113
**Date**: 2026-06-17
**Hit count**: 1
