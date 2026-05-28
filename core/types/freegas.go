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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// FreeGasConfigAddr is the predeploy address of the FreeGasConfig contract.
// The contract exposes:
//
//	function getList()           external view returns (address[] memory);
//	function isFreeGasEnabled()  external view returns (bool);
var FreeGasConfigAddr = common.HexToAddress("0x420000000000000000000000000000000000002E")

// FreeGasState is the per-block snapshot of FreeGasConfig read from the
// predeploy at block-start. A nil receiver is treated as "disabled".
type FreeGasState struct {
	Enabled bool
	List    map[common.Address]struct{}
}

// Has reports whether the given address is in the free-gas list. Safe to call
// on a nil receiver.
func (s *FreeGasState) Has(addr common.Address) bool {
	if s == nil || !s.Enabled || s.List == nil {
		return false
	}
	_, ok := s.List[addr]
	return ok
}

// Cached function selectors for the FreeGasConfig contract.
var (
	getListSelector          = crypto.Keccak256([]byte("getList()"))[:4]
	isFreeGasEnabledSelector = crypto.Keccak256([]byte("isFreeGasEnabled()"))[:4]
)

// GetListCalldata returns the calldata for invoking getList().
func GetListCalldata() []byte {
	out := make([]byte, 4)
	copy(out, getListSelector)
	return out
}

// IsFreeGasEnabledCalldata returns the calldata for invoking isFreeGasEnabled().
func IsFreeGasEnabledCalldata() []byte {
	out := make([]byte, 4)
	copy(out, isFreeGasEnabledSelector)
	return out
}

// DecodeAddressList decodes ABI-encoded `address[]` return data. Layout:
//
//	[0..32)  offset (must be 0x20 for a single dynamic return)
//	[32..64) length N
//	[64..)   N entries of 32 bytes each (address left-padded with 12 zero bytes)
func DecodeAddressList(ret []byte) ([]common.Address, error) {
	if len(ret) < 64 {
		return nil, errors.New("freegas: address[] return too short")
	}
	offset := binary.BigEndian.Uint64(ret[24:32])
	if offset != 32 {
		return nil, errors.New("freegas: unexpected offset in address[] return")
	}
	length := binary.BigEndian.Uint64(ret[56:64])
	if length > uint64(len(ret)-64)/32 {
		return nil, errors.New("freegas: declared length exceeds buffer")
	}
	out := make([]common.Address, length)
	for i := uint64(0); i < length; i++ {
		base := 64 + i*32
		out[i] = common.BytesToAddress(ret[base : base+32])
	}
	return out, nil
}

// DecodeBool decodes ABI-encoded `bool` return data (32 bytes, non-zero = true).
func DecodeBool(ret []byte) (bool, error) {
	if len(ret) < 32 {
		return false, errors.New("freegas: bool return too short")
	}
	for _, b := range ret[:32] {
		if b != 0 {
			return true, nil
		}
	}
	return false, nil
}

// IsFreeGasTx reports whether the given transaction qualifies for free-gas
// execution under the supplied per-block FreeGasState.
func IsFreeGasTx(tx *Transaction, fg *FreeGasState) bool {
	if fg == nil || !fg.Enabled {
		return false
	}
	if tx.Type() != DynamicFeeTxType {
		return false
	}
	to := tx.To()
	if to == nil {
		return false
	}
	return fg.Has(*to)
}
