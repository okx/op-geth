package miner

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
	"github.com/holiman/uint256"
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
		signer:       env.signer,
		state:        env.state.Copy(),
		tcount:       env.tcount,
		gasPool:      new(core.GasPool).AddGas(env.gasPool.Gas()),
		coinbase:     env.coinbase,
		evm:          nil,
		header:       types.CopyHeader(env.header),
		txs:          append([]*types.Transaction(nil), env.txs...),
		receipts:     append([]*types.Receipt(nil), env.receipts...),
		sidecars:     append([]*types.BlobTxSidecar(nil), env.sidecars...),
		blobs:        env.blobs,
		noTxs:        env.noTxs,
		rpcCtx:       env.rpcCtx,
		okPayTxs:     env.okPayTxs,
		blockDaBytes: env.blockDaBytes,
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

		err := miner.fillTransactions_XLayer(interrupt, work, existingTxHashes, genParam.realtimeEnabled)
		timer.Stop() // don't need timeout interruption any more
		if errors.Is(err, errBlockInterruptedByTimeout) {
			log.Warn("Incremental block building is interrupted", "allowance", common.PrettyDuration(miner.config.Recommit))
			payload.stoppedFlag.Store(true)
		} else if errors.Is(err, errBlockInterruptedByResolve) {
			log.Info("Incremental block building got interrupted by payload resolution")
			payload.stoppedFlag.Store(true)
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

func (miner *Miner) fillTransactions_XLayer(interrupt *atomic.Int32, env *environment, existTxs map[common.Hash]struct{}, realtimeEnabled bool) error {
	miner.confMu.RLock()
	tip := miner.config.GasPrice
	prio := miner.prio
	miner.confMu.RUnlock()

	// Retrieve the pending transactions pre-filtered by the 1559/4844 dynamic fees
	filter := txpool.PendingFilter{
		MinTip:      uint256.MustFromBig(tip),
		MaxDATxSize: miner.config.MaxDATxSize,
	}
	if env.header.BaseFee != nil {
		filter.BaseFee = uint256.MustFromBig(env.header.BaseFee)
	}
	if env.header.ExcessBlobGas != nil {
		filter.BlobFee = uint256.MustFromBig(eip4844.CalcBlobFee(miner.chainConfig, env.header))
	}
	if miner.chainConfig.IsOsaka(env.header.Number, env.header.Time) {
		filter.GasLimitCap = params.MaxTxGas
	}

	// For X Layer. Optimize fillTransactions by checking interruption signal first.
	// If payload is already interrupted, we immediately resolve payload before
	// getting pending transactions from the pool (which could potentially block on
	// pool reorgs).
	if err := checkInterrupt(interrupt); err != nil {
		return err
	}

	resultChan := make(chan pendingTxsResult, 1)
	go miner.asyncGetPendingTxsFromPool(&filter, resultChan)
	var pendingPlainTxs, pendingBlobTxs map[common.Address][]*txpool.LazyTransaction
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
loop:
	for {
		select {
		case result := <-resultChan:
			pendingPlainTxs = result.plainTxs
			pendingBlobTxs = result.blobTxs
			break loop
		case <-ticker.C:
			if err := checkInterrupt(interrupt); err != nil {
				return err
			}
		}
	}
	pendingPlainTxs = filterNewTxs(pendingPlainTxs, existTxs)
	pendingBlobTxs = filterNewTxs(pendingBlobTxs, existTxs)

	// Split the pending transactions into locals and remotes.
	prioPlainTxs, normalPlainTxs := make(map[common.Address][]*txpool.LazyTransaction), pendingPlainTxs
	prioBlobTxs, normalBlobTxs := make(map[common.Address][]*txpool.LazyTransaction), pendingBlobTxs

	type okPayTx struct {
		account common.Address
		tx      *txpool.LazyTransaction
	}

	okPayTxs := make(map[common.Address][]*txpool.LazyTransaction)

	sortedOkPayTxs := common.OrderedList[okPayTx]{}
	sortedOkPayTxs.SetCompareFunc(func(a, b okPayTx) int {
		if a.tx.Tx.Nonce() < b.tx.Tx.Nonce() {
			return -1
		}
		if a.tx.Tx.Nonce() > b.tx.Tx.Nonce() {
			return 1
		}
		return 0
	})

	accounts := miner.config.OkPaySenderAccounts

	// Skip the entire loop if OkPay priority feature is disabled
	if miner.config.OkPayPriorityEnable && len(accounts) > 0 {
		for _, account := range accounts {
			if txs := normalPlainTxs[account]; len(txs) > 0 {
				for _, tx := range txs {
					sortedOkPayTxs.Add(okPayTx{account: account, tx: tx})
				}
				delete(normalPlainTxs, account)
			}
		}

		if sortedOkPayTxs.Size() > 0 {
			sortedOkPayTxs.Sort()
			items := sortedOkPayTxs.Items()

			limit := int(miner.config.OkPayBlockPriorityTxsLimit) - env.okPayTxs
			if limit < 0 {
				limit = 0
			}
			if len(items) > limit {
				// Process priority transactions
				for _, item := range items[:limit] {
					okPayTxs[item.account] = append(okPayTxs[item.account], item.tx)
					env.okPayTxs++
				}
				// Put back unselected transactions
				for _, item := range items[limit:] {
					normalPlainTxs[item.account] = append(normalPlainTxs[item.account], item.tx)
				}
			} else {
				// All transactions get priority
				for _, item := range items {
					okPayTxs[item.account] = append(okPayTxs[item.account], item.tx)
					env.okPayTxs++
				}
			}
		}
	}
	// Process OkPay transactions first (highest priority)
	if len(okPayTxs) > 0 {
		okpayPlainTxs := newTransactionsByPriceAndNonce(env.signer, okPayTxs, env.header.BaseFee)
		emptyBlobTxs := newTransactionsByPriceAndNonce(env.signer, nil, env.header.BaseFee)
		// execStart removed: caller accumulates timings
		if err := miner.commitTransactions(env, okpayPlainTxs, emptyBlobTxs, interrupt, realtimeEnabled); err != nil {
			return err
		}
		// Note: execution timing is accumulated in caller scope (generateWork)
	}

	for _, account := range prio {
		if txs := normalPlainTxs[account]; len(txs) > 0 {
			delete(normalPlainTxs, account)
			prioPlainTxs[account] = txs
		}
		if txs := normalBlobTxs[account]; len(txs) > 0 {
			delete(normalBlobTxs, account)
			prioBlobTxs[account] = txs
		}
	}
	// Fill the block with all available pending transactions.
	if len(prioPlainTxs) > 0 || len(prioBlobTxs) > 0 {
		plainTxs := newTransactionsByPriceAndNonce(env.signer, prioPlainTxs, env.header.BaseFee)
		blobTxs := newTransactionsByPriceAndNonce(env.signer, prioBlobTxs, env.header.BaseFee)

		if err := miner.commitTransactions(env, plainTxs, blobTxs, interrupt, realtimeEnabled); err != nil {
			return err
		}
	}
	if len(normalPlainTxs) > 0 || len(normalBlobTxs) > 0 {
		plainTxs := newTransactionsByPriceAndNonce(env.signer, normalPlainTxs, env.header.BaseFee)
		blobTxs := newTransactionsByPriceAndNonce(env.signer, normalBlobTxs, env.header.BaseFee)

		if err := miner.commitTransactions(env, plainTxs, blobTxs, interrupt, realtimeEnabled); err != nil {
			return err
		}
	}
	return nil
}

func checkInterrupt(interrupt *atomic.Int32) error {
	// Check interruption signal and abort building if it's fired.
	if interrupt != nil {
		if signal := interrupt.Load(); signal != commitInterruptNone {
			return signalToErr(signal)
		}
	}
	return nil
}

type pendingTxsResult struct {
	plainTxs map[common.Address][]*txpool.LazyTransaction
	blobTxs  map[common.Address][]*txpool.LazyTransaction
}

func (miner *Miner) asyncGetPendingTxsFromPool(filter *txpool.PendingFilter, resultChan chan pendingTxsResult) {
	filter.OnlyPlainTxs, filter.OnlyBlobTxs = true, false
	pendingPlainTxs := miner.txpool.Pending(*filter)

	filter.OnlyPlainTxs, filter.OnlyBlobTxs = false, true
	pendingBlobTxs := miner.txpool.Pending(*filter)

	resultChan <- pendingTxsResult{
		plainTxs: pendingPlainTxs,
		blobTxs:  pendingBlobTxs,
	}
}

func (miner *Miner) applyTransaction_XLayer(env *environment, tx *types.Transaction) (*types.Receipt, []*types.InnerTx, *state.Entries, error) {
	var (
		snap = env.state.Snapshot()
		gp   = env.gasPool.Gas()
	)

	// Do not finalize statedb yet to generate changeset or bridge intercept
	receipt, innertxs, entries, err := core.ApplyTransaction_XLayer(env.evm, env.gasPool, env.state, env.header, tx, &env.header.GasUsed, false)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return receipt, innertxs, entries, err
	}

	// Only intercept LegacyTxType transactions (most common for cross-chain bridge transactions)
	if tx.Type() == types.LegacyTxType {
		// Get transaction sender
		sender, err := types.Sender(env.signer, tx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get sender: %w", err)
		}
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
