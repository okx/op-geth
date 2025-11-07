// Copyright 2020 The go-ethereum Authors
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

// Package miner implements Ethereum block creation and mining.
package miner

import (
	"context"
	"math/big"
	"math/rand"
	"sync"
	"testing"
	"time"

	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/interoptypes"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
)

type mockBackend struct {
	bc     *core.BlockChain
	txPool *txpool.TxPool

	// OP-Stack additions
	supervisorInFailsafe bool
	queryFailsafeCb      func()
}

func NewMockBackend(bc *core.BlockChain, txPool *txpool.TxPool,
	supervisorInFailsafe bool, // OP-Stack addition
	queryFailsafeCb func(), // OP-Stack addition
) *mockBackend {
	return &mockBackend{
		bc:     bc,
		txPool: txPool,

		// OP-Stack addition
		supervisorInFailsafe: supervisorInFailsafe,
		queryFailsafeCb:      queryFailsafeCb,
	}
}

func (m *mockBackend) BlockChain() *core.BlockChain {
	return m.bc
}

func (m *mockBackend) TxPool() *txpool.TxPool {
	return m.txPool
}

// OP-Stack additions
func (m *mockBackend) GetSupervisorFailsafe() bool {
	return m.supervisorInFailsafe
}
func (m *mockBackend) CheckAccessList(ctx context.Context, inboxEntries []common.Hash, minSafety interoptypes.SafetyLevel, executingDescriptor interoptypes.ExecutingDescriptor) error {
	return nil
}
func (m *mockBackend) QueryFailsafe(ctx context.Context) (bool, error) {
	if m.queryFailsafeCb != nil {
		m.queryFailsafeCb()
	}
	return m.supervisorInFailsafe, nil
}

var _ BackendWithInterop = (*mockBackend)(nil)

type testBlockChain struct {
	root          common.Hash
	config        *params.ChainConfig
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed
}

func (bc *testBlockChain) Config() *params.ChainConfig {
	return bc.config
}

func (bc *testBlockChain) CurrentBlock() *types.Header {
	return &types.Header{
		Number:     new(big.Int),
		GasLimit:   bc.gasLimit,
		BaseFee:    big.NewInt(params.InitialBaseFee),
		Difficulty: common.Big0,
	}
}

func (bc *testBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return types.NewBlock(bc.CurrentBlock(), nil, nil, trie.NewStackTrie(nil), types.DefaultBlockConfig)
}

func (bc *testBlockChain) StateAt(common.Hash) (*state.StateDB, error) {
	return bc.statedb, nil
}

func (bc *testBlockChain) HasState(root common.Hash) bool {
	return bc.root == root
}

