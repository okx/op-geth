// Copyright 2024 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/urfave/cli/v2"
)

// MigrationConfig represents configuration for state migration
type MigrationConfig struct {
	// MigrationPath is the path to the mdbx database for state migration
	MigrationPath string
	// IgnoreAddresses is a set of addresses to ignore during migration
	IgnoreAddresses map[common.Address]struct{}
	// MigrationSMTPath is the path to the SMT database for migration
	MigrationSMTPath string
	// IgnoreSMTVerify indicates whether to ignore SMT verification during migration
	IgnoreSMTVerify bool
}

// Bucket names for mdbx database
const (
	PlainStateBucket = "PlainState"
	CodeBucket       = "Code"
)

// Empty code hash constant
var EmptyCodeHash = common.Hash{}

// bytesToUint64 converts a byte slice to uint64
func bytesToUint64(data []byte) uint64 {
	var result uint64
	for i, b := range data {
		result |= uint64(b) << (8 * (len(data) - 1 - i))
	}
	return result
}

// decodeAccountData decodes account data in mdbx CBOR format
// This implements the CBOR decoding logic from DecodeForStorage
func decodeAccountData(enc []byte) (*types.Account, error) {
	account := &types.Account{
		Storage: make(map[common.Hash]common.Hash),
	}

	if len(enc) == 0 {
		return account, nil
	}

	var fieldSet = enc[0]
	var pos = 1

	// Field 1: Nonce
	if fieldSet&1 > 0 {
		if pos >= len(enc) {
			return account, fmt.Errorf("malformed CBOR: missing nonce length")
		}
		decodeLength := int(enc[pos])

		if len(enc) < pos+decodeLength+1 {
			return account, fmt.Errorf("malformed CBOR for Account.Nonce: length %d, available %d", decodeLength, len(enc)-pos-1)
		}

		nonce := bytesToUint64(enc[pos+1 : pos+decodeLength+1])
		account.Nonce = nonce

		pos += decodeLength + 1
	}

	// Field 2: Balance
	if fieldSet&2 > 0 {
		if pos >= len(enc) {
			return account, fmt.Errorf("malformed CBOR: missing balance length")
		}
		decodeLength := int(enc[pos])

		if len(enc) < pos+decodeLength+1 {
			return account, fmt.Errorf("malformed CBOR for Account.Balance: length %d, available %d", decodeLength, len(enc)-pos-1)
		}

		balance := new(big.Int).SetBytes(enc[pos+1 : pos+decodeLength+1])
		if balance.Sign() > 0 {
			account.Balance = balance
		}
		pos += decodeLength + 1
	}

	// Field 3: Incarnation (skip for now)
	if fieldSet&4 > 0 {
		if pos >= len(enc) {
			return account, fmt.Errorf("malformed CBOR: missing incarnation length")
		}
		decodeLength := int(enc[pos])

		if len(enc) < pos+decodeLength+1 {
			return account, fmt.Errorf("malformed CBOR for Account.Incarnation: length %d, available %d", decodeLength, len(enc)-pos-1)
		}

		// Skip incarnation data
		pos += decodeLength + 1
	}

	// Field 4: CodeHash
	if fieldSet&8 > 0 {
		if pos >= len(enc) {
			return account, fmt.Errorf("malformed CBOR: missing codehash length")
		}
		decodeLength := int(enc[pos])

		if decodeLength != 32 {
			return account, fmt.Errorf("codehash should be 32 bytes long, got %d instead", decodeLength)
		}

		if len(enc) < pos+decodeLength+1 {
			return account, fmt.Errorf("malformed CBOR for Account.CodeHash: length %d, available %d", decodeLength, len(enc)-pos-1)
		}

		codeHash := common.BytesToHash(enc[pos+1 : pos+decodeLength+1])
		if codeHash != EmptyCodeHash {
			// Store codeHash for later code retrieval
			account.Code = codeHash[:]
		}
		pos += decodeLength + 1
	}

	// Ensure Balance is not nil
	if account.Balance == nil {
		account.Balance = big.NewInt(0)
	}

	return account, nil
}

