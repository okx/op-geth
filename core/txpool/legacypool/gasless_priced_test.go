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
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// bcGaslessAllowAll is EVM bytecode for a stub Gasless predeploy whose
// getGaslessAllowance always returns (allowed=true, gasLimit=0xFFFFFFFF) — an
// allowance large enough to cover ordinary 21000-gas txs. Memory layout matches
// types.DecodeGaslessAllowance: word[0] non-zero => allowed, word[1] low 8 bytes
// => gasLimit.
var bcGaslessAllowAll = []byte{
	0x60, 0x01, 0x60, 0x00, 0x52, // MSTORE(0, 1)            -> allowed = true
	0x63, 0xFF, 0xFF, 0xFF, 0xFF, // PUSH4 0xFFFFFFFF
	0x60, 0x20, 0x52, // MSTORE(32, 0xFFFFFFFF)  -> gasLimit
	0x60, 0x40, 0x60, 0x00, 0xF3, // RETURN(0, 64)
}

// allowAllGaslessChecker returns a checker factory that authorizes every tx
// with the gasless shape (zero fees, eligible type, non-nil recipient) up to an
// effectively unlimited gas allowance. It stands in for the Gasless predeploy.
func allowAllGaslessChecker() func() types.GaslessChecker {
	return func() types.GaslessChecker {
		return func(*types.Transaction) (types.GaslessAllowance, error) {
			return types.GaslessAllowance{Allowed: true, GasLimit: math.MaxUint64}, nil
		}
	}
}

// gaslessTx builds a zero-fee dynamic-fee tx (the gasless shape) with the given
// nonce.
func gaslessTx(t *testing.T, nonce uint64) *types.Transaction {
	t.Helper()
	key, _ := crypto.GenerateKey()
	return dynamicFeeTx(nonce, 21000, big.NewInt(0), big.NewInt(0), key)
}

// feeTx builds a fee-paying dynamic-fee tx with the given fee cap.
func feeTx(t *testing.T, nonce uint64, feeCap int64) *types.Transaction {
	t.Helper()
	key, _ := crypto.GenerateKey()
	return dynamicFeeTx(nonce, 21000, big.NewInt(feeCap), big.NewInt(feeCap), key)
}

// containsHash reports whether txs contains a tx with the given hash.
func containsHash(txs types.Transactions, h common.Hash) bool {
	for _, tx := range txs {
		if tx.Hash() == h {
			return true
		}
	}
	return false
}

// heapHashes collects the hashes currently tracked across both priced heaps.
func heapHashes(l *pricedList) map[common.Hash]struct{} {
	out := make(map[common.Hash]struct{})
	for _, tx := range l.urgent.list {
		out[tx.Hash()] = struct{}{}
	}
	for _, tx := range l.floating.list {
		out[tx.Hash()] = struct{}{}
	}
	return out
}

// TestPricedListUnderpricedExemptsGasless asserts that a zero-fee gasless tx is
// never reported as underpriced (so a full pool still admits it), while a
// sub-floor fee-paying tx still is.
func TestPricedListUnderpricedExemptsGasless(t *testing.T) {
	all := newLookup()
	l := newPricedList(all)
	l.newGaslessChecker = allowAllGaslessChecker()

	// Fill the heaps with fee-paying transactions so the pool looks congested.
	for i := 0; i < 4; i++ {
		tx := feeTx(t, uint64(i), 100)
		all.Add(tx)
		l.Put(tx)
	}

	// A zero-fee gasless tx must be exempt from the underpriced check.
	if l.Underpriced(gaslessTx(t, 0)) {
		t.Fatal("gasless tx must be exempt from underpriced rejection")
	}

	// Control: with gasless disabled (no checker), the same zero-fee tx is
	// underpriced as before.
	l.newGaslessChecker = nil
	if !l.Underpriced(gaslessTx(t, 0)) {
		t.Fatal("with gasless disabled, a zero-fee tx must be underpriced")
	}

	// Control: a fee-paying tx cheaper than the pool floor is underpriced.
	l.newGaslessChecker = allowAllGaslessChecker()
	if !l.Underpriced(feeTx(t, 0, 1)) {
		t.Fatal("a sub-floor fee-paying tx should be underpriced")
	}
}

