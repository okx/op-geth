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

package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

func TestGaslessAddressFor(t *testing.T) {
	cases := []struct {
		name    string
		chainID *big.Int
		want    common.Address
	}{
		{"nil_chain_id", nil, defaultGaslessAddr},
		{"default_chain", big.NewInt(1), defaultGaslessAddr},
		{"chain_196", big.NewInt(196), gaslessAddrChain196},
		{"chain_1952", big.NewInt(1952), gaslessAddrChain1952},
		{"unknown_chain", big.NewInt(999), defaultGaslessAddr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GaslessAddressFor(tc.chainID); got != tc.want {
				t.Fatalf("GaslessAddressFor(%v) = %s, want %s", tc.chainID, got.Hex(), tc.want.Hex())
			}
		})
	}
}

func TestEncodeGetGaslessAllowanceCalldata(t *testing.T) {
	to := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	prefix := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02}
	got := EncodeGetGaslessAllowanceCalldata(to, prefix)

	// Selector check.
	if !bytes.Equal(got[:4], getGaslessAllowanceSelector) {
		t.Fatalf("selector mismatch: got %x want %x", got[:4], getGaslessAllowanceSelector)
	}
	// Address is left-padded to 32 bytes.
	wantAddr := make([]byte, 32)
	copy(wantAddr[12:], to.Bytes())
	if !bytes.Equal(got[4:36], wantAddr) {
		t.Fatalf("address mismatch: got %x want %x", got[4:36], wantAddr)
	}
	// Offset to bytes payload is 0x40 (right-aligned in the 32-byte slot).
	if got[4+32+31] != 0x40 {
		t.Fatalf("offset byte = 0x%x, want 0x40", got[4+32+31])
	}
	for i := 4 + 32; i < 4+32+31; i++ {
		if got[i] != 0 {
			t.Fatalf("offset slot byte %d = 0x%x, want 0", i, got[i])
		}
	}
	// Length is right-aligned uint64 in the next 32 bytes.
	if l := binary.BigEndian.Uint64(got[4+32+32+24 : 4+32+32+32]); l != uint64(len(prefix)) {
		t.Fatalf("length = %d, want %d", l, len(prefix))
	}
	// Data appears immediately after, padded to 32 bytes.
	payload := got[4+32+32+32:]
	if !bytes.Equal(payload[:len(prefix)], prefix) {
		t.Fatalf("data mismatch: got %x want %x", payload[:len(prefix)], prefix)
	}
	if len(payload) != 32 {
		t.Fatalf("padded payload length = %d, want 32", len(payload))
	}
}

func TestEncodeGetGaslessAllowanceCalldataEmptyPrefix(t *testing.T) {
	to := common.Address{}
	got := EncodeGetGaslessAllowanceCalldata(to, nil)
	// 4 selector + 32 addr + 32 offset + 32 length + 0 data padding
	if got, want := len(got), 4+32+32+32; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	if l := binary.BigEndian.Uint64(got[4+32+32+24 : 4+32+32+32]); l != 0 {
		t.Fatalf("length = %d, want 0", l)
	}
}

