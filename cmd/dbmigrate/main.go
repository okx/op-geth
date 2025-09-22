package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb/dbmigration"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/urfave/cli/v2"
)

const (
	// Migration batch sizes optimized for blockchain data
	defaultBatchSize  = 10 * 1024 * 1024 // 10MB batches for optimal performance
	defaultBatchCount = 1000             // Number of key-value pairs per batch
)

func dbMigrate(ctx *cli.Context) error {
	// Initialize logging
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true)))

	// Parse command line arguments
	dataDir := ctx.String("datadir")
	if dataDir == "" {
		return errors.New("data directory must be specified")
	}

	sourceType := strings.ToLower(ctx.String("from"))
	targetType := strings.ToLower(ctx.String("to"))
	targetDir := ctx.String("target-dir")
	force := ctx.Bool("force")
	verify := ctx.Bool("verify")
	batchSize := ctx.Int("batch-size")
	batchCount := ctx.Int("batch-count")

	// Validate parameters
	if sourceType == targetType {
		return errors.New("source and target database types must be different")
	}

	// Set default target directory
	if targetDir == "" {
		targetDir = filepath.Join(dataDir, "chaindata_migrated")
	}

	// Check if target directory exists
	if _, err := os.Stat(targetDir); err == nil && !force {
		return fmt.Errorf("target directory %s already exists, use --force to overwrite", targetDir)
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Determine source database path
	sourceDir := filepath.Join(dataDir, "geth", "chaindata")

	log.Info("Starting database migration",
		"sourceType", sourceType,
		"targetType", targetType,
		"sourceDir", sourceDir,
		"targetDir", targetDir,
		"batchSize", common.StorageSize(batchSize),
		"batchCount", batchCount,
		"verify", verify)

	// Open source database (read-only)
	sourceDB, err := dbmigration.OpenDatabase(sourceType, sourceDir, true)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceDB.Close()

	// Open target database (read-write)
	targetDB, err := dbmigration.OpenDatabase(targetType, targetDir, false)
	if err != nil {
		return fmt.Errorf("failed to open target database: %w", err)
	}
	defer targetDB.Close()

	// Create migrator and run migration
	migrator := dbmigration.NewDatabaseMigrator(sourceDB, targetDB, batchSize, batchCount, verify)

	// Setup metrics reporting
	if metrics.Enabled() {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					log.Info("Migration metrics", "stats", migrator.Stats.String())
				case <-migrator.Ctx.Done():
					return
				}
			}
		}()
	}

	// Execute migration
	if err := migrator.Run(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Final statistics
	stats := migrator.Stats
	log.Info("Database migration completed successfully",
		"totalKeys", stats.TotalKeys,
		"totalBytes", common.StorageSize(stats.TotalBytes),
		"duration", stats.Duration(),
		"avgSpeed", fmt.Sprintf("%.0f keys/s", float64(stats.ProcessedKeys)/stats.Duration().Seconds()),
		"avgThroughput", fmt.Sprintf("%s/s", common.StorageSize(float64(stats.ProcessedBytes)/stats.Duration().Seconds())),
		"batchesWritten", stats.BatchesWritten,
		"errorCount", stats.ErrorCount)

	return nil
}

func main() {
	app := &cli.App{
		Name:      "dbmigrate",
		Usage:     "Migrate database from one backend to another",
		ArgsUsage: "",
		Flags: []cli.Flag{
			utils.DataDirFlag,
			&cli.StringFlag{
				Name:  "from",
				Usage: "Source database type (pebble, rocksdb, leveldb)",
				Value: "pebble",
			},
			&cli.StringFlag{
				Name:  "to",
				Usage: "Target database type (pebble, rocksdb, leveldb)",
				Value: "rocksdb",
			},
			&cli.StringFlag{
				Name:  "target-dir",
				Usage: "Target directory for migrated database (defaults to datadir/chaindata_migrated)",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Force migration even if target directory exists",
			},
			&cli.BoolFlag{
				Name:  "verify",
				Usage: "Verify migration by comparing source and target databases",
			},
			&cli.IntFlag{
				Name:  "batch-size",
				Usage: "Migration batch size in bytes",
				Value: defaultBatchSize,
			},
			&cli.IntFlag{
				Name:  "batch-count",
				Usage: "Number of entries per batch",
				Value: defaultBatchCount,
			},
			&cli.BoolFlag{
				Name:  "include-ancient",
				Usage: "Include ancient/freezer data in migration",
			},
		},
		Action: dbMigrate,
		Description: `
The db-migrate command migrates an op-geth database from one backend to another.

This tool is specifically designed for migrating between Pebble, RocksDB, and LevelDB
backends while preserving all blockchain state data, including trie nodes, block data,
receipts, and other critical information.

Examples:
  # Migrate from Pebble to RocksDB (most common use case)
  geth db-migrate --from pebble --to rocksdb

  # Migrate with custom target directory
  geth db-migrate --from pebble --to rocksdb --target-dir /custom/path

  # Migrate with verification
  geth db-migrate --from pebble --to rocksdb --verify

  # Force migration overwriting existing target
  geth db-migrate --from pebble --to rocksdb --force

The migration process:
1. Opens source database in read-only mode
2. Creates target database with optimized settings
3. Migrates data in batches for memory efficiency
4. Optionally verifies the migration
5. Reports detailed progress and statistics
`,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
