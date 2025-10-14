package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/assert"
)

func TestGenerateFirstXlayerBlock(t *testing.T) {
	// Create a test chain config with XLayer enabled
	chainConfig := &params.ChainConfig{
		ChainID:           big.NewInt(1),
		LegacyXLayerBlock: big.NewInt(1000),
	}

	tests := []struct {
		name          string
		setupDB       func(db ethdb.Database)
		expectedError string
	}{
		{
			name: "no genesis block",
			setupDB: func(db ethdb.Database) {
				// Empty database
			},
			expectedError: "commitXLayerFirstBlock: genesis block not found",
		},
		{
			name: "genesis block not in database",
			setupDB: func(db ethdb.Database) {
				// Write canonical hash but no block
				rawdb.WriteCanonicalHash(db, common.HexToHash("0x123"), 0)
			},
			expectedError: "commitXLayerFirstBlock: genesis block not found in database",
		},
		{
			name: "successful generation",
			setupDB: func(db ethdb.Database) {
				// Create and write genesis block
				header := &types.Header{
					Number:     big.NewInt(0),
					Extra:      []byte("test genesis"),
					Time:       1234,
					Difficulty: big.NewInt(100),
					GasLimit:   8000000,
				}
				block := types.NewBlockWithHeader(header)
				rawdb.WriteBlock(db, block)
				rawdb.WriteCanonicalHash(db, block.Hash(), 0)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new database for each test
			testDB := rawdb.NewMemoryDatabase()

			// Setup test case
			tt.setupDB(testDB)

			// Execute test
			block, err := GenerateFirstXLayerBlock(testDB, chainConfig)

			// Verify results
			if tt.expectedError == "" {
				assert.NoError(t, err)
				assert.NotNil(t, block)
				assert.Equal(t, chainConfig.LegacyXLayerBlock.Uint64(), block.NumberU64())

				// Verify block header fields were preserved except number
				genesisHash := rawdb.ReadCanonicalHash(testDB, 0)
				genesisBlock := rawdb.ReadBlock(testDB, genesisHash, 0)
				assert.NotNil(t, genesisBlock)

				assert.Equal(t, genesisBlock.Header().Extra, block.Header().Extra)
				assert.Equal(t, genesisBlock.Header().Time, block.Header().Time)
				assert.Equal(t, genesisBlock.Header().Difficulty, block.Header().Difficulty)
				assert.Equal(t, genesisBlock.Header().GasLimit, block.Header().GasLimit)
			} else {
				assert.EqualError(t, err, tt.expectedError)
				assert.Nil(t, block)
			}
		})
	}
}

