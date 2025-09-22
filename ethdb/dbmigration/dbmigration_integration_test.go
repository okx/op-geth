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

//go:build integration
// +build integration

package dbmigration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethdb/pebble"
)

// TestDBMigrateCommandIntegration tests the db-migrate command through the CLI
func TestDBMigrateCommandIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup temporary directory structure
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source", "geth", "chaindata")
	targetDir := filepath.Join(tempDir, "target")

	// Create source directory
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Create and populate a test Pebble database
	sourceDB, err := pebble.New(sourceDir, 16, 16, "", false, false)
	if err != nil {
		t.Fatalf("failed to create source database: %v", err)
	}

	// Add some test data
	batch := sourceDB.NewBatch()
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("test-key-%08d", i)
		value := fmt.Sprintf("test-value-%08d-with-some-longer-content", i)
		if err := batch.Put([]byte(key), []byte(value)); err != nil {
			t.Fatalf("failed to add test data: %v", err)
		}
	}
	if err := batch.Write(); err != nil {
		t.Fatalf("failed to write test data: %v", err)
	}
	sourceDB.Close()

	// Build geth binary for testing
	gethBinary := buildGethBinary(t)
	defer os.Remove(gethBinary)

	// Test db-migrate command
	args := []string{
		"db-migrate",
		"--datadir", filepath.Join(tempDir, "source"),
		"--from", "pebble",
		"--to", "rocksdb",
		"--target-dir", targetDir,
		"--batch-size", "1024",
		"--batch-count", "10",
		"--verify",
	}

	cmd := exec.Command(gethBinary, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("db-migrate command failed: %v\nOutput: %s", err, output)
	}

	// Check that output contains expected messages
	outputStr := string(output)
	expectedPhrases := []string{
		"Starting database migration",
		"Migration completed successfully",
		"Verification completed successfully",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(outputStr, phrase) {
			t.Errorf("expected output to contain '%s', but it didn't.\nFull output: %s", phrase, outputStr)
		}
	}

	// Verify target directory was created
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Error("target directory was not created")
	}

	t.Logf("Migration integration test passed. Output:\n%s", outputStr)
}

// TestDBMigrateCommandHelp tests that the help message is displayed correctly
func TestDBMigrateCommandHelp(t *testing.T) {
	gethBinary := buildGethBinary(t)
	defer os.Remove(gethBinary)

	cmd := exec.Command(gethBinary, "db-migrate", "--help")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("db-migrate --help failed: %v", err)
	}

	outputStr := string(output)
	expectedPhrases := []string{
		"Migrate database from one backend to another",
		"--from",
		"--to",
		"--target-dir",
		"--verify",
		"pebble",
		"rocksdb",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(outputStr, phrase) {
			t.Errorf("help output should contain '%s', but it didn't.\nFull output: %s", phrase, outputStr)
		}
	}
}

// TestDBMigrateCommandValidation tests command validation
func TestDBMigrateCommandValidation(t *testing.T) {
	gethBinary := buildGethBinary(t)
	defer os.Remove(gethBinary)

	tests := []struct {
		name      string
		args      []string
		wantErr   bool
		errPhrase string
	}{
		{
			name:      "missing datadir",
			args:      []string{"db-migrate"},
			wantErr:   true,
			errPhrase: "data directory must be specified",
		},
		{
			name:      "same source and target",
			args:      []string{"db-migrate", "--datadir", "/tmp", "--from", "pebble", "--to", "pebble"},
			wantErr:   true,
			errPhrase: "source and target database types must be different",
		},
		{
			name:      "unsupported database type",
			args:      []string{"db-migrate", "--datadir", "/tmp", "--from", "unknown", "--to", "rocksdb"},
			wantErr:   true,
			errPhrase: "unsupported database type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(gethBinary, tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.wantErr && err == nil {
				t.Errorf("expected command to fail, but it succeeded. Output: %s", output)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected command to succeed, but it failed: %v. Output: %s", err, output)
			}

			if tt.wantErr && tt.errPhrase != "" {
				outputStr := string(output)
				if !strings.Contains(outputStr, tt.errPhrase) {
					t.Errorf("expected error output to contain '%s', but it didn't. Output: %s", tt.errPhrase, outputStr)
				}
			}
		})
	}
}

// buildGethBinary builds the geth binary for testing
func buildGethBinary(t *testing.T) string {
	// Create temporary binary name
	binaryPath := filepath.Join(t.TempDir(), "geth-test")

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = "." // Assuming we're in the cmd/geth directory

	// Set a reasonable timeout for building
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1") // Ensure CGO is enabled for RocksDB

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build geth binary: %v\nOutput: %s", err, output)
	}

	return binaryPath
}

// TestMigrationPerformance tests migration performance with larger datasets
func TestMigrationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	// Setup
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source", "geth", "chaindata")
	targetDir := filepath.Join(tempDir, "target")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	// Create a larger test database
	sourceDB, err := pebble.New(sourceDir, 64, 64, "", false, false)
	if err != nil {
		t.Fatalf("failed to create source database: %v", err)
	}

	// Add substantial test data
	const numEntries = 10000
	batch := sourceDB.NewBatch()
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("performance-test-key-%08d", i)
		value := make([]byte, 256) // 256 byte values
		for j := range value {
			value[j] = byte(i + j)
		}
		if err := batch.Put([]byte(key), value); err != nil {
			t.Fatalf("failed to add test data: %v", err)
		}

		// Write in batches to avoid memory issues
		if i%1000 == 999 {
			if err := batch.Write(); err != nil {
				t.Fatalf("failed to write batch: %v", err)
			}
			batch.Reset()
		}
	}
	if err := batch.Write(); err != nil {
		t.Fatalf("failed to write final batch: %v", err)
	}
	sourceDB.Close()

	// Build geth binary
	gethBinary := buildGethBinary(t)
	defer os.Remove(gethBinary)

	// Measure migration time
	startTime := time.Now()

	args := []string{
		"db-migrate",
		"--datadir", filepath.Join(tempDir, "source"),
		"--from", "pebble",
		"--to", "rocksdb",
		"--target-dir", targetDir,
		"--batch-size", "65536", // 64KB batches
		"--batch-count", "100",
	}

	cmd := exec.Command(gethBinary, args...)
	output, err := cmd.CombinedOutput()

	migrationDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("migration failed: %v\nOutput: %s", err, output)
	}

	// Performance checks
	entriesPerSecond := float64(numEntries) / migrationDuration.Seconds()
	bytesPerSecond := float64(numEntries*256) / migrationDuration.Seconds()

	t.Logf("Migration performance:")
	t.Logf("  Entries: %d", numEntries)
	t.Logf("  Duration: %v", migrationDuration)
	t.Logf("  Speed: %.0f entries/sec", entriesPerSecond)
	t.Logf("  Throughput: %.2f MB/sec", bytesPerSecond/1024/1024)

	// Basic performance expectations (adjust based on environment)
	if entriesPerSecond < 100 {
		t.Errorf("migration too slow: %.0f entries/sec (expected > 100)", entriesPerSecond)
	}

	t.Logf("Performance test output:\n%s", output)
}
