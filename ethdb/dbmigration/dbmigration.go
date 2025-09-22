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

package dbmigration

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/ethdb/rocksdb"
	"github.com/ethereum/go-ethereum/log"
)

const (
	progressReportEvery = 100000 // Report progress every 100k entries
)

// MigrationStats tracks migration progress and performance metrics
type MigrationStats struct {
	StartTime      time.Time
	EndTime        time.Time
	TotalKeys      int64
	TotalBytes     int64
	ProcessedKeys  int64
	ProcessedBytes int64
	BatchesWritten int64
	ErrorCount     int64
	LastReportTime time.Time
}

// Progress calculates migration progress as percentage
func (s *MigrationStats) Progress() float64 {
	if s.TotalKeys == 0 {
		return 0
	}
	return float64(s.ProcessedKeys) / float64(s.TotalKeys) * 100
}

// BytesProgress calculates bytes migration progress as percentage
func (s *MigrationStats) BytesProgress() float64 {
	if s.TotalBytes == 0 {
		return 0
	}
	return float64(s.ProcessedBytes) / float64(s.TotalBytes) * 100
}

// Duration returns the total migration duration
func (s *MigrationStats) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// String returns a formatted progress string
func (s *MigrationStats) String() string {
	duration := s.Duration()
	keysPerSec := float64(s.ProcessedKeys) / duration.Seconds()
	bytesPerSec := float64(s.ProcessedBytes) / duration.Seconds()

	return fmt.Sprintf(
		"Progress: %.2f%% (%d/%d keys, %s/%s bytes) | "+
			"Speed: %.0f keys/s, %s/s | "+
			"Batches: %d | Errors: %d | Duration: %v",
		s.Progress(),
		s.ProcessedKeys, s.TotalKeys,
		common.StorageSize(s.ProcessedBytes), common.StorageSize(s.TotalBytes),
		keysPerSec, common.StorageSize(bytesPerSec),
		s.BatchesWritten, s.ErrorCount,
		duration.Truncate(time.Second),
	)
}

// DatabaseMigrator handles the migration process between different database backends
type DatabaseMigrator struct {
	SourceDB      ethdb.Database
	TargetDB      ethdb.Database
	Stats         *MigrationStats
	BatchSize     int
	BatchCount    int
	VerifyEnabled bool
	Logger        log.Logger
	Ctx           context.Context
	Cancel        context.CancelFunc
}

// NewDatabaseMigrator creates a new database migrator instance
func NewDatabaseMigrator(sourceDB, targetDB ethdb.Database, batchSize, batchCount int, verifyEnabled bool) *DatabaseMigrator {
	ctx, cancel := context.WithCancel(context.Background())

	return &DatabaseMigrator{
		SourceDB:      sourceDB,
		TargetDB:      targetDB,
		Stats:         &MigrationStats{StartTime: time.Now()},
		BatchSize:     batchSize,
		BatchCount:    batchCount,
		VerifyEnabled: verifyEnabled,
		Logger:        log.New("component", "db-migrator"),
		Ctx:           ctx,
		Cancel:        cancel,
	}
}

