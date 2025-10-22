// Copyright 2015 The go-ethereum Authors
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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/holiman/uint256"
	"github.com/urfave/cli/v2"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/history"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/internal/era"
	"github.com/ethereum/go-ethereum/internal/era/eradl"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
)

// AccountVerificationResult holds the result of verifying a single account
type AccountVerificationResult struct {
	Address  common.Address
	Errors   []string
	Verified bool
}

// verifyAccount verifies a single account against the expected state
func verifyAccount(addr common.Address, expectedAccount types.Account, stateDB state.Database, genesisRoot common.Hash, resultChan chan<- AccountVerificationResult) {
	result := AccountVerificationResult{
		Address:  addr,
		Errors:   make([]string, 0),
		Verified: true,
	}

	statedb, err := state.New(genesisRoot, stateDB)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create state database: %v", err))
		result.Verified = false
		resultChan <- result
		return
	}

	// Verify balance
	expectedBalance := uint256.MustFromBig(expectedAccount.Balance)
	actualBalance := statedb.GetBalance(addr)
	if actualBalance.Cmp(expectedBalance) != 0 {
		result.Errors = append(result.Errors, fmt.Sprintf("Balance mismatch: expected %v, actual %v", expectedBalance, actualBalance))
		result.Verified = false
	}

	// Verify nonce
	expectedNonce := expectedAccount.Nonce
	actualNonce := statedb.GetNonce(addr)
	if actualNonce != expectedNonce {
		result.Errors = append(result.Errors, fmt.Sprintf("Nonce mismatch: expected %v, actual %v", expectedNonce, actualNonce))
		result.Verified = false
	}

	// Verify code
	expectedCode := expectedAccount.Code
	actualCode := statedb.GetCode(addr)
	if !bytes.Equal(actualCode, expectedCode) {
		result.Errors = append(result.Errors, fmt.Sprintf("Code mismatch: expected %v, actual %v", hexutil.Encode(expectedCode), hexutil.Encode(actualCode)))
		result.Verified = false
	}

	// Verify storage
	for key, expectedValue := range expectedAccount.Storage {
		actualValue := statedb.GetState(addr, key)
		if actualValue != expectedValue {
			result.Errors = append(result.Errors, fmt.Sprintf("Storage mismatch at key %v: expected %v, actual %v", key.Hex(), expectedValue.Hex(), actualValue.Hex()))
			result.Verified = false
		}
	}

	resultChan <- result
}

