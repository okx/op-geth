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

package legacypool

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// gaslessEvictTestTo is the recipient used for the synthetic gasless txs.
var gaslessEvictTestTo = common.HexToAddress("0x00000000000000000000000000000000000ca511")

// insertPendingTx places a signed tx (from a freshly funded, reserved sender)
// directly into the pending set, mirroring TestSetGasTip_GaslessNotEvicted.
func insertPendingTx(t *testing.T, pool *LegacyPool, tx *types.Transaction, sender common.Address) {
	t.Helper()
	testAddBalance(pool, sender, big.NewInt(1_000_000_000_000_000_000))
	pool.mu.Lock()
	defer pool.mu.Unlock()
	_ = pool.reserver.Hold(sender)
	pool.all.Add(tx)
	pool.priced.Put(tx)
	pool.promoteTx(sender, tx.Hash(), tx)
}

// signedGaslessTx builds a signed zero-fee tx aged by `age` (via SetTime).
func signedGaslessTx(t *testing.T, signer types.Signer, age time.Duration) (*types.Transaction, common.Address) {
	t.Helper()
	key, _ := crypto.GenerateKey()
	tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID:   params.TestChainConfig.ChainID,
		Nonce:     0,
		To:        &gaslessEvictTestTo,
		Gas:       21000,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
	}), signer, key)
	if err != nil {
		t.Fatalf("sign gasless tx: %v", err)
	}
	tx.SetTime(time.Now().Add(-age))
	return tx, crypto.PubkeyToAddress(key.PublicKey)
}

// signedNormalTx builds a signed fee-paying tx aged by `age`.
func signedNormalTx(t *testing.T, signer types.Signer, age time.Duration) (*types.Transaction, common.Address) {
	t.Helper()
	key, _ := crypto.GenerateKey()
	tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID:   params.TestChainConfig.ChainID,
		Nonce:     0,
		To:        &gaslessEvictTestTo,
		Gas:       21000,
		GasFeeCap: big.NewInt(1_000_000_000),
		GasTipCap: big.NewInt(1_000_000_000),
	}), signer, key)
	if err != nil {
		t.Fatalf("sign normal tx: %v", err)
	}
	tx.SetTime(time.Now().Add(-age))
	return tx, crypto.PubkeyToAddress(key.PublicKey)
}

func deployGaslessPredeploy(pool *LegacyPool) {
	predeploy := types.GaslessAddressFor(params.TestChainConfig.ChainID)
	pool.mu.Lock()
	pool.currentState.SetCode(predeploy, gaslessPredeployCode, tracing.CodeChangeUnspecified)
	pool.mu.Unlock()
}

// runGaslessEviction performs one eviction sweep exactly as the pool's loop does.
func runGaslessEviction(pool *LegacyPool) int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	list := pool.gaslessEvictList()
	for _, hash := range list {
		pool.removeTx(hash, true, true)
	}
	return len(list)
}

// TestGaslessLifetimeEviction verifies that only gasless (zero-fee) txs older
// than GaslessLifetime are dropped: a stale gasless tx is evicted, while a fresh
// gasless tx and a stale fee-paying tx are kept.
func TestGaslessLifetimeEviction(t *testing.T) {
	cfg := testTxPoolConfig
	cfg.AllowGasless = true
	cfg.GaslessLifetime = 30 * time.Minute
	pool, _ := setupPoolWithTxPoolConfig(params.TestChainConfig, cfg)
	defer pool.Close()
	deployGaslessPredeploy(pool)

	signer := types.LatestSignerForChainID(params.TestChainConfig.ChainID)
	staleGasless, staleGaslessAddr := signedGaslessTx(t, signer, 31*time.Minute) // older than cap → evict
	freshGasless, freshGaslessAddr := signedGaslessTx(t, signer, 1*time.Minute)  // within cap → keep
	staleNormal, staleNormalAddr := signedNormalTx(t, signer, 31*time.Minute)    // fee-paying → keep

	insertPendingTx(t, pool, staleGasless, staleGaslessAddr)
	insertPendingTx(t, pool, freshGasless, freshGaslessAddr)
	insertPendingTx(t, pool, staleNormal, staleNormalAddr)

	for _, tx := range []*types.Transaction{staleGasless, freshGasless, staleNormal} {
		if pool.Get(tx.Hash()) == nil {
			t.Fatalf("setup: tx %s should be present before eviction", tx.Hash().Hex())
		}
	}

	if n := runGaslessEviction(pool); n != 1 {
		t.Fatalf("expected exactly 1 gasless tx evicted, got %d", n)
	}
	if pool.Get(staleGasless.Hash()) != nil {
		t.Fatal("stale gasless tx should have been evicted")
	}
	if pool.Get(freshGasless.Hash()) == nil {
		t.Fatal("fresh gasless tx must be kept")
	}
	if pool.Get(staleNormal.Hash()) == nil {
		t.Fatal("stale fee-paying tx must be kept (not subject to the gasless cap)")
	}
}

// TestGaslessLifetimeEvictionDisabled verifies the cap is inert when gasless is
// disabled or the lifetime is non-positive, even for a very old zero-fee tx.
func TestGaslessLifetimeEvictionDisabled(t *testing.T) {
	t.Run("gasless disabled", func(t *testing.T) {
		cfg := testTxPoolConfig
		cfg.AllowGasless = false
		cfg.GaslessLifetime = 30 * time.Minute
		pool, _ := setupPoolWithTxPoolConfig(params.TestChainConfig, cfg)
		defer pool.Close()

		pool.mu.Lock()
		got := pool.gaslessEvictList()
		pool.mu.Unlock()
		if got != nil {
			t.Fatalf("no eviction expected when AllowGasless is false, got %d", len(got))
		}
	})

	t.Run("zero lifetime falls back to default", func(t *testing.T) {
		// A non-positive GaslessLifetime (e.g. from a config file that predates
		// the field) is sanitized back to the default rather than disabling the
		// cap, so an old gasless tx is still evicted.
		cfg := testTxPoolConfig
		cfg.AllowGasless = true
		cfg.GaslessLifetime = 0
		pool, _ := setupPoolWithTxPoolConfig(params.TestChainConfig, cfg)
		defer pool.Close()
		deployGaslessPredeploy(pool)

		if pool.config.GaslessLifetime != DefaultConfig.GaslessLifetime {
			t.Fatalf("GaslessLifetime should be sanitized to default %v, got %v",
				DefaultConfig.GaslessLifetime, pool.config.GaslessLifetime)
		}

		signer := types.LatestSignerForChainID(params.TestChainConfig.ChainID)
		old, addr := signedGaslessTx(t, signer, 24*time.Hour)
		insertPendingTx(t, pool, old, addr)

		if n := runGaslessEviction(pool); n != 1 {
			t.Fatalf("old gasless tx should be evicted under the default cap, got %d", n)
		}
		if pool.Get(old.Hash()) != nil {
			t.Fatal("old gasless tx should have been evicted")
		}
	})
}
