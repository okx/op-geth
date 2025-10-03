package core

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
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
	currentBlockNumber := rawdb.ReadHeaderNumber(chainDb, currentBlockHash)
	if currentBlockNumber == nil {
		return errors.New("commitXLayerFirstBlock: current block number not found")
	}

	if *currentBlockNumber != 0 {
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
