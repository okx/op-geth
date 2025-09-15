package core

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"unsafe"

	"github.com/ledgerwatch/erigon-lib/kv/iter"
	"github.com/ledgerwatch/erigon-lib/kv/order"
	"github.com/stretchr/testify/assert"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/stretchr/testify/mock"
)

type MockRoDB struct {
	mock.Mock
}

func (m *MockRoDB) ReadOnly() bool {
	panic("unreachable")
}

func (m *MockRoDB) BeginRo(ctx context.Context) (kv.Tx, error) {
	panic("unreachable")
}

func (m *MockRoDB) AllTables() kv.TableCfg {
	panic("unreachable")
}

func (m *MockRoDB) PageSize() uint64 {
	panic("unreachable")
}

func (m *MockRoDB) CHandle() unsafe.Pointer {
	panic("unreachable")
}

func (m *MockRoDB) View(ctx context.Context, f func(tx kv.Tx) error) error {
	args := m.Called(ctx, f)
	return args.Error(0)
}

func (m *MockRoDB) Close() {
	m.Called()
}

type MockTx struct {
	mock.Mock
}

func (m *MockTx) Cursor(table string) (kv.Cursor, error) {
	panic("unreachable")
}

func (m *MockTx) CursorDupSort(table string) (kv.CursorDupSort, error) {
	panic("unreachable")
}

func (m *MockTx) DBSize() (uint64, error) {
	panic("unreachable")
}

func (m *MockTx) Range(table string, fromPrefix, toPrefix []byte) (iter.KV, error) {
	panic("unreachable")
}

func (m *MockTx) RangeAscend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	panic("unreachable")
}

func (m *MockTx) RangeDescend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	panic("unreachable")
}

func (m *MockTx) Prefix(table string, prefix []byte) (iter.KV, error) {
	panic("unreachable")
}

func (m *MockTx) RangeDupSort(table string, key []byte, fromPrefix, toPrefix []byte, asc order.By, limit int) (iter.KV, error) {
	panic("unreachable")
}

func (m *MockTx) CHandle() unsafe.Pointer {
	panic("unreachable")
}

func (m *MockTx) BucketSize(table string) (uint64, error) {
	panic("unreachable")
}

func (m *MockTx) ForEach(bucket string, start []byte, f func(k, v []byte) error) error {
	args := m.Called(bucket, start, f)
	return args.Error(0)
}

func (m *MockTx) GetOne(bucket string, key []byte) ([]byte, error) {
	return []byte{1, 2, 3, 4}, nil
}

func (m *MockTx) Has(table string, key []byte) (bool, error) {
	args := m.Called(table, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockTx) ForPrefix(table string, prefix []byte, walker func(k []byte, v []byte) error) error {
	args := m.Called(table, prefix, walker)
	return args.Error(0)
}

func (m *MockTx) ForAmount(table string, prefix []byte, amount uint32, walker func(k []byte, v []byte) error) error {
	args := m.Called(table, prefix, amount, walker)
	return args.Error(0)
}

// Add the missing methods
func (m *MockTx) ReadSequence(table string) (uint64, error) {
	args := m.Called(table)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockTx) ListBuckets() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockTx) ViewID() uint64 {
	args := m.Called()
	return args.Get(0).(uint64)
}

func (m *MockTx) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTx) Rollback() {
	m.Called()
}

