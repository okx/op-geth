// Copyright 2018 The go-ethereum Authors
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
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// ReadDatabaseVersion retrieves the version number of the database.
func ReadDatabaseVersion(db ethdb.KeyValueReader) *uint64 {
	var version uint64

	enc, _ := db.Get(databaseVersionKey)
	if len(enc) == 0 {
		return nil
	}
	if err := rlp.DecodeBytes(enc, &version); err != nil {
		return nil
	}

	return &version
}

// WriteDatabaseVersion stores the version number of the database
func WriteDatabaseVersion(db ethdb.KeyValueWriter, version uint64) {
	enc, err := rlp.EncodeToBytes(version)
	if err != nil {
		log.Crit("Failed to encode database version", "err", err)
	}
	if err = db.Put(databaseVersionKey, enc); err != nil {
		log.Crit("Failed to store the database version", "err", err)
	}
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func ReadChainConfig(db ethdb.KeyValueReader, hash common.Hash) *params.ChainConfig {
	data, _ := db.Get(configKey(hash))
	if len(data) == 0 {
		return nil
	}
	var config params.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Error("Invalid chain config JSON", "hash", hash, "err", err)
		return nil
	}
	if config.Optimism != nil {
		config.Clique = nil // get rid of legacy clique data in chain config (optimism goerli issue)
	}
	return &config
}

// WriteChainConfig writes the chain config settings to the database.
func WriteChainConfig(db ethdb.KeyValueWriter, hash common.Hash, cfg *params.ChainConfig) {
	if cfg == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		log.Crit("Failed to JSON encode chain config", "err", err)
	}
	if err := db.Put(configKey(hash), data); err != nil {
		log.Crit("Failed to store chain config", "err", err)
	}
}

const (
	maxChunkSize = 4 * 1024 * 1024 // 4MB
	chunkPrefix  = 0x00            // Special prefix indicating chunked data
)

// WriteGenesisStateSpec writes the genesis state specification into the disk.
// If the data is larger than 4MB, it will be split into chunks.
func WriteGenesisStateSpec(db ethdb.KeyValueWriter, blockhash common.Hash, data []byte) {
	key := genesisStateSpecKey(blockhash)

	if len(data) <= maxChunkSize {
		// Data is small enough, write directly
		if err := db.Put(key, data); err != nil {
			log.Crit("Failed to store genesis state", "err", err, "size", len(data))
		}
		log.Info("Genesis state written directly", "size", len(data))
		return
	}

	// Data is too large, split into chunks
	numChunks := (len(data) + maxChunkSize - 1) / maxChunkSize // Ceiling division

	if numChunks > 65535 { // (1<<16) - 1
		log.Crit("Genesis state too large for chunking", "size", len(data), "max_chunks", 255)
	}

	// Write the special header indicating chunked data
	header := []byte{chunkPrefix, byte(numChunks)}
	if err := db.Put(key, header); err != nil {
		log.Crit("Failed to store genesis state header", "err", err, "chunks", numChunks)
	}

	// Write each chunk
	for i := 0; i < numChunks; i++ {
		start := i * maxChunkSize
		end := start + maxChunkSize
		if end > len(data) {
			end = len(data)
		}

		chunkKey := append(key, byte(i)) // Append chunk index to original key
		chunk := data[start:end]

		if err := db.Put(chunkKey, chunk); err != nil {
			log.Crit("Failed to store genesis state chunk", "err", err, "chunk", i, "size", len(chunk))
		}
	}

	log.Info("Genesis state written in chunks", "total_size", len(data), "chunks", numChunks, "chunk_size", maxChunkSize)
}

// ReadGenesisStateSpec retrieves the genesis state specification based on the
// given genesis (block-)hash. Handles both direct and chunked data.
func ReadGenesisStateSpec(db ethdb.KeyValueReader, blockhash common.Hash) []byte {
	key := genesisStateSpecKey(blockhash)
	data, _ := db.Get(key)

	if len(data) == 0 {
		return nil
	}

	// Check if this is chunked data
	if len(data) == 2 && data[0] == chunkPrefix {
		numChunks := int(data[1])

		if numChunks == 0 || numChunks > 65535 {
			log.Error("Invalid chunk count in genesis state header", "chunks", numChunks)
			return nil
		}

		// Read all chunks
		var chunks [][]byte
		totalSize := 0

		for i := 0; i < numChunks; i++ {
			chunkKey := append(key, byte(i))
			chunk, _ := db.Get(chunkKey)
			if len(chunk) == 0 {
				log.Error("Missing genesis state chunk", "chunk", i, "total_chunks", numChunks)
				return nil
			}
			chunks = append(chunks, chunk)
			totalSize += len(chunk)
		}

		// Combine all chunks
		result := make([]byte, 0, totalSize)
		for _, chunk := range chunks {
			result = append(result, chunk...)
		}

		log.Info("Genesis state read from chunks", "total_size", totalSize, "chunks", numChunks)
		return result
	}

	// Direct data (not chunked)
	log.Info("Genesis state read directly", "size", len(data))
	return data
}

