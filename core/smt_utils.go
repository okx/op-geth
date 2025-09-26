//go:build migrate_xlayer

// Copyright 2024 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unsafe"

	poseidon "github.com/okx/poseidongold/go"
)

const (
	KEY_BALANCE              = 0
	KEY_NONCE                = 1
	SC_CODE                  = 2
	SC_STORAGE               = 3
	SC_LENGTH                = 4
	HASH_POSEIDON_ALL_ZEROES = "0xc71603f33a1144ca7953db0ab48808f4c4055e3364a246c33c18a9786cb0b359"
	BYTECODE_ELEMENTS_HASH   = 8
	BYTECODE_BYTES_ELEMENT   = 7
)

type NodeValue8 [8]*big.Int
type NodeValue12 [12]*big.Int
type NodeKey [4]uint64

type NodeType int

type Side int

var (
	LeafCapacity   = [4]uint64{1, 0, 0, 0}
	BranchCapacity = [4]uint64{0, 0, 0, 0}
	hashFunc       = poseidon.HashWithResult
)

func Hash(in [8]uint64, capacity [4]uint64) [4]uint64 {
	var result [4]uint64 = [4]uint64{0, 0, 0, 0}
	hashFunc(&in, &capacity, &result)
	return result
}

func HashByPointers(in *[8]uint64, capacity *[4]uint64) *[4]uint64 {
	var result [4]uint64 = [4]uint64{0, 0, 0, 0}
	hashFunc(in, capacity, &result)
	return &result
}

func (nk *NodeKey) IsZero() bool {
	return nk[0] == 0 && nk[1] == 0 && nk[2] == 0 && nk[3] == 0
}

func (nk *NodeKey) IsEqualTo(nk2 NodeKey) bool {
	return nk[0] == nk2[0] && nk[1] == nk2[1] && nk[2] == nk2[2] && nk[3] == nk2[3]
}

func (nk *NodeKey) ToBigInt() *big.Int {
	return ArrayToScalar(nk[:])
}

func (nk *NodeKey) AsUint64Pointer() *[4]uint64 {
	return (*[4]uint64)(nk)
}

func (nv *NodeValue8) IsZero() bool {
	if nv == nil {
		return true
	}

	for i := 0; i < 8; i++ {
		if nv[i] == nil || nv[i].Uint64() != 0 {
			return false
		}
	}

	return true
}

// part = 0 for first 4 values, 1 for the last 4 values
func (nv *NodeValue8) SetHalfValue(values [4]uint64, part int) error {
	if part < 0 || part > 1 {
		return fmt.Errorf("part must be 0 or 1")
	}

	partI := part * 4
	for i, v := range values {
		nlh := big.Int{}
		nlh.SetUint64(v)
		nv[i+partI] = &nlh
	}

	return nil
}

func (nv *NodeValue8) ToUintArray() [8]uint64 {
	var result [8]uint64

	if nv != nil {
		for i := 0; i < 8; i++ {
			if nv[i] != nil {
				result[i] = nv[i].Uint64()
			}
			// if nv[i] is nil, result[i] will remain as its zero value (0)
		}
	}
	// if nv is nil, result will be an array of 8 zeros

	return result
}

func (nv *NodeValue8) ToUintArrayByPointer() *[8]uint64 {
	var result [8]uint64

	if nv != nil {
		for i := 0; i < 8; i++ {
			if nv[i] != nil {
				result[i] = nv[i].Uint64()
			}
			// if nv[i] is nil, result[i] will remain as its zero value (0)
		}
	}
	// if nv is nil, result will be an array of 8 zeros

	return &result
}

func (nv *NodeValue12) ToBigInt() *big.Int {
	return ArrayToScalarBig(nv[:])
}

func (nv *NodeValue12) StripCapacity() [8]uint64 {
	return [8]uint64{nv[0].Uint64(), nv[1].Uint64(), nv[2].Uint64(), nv[3].Uint64(), nv[4].Uint64(), nv[5].Uint64(), nv[6].Uint64(), nv[7].Uint64()}
}

func (nv *NodeValue12) Get0to4() *NodeKey {
	// slice it 0-4
	return &NodeKey{nv[0].Uint64(), nv[1].Uint64(), nv[2].Uint64(), nv[3].Uint64()}
}

func (nv *NodeValue12) Get4to8() *NodeKey {
	// slice it 4-8
	return &NodeKey{nv[4].Uint64(), nv[5].Uint64(), nv[6].Uint64(), nv[7].Uint64()}
}

