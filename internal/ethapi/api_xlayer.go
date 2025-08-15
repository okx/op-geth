package ethapi

import (
	"context"
	"math/big"
	"time"

	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func (*BlockChainAPI) RealtimeEnabled(ctx context.Context) (bool, error) {
	return false, nil
}

func NewRPCTransaction(tx *types.Transaction, txblockhash libcommon.Hash, blockNumber uint64, blockTime uint64, index uint64, baseFee *big.Int, config *params.ChainConfig, receipt *types.Receipt) *RPCTransaction {
	return newRPCTransaction(tx, txblockhash, blockNumber, blockTime, index, baseFee, config, receipt)
}

func DoCallRealtime(ctx context.Context, b Backend, args TransactionArgs, statedb *state.StateDB, header *types.Header, overrides *override.StateOverride, blockOverrides *override.BlockOverrides, timeout time.Duration, globalGasCap uint64) (*core.ExecutionResult, error) {
	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())
	return doCall(ctx, b, args, statedb, header, overrides, blockOverrides, timeout, globalGasCap)
}

func MarshalReceipt(receipt *types.Receipt, blockNumber uint64, signer types.Signer, tx *types.Transaction, chainConfig *params.ChainConfig) map[string]interface{} {
	return marshalReceipt(receipt, receipt.BlockHash, blockNumber, signer, tx, int(receipt.TransactionIndex), chainConfig)
}

func GetEstimateGasErrorRatio() float64 {
	return estimateGasErrorRatio
}

func NewRevertError(revert []byte) *revertError {
	return newRevertError(revert)
}

// todo: remove this as it will be the implemented eth_getInternalTransactions for X Layer's internal transactions
func (*TransactionAPI) GetInternalTransactions(ctx context.Context, hash libcommon.Hash) ([]*types.InnerTx, error) {
	return nil, nil
}

// todo: remove this as it will be the implemented eth_getBlockInternalTransactions for X Layer's internal transactions
func (*BlockChainAPI) GetBlockInternalTransactions(ctx context.Context, blockNr rpc.BlockNumber) (map[libcommon.Hash][]*types.InnerTx, error) {
	return nil, nil
}