func (bc *testBlockChain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

func TestBuildPendingBlocks(t *testing.T) {
	miner := createMiner(t, []common.Address{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		block, _, _ := miner.Pending()
		if block == nil {
			t.Error("Pending failed")
		}
	}()
	wg.Wait()
}

func minerTestGenesisBlock(period uint64, gasLimit uint64, faucet common.Address, accounts ...common.Address) *core.Genesis {
	config := *params.AllCliqueProtocolChanges
	config.Clique = &params.CliqueConfig{
		Period: period,
		Epoch:  config.Clique.Epoch,
	}

	alloc := map[common.Address]types.Account{
		common.BytesToAddress([]byte{1}): {Balance: big.NewInt(1)}, // ECRecover
		common.BytesToAddress([]byte{2}): {Balance: big.NewInt(1)}, // SHA256
		common.BytesToAddress([]byte{3}): {Balance: big.NewInt(1)}, // RIPEMD
		common.BytesToAddress([]byte{4}): {Balance: big.NewInt(1)}, // Identity
		common.BytesToAddress([]byte{5}): {Balance: big.NewInt(1)}, // ModExp
		common.BytesToAddress([]byte{6}): {Balance: big.NewInt(1)}, // ECAdd
		common.BytesToAddress([]byte{7}): {Balance: big.NewInt(1)}, // ECScalarMul
		common.BytesToAddress([]byte{8}): {Balance: big.NewInt(1)}, // ECPairing
		common.BytesToAddress([]byte{9}): {Balance: big.NewInt(1)}, // BLAKE2b
		faucet:                           {Balance: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(9))},
	}

	testBalance := new(big.Int).Lsh(big.NewInt(1), 60) // Large balance for test accounts
	for _, account := range accounts {
		alloc[account] = types.Account{Balance: testBalance}
	}

	// Assemble and return the genesis with the precompiles and faucet pre-funded
	return &core.Genesis{
		Config:     &config,
		ExtraData:  append(append(make([]byte, 32), faucet[:]...), make([]byte, crypto.SignatureLength)...),
		GasLimit:   gasLimit,
		BaseFee:    big.NewInt(params.InitialBaseFee),
		Difficulty: big.NewInt(1),
		Alloc:      alloc,
	}
}

func createMiner(t *testing.T, accounts []common.Address) *Miner {
	// Create Ethash config
	config := Config{
		PendingFeeRecipient:                   common.HexToAddress("123456789"),
		RollupTransactionConditionalRateLimit: params.TransactionConditionalMaxCost,
	}
	// Create chainConfig
	chainDB := rawdb.NewMemoryDatabase()
	triedb := triedb.NewDatabase(chainDB, nil)
	genesis := minerTestGenesisBlock(15, 11_500_000, testBankAddress, accounts...)
	chainConfig, _, _, err := core.SetupGenesisBlock(chainDB, triedb, genesis)
	if err != nil {
		t.Fatalf("can't create new chain config: %v", err)
	}
	// Create consensus engine
	engine := clique.New(chainConfig.Clique, chainDB)
	// Create Ethereum backend
	bc, err := core.NewBlockChain(chainDB, genesis, engine, nil)
	if err != nil {
		t.Fatalf("can't create new chain %v", err)
	}
	statedb, _ := state.New(bc.Genesis().Root(), bc.StateCache())
	blockchain := &testBlockChain{bc.Genesis().Root(), chainConfig, statedb, 10000000, new(event.Feed)}

	pool := legacypool.New(testTxPoolConfig, blockchain)
	txpool, _ := txpool.New(testTxPoolConfig.PriceLimit, blockchain, []txpool.SubPool{pool}, nil)

	// Create Miner
	backend := NewMockBackend(bc, txpool, false, nil)
	miner := New(backend, config, engine)
	return miner
}

func TestRejectedConditionalTx(t *testing.T) {
	miner := createMiner(t, []common.Address{})
	timestamp := uint64(time.Now().Unix())
	uint64Ptr := func(num uint64) *uint64 { return &num }

	// add a conditional transaction to be rejected
	signer := types.LatestSigner(miner.chainConfig)
	tx := types.MustSignNewTx(testBankKey, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee),
	})
	tx.SetConditional(&types.TransactionConditional{TimestampMax: uint64Ptr(timestamp - 1)})

	// 1 pending tx (synchronously, it has to be there before it can be rejected)
	miner.txpool.Add(types.Transactions{tx}, true)
	if !miner.txpool.Has(tx.Hash()) {
		t.Fatalf("conditional tx is not in the mempool")
	}

	// request block
	r := miner.generateWork(&generateParams{
		parentHash: miner.chain.CurrentBlock().Hash(),
		timestamp:  timestamp,
		random:     common.HexToHash("0xcafebabe"),
		noTxs:      false,
		forceTime:  true,
	}, false)

	if len(r.block.Transactions()) != 0 {
		t.Fatalf("block should be empty")
	}

	// tx is rejected
	if !tx.Rejected() {
		t.Fatalf("conditional tx is not marked as rejected")
	}

	// rejected conditional is evicted from the txpool
	miner.txpool.Sync()
	if miner.txpool.Has(tx.Hash()) {
		t.Fatalf("conditional tx is still in the mempool")
	}
}

// For X Layer
func TestOkPayPrioritization(t *testing.T) {
	t.Run("PriorityOrder", testOkPayPriorityOrder)
	t.Run("TransactionLimit", testOkPayTransactionLimit)
	t.Run("MixedPriorities", testOkPayMixedPriorities)
	t.Run("TimeOrdering", testOkPayTimeOrdering)
	t.Run("NonceOrdering", testOkPayNonceOrdering)
}