var (
	initCommand = &cli.Command{
		Action:    initGenesis,
		Name:      "init",
		Usage:     "Bootstrap and initialize a new genesis block",
		ArgsUsage: "<genesisPath>",
		Flags: slices.Concat([]cli.Flag{
			utils.CachePreimagesFlag,
			utils.OverrideOsaka,
			utils.OverrideVerkle,
		}, utils.DatabaseFlags),
		Description: `
The init command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.

If --no-verify is provided, the command will skip verify
the genesis state after initialization. Use --ignore-addresses to specify
addresses to skip during verification.`,
	}
	dumpGenesisCommand = &cli.Command{
		Action:    dumpGenesis,
		Name:      "dumpgenesis",
		Usage:     "Dumps genesis block JSON configuration to stdout",
		ArgsUsage: "",
		Flags:     slices.Concat([]cli.Flag{utils.DataDirFlag}, utils.NetworkFlags),
		Description: `
The dumpgenesis command prints the genesis configuration of the network preset
if one is set.  Otherwise it prints the genesis from the datadir.`,
	}
	importCommand = &cli.Command{
		Action:    importChain,
		Name:      "import",
		Usage:     "Import a blockchain file",
		ArgsUsage: "<filename> (<filename 2> ... <filename N>) ",
		Flags: slices.Concat([]cli.Flag{
			utils.GCModeFlag,
			utils.SnapshotFlag,
			utils.CacheFlag,
			utils.CacheDatabaseFlag,
			utils.CacheTrieFlag,
			utils.CacheGCFlag,
			utils.CacheSnapshotFlag,
			utils.CacheNoPrefetchFlag,
			utils.CachePreimagesFlag,
			utils.NoCompactionFlag,
			utils.MetricsEnabledFlag,
			utils.MetricsEnabledExpensiveFlag,
			utils.MetricsHTTPFlag,
			utils.MetricsPortFlag,
			utils.MetricsEnableInfluxDBFlag,
			utils.MetricsEnableInfluxDBV2Flag,
			utils.MetricsInfluxDBEndpointFlag,
			utils.MetricsInfluxDBDatabaseFlag,
			utils.MetricsInfluxDBUsernameFlag,
			utils.MetricsInfluxDBPasswordFlag,
			utils.MetricsInfluxDBTagsFlag,
			utils.MetricsInfluxDBTokenFlag,
			utils.MetricsInfluxDBBucketFlag,
			utils.MetricsInfluxDBOrganizationFlag,
			utils.TxLookupLimitFlag,
			utils.VMTraceFlag,
			utils.VMTraceJsonConfigFlag,
			utils.TransactionHistoryFlag,
			utils.LogHistoryFlag,
			utils.LogNoHistoryFlag,
			utils.LogExportCheckpointsFlag,
			utils.StateHistoryFlag,
		}, utils.DatabaseFlags, debug.Flags),
		Before: func(ctx *cli.Context) error {
			flags.MigrateGlobalFlags(ctx)
			return debug.Setup(ctx)
		},
		Description: `
The import command allows the import of blocks from an RLP-encoded format. This format can be a single file
containing multiple RLP-encoded blocks, or multiple files can be given.

If only one file is used, an import error will result in the entire import process failing. If
multiple files are processed, the import process will continue even if an individual RLP file fails
to import successfully.`,
	}
	exportCommand = &cli.Command{
		Action:    exportChain,
		Name:      "export",
		Usage:     "Export blockchain into file",
		ArgsUsage: "<filename> [<blockNumFirst> <blockNumLast>]",
		Flags:     slices.Concat([]cli.Flag{utils.CacheFlag}, utils.DatabaseFlags),
		Description: `
Requires a first argument of the file to write to.
Optional second and third arguments control the first and
last block to write. In this mode, the file will be appended
if already existing. If the file ends with .gz, the output will
be gzipped.`,
	}
	importHistoryCommand = &cli.Command{
		Action:    importHistory,
		Name:      "import-history",
		Usage:     "Import an Era archive",
		ArgsUsage: "<dir>",
		Flags:     slices.Concat([]cli.Flag{utils.TxLookupLimitFlag, utils.TransactionHistoryFlag}, utils.DatabaseFlags, utils.NetworkFlags),
		Description: `
The import-history command will import blocks and their corresponding receipts
from Era archives.
`,
	}
	exportHistoryCommand = &cli.Command{
		Action:    exportHistory,
		Name:      "export-history",
		Usage:     "Export blockchain history to Era archives",
		ArgsUsage: "<dir> <first> <last>",
		Flags:     utils.DatabaseFlags,
		Description: `
The export-history command will export blocks and their corresponding receipts
into Era archives. Eras are typically packaged in steps of 8192 blocks.
`,
	}
	importPreimagesCommand = &cli.Command{
		Action:    importPreimages,
		Name:      "import-preimages",
		Usage:     "Import the preimage database from an RLP stream",
		ArgsUsage: "<datafile>",
		Flags:     slices.Concat([]cli.Flag{utils.CacheFlag}, utils.DatabaseFlags),
		Description: `
The import-preimages command imports hash preimages from an RLP encoded stream.
It's deprecated, please use "geth db import" instead.
`,
	}

	dumpCommand = &cli.Command{
		Action:    dump,
		Name:      "dump",
		Usage:     "Dump a specific block from storage",
		ArgsUsage: "[? <blockHash> | <blockNum>]",
		Flags: slices.Concat([]cli.Flag{
			utils.CacheFlag,
			utils.IterativeOutputFlag,
			utils.ExcludeCodeFlag,
			utils.ExcludeStorageFlag,
			utils.IncludeIncompletesFlag,
			utils.StartKeyFlag,
			utils.DumpLimitFlag,
		}, utils.DatabaseFlags),
		Description: `
This command dumps out the state for a given block (or latest, if none provided).
`,
	}

	pruneHistoryCommand = &cli.Command{
		Action:    pruneHistory,
		Name:      "prune-history",
		Usage:     "Prune blockchain history (block bodies and receipts) up to the merge block",
		ArgsUsage: "",
		Flags:     utils.DatabaseFlags,
		Description: `
The prune-history command removes historical block bodies and receipts from the
blockchain database up to the merge block, while preserving block headers. This
helps reduce storage requirements for nodes that don't need full historical data.`,
	}

	downloadEraCommand = &cli.Command{
		Action:    downloadEra,
		Name:      "download-era",
		Usage:     "Fetches era1 files (pre-merge history) from an HTTP endpoint",
		ArgsUsage: "",
		Flags: slices.Concat(
			utils.DatabaseFlags,
			utils.NetworkFlags,
			[]cli.Flag{
				eraBlockFlag,
				eraEpochFlag,
				eraAllFlag,
				eraServerFlag,
			},
		),
	}
)

