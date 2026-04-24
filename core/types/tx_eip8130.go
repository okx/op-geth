// Copyright 2025 The go-ethereum Authors
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
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// AATx implements the EIP-8130 account-abstraction transaction type.
//
// Wire format (RLP list after the 0x7B type byte):
//
//	[chain_id, from?, nonce_key, nonce_sequence, expiry,
//	 gas_price, gas_limit, account_changes, calls,
//	 payer?, sender_auth, payer_auth]
//
// from and payer use the "optional address" encoding:
// zero address → 0x80 (empty RLP string), non-zero → 20-byte RLP string.
type AATx struct {
	ChainID        uint64
	From           common.Address // zero ⇒ EOA-recovery mode
	NonceKey       *big.Int       // 2D-nonce channel selector
	NonceSequence  uint64         // 2D-nonce sequential counter
	Expiry         uint64         // Unix timestamp; 0 = no expiry
	GasPrice       *big.Int
	Gas            uint64
	AccountChanges rlp.RawValue // raw RLP: account_changes list
	Calls          rlp.RawValue // raw RLP: calls list-of-lists
	Payer          common.Address // zero ⇒ sender pays
	SenderAuth     []byte
	PayerAuth      []byte
}

// aaOptAddr encodes an address as an RLP optional:
// zero address → 0x80 (empty string / None), non-zero → 20-byte string.
// This matches Rust's encode_optional_address / decode_optional_address.
type aaOptAddr [20]byte

func (a aaOptAddr) EncodeRLP(w io.Writer) error {
	if a == (aaOptAddr{}) {
		_, err := w.Write([]byte{0x80})
		return err
	}
	return rlp.Encode(w, a[:])
}

// aaTxWire is the struct used for RLP encode/decode, mirroring the wire layout.
type aaTxWire struct {
	ChainID        uint64
	From           aaOptAddr
	NonceKey       *big.Int
	NonceSequence  uint64
	Expiry         uint64
	GasPrice       *big.Int
	Gas            uint64
	AccountChanges rlp.RawValue
	Calls          rlp.RawValue
	Payer          aaOptAddr
	SenderAuth     []byte
	PayerAuth      []byte
}

// EncodeRLP implements rlp.Encoder, delegating to aaTxWire for optional-address encoding.
func (tx *AATx) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &aaTxWire{
		ChainID:        tx.ChainID,
		From:           aaOptAddr(tx.From),
		NonceKey:       tx.NonceKey,
		NonceSequence:  tx.NonceSequence,
		Expiry:         tx.Expiry,
		GasPrice:       tx.GasPrice,
		Gas:            tx.Gas,
		AccountChanges: tx.AccountChanges,
		Calls:          tx.Calls,
		Payer:          aaOptAddr(tx.Payer),
		SenderAuth:     tx.SenderAuth,
		PayerAuth:      tx.PayerAuth,
	})
}

// DecodeRLP implements rlp.Decoder.
func (tx *AATx) DecodeRLP(s *rlp.Stream) error {
	// Decode the outer list into aaTxWire, then decode optional addresses manually.
	var wire struct {
		ChainID        uint64
		From           []byte
		NonceKey       *big.Int
		NonceSequence  uint64
		Expiry         uint64
		GasPrice       *big.Int
		Gas            uint64
		AccountChanges rlp.RawValue
		Calls          rlp.RawValue
		Payer          []byte
		SenderAuth     []byte
		PayerAuth      []byte
	}
	if err := s.Decode(&wire); err != nil {
		return fmt.Errorf("decode AATx: %w", err)
	}

	from, err := decodeOptionalAddrBytes(wire.From)
	if err != nil {
		return fmt.Errorf("decode AATx from: %w", err)
	}
	payer, err := decodeOptionalAddrBytes(wire.Payer)
	if err != nil {
		return fmt.Errorf("decode AATx payer: %w", err)
	}

	tx.ChainID = wire.ChainID
	tx.From = from
	tx.NonceKey = wire.NonceKey
	if tx.NonceKey == nil {
		tx.NonceKey = new(big.Int)
	}
	tx.NonceSequence = wire.NonceSequence
	tx.Expiry = wire.Expiry
	tx.GasPrice = wire.GasPrice
	if tx.GasPrice == nil {
		tx.GasPrice = new(big.Int)
	}
	tx.Gas = wire.Gas
	tx.AccountChanges = wire.AccountChanges
	tx.Calls = wire.Calls
	tx.Payer = payer
	tx.SenderAuth = wire.SenderAuth
	tx.PayerAuth = wire.PayerAuth
	return nil
}