func TestDecodeGaslessAllowance(t *testing.T) {
	build := func(allowed bool, limit uint64) []byte {
		out := make([]byte, 64)
		if allowed {
			out[31] = 1
		}
		binary.BigEndian.PutUint64(out[32+24:64], limit)
		return out
	}
	// buildRaw lets us craft an arbitrary 32-byte bool word (boolWord) together
	// with a normally-encoded uint64 gasLimit word, so we can lock in the strict
	// canonical bool decoding for non-canonical inputs.
	buildRaw := func(boolWord [32]byte, limit uint64) []byte {
		out := make([]byte, 64)
		copy(out[:32], boolWord[:])
		binary.BigEndian.PutUint64(out[32+24:64], limit)
		return out
	}
	// canonical true word: all-zero except byte[31] == 1.
	canonicalTrue := [32]byte{}
	canonicalTrue[31] = 1
	// non-canonical truthy: byte[31] == 2.
	nonCanon2 := [32]byte{}
	nonCanon2[31] = 2
	// non-canonical truthy: byte[31] == 0xff.
	nonCanonFF := [32]byte{}
	nonCanonFF[31] = 0xff
	// high byte set, last byte zero.
	highByteSet := [32]byte{}
	highByteSet[0] = 1
	// extra high bit alongside a canonical-looking last byte.
	extraHighBit := [32]byte{}
	extraHighBit[30] = 1
	extraHighBit[31] = 1
	// canonical-looking last byte but a dirty interior byte.
	dirtyInterior := [32]byte{}
	dirtyInterior[15] = 0x80
	dirtyInterior[31] = 1
	cases := []struct {
		name      string
		in        []byte
		wantOK    bool
		want      GaslessAllowance
		wantError bool
	}{
		{"allowed_with_limit", build(true, 500_000), true, GaslessAllowance{Allowed: true, GasLimit: 500_000}, false},
		{"disallowed_zero_limit", build(false, 0), true, GaslessAllowance{Allowed: false, GasLimit: 0}, false},
		{"allowed_zero_limit", build(true, 0), true, GaslessAllowance{Allowed: true, GasLimit: 0}, false},
		{"too_short", make([]byte, 32), false, GaslessAllowance{}, true},

		// --- strict canonical bool decoding (fail-closed) ---
		// Canonical true via raw word, with a nonzero gasLimit, decodes true.
		{"canonical_true_raw", buildRaw(canonicalTrue, 123_456), true, GaslessAllowance{Allowed: true, GasLimit: 123_456}, false},
		// All-zero word → false. gasLimit still parsed independently.
		{"all_zero_raw", buildRaw([32]byte{}, 777), true, GaslessAllowance{Allowed: false, GasLimit: 777}, false},
		// byte[31] == 2 is non-canonical truthy → MUST be false; gasLimit still decoded.
		{"noncanon_last_byte_2", buildRaw(nonCanon2, 250_000), true, GaslessAllowance{Allowed: false, GasLimit: 250_000}, false},
		// byte[31] == 0xff → MUST be false.
		{"noncanon_last_byte_ff", buildRaw(nonCanonFF, 0), true, GaslessAllowance{Allowed: false, GasLimit: 0}, false},
		// high byte set, last byte zero → false.
		{"high_byte_set_last_zero", buildRaw(highByteSet, 0), true, GaslessAllowance{Allowed: false, GasLimit: 0}, false},
		// extra high bit alongside canonical last byte → false.
		{"extra_high_bit_with_canonical_last", buildRaw(extraHighBit, 0), true, GaslessAllowance{Allowed: false, GasLimit: 0}, false},
		// canonical last byte but dirty interior byte → false. gasLimit still decoded.
		{"canonical_last_dirty_interior", buildRaw(dirtyInterior, 999_999), true, GaslessAllowance{Allowed: false, GasLimit: 999_999}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeGaslessAllowance(tc.in)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestGaslessDataPrefix(t *testing.T) {
	short := []byte{1, 2, 3}
	long := make([]byte, 1024) // intentionally exceeds the previous 128-byte cap
	for i := range long {
		long[i] = byte(i)
	}
	cases := []struct {
		name string
		data []byte
		want []byte
	}{
		{"empty", nil, []byte{}},
		{"short", short, short},
		{"long_returned_in_full", long, long},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := NewTx(&DynamicFeeTx{
				Nonce: 0,
				To:    &common.Address{},
				Data:  tc.data,
			})
			got := GaslessDataPrefix(tx)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			if len(got) > 0 && !bytes.Equal(got, tc.want) {
				t.Fatalf("data mismatch:\n got  %x\n want %x", got, tc.want)
			}
		})
	}
}

func newDynFeeTx(t *testing.T, to *common.Address, tip, feeCap *big.Int, gas uint64) *Transaction {
	t.Helper()
	tx := NewTx(&DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     7,
		To:        to,
		Gas:       gas,
		GasFeeCap: feeCap,
		GasTipCap: tip,
	})
	return tx
}