var (
	eraBlockFlag = &cli.StringFlag{
		Name:  "block",
		Usage: "Block number to fetch. (can also be a range <start>-<end>)",
	}
	eraEpochFlag = &cli.StringFlag{
		Name:  "epoch",
		Usage: "Epoch number to fetch (can also be a range <start>-<end>)",
	}
	eraAllFlag = &cli.BoolFlag{
		Name:  "all",
		Usage: "Download all available era1 files",
	}
	eraServerFlag = &cli.StringFlag{
		Name:  "server",
		Usage: "era1 server URL",
	}
)

// initGenesis will initialise the given JSON format genesis file and writes it as
// the zero'd block (i.e. genesis) or will fail hard if it can't succeed.
func initGenesis(ctx *cli.Context) error {
	initStart := time.Now()
	if ctx.Args().Len() != 1 {
		utils.Fatalf("need genesis.json file as the only argument")
	}
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("invalid path to genesis file")
	}

	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	start := time.Now()
	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}
	log.Info("read file and decode json", "elapsed", time.Since(start))
	// Open and initialise both full and light databases
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	var overrides core.ChainOverrides
	if ctx.IsSet(utils.OverrideOsaka.Name) {
		v := ctx.Uint64(utils.OverrideOsaka.Name)
		overrides.OverrideOsaka = &v
	}
	if ctx.IsSet(utils.OverrideVerkle.Name) {
		v := ctx.Uint64(utils.OverrideVerkle.Name)
		overrides.OverrideVerkle = &v
	}

	chaindb := utils.MakeChainDatabase(ctx, stack, false)
	defer chaindb.Close()

	triedb := utils.MakeTrieDatabase(ctx, chaindb, ctx.Bool(utils.CachePreimagesFlag.Name), false, genesis.IsVerkle())
	defer triedb.Close()

	_, hash, compatErr, err := core.SetupGenesisBlockWithOverride(chaindb, triedb, genesis, &overrides)
	if err != nil {
		utils.Fatalf("Failed to write genesis block: %v", err)
	}
	if compatErr != nil {
		utils.Fatalf("Failed to write chain config: %v", compatErr)
	}
	log.Info("Successfully wrote genesis state", "database", "chaindata", "hash", hash, "elapsed", time.Since(initStart))
	return nil
}

