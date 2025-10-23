package realtimeapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/rpc"
)

type BlockchainAPI interface {
	GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error)
	GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error)
	GetStorageAt(ctx context.Context, address common.Address, key string, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error)
	GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error)
	GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error)
	GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) ([]map[string]interface{}, error)
	Call(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Bytes, error)
	EstimateGas(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Uint64, error)
	BlockNumber() hexutil.Uint64
}

type TransactionAPI interface {
	GetTransactionByHash(ctx context.Context, hash common.Hash) (*ethapi.RPCTransaction, error)
	GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error)
	GetInternalTransactions(ctx context.Context, hash common.Hash) ([]*types.InnerTx, error)
	GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error)
	GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (*hexutil.Uint, error)
	GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Uint64, error)
	GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (*ethapi.RPCTransaction, error)
	GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (*ethapi.RPCTransaction, error)
	GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error)
	GetBlockInternalTransactions(ctx context.Context, blockNr rpc.BlockNumber) (map[common.Hash][]*types.InnerTx, error)
}