// estimateSize estimates the total size of the database for progress tracking
func (m *DatabaseMigrator) estimateSize() error {
	m.Logger.Info("Estimating database size for progress tracking...")

	// Create iterator for the entire database
	iter := m.SourceDB.NewIterator(nil, nil)
	defer iter.Release()

	var totalKeys, totalBytes int64
	sampleCount := 0
	const maxSamples = 10000 // Sample first 10k entries for estimation

	for iter.Next() && sampleCount < maxSamples {
		totalKeys++
		totalBytes += int64(len(iter.Key()) + len(iter.Value()))
		sampleCount++

		// Check for cancellation
		select {
		case <-m.Ctx.Done():
			return m.Ctx.Err()
		default:
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error during size estimation: %w", err)
	}

	// If we sampled less than maxSamples, we have exact numbers
	if sampleCount < maxSamples {
		m.Stats.TotalKeys = totalKeys
		m.Stats.TotalBytes = totalBytes
	} else {
		// Estimate based on sample - this is approximate
		// Continue counting for a more accurate estimate with full scan
		for iter.Next() {
			totalKeys++
			totalBytes += int64(len(iter.Key()) + len(iter.Value()))

			// Check for cancellation periodically
			if totalKeys%progressReportEvery == 0 {
				select {
				case <-m.Ctx.Done():
					return m.Ctx.Err()
				default:
				}
			}
		}

		m.Stats.TotalKeys = totalKeys
		m.Stats.TotalBytes = totalBytes
	}

	m.Logger.Info("Database size estimation complete",
		"totalKeys", m.Stats.TotalKeys,
		"totalBytes", common.StorageSize(m.Stats.TotalBytes),
	)

	return nil
}

// migrate performs the actual database migration
func (m *DatabaseMigrator) migrate() error {
	m.Logger.Info("Starting database migration")

	// Estimate database size for progress tracking
	if err := m.estimateSize(); err != nil {
		return fmt.Errorf("failed to estimate database size: %w", err)
	}

	// Create iterator for the entire database
	iter := m.SourceDB.NewIterator(nil, nil)
	defer iter.Release()

	// Create batch for efficient writing
	batch := m.TargetDB.NewBatchWithSize(m.BatchSize)
	defer func() {
		if batch.ValueSize() > 0 {
			if err := batch.Write(); err != nil {
				m.Logger.Error("Failed to write final batch", "err", err)
			}
		}
	}()

	var (
		batchCount     = 0
		batchBytes     = 0
		lastReportTime = time.Now()
	)

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Copy key and value to ensure they remain valid
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)

		// Add to batch
		if err := batch.Put(keyCopy, valueCopy); err != nil {
			atomic.AddInt64(&m.Stats.ErrorCount, 1)
			m.Logger.Error("Failed to add entry to batch", "key", fmt.Sprintf("%x", keyCopy[:min(len(keyCopy), 32)]), "err", err)
			continue
		}

		// Update statistics
		atomic.AddInt64(&m.Stats.ProcessedKeys, 1)
		atomic.AddInt64(&m.Stats.ProcessedBytes, int64(len(keyCopy)+len(valueCopy)))
		batchCount++
		batchBytes += len(keyCopy) + len(valueCopy)

		// Write batch when it reaches the specified size or count
		if batchBytes >= m.BatchSize || batchCount >= m.BatchCount {
			if err := batch.Write(); err != nil {
				atomic.AddInt64(&m.Stats.ErrorCount, 1)
				m.Logger.Error("Failed to write batch", "err", err)
				return fmt.Errorf("batch write failed: %w", err)
			}

			atomic.AddInt64(&m.Stats.BatchesWritten, 1)
			batch.Reset()
			batchCount = 0
			batchBytes = 0
		}

		// Report progress periodically
		if time.Since(lastReportTime) >= 5*time.Second {
			m.Logger.Info("Migration progress", "stats", m.Stats.String())
			lastReportTime = time.Now()
		}

		// Check for cancellation
		select {
		case <-m.Ctx.Done():
			return m.Ctx.Err()
		default:
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("iterator error during migration: %w", err)
	}

	// Write any remaining data in the batch
	if batch.ValueSize() > 0 {
		if err := batch.Write(); err != nil {
			return fmt.Errorf("failed to write final batch: %w", err)
		}
		atomic.AddInt64(&m.Stats.BatchesWritten, 1)
	}

	m.Stats.EndTime = time.Now()
	m.Logger.Info("Migration completed successfully", "stats", m.Stats.String())

	return nil
}

