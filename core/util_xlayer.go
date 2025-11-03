package core

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// generateFirstXLayerBlock creates the first xlayer block based on the genesis block.
// It reads the genesis block and creates a new block with the same content but with LegacyXLayerBlock number.
func GenerateFirstXLayerBlock(chainDb ethdb.Database, chainConfig *params.ChainConfig) (*types.Block, error) {
	// Get genesis block hash
	genesisHash := rawdb.ReadCanonicalHash(chainDb, 0)
	if (genesisHash == common.Hash{}) {
		return nil, errors.New("commitXLayerFirstBlock: genesis block not found")
	}
	// fetch genesis block from database
	genesisBlock := rawdb.ReadBlock(chainDb, genesisHash, 0)
	if genesisBlock == nil {
		return nil, errors.New("commitXLayerFirstBlock: genesis block not found in database")
	}

	fistXLayerBlockHeader := genesisBlock.Header()
	fistXLayerBlockHeader.Number = big.NewInt(int64(chainConfig.LegacyXLayerBlock.Uint64()))
	log.Info("commitXLayerFirstBlock: override genesis block number", "number", fistXLayerBlockHeader.Number.Uint64(), "configNumber", chainConfig.LegacyXLayerBlock.Uint64())

	// Create a new block with the modified header
	return types.NewBlockWithHeader(fistXLayerBlockHeader), nil
}

// commitXLayerFirstBlock commits the first xlayer block when current block number is 0.
// It takes the genesis block and modifies its number to LegacyXLayerBlock number.
func CommitXLayerFirstBlock(chainDb ethdb.Database, chainConfig *params.ChainConfig) error {
	// get current block number
	currentBlockHash := rawdb.ReadHeadBlockHash(chainDb)
	if currentBlockHash == (common.Hash{}) {
		return errors.New("commitXLayerFirstBlock: current block hash not found")
	}
	currentBlockNumber, ok := rawdb.ReadHeaderNumber(chainDb, currentBlockHash)
	if !ok {
		return errors.New("commitXLayerFirstBlock: current block number not found")
	}

	if currentBlockNumber != 0 {
		return nil
	}

	log.Info("commitXLayerFirstBlock: current block number is 0, should write xlayer first block")
	fistXLayerBlock, err := GenerateFirstXLayerBlock(chainDb, chainConfig)
	if err != nil {
		return err
	}

	batch := chainDb.NewBatch()
	rawdb.WriteBlock(batch, fistXLayerBlock)
	rawdb.WriteReceipts(batch, fistXLayerBlock.Hash(), fistXLayerBlock.NumberU64(), nil)
	rawdb.WriteCanonicalHash(batch, fistXLayerBlock.Hash(), fistXLayerBlock.NumberU64())
	rawdb.WriteHeadBlockHash(batch, fistXLayerBlock.Hash())
	rawdb.WriteHeadFastBlockHash(batch, fistXLayerBlock.Hash())
	rawdb.WriteHeadHeaderHash(batch, fistXLayerBlock.Hash())
	rawdb.WriteChainConfig(batch, fistXLayerBlock.Hash(), chainConfig)
	if err = batch.Write(); err != nil {
		return err
	}
	log.Info("commitXLayerFirstBlock", "err", err, "number", fistXLayerBlock.NumberU64(), "hash", fistXLayerBlock.Hash())
	return err
}

// RestoreGenesisBlockFromFreezer checks if block 0 exists in the freezer but not in leveldb,
// and restores it from freezer to leveldb if needed.
func RestoreGenesisBlockFromFreezer(chainDb ethdb.Database) error {
	// Check if block 0 exists in leveldb
	genesisHashFromLeveldb := rawdb.ReadCanonicalHash(chainDb, 0)
	if genesisHashFromLeveldb != (common.Hash{}) {
		// Block 0 exists in leveldb, check if we can read the full block
		genesisBlock := rawdb.ReadBlock(chainDb, genesisHashFromLeveldb, 0)
		if genesisBlock != nil {
			log.Info("RestoreGenesisBlockFromFreezer: block 0 exists in leveldb, no need to restore")
			return nil
		}
	}

	log.Info("RestoreGenesisBlockFromFreezer: block 0 is missing from db, recover from ancientdb")

	// Block 0 is missing from leveldb, check if it exists in freezer
	var (
		hashInFreezer     []byte
		headerInFreezer   []byte
		bodyInFreezer     []byte
		receiptsInFreezer []byte
		hasDataInFreezer  bool
	)

	err := chainDb.ReadAncients(func(reader ethdb.AncientReaderOp) error {
		var err error
		hashInFreezer, err = reader.Ancient(rawdb.ChainFreezerHashTable, 0)
		if err != nil || len(hashInFreezer) == 0 {
			return nil // No data in freezer
		}

		headerInFreezer, err = reader.Ancient(rawdb.ChainFreezerHeaderTable, 0)
		if err != nil || len(headerInFreezer) == 0 {
			return nil
		}

		// Read body and receipts (they might be empty for genesis block)
		bodyInFreezer, _ = reader.Ancient(rawdb.ChainFreezerBodiesTable, 0)
		receiptsInFreezer, _ = reader.Ancient(rawdb.ChainFreezerReceiptTable, 0)

		hasDataInFreezer = true
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to read from freezer: %v", err)
	}

	if !hasDataInFreezer {
		// No data in freezer, nothing to restore
		return nil
	}

	// We have data in freezer, restore it to leveldb
	log.Info("RestoreGenesisBlockFromFreezer: restoring block 0 from freezer to leveldb")

	genesisHash := common.BytesToHash(hashInFreezer)

	// Decode header
	var header types.Header
	if err := rlp.DecodeBytes(headerInFreezer, &header); err != nil {
		return fmt.Errorf("failed to decode genesis header: %v", err)
	}

	// Verify the header hash matches the canonical hash
	if header.Hash() != genesisHash {
		return fmt.Errorf("header hash mismatch: expected %v, got %v", genesisHash, header.Hash())
	}

	// Decode body
	var body types.Body
	if len(bodyInFreezer) > 0 {
		if err := rlp.DecodeBytes(bodyInFreezer, &body); err != nil {
			return fmt.Errorf("failed to decode genesis body: %v", err)
		}
	}

	// Reconstruct the block
	genesisBlock := types.NewBlockWithHeader(&header).WithBody(body)

	// Write to leveldb
	batch := chainDb.NewBatch()
	rawdb.WriteBlock(batch, genesisBlock)
	rawdb.WriteCanonicalHash(batch, genesisHash, 0)

	// Write receipts if available
	if len(receiptsInFreezer) > 0 {
		rawdb.WriteRawReceipts(batch, genesisHash, 0, receiptsInFreezer)
	}

	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to write genesis block to leveldb: %v", err)
	}

	log.Info("RestoreGenesisBlockFromFreezer: successfully restored block 0", "hash", genesisHash)
	return nil
}
