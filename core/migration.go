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
	"github.com/google/btree"
	"math/big"
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/logger"

	"github.com/bytedance/sonic"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	erigonlog "github.com/ledgerwatch/log/v3"
	"github.com/urfave/cli/v2"
)

// MigrationConfig represents configuration for state migration
type MigrationConfig struct {
	// ChainDataPath is the path to the mdbx database for state migration
	ChainDataPath string
	// IgnoreAddresses is a set of addresses to ignore during migration
	IgnoreAddresses map[common.Address]struct{}
	// SMTDataPath is the path to the SMT database for migration
	SMTDataPath string
	// IgnoreSMTVerify indicates whether to ignore SMT verification during migration
	IgnoreSMTVerify bool
}

// Bucket names for mdbx database
const (
	PlainStateBucket = "PlainState"
	CodeBucket       = "Code"
)

var ERIGON_SCALABLE_ADDRESS = common.HexToAddress("0x000000000000000000000000000000005ca1ab1e")

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

func IsEmptyAccount(acct types.Account) bool {
	return acct.Nonce == 0 &&
		(acct.Balance == nil || acct.Balance.Cmp(big.NewInt(0)) == 0) && len(acct.Code) == 0
}

func mergeConflictAccount(addr common.Address, xlayerErigonAcct, opGenesisAcct *types.Account) types.Account {

	var destAccount types.Account
	switch addr {
	// black hole on XLayer with no code or storage
	// WETH preinstalled on OP-stack
	// so we use nonce from xlayer, but code from op
	// both xlayer and op have no storage for this address
	case common.HexToAddress("0x4200000000000000000000000000000000000006"):
		destAccount.Nonce = xlayerErigonAcct.Nonce
		// The address 0x4200000000000000000000000000000000000006 has only a small amount of OKB, 0.000011 on 11th Sep.
		// For compatibility and security reasons, we will zero out the balance of this address.
		logger.Warn("mergeAlloc: clear balance for 0x4200000000000000000000000000000000000006", "balance", xlayerErigonAcct.Balance)
		if len(xlayerErigonAcct.Code) != 0 {
			logger.Error("mergeAlloc: black hole has code", "code length", len(xlayerErigonAcct.Code))
		}
		destAccount.Code = opGenesisAcct.Code
		if len(xlayerErigonAcct.Storage) != 0 {
			logger.Error("mergeAlloc: black hole has storage", "storage length", len(xlayerErigonAcct.Storage))
		}
		// `create2Deployer` on both xlayer and op stack
		// op uses a version of code that does not have an owner, so we use nonce and balance from xlayer, but the code from op
	case common.HexToAddress("0x13b0d85ccb8bf860b6b79af3029fca081ae9bef2"):
		destAccount.Nonce = xlayerErigonAcct.Nonce
		destAccount.Balance = xlayerErigonAcct.Balance
		destAccount.Code = opGenesisAcct.Code
		if len(xlayerErigonAcct.Code) == 0 {
			logger.Error("mergeAlloc: create2Deployer has no code")
		}
		if len(xlayerErigonAcct.Storage) != 0 {
			logger.Error("mergeAlloc: create2Deployer has storage", "storage length", len(xlayerErigonAcct.Storage))
		}
		// Permit2 use code and storage from xlayer
	case common.HexToAddress("000000000022d473030f116ddee9f6b43ac78ba3"):
		destAccount.Nonce = xlayerErigonAcct.Nonce
		destAccount.Balance = xlayerErigonAcct.Balance
		destAccount.Code = xlayerErigonAcct.Code
		destAccount.Storage = xlayerErigonAcct.Storage
		if len(xlayerErigonAcct.Code) == 0 {
			logger.Error("mergeAlloc: permit2 has no code")
		}
		if len(xlayerErigonAcct.Storage) != 0 {
			logger.Error("mergeAlloc: permit2 has storage", "storage length", len(xlayerErigonAcct.Storage))
		}
	default:
		destAccount.Balance = opGenesisAcct.Balance
		destAccount.Nonce = opGenesisAcct.Nonce
		destAccount.Code = opGenesisAcct.Code
		destAccount.Storage = opGenesisAcct.Storage
	}

	return destAccount
}

