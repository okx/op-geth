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
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/ethdb/rocksdb"
	"github.com/ethereum/go-ethereum/log"
)

func TestMigrationStats(t *testing.T) {
	stats := &MigrationStats{
		StartTime:      time.Now().Add(-10 * time.Second),
		TotalKeys:      1000,
		TotalBytes:     10000,
		ProcessedKeys:  500,
		ProcessedBytes: 5000,
		BatchesWritten: 10,
		ErrorCount:     2,
	}

	// Test progress calculation
	if progress := stats.Progress(); progress != 50.0 {
		t.Errorf("expected progress 50%%, got %.2f%%", progress)
	}

	if bytesProgress := stats.BytesProgress(); bytesProgress != 50.0 {
		t.Errorf("expected bytes progress 50%%, got %.2f%%", bytesProgress)
	}

	// Test duration calculation
	duration := stats.Duration()
	if duration < 9*time.Second || duration > 11*time.Second {
		t.Errorf("expected duration around 10s, got %v", duration)
	}

	// Test string representation
	statsStr := stats.String()
	if len(statsStr) == 0 {
		t.Error("stats string should not be empty")
	}
	t.Logf("Stats string: %s", statsStr)
}

func TestDatabaseMigrator_MemoryToMemory(t *testing.T) {
	// Create source and target memory databases
	sourceDB := rawdb.NewDatabase(memorydb.New())
	targetDB := rawdb.NewDatabase(memorydb.New())

	// Populate source database with test data
	testData := generateTestData(t, 100)
	for key, value := range testData {
		if err := sourceDB.Put([]byte(key), value); err != nil {
			t.Fatalf("failed to put test data: %v", err)
		}
	}

	// Create migrator
	migrator := NewDatabaseMigrator(sourceDB, targetDB, 1024, 10, true)

	// Run migration
	if err := migrator.Run(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify all data was migrated
	for key, expectedValue := range testData {
		actualValue, err := targetDB.Get([]byte(key))
		if err != nil {
			t.Errorf("key %s not found in target database: %v", key, err)
			continue
		}

		if !bytes.Equal(expectedValue, actualValue) {
			t.Errorf("value mismatch for key %s: expected %x, got %x", key, expectedValue, actualValue)
		}
	}

	// Check statistics
	stats := migrator.Stats
	if stats.ProcessedKeys != int64(len(testData)) {
		t.Errorf("expected %d processed keys, got %d", len(testData), stats.ProcessedKeys)
	}

	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}
}

func TestDatabaseMigrator_LargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large dataset test in short mode")
	}

	// Create source and target memory databases
	sourceDB := rawdb.NewDatabase(memorydb.New())
	targetDB := rawdb.NewDatabase(memorydb.New())

	// Generate larger dataset
	testData := generateTestData(t, 10000)
	for key, value := range testData {
		if err := sourceDB.Put([]byte(key), value); err != nil {
			t.Fatalf("failed to put test data: %v", err)
		}
	}

	// Create migrator with smaller batches to test batching logic
	migrator := NewDatabaseMigrator(sourceDB, targetDB, 512, 5, false)

	// Run migration
	if err := migrator.Run(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify data count matches
	sourceIter := sourceDB.NewIterator(nil, nil)
	targetIter := targetDB.NewIterator(nil, nil)
	defer sourceIter.Release()
	defer targetIter.Release()

	sourceCount := 0
	for sourceIter.Next() {
		sourceCount++
	}

	targetCount := 0
	for targetIter.Next() {
		targetCount++
	}

	if sourceCount != targetCount {
		t.Errorf("key count mismatch: source %d, target %d", sourceCount, targetCount)
	}

	if sourceCount != len(testData) {
		t.Errorf("expected %d keys, got %d", len(testData), sourceCount)
	}
}

