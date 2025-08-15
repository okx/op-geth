package realtimeapi

import (
	"context"
	"fmt"
	"math/big"

	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	realtimeCache "github.com/ethereum/go-ethereum/realtime/cache"
	"github.com/ethereum/go-ethereum/realtime/subscription"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	MockBlockHash         = libcommon.BytesToHash([]byte{1})
	EmptyBlockHash        = libcommon.Hash{}
	ErrRealtimeNotEnabled = fmt.Errorf("realtime is not enabled")
)

type RealtimeAPIImpl struct {
	cacheDB    *realtimeCache.RealtimeCache
	subService *subscription.RealtimeSubscription
	b          ethapi.Backend
	filterApi  *filters.FilterAPI
}

func NewRealtimeAPI(
	cacheDB *realtimeCache.RealtimeCache,
	subService *subscription.RealtimeSubscription,
	base ethapi.Backend,
	filterApi *filters.FilterAPI,
) *RealtimeAPIImpl {
	return &RealtimeAPIImpl{
		cacheDB:    cacheDB,
		subService: subService,
		b:          base,
		filterApi:  filterApi,
	}
}

func (api *RealtimeAPIImpl) getBlockNumber(blockNr rpc.BlockNumber) (uint64, bool, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return 0, false, ErrRealtimeNotEnabled
	}

	confirmHeight, err := api.getConfirmHeightFromCache()
	if err != nil {
		return 0, false, err
	}

	switch blockNr {
	case rpc.LatestBlockNumber:
		return confirmHeight, true, nil
	case rpc.PendingBlockNumber:
		pendingHeight, err := api.getPendingHeightFromCache()
		if err != nil {
			return 0, false, err
		}
		return pendingHeight, true, nil
	// Unsupported tags
	case rpc.EarliestBlockNumber:
		return 0, false, fmt.Errorf("earliest block number is not realtime supported")
	case rpc.FinalizedBlockNumber:
		return 0, false, fmt.Errorf("finalized block number is not realtime supported")
	case rpc.SafeBlockNumber:
		return 0, false, fmt.Errorf("safe block number is not realtime supported")
	default:
		blockNumber := uint64(blockNr.Int64())
		if blockNumber > confirmHeight {
			return 0, false, fmt.Errorf("block with number %d not found", blockNumber)
		}
		return blockNumber, blockNumber == confirmHeight, nil
	}
}

func (api *RealtimeAPIImpl) getPendingHeightFromCache() (uint64, error) {
	pendingHeight := api.cacheDB.GetCurrentPendingHeight()
	if pendingHeight == 0 {
		return 0, fmt.Errorf("no pending block number found in realtime cache")
	}
	return pendingHeight, nil
}

func (api *RealtimeAPIImpl) getConfirmHeightFromCache() (uint64, error) {
	confirmHeight := api.cacheDB.GetHighestConfirmHeight()
	if confirmHeight == 0 {
		return 0, fmt.Errorf("no confirmed block number found in realtime cache")
	}
	return confirmHeight, nil
}

func (api *RealtimeAPIImpl) createStateReader(blockNrOrHash *rpc.BlockNumberOrHash) (reader state.Reader, blockNumber uint64, err error) {
	if blockNrOrHash.BlockNumber == nil {
		// todo: add support for latest block hash
		return nil, 0, fmt.Errorf("failed to create state reader: block number is nil")
	}

	confirmHeight, err := api.getConfirmHeightFromCache()
	if err != nil {
		return nil, 0, err
	}
	pendingHeight, err := api.getPendingHeightFromCache()
	if err != nil {
		return nil, 0, err
	}

	// Realtime supports pending and latest tags only
	if *blockNrOrHash.BlockNumber == rpc.PendingBlockNumber || *blockNrOrHash.BlockNumber == rpc.BlockNumber(pendingHeight) {
		pendingReader := api.cacheDB.GetPendingStateCache(pendingHeight)
		if pendingReader != nil {
			reader = pendingReader
		} else {
			// Pending block was closed, we use the latest confirmed global state
			reader = api.cacheDB.State
		}
		blockNumber = pendingHeight
	} else if *blockNrOrHash.BlockNumber == rpc.LatestBlockNumber || *blockNrOrHash.BlockNumber == rpc.BlockNumber(confirmHeight) {
		reader = api.cacheDB.State
		blockNumber = confirmHeight
	}
	return
}

func (api *RealtimeAPIImpl) GetStateDB(ctx context.Context, reader state.Reader) (*state.StateDB, error) {
	statedb, _, err := api.b.StateAndHeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	// Override the reader with the realtime state cache layer
	statedb.SetReader(reader)

	return statedb, nil
}

// newRPCTransaction_realtime returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
// Note that realtime API do not support blockHash.
func newRPCTransaction_realtime(tx *types.Transaction, txblockhash libcommon.Hash, blockNumber uint64, blockTime uint64, index uint64, baseFee *big.Int, config *params.ChainConfig, receipt *types.Receipt) *ethapi.RPCTransaction {
	blockhash := txblockhash
	if blockhash == EmptyBlockHash {
		blockhash = MockBlockHash
	}

	result := ethapi.NewRPCTransaction(tx, blockhash, blockNumber, blockTime, index, baseFee, config, receipt)
	result.BlockHash = &libcommon.Hash{}
	return result
}

// formatBlockResponse creates a formatted block response from cache data
// This utility function consolidates the block formatting logic used by both
// GetBlockByNumber and GetBlockByHash methods
func (api *RealtimeAPIImpl) tryGetBlockResponseFromNumber(
	ctx context.Context,
	blockNum uint64,
	fullTx bool,
) (map[string]interface{}, error) {
	header, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNum)
	if !ok {
		return nil, fmt.Errorf("header not found for block %d", blockNum)
	}

	var body types.Body
	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if ok {
		for _, txHash := range txHashes {
			if tx, _, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash); exists {
				body.Transactions = append(body.Transactions, tx)
			} else {
				return nil, fmt.Errorf("transaction %s not found in cache", txHash.Hex())
			}
		}
	}

	block := types.NewBlockWithHeader(header).WithBody(body)

	response, err := ethapi.RPCMarshalBlock(ctx, block, true, fullTx, api.b.ChainConfig(), api.cacheDB)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block: %w", err)
	}

	return response, nil
}
