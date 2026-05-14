---
name: "op-stack-hardforks"
description: "Pitfalls related to OP Stack hardfork validation and fork configuration"
---
# OP Stack Hardfork Pitfalls

[Pitfall] **checkOptimismPayloadAttributes panics on nil**: The function directly dereferences `payloadAttributes` pointer. Callers must guarantee non-nil before calling. **Correct approach**: Always guard with nil check at call site in `forkChoiceUpdated`. Source: `eth/catalyst/api_optimism.go:39`. Affected module: eth/catalyst.

[Pitfall] **EIP1559Params error message typo**: Error for non-empty EIP1559Params pre-Holocene says "eip155Params" (155, not 1559), which confuses log consumers and monitoring rules. **Correct approach**: Fix error string to "eip1559Params". Source: `eth/catalyst/api_optimism.go:59`. Affected module: eth/catalyst.

[Pitfall] **XLayer hardcoded fork always overwrites DB value**: `ApplyXLayerHardcodedForks` logs Warn but unconditionally replaces DB `JovianTime` with hardcoded value. On a chain that purposely deactivated a fork, the hardcoded value silently re-enables it. **Correct approach**: Require explicit operator acknowledgment or block startup on divergence. Source: `params/config_xlayer.go:63-69`. Affected module: params.

[Pitfall] **gatherForksXLayer collects ALL *uint64 "Time" fields**: Uses reflection to collect every `*uint64` field with "Time" suffix, including potential non-fork fields in future upstreams. **Correct approach**: Be vigilant when upstreaming `ChainConfig` changes; new timestamp fields corrupt fork-ID checksum. Source: `core/forkid/forkid_xlayer.go:152-177`. Affected module: core/forkid.

[Pitfall] **forkid_xlayer accepts on impossible validation**: When validation loop exhausts all forks without match (should be impossible), it returns nil (accept) instead of error. **Correct approach**: Return `ErrLocalIncompatibleOrStale` in the impossible case. Source: `core/forkid/forkid_xlayer.go:145-146`. Affected module: core/forkid.

[Warning] **XLayerHardcodedForks only covers JovianTime**: Karst and future forks require explicit code additions or they won't be hardcoded. Source: `params/config_xlayer.go`. Affected module: params.

[Pitfall] **Calling FCU V3 for non-Cancun/Prague/Osaka block**: Hard INVALID returned before any chain update; spec TODO notes FCU should still apply head update even with bad params. Source: `eth/catalyst/api.go:206-215`. Affected module: eth/catalyst.

[Pitfall] **Isthmus WithdrawalsRoot validation**: `checkOptimismPayload` returns error if `WithdrawalsRoot != nil` pre-Isthmus AND requires it post-Isthmus. Symmetrical but strict. Source: `eth/catalyst/api_optimism.go:27-33`. Affected module: eth/catalyst.