func TestDatabaseMigrator_CancellationSupport(t *testing.T) {
	// Create source and target memory databases
	sourceDB := rawdb.NewDatabase(memorydb.New())
	targetDB := rawdb.NewDatabase(memorydb.New())

	// Populate with substantial test data
	testData := generateTestData(t, 1000)
	for key, value := range testData {
		if err := sourceDB.Put([]byte(key), value); err != nil {
			t.Fatalf("failed to put test data: %v", err)
		}
	}

	// Create migrator
	migrator := NewDatabaseMigrator(sourceDB, targetDB, 1024, 10, false)

	// Cancel migration after a short delay
	time.Sleep(2 * time.Millisecond)
	migrator.Cancel()
	time.Sleep(2 * time.Millisecond)

	// Run migration (should be cancelled)
	err := migrator.Run()
	if err == nil {
		t.Error("expected migration to be cancelled")
	}

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestPebbleToRocksDBMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pebble to rocksdb test in short mode")
	}

	// Setup temporary directories
	tempDir := t.TempDir()
	pebbleDir := filepath.Join(tempDir, "pebble")
	rocksdbDir := filepath.Join(tempDir, "rocksdb")

	// Create directories
	if err := os.MkdirAll(pebbleDir, 0755); err != nil {
		t.Fatalf("failed to create pebble directory: %v", err)
	}
	if err := os.MkdirAll(rocksdbDir, 0755); err != nil {
		t.Fatalf("failed to create rocksdb directory: %v", err)
	}

	// Create and populate Pebble database
	pebbleKV, err := pebble.New(pebbleDir, 16, 16, "", false, false)
	if err != nil {
		t.Fatalf("failed to create pebble database: %v", err)
	}
	pebbleDB := rawdb.NewDatabase(pebbleKV)

	// Add test data
	testData := generateTestData(t, 500)
	batch := pebbleDB.NewBatch()
	for key, value := range testData {
		if err := batch.Put([]byte(key), value); err != nil {
			batch.Reset()
			pebbleDB.Close()
			t.Fatalf("failed to add test data to batch: %v", err)
		}
	}
	if err := batch.Write(); err != nil {
		batch.Reset()
		pebbleDB.Close()
		t.Fatalf("failed to write batch: %v", err)
	}
	pebbleDB.Close()

	// Reopen Pebble database in read-only mode
	pebbleKVRO, err := pebble.New(pebbleDir, 16, 16, "", true, false)
	if err != nil {
		t.Fatalf("failed to reopen pebble database: %v", err)
	}
	pebbleDB = rawdb.NewDatabase(pebbleKVRO)
	defer pebbleDB.Close()

	// Create RocksDB database
	rocksKV, err := rocksdb.New(rocksdbDir, 16, 16, "", false)
	if err != nil {
		t.Fatalf("failed to create rocksdb database: %v", err)
	}
	rocksDB := rawdb.NewDatabase(rocksKV)
	defer rocksDB.Close()

	// Create migrator and run migration
	migrator := NewDatabaseMigrator(pebbleDB, rocksDB, 1024, 50, true)
	if err := migrator.Run(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify migration success
	stats := migrator.Stats
	if stats.ProcessedKeys != int64(len(testData)) {
		t.Errorf("expected %d processed keys, got %d", len(testData), stats.ProcessedKeys)
	}

	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}

	t.Logf("Migration completed: %s", stats.String())
}

func TestLevelDBToRocksDBMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leveldb to rocksdb test in short mode")
	}

	// Setup temporary directories
	tempDir := t.TempDir()
	leveldbDir := filepath.Join(tempDir, "leveldb")
	rocksdbDir := filepath.Join(tempDir, "rocksdb")

	// Create directories
	if err := os.MkdirAll(leveldbDir, 0755); err != nil {
		t.Fatalf("failed to create leveldb directory: %v", err)
	}
	if err := os.MkdirAll(rocksdbDir, 0755); err != nil {
		t.Fatalf("failed to create rocksdb directory: %v", err)
	}

	// Create and populate LevelDB database
	levelKV, err := leveldb.New(leveldbDir, 16, 16, "", false)
	if err != nil {
		t.Fatalf("failed to create leveldb database: %v", err)
	}
	levelDB := rawdb.NewDatabase(levelKV)

	// Add test data
	testData := generateTestData(t, 500)
	batch := levelDB.NewBatch()
	for key, value := range testData {
		if err := batch.Put([]byte(key), value); err != nil {
			batch.Reset()
			levelDB.Close()
			t.Fatalf("failed to add test data to batch: %v", err)
		}
	}
	if err := batch.Write(); err != nil {
		batch.Reset()
		levelDB.Close()
		t.Fatalf("failed to write batch: %v", err)
	}
	levelDB.Close()

	// Reopen LevelDB database in read-only mode
	levelKVRO, err := leveldb.New(leveldbDir, 16, 16, "", true)
	if err != nil {
		t.Fatalf("failed to reopen leveldb database: %v", err)
	}
	levelDB = rawdb.NewDatabase(levelKVRO)
	defer levelDB.Close()

	// Create RocksDB database
	rocksKV, err := rocksdb.New(rocksdbDir, 16, 16, "", false)
	if err != nil {
		t.Fatalf("failed to create rocksdb database: %v", err)
	}
	rocksDB := rawdb.NewDatabase(rocksKV)
	defer rocksDB.Close()

	// Create migrator and run migration
	migrator := NewDatabaseMigrator(levelDB, rocksDB, 1024, 50, true)
	if err := migrator.Run(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify migration success
	stats := migrator.Stats
	if stats.ProcessedKeys != int64(len(testData)) {
		t.Errorf("expected %d processed keys, got %d", len(testData), stats.ProcessedKeys)
	}

	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}

	t.Logf("LevelDB to RocksDB migration completed: %s", stats.String())
}