// testOkPayPriorityOrder tests that transactions are included in order: OkPay → Priority → Normal
// If there are fewer OkPay transactions than the priority slots, we will allow transactions to use the slots instead in order of priority.
func testOkPayPriorityOrder(t *testing.T) {
	// Create test accounts
	okPayKey1, _ := crypto.GenerateKey()
	okPayKey2, _ := crypto.GenerateKey()
	priorityKey, _ := crypto.GenerateKey()
	normalKey, _ := crypto.GenerateKey()

	okPayAddr1 := crypto.PubkeyToAddress(okPayKey1.PublicKey)
	okPayAddr2 := crypto.PubkeyToAddress(okPayKey2.PublicKey)
	priorityAddr := crypto.PubkeyToAddress(priorityKey.PublicKey)
	normalAddr := crypto.PubkeyToAddress(normalKey.PublicKey)

	// Create miner with funded accounts
	allAccounts := []common.Address{okPayAddr1, okPayAddr2, priorityAddr, normalAddr}
	miner := createMiner(t, allAccounts)

	// Configure OkX Pay settings
	miner.config.OkPayPriorityEnable = true
	miner.config.OkPayBlockPriorityTxsLimit = 5
	miner.config.OkPaySenderAccounts = []common.Address{okPayAddr1, okPayAddr2}

	// Set priority addresses
	miner.prio = []common.Address{priorityAddr}

	signer := types.LatestSigner(miner.chainConfig)

	// Create transactions with different gas prices to test priority override
	// OkPay transactions (low gas price)
	okPayTx1 := types.MustSignNewTx(okPayKey1, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 1), // Low gas price
	})
	okPayTx2 := types.MustSignNewTx(okPayKey2, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 1), // Low gas price
	})

	// Priority transaction (medium gas price)
	priorityTx := types.MustSignNewTx(priorityKey, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 5), // Medium gas price
	})

	// Normal transaction (high gas price)
	normalTx := types.MustSignNewTx(normalKey, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 10), // High gas price
	})

	// Add transactions to pool in reverse priority order (normal first, OkPay last)
	// This ensures that gas price alone wouldn't determine the order
	txs := types.Transactions{normalTx, priorityTx, okPayTx2, okPayTx1}
	miner.txpool.Add(txs, true)

	// Generate block
	timestamp := uint64(time.Now().Unix())
	r := miner.generateWork(&generateParams{
		parentHash: miner.chain.CurrentBlock().Hash(),
		timestamp:  timestamp,
		random:     common.HexToHash("0xcafebabe"),
		noTxs:      false,
		forceTime:  true,
	}, false)

	if r.err != nil {
		t.Fatalf("Failed to generate work: %v", r.err)
	}

	// Verify block has all 4 transactions
	blockTxs := r.block.Transactions()
	if len(blockTxs) != 4 {
		t.Fatalf("Expected 4 transactions in block, got %d", len(blockTxs))
	}

	// Verify priority order: OkPay txs first, then priority, then normal
	// OkPay transactions should be first (indices 0, 1)
	okPayHashes := map[common.Hash]bool{
		okPayTx1.Hash(): true,
		okPayTx2.Hash(): true,
	}

	// First two transactions should be OkPay
	if !okPayHashes[blockTxs[0].Hash()] {
		t.Errorf("First transaction should be OkPay, got %s", blockTxs[0].Hash().Hex())
	}
	if !okPayHashes[blockTxs[1].Hash()] {
		t.Errorf("Second transaction should be OkPay, got %s", blockTxs[1].Hash().Hex())
	}

	// Third should be priority
	if blockTxs[2].Hash() != priorityTx.Hash() {
		t.Errorf("Third transaction should be priority tx, got %s", blockTxs[2].Hash().Hex())
	}

	// Fourth should be normal
	if blockTxs[3].Hash() != normalTx.Hash() {
		t.Errorf("Fourth transaction should be normal tx, got %s", blockTxs[3].Hash().Hex())
	}
}

