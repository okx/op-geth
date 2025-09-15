// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

// Key format: "InnerTx" + blockNum (uint64 big endian) + txIndex (uint32 big endian) -> RLP([]*InnerTx)

const InnerTxPrefix = "InnerTx"

// innerTxKey generates the database key for inner transactions
// Key format: "InnerTx" + blockNum (uint64 big endian) + txIndex (uint32 big endian)
func innerTxKey(blockNum uint64, txIndex uint32) []byte {
	key := make([]byte, len(InnerTxPrefix)+8+4)
	copy(key, InnerTxPrefix)
	binary.BigEndian.PutUint64(key[len(InnerTxPrefix):], blockNum)
	binary.BigEndian.PutUint32(key[len(InnerTxPrefix)+8:], txIndex)
	return key
}

// WriteInnerTxs stores inner transactions for a specific transaction in a block
func WriteInnerTxs(db ethdb.KeyValueWriter, blockNum uint64, txIndex uint32, innerTxs []*types.InnerTx) error {
	if len(innerTxs) == 0 {
		return nil
	}

	key := innerTxKey(blockNum, txIndex)

	data, err := rlp.EncodeToBytes(innerTxs)
	if err != nil {
		return err
	}

	return db.Put(key, data)
}

// ReadInnerTxs retrieves inner transactions for a specific transaction in a block
func ReadInnerTxs(db ethdb.KeyValueReader, blockNum uint64, txIndex uint32) ([]*types.InnerTx, error) {
	key := innerTxKey(blockNum, txIndex)

	// First check if the key exists to distinguish between "not found" and "database error"
	exists, err := db.Has(key)
	if err != nil {
		return nil, err // Database error
	}
	if !exists {
		return []*types.InnerTx{}, nil // Key doesn't exist, return empty slice
	}

	data, err := db.Get(key)
	if err != nil {
		return nil, err
	}

	var innerTxs []*types.InnerTx
	if err := rlp.DecodeBytes(data, &innerTxs); err != nil {
		return nil, err
	}
	return innerTxs, nil
}

// ReadInnerTxsByTxHash retrieves inner transactions by transaction hash
// This requires looking up the block number and transaction index first
func ReadInnerTxsByTxHash(db ethdb.Reader, txHash common.Hash) ([]*types.InnerTx, error) {
	blockNumber := ReadTxLookupEntry(db, txHash)
	if blockNumber == nil {
		return nil, nil
	}

	// Get the block to find the transaction index
	block := ReadBlock(db, ReadCanonicalHash(db, *blockNumber), *blockNumber)
	if block == nil {
		return nil, nil
	}

	// Find the transaction index within the block
	txIndex := -1
	for i, tx := range block.Transactions() {
		if tx.Hash() == txHash {
			txIndex = i
			break
		}
	}

	if txIndex == -1 {
		return nil, nil
	}

	return ReadInnerTxs(db, *blockNumber, uint32(txIndex))
}

// DeleteBlockInnerTxs removes inner transactions for all transactions in a block
func DeleteBlockInnerTxs(db ethdb.KeyValueWriter, blockNum uint64, txCount int) error {
	for txIndex := 0; txIndex < txCount; txIndex++ {
		key := innerTxKey(blockNum, uint32(txIndex))
		if err := db.Delete(key); err != nil {
			log.Error("Failed to delete inner transactions", "block", blockNum, "tx", txIndex, "err", err)
		}
	}
	return nil
}

// DeleteBlockInnerTxsBatch adds inner transaction deletion operations for an entire block to a batch
func DeleteBlockInnerTxsBatch(batch ethdb.Batch, blockNum uint64, txCount int) {
	for txIndex := 0; txIndex < txCount; txIndex++ {
		key := innerTxKey(blockNum, uint32(txIndex))
		if err := batch.Delete(key); err != nil {
			log.Error("Failed to delete inner transactions", "block", blockNum, "tx", txIndex, "err", err)
		}
	}
}