func TestOpenDatabase(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		dbType   string
		readonly bool
		wantErr  bool
	}{
		{"pebble", false, false},
		{"pebble", true, false},
		{"rocksdb", false, false},
		{"rocksdb", true, false},
		{"leveldb", false, false}, // Now implemented
		{"leveldb", true, false},  // Now implemented
		{"invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_readonly_%t", tt.dbType, tt.readonly), func(t *testing.T) {
			dbDir := filepath.Join(tempDir, tt.dbType)
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				t.Fatalf("failed to create directory: %v", err)
			}

			db, err := OpenDatabase(tt.dbType, dbDir, tt.readonly)
			if (err != nil) != tt.wantErr {
				t.Errorf("openDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if db != nil {
				// Test basic database operations if not readonly
				if !tt.readonly {
					testKey := []byte("test-key")
					testValue := []byte("test-value")

					// Test Put/Get
					if err := db.Put(testKey, testValue); err != nil {
						t.Errorf("failed to put data: %v", err)
					}

					retrievedValue, err := db.Get(testKey)
					if err != nil {
						t.Errorf("failed to get data: %v", err)
					}

					if !bytes.Equal(testValue, retrievedValue) {
						t.Errorf("value mismatch: expected %x, got %x", testValue, retrievedValue)
					}
				}

				db.Close()
			}
		})
	}
}

func TestMigrationWithCorruptedData(t *testing.T) {
	// Setup logger to capture error messages - simplified for testing
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelWarn, false)))

	sourceDB := rawdb.NewDatabase(memorydb.New())
	targetDB := &faultyDatabase{Database: rawdb.NewDatabase(memorydb.New()), putFailureRate: 0.1}

	// Add test data
	testData := generateTestData(t, 100)
	for key, value := range testData {
		if err := sourceDB.Put([]byte(key), value); err != nil {
			t.Fatalf("failed to put test data: %v", err)
		}
	}

	// Create migrator
	migrator := NewDatabaseMigrator(sourceDB, targetDB, 1024, 10, false)

	// Run migration (should have some errors but continue)
	if err := migrator.Run(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Check that some errors were recorded
	stats := migrator.Stats
	if stats.ErrorCount == 0 {
		t.Error("expected some errors due to faulty database, got none")
	}

	t.Logf("Migration with faulty database: %s", stats.String())
}

// generateTestData creates a map of test key-value pairs
func generateTestData(t *testing.T, count int) map[string][]byte {
	testData := make(map[string][]byte)

	for i := 0; i < count; i++ {
		// Generate realistic blockchain-like keys
		key := fmt.Sprintf("trie-node-%08d", i)

		// Generate random value data
		valueSize := 32 + (i % 256) // Variable size values
		value := make([]byte, valueSize)
		if _, err := rand.Read(value); err != nil {
			t.Fatalf("failed to generate random data: %v", err)
		}

		testData[key] = value
	}

	// Add some special keys that might be found in a real blockchain database
	specialKeys := []string{
		"LastHeader",
		"LastBlock",
		"LastFast",
		"TrieCleanCacheSize",
		"SnapshotSyncStatus",
	}

	for _, key := range specialKeys {
		value := make([]byte, 64)
		if _, err := rand.Read(value); err != nil {
			t.Fatalf("failed to generate random data: %v", err)
		}
		testData[key] = value
	}

	return testData
}

// faultyDatabase is a test database that fails operations with a specified rate
type faultyDatabase struct {
	ethdb.Database
	putFailureRate float64
}

func (db *faultyDatabase) Put(key []byte, value []byte) error {
	// Simulate random failures
	if len(key)%10 < int(db.putFailureRate*10) {
		return fmt.Errorf("simulated database failure")
	}
	return db.Database.Put(key, value)
}

func (db *faultyDatabase) NewBatch() ethdb.Batch {
	return &faultyBatch{
		Batch:          db.Database.NewBatch(),
		putFailureRate: db.putFailureRate,
	}
}

func (db *faultyDatabase) NewBatchWithSize(size int) ethdb.Batch {
	return &faultyBatch{
		Batch:          db.Database.NewBatchWithSize(size),
		putFailureRate: db.putFailureRate,
	}
}

// faultyBatch is a test batch that fails operations with a specified rate
type faultyBatch struct {
	ethdb.Batch
	putFailureRate float64
}

func (b *faultyBatch) Put(key, value []byte) error {
	// Simulate random failures
	if len(key)%10 < int(b.putFailureRate*10) {
		return fmt.Errorf("simulated batch failure")
	}
	return b.Batch.Put(key, value)
}

func TestMinFunction(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 2, -1},
		{10, 10, 10},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

// Benchmark migration performance
func BenchmarkDatabaseMigration(b *testing.B) {
	sourceDB := rawdb.NewDatabase(memorydb.New())
	targetDB := rawdb.NewDatabase(memorydb.New())

	// Prepare test data
	testData := make(map[string][]byte)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("benchmark-key-%08d", i)
		value := make([]byte, 256)
		rand.Read(value)
		testData[key] = value
		sourceDB.Put([]byte(key), value)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Clear target database
		targetDB = rawdb.NewDatabase(memorydb.New())

		// Run migration
		migrator := NewDatabaseMigrator(sourceDB, targetDB, 1024, 100, false)
		if err := migrator.Run(); err != nil {
			b.Fatalf("migration failed: %v", err)
		}
	}
}
