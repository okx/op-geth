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
)

// AaTxType is the EIP-8130 (Native AA) transaction type identifier.
const AaTxType = 0x7B

// AaTx is an opaque container for an EIP-8130 Account Abstraction transaction.
//
// op-node and op-batcher (the only Go consumers in our stack) never need to
// inspect the inner fields of an AA transaction — they only round-trip the
// raw RLP bytes through block fetching, batching, and engine API calls. The
// real EIP-8130 logic lives in op-reth (Rust) per the protocol spec.
//
// Therefore this implementation stores the typed-payload bytes verbatim and
// emits them on demand. The recovered transaction hash is the standard
// keccak256(0x7B || RawPayload), matching what op-reth produces.
//
// Accessors that don't apply to the AA tx semantics return zero values; this
// is safe because op-node never reads gas / nonce / value / to / data /
// signature for type-0x7B transactions when forwarding them.
type AaTx struct {
	// RawPayload holds the EIP-2718 typed-payload bytes WITHOUT the 0x7B
	// prefix byte. That is, the RLP-encoded list of EIP-8130 fields.
	RawPayload []byte
}

// copy creates a deep copy of the transaction data.
func (tx *AaTx) copy() TxData {
	return &AaTx{RawPayload: common.CopyBytes(tx.RawPayload)}
}

// accessors for innerTx.
func (tx *AaTx) txType() byte           { return AaTxType }
func (tx *AaTx) chainID() *big.Int      { return common.Big0 }
func (tx *AaTx) accessList() AccessList { return nil }
func (tx *AaTx) data() []byte           { return nil }
func (tx *AaTx) gas() uint64            { return 0 }
func (tx *AaTx) gasFeeCap() *big.Int    { return new(big.Int) }
func (tx *AaTx) gasTipCap() *big.Int    { return new(big.Int) }
func (tx *AaTx) gasPrice() *big.Int     { return new(big.Int) }
func (tx *AaTx) value() *big.Int        { return new(big.Int) }
func (tx *AaTx) nonce() uint64          { return 0 }
func (tx *AaTx) to() *common.Address    { return nil }
func (tx *AaTx) isSystemTx() bool       { return false }

func (tx *AaTx) effectiveGasPrice(dst *big.Int, baseFee *big.Int) *big.Int {
	return dst.Set(new(big.Int))
}

func (tx *AaTx) effectiveNonce() *uint64 { return nil }

func (tx *AaTx) sigHash(*big.Int) common.Hash {
	// EIP-8130 has its own dual-domain signing flow (sender_signature_hash /
	// payer_signature_hash) on the Rust side. This Go-side TxData is only
	// used for round-tripping; signing must never go through this path.
	panic("AA transactions cannot be signed via the standard Transaction signer")
}

func (tx *AaTx) rawSignatureValues() (v, r, s *big.Int) {
	// AA tx authentication is in sender_auth/payer_auth bytes, not in
	// top-level v/r/s. Return zero per Deposit-tx convention.
	return common.Big0, common.Big0, common.Big0
}

func (tx *AaTx) setSignatureValues(chainID, v, r, s *big.Int) {
	// no-op — see rawSignatureValues
}

// encode appends the saved RLP payload bytes to the buffer. The 0x7B type
// byte is written by Transaction.encodeTyped before this is called.
func (tx *AaTx) encode(b *bytes.Buffer) error {
	_, err := b.Write(tx.RawPayload)
	return err
}

// decode stores the raw payload bytes (everything after the 0x7B type byte).
func (tx *AaTx) decode(input []byte) error {
	tx.RawPayload = common.CopyBytes(input)
	return nil
}