func (nv *NodeValue12) GetNodeValue8() *NodeValue8 {
	return &NodeValue8{nv[0], nv[1], nv[2], nv[3], nv[4], nv[5], nv[6], nv[7]}
}

func (nv *NodeValue12) Get0to8() [8]uint64 {
	// slice it from 0-8
	return [8]uint64{nv[0].Uint64(), nv[1].Uint64(), nv[2].Uint64(), nv[3].Uint64(), nv[4].Uint64(), nv[5].Uint64(), nv[6].Uint64(), nv[7].Uint64()}
}

func (nv *NodeValue12) IsUniqueSibling() (int, error) {
	count := 0
	fnd := 0
	a := nv[:]

	for i := 0; i < len(a); i += 4 {
		k := NodeKeyFromBigIntArray(a[i : i+4])
		if !k.IsZero() {
			count++
			fnd = i / 4
		}
	}
	if count == 1 {
		return fnd, nil
	}
	return -1, nil
}

func NodeKeyFromBigIntArray(arr []*big.Int) NodeKey {
	nk := NodeKey{}
	for i, v := range arr {
		if v != nil {
			nk[i] = v.Uint64()
		} else {
			nk[i] = 0
		}
	}
	return nk
}

func (nv *NodeValue12) IsZero() bool {
	zero := false
	for _, v := range nv {
		if v.Cmp(big.NewInt(0)) == 0 {
			zero = true
		} else {
			zero = false
			break
		}
	}
	return zero
}

func (nv *NodeValue12) IsFinalNode() bool {
	if nv[8] == nil {
		return false
	}
	return nv[8].Cmp(big.NewInt(1)) == 0
}

// For X Layer, optimize the conversion
func ConvertHexToBigInt(hexStr string) *big.Int {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	isOdd := len(hexStr)%2 != 0
	dstLen := len(hexStr) / 2
	if isOdd {
		dstLen += 1
	}

	dst := make([]byte, dstLen)

	if isOdd {
		singleChar := hexStr[0]
		if singleChar >= 'a' && singleChar <= 'f' {
			dst[0] = singleChar - 'a' + 10
		} else if singleChar >= 'A' && singleChar <= 'F' {
			dst[0] = singleChar - 'A' + 10
		} else {
			dst[0] = singleChar - '0'
		}
		hexStr = hexStr[1:]
	}
	if len(hexStr) != 0 {
		var newDst = dst[:]
		if isOdd {
			newDst = dst[1:]
		}
		n, _ := hex.Decode(newDst, unsafe.Slice(unsafe.StringData(hexStr), len(hexStr)))
		if isOdd {
			dst = dst[:n+1]
		} else {
			dst = dst[:n]
		}
	}

	return new(big.Int).SetBytes(dst)
}

func ScalarToArrayUint64(scalar *big.Int) [8]uint64 {
	var result [8]uint64

	if scalar == nil || scalar.Sign() == 0 {
		return result
	}

	scalarCopy := new(big.Int).Set(scalar)

	for i := 0; i < 8; i++ {
		result[i] = scalarCopy.Uint64() & 0xFFFFFFFF
		scalarCopy.Rsh(scalarCopy, 32)
	}

	return result
}

func ArrayToScalar(array []uint64) *big.Int {
	// For X Layer, optimize the conversion
	if strconv.IntSize == 64 {
		abs := make([]big.Word, len(array))
		for i, v := range array {
			abs[i] = big.Word(v)
		}
		return new(big.Int).SetBits(abs)
	} else {
		abs := make([]big.Word, len(array)*2)
		for i, v := range array {
			abs[i*2] = big.Word(v)
			abs[i*2+1] = big.Word(v >> 32)
		}
		return new(big.Int).SetBits(abs)
	}
}

func ArrayToScalarBig(array []*big.Int) *big.Int {
	// For X Layer, optimize the conversion
	// fast path for 64-bit systems
	v, ok := arrayToScalarBigFast(array)
	if ok {
		return v
	}
	return arrayToScalarBigSlow(array)
}

func (nk *NodeKey) GetPath() []int {
	res := make([]int, 0, 256)
	auxk := [4]uint64{nk[0], nk[1], nk[2], nk[3]}

	for j := 0; j < 64; j++ {
		for i := 0; i < 4; i++ {
			res = append(res, int(auxk[i]&1)) // Append the LSB of the current part to res
			auxk[i] >>= 1                     // Right shift the current part
		}
	}

	return res
}

