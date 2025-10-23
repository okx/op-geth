package types

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestGenesisAlloc_DeepCopy(t *testing.T) {
	// Create test data
	addr1 := common.HexToAddress("0x1234567890123456789012345678901234567890")
	addr2 := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")

	original := GenesisAlloc{
		addr1: Account{
			Code: []byte{0x01, 0x02, 0x03, 0x04},
			Storage: map[common.Hash]common.Hash{
				common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"): common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"),
				common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"): common.HexToHash("0x4444444444444444444444444444444444444444444444444444444444444444"),
			},
			Balance:    big.NewInt(1000000000000000000), // 1 ETH
			Nonce:      42,
			PrivateKey: []byte{0xaa, 0xbb, 0xcc, 0xdd},
		},
		addr2: Account{
			Code: []byte{0x05, 0x06, 0x07, 0x08, 0x09},
			Storage: map[common.Hash]common.Hash{
				common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555"): common.HexToHash("0x6666666666666666666666666666666666666666666666666666666666666666"),
			},
			Balance:    big.NewInt(2000000000000000000), // 2 ETH
			Nonce:      84,
			PrivateKey: []byte{0xee, 0xff},
		},
	}

	// Test case 1: Normal deep copy
	t.Run("NormalDeepCopy", func(t *testing.T) {
		copied := original.DeepCopy()

		// Verify it's not the same pointer
		if copied == &original {
			t.Error("DeepCopy returned the same pointer")
		}

		// Verify all addresses are present
		if len(*copied) != len(original) {
			t.Errorf("Expected %d accounts, got %d", len(original), len(*copied))
		}

		// Verify each account is deeply copied
		for addr, originalAccount := range original {
			copiedAccount, exists := (*copied)[addr]
			if !exists {
				t.Errorf("Address %v not found in copied alloc", addr)
				continue
			}

			verifyAccountDeepCopy(t, addr, originalAccount, copiedAccount)
		}
	})

	// Test case 2: Empty alloc
	t.Run("EmptyAlloc", func(t *testing.T) {
		empty := GenesisAlloc{}
		copied := empty.DeepCopy()

		if copied == &empty {
			t.Error("DeepCopy returned the same pointer for empty alloc")
		}

		if len(*copied) != 0 {
			t.Errorf("Expected empty copied alloc, got %d accounts", len(*copied))
		}
	})

	// Test case 3: Nil fields
	t.Run("NilFields", func(t *testing.T) {
		nilFields := GenesisAlloc{
			addr1: Account{
				Code:       nil,
				Storage:    nil,
				Balance:    nil,
				Nonce:      0,
				PrivateKey: nil,
			},
		}

		copied := nilFields.DeepCopy()
		copiedAccount := (*copied)[addr1]

		if copiedAccount.Code != nil {
			t.Error("Expected nil Code, got non-nil")
		}
		if copiedAccount.Storage != nil {
			t.Error("Expected nil Storage, got non-nil")
		}
		if copiedAccount.Balance != nil {
			t.Error("Expected nil Balance, got non-nil")
		}
		if copiedAccount.PrivateKey != nil {
			t.Error("Expected nil PrivateKey, got non-nil")
		}
		if copiedAccount.Nonce != 0 {
			t.Error("Expected Nonce 0, got", copiedAccount.Nonce)
		}
	})

	// Test case 4: Modification isolation
	t.Run("ModificationIsolation", func(t *testing.T) {
		copied := original.DeepCopy()

		// Modify original
		original[addr1].Balance.Add(original[addr1].Balance, big.NewInt(1000))
		original[addr1].Code[0] = 0xff
		original[addr1].Storage[common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")] = common.HexToHash("0x9999999999999999999999999999999999999999999999999999999999999999")

		// Verify copied is unchanged
		copiedAccount := (*copied)[addr1]
		if copiedAccount.Balance.Cmp(big.NewInt(1000000000000000000)) != 0 {
			t.Error("Copied balance was modified when original was changed")
		}
		if copiedAccount.Code[0] != 0x01 {
			t.Error("Copied code was modified when original was changed")
		}
		if copiedAccount.Storage[common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")] != common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222") {
			t.Error("Copied storage was modified when original was changed")
		}
		if copiedAccount.Nonce != 42 {
			t.Error("Copied nonce was modified when original was changed")
		}
	})

	// Test case 5: Large storage map
	t.Run("LargeStorageMap", func(t *testing.T) {
		largeStorage := make(map[common.Hash]common.Hash)
		for i := 0; i < 1000; i++ {
			key := common.BigToHash(big.NewInt(int64(i)))
			value := common.BigToHash(big.NewInt(int64(i * 2)))
			largeStorage[key] = value
		}

		largeAlloc := GenesisAlloc{
			addr1: Account{
				Storage: largeStorage,
				Balance: big.NewInt(5000000000000000000),
				Nonce:   100,
			},
		}

		copied := largeAlloc.DeepCopy()
		copiedAccount := (*copied)[addr1]

		// Verify all storage entries are copied
		if len(copiedAccount.Storage) != len(largeStorage) {
			t.Errorf("Expected %d storage entries, got %d", len(largeStorage), len(copiedAccount.Storage))
		}

		for key, expectedValue := range largeStorage {
			if actualValue, exists := copiedAccount.Storage[key]; !exists {
				t.Errorf("Storage key %v not found in copied account", key)
			} else if actualValue != expectedValue {
				t.Errorf("Storage value mismatch for key %v: expected %v, got %v", key, expectedValue, actualValue)
			}
		}
	})
}

