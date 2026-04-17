// Copyright 2026 The go-ethereum Authors
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
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// Eip8130TxType is the EIP-8130 Native Account Abstraction transaction type.
const Eip8130TxType = 0x7B

// Eip8130Call represents a single call within a phase.
type Eip8130Call struct {
	To   common.Address
	Data []byte
}

// Eip8130Tx implements the EIP-8130 Native Account Abstraction transaction.
//
// RLP encoding order matches the Rust-side TxEip8130 (eip8130-consensus/src/tx.rs):
//
//	[chain_id, from, nonce_key, nonce_sequence, expiry,
//	 max_priority_fee_per_gas, max_fee_per_gas, gas_limit,
//	 account_changes, calls, payer,
//	 sender_auth, payer_auth]
type Eip8130Tx struct {
	ChainID              uint64
	From                 *common.Address `rlp:"nil"` // nil = derived via ecrecover
	NonceKey             *big.Int        // 2D nonce channel (uint256)
	NonceSequence        uint64
	Expiry               uint64 // 0 = no expiry
	MaxPriorityFeePerGas *big.Int
	MaxFeePerGas         *big.Int
	GasLimit             uint64
	AccountChanges       []byte // opaque RLP-encoded account change entries
	Calls                []byte // opaque RLP-encoded phased call batches
	Payer                *common.Address `rlp:"nil"` // nil = sender pays
	SenderAuth           []byte
	PayerAuth            []byte

	// Signature values — not part of the canonical RLP payload above.
	// These are set by the signer and used for hash/sender recovery.
	V *big.Int `rlp:"-"`
	R *big.Int `rlp:"-"`
	S *big.Int `rlp:"-"`
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *Eip8130Tx) copy() TxData {
	cpy := &Eip8130Tx{
		ChainID:       tx.ChainID,
		From:          copyAddressPtr(tx.From),
		NonceKey:      new(big.Int),
		NonceSequence: tx.NonceSequence,
		Expiry:        tx.Expiry,
		GasLimit:      tx.GasLimit,
		Payer:         copyAddressPtr(tx.Payer),
		SenderAuth:    common.CopyBytes(tx.SenderAuth),
		PayerAuth:     common.CopyBytes(tx.PayerAuth),
		AccountChanges: common.CopyBytes(tx.AccountChanges),
		Calls:         common.CopyBytes(tx.Calls),
		MaxPriorityFeePerGas: new(big.Int),
		MaxFeePerGas:         new(big.Int),
		V:                    new(big.Int),
		R:                    new(big.Int),
		S:                    new(big.Int),
	}
	if tx.NonceKey != nil {
		cpy.NonceKey.Set(tx.NonceKey)
	}
	if tx.MaxPriorityFeePerGas != nil {
		cpy.MaxPriorityFeePerGas.Set(tx.MaxPriorityFeePerGas)
	}
	if tx.MaxFeePerGas != nil {
		cpy.MaxFeePerGas.Set(tx.MaxFeePerGas)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	return cpy
}

// accessors for innerTx.
func (tx *Eip8130Tx) txType() byte           { return Eip8130TxType }
func (tx *Eip8130Tx) chainID() *big.Int      { return new(big.Int).SetUint64(tx.ChainID) }
func (tx *Eip8130Tx) accessList() AccessList  { return nil }
func (tx *Eip8130Tx) data() []byte            { return tx.SenderAuth }
func (tx *Eip8130Tx) gas() uint64             { return tx.GasLimit }
func (tx *Eip8130Tx) gasFeeCap() *big.Int     { return new(big.Int).Set(tx.MaxFeePerGas) }
func (tx *Eip8130Tx) gasTipCap() *big.Int     { return new(big.Int).Set(tx.MaxPriorityFeePerGas) }
func (tx *Eip8130Tx) gasPrice() *big.Int      { return new(big.Int).Set(tx.MaxFeePerGas) }
func (tx *Eip8130Tx) value() *big.Int         { return common.Big0 }
func (tx *Eip8130Tx) nonce() uint64           { return tx.NonceSequence }
func (tx *Eip8130Tx) to() *common.Address     { return nil } // AA txs have no single "to"
func (tx *Eip8130Tx) isSystemTx() bool        { return false }

func (tx *Eip8130Tx) effectiveGasPrice(dst *big.Int, baseFee *big.Int) *big.Int {
	if baseFee == nil {
		return dst.Set(tx.MaxFeePerGas)
	}
	tip := dst.Sub(tx.MaxFeePerGas, baseFee)
	if tip.Cmp(tx.MaxPriorityFeePerGas) > 0 {
		tip.Set(tx.MaxPriorityFeePerGas)
	}
	return tip.Add(tip, baseFee)
}

func (tx *Eip8130Tx) rawSignatureValues() (v, r, s *big.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *Eip8130Tx) setSignatureValues(chainID, v, r, s *big.Int) {
	tx.V = v
	tx.R = r
	tx.S = s
}

func (tx *Eip8130Tx) encode(b *bytes.Buffer) error {
	return rlp.Encode(b, tx)
}

func (tx *Eip8130Tx) decode(input []byte) error {
	return rlp.DecodeBytes(input, tx)
}

func (tx *Eip8130Tx) sigHash(chainID *big.Int) common.Hash {
	return prefixedRlpHash(
		Eip8130TxType,
		[]any{
			tx.ChainID,
			tx.From,
			tx.NonceKey,
			tx.NonceSequence,
			tx.Expiry,
			tx.MaxPriorityFeePerGas,
			tx.MaxFeePerGas,
			tx.GasLimit,
			tx.AccountChanges,
			tx.Calls,
			tx.Payer,
		})
}

// Eip8130Inner returns the inner EIP-8130 transaction data if the transaction
// is of type Eip8130TxType. Returns nil otherwise.
func (tx *Transaction) Eip8130Inner() *Eip8130Tx {
	aaTx, ok := tx.inner.(*Eip8130Tx)
	if !ok {
		return nil
	}
	return aaTx
}