// TestPricedListDiscardExemptsGasless asserts that congestion eviction never
// drops live gasless txs: they are the cheapest and would normally be evicted
// first, but must be retained and re-inserted into the heaps.
func TestPricedListDiscardExemptsGasless(t *testing.T) {
	all := newLookup()
	l := newPricedList(all)
	l.newGaslessChecker = allowAllGaslessChecker()

	var gaslessHashes []common.Hash
	// Two zero-fee gasless txs (cheapest — eviction targets them first).
	for i := 0; i < 2; i++ {
		tx := gaslessTx(t, uint64(i))
		all.Add(tx)
		l.Put(tx)
		gaslessHashes = append(gaslessHashes, tx.Hash())
	}
	// Three fee-paying txs that are legitimate eviction candidates.
	for i := 0; i < 3; i++ {
		tx := feeTx(t, uint64(i), 100)
		all.Add(tx)
		l.Put(tx)
	}

	// Ask to free three slots; only the fee-paying txs should be discarded.
	drop, ok := l.Discard(3)
	if !ok {
		t.Fatal("Discard should succeed when enough fee-paying txs can be evicted")
	}
	for _, h := range gaslessHashes {
		if containsHash(drop, h) {
			t.Fatalf("gasless tx %s must not be discarded", h.Hex())
		}
	}
	// The exempt gasless txs must remain tracked in the heaps.
	remaining := heapHashes(l)
	for _, h := range gaslessHashes {
		if _, ok := remaining[h]; !ok {
			t.Fatalf("gasless tx %s must be retained in the priced heaps after Discard", h.Hex())
		}
	}
}

// TestSetGasTipExemptsGasless is an end-to-end check that raising the pool's
// minimum tip does not evict a live gasless tx (zero tip), while it still drops
// a fee-paying tx whose tip falls below the new threshold.
func TestSetGasTipExemptsGasless(t *testing.T) {
	gaslessKey, _ := crypto.GenerateKey()
	normalKey, _ := crypto.GenerateKey()
	gaslessAddr := crypto.PubkeyToAddress(gaslessKey.PublicKey)
	normalAddr := crypto.PubkeyToAddress(normalKey.PublicKey)

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	// Deploy the Gasless predeploy stub for the test chain and fund both senders.
	statedb.SetCode(types.GaslessAddressFor(params.TestChainConfig.ChainID), bcGaslessAllowAll, tracing.CodeChangeUnspecified)
	statedb.SetBalance(gaslessAddr, new(uint256.Int).SetUint64(params.Ether), tracing.BalanceChangeUnspecified)
	statedb.SetBalance(normalAddr, new(uint256.Int).SetUint64(params.Ether), tracing.BalanceChangeUnspecified)

	blockchain := newTestBlockChain(params.TestChainConfig, 10_000_000, statedb, new(event.Feed))
	config := testTxPoolConfig
	config.AllowGasless = true
	pool := New(config, blockchain)
	if err := pool.Init(config.PriceLimit, blockchain.CurrentBlock(), newReserver()); err != nil {
		t.Fatalf("pool init: %v", err)
	}
	<-pool.initDoneCh
	defer pool.Close()

	gasless := dynamicFeeTx(0, 21000, big.NewInt(0), big.NewInt(0), gaslessKey)
	normal := dynamicFeeTx(0, 21000, big.NewInt(5), big.NewInt(5), normalKey)
	for i, err := range pool.addRemotesSync([]*types.Transaction{gasless, normal}) {
		if err != nil {
			t.Fatalf("failed to add tx %d: %v", i, err)
		}
	}
	if pool.Get(gasless.Hash()) == nil {
		t.Fatal("gasless tx was not admitted to the pool")
	}
	if pool.Get(normal.Hash()) == nil {
		t.Fatal("normal tx was not admitted to the pool")
	}

	// Raise the minimum tip above the fee-paying tx's tip (and far above the
	// gasless tx's zero tip).
	pool.SetGasTip(big.NewInt(100))

	if pool.Get(gasless.Hash()) == nil {
		t.Fatal("gasless tx was dropped by SetGasTip despite being fee-exempt")
	}
	if pool.Get(normal.Hash()) != nil {
		t.Fatal("sub-tip fee-paying tx should have been dropped by SetGasTip")
	}
}