// crashList is a list of unclean-shutdown-markers, for rlp-encoding to the
// database
type crashList struct {
	Discarded uint64   // how many ucs have we deleted
	Recent    []uint64 // unix timestamps of 10 latest unclean shutdowns
}

const crashesToKeep = 10

// PushUncleanShutdownMarker appends a new unclean shutdown marker and returns
// the previous data
// - a list of timestamps
// - a count of how many old unclean-shutdowns have been discarded
func PushUncleanShutdownMarker(db ethdb.KeyValueStore) ([]uint64, uint64, error) {
	var uncleanShutdowns crashList
	// Read old data
	if data, err := db.Get(uncleanShutdownKey); err == nil {
		if err := rlp.DecodeBytes(data, &uncleanShutdowns); err != nil {
			return nil, 0, err
		}
	}
	var discarded = uncleanShutdowns.Discarded
	var previous = make([]uint64, len(uncleanShutdowns.Recent))
	copy(previous, uncleanShutdowns.Recent)
	// Add a new (but cap it)
	uncleanShutdowns.Recent = append(uncleanShutdowns.Recent, uint64(time.Now().Unix()))
	if count := len(uncleanShutdowns.Recent); count > crashesToKeep+1 {
		numDel := count - (crashesToKeep + 1)
		uncleanShutdowns.Recent = uncleanShutdowns.Recent[numDel:]
		uncleanShutdowns.Discarded += uint64(numDel)
	}
	// And save it again
	data, _ := rlp.EncodeToBytes(uncleanShutdowns)
	if err := db.Put(uncleanShutdownKey, data); err != nil {
		log.Warn("Failed to write unclean-shutdown marker", "err", err)
		return nil, 0, err
	}
	return previous, discarded, nil
}

// PopUncleanShutdownMarker removes the last unclean shutdown marker
func PopUncleanShutdownMarker(db ethdb.KeyValueStore) {
	var uncleanShutdowns crashList
	// Read old data
	if data, err := db.Get(uncleanShutdownKey); err != nil {
		log.Warn("Error reading unclean shutdown markers", "error", err)
	} else if err := rlp.DecodeBytes(data, &uncleanShutdowns); err != nil {
		log.Error("Error decoding unclean shutdown markers", "error", err) // Should mos def _not_ happen
	}
	if l := len(uncleanShutdowns.Recent); l > 0 {
		uncleanShutdowns.Recent = uncleanShutdowns.Recent[:l-1]
	}
	data, _ := rlp.EncodeToBytes(uncleanShutdowns)
	if err := db.Put(uncleanShutdownKey, data); err != nil {
		log.Warn("Failed to clear unclean-shutdown marker", "err", err)
	}
}

// UpdateUncleanShutdownMarker updates the last marker's timestamp to now.
func UpdateUncleanShutdownMarker(db ethdb.KeyValueStore) {
	var uncleanShutdowns crashList
	// Read old data
	if data, err := db.Get(uncleanShutdownKey); err != nil {
		log.Warn("Error reading unclean shutdown markers", "error", err)
	} else if err := rlp.DecodeBytes(data, &uncleanShutdowns); err != nil {
		log.Warn("Error decoding unclean shutdown markers", "error", err)
	}
	// This shouldn't happen because we push a marker on Backend instantiation
	count := len(uncleanShutdowns.Recent)
	if count == 0 {
		log.Warn("No unclean shutdown marker to update")
		return
	}
	uncleanShutdowns.Recent[count-1] = uint64(time.Now().Unix())
	data, _ := rlp.EncodeToBytes(uncleanShutdowns)
	if err := db.Put(uncleanShutdownKey, data); err != nil {
		log.Warn("Failed to write unclean-shutdown marker", "err", err)
	}
}

// ReadTransitionStatus retrieves the eth2 transition status from the database
func ReadTransitionStatus(db ethdb.KeyValueReader) []byte {
	data, _ := db.Get(transitionStatusKey)
	return data
}

// WriteTransitionStatus stores the eth2 transition status to the database
func WriteTransitionStatus(db ethdb.KeyValueWriter, data []byte) {
	if err := db.Put(transitionStatusKey, data); err != nil {
		log.Crit("Failed to store the eth2 transition status", "err", err)
	}
}
