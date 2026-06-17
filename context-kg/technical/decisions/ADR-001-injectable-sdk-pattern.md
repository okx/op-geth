---
name: ADR-001-injectable-sdk-pattern
description: Use injectable function variables when private SDK modules are unreachable in build/CI
---

# ADR-001: Injectable SDK Pattern for Unreachable Private Modules

**Status**: Accepted
**Date**: 2026-06-17

## Context

The `ok-kms-go v1.0.9` module is hosted on private GitLab (`gitlab.okg.com/okcoin-commons/ok-kms-go`) and is not accessible in CI/test environments. Direct import would make the package unbuildable and untestable outside the production infrastructure.

## Decision

Use package-level injectable function variables (`sdkInit func() error`, `sdkGetSecretValue func(string) (string, error)`) with a `RegisterSDK()` function for production wiring. Tests inject mocks directly; production code calls `RegisterSDK(okkms.Init, okkms.GetSecretValue)` in a build-tagged `init()` file.

## Consequences

- **Positive**: Package compiles and tests pass without access to private GitLab; full unit test coverage with injected mocks; no build-time dependency on network.
- **Positive**: Production wiring is a single `RegisterSDK()` call in an init file — minimal integration surface.
- **Negative**: Requires a build-tagged init file for production (must not be forgotten during deployment).
- **Negative**: `RegisterSDK()` must be called before `InitFromEnv()` — temporal coupling enforced by Go's `init()` ordering (package init runs before `main`).

## Alternatives Considered

1. **Add `replace` directive in go.mod** — ties all developers to a specific local path; breaks CI.
2. **Vendor the module** — licensing/compliance risk; stale copies.
3. **Interface-based abstraction** — heavier than needed for 2 functions; injectable functions are idiomatic for Go test doubles.

**Source**: TDD Development Stage 2.1 — Design Decision (XLOP-1113)