func TestIsGaslessTxFor(t *testing.T) {
	addr := common.HexToAddress("0xabcdef0000000000000000000000000000001234")

	allowed := func(GasLimit uint64) GaslessChecker {
		return func(*Transaction) (GaslessAllowance, error) {
			return GaslessAllowance{Allowed: true, GasLimit: GasLimit}, nil
		}
	}
	denied := func(*Transaction) (GaslessAllowance, error) {
		return GaslessAllowance{Allowed: false}, nil
	}
	erroring := func(*Transaction) (GaslessAllowance, error) {
		return GaslessAllowance{}, errors.New("contract revert")
	}

	t.Run("nil_checker", func(t *testing.T) {
		tx := newDynFeeTx(t, &addr, big.NewInt(0), big.NewInt(0), 50_000)
		if IsGaslessTxFor(tx, nil) {
			t.Fatal("expected false for nil checker")
		}
	})

	t.Run("non_zero_tip_rejected", func(t *testing.T) {
		tx := newDynFeeTx(t, &addr, big.NewInt(1), big.NewInt(2), 50_000)
		if IsGaslessTxFor(tx, allowed(0)) {
			t.Fatal("expected false for non-zero tip")
		}
	})

	t.Run("non_zero_feecap_rejected", func(t *testing.T) {
		tx := newDynFeeTx(t, &addr, big.NewInt(0), big.NewInt(2), 50_000)
		if IsGaslessTxFor(tx, allowed(0)) {
			t.Fatal("expected false for non-zero feecap")
		}
	})

	t.Run("nil_to_rejected", func(t *testing.T) {
		tx := newDynFeeTx(t, nil, big.NewInt(0), big.NewInt(0), 50_000)
		if IsGaslessTxFor(tx, allowed(0)) {
			t.Fatal("expected false for contract creation (nil to)")
		}
	})

	t.Run("checker_denied", func(t *testing.T) {
		tx := newDynFeeTx(t, &addr, big.NewInt(0), big.NewInt(0), 50_000)
		if IsGaslessTxFor(tx, denied) {
			t.Fatal("expected false when contract denies")
		}
	})

	t.Run("checker_error", func(t *testing.T) {
		tx := newDynFeeTx(t, &addr, big.NewInt(0), big.NewInt(0), 50_000)
		if IsGaslessTxFor(tx, erroring) {
			t.Fatal("expected false when checker errors")
		}
	})

	t.Run("over_gas_limit_rejected", func(t *testing.T) {
		tx := newDynFeeTx(t, &addr, big.NewInt(0), big.NewInt(0), 200_000)
		if IsGaslessTxFor(tx, allowed(100_000)) {
			t.Fatal("expected false when tx gas exceeds allowance.GasLimit")
		}
	})

	t.Run("at_limit_accepted", func(t *testing.T) {
		tx := newDynFeeTx(t, &addr, big.NewInt(0), big.NewInt(0), 100_000)
		if !IsGaslessTxFor(tx, allowed(100_000)) {
			t.Fatal("expected true at exact gas limit")
		}
	})

	t.Run("legacy_zero_price_rejected", func(t *testing.T) {
		tx := NewTx(&LegacyTx{
			Nonce:    0,
			To:       &addr,
			Gas:      50_000,
			GasPrice: big.NewInt(1), // gasPrice != 0
		})
		if IsGaslessTxFor(tx, allowed(0)) {
			t.Fatal("expected false when gasPrice != 0")
		}
	})

	// Deposit txs must NEVER be classified gasless, even when all fees are zero
	// and the checker reports allowed=true: the type check in IsGaslessTxFor
	// excludes them. Other types (e.g. EIP-7702 SetCode) are not type-excluded.
	t.Run("deposit_tx_never_gasless", func(t *testing.T) {
		tx := NewTx(&DepositTx{
			To:    &addr,
			Value: big.NewInt(0),
			Gas:   50_000,
			// Deposit txs carry no fee fields; nothing but the type excludes it.
		})
		if IsGaslessTxFor(tx, allowed(1_000_000)) {
			t.Fatal("expected false: deposit tx must never be gasless")
		}
	})

	t.Run("setcode_tx_can_be_gasless", func(t *testing.T) {
		// SetCode is not type-excluded (parity with reth); auth-list
		// well-formedness is checked later in preCheck, not here.
		tx := NewTx(&SetCodeTx{
			ChainID:   uint256.NewInt(1),
			Nonce:     0,
			To:        addr,
			Gas:       50_000,
			GasTipCap: uint256.NewInt(0),
			GasFeeCap: uint256.NewInt(0),
			Value:     uint256.NewInt(0),
			AuthList: []SetCodeAuthorization{{
				ChainID: *uint256.NewInt(1),
				Address: addr,
			}},
		})
		if !IsGaslessTxFor(tx, allowed(1_000_000)) {
			t.Fatal("expected true: zero-fee whitelisted SetCodeTx should be gasless")
		}
	})
}