func TestCommitXlayerFirstBlock(t *testing.T) {
	// Create a test chain config with XLayer enabled
	chainConfig := &params.ChainConfig{
		ChainID:           big.NewInt(1),
		LegacyXLayerBlock: big.NewInt(1000),
	}

	tests := []struct {
		name          string
		setupDB       func(db ethdb.Database)
		expectedError string
	}{
		{
			name: "no current block hash",
			setupDB: func(db ethdb.Database) {
				// Empty database
			},
			expectedError: "commitXLayerFirstBlock: current block hash not found",
		},
		{
			name: "no current block number",
			setupDB: func(db ethdb.Database) {
				hash := common.HexToHash("0x123")
				rawdb.WriteHeadBlockHash(db, hash)
			},
			expectedError: "commitXLayerFirstBlock: current block number not found",
		},
		{
			name: "non-zero current block",
			setupDB: func(db ethdb.Database) {
				hash := common.HexToHash("0x123")
				rawdb.WriteHeadBlockHash(db, hash)
				rawdb.WriteHeaderNumber(db, hash, 10)
			},
			expectedError: "",
		},
		{
			name: "no genesis block",
			setupDB: func(db ethdb.Database) {
				hash := common.HexToHash("0x123")
				rawdb.WriteHeadBlockHash(db, hash)
				rawdb.WriteHeaderNumber(db, hash, 0)
			},
			expectedError: "commitXLayerFirstBlock: genesis block not found",
		},
		{
			name: "successful commit",
			setupDB: func(db ethdb.Database) {
				// Write head block hash and number (0)
				hash := common.HexToHash("0x123")
				rawdb.WriteHeadBlockHash(db, hash)
				rawdb.WriteHeaderNumber(db, hash, 0)

				// Create and write genesis block
				header := &types.Header{
					Number: big.NewInt(0),
					Extra:  []byte("test genesis"),
					Time:   1234,
				}
				block := types.NewBlockWithHeader(header)
				rawdb.WriteBlock(db, block)
				rawdb.WriteCanonicalHash(db, block.Hash(), 0)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new database for each test
			testDB := rawdb.NewMemoryDatabase()

			// Setup test case
			tt.setupDB(testDB)

			// Execute test
			err := CommitXLayerFirstBlock(testDB, chainConfig)

			// Verify results
			if tt.expectedError == "" {
				assert.NoError(t, err)
				if tt.name == "successful commit" {
					// Verify the xlayer block was written correctly
					headHash := rawdb.ReadHeadBlockHash(testDB)
					headNumber, ok := rawdb.ReadHeaderNumber(testDB, headHash)
					assert.True(t, ok)
					assert.Equal(t, chainConfig.LegacyXLayerBlock.Uint64(), headNumber)

					// Verify block contents
					block := rawdb.ReadBlock(testDB, headHash, headNumber)
					assert.NotNil(t, block)
					assert.Equal(t, chainConfig.LegacyXLayerBlock.Uint64(), block.NumberU64())

					// Verify chain config was written
					storedConfig := rawdb.ReadChainConfig(testDB, headHash)
					assert.NotNil(t, storedConfig)
					assert.Equal(t, chainConfig.ChainID, storedConfig.ChainID)
					assert.Equal(t, chainConfig.LegacyXLayerBlock, storedConfig.LegacyXLayerBlock)
				}
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}
		})
	}
}

func TestCommitXlayerFirstBlock_WithGenesisBlock(t *testing.T) {
	// Create an in-memory database
	db := rawdb.NewMemoryDatabase()

	// Create a test chain config
	chainConfig := &params.ChainConfig{
		ChainID:           big.NewInt(1),
		LegacyXLayerBlock: big.NewInt(1000),
	}

	// Create a genesis block with some state
	genesisHeader := &types.Header{
		Number:     big.NewInt(0),
		Extra:      []byte("test genesis"),
		Time:       1234,
		Difficulty: big.NewInt(100),
		GasLimit:   8000000,
	}
	genesisBlock := types.NewBlockWithHeader(genesisHeader)

	// Write genesis block
	rawdb.WriteBlock(db, genesisBlock)
	rawdb.WriteCanonicalHash(db, genesisBlock.Hash(), 0)
	rawdb.WriteHeadBlockHash(db, genesisBlock.Hash())
	rawdb.WriteHeaderNumber(db, genesisBlock.Hash(), 0)

	// Execute test
	err := CommitXLayerFirstBlock(db, chainConfig)
	assert.NoError(t, err)

	// Verify the xlayer block
	headHash := rawdb.ReadHeadBlockHash(db)
	headNumber, ok := rawdb.ReadHeaderNumber(db, headHash)
	assert.True(t, ok)
	assert.Equal(t, chainConfig.LegacyXLayerBlock.Uint64(), headNumber)

	// Read the written block
	block := rawdb.ReadBlock(db, headHash, headNumber)
	assert.NotNil(t, block)

	// Verify block header fields were preserved except number
	assert.Equal(t, chainConfig.LegacyXLayerBlock.Uint64(), block.NumberU64())
	assert.Equal(t, genesisHeader.Extra, block.Header().Extra)
	assert.Equal(t, genesisHeader.Time, block.Header().Time)
	assert.Equal(t, genesisHeader.Difficulty, block.Header().Difficulty)
	assert.Equal(t, genesisHeader.GasLimit, block.Header().GasLimit)
}
