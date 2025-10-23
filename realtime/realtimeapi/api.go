package realtimeapi

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	realtimeCache "github.com/ethereum/go-ethereum/realtime/cache"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	MockBlockHash         = common.BytesToHash([]byte{1})
	EmptyBlockHash        = common.Hash{}
	ErrRealtimeNotEnabled = fmt.Errorf("realtime is not enabled")
)

type RealtimeAPIImpl struct {
	cacheDB        *realtimeCache.RealtimeCache
	b              ethapi.Backend
	blockchainApi  BlockchainAPI
	transactionApi TransactionAPI
}

func NewRealtimeAPI(
	cacheDB *realtimeCache.RealtimeCache,
	base ethapi.Backend,
	blockchainApi BlockchainAPI,
	transactionApi TransactionAPI,
) *RealtimeAPIImpl {
	return &RealtimeAPIImpl{
		cacheDB:        cacheDB,
		b:              base,
		blockchainApi:  blockchainApi,
		transactionApi: transactionApi,
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
	pendingHeight := api.cacheDB.GetNextPendingHeight()

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
		pendingReader, pendingHeight := api.cacheDB.GetPendingStateReader()
		if pendingReader == nil {
			// No pending block opened yet, use latest state cache
			pendingReader, pendingHeight = api.cacheDB.GetLatestStateReader()
		}
		return pendingReader, pendingHeight, nil
	} else {
		reader := api.cacheDB.GetStateReaderByHeight(blockHeight)
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
	statedb.SetReaderXLayer(reader)

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
	isPending bool,
) (map[string]interface{}, error) {
	header, withdrawals, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		if isPending {
			// Pending block not open yet. Default to latest block
			blockNum = api.cacheDB.GetHighestConfirmHeight()
			header, _, _, _, ok = api.cacheDB.Stateless.GetBlockInfo(blockNum)
			if !ok {
				return nil, fmt.Errorf("header not found for block %d", blockNum)
			}
			isPending = false
		} else {
			return nil, fmt.Errorf("header not found for block %d", blockNum)
		}
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return nil, fmt.Errorf("block txs not found for block %d", blockNum)
	}
	transactions := make(types.Transactions, 0, len(txHashes))
	for _, txHash := range txHashes {
		txn, _, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash)
		if !exists {
			return nil, fmt.Errorf("transaction %s not found for block %d", txHash.Hex(), blockNum)
		}
		transactions = append(transactions, txn)
	}
	var bw types.Withdrawals
	if withdrawals != nil {
		bw = *withdrawals
	}
	block := types.NewBlockWithHeader(header).WithBody(types.Body{
		Transactions: transactions,
		Withdrawals:  bw,
	})

	response, err := ethapi.RPCMarshalBlock(ctx, block, true, fullTx, api.b.ChainConfig(), api.cacheDB)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block: %w", err)
	}
	if isPending {
		for _, field := range []string{"hash"} {
			response[field] = nil
		}
		if fullTx {
			if txs, ok := response["transactions"].([]interface{}); ok {
				for _, tx := range txs {
					if rpcTx, ok := tx.(*ethapi.RPCTransaction); ok {
						rpcTx.BlockHash = nil
					}
				}
			}
		}
	}
	return response, nil
}