func RemoveKeyBits(k NodeKey, nBits int) NodeKey {
	var auxk NodeKey
	fullLevels := nBits / 4

	for i := 0; i < 4; i++ {
		n := fullLevels
		if fullLevels*4+i < nBits {
			n += 1
		}
		auxk[i] = k[i] >> uint(n)
	}

	return auxk
}

func KeyEthAddrBalance(ethAddr string) NodeKey {
	return Key(ethAddr, KEY_BALANCE)
}

func KeyEthAddrNonce(ethAddr string) NodeKey {
	return Key(ethAddr, KEY_NONCE)
}

func KeyContractCode(ethAddr string) NodeKey {
	return Key(ethAddr, SC_CODE)
}

func KeyContractLength(ethAddr string) NodeKey {
	return Key(ethAddr, SC_LENGTH)
}

var key1Capacity = [4]uint64{4330397376401421145, 14124799381142128323, 8742572140681234676, 14345658006221440202}

func Key(ethAddr string, c int) NodeKey {
	a := ConvertHexToBigInt(ethAddr)
	key1 := ScalarToArrayUint64(a)
	key1[6] = uint64(c)
	key1[7] = 0

	return Hash(key1, key1Capacity)
}

func charToByte(c byte) byte {
	if c >= '0' && c <= '9' {
		return c - '0'
	}
	if c >= 'a' && c <= 'f' {
		return c - 'a' + 10
	}
	if c >= 'A' && c <= 'F' {
		return c - 'A' + 10
	}
	// should not reach here
	return 0
}

func HashContractBytecodeBigInt(bc string) *big.Int {
	bytecode := strings.TrimPrefix(bc, "0x")

	targetBytesLen := len(bytecode) / 2
	if len(bytecode)%2 != 0 {
		targetBytesLen += 1
	}

	targetBytesLen += 1

	if targetBytesLen%56 != 0 {
		targetBytesLen = targetBytesLen + (56 - targetBytesLen%56)
	}

	targetBytesLen = targetBytesLen / 7 * 8

	targetBytes := make([]byte, targetBytesLen)

	counter := 0
	offset := 0
	i := 0

	if len(bytecode)%2 != 0 {
		targetBytes[offset] = charToByte(bytecode[0])
		offset += 1
		counter += 1
		i += 1
	}

	for ; i < len(bytecode); i += 2 {
		targetBytes[offset] = charToByte(bytecode[i])<<4 | charToByte(bytecode[i+1])
		offset += 1
		counter += 1
		if counter == BYTECODE_BYTES_ELEMENT {
			counter = 0
			offset += 1
			// targetBytes[offset] = 0
		}
	}

	targetBytes[offset] = 0x01
	targetBytes[len(targetBytes)-2] |= 0x80

	tmpData := &[8]uint64{}
	tmpHash := (*[4]uint64)(unsafe.Pointer(tmpData))
	var result = (*[4]uint64)(unsafe.Pointer(unsafe.SliceData(tmpData[4:])))
	for i := 0; i < len(targetBytes); i += 64 {
		in := (*[8]uint64)(unsafe.Pointer(unsafe.SliceData(targetBytes[i:])))
		hashFunc(in, tmpHash, result)
		tmpHash, result = result, tmpHash
	}

	return ArrayToScalar(tmpHash[:])
}

// Helper functions for optimization
func arrayToScalarBigFast(array []*big.Int) (*big.Int, bool) {
	// Fast path implementation for 64-bit systems
	if len(array) != 8 {
		return nil, false
	}

	// Check if all values fit in 32 bits
	for _, v := range array {
		if v == nil || v.BitLen() > 32 {
			return nil, false
		}
	}

	// Fast conversion
	result := new(big.Int)
	for i := len(array) - 1; i >= 0; i-- {
		result.Lsh(result, 32)
		if array[i] != nil {
			result.Add(result, array[i])
		}
	}
	return result, true
}

func arrayToScalarBigSlow(array []*big.Int) *big.Int {
	scalar := new(big.Int)
	for i := len(array) - 1; i >= 0; i-- {
		scalar.Lsh(scalar, 32)
		if array[i] != nil {
			scalar.Add(scalar, array[i])
		}
	}
	return scalar
}
