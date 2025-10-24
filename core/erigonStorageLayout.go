package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/iden3/go-iden3-crypto/keccak256"
	"math/big"
)

// LeftPadBytes zero-pads slice to the left up to length l.
func LeftPadBytes(slice []byte, l int) []byte {
	if l <= len(slice) {
		return slice
	}

	padded := make([]byte, l)
	copy(padded[l-len(slice):], slice)

	return padded
}

func roleLayout(proposerRoleId common.Hash, account common.Address) (common.Hash, common.Hash) {

	d1 := LeftPadBytes(proposerRoleId.Bytes(), 32)
	d2 := LeftPadBytes(common.HexToHash("0x0").Bytes(), 32)
	base := keccak256.Hash(d1, d2)

	d3 := LeftPadBytes(account.Bytes(), 32)
	d4 := LeftPadBytes(base, 32)
	memberSlot := keccak256.Hash(d3, d4)
	memberSlotHash := common.BytesToHash(memberSlot[:])

	baseBigInt := new(big.Int).SetBytes(base)
	baseBigInt.Add(baseBigInt, big.NewInt(1))

	adminSlot := LeftPadBytes(baseBigInt.Bytes(), 32)

	return memberSlotHash, common.BytesToHash(adminSlot[:])
}
func GetProposerSlot(account common.Address) (common.Hash, common.Hash) {
	proposerRoleId := common.HexToHash("0xb09aa5aeb3702cfd50b6b62bc4532604938f21248a27a1d5ca736082b6819cc1")
	return roleLayout(proposerRoleId, account)
}

func GetExecutorSlot(account common.Address) (common.Hash, common.Hash) {
	executorRoleId := common.HexToHash("0xd8aa0f3194971a2a116679f7c2090f6939c8d4e01a2a8d7e41d55e5351469e63")
	return roleLayout(executorRoleId, account)
}

func CalculateBatchInboxAddr(chainID common.Hash) common.Address {
	versionByte := byte(0x00)
	var out common.Address
	out[0] = versionByte
	copy(out[1:], keccak256.Hash(chainID[:])[:19])
	return out
}
