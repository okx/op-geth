---
name: ADR-001-kms-init-mutex-over-sync-once
description: KMS package uses mu.Lock()+initialized bool instead of sync.Once to allow test reset
---

# ADR-001: KMS Init uses Mutex + Bool instead of sync.Once

**Status**: Accepted
**Date**: 2026-06-17

## Context

The `internal/kms` package needs idempotent initialization (`Init()` called multiple times is safe). `sync.Once` is the standard Go pattern for this, but it cannot be reset — making it impossible to test different Init() outcomes (success, failure, partial config) within a single test binary without process restart.

## Decision

Use `mu sync.Mutex` + `initialized bool` instead of `sync.Once`:
- `mu.Lock()` provides the same concurrency safety as `sync.Once`
- `initialized` flag provides idempotency in production (second `Init()` returns cached result)
- `ResetForTest()` (test-only) acquires `mu` and clears `initialized` + `enabled`, enabling isolated test scenarios

## Consequences

- **Positive**: Full testability — each test case can reset and re-initialize with different env vars / SDK behavior
- **Positive**: Same production guarantees as `sync.Once` (mutex-guarded, idempotent)
- **Negative**: Slightly more verbose than `sync.Once` (3 lines vs 1)
- **Negative**: `ResetForTest()` must be called in test setup to prevent state leakage between tests

**Source**: TDD Summary F-05 (A-06), XLOP-1113
