package types

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

// TransactionMessage represents the structure of the transaction message to be sent to Kafka
type TransactionMessage struct {
	// Sequenced block number
	BlockNumber uint64 `json:"blockNumber"`
	BlockTime   uint64 `json:"blockTime"`

	// Common tx fields
	Type    uint8           `json:"type"`
	Hash    common.Hash     `json:"hash"`
	ChainID *big.Int        `json:"chainId"`
	Nonce   uint64          `json:"nonce"`
	Gas     uint64          `json:"gas"`
	To      *common.Address `json:"to"`
	Value   *big.Int        `json:"value"`
	Data    []byte          `json:"data"`
	V       *big.Int        `json:"v"`
	R       *big.Int        `json:"r"`
	S       *big.Int        `json:"s"`

	// For legacy txs
	GasPrice *big.Int `json:"gasPrice"`
	// For EIP-1559 and EIP-2930 txs
	AccessList []AccessTupleMessage `json:"accessList"`
	Tip        *big.Int             `json:"tip"`
	FeeCap     *big.Int             `json:"feeCap"`
	// For blob txs
	MaxFeePerBlobGas    *big.Int `json:"maxFeePerBlobGas"`
	BlobVersionedHashes []string `json:"blobVersionedHashes"`

	// Receipt data
	Receipt *types.Receipt `json:"receipt"`

	// Inner transactions
	InnerTxs []*types.InnerTx `json:"innerTxs"`

	// Changeset
	Changeset *realtimeTypes.Changeset `json:"changeset"`
}

func ToKafkaTransactionMessage(tx *types.Transaction, receipt *types.Receipt, innerTxs []*types.InnerTx, changeset *realtimeTypes.Changeset, blockNumber uint64, blockTime uint64) (txMsg TransactionMessage, err error) {
	// Parse tx
	if tx == nil || receipt == nil || innerTxs == nil || changeset == nil {
		return TransactionMessage{}, fmt.Errorf("nil tx data received")
	}

	// Parse tx
	switch tx.Type() {
	case types.LegacyTxType:
		txMsg, err = fromLegacyTxMessage(tx, blockNumber, blockTime)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse legacy tx error: %w", err)
		}
	case types.AccessListTxType:
		txMsg, err = fromAccessListTxMessage(tx, blockNumber, blockTime)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse accesslist tx error: %w", err)
		}
	case types.DynamicFeeTxType:
		txMsg, err = fromDynamicFeeTxMessage(tx, blockNumber, blockTime)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse dynamic fee tx error: %w", err)
		}
	case types.BlobTxType:
		txMsg, err = fromBlobTxMessage(tx, blockNumber, blockTime)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse blob tx error: %w", err)
		}
	default:
		return TransactionMessage{}, fmt.Errorf("unsupported transaction type: %d", tx.Type())
	}

	// Parse receipt
	txMsg.Receipt = receipt
	txMsg.InnerTxs = innerTxs
	txMsg.Changeset = changeset

	return txMsg, nil
}

func (msg TransactionMessage) GetTransaction() (*types.Transaction, uint64, error) {
	blockNumber := msg.BlockNumber

	// Get tx
	switch msg.Type {
	case types.LegacyTxType:
		return msg.toLegacyTx(), blockNumber, nil
	case types.AccessListTxType:
		return msg.toAccessListTx(), blockNumber, nil
	case types.DynamicFeeTxType:
		return msg.toDynamicFeeTx(), blockNumber, nil
	case types.BlobTxType:
		return msg.toBlobTx(), blockNumber, nil
	default:
		return nil, blockNumber, fmt.Errorf("unsupported transaction type: %d", msg.Type)
	}
}

func (msg TransactionMessage) GetReceipt() (*types.Receipt, error) {
	if msg.Receipt == nil {
		return nil, fmt.Errorf("receipt is nil")
	}

	return msg.Receipt, nil
}

func (msg TransactionMessage) GetInnerTxs() ([]*types.InnerTx, error) {
	if msg.InnerTxs == nil {
		return nil, fmt.Errorf("innerTxs is nil")
	}

	return msg.InnerTxs, nil
}

