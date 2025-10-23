package miner

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

const DefaultTxInfosSize = 20000

type RealtimeBackend interface {
	RealtimeEnabled() bool
	GetRealtimeBlockInfoChan() chan *realtimeTypes.BlockInfo
	GetRealtimeTxInfoChan() chan state.TxInfo
	SendRealtimeErrorTrigger(height uint64)
}

// snapshot creates a lightweight copy of the environment for incremental building.
// The EVM is not copied as it will be recreated when needed.
func (env *environment) snapshot() *environment {
	snap := &environment{
		signer:   env.signer,
		state:    env.state.Copy(),
		tcount:   env.tcount,
		gasPool:  new(core.GasPool).AddGas(env.gasPool.Gas()),
		coinbase: env.coinbase,
		evm:      nil,
		header:   types.CopyHeader(env.header),
		txs:      append([]*types.Transaction(nil), env.txs...),
		receipts: append([]*types.Receipt(nil), env.receipts...),
		sidecars: append([]*types.BlobTxSidecar(nil), env.sidecars...),
		blobs:    env.blobs,
		noTxs:    env.noTxs,
		rpcCtx:   env.rpcCtx,
		okPayTxs: env.okPayTxs,
	}
	if env.witness != nil {
		snap.witness = env.witness.Copy()
	}
	return snap
}

func (miner *Miner) tryIncrementalUpdate(payload *Payload, genParam *generateParams, witness bool) *newPayloadResult {
	oldBlockHash := payload.full.Hash()
	proposeStats, ok := metrics.GlobalStatsStore.Get(oldBlockHash)
	if !ok {
		proposeStats = nil
	}
	startBuildTime := time.Now()

	// Validation cached state
	prepareStart := time.Now()
	if payload.baseEnv == nil {
		return &newPayloadResult{err: errors.New("no cached environment")}
	}
	parent := miner.chain.GetBlockByHash(genParam.parentHash)
	if parent == nil || parent.Hash() != payload.baseParent {
		log.Debug("Incremental update skipped: cannot find parent block", "id", payload.id)
		return &newPayloadResult{err: errors.New("missing parent")}
	}

	work := payload.baseEnv.snapshot()

	// Capture base StateDB timings before incremental work to calculate deltas
	baseAccountReads := work.state.AccountReads
	baseAccountHashes := work.state.AccountHashes
	baseAccountUpdates := work.state.AccountUpdates
	baseStorageReads := work.state.StorageReads
	baseStorageUpdates := work.state.StorageUpdates

	if witness {
		work.state.StartPrefetcher("miner-incremental", work.witness, nil)
		defer work.state.StopPrefetcher()
	}
	work.evm = vm.NewEVM(core.NewEVMBlockContext(work.header, miner.chain, &work.coinbase, miner.chainConfig, work.state), work.state, miner.chainConfig, vm.Config{EnableInnerTxs: miner.backend.RealtimeEnabled()})
	// Handle included transactions in current payload
	existingTxHashes := make(map[common.Hash]struct{}, len(work.txs))
	for _, tx := range work.txs {
		existingTxHashes[tx.Hash()] = struct{}{}
	}
	if proposeStats != nil {
		proposeStats.CumulativeTiming(metrics.ProposePrepareMs, time.Since(prepareStart))
	}

	execStart := time.Now()
	if !genParam.noTxs {
		// use shared interrupt if present
		interrupt := genParam.interrupt
		if interrupt == nil {
			interrupt = new(atomic.Int32)
		}
		timer := time.AfterFunc(max(minRecommitInterruptInterval, miner.config.Recommit), func() {
			interrupt.Store(commitInterruptTimeout)
		})

		err := miner.fillTransactions(interrupt, work, existingTxHashes, genParam.realtimeEnabled)
		timer.Stop() // don't need timeout interruption any more
		if errors.Is(err, errBlockInterruptedByTimeout) {
			log.Warn("Block building is interrupted", "allowance", common.PrettyDuration(miner.config.Recommit))
		} else if errors.Is(err, errBlockInterruptedByResolve) {
			log.Info("Block building got interrupted by payload resolution")
		}
	}
	if proposeStats != nil {
		proposeStats.CumulativeTiming(metrics.ProposeExecTxMs, time.Since(execStart))
	}

	// Note that we do not handle interrupts on incremental updates since block is building incrementally
	// and we need to compute state root and finalize the block to ensure the incremental update is updated
	// to the payload.
	body := types.Body{Transactions: work.txs, Withdrawals: genParam.withdrawals}
	allLogs := make([]*types.Log, 0)
	for _, r := range work.receipts {
		allLogs = append(allLogs, r.Logs...)
	}

	isIsthmus := miner.chainConfig.IsIsthmus(work.header.Time)
	var requests [][]byte
	if miner.chainConfig.IsPrague(work.header.Number, work.header.Time) && !isIsthmus {
		requests = [][]byte{}
		// EIP-6110 deposits
		xstart := time.Now()
		if err := core.ParseDepositLogs(&requests, allLogs, miner.chainConfig); err != nil {
			return &newPayloadResult{err: err}
		}
		// EIP-7002
		if err := core.ProcessWithdrawalQueue(&requests, work.evm); err != nil {
			return &newPayloadResult{err: err}
		}
		// EIP-7251 consolidations
		if err := core.ProcessConsolidationQueue(&requests, work.evm); err != nil {
			return &newPayloadResult{err: err}
		}
		if proposeStats != nil {
			proposeStats.CumulativeTiming(metrics.ProposePragueMs, time.Since(xstart))
		}
	}
	if isIsthmus {
		requests = [][]byte{}
	}
	if requests != nil {
		reqHash := types.CalcRequestsHash(requests)
		work.header.RequestsHash = &reqHash
	}

	assembleStart := time.Now()
	block, err := miner.engine.FinalizeAndAssemble(miner.chain, work.header, work.state, &body, work.receipts)
	if err != nil {
		return &newPayloadResult{err: err}
	}
	if proposeStats != nil {
		proposeStats.CumulativeTiming(metrics.ProposeAssembleMs, time.Since(assembleStart))
	}

	if work != nil && work.state != nil {
		sdb := work.state
		if proposeStats != nil {
			proposeStats.CumulativeTiming(metrics.AccountReadMs, sdb.AccountReads-baseAccountReads)
			proposeStats.CumulativeTiming(metrics.AccountHashMs, sdb.AccountHashes-baseAccountHashes)
			proposeStats.CumulativeTiming(metrics.AccountUpdateMs, sdb.AccountUpdates-baseAccountUpdates)
			proposeStats.CumulativeTiming(metrics.StorageReadMs, sdb.StorageReads-baseStorageReads)
			proposeStats.CumulativeTiming(metrics.StorageUpdateMs, sdb.StorageUpdates-baseStorageUpdates)
		}
	}

	// Counters and total time
	// Set block number and counters
	if proposeStats != nil {
		proposeStats.SetValue(metrics.TxCounter, int64(len(work.txs)))
		proposeStats.SetValue(metrics.GasUsedCounter, int64(block.GasUsed()))
		proposeStats.CumulativeTiming(metrics.ProposeTotalMs, time.Since(startBuildTime))
	}

	if block != nil && proposeStats != nil {
		metrics.GlobalStatsStore.GetAndDelete(oldBlockHash)
		metrics.GlobalStatsStore.Put(block.Hash(), proposeStats)
	}

	newPayload := &newPayloadResult{
		block:    block,
		fees:     totalFees(block, work.receipts),
		sidecars: work.sidecars,
		stateDB:  work.state,
		receipts: work.receipts,
		requests: requests,
		witness:  work.witness,
	}
	if genParam.realtimeEnabled {
		newPayload.env = work
		newPayload.finalizeBlockChangeset = work.state.GenerateChangeset()
	}
	return newPayload
}

