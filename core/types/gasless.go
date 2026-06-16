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
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Default Gasless predeploy address. Chain-specific overrides live in
// GaslessAddressFor.
var (
	defaultGaslessAddr   = common.HexToAddress("0x70CA900387FCD29C2A71d511F10E5c961dc9363F")
	gaslessAddrChain196  = common.HexToAddress("0x19787404b0c70021b4752028f7e3a92313885B27")
	gaslessAddrChain1952 = common.HexToAddress("0x19787404b0c70021b4752028f7e3a92313885B27")
)

// GaslessAddressFor returns the predeploy address of the Gasless contract for
// the given chain id. The mapping is:
//   - chain 196   → 0x19787404b0c70021b4752028f7e3a92313885B27
//   - chain 1952  → 0x19787404b0c70021b4752028f7e3a92313885B27
//   - otherwise   → 0x4200000000000000000000000000000000000700
func GaslessAddressFor(chainID *big.Int) common.Address {
	if chainID == nil {
		return defaultGaslessAddr
	}
	switch chainID.Int64() {
	case 196:
		return gaslessAddrChain196
	case 1952:
		return gaslessAddrChain1952
	default:
		return defaultGaslessAddr
	}
}

// Pre-computed selector for
//
//	function getGaslessAllowance(address to, bytes calldata dataPrefix)
//	    external view returns (bool allowed, uint64 gasLimit);
var getGaslessAllowanceSelector = crypto.Keccak256([]byte("getGaslessAllowance(address,bytes)"))[:4]

// EncodeGetGaslessAllowanceCalldata encodes the ABI calldata for the
// getGaslessAllowance(address,bytes) view function.
func EncodeGetGaslessAllowanceCalldata(to common.Address, dataPrefix []byte) []byte {
	// Layout for (address, bytes):
	//   [0..4)         selector
	//   [4..36)        address (left-padded to 32 bytes)
	//   [36..68)       offset to bytes payload (= 0x40)
	//   [68..100)      bytes length
	//   [100..)        bytes data (padded to 32-byte boundary)
	dataLen := len(dataPrefix)
	padded := (dataLen + 31) &^ 31
	out := make([]byte, 4+32+32+32+padded)
	copy(out[0:4], getGaslessAllowanceSelector)
	copy(out[4+12:4+32], to.Bytes())
	out[4+32+31] = 0x40 // offset
	binary.BigEndian.PutUint64(out[4+32+32+24:4+32+32+32], uint64(dataLen))
	copy(out[4+32+32+32:], dataPrefix)
	return out
}

// GaslessAllowance is the decoded return of getGaslessAllowance.
type GaslessAllowance struct {
	Allowed  bool
	GasLimit uint64
}

// DecodeGaslessAllowance parses (bool, uint64) ABI return data. Layout:
//
//	[0..32)   bool   — non-zero last byte means true
//	[32..64)  uint64 — right-aligned in 32 bytes
func DecodeGaslessAllowance(ret []byte) (GaslessAllowance, error) {
	if len(ret) < 64 {
		return GaslessAllowance{}, errors.New("gasless: allowance return too short")
	}
	var out GaslessAllowance
	for _, b := range ret[:32] {
		if b != 0 {
			out.Allowed = true
			break
		}
	}
	out.GasLimit = binary.BigEndian.Uint64(ret[32+24 : 32+32])
	return out, nil
}

// GaslessChecker is bound to a specific EVM and chain. It returns the
// predeploy's allowance answer for the given transaction.
type GaslessChecker func(tx *Transaction) (GaslessAllowance, error)

// GaslessDataPrefix returns tx.Data() as-is — the argument we feed into the
// predeploy as `dataPrefix`. There is no length cap; the predeploy is expected
// to inspect only the prefix it cares about.
func GaslessDataPrefix(tx *Transaction) []byte {
	return tx.Data()
}

// MaybeGaslessShape reports whether tx has the structural shape of a gasless
// transaction — everything that can be decided without invoking the predeploy:
//   - it is an ordinary call/value tx (legacy, access-list or dynamic-fee),
//   - its gasPrice, gasFeeCap and gasTipCap are all zero,
//   - its `to` is non-nil.
//
// It is a cheap pre-filter so fee-based callers (txpool admission/eviction) can
// skip the predeploy probe — and any head-state fetch behind the checker — for
// the overwhelming majority of (fee-paying) transactions, which can never be
// gasless. Only ordinary call/value transaction types are eligible: special
// types carry type-specific consensus validity rules that the gasless
// fee-exemption path in state_transition deliberately skips (deposits are
// system txs, blob txs have blob-gas accounting, and EIP-7702 SetCode txs
// require a non-empty, well-formed authorization list). Gating gasless to the
// plain types via an allowlist also keeps any future tx type excluded by
// default.
func MaybeGaslessShape(tx *Transaction) bool {
	switch tx.Type() {
	case LegacyTxType, AccessListTxType, DynamicFeeTxType:
		// eligible
	default:
		return false
	}
	if tx.inner.gasPrice().Sign() != 0 || tx.inner.gasFeeCap().Sign() != 0 || tx.inner.gasTipCap().Sign() != 0 {
		return false
	}
	return tx.To() != nil
}

// IsGaslessTxFor reports whether the given transaction qualifies for gasless
// execution. A tx is gasless iff:
//   - it has the gasless shape (see MaybeGaslessShape), and
//   - the predeploy's getGaslessAllowance(to, dataPrefix) returns allowed=true
//     and the tx's gas limit does not exceed the returned gasLimit.
//
// The decision is delegated to the supplied checker so callers can re-use a
// single EVM across many transactions.
func IsGaslessTxFor(tx *Transaction, checker GaslessChecker) bool {
	if checker == nil {
		return false
	}
	if !MaybeGaslessShape(tx) {
		return false
	}
	allowance, err := checker(tx)
	if err != nil || !allowance.Allowed {
		return false
	}
	if tx.Gas() > allowance.GasLimit {
		return false
	}
	return true
}
