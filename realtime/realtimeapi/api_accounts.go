package realtimeapi

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

func (api *RealtimeAPIImpl) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBalance(ctx, address, blockNrOrHash)
	}

	reader, _, err := api.createStateReader(blockNrOrHash)
	if err != nil || reader == nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBalance(ctx, address, blockNrOrHash)
	}

	acc, err := reader.Account(address)
	if err != nil {
		return nil, fmt.Errorf("cant get a balance for account %x: %w", address.String(), err)
	}
	if acc == nil {
		// Special case - non-existent account is assumed to have zero balance
		return (*hexutil.Big)(big.NewInt(0)), nil
	}

	return (*hexutil.Big)(acc.Balance.ToBig()), nil
}

func (api *RealtimeAPIImpl) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Uint64, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionCount(ctx, address, blockNrOrHash)
	}

	reader, _, err := api.createStateReader(blockNrOrHash)
	if err != nil || reader == nil {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionCount(ctx, address, blockNrOrHash)
	}

	backend := ethapi.NewTransactionAPI(api.b, nil)
	ethNonce, err := backend.GetTransactionCount(ctx, address, blockNrOrHash)
	if err != nil {
		ethNonce = nil
	}

	var cacheNonce *hexutil.Uint64
	acc, err := reader.Account(address)
	if err != nil {
		cacheNonce = nil
	} else if acc != nil {
		nonce := hexutil.Uint64(acc.Nonce)
		cacheNonce = &nonce
	}

	if ethNonce == nil && cacheNonce == nil {
		return nil, fmt.Errorf("failed to get transaction count for account %x from both sources", address)
	}

	if ethNonce == nil {
		return cacheNonce, nil
	}
	if cacheNonce == nil {
		return ethNonce, nil
	}

	if *ethNonce > *cacheNonce {
		return ethNonce, nil
	}
	return cacheNonce, nil
}

func (api *RealtimeAPIImpl) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetCode(ctx, address, blockNrOrHash)
	}

	reader, _, err := api.createStateReader(blockNrOrHash)
	if err != nil || reader == nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetCode(ctx, address, blockNrOrHash)
	}

	acc, err := reader.Account(address)
	if acc == nil || err != nil {
		return hexutil.Bytes(""), nil
	}
	res, _ := reader.Code(address, common.BytesToHash(acc.CodeHash))
	if res == nil {
		return hexutil.Bytes(""), nil
	}
	return res, nil
}

func (api *RealtimeAPIImpl) GetStorageAt(ctx context.Context, address common.Address, index string, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetStorageAt(ctx, address, index, blockNrOrHash)
	}

	reader, _, err := api.createStateReader(blockNrOrHash)
	if err != nil || reader == nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetStorageAt(ctx, address, index, blockNrOrHash)
	}

	var empty []byte
	acc, err := reader.Account(address)
	if acc == nil || err != nil {
		return empty, err
	}

	res, err := reader.Storage(address, common.HexToHash(index))
	if err != nil {
		res = common.BytesToHash(empty)
	}
	return res[:], err
}