func dumpGenesis(ctx *cli.Context) error {
	// check if there is a testnet preset enabled
	var genesis *core.Genesis
	if utils.IsNetworkPreset(ctx) {
		genesis = utils.MakeGenesis(ctx)
	} else if ctx.IsSet(utils.DeveloperFlag.Name) && !ctx.IsSet(utils.DataDirFlag.Name) {
		genesis = core.DeveloperGenesisBlock(11_500_000, nil)
	}

	if genesis != nil {
		if err := json.NewEncoder(os.Stdout).Encode(genesis); err != nil {
			utils.Fatalf("could not encode genesis: %s", err)
		}
		return nil
	}

	// dump whatever already exists in the datadir
	stack, _ := makeConfigNode(ctx)

	db, err := stack.OpenDatabaseWithOptions("chaindata", node.DatabaseOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer db.Close()

	genesis, err = core.ReadGenesis(db)
	if err != nil {
		utils.Fatalf("failed to read genesis: %s", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(*genesis); err != nil {
		utils.Fatalf("could not encode stored genesis: %s", err)
	}

	return nil
}

func importChain(ctx *cli.Context) error {
	if ctx.Args().Len() < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	stack, cfg := makeConfigNode(ctx)
	defer stack.Close()

	// Start metrics export if enabled
	utils.SetupMetrics(&cfg.Metrics)

	chain, db := utils.MakeChain(ctx, stack, false)
	defer db.Close()

	// Start periodically gathering memory profiles
	var peakMemAlloc, peakMemSys atomic.Uint64
	go func() {
		stats := new(runtime.MemStats)
		for {
			runtime.ReadMemStats(stats)
			if peakMemAlloc.Load() < stats.Alloc {
				peakMemAlloc.Store(stats.Alloc)
			}
			if peakMemSys.Load() < stats.Sys {
				peakMemSys.Store(stats.Sys)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	// Import the chain
	start := time.Now()

	var importErr error

	if ctx.Args().Len() == 1 {
		if err := utils.ImportChain(chain, ctx.Args().First()); err != nil {
			importErr = err
			log.Error("Import error", "err", err)
		}
	} else {
		for _, arg := range ctx.Args().Slice() {
			if err := utils.ImportChain(chain, arg); err != nil {
				importErr = err
				log.Error("Import error", "file", arg, "err", err)
				if err == utils.ErrImportInterrupted {
					break
				}
			}
		}
	}
	chain.Stop()
	fmt.Printf("Import done in %v.\n\n", time.Since(start))

	// Output pre-compaction stats mostly to see the import trashing
	showDBStats(db)

	// Print the memory statistics used by the importing
	mem := new(runtime.MemStats)
	runtime.ReadMemStats(mem)

	fmt.Printf("Object memory: %.3f MB current, %.3f MB peak\n", float64(mem.Alloc)/1024/1024, float64(peakMemAlloc.Load())/1024/1024)
	fmt.Printf("System memory: %.3f MB current, %.3f MB peak\n", float64(mem.Sys)/1024/1024, float64(peakMemSys.Load())/1024/1024)
	fmt.Printf("Allocations:   %.3f million\n", float64(mem.Mallocs)/1000000)
	fmt.Printf("GC pause:      %v\n\n", time.Duration(mem.PauseTotalNs))

	if ctx.Bool(utils.NoCompactionFlag.Name) {
		return nil
	}

	// Compact the entire database to more accurately measure disk io and print the stats
	start = time.Now()
	fmt.Println("Compacting entire database...")
	if err := db.Compact(nil, nil); err != nil {
		utils.Fatalf("Compaction failed: %v", err)
	}
	fmt.Printf("Compaction done in %v.\n\n", time.Since(start))

	showDBStats(db)
	return importErr
}

func exportChain(ctx *cli.Context) error {
	if ctx.Args().Len() < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	chain, db := utils.MakeChain(ctx, stack, true)
	defer db.Close()
	start := time.Now()

	var err error
	fp := ctx.Args().First()
	if ctx.Args().Len() < 3 {
		err = utils.ExportChain(chain, fp)
	} else {
		// This can be improved to allow for numbers larger than 9223372036854775807
		first, ferr := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
		last, lerr := strconv.ParseInt(ctx.Args().Get(2), 10, 64)
		if ferr != nil || lerr != nil {
			utils.Fatalf("Export error in parsing parameters: block number not an integer\n")
		}
		if first < 0 || last < 0 {
			utils.Fatalf("Export error: block number must be greater than 0\n")
		}
		if head := chain.CurrentSnapBlock(); uint64(last) > head.Number.Uint64() {
			utils.Fatalf("Export error: block number %d larger than head block %d\n", uint64(last), head.Number.Uint64())
		}
		err = utils.ExportAppendChain(chain, fp, uint64(first), uint64(last))
	}
	if err != nil {
		utils.Fatalf("Export error: %v\n", err)
	}
	fmt.Printf("Export done in %v\n", time.Since(start))
	return nil
}

func importHistory(ctx *cli.Context) error {
	if ctx.Args().Len() != 1 {
		utils.Fatalf("usage: %s", ctx.Command.ArgsUsage)
	}

	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	chain, db := utils.MakeChain(ctx, stack, false)
	defer db.Close()

	var (
		start   = time.Now()
		dir     = ctx.Args().Get(0)
		network string
	)

	// Determine network.
	if utils.IsNetworkPreset(ctx) {
		switch {
		case ctx.Bool(utils.MainnetFlag.Name):
			network = "mainnet"
		case ctx.Bool(utils.SepoliaFlag.Name):
			network = "sepolia"
		case ctx.Bool(utils.HoleskyFlag.Name):
			network = "holesky"
		case ctx.Bool(utils.HoodiFlag.Name):
			network = "hoodi"
		}
	} else {
		// No network flag set, try to determine network based on files
		// present in directory.
		var networks []string
		for _, n := range params.NetworkNames {
			entries, err := era.ReadDir(dir, n)
			if err != nil {
				return fmt.Errorf("error reading %s: %w", dir, err)
			}
			if len(entries) > 0 {
				networks = append(networks, n)
			}
		}
		if len(networks) == 0 {
			return fmt.Errorf("no era1 files found in %s", dir)
		}
		if len(networks) > 1 {
			return errors.New("multiple networks found, use a network flag to specify desired network")
		}
		network = networks[0]
	}

	if err := utils.ImportHistory(chain, dir, network); err != nil {
		return err
	}
	fmt.Printf("Import done in %v\n", time.Since(start))
	return nil
}

// exportHistory exports chain history in Era archives at a specified
// directory.
func exportHistory(ctx *cli.Context) error {
	if ctx.Args().Len() != 3 {
		utils.Fatalf("usage: %s", ctx.Command.ArgsUsage)
	}

	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	chain, _ := utils.MakeChain(ctx, stack, true)
	start := time.Now()

	var (
		dir         = ctx.Args().Get(0)
		first, ferr = strconv.ParseInt(ctx.Args().Get(1), 10, 64)
		last, lerr  = strconv.ParseInt(ctx.Args().Get(2), 10, 64)
	)
	if ferr != nil || lerr != nil {
		utils.Fatalf("Export error in parsing parameters: block number not an integer\n")
	}
	if first < 0 || last < 0 {
		utils.Fatalf("Export error: block number must be greater than 0\n")
	}
	if head := chain.CurrentSnapBlock(); uint64(last) > head.Number.Uint64() {
		utils.Fatalf("Export error: block number %d larger than head block %d\n", uint64(last), head.Number.Uint64())
	}
	err := utils.ExportHistory(chain, dir, uint64(first), uint64(last), uint64(era.MaxEra1Size))
	if err != nil {
		utils.Fatalf("Export error: %v\n", err)
	}
	fmt.Printf("Export done in %v\n", time.Since(start))
	return nil
}

// importPreimages imports preimage data from the specified file.
// it is deprecated, and the export function has been removed, but
// the import function is kept around for the time being so that
// older file formats can still be imported.
func importPreimages(ctx *cli.Context) error {
	if ctx.Args().Len() < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	db := utils.MakeChainDatabase(ctx, stack, false)
	defer db.Close()
	start := time.Now()

	if err := utils.ImportPreimages(db, ctx.Args().First()); err != nil {
		utils.Fatalf("Import error: %v\n", err)
	}
	fmt.Printf("Import done in %v\n", time.Since(start))
	return nil
}

func parseDumpConfig(ctx *cli.Context, db ethdb.Database) (*state.DumpConfig, common.Hash, error) {
	var header *types.Header
	if ctx.NArg() > 1 {
		return nil, common.Hash{}, fmt.Errorf("expected 1 argument (number or hash), got %d", ctx.NArg())
	}
	if ctx.NArg() == 1 {
		arg := ctx.Args().First()
		if hashish(arg) {
			hash := common.HexToHash(arg)
			if number, ok := rawdb.ReadHeaderNumber(db, hash); ok {
				header = rawdb.ReadHeader(db, hash, number)
			} else {
				return nil, common.Hash{}, fmt.Errorf("block %x not found", hash)
			}
		} else {
			number, err := strconv.ParseUint(arg, 10, 64)
			if err != nil {
				return nil, common.Hash{}, err
			}
			if hash := rawdb.ReadCanonicalHash(db, number); hash != (common.Hash{}) {
				header = rawdb.ReadHeader(db, hash, number)
			} else {
				return nil, common.Hash{}, fmt.Errorf("header for block %d not found", number)
			}
		}
	} else {
		// Use latest
		header = rawdb.ReadHeadHeader(db)
	}
	if header == nil {
		return nil, common.Hash{}, errors.New("no head block found")
	}
	startArg := common.FromHex(ctx.String(utils.StartKeyFlag.Name))
	var start common.Hash
	switch len(startArg) {
	case 0: // common.Hash
	case 32:
		start = common.BytesToHash(startArg)
	case 20:
		start = crypto.Keccak256Hash(startArg)
		log.Info("Converting start-address to hash", "address", common.BytesToAddress(startArg), "hash", start.Hex())
	default:
		return nil, common.Hash{}, fmt.Errorf("invalid start argument: %x. 20 or 32 hex-encoded bytes required", startArg)
	}
	conf := &state.DumpConfig{
		SkipCode:          ctx.Bool(utils.ExcludeCodeFlag.Name),
		SkipStorage:       ctx.Bool(utils.ExcludeStorageFlag.Name),
		OnlyWithAddresses: !ctx.Bool(utils.IncludeIncompletesFlag.Name),
		Start:             start.Bytes(),
		Max:               ctx.Uint64(utils.DumpLimitFlag.Name),
	}
	log.Info("State dump configured", "block", header.Number, "hash", header.Hash().Hex(),
		"skipcode", conf.SkipCode, "skipstorage", conf.SkipStorage,
		"start", hexutil.Encode(conf.Start), "limit", conf.Max)
	return conf, header.Root, nil
}

func dump(ctx *cli.Context) error {
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	db := utils.MakeChainDatabase(ctx, stack, true)
	defer db.Close()

	conf, root, err := parseDumpConfig(ctx, db)
	if err != nil {
		return err
	}
	triedb := utils.MakeTrieDatabase(ctx, db, true, true, false) // always enable preimage lookup
	defer triedb.Close()

	state, err := state.New(root, state.NewDatabase(triedb, nil))
	if err != nil {
		return err
	}
	if ctx.Bool(utils.IterativeOutputFlag.Name) {
		state.IterativeDump(conf, json.NewEncoder(os.Stdout))
	} else {
		fmt.Println(string(state.Dump(conf)))
	}
	return nil
}

// hashish returns true for strings that look like hashes.
func hashish(x string) bool {
	_, err := strconv.Atoi(x)
	return err != nil
}

func pruneHistory(ctx *cli.Context) error {
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	// Open the chain database
	chain, chaindb := utils.MakeChain(ctx, stack, false)
	defer chaindb.Close()
	defer chain.Stop()

	// Determine the prune point. This will be the first PoS block.
	prunePoint, ok := history.PrunePoints[chain.Genesis().Hash()]
	if !ok || prunePoint == nil {
		return errors.New("prune point not found")
	}
	var (
		mergeBlock     = prunePoint.BlockNumber
		mergeBlockHash = prunePoint.BlockHash.Hex()
	)

	// Check we're far enough past merge to ensure all data is in freezer
	currentHeader := chain.CurrentHeader()
	if currentHeader == nil {
		return errors.New("current header not found")
	}
	if currentHeader.Number.Uint64() < mergeBlock+params.FullImmutabilityThreshold {
		return fmt.Errorf("chain not far enough past merge block, need %d more blocks",
			mergeBlock+params.FullImmutabilityThreshold-currentHeader.Number.Uint64())
	}

	// Double-check the prune block in db has the expected hash.
	hash := rawdb.ReadCanonicalHash(chaindb, mergeBlock)
	if hash != common.HexToHash(mergeBlockHash) {
		return fmt.Errorf("merge block hash mismatch: got %s, want %s", hash.Hex(), mergeBlockHash)
	}

	log.Info("Starting history pruning", "head", currentHeader.Number, "tail", mergeBlock, "tailHash", mergeBlockHash)
	start := time.Now()
	rawdb.PruneTransactionIndex(chaindb, mergeBlock)
	if _, err := chaindb.TruncateTail(mergeBlock); err != nil {
		return fmt.Errorf("failed to truncate ancient data: %v", err)
	}
	log.Info("History pruning completed", "tail", mergeBlock, "elapsed", common.PrettyDuration(time.Since(start)))

	// TODO(s1na): what if there is a crash between the two prune operations?

	return nil
}

func verifyGenesisInternal(ctx *cli.Context, genesis *core.Genesis, ignoreAddresses map[common.Address]bool) error {
	start := time.Now()

	// Open the database
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	chaindb, err := stack.OpenDatabaseWithFreezer("chaindata", 512, 1024, ctx.String(utils.AncientFlag.Name), "", true)
	if err != nil {
		utils.Fatalf("Failed to open database: %v", err)
	}
	defer chaindb.Close()

	// Get the genesis block from the database
	genesisHash := rawdb.ReadCanonicalHash(chaindb, genesis.Number)
	if genesisHash == (common.Hash{}) {
		utils.Fatalf("No genesis block found in database")
	}

	genesisBlock := rawdb.ReadBlock(chaindb, genesisHash, genesis.Number)
	if genesisBlock == nil {
		utils.Fatalf("Failed to read genesis block from database")
	}

	log.Info("Found genesis block", "hash", genesisHash, "stateRoot", genesisBlock.Root())

	// Create trie database
	triedb := utils.MakeTrieDatabase(ctx, chaindb, ctx.Bool(utils.CachePreimagesFlag.Name), true, genesis.IsVerkle())
	defer triedb.Close()

	// Create state database (shared across workers)
	stateDB := state.NewDatabase(triedb, nil)

	// Prepare accounts for verification
	accountsToVerify := make([]common.Address, len(genesis.Alloc))

	for addr, _ := range genesis.Alloc {
		if ignoreAddresses != nil && ignoreAddresses[addr] {
			log.Info("Skipping ignored address", "address", addr.Hex())
			continue
		}
		accountsToVerify = append(accountsToVerify, addr)
	}

	log.Info("Starting concurrent verification", "total_accounts", len(accountsToVerify))

	numWorkers := runtime.NumCPU()
	if numWorkers > len(accountsToVerify) {
		numWorkers = len(accountsToVerify)
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	log.Info("Using concurrent workers", "workers", numWorkers)

	resultChan := make(chan AccountVerificationResult, len(accountsToVerify))
	var wg sync.WaitGroup

	accountsPerWorker := len(accountsToVerify) / numWorkers
	if len(accountsToVerify)%numWorkers != 0 {
		accountsPerWorker++
	}

	for i := 0; i < numWorkers; i++ {
		startIdx := i * accountsPerWorker
		endIdx := startIdx + accountsPerWorker
		if endIdx > len(accountsToVerify) {
			endIdx = len(accountsToVerify)
		}

		if startIdx >= len(accountsToVerify) {
			break
		}

		wg.Add(1)
		go func(workerID int, accounts []common.Address) {
			defer wg.Done()

			log.Info("Worker started", "worker", workerID, "accounts", len(accounts))

			for _, addr := range accounts {
				verifyAccount(addr, genesis.Alloc[addr], stateDB, genesisBlock.Root(), resultChan)
			}

			log.Info("Worker completed", "worker", workerID, "accounts_processed", len(accounts))
		}(i, accountsToVerify[startIdx:endIdx])
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var verifiedCount int64
	var errorCount int64
	var ignoredCount int64

	// Process results as they come in
	for result := range resultChan {
		atomic.AddInt64(&verifiedCount, 1)

		if !result.Verified {
			atomic.AddInt64(&errorCount, int64(len(result.Errors)))
			for _, errMsg := range result.Errors {
				log.Error("Account verification failed", "address", result.Address.Hex(), "error", errMsg)
			}
		}

		// Progress reporting
		if verifiedCount%1000 == 0 {
			log.Info("Verification progress", "verified", verifiedCount, "errors", errorCount)
		}
	}

	// Summary
	if errorCount == 0 {
		log.Info("✅ Genesis verification successful",
			"accounts_verified", verifiedCount,
			"accounts_ignored", ignoredCount,
			"elapsed", common.PrettyDuration(time.Since(start)))
	} else {
		log.Error("❌ Genesis verification failed",
			"errors", errorCount,
			"accounts_verified", verifiedCount,
			"accounts_ignored", ignoredCount)
		return fmt.Errorf("verification failed with %d errors", errorCount)
	}

	return nil
}

// verifyGenesis verifies that the saved state in the trie database is consistent
// with the provided genesis.json file.
func verifyGenesis(ctx *cli.Context) error {
	if ctx.Args().Len() < 1 {
		utils.Fatalf("need genesis.json file as the first argument")
	}
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("invalid path to genesis file")
	}

	// Parse ignore addresses
	var ignoreAddresses map[common.Address]bool
	if ctx.IsSet("ignore-addresses") {
		ignoreList := ctx.String("ignore-addresses")
		if ignoreList != "" {
			ignoreAddresses = make(map[common.Address]bool)
			addresses := strings.Split(ignoreList, ",")
			for _, addrStr := range addresses {
				addrStr = strings.TrimSpace(addrStr)
				if addrStr != "" {
					addr := common.HexToAddress(addrStr)
					ignoreAddresses[addr] = true
					log.Info("Ignoring address during verification", "address", addr.Hex())
				}
			}
		}
	} else {
		log.Info("full verification", "path", genesisPath)
	}

	// Read and parse the genesis file
	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	return verifyGenesisInternal(ctx, genesis, ignoreAddresses)
}

// downladEra is the era1 file downloader tool.
func downloadEra(ctx *cli.Context) error {
	flags.CheckExclusive(ctx, eraBlockFlag, eraEpochFlag, eraAllFlag)

	// Resolve the network.
	var network = "mainnet"
	if utils.IsNetworkPreset(ctx) {
		switch {
		case ctx.IsSet(utils.MainnetFlag.Name):
		case ctx.IsSet(utils.SepoliaFlag.Name):
			network = "sepolia"
		default:
			return errors.New("unsupported network, no known era1 checksums")
		}
	}

	// Resolve the destination directory.
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	ancients := stack.ResolveAncient("chaindata", "")
	dir := filepath.Join(ancients, rawdb.ChainFreezerName, "era")
	if ctx.IsSet(utils.EraFlag.Name) {
		dir = filepath.Join(ancients, ctx.String(utils.EraFlag.Name))
	}

	baseURL := ctx.String(eraServerFlag.Name)
	if baseURL == "" {
		return fmt.Errorf("need --%s flag to download", eraServerFlag.Name)
	}

	l, err := eradl.New(baseURL, network)
	if err != nil {
		return err
	}
	switch {
	case ctx.IsSet(eraAllFlag.Name):
		return l.DownloadAll(dir)

	case ctx.IsSet(eraBlockFlag.Name):
		s := ctx.String(eraBlockFlag.Name)
		start, end, ok := parseRange(s)
		if !ok {
			return fmt.Errorf("invalid block range: %q", s)
		}
		return l.DownloadBlockRange(start, end, dir)

	case ctx.IsSet(eraEpochFlag.Name):
		s := ctx.String(eraEpochFlag.Name)
		start, end, ok := parseRange(s)
		if !ok {
			return fmt.Errorf("invalid epoch range: %q", s)
		}
		return l.DownloadEpochRange(start, end, dir)

	default:
		return fmt.Errorf("specify one of --%s, --%s, or --%s to download", eraAllFlag.Name, eraBlockFlag.Name, eraEpochFlag.Name)
	}
}

func parseRange(s string) (start uint64, end uint64, ok bool) {
	log.Info("Parsing block range", "input", s)
	if m, _ := regexp.MatchString("^[0-9]+-[0-9]+$", s); m {
		s1, s2, _ := strings.Cut(s, "-")
		start, err := strconv.ParseUint(s1, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		end, err = strconv.ParseUint(s2, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		if start > end {
			return 0, 0, false
		}
		log.Info("Parsing block range", "start", start, "end", end)
		return start, end, true
	}
	if m, _ := regexp.MatchString("^[0-9]+$", s); m {
		start, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		end = start
		log.Info("Parsing single block range", "block", start)
		return start, end, true
	}
	return 0, 0, false
}
