//go:build !cgo

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
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
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
		Balance: big.NewInt(0),
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

	return account, nil
}

func mergeConflictAccount(addr common.Address, destAccount, genesisAccount *types.Account) {
	switch addr {
	// TODO: implement conflict cases here:
	// case params.WithdrawalQueueAddress:
	// 	dbAccount.Balance = genesisAccount.Balance
	// 	dbAccount.Nonce = genesisAccount.Nonce
	// 	dbAccount.Code = genesisAccount.Code
	// 	dbAccount.Storage = genesisAccount.Storage
	default:
		destAccount.Balance = genesisAccount.Balance
		destAccount.Nonce = genesisAccount.Nonce
		destAccount.Code = genesisAccount.Code
		destAccount.Storage = genesisAccount.Storage
	}
}

func mergeGenesisAlloc(dbAlloc, genesisAlloc *types.GenesisAlloc) (types.GenesisAlloc, error) {
	overridedAlloc := make(types.GenesisAlloc)
	for addr, account := range *genesisAlloc {
		if dbAccount, exists := (*dbAlloc)[addr]; exists {
			overridedAlloc[addr] = types.Account{
				Balance: dbAccount.Balance,
				Nonce:   dbAccount.Nonce,
				Code:    dbAccount.Code,
				Storage: dbAccount.Storage,
			}
			mergeConflictAccount(addr, &dbAccount, &account)
		} else {
			if account.Balance == nil {
				account.Balance = big.NewInt(0)
			}

			(*dbAlloc)[addr] = account
		}
	}

	return overridedAlloc, nil
}

// ScanDB scans the mdbx database and returns the ignored genesis alloc
func ScanDB(migrationPath string, genesis *Genesis, ignoreAddresses map[common.Address]struct{}) (types.GenesisAlloc, types.GenesisAlloc, error) {
	panic("not implemented")
}

// verifySMT verifies the SMT (Sparse Merkle Tree) data
func verifySMT(migrationPath string, alloc, ignoredAlloc, overridedAlloc *types.GenesisAlloc) error {

	// TODO: Implement SMT verification logic
	// Should consider both ga and ignoredAlloc

	log.Info("verifySMT called", "migrationPath", migrationPath, "accounts", len(*alloc))
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
	ignoredAlloc, overridedAlloc, err := ScanDB(migrationConfig.MigrationPath, genesis, ignoreAddresses) // genesis.Alloc will be updated
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
		smtErr = verifySMT(migrationConfig.MigrationPath, &genesis.Alloc, &ignoredAlloc, &overridedAlloc)
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
