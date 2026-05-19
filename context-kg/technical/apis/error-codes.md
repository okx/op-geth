---
name: "error-codes"
description: "Error code registry for JSON-RPC, ethapi, sequencer API, and VM error codes"
---
# Error Codes

## JSON-RPC Standard Codes (rpc package)

| Code | Name | Description |
|------|------|-------------|
| -32700 | `parseError` | JSON parse failure |
| -32600 | `invalidRequestError` | Invalid JSON-RPC request |
| -32601 | `methodNotFoundError` | Method not found or notifications unsupported |
| -32602 | `invalidParamsError` | Invalid parameters |
| -32603 | `errcodePanic` / `errcodeMarshalError` | Internal/marshal panic |
| -32000 | `errcodeDefault` | Generic server error (also `TxIndexingError`) |
| -32002 | `errcodeTimeout` | Request timed out |
| -32003 | `errcodeResponseTooLarge` | Response too large |

## EthAPI Error Codes (internal/ethapi)

| Code | Name | Description |
|------|------|-------------|
| 3 | `revertError` | EVM execution reverted; `ErrorData()` returns hex ABI revert reason |
| 4 | `txSyncTimeoutError` | `SendRawTransactionSync` timed out; `ErrorData()` returns tx hash |
| -32000 | `TxIndexingError` | Transaction indexing in progress |
| -32015 | `errCodeVMError` | VM execution error (distinct from revert) |
| -32602 | `errCodeInvalidParams` | Invalid parameters |
| -32603 | `errCodeInternalError` | Internal error fallback |
| -32801 | `NoHistoricalFallbackError` | No historical RPC available for pre-bedrock |
| -38010 | `errCodeNonceTooLow` | Nonce too low |
| -38011 | `errCodeNonceTooHigh` | Nonce too high |
| -38013 | `errCodeIntrinsicGas` | Intrinsic gas too low |
| -38014 | `errCodeInsufficientFunds` | Insufficient funds |
| -38015 | `errCodeBlockGasLimitReached` | Block gas limit reached (simulate) |
| -38020 | `errCodeBlockNumberInvalid` | Invalid block number (simulate) |
| -38021 | `errCodeBlockTimestampInvalid` | Invalid block timestamp (simulate) |
| -38024 | `errCodeSenderIsNotEOA` | Sender is not EOA |
| -38025 | `errCodeMaxInitCodeSizeExceeded` | Max initcode size exceeded |
| -38026 | `errCodeClientLimitExceeded` | Client limit exceeded |

## Sequencer API Error Codes (internal/sequencerapi)

| Code | Name | Description |
|------|------|-------------|
| -32003 | `TransactionConditionalRejectedErrCode` | Conditional tx rejected (header/state check failed) |
| -32005 | `TransactionConditionalCostExceededMaxErrCode` | Conditional tx cost exceeded max or rate limited |

## VM Error Codes (core/vm)

| Code | Name | Description |
|------|------|-------------|
| 1 | `VMErrorCodeOutOfGas` | Out of gas |
| 2 | `VMErrorCodeCodeStoreOutOfGas` | Code store out of gas |
| 3 | `VMErrorCodeDepth` | Max call depth exceeded |
| 4 | `VMErrorCodeInsufficientBalance` | Insufficient balance |
| 5 | `VMErrorCodeContractAddressCollision` | Contract address collision |
| 6 | `VMErrorCodeExecutionReverted` | Execution reverted (REVERT opcode) |
| 7 | `VMErrorCodeMaxCodeSizeExceeded` | Max code size exceeded |
| 8 | `VMErrorCodeInvalidJump` | Invalid jump destination |
| 9 | `VMErrorCodeWriteProtection` | Write protection (static call) |
| 10 | `VMErrorCodeReturnDataOutOfBounds` | Return data out of bounds |
| 11 | `VMErrorCodeGasUintOverflow` | Gas uint overflow |
| 12 | `VMErrorCodeInvalidCode` | Invalid code |
| 13 | `VMErrorCodeNonceUintOverflow` | Nonce uint overflow |
| 14 | `VMErrorCodeStackUnderflow` | Stack underflow |
| 15 | `VMErrorCodeStackOverflow` | Stack overflow |
| 16 | `VMErrorCodeInvalidOpCode` | Invalid opcode |
| MaxInt-1 | `VMErrorCodeUnknown` | Unrecognized VM error |