// verify performs post-migration verification by comparing source and target databases
func (m *DatabaseMigrator) verify() error {
	if !m.VerifyEnabled {
		return nil
	}

	m.Logger.Info("Starting migration verification")
	startTime := time.Now()

	// Create iterator for source database
	sourceIter := m.SourceDB.NewIterator(nil, nil)
	defer sourceIter.Release()

	var (
		verifiedKeys int64
		errorCount   int64
		lastReport   = time.Now()
	)

	for sourceIter.Next() {
		key := sourceIter.Key()
		sourceValue := sourceIter.Value()

		// Get value from target database
		targetValue, err := m.TargetDB.Get(key)
		if err != nil {
			atomic.AddInt64(&errorCount, 1)
			m.Logger.Error("Key missing in target database",
				"key", fmt.Sprintf("%x", key[:min(len(key), 32)]),
				"err", err)
			continue
		}

		// Compare values
		if len(sourceValue) != len(targetValue) {
			atomic.AddInt64(&errorCount, 1)
			m.Logger.Error("Value length mismatch",
				"key", fmt.Sprintf("%x", key[:min(len(key), 32)]),
				"sourceLen", len(sourceValue),
				"targetLen", len(targetValue))
			continue
		}

		// Byte-by-byte comparison for accuracy
		for i := 0; i < len(sourceValue); i++ {
			if sourceValue[i] != targetValue[i] {
				atomic.AddInt64(&errorCount, 1)
				m.Logger.Error("Value content mismatch",
					"key", fmt.Sprintf("%x", key[:min(len(key), 32)]),
					"position", i)
				break
			}
		}

		atomic.AddInt64(&verifiedKeys, 1)

		// Report verification progress
		if time.Since(lastReport) >= 10*time.Second {
			progress := float64(verifiedKeys) / float64(m.Stats.TotalKeys) * 100
			m.Logger.Info("Verification progress",
				"progress", fmt.Sprintf("%.2f%%", progress),
				"verified", verifiedKeys,
				"errors", errorCount,
				"duration", time.Since(startTime).Truncate(time.Second))
			lastReport = time.Now()
		}

		// Check for cancellation
		select {
		case <-m.Ctx.Done():
			return m.Ctx.Err()
		default:
		}
	}

	if err := sourceIter.Error(); err != nil {
		return fmt.Errorf("iterator error during verification: %w", err)
	}

	verificationDuration := time.Since(startTime)

	if errorCount > 0 {
		m.Logger.Error("Migration verification failed",
			"verifiedKeys", verifiedKeys,
			"errorCount", errorCount,
			"duration", verificationDuration)
		return fmt.Errorf("verification failed: %d errors found out of %d keys", errorCount, verifiedKeys)
	}

	m.Logger.Info("Migration verification completed successfully",
		"verifiedKeys", verifiedKeys,
		"duration", verificationDuration)

	return nil
}

// Run executes the migration process
func (m *DatabaseMigrator) Run() error {
	if m.Ctx.Err() != nil {
		return m.Ctx.Err()
	}

	defer m.Cancel()

	// Perform migration
	if err := m.migrate(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Perform verification if requested
	if err := m.verify(); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	return nil
}

// openDatabase opens a database with the specified type and configuration
func OpenDatabase(dbType, dataDir string, readonly bool) (ethdb.Database, error) {
	switch strings.ToLower(dbType) {
	case "pebble":
		kvdb, err := pebble.New(dataDir, 256, 256, "geth/db/chaindata/", readonly, false)
		if err != nil {
			return nil, err
		}
		// Wrap with rawdb.NewDatabase to get full ethdb.Database interface
		return rawdb.NewDatabase(kvdb), nil
	case "rocksdb":
		kvdb, err := rocksdb.New(dataDir, 256, 256, "geth/db/chaindata/", readonly)
		if err != nil {
			return nil, err
		}
		// Wrap with rawdb.NewDatabase to get full ethdb.Database interface
		return rawdb.NewDatabase(kvdb), nil
	case "leveldb":
		// LevelDB implementation would go here if needed
		return nil, fmt.Errorf("leveldb migration not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
