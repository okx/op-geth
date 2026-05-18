---
name: "terminology"
description: "Domain term glossary for unified terminology across backend skills"
---
# Domain Terminology

| Term | Description | References |
|------|-------------|------------|
| `AccessListTxType` | Transaction type 0x01 — EIP-2930 access list transaction | `core/types/transaction.go` |
| `Address` | 20-byte Ethereum address type (AddressLength=20) | `common/types.go` |
| `AltDAConfig` | Alternative Data Availability config: challenge contract, windows, commitment type | `superchain/types.go` |
| `BatchInboxAddr` | Address where sequencer submits batches on L1 | `superchain/types.go` |
| `Bedrock` | First major OP Stack hardfork; activation tracked by `BedrockBlock` (block number) | `params/config_op.go` |
| `BlobTxType` | Transaction type 0x03 — EIP-4844 blob-carrying transaction | `core/types/transaction.go` |
| `BlockConfig` | Per-block feature flags struct; tracks `IsIsthmusEnabled` | `core/types/block_config.go` |
| `BlockContext` | EVM auxiliary block-level info: Coinbase, GasLimit, BlockNumber, Time, BaseFee, L1CostFunc, OperatorCostFunc, SlotNum | `core/vm/evm.go` |
| `BlockNonce` | 64-bit hash field proving PoW computation; legacy, unused in OP Stack | `core/types/block.go` |
| `Canyon` | OP Stack hardfork; introduces updated deposit receipt hash computation (version 1) | `params/config_op.go` |
| `CanyonDepositReceiptVersion` | Receipt version 1 for post-Canyon deposit receipts | `core/types/receipt.go` |
| `ChainConfig` | Chain configuration struct containing all hardfork activation blocks/timestamps plus Optimism and XLayer config | `params/config.go` |
| `DAFootprintGasScalar` | DA footprint gas scalar; introduced in Jovian hardfork | `core/types/receipt.go` |
| `DataAvailabilityType` | String field specifying DA type (calldata, blobs, alt-da) | `superchain/types.go` |
| `Delta` | OP Stack hardfork | `superchain/types.go` |
| `DepositNonce` | Actual nonce used by a deposit tx, stored in receipt since Regolith | `core/types/receipt.go` |
| `DepositReceiptVersion` | Version field indicating updated hash computation for post-Canyon deposits | `core/types/receipt.go` |
| `DynamicFeeTxType` | Transaction type 0x02 — EIP-1559 base fee + tip transaction | `core/types/transaction.go` |
| `Ecotone` | OP Stack hardfork; introduces L1 blob fee and updated fee scalars | `params/config_op.go` |
| `Engine` | Consensus engine interface: VerifyHeader, VerifyUncles, Prepare, Finalize, Seal | `consensus/consensus.go` |
| `ErrAlreadyKnown` | Txpool error: transaction already in pool | `core/txpool/errors.go` |
| `ErrAlreadyReserved` | Txpool error: sender has pending tx in a different subpool | `core/txpool/errors.go` |
| `ErrDepth` | EVM error: max call depth exceeded | `core/vm/errors.go` |
| `ErrExecutionReverted` | EVM error: REVERT opcode | `core/vm/errors.go` |
| `ErrGasLimit` | Txpool error: tx gas limit exceeds block gas limit | `core/txpool/errors.go` |
| `ErrOutOfGas` | EVM error: gas exhausted | `core/vm/errors.go` |
| `ErrReplaceUnderpriced` | Txpool error: replacement tx does not meet price bump | `core/txpool/errors.go` |
| `ErrTxGasPriceTooLow` | Txpool error: gas price below configured minimum | `core/txpool/errors.go` |
| `ErrUnderpriced` | Txpool error: gas price too low for pool minimum | `core/txpool/errors.go` |
| `ErrWriteProtection` | EVM error: state-modifying call in static context | `core/vm/errors.go` |
| `ExecutableData` | Engine API struct for execution payload data | `beacon/engine/types.go` |
| `Fjord` | OP Stack hardfork; deprecates `L1GasUsed` | `params/config_op.go` |
| `Granite` | OP Stack hardfork | `params/config_op.go` |
| `HardforkConfig` | Superchain config listing all OP fork timestamps | `superchain/types.go` |
| `Hash` | 32-byte Keccak256 hash type (HashLength=32) | `common/types.go` |
| `Header` | Block header struct containing all consensus fields | `core/types/block.go` |
| `Holocene` | OP Stack hardfork; introduces `EIP1559Params` in PayloadAttributes | `params/config_op.go` |
| `Interop` | OP Stack hardfork enabling cross-chain interoperability | `params/config_op.go` |
| `Isthmus` | OP Stack hardfork; introduces operator fees and withdrawals root | `params/config_op.go` |
| `Jovian` | OP Stack hardfork; introduces DA footprint gas scalar; repurposes BlobGasUsed for DA footprint | `params/config_op.go` |
| `Karst` | OP Stack hardfork (XLayer-specific designation) | `params/config_op.go` |
| `L1BaseFeeScalar` | Scalar applied to L1 base fee for cost calc; introduced in Ecotone | `core/types/receipt.go` |
| `L1BlobBaseFee` | Blob base fee on L1; nil prior to Ecotone | `core/types/receipt.go` |
| `L1BlobBaseFeeScalar` | Scalar applied to L1 blob base fee for cost calc; introduced in Ecotone | `core/types/receipt.go` |
| `L1CostFunc` | Function type returning L1 rollup message cost; part of BlockContext | `core/vm/evm.go` |
| `L1Fee` | L1 data availability fee charged to the transaction | `core/types/receipt.go` |
| `L1GasPrice` | L1 base fee present in receipt; L1 basefee after Bedrock | `core/types/receipt.go` |
| `L1GasUsed` | L1 gas used for DA; deprecated as of Fjord | `core/types/receipt.go` |
| `LegacyTxType` | Transaction type 0x00 — original pre-EIP-2718 transaction format | `core/types/transaction.go` |
| `MaxSequencerDrift` | Maximum seconds sequencer timestamp may drift ahead of L1 | `superchain/types.go` |
| `MigrationConfig` | Config for routing pre-migration blocks to legacy XLayer-Erigon RPC | `eth/ethconfig/config_xlayer.go` |
| `MonitorConfig` | Config for transaction trace logging | `eth/ethconfig/config_xlayer.go` |
| `NoTxPool` | PayloadAttributes flag; when true, only forced Transactions list is used | `beacon/engine/types.go` |
| `OpCode` | Single byte representing an EVM opcode | `core/vm/opcodes.go` |
| `OperatorCostFunc` | Function type returning operator cost; OP Stack addition (Isthmus+) | `core/vm/evm.go` |
| `OperatorFeeConstant` | Constant component of operator fee; introduced in Isthmus | `core/types/receipt.go` |
| `OperatorFeeScalar` | Scalar component of operator fee; introduced in Isthmus | `core/types/receipt.go` |
| `OptimismConfig` | Optimism-specific fee config: EIP1559Elasticity, EIP1559Denominator, EIP1559DenominatorCanyon | `params/config_op.go` |
| `PayloadAttributes` | Engine API struct: Timestamp, Random, SuggestedFeeRecipient, Withdrawals, BeaconRoot, Transactions, NoTxPool, GasLimit, EIP1559Params, MinBaseFee | `beacon/engine/types.go` |
| `PayloadVersion` | Byte identifier for ExecutionPayload version (V1-V4); V4 adds slotNumber (EIP-7843) | `beacon/engine/types.go` |
| `Receipt` | Transaction execution result with consensus, implementation, and OP Stack L1/operator fee fields | `core/types/receipt.go` |
| `Regolith` | OP Stack hardfork after Bedrock; introduces deposit receipt nonce storage | `params/config_op.go` |
| `RolesConfig` | Superchain roles: SystemConfigOwner, ProxyAdminOwner, Guardian, Challenger, Proposer, UnsafeBlockSigner, BatchSubmitter | `superchain/types.go` |
| `RollupCostFunc` | Function type computing rollup (L1+operator) cost for a transaction | `core/txpool/rollup.go` |
| `RollupTransaction` | Interface extending `types.RollupTransaction` with `Cost()` | `core/txpool/rollup.go` |
| `SendRawTransactionConditional` | Sequencer API method for submitting tx with inclusion preconditions | `internal/sequencerapi/api.go` |
| `SeqWindowSize` | Maximum number of L1 blocks a sequencer can be ahead | `superchain/types.go` |
| `SetCodeTxType` | Transaction type 0x04 — EIP-7702 set-code transaction | `core/types/transaction.go` |
| `Superchain` | Struct representing OP superchain network config | `superchain/superchain.go` |
| `SystemConfig` | Superchain on-chain system configuration: BatcherAddr, Overhead, Scalar, GasLimit, BaseFeeScalar, BlobBaseFeeScalar | `superchain/types.go` |
| `Transaction` | Ethereum transaction wrapper with inner TxData plus caches (hash, size, from, rollupCostData, conditional) | `core/types/transaction.go` |
| `TransactionConditional` | Optional preconditions for conditional tx inclusion | `core/types/transaction.go` |
| `TxData` | Interface for underlying tx data; implemented by LegacyTx, DynamicFeeTx, AccessListTx, BlobTx, SetCodeTx | `core/types/transaction.go` |
| `TxIndexingError` | ethapi error indicating tx indexing still in progress; JSON-RPC code -32000 | `internal/ethapi/errors.go` |
| `VMError` | VM execution error with stable integer error code | `core/vm/errors.go` |
| `P2PConfig` | XLayer P2P configuration sub-struct containing `ETH69Compat` boolean field | `eth/ethconfig/config_xlayer.go` |
| `SetETH69CompatEnabled` | Sets the runtime atomic flag controlling ETH69 capability stripping; called during startup wiring | `p2p/transport_xlayer.go` |
| `XLayerConfig` | XLayer-specific eth backend config: LegacyPp (migration), Monitor (trace log), and P2P (ETH69 compat) | `eth/ethconfig/config_xlayer.go` |
| `XLayerForkConfig` | Hardcoded fork time overrides for XLayer chains | `params/config_xlayer.go` |
| `XLayerMainnetChainID` | Chain ID 196 for XLayer mainnet | `params/config_xlayer.go` |
| `XLayerTestnetChainID` | Chain ID 1952 for XLayer testnet (Sepolia-based) | `params/config_xlayer.go` |
| `revertError` | ethapi error wrapping EVM revert with JSON-RPC error code 3 and hex revert reason | `internal/ethapi/errors.go` |

