// Copyright 2021 The go-ethereum Authors
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

// txJSON is the JSON representation of transactions.
type txJSON struct {
	Type hexutil.Uint64 `json:"type"`

	ChainID              *hexutil.Big           `json:"chainId,omitempty"`
	Nonce                *hexutil.Uint64        `json:"nonce"`
	To                   *common.Address        `json:"to"`
	Gas                  *hexutil.Uint64        `json:"gas"`
	GasPrice             *hexutil.Big           `json:"gasPrice"`
	MaxPriorityFeePerGas *hexutil.Big           `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *hexutil.Big           `json:"maxFeePerGas"`
	MaxFeePerBlobGas     *hexutil.Big           `json:"maxFeePerBlobGas,omitempty"`
	Value                *hexutil.Big           `json:"value"`
	Input                *hexutil.Bytes         `json:"input"`
	AccessList           *AccessList            `json:"accessList,omitempty"`
	BlobVersionedHashes  []common.Hash          `json:"blobVersionedHashes,omitempty"`
	AuthorizationList    []SetCodeAuthorization `json:"authorizationList,omitempty"`
	V                    *hexutil.Big           `json:"v"`
	R                    *hexutil.Big           `json:"r"`
	S                    *hexutil.Big           `json:"s"`
	YParity              *hexutil.Uint64        `json:"yParity,omitempty"`

	// Deposit transaction fields
	SourceHash *common.Hash    `json:"sourceHash,omitempty"`
	From       *common.Address `json:"from,omitempty"`
	Mint       *hexutil.Big    `json:"mint,omitempty"`
	IsSystemTx *bool           `json:"isSystemTx,omitempty"`

	// Blob transaction sidecar encoding:
	Blobs       []kzg4844.Blob       `json:"blobs,omitempty"`
	Commitments []kzg4844.Commitment `json:"commitments,omitempty"`
	Proofs      []kzg4844.Proof      `json:"proofs,omitempty"`

	// EIP-8130 AA transaction fields:
	NonceKey      *hexutil.Big    `json:"nonceKey,omitempty"`
	NonceSequence *hexutil.Uint64 `json:"nonceSequence,omitempty"`
	Expiry        *hexutil.Uint64 `json:"expiry,omitempty"`
	// AccountChanges and Calls are complex nested types; kept as raw JSON for
	// round-trip fidelity. The UnmarshalJSON path converts them to RLP for
	// AATx.AccountChanges / AATx.Calls.
	AccountChanges json.RawMessage `json:"accountChanges,omitempty"`
	Calls          json.RawMessage `json:"calls,omitempty"`
	Payer          *common.Address `json:"payer,omitempty"`
	SenderAuth     *hexutil.Bytes  `json:"senderAuth,omitempty"`
	PayerAuth      *hexutil.Bytes  `json:"payerAuth,omitempty"`

	// Only used for encoding:
	Hash common.Hash `json:"hash"`
}

// yParityValue returns the YParity value from JSON. For backwards-compatibility reasons,
// this can be given in the 'v' field or the 'yParity' field. If both exist, they must match.
func (tx *txJSON) yParityValue() (*big.Int, error) {
	if tx.YParity != nil {
		val := uint64(*tx.YParity)
		if val != 0 && val != 1 {
			return nil, errInvalidYParity
		}
		bigval := new(big.Int).SetUint64(val)
		if tx.V != nil && tx.V.ToInt().Cmp(bigval) != 0 {
			return nil, errVYParityMismatch
		}
		return bigval, nil
	}
	if tx.V != nil {
		return tx.V.ToInt(), nil
	}
	return nil, errVYParityMissing
}

// MarshalJSON marshals as JSON with a hash.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	var enc txJSON
	// These are set for all tx types.
	enc.Hash = tx.Hash()
	enc.Type = hexutil.Uint64(tx.Type())

	// Other fields are set conditionally depending on tx type.
	switch itx := tx.inner.(type) {
	case *LegacyTx:
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.To = tx.To()
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.GasPrice = (*hexutil.Big)(itx.GasPrice)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.V = (*hexutil.Big)(itx.V)
		enc.R = (*hexutil.Big)(itx.R)
		enc.S = (*hexutil.Big)(itx.S)
		if tx.Protected() {
			enc.ChainID = (*hexutil.Big)(tx.ChainId())
		}

	case *AccessListTx:
		enc.ChainID = (*hexutil.Big)(itx.ChainID)
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.To = tx.To()
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.GasPrice = (*hexutil.Big)(itx.GasPrice)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.AccessList = &itx.AccessList
		enc.V = (*hexutil.Big)(itx.V)
		enc.R = (*hexutil.Big)(itx.R)
		enc.S = (*hexutil.Big)(itx.S)
		yparity := itx.V.Uint64()
		enc.YParity = (*hexutil.Uint64)(&yparity)

	case *DynamicFeeTx:
		enc.ChainID = (*hexutil.Big)(itx.ChainID)
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.To = tx.To()
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.MaxFeePerGas = (*hexutil.Big)(itx.GasFeeCap)
		enc.MaxPriorityFeePerGas = (*hexutil.Big)(itx.GasTipCap)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.AccessList = &itx.AccessList
		enc.V = (*hexutil.Big)(itx.V)
		enc.R = (*hexutil.Big)(itx.R)
		enc.S = (*hexutil.Big)(itx.S)
		yparity := itx.V.Uint64()
		enc.YParity = (*hexutil.Uint64)(&yparity)

	case *BlobTx:
		enc.ChainID = (*hexutil.Big)(itx.ChainID.ToBig())
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.MaxFeePerGas = (*hexutil.Big)(itx.GasFeeCap.ToBig())
		enc.MaxPriorityFeePerGas = (*hexutil.Big)(itx.GasTipCap.ToBig())
		enc.MaxFeePerBlobGas = (*hexutil.Big)(itx.BlobFeeCap.ToBig())
		enc.Value = (*hexutil.Big)(itx.Value.ToBig())
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.AccessList = &itx.AccessList
		enc.BlobVersionedHashes = itx.BlobHashes
		enc.To = tx.To()
		enc.V = (*hexutil.Big)(itx.V.ToBig())
		enc.R = (*hexutil.Big)(itx.R.ToBig())
		enc.S = (*hexutil.Big)(itx.S.ToBig())
		yparity := itx.V.Uint64()
		enc.YParity = (*hexutil.Uint64)(&yparity)
		if sidecar := itx.Sidecar; sidecar != nil {
			enc.Blobs = itx.Sidecar.Blobs
			enc.Commitments = itx.Sidecar.Commitments
			enc.Proofs = itx.Sidecar.Proofs
		}
	case *SetCodeTx:
		enc.ChainID = (*hexutil.Big)(itx.ChainID.ToBig())
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.To = tx.To()
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.MaxFeePerGas = (*hexutil.Big)(itx.GasFeeCap.ToBig())
		enc.MaxPriorityFeePerGas = (*hexutil.Big)(itx.GasTipCap.ToBig())
		enc.Value = (*hexutil.Big)(itx.Value.ToBig())
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.AccessList = &itx.AccessList
		enc.AuthorizationList = itx.AuthList
		enc.V = (*hexutil.Big)(itx.V.ToBig())
		enc.R = (*hexutil.Big)(itx.R.ToBig())
		enc.S = (*hexutil.Big)(itx.S.ToBig())
		yparity := itx.V.Uint64()
		enc.YParity = (*hexutil.Uint64)(&yparity)

	case *DepositTx:
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.To = tx.To()
		enc.SourceHash = &itx.SourceHash
		enc.From = &itx.From
		if itx.Mint != nil {
			enc.Mint = (*hexutil.Big)(itx.Mint)
		}
		enc.IsSystemTx = &itx.IsSystemTransaction
		// other fields will show up as null.

	case *depositTxWithNonce:
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.To = tx.To()
		enc.SourceHash = &itx.SourceHash
		enc.From = &itx.From
		if itx.Mint != nil {
			enc.Mint = (*hexutil.Big)(itx.Mint)
		}
		enc.IsSystemTx = &itx.IsSystemTransaction
		enc.Nonce = (*hexutil.Uint64)(&itx.EffectiveNonce)
		// other fields will show up as null.

	case *AATx:
		enc.ChainID = (*hexutil.Big)(new(big.Int).SetUint64(itx.ChainID))
		enc.From = &itx.From
		enc.NonceKey = (*hexutil.Big)(itx.NonceKey)
		ns := hexutil.Uint64(itx.NonceSequence)
		enc.NonceSequence = &ns
		ex := hexutil.Uint64(itx.Expiry)
		enc.Expiry = &ex
		enc.GasPrice = (*hexutil.Big)(itx.GasPrice)
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.AccountChanges = aaRLPToJSON(itx.AccountChanges)
		enc.Calls = aaRLPToJSON(itx.Calls)
		if itx.Payer != (common.Address{}) {
			enc.Payer = &itx.Payer
		}
		senderAuth := hexutil.Bytes(itx.SenderAuth)
		enc.SenderAuth = &senderAuth
		payerAuth := hexutil.Bytes(itx.PayerAuth)
		enc.PayerAuth = &payerAuth
	}
	return json.Marshal(&enc)
}

// aaRLPToJSON returns the raw RLP value as a JSON raw message (for round-trip).
// The RLP bytes are base64 or hex — we keep them as a raw JSON null if empty.
func aaRLPToJSON(raw rlp.RawValue) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	// Pass raw RLP bytes through hexutil for JSON embedding.
	return json.RawMessage(`"` + hexutil.Encode(raw) + `"`)
}

// aaCallRLP is the on-wire representation of a single AA call: [to, data].
type aaCallRLP struct {
	To   common.Address
	Data []byte
}

// aaCallJSON is the JSON representation of a single AA call from xlayer-reth RPC.
type aaCallJSON struct {
	To   common.Address `json:"to"`
	Data hexutil.Bytes  `json:"data"`
}

// aaCallsFromJSON converts the RPC-JSON calls format to RLP.
// Accepts both structured JSON ([[{to,data},...],…]) from xlayer-reth and
// hex-encoded RLP strings ("0x…") from Go round-trips.
func aaCallsFromJSON(raw json.RawMessage) (rlp.RawValue, error) {
	if len(raw) == 0 || string(raw) == "null" {
		b, err := rlp.EncodeToBytes([][]aaCallRLP{})
		return rlp.RawValue(b), err
	}
	s := string(raw)
	if len(s) > 2 && s[0] == '"' && s[1] == '0' && s[2] == 'x' {
		var hexStr hexutil.Bytes
		if err := json.Unmarshal(raw, &hexStr); err != nil {
			return nil, err
		}
		return rlp.RawValue(hexStr), nil
	}
	var phases [][]aaCallJSON
	if err := json.Unmarshal(raw, &phases); err != nil {
		return nil, fmt.Errorf("calls unmarshal: %w", err)
	}
	rlpPhases := make([][]aaCallRLP, len(phases))
	for i, phase := range phases {
		rlpPhases[i] = make([]aaCallRLP, len(phase))
		for j, call := range phase {
			rlpPhases[i][j] = aaCallRLP{To: call.To, Data: []byte(call.Data)}
		}
	}
	b, err := rlp.EncodeToBytes(rlpPhases)
	return rlp.RawValue(b), err
}

// aaAccountChangesFromJSON converts the RPC-JSON accountChanges to RLP.
// Structured JSON arrays from xlayer-reth and hex-encoded RLP strings are both handled.
// Non-empty structured arrays are not yet supported.
func aaAccountChangesFromJSON(raw json.RawMessage) (rlp.RawValue, error) {
	if len(raw) == 0 || string(raw) == "null" {
		b, err := rlp.EncodeToBytes([]struct{}{})
		return rlp.RawValue(b), err
	}
	s := string(raw)
	if len(s) > 2 && s[0] == '"' && s[1] == '0' && s[2] == 'x' {
		var hexStr hexutil.Bytes
		if err := json.Unmarshal(raw, &hexStr); err != nil {
			return nil, err
		}
		return rlp.RawValue(hexStr), nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("accountChanges unmarshal: %w", err)
	}
	if len(arr) == 0 {
		b, err := rlp.EncodeToBytes([]struct{}{})
		return rlp.RawValue(b), err
	}
	return nil, fmt.Errorf("non-empty accountChanges not yet supported in JSON decode")
}

// UnmarshalJSON unmarshals from JSON.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txJSON
	err := json.Unmarshal(input, &dec)
	if err != nil {
		return err
	}

	// Decode / verify fields according to transaction type.
	var inner TxData
	switch dec.Type {
	case LegacyTxType:
		var itx LegacyTx
		inner = &itx
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' in transaction")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.GasPrice == nil {
			return errors.New("missing required field 'gasPrice' in transaction")
		}
		itx.GasPrice = (*big.Int)(dec.GasPrice)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input

		// signature R
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R = (*big.Int)(dec.R)
		// signature S
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S = (*big.Int)(dec.S)
		// signature V
		if dec.V == nil {
			return errors.New("missing required field 'v' in transaction")
		}
		itx.V = (*big.Int)(dec.V)
		if itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0 {
			if err := sanityCheckSignature(itx.V, itx.R, itx.S, true); err != nil {
				return err
			}
		}

	case AccessListTxType:
		var itx AccessListTx
		inner = &itx
		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		itx.ChainID = (*big.Int)(dec.ChainID)
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' in transaction")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.GasPrice == nil {
			return errors.New("missing required field 'gasPrice' in transaction")
		}
		itx.GasPrice = (*big.Int)(dec.GasPrice)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}

		// signature R
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R = (*big.Int)(dec.R)
		// signature S
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S = (*big.Int)(dec.S)
		// signature V
		itx.V, err = dec.yParityValue()
		if err != nil {
			return err
		}
		if itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0 {
			if err := sanityCheckSignature(itx.V, itx.R, itx.S, false); err != nil {
				return err
			}
		}

	case DynamicFeeTxType:
		var itx DynamicFeeTx
		inner = &itx
		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		itx.ChainID = (*big.Int)(dec.ChainID)
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.MaxPriorityFeePerGas == nil {
			return errors.New("missing required field 'maxPriorityFeePerGas' for txdata")
		}
		itx.GasTipCap = (*big.Int)(dec.MaxPriorityFeePerGas)
		if dec.MaxFeePerGas == nil {
			return errors.New("missing required field 'maxFeePerGas' for txdata")
		}
		itx.GasFeeCap = (*big.Int)(dec.MaxFeePerGas)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}

		// signature R
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R = (*big.Int)(dec.R)
		// signature S
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S = (*big.Int)(dec.S)
		// signature V
		itx.V, err = dec.yParityValue()
		if err != nil {
			return err
		}
		if itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0 {
			if err := sanityCheckSignature(itx.V, itx.R, itx.S, false); err != nil {
				return err
			}
		}

	case BlobTxType:
		var itx BlobTx
		inner = &itx
		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		var overflow bool
		itx.ChainID, overflow = uint256.FromBig(dec.ChainID.ToInt())
		if overflow {
			return errors.New("'chainId' value overflows uint256")
		}
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To == nil {
			return errors.New("missing required field 'to' in transaction")
		}
		itx.To = *dec.To
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.MaxPriorityFeePerGas == nil {
			return errors.New("missing required field 'maxPriorityFeePerGas' for txdata")
		}
		itx.GasTipCap = uint256.MustFromBig((*big.Int)(dec.MaxPriorityFeePerGas))
		if dec.MaxFeePerGas == nil {
			return errors.New("missing required field 'maxFeePerGas' for txdata")
		}
		itx.GasFeeCap = uint256.MustFromBig((*big.Int)(dec.MaxFeePerGas))
		if dec.MaxFeePerBlobGas == nil {
			return errors.New("missing required field 'maxFeePerBlobGas' for txdata")
		}
		itx.BlobFeeCap = uint256.MustFromBig((*big.Int)(dec.MaxFeePerBlobGas))
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = uint256.MustFromBig((*big.Int)(dec.Value))
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}
		if dec.BlobVersionedHashes == nil {
			return errors.New("missing required field 'blobVersionedHashes' in transaction")
		}
		itx.BlobHashes = dec.BlobVersionedHashes

		// signature R
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R, overflow = uint256.FromBig((*big.Int)(dec.R))
		if overflow {
			return errors.New("'r' value overflows uint256")
		}
		// signature S
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S, overflow = uint256.FromBig((*big.Int)(dec.S))
		if overflow {
			return errors.New("'s' value overflows uint256")
		}
		// signature V
		vbig, err := dec.yParityValue()
		if err != nil {
			return err
		}
		itx.V, overflow = uint256.FromBig(vbig)
		if overflow {
			return errors.New("'v' value overflows uint256")
		}
		if itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0 {
			if err := sanityCheckSignature(vbig, itx.R.ToBig(), itx.S.ToBig(), false); err != nil {
				return err
			}
		}

	case SetCodeTxType:
		var itx SetCodeTx
		inner = &itx
		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		var overflow bool
		itx.ChainID, overflow = uint256.FromBig(dec.ChainID.ToInt())
		if overflow {
			return errors.New("'chainId' value overflows uint256")
		}
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To == nil {
			return errors.New("missing required field 'to' in transaction")
		}
		itx.To = *dec.To
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.MaxPriorityFeePerGas == nil {
			return errors.New("missing required field 'maxPriorityFeePerGas' for txdata")
		}
		itx.GasTipCap = uint256.MustFromBig((*big.Int)(dec.MaxPriorityFeePerGas))
		if dec.MaxFeePerGas == nil {
			return errors.New("missing required field 'maxFeePerGas' for txdata")
		}
		itx.GasFeeCap = uint256.MustFromBig((*big.Int)(dec.MaxFeePerGas))
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = uint256.MustFromBig((*big.Int)(dec.Value))
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}
		if dec.AuthorizationList == nil {
			return errors.New("missing required field 'authorizationList' in transaction")
		}
		itx.AuthList = dec.AuthorizationList

		// signature R
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R, overflow = uint256.FromBig((*big.Int)(dec.R))
		if overflow {
			return errors.New("'r' value overflows uint256")
		}
		// signature S
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S, overflow = uint256.FromBig((*big.Int)(dec.S))
		if overflow {
			return errors.New("'s' value overflows uint256")
		}
		// signature V
		vbig, err := dec.yParityValue()
		if err != nil {
			return err
		}
		itx.V, overflow = uint256.FromBig(vbig)
		if overflow {
			return errors.New("'v' value overflows uint256")
		}
		if itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0 {
			if err := sanityCheckSignature(vbig, itx.R.ToBig(), itx.S.ToBig(), false); err != nil {
				return err
			}
		}

	case DepositTxType:
		if dec.AccessList != nil || dec.MaxFeePerGas != nil ||
			dec.MaxPriorityFeePerGas != nil {
			return errors.New("unexpected field(s) in deposit transaction")
		}
		if dec.GasPrice != nil && dec.GasPrice.ToInt().Cmp(common.Big0) != 0 {
			return errors.New("deposit transaction GasPrice must be 0")
		}
		if (dec.V != nil && dec.V.ToInt().Cmp(common.Big0) != 0) ||
			(dec.R != nil && dec.R.ToInt().Cmp(common.Big0) != 0) ||
			(dec.S != nil && dec.S.ToInt().Cmp(common.Big0) != 0) {
			return errors.New("deposit transaction signature must be 0 or unset")
		}
		var itx DepositTx
		inner = &itx
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		// mint may be omitted or nil if there is nothing to mint.
		itx.Mint = (*big.Int)(dec.Mint)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.From == nil {
			return errors.New("missing required field 'from' in transaction")
		}
		itx.From = *dec.From
		if dec.SourceHash == nil {
			return errors.New("missing required field 'sourceHash' in transaction")
		}
		itx.SourceHash = *dec.SourceHash
		// IsSystemTx may be omitted. Defaults to false.
		if dec.IsSystemTx != nil {
			itx.IsSystemTransaction = *dec.IsSystemTx
		}

		if dec.Nonce != nil {
			inner = &depositTxWithNonce{DepositTx: itx, EffectiveNonce: uint64(*dec.Nonce)}
		}

	case AATxType:
		var itx AATx
		inner = &itx
		if dec.ChainID != nil {
			itx.ChainID = dec.ChainID.ToInt().Uint64()
		}
		// Wire `from` is zero (RLP 0x80) for EOA-mode. The JSON `from` field currently
		// carries the recovered sender (injected by xlayer-reth), not the wire value.
		// Always keep From=zero so MarshalBinary produces the correct EOA-mode wire bytes.
		// TODO: once xlayer-reth serializes wire `from` (null for EOA, addr for explicit),
		// switch to: if dec.From != nil { itx.From = *dec.From }
		if dec.GasPrice == nil {
			return errors.New("missing required field 'gasPrice' for AA transaction")
		}
		itx.GasPrice = dec.GasPrice.ToInt()
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for AA transaction")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.NonceKey != nil {
			itx.NonceKey = dec.NonceKey.ToInt()
		} else {
			itx.NonceKey = new(big.Int)
		}
		if dec.NonceSequence != nil {
			itx.NonceSequence = uint64(*dec.NonceSequence)
		}
		if dec.Expiry != nil {
			itx.Expiry = uint64(*dec.Expiry)
		}
		var aaErr error
		itx.AccountChanges, aaErr = aaAccountChangesFromJSON(dec.AccountChanges)
		if aaErr != nil {
			return fmt.Errorf("AA accountChanges: %w", aaErr)
		}
		itx.Calls, aaErr = aaCallsFromJSON(dec.Calls)
		if aaErr != nil {
			return fmt.Errorf("AA calls: %w", aaErr)
		}
		if dec.Payer != nil {
			itx.Payer = *dec.Payer
		}
		if dec.SenderAuth != nil {
			itx.SenderAuth = []byte(*dec.SenderAuth)
		}
		if dec.PayerAuth != nil {
			itx.PayerAuth = []byte(*dec.PayerAuth)
		}

	default:
		return ErrTxTypeNotSupported
	}

	// Now set the inner transaction.
	tx.setDecoded(inner, 0)

	return nil
}

type depositTxWithNonce struct {
	DepositTx
	EffectiveNonce uint64
}

// EncodeRLP ensures that RLP encoding this transaction excludes the nonce. Otherwise, the tx Hash would change
func (tx *depositTxWithNonce) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, tx.DepositTx)
}

func (tx *depositTxWithNonce) effectiveNonce() *uint64 { return &tx.EffectiveNonce }