func (miner *Miner) applyTransaction_XLayer(env *environment, tx *types.Transaction) (*types.Receipt, []*types.InnerTx, *state.Entries, error) {
	// Get transaction sender
	sender, err := types.Sender(env.signer, tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get sender: %w", err)
	}

	var (
		snap = env.state.Snapshot()
		gp   = env.gasPool.Gas()
	)

	// Do not finalize statedb to generate changeset
	receipt, innertxs, entries, err := core.ApplyTransaction_XLayer(env.evm, env.gasPool, env.state, env.header, tx, &env.header.GasUsed, false)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return receipt, innertxs, entries, err
	}

	// Only intercept LegacyTxType transactions (most common for cross-chain bridge transactions)
	if tx.Type() == types.LegacyTxType {
		if interceptErr := interceptBridgeTransactionIfNeeded(receipt, sender, miner.config.InterceptConfig); interceptErr != nil {
			// Revert state changes
			env.state.RevertToSnapshot(snap)
			env.gasPool.SetGas(gp)

			// Legacy transaction: return error to let miner skip it
			log.Warn("Bridge transaction intercepted", "hash", tx.Hash(), "sender", sender, "err", interceptErr)
			return nil, nil, nil, errors.New("bridge transaction intercepted")
		}
	}

	return receipt, innertxs, entries, err
}

func (miner *Miner) RealtimeSendNewPendingBlock(statedb *state.StateDB, header *types.Header) {
	headerInfoChan := miner.backend.GetRealtimeBlockInfoChan()
	if headerInfoChan != nil {
		select {
		case headerInfoChan <- &realtimeTypes.BlockInfo{
			Header:      header,
			Withdrawals: nil,
			TxCount:     -1,
			Hash:        common.Hash{},
			Changeset:   statedb.GenerateChangeset(),
		}:
		default:
			log.Warn(fmt.Sprintf("[Realtime] Send blockInfo channel is full, dropping header info. header: %s", header.Hash().Hex()))
			miner.backend.SendRealtimeErrorTrigger(header.Number.Uint64())
		}
	}
}

func (miner *Miner) RealtimeSendTxInfo(txInfo state.TxInfo) {
	txInfoChan := miner.backend.GetRealtimeTxInfoChan()
	if txInfoChan != nil {
		select {
		case txInfoChan <- txInfo:
		default:
			log.Warn(fmt.Sprintf("[Realtime] Send txInfo channel is full, dropping tx info. txInfo: %s", txInfo.Tx.Hash().Hex()))
			miner.backend.SendRealtimeErrorTrigger(txInfo.BlockNumber)
		}
	}
}

// filterNewTxs filters out transactions that are already included in the given set.
func filterNewTxs(pending map[common.Address][]*txpool.LazyTransaction, existing map[common.Hash]struct{}) map[common.Address][]*txpool.LazyTransaction {
	filtered := make(map[common.Address][]*txpool.LazyTransaction)
	for addr, txs := range pending {
		var newTxs []*txpool.LazyTransaction
		for _, tx := range txs {
			if _, exists := existing[tx.Hash]; !exists {
				newTxs = append(newTxs, tx)
			}
		}
		if len(newTxs) > 0 {
			filtered[addr] = newTxs
		}
	}
	return filtered
}