func verifyAccountDeepCopy(t *testing.T, addr common.Address, original, copied Account) {
	// Verify Code slice
	if len(original.Code) != len(copied.Code) {
		t.Errorf("Address %v: Code length mismatch: expected %d, got %d", addr, len(original.Code), len(copied.Code))
	}
	for i, b := range original.Code {
		if copied.Code[i] != b {
			t.Errorf("Address %v: Code byte %d mismatch: expected %d, got %d", addr, i, b, copied.Code[i])
		}
	}

	// Verify Code slice is not the same underlying array
	if len(original.Code) > 0 && &original.Code[0] == &copied.Code[0] {
		t.Errorf("Address %v: Code slice shares underlying array", addr)
	}

	// Verify Storage map
	if len(original.Storage) != len(copied.Storage) {
		t.Errorf("Address %v: Storage length mismatch: expected %d, got %d", addr, len(original.Storage), len(copied.Storage))
	}
	for key, expectedValue := range original.Storage {
		if actualValue, exists := copied.Storage[key]; !exists {
			t.Errorf("Address %v: Storage key %v not found in copied account", addr, key)
		} else if actualValue != expectedValue {
			t.Errorf("Address %v: Storage value mismatch for key %v: expected %v, got %v", addr, key, expectedValue, actualValue)
		}
	}

	// Verify Storage map is not the same map
	if len(original.Storage) > 0 && &original.Storage == &copied.Storage {
		t.Errorf("Address %v: Storage map is the same reference", addr)
	}

	// Verify Balance
	if original.Balance == nil && copied.Balance != nil {
		t.Errorf("Address %v: Expected nil Balance, got non-nil", addr)
	} else if original.Balance != nil && copied.Balance == nil {
		t.Errorf("Address %v: Expected non-nil Balance, got nil", addr)
	} else if original.Balance != nil && copied.Balance != nil {
		if original.Balance.Cmp(copied.Balance) != 0 {
			t.Errorf("Address %v: Balance mismatch: expected %v, got %v", addr, original.Balance, copied.Balance)
		}
		// Verify Balance is not the same big.Int
		if original.Balance == copied.Balance {
			t.Errorf("Address %v: Balance shares the same big.Int reference", addr)
		}
	}

	// Verify Nonce
	if original.Nonce != copied.Nonce {
		t.Errorf("Address %v: Nonce mismatch: expected %d, got %d", addr, original.Nonce, copied.Nonce)
	}

}

// Benchmark tests
func BenchmarkGenesisAlloc_DeepCopy(b *testing.B) {
	// Create a realistic test alloc with multiple accounts
	alloc := GenesisAlloc{}

	// Add 100 accounts with various data sizes
	for i := 0; i < 100; i++ {
		addr := common.BigToAddress(big.NewInt(int64(i)))

		// Create storage map with varying sizes
		storage := make(map[common.Hash]common.Hash)
		storageSize := i % 50 // 0-49 storage entries
		for j := 0; j < storageSize; j++ {
			key := common.BigToHash(big.NewInt(int64(j)))
			value := common.BigToHash(big.NewInt(int64(j * 2)))
			storage[key] = value
		}

		alloc[addr] = Account{
			Code:       make([]byte, i%100), // 0-99 bytes
			Storage:    storage,
			Balance:    big.NewInt(int64(i * 1000000000000000000)), // i ETH
			Nonce:      uint64(i),
			PrivateKey: make([]byte, i%20), // 0-19 bytes
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = alloc.DeepCopy()
	}
}