// generateMigrateAlloc filters out ignored addresses from dbAlloc, merges with genesisAlloc, and returns the final migrateAlloc
func generateMigrateAlloc(dbAlloc types.GenesisAlloc, ignoreAddresses map[common.Address]struct{}, genesisAlloc *types.GenesisAlloc) types.GenesisAlloc {
	start := time.Now()
	migrateAlloc := make(types.GenesisAlloc)

	// Remove ignored addresses from dbAlloc
	for addr, account := range dbAlloc {
		if _, exists := ignoreAddresses[addr]; !exists {
			if !IsEmptyAccount(account) {
				migrateAlloc[addr] = account
			} else {
				log.Warn("mergeAlloc: empty account found", "address", addr.Hex())
			}
		} else {
			log.Info("mergeAlloc: skipping ignored address", "address", addr.Hex())
		}
	}
	log.Info("mergeAlloc: filtered ignored addresses", "kept", len(migrateAlloc), "elapsed", time.Since(start))

	start = time.Now()
	// Merge with genesisAlloc and handle conflicts
	for addr, genesisAccount := range *genesisAlloc {
		var destAccount types.Account
		if dbAccount, exists := migrateAlloc[addr]; exists {
			log.Warn("mergeAlloc: account conflict detected", "address", addr.Hex())
			destAccount = mergeConflictAccount(addr, &dbAccount, &genesisAccount)
		} else {
			destAccount = genesisAccount
		}

		if destAccount.Balance == nil {
			destAccount.Balance = big.NewInt(0) // Should set to zero if not set
		}

		migrateAlloc[addr] = destAccount
	}
	log.Info("mergeAlloc: merge completed", "status", "✅", "total_accounts", len(migrateAlloc), "elapsed", time.Since(start))

	return migrateAlloc
}

// LoadErigonGenesisData load erigon db plainstate data
func LoadErigonGenesisData(migrationPath string) (types.GenesisAlloc, error) {
	start := time.Now()
	log.Info("LoadDB: scanning migration database", "path", migrationPath)
	db, err := setupDB(migrationPath)
	if err != nil {
		log.Error("LoadDB: failed to setup database", "err", err)
		return nil, err
	}
	defer db.Close()
	dbAlloc, err := ScanDB(db)
	if err != nil {
		return nil, fmt.Errorf("failed to scan migration database: %w", err)
	}
	log.Info("LoadDB: successfully loaded erigon genesis data", "status", "✅", "accounts", len(dbAlloc), "elapsed", time.Since(start))
	return dbAlloc, nil
}

func setupDB(migrationPath string) (kv.RwDB, error) {
	log.Info("LoadDB: setting up database", "path", migrationPath)
	opts := mdbx.NewMDBX(erigonlog.New()).Path(migrationPath)
	db, err := opts.Open(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to open migration database: %w", err)
	}
	log.Info("LoadDB: database opened successfully")
	return db, nil
}