// TestScanDB tests the ScanDB function with various scenarios
func TestScanDB(t *testing.T) {
	tests := []struct {
		name           string
		migrationPath  string
		setupMocks     func(*MockRoDB, *MockTx)
		expectedResult types.GenesisAlloc
		expectedError  string
	}{
		{
			name:          "successful scan with accounts and storage",
			migrationPath: "/tmp/data",
			setupMocks: func(db *MockRoDB, tx *MockTx) {
				// Mock db.View
				db.On("View", mock.Anything, mock.AnythingOfType("func(kv.Tx) error")).Run(func(args mock.Arguments) {
					fn := args.Get(1).(func(kv.Tx) error)
					fn(tx)
				}).Return(nil)

				// Fix the ForEach mock - use mock.MatchedBy for the function
				tx.On("ForEach", PlainStateBucket, mock.MatchedBy(func(start []byte) bool {
					// Accept both nil and empty slice
					return start == nil || len(start) == 0
				}), mock.MatchedBy(func(fn func(k, v []byte) error) bool {
					// Simulate account data (20-byte key)
					addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
					accountData := createMockAccountData(big.NewInt(1000000000000000000), []byte{1, 2, 3, 4}, 5)
					fn(addr.Bytes(), accountData)

					// Simulate storage data (60-byte key: 20-byte addr + 8-bytes incarnation + 32-byte storage key)
					incarnation := []byte{0, 0, 0, 0, 0, 0, 0, 1}
					storageKey := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
					storageValue := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
					storageKeyBytes := append(addr.Bytes(), incarnation...)
					storageKeyBytes = append(storageKeyBytes, storageKey.Bytes()...)
					fn(storageKeyBytes, storageValue.Bytes())

					return true
				})).Return(nil)

			},
			expectedResult: types.GenesisAlloc{
				common.HexToAddress("0x1234567890123456789012345678901234567890"): {
					Balance: big.NewInt(1000000000000000000),
					Code:    []byte{1, 2, 3, 4},
					Nonce:   5,
					Storage: map[common.Hash]common.Hash{
						common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"): common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
					},
				},
			},
		},
		{
			name:          "view transaction error",
			migrationPath: "/tmp/data",
			setupMocks: func(db *MockRoDB, tx *MockTx) {
				db.On("View", mock.Anything, mock.AnythingOfType("func(kv.Tx) error")).Return(errors.New("view failed"))
			},
			expectedError: "failed to scan migration database: view failed",
		},
		{
			name:          "empty database",
			migrationPath: "/tmp/data",
			setupMocks: func(db *MockRoDB, tx *MockTx) {
				db.On("View", mock.Anything, mock.AnythingOfType("func(kv.Tx) error")).Run(func(args mock.Arguments) {
					fn := args.Get(1).(func(kv.Tx) error)
					fn(tx)
				}).Return(nil)
				tx.On("ForEach", PlainStateBucket, mock.MatchedBy(func(start []byte) bool {
					return start == nil || len(start) == 0
				}), mock.MatchedBy(func(fn func(k, v []byte) error) bool {
					// Empty database - no data to process
					return true
				})).Return(nil)
			},
			expectedResult: types.GenesisAlloc{},
		},
		{
			name:          "account with empty code",
			migrationPath: "/tmp/data",
			setupMocks: func(db *MockRoDB, tx *MockTx) {
				db.On("View", mock.Anything, mock.AnythingOfType("func(kv.Tx) error")).Run(func(args mock.Arguments) {
					fn := args.Get(1).(func(kv.Tx) error)
					fn(tx)
				}).Return(nil)

				tx.On("ForEach", PlainStateBucket, mock.MatchedBy(func(start []byte) bool {
					return start == nil || len(start) == 0
				}), mock.MatchedBy(func(fn func(k, v []byte) error) bool {
					addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
					// Account with empty code
					accountData := createMockAccountData(big.NewInt(1000000000000000000), []byte{}, 0)
					fn(addr.Bytes(), accountData)
					return true
				})).Return(nil)
			},
			expectedResult: types.GenesisAlloc{
				common.HexToAddress("0x1234567890123456789012345678901234567890"): {
					Balance: big.NewInt(1000000000000000000),
					Code:    nil,
					Nonce:   0,
					Storage: make(map[common.Hash]common.Hash),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &MockRoDB{}
			mockTx := &MockTx{}

			tt.setupMocks(mockDB, mockTx)

			result, err := ScanDB(mockDB)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockDB.AssertExpectations(t)
			mockTx.AssertExpectations(t)
		})
	}
}

// Helper function to create mock account data in CBOR format
func createMockAccountData(balance *big.Int, code []byte, nonce uint64) []byte {
	var result []byte

	// Calculate field set (bit flags for which fields are present)
	var fieldSet byte = 0

	// Field 1: Nonce (bit 0)
	if nonce > 0 {
		fieldSet |= 1
	}

	// Field 2: Balance (bit 1)
	if balance != nil && balance.Sign() > 0 {
		fieldSet |= 2
	}

	// Field 4: CodeHash (bit 3) - we'll set this if code is provided
	if len(code) > 0 {
		fieldSet |= 8
	}

	// Start with field set byte
	result = append(result, fieldSet)

	// Field 1: Nonce
	if fieldSet&1 > 0 {
		nonceBytes := uint64ToBytes(nonce)
		result = append(result, byte(len(nonceBytes))) // length
		result = append(result, nonceBytes...)         // data
	}

	// Field 2: Balance
	if fieldSet&2 > 0 {
		balanceBytes := balance.Bytes()
		result = append(result, byte(len(balanceBytes))) // length
		result = append(result, balanceBytes...)         // data
	}

	// Field 4: CodeHash
	if fieldSet&8 > 0 {
		// For testing, we'll use a simple hash of the code
		codeHash := crypto.Keccak256Hash(code)
		result = append(result, 32)             // length (always 32 for hash)
		result = append(result, codeHash[:]...) // data
	}

	return result
}

// Helper function to convert uint64 to bytes (big-endian)
func uint64ToBytes(value uint64) []byte {
	result := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		result[7-i] = byte(value >> (8 * i))
	}

	// Remove leading zeros
	for len(result) > 1 && result[0] == 0 {
		result = result[1:]
	}

	return result
}
