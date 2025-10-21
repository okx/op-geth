package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var genesisNumberKey = []byte("GenesisNumber")

// WriteGenesisNumber writes genesis number to database
func WriteGenesisNumber(db ethdb.KeyValueWriter, number uint64) {
	data := encodeBlockNumber(number)
	if err := db.Put(genesisNumberKey, data); err != nil {
		log.Crit("Failed to write genesis number", "err", err)
	}
}

// ReadGenesisNumber returns the genesis number
func ReadGenesisNumber(db ethdb.KeyValueReader) uint64 {
	data, err := db.Get(genesisNumberKey)
	if err != nil {
		log.Warn("Failed to read genesis number", "err", err)
		return 0
	}
	return binary.BigEndian.Uint64(data)
}