func (msg TransactionMessage) GetAllTxData() (uint64, *types.Transaction, *types.Receipt, []*types.InnerTx, error) {
	tx, blockNum, err := msg.GetTransaction()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	receipt, err := msg.GetReceipt()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	innerTxs, err := msg.GetInnerTxs()
	if err != nil {
		return 0, nil, nil, nil, err
	}

	return blockNum, tx, receipt, innerTxs, nil
}

func (msg TransactionMessage) GetChangeset() (*realtimeTypes.Changeset, error) {
	if msg.Changeset == nil {
		return nil, fmt.Errorf("changeset is nil")
	}

	return msg.Changeset, nil
}

func (msg TransactionMessage) getAccessList() types.AccessList {
	accessList := make([]types.AccessTuple, 0, len(msg.AccessList))
	for _, tuple := range msg.AccessList {
		accessList = append(accessList, tuple.toAccessTuple())
	}

	return accessList
}

func (msg TransactionMessage) getBlobVersionedHashes() []common.Hash {
	blobVersionedHashes := make([]common.Hash, 0, len(msg.BlobVersionedHashes))
	for _, hash := range msg.BlobVersionedHashes {
		blobVersionedHashes = append(blobVersionedHashes, common.HexToHash(hash))
	}
	return blobVersionedHashes
}

func (msg TransactionMessage) Validate() error {
	if _, _, err := msg.GetTransaction(); err != nil {
		return err
	}
	if _, err := msg.GetReceipt(); err != nil {
		return err
	}
	if _, err := msg.GetInnerTxs(); err != nil {
		return err
	}
	if _, err := msg.GetChangeset(); err != nil {
		return err
	}

	return nil
}

func (msg TransactionMessage) MarshalJSON() ([]byte, error) {
	type TransactionMessage struct {
		BlockNumber         uint64                   `json:"blockNumber"`
		Type                uint8                    `json:"type"`
		Hash                common.Hash              `json:"hash"`
		ChainID             *big.Int                 `json:"chainId"`
		Nonce               uint64                   `json:"nonce"`
		Gas                 uint64                   `json:"gas"`
		To                  *common.Address          `json:"to"`
		Value               *big.Int                 `json:"value"`
		Data                []byte                   `json:"data"`
		V                   *big.Int                 `json:"v"`
		R                   *big.Int                 `json:"r"`
		S                   *big.Int                 `json:"s"`
		GasPrice            *big.Int                 `json:"gasPrice"`
		AccessList          []AccessTupleMessage     `json:"accessList"`
		Tip                 *big.Int                 `json:"tip"`
		FeeCap              *big.Int                 `json:"feeCap"`
		MaxFeePerBlobGas    *big.Int                 `json:"maxFeePerBlobGas"`
		BlobVersionedHashes []string                 `json:"blobVersionedHashes"`
		Receipt             *types.Receipt           `json:"receipt"`
		InnerTxs            []*types.InnerTx         `json:"innerTxs"`
		Changeset           *realtimeTypes.Changeset `json:"changeset"`
	}

	var enc TransactionMessage
	enc.BlockNumber = msg.BlockNumber
	enc.Type = msg.Type
	enc.Hash = msg.Hash
	enc.ChainID = msg.ChainID
	enc.Nonce = msg.Nonce
	enc.Gas = msg.Gas
	enc.To = msg.To
	enc.Value = msg.Value
	enc.Data = msg.Data
	enc.R = msg.R
	enc.S = msg.S
	enc.V = msg.V
	enc.GasPrice = msg.GasPrice
	enc.AccessList = msg.AccessList
	enc.Tip = msg.Tip
	enc.FeeCap = msg.FeeCap
	enc.MaxFeePerBlobGas = msg.MaxFeePerBlobGas
	enc.BlobVersionedHashes = msg.BlobVersionedHashes
	enc.InnerTxs = msg.InnerTxs
	enc.Changeset = msg.Changeset

	if msg.Receipt != nil {
		// Handle nil logs
		receipt := *msg.Receipt
		if receipt.Logs == nil {
			receipt.Logs = []*types.Log{}
		}

		// Handle nil topics
		for _, log := range receipt.Logs {
			if log != nil {
				if log.Topics == nil {
					log.Topics = []common.Hash{}
				}
			}
		}
		enc.Receipt = &receipt
	}

	return json.Marshal(&enc)
}