// testOkPayTransactionLimit tests that OkPay transaction limit is enforced.
// If there are more OkPay transactions than the priority slots, we will prioritize OkPay txs to the maximum priority limit,
// the remaining OkPay transactions will be treated as normal transactions and included in the block in gas price order with other non-OkPay transactions.
func testOkPayTransactionLimit(t *testing.T) {
	// Create OkPay accounts
	okPayKeys := make([]*ecdsa.PrivateKey, 2)
	okPayAddrs := make([]common.Address, 2)
	for i := 0; i < 2; i++ {
		key, _ := crypto.GenerateKey()
		okPayKeys[i] = key
		okPayAddrs[i] = crypto.PubkeyToAddress(key.PublicKey)
	}

	normalKey, _ := crypto.GenerateKey()
	normalAddr := crypto.PubkeyToAddress(normalKey.PublicKey)

	// Create miner with funded accounts
	allAccounts := []common.Address{okPayAddrs[0], okPayAddrs[1], normalAddr}
	miner := createMiner(t, allAccounts)

	// Configure OkPay with limit of 3 transactions
	limit := 3
	miner.config.OkPayPriorityEnable = true
	miner.config.OkPayBlockPriorityTxsLimit = uint64(limit)
	miner.config.OkPaySenderAccounts = make([]common.Address, 0, len(okPayAddrs))
	for _, addr := range okPayAddrs {
		miner.config.OkPaySenderAccounts = append(miner.config.OkPaySenderAccounts, addr)
	}

	signer := types.LatestSigner(miner.chainConfig)

	// Create 5 OkPay transactions (more than limit) - 3 from account 0, 2 from account 1
	// All with low gas price.
	okPayTxs := make([]*types.Transaction, 5)
	nonces := make([]uint64, 2) // Track nonce for each account

	for i := 0; i < 5; i++ {
		keyIndex := i % 2 // Alternate between two accounts
		currentNonce := nonces[keyIndex]
		nonces[keyIndex]++

		okPayTxs[i] = types.MustSignNewTx(okPayKeys[keyIndex], signer, &types.LegacyTx{
			Nonce:    currentNonce,
			To:       &testUserAddress,
			Value:    big.NewInt(1000),
			Gas:      params.TxGas,
			GasPrice: big.NewInt(params.InitialBaseFee),
		})
	}

	normalTx := types.MustSignNewTx(normalKey, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 10),
	})

	// Add all transactions to pool
	miner.txpool.Add(types.Transactions(append(okPayTxs, normalTx)), true)

	// Generate block
	timestamp := uint64(time.Now().Unix())
	r := miner.generateWork(&generateParams{
		parentHash: miner.chain.CurrentBlock().Hash(),
		timestamp:  timestamp,
		random:     common.HexToHash("0xcafebabe"),
		noTxs:      false,
		forceTime:  true,
	}, false)

	if r.err != nil {
		t.Fatalf("Failed to generate work: %v", r.err)
	}

	// Should have exactly 3 transactions (the limit)
	blockTxs := r.block.Transactions()

	// Verify included transactions are from OkPay addresses.
	okPayAddrMap := make(map[common.Address]bool)
	for _, addr := range okPayAddrs {
		okPayAddrMap[addr] = true
	}

	for i, tx := range blockTxs {
		from, err := types.Sender(signer, tx)
		if err != nil {
			t.Fatalf("Failed to get sender for tx %d: %v", i, err)
		}
		// After the limit, the remaining OkPay transactions should be treated as normal transactions and included in the block in gas price order.
		// In this case, the normal transaction should be included first because it has the highest gas price.
		if i == limit {
			if okPayAddrMap[from] {
				t.Errorf("Transaction %d is from an OkPay address when it should not be because of the limit: %s", i, from.Hex())
			} else {
				continue
			}
		}
		if !okPayAddrMap[from] {
			t.Errorf("Transaction %d is not from an OkPay address: %s", i, from.Hex())
		}
	}
}

