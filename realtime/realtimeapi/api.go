package realtimeapi

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
	MockBlockHash         = common.BytesToHash([]byte{1})
	EmptyBlockHash        = common.Hash{}
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

func (api *RealtimeAPIImpl) getBlockNumberOrHash(blockNrOrHash rpc.BlockNumberOrHash) (uint64, bool, bool, error) {
	if hash, ok := blockNrOrHash.Hash(); ok {
		blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(hash)
		if !found {
			return 0, false, false, fmt.Errorf("block %x not found", hash)
		}
		confirmHeight, err := api.getConfirmHeightFromCache()
		if err != nil {
			return 0, false, false, err
		}
		return blockNum, blockNum == confirmHeight, false, nil
	} else {
		if blockNrOrHash.BlockNumber == nil {
			return 0, false, false, fmt.Errorf("no block number or hash provided")
		}
		return api.getBlockNumber(*blockNrOrHash.BlockNumber)
	}
}

func (api *RealtimeAPIImpl) getBlockNumber(blockNr rpc.BlockNumber) (uint64, bool, bool, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return 0, false, false, ErrRealtimeNotEnabled
	}

	confirmHeight, err := api.getConfirmHeightFromCache()
	if err != nil {
		return 0, false, false, err
	}
	pendingHeight, err := api.getPendingHeightFromCache()
	if err != nil {
		return 0, false, false, err
	}

	switch blockNr {
	case rpc.LatestBlockNumber:
		return confirmHeight, true, false, nil
	case rpc.PendingBlockNumber:
		return pendingHeight, false, true, nil
	// Unsupported tags
	case rpc.EarliestBlockNumber:
		return 0, false, false, fmt.Errorf("earliest block number is not realtime supported")
	case rpc.FinalizedBlockNumber:
		return 0, false, false, fmt.Errorf("finalized block number is not realtime supported")
	case rpc.SafeBlockNumber:
		return 0, false, false, fmt.Errorf("safe block number is not realtime supported")
	default:
		blockNumber := uint64(blockNr.Int64())
		if blockNumber > pendingHeight {
			return 0, false, false, fmt.Errorf("block with number %d not found", blockNumber)
		}
		return blockNumber, blockNumber == confirmHeight, blockNumber == pendingHeight, nil
	}
}

func (api *RealtimeAPIImpl) getPendingHeightFromCache() (uint64, error) {
	pendingHeight := api.cacheDB.GetPendingHeight()
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

func (api *RealtimeAPIImpl) createStateReader(blockNrOrHash rpc.BlockNumberOrHash) (state.Reader, uint64, error) {
	blockHeight, _, isPending, err := api.getBlockNumberOrHash(blockNrOrHash)
	if err != nil {
		return nil, 0, err
	}

	if isPending {
		pendingReader, pendingHeight := api.cacheDB.GetPendingStateCache()
		if pendingReader == nil {
			// No pending block opened yet, use latest state cache
			pendingReader, pendingHeight = api.cacheDB.GetLatestStateCache()
		}
		return pendingReader, pendingHeight, nil
	} else {
		reader := api.cacheDB.GetStateCacheByHeight(blockHeight)
		if reader == nil {
			return nil, 0, fmt.Errorf("state reader not found for block %d", blockHeight)
		}
		return reader, blockHeight, nil
	}
}

func (api *RealtimeAPIImpl) GetStateDbWithCacheReader(ctx context.Context, reader state.Reader) (*state.StateDB, error) {
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
func newRPCTransaction_realtime(tx *types.Transaction, txblockhash common.Hash, blockNumber uint64, blockTime uint64, index uint64, baseFee *big.Int, config *params.ChainConfig, receipt *types.Receipt) *ethapi.RPCTransaction {
	blockhash := txblockhash
	if blockhash == EmptyBlockHash {
		blockhash = MockBlockHash
	}

	result := ethapi.NewRPCTransaction(tx, blockhash, blockNumber, blockTime, index, baseFee, config, receipt)
	result.BlockHash = &txblockhash
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
	header, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
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