// ScanDB scans the mdbx database and returns the ignored genesis alloc
func ScanDB(migrationPath string, ga *types.GenesisAlloc, ignoreAddresses map[common.Address]struct{}) (types.GenesisAlloc, error) {
	start := time.Now()
	log.Info("Starting ScanDB", "path", migrationPath)

	// Open database with proper error handling
	db, err := mdbx.Open(migrationPath, nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to open migration database: %w", err)
	}
	defer db.Close()

	ignoredAlloc := make(types.GenesisAlloc)

	if err := db.View(context.Background(), func(tx kv.Tx) error {
		return tx.ForEach(PlainStateBucket, nil, func(k, v []byte) error {
			// Process accounts (keys with length 20)
			if len(k) == 20 {
				addr := common.BytesToAddress(k)

				// Fixme: if xlayer account balance or nonce conflict with target node(such as op-geth), currently we use op-geth
				if _, exists := (*ga)[addr]; exists {
					log.Warn("account state conflict found", "account", addr.Hex())
					return nil
				}
				// Decode account data
				genesisAccount, err := decodeAccountData(v)
				if err != nil {
					log.Warn("Failed to decode account", "address", addr.Hex(), "error", err)
					return nil
				}

				// Get code data if account has code
				if len(genesisAccount.Code) > 0 {
					codeHash := common.BytesToHash(genesisAccount.Code)
					if codeHash != EmptyCodeHash {
						code, err := tx.GetOne(CodeBucket, codeHash[:])
						if err == nil && len(code) > 0 {
							// Make a copy to avoid potential memory issues
							genesisAccount.Code = make([]byte, len(code))
							copy(genesisAccount.Code, code)
						}
					}
				}

				// Check if address should be ignored
				if _, shouldIgnore := ignoreAddresses[addr]; shouldIgnore {
					ignoredAlloc[addr] = *genesisAccount
				} else {
					// Update the original ga
					(*ga)[addr] = *genesisAccount
				}
			}

			// Process storage (keys with length > 28)
			if len(k) > 28 {
				addr := common.BytesToAddress(k[:20])

				storageKey := common.BytesToHash(k[28:])
				storageValue := common.BytesToHash(v)

				// Check if address should be ignored
				if _, shouldIgnore := ignoreAddresses[addr]; shouldIgnore {
					// Add storage to the corresponding account in ignoredAlloc
					if account, exists := ignoredAlloc[addr]; exists {
						if account.Storage == nil {
							account.Storage = make(map[common.Hash]common.Hash)
						}
						account.Storage[storageKey] = storageValue
						ignoredAlloc[addr] = account
					}
				} else {
					// Add storage to the corresponding account in ga
					if account, exists := (*ga)[addr]; exists {
						if account.Storage == nil {
							account.Storage = make(map[common.Hash]common.Hash)
						}
						account.Storage[storageKey] = storageValue
						(*ga)[addr] = account
					}
				}
			}
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("failed to scan migration database: %w", err)
	}

	log.Info("ScanDB completed", "ignored_accounts", len(ignoredAlloc), "genesis_accounts", len(*ga), "elapsed", time.Since(start))

	return ignoredAlloc, nil
}

// verifySMT verifies the SMT (Sparse Merkle Tree) data
func verifySMT(migrationPath string, genesisAlloc, ignoredAlloc *types.GenesisAlloc) error {

	// TODO: Implement SMT verification logic
	// Should consider both ga and ignoredAlloc

	log.Info("verifySMT called", "migrationPath", migrationPath, "accounts", len(*genesisAlloc))
	return nil
}

// SetupGenesisBlockWithMigrationData sets up the genesis block with migration data
func SetupGenesisBlockWithMigrationData(chaindb ethdb.Database, triedb *triedb.Database, genesis *Genesis, overrides *ChainOverrides, ctx *cli.Context) (*params.ChainConfig, common.Hash, *params.ConfigCompatError, error) {
	// Get migration path from CLI context
	migrationPath := ctx.String("chaindata")
	if migrationPath == "" {
		return nil, common.Hash{}, nil, fmt.Errorf("migration path is required")
	}

	// Parse ignore addresses from command line
	ignoreAddresses := make(map[common.Address]struct{})
	ignoreAddressesStr := ctx.String("ignore-addresses")
	if ignoreAddressesStr != "" {
		addresses := strings.Split(ignoreAddressesStr, ",")
		for _, addrStr := range addresses {
			addrStr = strings.TrimSpace(addrStr)
			if addrStr != "" {
				if addr := common.HexToAddress(addrStr); addr != (common.Address{}) {
					ignoreAddresses[addr] = struct{}{}
				} else {
					return nil, common.Hash{}, nil, fmt.Errorf("invalid address format: %s", addrStr)
				}
			}
		}
	}

	// Create migration config
	migrationConfig := &MigrationConfig{
		MigrationPath:    migrationPath,
		IgnoreAddresses:  ignoreAddresses,
		MigrationSMTPath: ctx.String("smt-db-path"),
		IgnoreSMTVerify:  ctx.Bool("ignore-smt-verify"),
	}

	// Scan migration database and update genesis.Alloc
	log.Info("Scanning migration database to update genesis alloc")
	ignoredAlloc, err := ScanDB(migrationConfig.MigrationPath, &genesis.Alloc, ignoreAddresses) // genesis.Alloc will be updated
	if err != nil {
		return nil, common.Hash{}, nil, fmt.Errorf("failed to scan migration database: %w", err)
	}

	var wg sync.WaitGroup
	var smtErr error
	var setupErr error
	var hash common.Hash
	var compatErr *params.ConfigCompatError
	var cfg *params.ChainConfig

	// Start verifySMT in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		smtErr = verifySMT(migrationConfig.MigrationPath, &genesis.Alloc, &ignoredAlloc)
	}()
	// Start SetupGenesisBlockWithOverride
	wg.Add(1)
	go func() {
		defer wg.Done()
		cfg, hash, compatErr, setupErr = SetupGenesisBlockWithOverride(chaindb, triedb, genesis, overrides)
	}()
	// Wait for both goroutines to complete
	wg.Wait()
	// Check for errors
	if smtErr != nil {
		return nil, common.Hash{}, nil, fmt.Errorf("SMT verification failed: %w", smtErr)
	}
	if setupErr != nil {
		return nil, common.Hash{}, nil, fmt.Errorf("failed to setup genesis block: %w", setupErr)
	}

	log.Info("Updated genesis alloc with migration data", "ignored_accounts", len(ignoredAlloc), "updated_accounts", len(genesis.Alloc))

	return cfg, hash, compatErr, setupErr
}