## Synonyms

| Term A | Term B | Context |
|--------|--------|---------|
| `BaseFee` | `baseFeePerGas` | `types.Header` Go field vs JSON name |
| `BlobGasUsed` (header) | DA footprint | `core/types/block.go` — OP Stack repurposes post-Jovian |
| `Coinbase` | `miner` | `types.Header` Go field vs JSON name |
| `DepositTx` | `SystemTx` | `core/types/transaction.go` — L1-originated forced transactions |
| `Extra` | `extraData` | `types.Header` Go field vs JSON name |
| `FeeScalar` | `L1FeeScalar` | `Receipt.FeeScalar` has JSON tag `l1FeeScalar` |
| `L1GasPrice` | L1 Basefee | Same value, different naming pre/post Bedrock |
| `NetworkId` | `ChainID` | `eth/ethconfig/config.go` — when left zero, chain ID is used |
| `PPRPCUrl` | XLayer-Erigon RPC endpoint | `ethconfig.MigrationConfig` field |
| `ParentBeaconRoot` | `parentBeaconBlockRoot` | `types.Header` Go field vs JSON name |
| `Random` | `prevRandao` | `engine.PayloadAttributes` Go field vs JSON name |
| `ReceiptHash` | `receiptsRoot` | `types.Header` Go field vs JSON name |
| `Root` | `stateRoot` | `types.Header` Go field vs JSON name |
| `Time` | `timestamp` | `types.Header` Go field vs JSON name |
| `TxHash` | `transactionsRoot` | `types.Header` Go field vs JSON name |
| `UncleHash` | `sha3Uncles` | `types.Header` Go field vs JSON name |
| `WithdrawalsHash` | `withdrawalsRoot` | `types.Header` Go field vs JSON name |
