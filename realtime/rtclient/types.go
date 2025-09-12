package rtclient

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
)

type RealtimeDebugResult struct {
	ConfirmHeight   uint64   `json:"confirmHeight"`
	ExecutionHeight uint64   `json:"executionHeight"`
	Mismatches      []string `json:"mismatches"`
}

type BigInt struct {
	*big.Int
}

// UnmarshalJSON implements json.Unmarshaler for BigInt
func (bi *BigInt) UnmarshalJSON(data []byte) error {
	if bi.Int == nil {
		bi.Int = new(big.Int)
	}
	// Remove quotes
	unquotedData, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}

	if len(unquotedData) > 2 && unquotedData[0] == '0' && unquotedData[1] == 'x' {
		unquotedData = unquotedData[2:]
	}
	_, success := bi.SetString(unquotedData, 16)
	if !success {
		return errors.New("failed to convert string to big.Int")
	}
	return nil
}

type Int int

// UnmarshalJSON implements json.Unmarshaler for Int
func (i *Int) UnmarshalJSON(data []byte) error {
	unquotedData, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}

	if len(unquotedData) > 2 && unquotedData[0] == '0' && unquotedData[1] == 'x' {
		unquotedData = unquotedData[2:]
	}

	num, err := strconv.ParseInt(unquotedData, 16, 64)
	if err != nil {
		return err
	}

	*i = Int(num)
	return nil
}

// RpcTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RpcTransaction struct {
	BlockNumber      *string         `json:"blockNumber,omitempty"`
	BlockHash        *common.Hash    `json:"blockHash,omitempty"`
	From             *common.Address `json:"from,omitempty"`
	Gas              *BigInt         `json:"gas,omitempty"`
	GasPrice         *BigInt         `json:"gasPrice,omitempty"`
	Hash             *string         `json:"hash,omitempty"`
	Input            *string         `json:"input,omitempty"`
	Nonce            *BigInt         `json:"nonce,omitempty"`
	R                *string         `json:"r,omitempty"`
	S                *string         `json:"s,omitempty"`
	To               *common.Address `json:"to,omitempty"`
	TransactionIndex *Int            `json:"transactionIndex,omitempty"`
	Type             *Int            `json:"type,omitempty"`
	V                *string         `json:"v,omitempty"`
	Value            *BigInt         `json:"value,omitempty"`
}
