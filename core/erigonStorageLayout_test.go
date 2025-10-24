package core

import (
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestGetProposerSlot_Deterministic(t *testing.T) {
	account := common.HexToAddress("0xff6250d0E86A2465B0C1bF8e36409503d6a26963")
	memBerSlot, adminSlot := GetProposerSlot(account)

	if memBerSlot.Cmp(common.HexToHash("0xc8e266e0814671642b74f3807affd27009fcc23f713ea92d1743e0ee0c1e7603")) != 0 {
		t.Errorf("MEMBER_SLOT should be equal to zero")
	}
	if adminSlot.Cmp(common.HexToHash("0x3412d5605ac6cd444957cedb533e5dacad6378b4bc819ebe3652188a665066d6")) != 0 {
		t.Errorf("ADMIN_SLOT should be equal to zero")
	}
}

func TestGetExecutorSlot_Deterministic(t *testing.T) {
	account := common.HexToAddress("0xff6250d0E86A2465B0C1bF8e36409503d6a26963")
	memBerSlot, adminSlot := GetExecutorSlot(account)

	if memBerSlot.Cmp(common.HexToHash("0x9b3efc411c5f69533db363941e091f6f3af8b7e306525413577a56d27e5dbe73")) != 0 {
		t.Errorf("MEMBER_SLOT should be equal to zero")
	}
	if adminSlot.Cmp(common.HexToHash("0xdae2aa361dfd1ca020a396615627d436107c35eff9fe7738a3512819782d706a")) != 0 {
		t.Errorf("ADMIN_SLOT should be equal to zero")
	}
}

func TestCalculateBatchInboxAddr(t *testing.T) {
	chainId := LeftPadBytes(big.NewInt(196).Bytes(), 32)
	chainIdHash := common.BytesToHash(chainId)
	addr := CalculateBatchInboxAddr(chainIdHash)
	assert.Equal(t, "0x002bdE9b0c0857AEE2cFFDea6b8723eAF5989449", addr.Hex())
}
