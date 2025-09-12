package types

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

func fromCommonTxMessage(tx *types.Transaction, blockNumber uint64, blockTime uint64) (TransactionMessage, error) {
	msg := TransactionMessage{
		BlockNumber: blockNumber,
		BlockTime:   blockTime,
		Type:        tx.Type(),
		Hash:        tx.Hash(),
		ChainID:     tx.ChainId(),
		Nonce:       tx.Nonce(),
		Gas:         tx.Gas(),
		To:          tx.To(),
		Value:       tx.Value(),
		Data:        tx.Data(),
	}
	msg.V, msg.R, msg.S = tx.RawSignatureValues()

	return msg, nil
}

func fromLegacyTxMessage(tx *types.Transaction, blockNumber uint64, blockTime uint64) (TransactionMessage, error) {
	msg, err := fromCommonTxMessage(tx, blockNumber, blockTime)
	if err != nil {
		return TransactionMessage{}, err
	}
	msg.GasPrice = tx.GasPrice()

	return msg, nil
}

func fromAccessListTxMessage(tx *types.Transaction, blockNumber uint64, blockTime uint64) (TransactionMessage, error) {
	msg, err := fromLegacyTxMessage(tx, blockNumber, blockTime)
	if err != nil {
		return TransactionMessage{}, err
	}
	accessList := tx.AccessList()
	msg.AccessList = make([]AccessTupleMessage, 0, len(accessList))
	for _, tuple := range accessList {
		msg.AccessList = append(msg.AccessList, fromAccessTuple(tuple))
	}

	return msg, nil
}

func fromDynamicFeeTxMessage(tx *types.Transaction, blockNumber uint64, blockTime uint64) (TransactionMessage, error) {
	msg, err := fromAccessListTxMessage(tx, blockNumber, blockTime)
	if err != nil {
		return TransactionMessage{}, err
	}
	msg.Tip = tx.GasTipCap()
	msg.FeeCap = tx.GasFeeCap()

	return msg, nil
}

func fromBlobTxMessage(tx *types.Transaction, blockNumber uint64, blockTime uint64) (TransactionMessage, error) {
	// Check if it's a BlobTx or BlobTxWrapper
	msg, err := fromDynamicFeeTxMessage(tx, blockNumber, blockTime)
	if err != nil {
		return TransactionMessage{}, err
	}
	msg.MaxFeePerBlobGas = tx.BlobGasFeeCap()
	msg.BlobVersionedHashes = make([]string, 0, len(tx.BlobHashes()))
	for _, hash := range tx.BlobHashes() {
		msg.BlobVersionedHashes = append(msg.BlobVersionedHashes, hash.String())
	}

	return msg, nil
}

func (msg TransactionMessage) toLegacyTx() *types.Transaction {
	return types.NewTx(&types.LegacyTx{
		Nonce:    msg.Nonce,
		Gas:      msg.Gas,
		GasPrice: msg.GasPrice,
		To:       msg.To,
		Value:    msg.Value,
		Data:     msg.Data,
		V:        msg.V,
		R:        msg.R,
		S:        msg.S,
	})
}

func (msg TransactionMessage) toAccessListTx() *types.Transaction {
	return types.NewTx(&types.AccessListTx{
		ChainID:    msg.ChainID,
		Nonce:      msg.Nonce,
		Gas:        msg.Gas,
		GasPrice:   msg.GasPrice,
		To:         msg.To,
		Value:      msg.Value,
		Data:       msg.Data,
		V:          msg.V,
		R:          msg.R,
		S:          msg.S,
		AccessList: msg.getAccessList(),
	})
}

func (msg TransactionMessage) toDynamicFeeTx() *types.Transaction {
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:    msg.ChainID,
		Nonce:      msg.Nonce,
		Gas:        msg.Gas,
		GasFeeCap:  msg.FeeCap,
		GasTipCap:  msg.Tip,
		To:         msg.To,
		Value:      msg.Value,
		Data:       msg.Data,
		V:          msg.V,
		R:          msg.R,
		S:          msg.S,
		AccessList: msg.getAccessList(),
	})
}

func (msg TransactionMessage) toBlobTx() *types.Transaction {
	return types.NewTx(&types.BlobTx{
		ChainID:    uint256.MustFromBig(msg.ChainID),
		Nonce:      msg.Nonce,
		Gas:        msg.Gas,
		GasFeeCap:  uint256.MustFromBig(msg.FeeCap),
		GasTipCap:  uint256.MustFromBig(msg.Tip),
		To:         *msg.To,
		Value:      uint256.MustFromBig(msg.Value),
		Data:       msg.Data,
		V:          uint256.MustFromBig(msg.V),
		R:          uint256.MustFromBig(msg.R),
		S:          uint256.MustFromBig(msg.S),
		AccessList: msg.getAccessList(),
		BlobFeeCap: uint256.MustFromBig(msg.MaxFeePerBlobGas),
		BlobHashes: msg.getBlobVersionedHashes(),
	})
}