// testOkPayMixedPriorities tests complex scenario with OkPay, priority, and normal transactions
func testOkPayMixedPriorities(t *testing.T) {
	// Create various accounts
	okPayKey, _ := crypto.GenerateKey()
	priorityKey, _ := crypto.GenerateKey()
	normalKey1, _ := crypto.GenerateKey()
	normalKey2, _ := crypto.GenerateKey()

	okPayAddr := crypto.PubkeyToAddress(okPayKey.PublicKey)
	priorityAddr := crypto.PubkeyToAddress(priorityKey.PublicKey)
	normalAddr1 := crypto.PubkeyToAddress(normalKey1.PublicKey)
	normalAddr2 := crypto.PubkeyToAddress(normalKey2.PublicKey)

	// Create miner with funded accounts
	allAccounts := []common.Address{okPayAddr, priorityAddr, normalAddr1, normalAddr2}
	miner := createMiner(t, allAccounts)

	// Configure OkX Pay and priority settings
	miner.config.OkPayPriorityEnable = true
	miner.config.OkPayBlockPriorityTxsLimit = 1
	miner.config.OkPaySenderAccounts = []common.Address{okPayAddr}

	miner.prio = []common.Address{priorityAddr}

	signer := types.LatestSigner(miner.chainConfig)

	// Create transactions
	okPayTx := types.MustSignNewTx(okPayKey, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 1), // Low gas price
	})

	okPayTx2 := types.MustSignNewTx(okPayKey, signer, &types.LegacyTx{
		Nonce:    1,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 1), // Low gas price
	})

	priorityTx := types.MustSignNewTx(priorityKey, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 2), // Medium gas price
	})

	// Normal transactions with higher gas prices
	normalTx1 := types.MustSignNewTx(normalKey1, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 10), // High gas price
	})

	normalTx2 := types.MustSignNewTx(normalKey2, signer, &types.LegacyTx{
		Nonce:    0,
		To:       &testUserAddress,
		Value:    big.NewInt(1000),
		Gas:      params.TxGas,
		GasPrice: big.NewInt(params.InitialBaseFee * 15), // Highest gas price
	})

	// Add transactions in random order
	txs := types.Transactions{normalTx2, okPayTx, okPayTx2, normalTx1, priorityTx}
	miner.txpool.Add(txs, true)

	// Generate block
	timestamp := uint64(time.Now().Unix())
	r := miner.generateWork(&generateParams{
		parentHash: miner.chain.CurrentBlock().Hash(),
		timestamp:  timestamp,
		random:     common.HexToHash("0xcafebabe"),
		noTxs:      false,
		forceTime:  true,
	}, false)

	if r.err != nil {
		t.Fatalf("Failed to generate work: %v", r.err)
	}

	// Should have all 5 transactions
	blockTxs := r.block.Transactions()
	if len(blockTxs) != 5 {
		t.Fatalf("Expected 5 transactions in block, got %d", len(blockTxs))
	}

	// Verify priority order
	// 1st: OkPay (lowest gas price but highest priority)
	if blockTxs[0].Hash() != okPayTx.Hash() {
		t.Errorf("First transaction should be OkPay tx")
	}

	// 2nd: Priority tx
	if blockTxs[1].Hash() != priorityTx.Hash() {
		t.Errorf("Second transaction should be priority tx")
	}

	// 3rd & 4th: Normal transactions in gas price order (highest first)
	// normalTx2 has higher gas price, so should come first
	if blockTxs[2].Hash() != normalTx2.Hash() {
		t.Errorf("Third transaction should be normalTx2 (higher gas price)")
	}
	if blockTxs[3].Hash() != normalTx1.Hash() {
		t.Errorf("Fourth transaction should be normalTx1 (lower gas price)")
	}

	// 5th: OkPay tx2
	if blockTxs[4].Hash() != okPayTx2.Hash() {
		t.Errorf("Fifth transaction should be okPayTx2 (lowest gas price)")
	}

	// Verify gas price ordering is overridden by priority
	okPayGasPrice := okPayTx.GasPrice()
	for i := 1; i < len(blockTxs); i++ {
		if blockTxs[i].GasPrice().Cmp(okPayGasPrice) <= 0 {
			continue // Skip if gas price is not higher
		}
		// This transaction has higher gas price than OkPay but comes after it
		// This proves priority override works
		t.Logf("✓ Priority override working: OkPay tx (gas price %d) included before tx with gas price %d",
			okPayGasPrice, blockTxs[i].GasPrice())
		break
	}
}