// ScanDB scans the mdbx database and returns the dbAlloc
func ScanDB(db kv.RoDB) (types.GenesisAlloc, error) {
	start := time.Now()
	dbAlloc := make(types.GenesisAlloc)

	var total uint64

	if err := db.View(context.Background(), func(tx kv.Tx) error {
		var skipNums uint64 = 0
		return tx.ForEach(PlainStateBucket, nil, func(k, v []byte) error {
			if skipNums > 0 {
				skipNums--
				return nil
			}
			total++

			// Process accounts (keys with length 20)
			if len(k) == 20 {
				addr := common.BytesToAddress(k)

				// Decode account data
				genesisAccount, err := decodeAccountData(v)
				if err != nil {
					log.Warn("LoadDB: failed to decode account", "address", addr.Hex(), "error", err)
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

				// Write all accounts to dbAlloc (regardless of ignore status)
				dbAlloc[addr] = *genesisAccount
			}

			// Process storage (keys with length > 28)
			if len(k) > 28 {

				addr := common.BytesToAddress(k[:20])

				if account, exists := dbAlloc[addr]; exists {
					if account.Storage == nil {
						account.Storage = make(map[common.Hash]common.Hash)
					}

					storageKey := common.BytesToHash(k[28:])
					storageValue := common.BytesToHash(v)

					if addr == ERIGON_SCALABLE_ADDRESS {
						logger.Info("start load scalable acct", "address", addr, "incarnation", k[20:28])
						storage, scalableStorageCount, err := processScalableAddressStorageConcurrently(db, k[:28])
						if err != nil {
							logger.Error("processing scalable address storage", "error", err)
						}
						if scalableStorageCount < 1 {
							logger.Warn("scalable acct storage is zero")
						} else {
							logger.Info("scalable storage", "count", scalableStorageCount)
							account.Storage = storage
							dbAlloc[addr] = account
							skipNums = scalableStorageCount - 1
						}

					} else {
						account.Storage[storageKey] = storageValue
						dbAlloc[addr] = account

					}

				} else {
					logger.Error("account not exist for storage", "addr", addr)
				}

			}
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("failed to scan migration database: %w", err)
	}

	log.Info("scalabel storage", "count", len(dbAlloc[SCALABEL_ADDR].Storage))

	tr := btree.New(2) // 2 is the B-tree degree

	for key, val := range dbAlloc[SCALABEL_ADDR].Storage {
		tr.ReplaceOrInsert(Item{key.Hex(), val.Hex()})
	}

	count := 0
	tr.Ascend(func(item btree.Item) bool {
		fmt.Println("top:", count, item)
		count++
		return count < 5
	})

	count = 0
	tr.Descend(func(item btree.Item) bool {
		fmt.Println("bot:", count, item)
		count++
		return count < 5
	})

	log.Info("LoadDB: database scan completed", "accounts", len(dbAlloc), "elapsed", time.Since(start))

	return dbAlloc, nil
}

var SCALABEL_ADDR = common.HexToAddress("0x000000000000000000000000000000005ca1ab1e")

func (a Item) Less(b btree.Item) bool {
	return a.Key < b.(Item).Key
}

type Item struct {
	Key   string
	Value string
}

func getSmtBatchRootHashOrigin(chainDataPath, smtDataPath string) (*big.Int, error) {
	// Choose database path: use smtDataPath if not empty, otherwise use chainDataPath
	dbPath := smtDataPath
	if dbPath == "" {
		if chainDataPath == "" {
			return nil, fmt.Errorf("both chaindata and smt-db-path are empty")
		}
		dbPath = chainDataPath
	}

	ctx := context.Background()

	// Open database
	opts := mdbx.NewMDBX(erigonlog.New()).Path(dbPath)
	db, err := opts.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}
	defer db.Close()

	// Begin read transaction
	tx, err := db.BeginRo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get lastRoot from SMT stats table
	lastRootData, err := tx.GetOne("HermezSmtStats", []byte("lastRoot"))
	if err != nil {
		return big.NewInt(0), nil // Return zero if table doesn't exist
	}

	if lastRootData == nil {
		return big.NewInt(0), nil // Return zero if no data found
	}

	// Convert hex string to big.Int using hexutil
	lastRootStr := string(lastRootData)
	lastRoot, err := hexutil.DecodeBig(lastRootStr)
	if err != nil {
		return big.NewInt(0), nil // Return zero if conversion fails
	}

	return lastRoot, nil
}

func TinyScalarToArrayUint64(scalar uint64) [8]uint64 {
	var result [8]uint64

	result[0] = scalar & 0xFFFFFFFF
	result[1] = (scalar >> 32) & 0xFFFFFFFF

	return result
}

type NodeKV struct {
	Key       NodeKey
	Value     [8]uint64
	leafHash  [4]uint64
	level     int // [0, 255]
	path      []int
	shortPath [4]uint64 // Store 256 bits using 4 uint64s
}

func (nv *NodeKV) ToHexString() string {
	// Convert [8]uint64 to big.Int
	valueBig := new(big.Int)
	for i := 0; i < 8; i++ {
		valueBig.Lsh(valueBig, 64)
		valueBig.Add(valueBig, new(big.Int).SetUint64(nv.Value[i]))
	}
	return fmt.Sprintf("Key: %x, Value: %x", nv.Key.ToBigInt().Text(16), valueBig.Text(16))
}

func calculateLevels(nodeKvs []*NodeKV, startLevel int) {
	if len(nodeKvs) <= 1 || startLevel > 255 {
		return
	}

	// Find the position that split the tree into two subtrees.
	splitIndex := sort.Search(len(nodeKvs), func(i int) bool {
		return nodeKvs[i].Key.GetPath()[startLevel] == 1
	})

	if splitIndex == 0 {
		// All nodes belong to the right subtree
		calculateLevels(nodeKvs, startLevel+1)
	} else if splitIndex == len(nodeKvs) {
		// All nodes belong to the left subtree
		calculateLevels(nodeKvs, startLevel+1)
	} else {
		// Both left subtree and right subtree have node(s)
		for i := range nodeKvs {
			nodeKvs[i].level = startLevel + 1
		}

		// Enable concurrent processing only when data size is large enough
		if len(nodeKvs) > 10000 {
			var wg sync.WaitGroup
			wg.Add(2)

			// Process left subtree concurrently
			go func() {
				defer wg.Done()
				calculateLevels(nodeKvs[:splitIndex], startLevel+1)
			}()

			// Process right subtree concurrently
			go func() {
				defer wg.Done()
				calculateLevels(nodeKvs[splitIndex:], startLevel+1)
			}()

			wg.Wait()
		} else {
			// Process sequentially when data size is small
			calculateLevels(nodeKvs[:splitIndex], startLevel+1)
			calculateLevels(nodeKvs[splitIndex:], startLevel+1)
		}
	}
}

func KeyContractStorageHack(ethaddr [8]uint64, storageKey string) NodeKey {
	storageKeyBig := ConvertHexToBigInt(storageKey)
	storageKeyArr := ScalarToArrayUint64(storageKeyBig)
	hk0 := HashByPointers(&storageKeyArr, &BranchCapacity)
	var key1 = [8]uint64{ethaddr[0], ethaddr[1], ethaddr[2], ethaddr[3], ethaddr[4], ethaddr[5], uint64(SC_STORAGE), uint64(0)}
	return *HashByPointers(&key1, hk0)
}

func calculateRoot(nodeKVs []*NodeKV, start, end, level int) [4]uint64 {
	if start >= end {
		return [4]uint64{0, 0, 0, 0} // Return zero hash for empty node
	}

	if start+1 == end {
		return nodeKVs[start].leafHash // Return leaf hash for single node
	}

	// Find the split point
	splitIndex := sort.Search(end-start, func(i int) bool {
		return nodeKVs[start+i].path[level] == 1
	}) + start

	// Calculate hashes for left and right subtrees
	var leftHash, rightHash [4]uint64
	if end-start > 10000 {
		var wg sync.WaitGroup

		if splitIndex > start {
			wg.Add(1)
			go func() {
				defer wg.Done()
				leftHash = calculateRoot(nodeKVs, start, splitIndex, level+1)
			}()
		}

		if splitIndex < end {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rightHash = calculateRoot(nodeKVs, splitIndex, end, level+1)
			}()
		}

		wg.Wait()
	} else {
		if splitIndex > start {
			leftHash = calculateRoot(nodeKVs, start, splitIndex, level+1)
		}
		if splitIndex < end {
			rightHash = calculateRoot(nodeKVs, splitIndex, end, level+1)
		}
	}

	// Calculate hash for current node
	return *HashByPointers(
		&[8]uint64{
			leftHash[0], leftHash[1], leftHash[2], leftHash[3],
			rightHash[0], rightHash[1], rightHash[2], rightHash[3],
		},
		&BranchCapacity,
	)
}

func calcSmtRoot(alloc types.GenesisAlloc) (*big.Int, error) {
	nodeKvs := make([]*NodeKV, 0)
	start1 := time.Now()

	// Get all addresses and process them in chunks
	addrs := make([]common.Address, 0, len(alloc))
	for addr := range alloc {
		addrs = append(addrs, addr)
	}

	numWorkers := runtime.NumCPU()
	log.Info("verifySMT: starting calculation", "addresses", len(addrs), "workers", numWorkers)
	chunkSize := (len(addrs) + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < len(addrs); i += chunkSize {
		end := i + chunkSize
		if end > len(addrs) {
			end = len(addrs)
		}

		wg.Add(1)
		go func(addrSlice []common.Address) {
			defer wg.Done()
			localKvs := make([]*NodeKV, 0, len(addrSlice)*2+2)

			for _, addr := range addrSlice {
				acc := alloc[addr]
				addrStr := addr.Hex()

				// Process balance
				if acc.Balance != nil && acc.Balance.Sign() > 0 {
					balanceKey := KeyEthAddrBalance(addrStr)
					balanceValue := ScalarToArrayUint64(acc.Balance)
					localKvs = append(localKvs, &NodeKV{
						Key:   balanceKey,
						Value: balanceValue,
					})
				}

				// Process nonce
				if acc.Nonce > 0 {
					nonceKey := KeyEthAddrNonce(addrStr)
					nonceBig := new(big.Int).SetUint64(acc.Nonce)
					nonceValue := ScalarToArrayUint64(nonceBig)
					localKvs = append(localKvs, &NodeKV{
						Key:   nonceKey,
						Value: nonceValue,
					})
				}

				// Process code
				if len(acc.Code) > 0 {
					keyContractCode := KeyContractCode(addrStr)
					keyContractLength := KeyContractLength(addrStr)
					bi := HashContractBytecodeBigInt(common.Bytes2Hex(acc.Code))
					localKvs = append(localKvs, &NodeKV{
						Key:   keyContractCode,
						Value: ScalarToArrayUint64(bi),
					})
					localKvs = append(localKvs, &NodeKV{
						Key:   keyContractLength,
						Value: TinyScalarToArrayUint64(uint64(len(acc.Code))),
					})
				}

				// Process storage
				if len(acc.Storage) > 50000 {
					// Convert storage to slice for parallel processing
					storageKeys := make([]common.Hash, 0, len(acc.Storage))
					for k := range acc.Storage {
						storageKeys = append(storageKeys, k)
					}

					// Calculate chunk size for each worker
					numStorageWorkers := runtime.NumCPU()
					storageChunkSize := (len(storageKeys) + numStorageWorkers - 1) / numStorageWorkers

					var storageWg sync.WaitGroup
					var storageMu sync.Mutex
					addrBig := addr.Big()
					addrArr := ScalarToArrayUint64(addrBig)

					// Process storage concurrently
					for i := 0; i < len(storageKeys); i += storageChunkSize {
						end := i + storageChunkSize
						if end > len(storageKeys) {
							end = len(storageKeys)
						}

						storageWg.Add(1)
						go func(keys []common.Hash) {
							defer storageWg.Done()
							localStorageKvs := make([]*NodeKV, 0, len(keys))

							for _, k := range keys {
								v := acc.Storage[k]
								storageBig := v.Big()
								storageValue := ScalarToArrayUint64(storageBig)
								storageKey := KeyContractStorageHack(addrArr, k.Hex())
								localStorageKvs = append(localStorageKvs, &NodeKV{
									Key:   storageKey,
									Value: storageValue,
								})
							}

							storageMu.Lock()
							localKvs = append(localKvs, localStorageKvs...)
							storageMu.Unlock()
						}(storageKeys[i:end])
					}

					storageWg.Wait()
				} else {
					// Process storage sequentially when size is small
					addrBig := addr.Big()
					addrArr := ScalarToArrayUint64(addrBig)
					for k, v := range acc.Storage {
						storageBig := v.Big()
						storageValue := ScalarToArrayUint64(storageBig)
						storageKey := KeyContractStorageHack(addrArr, k.Hex())
						localKvs = append(localKvs, &NodeKV{
							Key:   storageKey,
							Value: storageValue,
						})
					}
				}
			}

			// Calculate path and shortPath
			for i := range localKvs {
				localKvs[i].path = localKvs[i].Key.GetPath()
				// Convert path to shortPath
				for j := 0; j < 256; j++ {
					if localKvs[i].path[j] == 1 {
						// Set corresponding bit to 1
						// j=0 should map to highest bit, so use 63-(j%64)
						blockIdx := j / 64      // Determine which uint64
						bitPos := 63 - (j % 64) // Position in this uint64, starting from highest bit
						localKvs[i].shortPath[blockIdx] |= uint64(1) << uint64(bitPos)
					}
				}
			}

			// Merge results
			mu.Lock()
			nodeKvs = append(nodeKvs, localKvs...)
			mu.Unlock()
		}(addrs[i:end])
	}

	wg.Wait()
	log.Info("verifySMT: data preparation completed", "nodes", len(nodeKvs), "elapsed", time.Since(start1))

	start11 := time.Now()
	slices.SortFunc(nodeKvs, func(a, b *NodeKV) int {
		// Directly compare uint64 arrays
		for i := 0; i < 4; i++ {
			if a.shortPath[i] < b.shortPath[i] {
				return -1
			}
			if a.shortPath[i] > b.shortPath[i] {
				return 1
			}
		}
		return 0
	})
	log.Info("verifySMT: node sorting completed", "elapsed", time.Since(start11))

	start2 := time.Now()
	calculateLevels(nodeKvs, 0)
	log.Info("verifySMT: level calculation completed", "elapsed", time.Since(start2))

	start3 := time.Now()
	// 2. Calculate leaf node hashes concurrently
	chunkSize = (len(nodeKvs) + numWorkers - 1) / numWorkers

	for i := 0; i < len(nodeKvs); i += chunkSize {
		end := i + chunkSize
		if end > len(nodeKvs) {
			end = len(nodeKvs)
		}

		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				// value hash
				valueHash := HashByPointers(
					&nodeKvs[i].Value,
					&BranchCapacity,
				)

				// leaf hash
				remainingKey := RemoveKeyBits(nodeKvs[i].Key, nodeKvs[i].level)
				nodeKvs[i].leafHash = *HashByPointers(
					&[8]uint64{
						remainingKey[0], remainingKey[1], remainingKey[2], remainingKey[3],
						valueHash[0], valueHash[1], valueHash[2], valueHash[3],
					},
					&LeafCapacity,
				)
			}
		}(i, end)
	}
	wg.Wait()
	log.Info("verifySMT: leaf hash calculation completed", "elapsed", time.Since(start3))

	start4 := time.Now()
	root := NodeKey(calculateRoot(nodeKvs, 0, len(nodeKvs), 0))
	log.Info("verifySMT: root hash calculation completed", "elapsed", time.Since(start4))

	return root.ToBigInt(), nil
}

// verifySMT verifies the SMT (Sparse Merkle Tree) data
func verifySMT(chainDataPath string, smtDataPath string, dbAlloc *types.GenesisAlloc) error {
	start := time.Now()
	log.Info("verifySMT: starting verification", "chainDataPath", chainDataPath, "smtDataPath", smtDataPath, "accounts", len(*dbAlloc))
	smtBatchRootHashOrigin, err := getSmtBatchRootHashOrigin(chainDataPath, smtDataPath)
	if err != nil {
		log.Error("verifySMT: failed to get origin hash", "error", err)
		return err
	}
	log.Info("verifySMT: origin hash retrieved", "hash", fmt.Sprintf("0x%s", smtBatchRootHashOrigin.Text(16)), "elapsed", time.Since(start))

	// Use dbAlloc directly for SMT verification
	smtBatchRootHashRebuild, err := calcSmtRoot(*dbAlloc)
	if err != nil {
		log.Error("verifySMT: failed to calculate rebuild hash", "error", err)
		return err
	}
	log.Info("verifySMT: rebuild hash calculated", "origin", fmt.Sprintf("0x%s", smtBatchRootHashOrigin.Text(16)), "rebuild", fmt.Sprintf("0x%s", smtBatchRootHashRebuild.Text(16)))

	if smtBatchRootHashOrigin != nil {
		if smtBatchRootHashOrigin.Text(16) == smtBatchRootHashRebuild.Text(16) {
			log.Info("verifySMT: verification passed", "status", "✅", "elapsed", time.Since(start))
		} else {
			log.Error("verifySMT: verification failed", "status", "❌", "elapsed", time.Since(start))
		}
	}
	return nil
}

func dumpGenesis(genesis *Genesis, outputPath string) {
	start := time.Now()
	log.Info("dumpGenesis: starting dump")
	buf, err := sonic.MarshalIndent(genesis, "", "  ")
	if err != nil {
		log.Warn("dumpGenesis: failed to marshal genesis", "error", err)
	} else if outputPath != "" {
		if err := os.WriteFile(outputPath, buf, 0666); err != nil {
			log.Warn("dumpGenesis: failed to write file", "path", outputPath, "error", err)
		} else {
			log.Info("dumpGenesis: file written successfully", "path", outputPath)
		}
	}
	log.Info("dumpGenesis: completed", "elapsed", time.Since(start))
}

// SetupGenesisBlockWithMigrationData sets up the genesis block with migration data
func SetupGenesisBlockWithMigrationData(chaindb ethdb.Database, triedb *triedb.Database, genesis *Genesis, overrides *ChainOverrides, ctx *cli.Context) (*params.ChainConfig, common.Hash, *params.ConfigCompatError, error) {
	// Get migration path from CLI context
	chainDataPath := ctx.String("chaindata")
	if chainDataPath == "" {
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
		ChainDataPath:   chainDataPath,
		IgnoreAddresses: ignoreAddresses,
		SMTDataPath:     ctx.String("smt-db-path"),
		IgnoreSMTVerify: ctx.Bool("ignore-smt-verify"),
	}

	// isStandaloneDb
	isStandaloneDb := migrationConfig.SMTDataPath != ""
	kv.InitStandaloneSMT(isStandaloneDb)
	dbAlloc, err := LoadErigonGenesisData(migrationConfig.ChainDataPath)
	if err != nil {
		log.Error("SetupGenesis: failed to load erigon genesis data", "error", err)
		return nil, common.Hash{}, nil, err
	}

	// Start parallel processes
	var wg sync.WaitGroup
	var smtErr error

	// 1. Start verifySMT in parallel
	if !migrationConfig.IgnoreSMTVerify {
		wg.Add(1)
		go func() {
			defer wg.Done()
			smtErr = verifySMT(migrationConfig.ChainDataPath, migrationConfig.SMTDataPath, &dbAlloc)
		}()
	}

	// 2. Generate migrateAlloc by filtering out ignored addresses and merging with genesis.Alloc
	migrateAlloc := generateMigrateAlloc(dbAlloc, ignoreAddresses, &genesis.Alloc)
	// Update genesis.Alloc with the merged result
	genesis.Alloc = migrateAlloc

	// 3. Dump genesis to file in parallel if needed
	if ctx.String("output") != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dumpGenesis(genesis, ctx.String("output"))
		}()
	}

	// 4. Start SetupGenesisBlockWithOverride in parallel
	var setupErr error
	var hash common.Hash
	var compatErr *params.ConfigCompatError
	var cfg *params.ChainConfig
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		cfg, hash, compatErr, setupErr = SetupGenesisBlockWithOverride(chaindb, triedb, genesis, overrides)
		if smtErr == nil && setupErr == nil {
			log.Info("SetupGenesis: migration completed successfully", "status", "✅", "elapsed", time.Since(start))
		}
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

	log.Info("Updated genesis alloc with migration data", "total_accounts", len(dbAlloc), "migrate_accounts", len(migrateAlloc), "updated_accounts", len(genesis.Alloc))

	return cfg, hash, compatErr, setupErr
}
