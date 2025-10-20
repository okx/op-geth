// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"testing"
	"time"
)

// TestXlayerAncientProxyShouldProxy tests the ShouldProxy logic.
func TestXlayerAncientProxyShouldProxy(t *testing.T) {
	tests := []struct {
		name            string
		legacyThreshold uint64
		number          uint64
		expectedResult  bool
	}{
		{
			name:            "block 0 (genesis)",
			legacyThreshold: 1000,
			number:          0,
			expectedResult:  false,
		},
		{
			name:            "legacy block within range",
			legacyThreshold: 1000,
			number:          100,
			expectedResult:  true,
		},
		{
			name:            "block at threshold",
			legacyThreshold: 1000,
			number:          1000,
			expectedResult:  false,
		},
		{
			name:            "block above threshold",
			legacyThreshold: 1000,
			number:          1001,
			expectedResult:  false,
		},
		{
			name:            "edge case: threshold is 1",
			legacyThreshold: 1,
			number:          1,
			expectedResult:  false,
		},
		{
			name:            "edge case: threshold is 2, block 1",
			legacyThreshold: 2,
			number:          1,
			expectedResult:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock proxy without RPC client
			proxy := &XlayerAncientProxy{
				legacyThreshold: tt.legacyThreshold,
				rpcClient:       nil, // Not needed for ShouldProxy test
			}
			result := proxy.ShouldProxy(tt.number)
			if result != tt.expectedResult {
				t.Errorf("ShouldProxy(%d) = %v, want %v", tt.number, result, tt.expectedResult)
			}
		})
	}
}

// TestXlayerAncientProxyNil tests that nil proxy behaves correctly.
func TestXlayerAncientProxyNil(t *testing.T) {
	var proxy *XlayerAncientProxy = nil

	// Should return false for any block
	if proxy.ShouldProxy(100) {
		t.Error("nil proxy should not proxy any block")
	}

	// Close should not panic
	proxy.Close()
}

// TestNewXlayerAncientProxy tests proxy creation logic.
func TestNewXlayerAncientProxy(t *testing.T) {
	tests := []struct {
		name            string
		legacyThreshold uint64
		ppRPCUrl        string
		expectNil       bool
		description     string
	}{
		{
			name:            "empty URL",
			legacyThreshold: 1000,
			ppRPCUrl:        "",
			expectNil:       true,
			description:     "Empty URL should return nil",
		},
		{
			name:            "zero threshold",
			legacyThreshold: 0,
			ppRPCUrl:        "http://localhost:8545",
			expectNil:       true,
			description:     "Zero threshold should return nil",
		},
		{
			name:            "valid configuration",
			legacyThreshold: 1000,
			ppRPCUrl:        "http://localhost:8545",
			expectNil:       false,
			description:     "Should create proxy even if RPC is not reachable (will fail later)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := NewXlayerAncientProxy(tt.legacyThreshold, tt.ppRPCUrl, 100*time.Millisecond, 80)
			if tt.expectNil && proxy != nil {
				t.Errorf("%s: expected nil proxy, got non-nil", tt.description)
				proxy.Close()
			} else if !tt.expectNil && proxy == nil {
				t.Errorf("%s: expected non-nil proxy, got nil", tt.description)
			} else if proxy != nil {
				// Clean up
				proxy.Close()
			}
		})
	}
}

// Helper function to create a pointer to uint64
func ptr(v uint64) *uint64 {
	return &v
}
