package engine

import "github.com/ethereum/go-ethereum/core/types"

func DecodeTransactionsXLayer(enc [][]byte) ([]*types.Transaction, error) {
	return decodeTransactions(enc)
}