func decodeOptionalAddrBytes(b []byte) (common.Address, error) {
	switch len(b) {
	case 0:
		return common.Address{}, nil
	case 20:
		return common.BytesToAddress(b), nil
	default:
		return common.Address{}, fmt.Errorf("invalid optional address length %d", len(b))
	}
}

// copy implements TxData.
func (tx *AATx) copy() TxData {
	cpy := &AATx{
		ChainID:       tx.ChainID,
		From:          tx.From,
		NonceSequence: tx.NonceSequence,
		Expiry:        tx.Expiry,
		Gas:           tx.Gas,
		Payer:         tx.Payer,
		NonceKey:      new(big.Int),
		GasPrice:      new(big.Int),
	}
	if tx.NonceKey != nil {
		cpy.NonceKey.Set(tx.NonceKey)
	}
	if tx.GasPrice != nil {
		cpy.GasPrice.Set(tx.GasPrice)
	}
	if tx.AccountChanges != nil {
		cpy.AccountChanges = make(rlp.RawValue, len(tx.AccountChanges))
		copy(cpy.AccountChanges, tx.AccountChanges)
	}
	if tx.Calls != nil {
		cpy.Calls = make(rlp.RawValue, len(tx.Calls))
		copy(cpy.Calls, tx.Calls)
	}
	cpy.SenderAuth = common.CopyBytes(tx.SenderAuth)
	cpy.PayerAuth = common.CopyBytes(tx.PayerAuth)
	return cpy
}

// TxData accessor methods.
func (tx *AATx) txType() byte           { return AATxType }
func (tx *AATx) chainID() *big.Int      { return new(big.Int).SetUint64(tx.ChainID) }
func (tx *AATx) accessList() AccessList { return nil }
func (tx *AATx) data() []byte           { return nil }
func (tx *AATx) gas() uint64            { return tx.Gas }
func (tx *AATx) gasPrice() *big.Int     { return new(big.Int).Set(tx.GasPrice) }
func (tx *AATx) gasTipCap() *big.Int    { return new(big.Int).Set(tx.GasPrice) }
func (tx *AATx) gasFeeCap() *big.Int    { return new(big.Int).Set(tx.GasPrice) }
func (tx *AATx) value() *big.Int        { return new(big.Int) }
func (tx *AATx) nonce() uint64          { return tx.NonceSequence }
func (tx *AATx) to() *common.Address    { return nil }
func (tx *AATx) isSystemTx() bool       { return false }

func (tx *AATx) effectiveGasPrice(dst *big.Int, _ *big.Int) *big.Int {
	return dst.Set(tx.GasPrice)
}

func (tx *AATx) rawSignatureValues() (v, r, s *big.Int) {
	return common.Big0, common.Big0, common.Big0
}

func (tx *AATx) setSignatureValues(_, _, _, _ *big.Int) {
	// AA transactions are authorised by sender_auth / payer_auth blobs, not ECDSA.
}

// sigHash returns the EIP-8130 sender-signing hash:
// keccak256(0x7B || RLP([chain_id, from, nonce_key, nonce_sequence, expiry,
//
//	gas_price, gas_limit, account_changes, calls, payer]))
//
// sender_auth and payer_auth are intentionally excluded.
func (tx *AATx) sigHash(_ *big.Int) common.Hash {
	return prefixedRlpHash(AATxType, []any{
		tx.ChainID,
		aaOptAddr(tx.From),
		tx.NonceKey,
		tx.NonceSequence,
		tx.Expiry,
		tx.GasPrice,
		tx.Gas,
		tx.AccountChanges,
		tx.Calls,
		aaOptAddr(tx.Payer),
	})
}

func (tx *AATx) encode(b *bytes.Buffer) error {
	return rlp.Encode(b, tx)
}

func (tx *AATx) decode(input []byte) error {
	return rlp.DecodeBytes(input, tx)
}