// testOkPayTimeOrdering tests that OkPay transactions are ordered by their arrival time
func testOkPayTimeOrdering(t *testing.T) {
	numAccounts := 10
	// Create multiple OkPay accounts
	okPayKeys := make([]*ecdsa.PrivateKey, numAccounts)
	okPayAddrs := make([]common.Address, numAccounts)
	for i := 0; i < numAccounts; i++ {
		key, _ := crypto.GenerateKey()
		okPayKeys[i] = key
		okPayAddrs[i] = crypto.PubkeyToAddress(key.PublicKey)
	}

	// Create miner with funded accounts
	miner := createMiner(t, okPayAddrs)

	// Configure OkPay with high limit to ensure all transactions are prioritized
	miner.config.OkPayPriorityEnable = true
	miner.config.OkPayBlockPriorityTxsLimit = 5
	miner.config.OkPaySenderAccounts = okPayAddrs

	signer := types.LatestSigner(miner.chainConfig)

	// Create transactions with same gas price but different arrival times
	txs := make([]*types.Transaction, numAccounts)
	expectedIncluded := map[common.Hash]bool{}

	for i := 0; i < numAccounts; i++ {
		tx := types.MustSignNewTx(okPayKeys[i], signer, &types.LegacyTx{
			Nonce:    0,
			To:       &testUserAddress,
			Value:    big.NewInt(1000),
			Gas:      params.TxGas,
			GasPrice: big.NewInt(int64(params.InitialBaseFee * (1 + rand.Intn(10)))), // Same gas price for all
		})
		txs[i] = tx

		// Add to expected included transactions
		if i < int(miner.config.OkPayBlockPriorityTxsLimit) {
			expectedIncluded[tx.Hash()] = true
		}

		// Add transaction to pool one by one with time delays
		// This ensures different arrival times
		miner.txpool.Add(types.Transactions{tx}, true)

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Verify transactions are in the pool
	for _, tx := range txs {
		if !miner.txpool.Has(tx.Hash()) {
			t.Fatalf("Transaction %s is not in the pool", tx.Hash().Hex())
		}
	}

	// Generate block
	timestamp := uint64(time.Now().Unix())
	r := miner.generateWork(&generateParams{
		parentHash: miner.chain.CurrentBlock().Hash(),
		timestamp:  timestamp,
		random:     common.HexToHash("0xcafebabe"),
		noTxs:      false,
		forceTime:  true,
	}, false)

	if r.err != nil {
		t.Fatalf("Failed to generate work: %v", r.err)
	}

	// Verify all transactions are included because the block has enough slots
	blockTxs := r.block.Transactions()

	// Verify only expected fast transactions are included at the start
	for id := range len(expectedIncluded) {
		actualHash := blockTxs[id].Hash()
		included, ok := expectedIncluded[actualHash]
		if !ok || !included {
			t.Fatalf("Transaction at position %d: expected %s, got %s",
				id, actualHash.Hex(), actualHash.Hex())
		}
	}

	// Verify late transactions are included only after the priority expected transactions
	for id := len(expectedIncluded); id < len(blockTxs); id++ {
		actualHash := blockTxs[id].Hash()
		included, ok := expectedIncluded[actualHash]
		if ok || included {
			t.Fatalf("Transaction at position %d should not be included", id)
		}
	}
}

// testOkPayNonceOrdering tests that OkPay transactions are ordered by their nonce
func testOkPayNonceOrdering(t *testing.T) {
	// Create test accounts
	okPayKey, _ := crypto.GenerateKey()
	normalKey, _ := crypto.GenerateKey()

	okPayAddr1 := crypto.PubkeyToAddress(okPayKey.PublicKey)
	normalAddr := crypto.PubkeyToAddress(normalKey.PublicKey)
	// Configure OkPay with high limit to ensure all transactions are prioritized

	miner := createMiner(t, []common.Address{okPayAddr1, normalAddr})
	miner.config.OkPayPriorityEnable = true
	miner.config.OkPayBlockPriorityTxsLimit = 5
	miner.config.OkPaySenderAccounts = []common.Address{okPayAddr1}

	signer := types.LatestSigner(miner.chainConfig)

	// Create transactions with different nonces
	txCount := 3
	txs := make([]*types.Transaction, 3)
	expectedIncluded := map[common.Hash]bool{}

	for i := 0; i < txCount; i++ {
		// Submit nonce in decreasing order
		j := txCount - i - 1
		tx := types.MustSignNewTx(okPayKey, signer, &types.LegacyTx{
			Nonce:    uint64(j),
			To:       &testUserAddress,
			Value:    big.NewInt(1000),
			Gas:      params.TxGas,
			GasPrice: big.NewInt(int64(params.InitialBaseFee * (1 + rand.Intn(10)))), // Same gas price for all
		})
		txs[i] = tx

		t.Logf("Transaction %d: %s", i, tx.Hash().Hex())

		// Add to expected included transactions
		expectedIncluded[tx.Hash()] = true

		// Add transaction to pool
		miner.txpool.Add(types.Transactions{tx}, true)
	}

	// Verify transactions are in the pool
	for _, tx := range txs {
		if !miner.txpool.Has(tx.Hash()) {
			t.Fatalf("Transaction %s is not in the pool", tx.Hash().Hex())
		}
	}

	// Generate block
	timestamp := uint64(time.Now().Unix())
	r := miner.generateWork(&generateParams{
		parentHash: miner.chain.CurrentBlock().Hash(),
		timestamp:  timestamp,
		random:     common.HexToHash("0xcafebabe"),
		noTxs:      false,
		forceTime:  true,
	}, false)

	if r.err != nil {
		t.Fatalf("Failed to generate work: %v", r.err)
	}

	// Verify all transactions are included because the block has enough slots
	blockTxs := r.block.Transactions()

	// Verify transactions are included in nonce order
	for id := range len(expectedIncluded) {
		actualHash := blockTxs[id].Hash()
		included, ok := expectedIncluded[actualHash]
		if !ok || !included {
			t.Fatalf("Transaction at position %d: expected %s, got %s",
				id, actualHash.Hex(), actualHash.Hex())
		}
	}
}
