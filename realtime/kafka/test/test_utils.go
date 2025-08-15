package test

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"

	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
	"github.com/holiman/uint256"
	"gotest.tools/assert"
)

var (
	DefaultL2ChainID         *big.Int = big.NewInt(195)
	DefaultL2AdminAddress             = "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
	DefaultL2AdminPrivateKey          = "0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"

	addr         = libcommon.HexToAddress("0x0000000000000000000000000000000000000001")
	testFromAddr = libcommon.HexToAddress("0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534")
	testToAddr   = libcommon.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	accesses     = types.AccessList{{Address: addr, StorageKeys: []libcommon.Hash{{0}}}}
)

var (
	signer        = types.LatestSignerForChainID(DefaultL2ChainID)
	privateKey, _ = getPrivateKeyFromHex(DefaultL2AdminPrivateKey)

	emptyTx = types.NewTransaction(
		0,
		libcommon.HexToAddress(testToAddr.String()),
		big.NewInt(0), 0, big.NewInt(10),
		nil,
	)
	signedEmptyTx, _ = types.SignTx(emptyTx, signer, privateKey)
	emptyTxReceipt   = types.NewReceipt(nil, false, 1000)

	legacyTx = types.NewTransaction(
		3,
		testToAddr,
		big.NewInt(10),
		2000,
		big.NewInt(1),
		libcommon.FromHex("5544"),
	)
	signedLegacyTx, _ = types.SignTx(legacyTx, signer, privateKey)

	accessListTx = types.NewTx(&types.AccessListTx{
		ChainID:    DefaultL2ChainID,
		Nonce:      3,
		Gas:        25000,
		GasPrice:   big.NewInt(1),
		To:         &testToAddr,
		Value:      big.NewInt(10),
		Data:       libcommon.FromHex("5544"),
		AccessList: accesses,
	})
	signedAccessListTx, _ = types.SignTx(accessListTx, signer, privateKey)

	dynFeeTx = types.NewTx(&types.DynamicFeeTx{
		ChainID:    DefaultL2ChainID,
		Nonce:      3,
		Gas:        25000,
		GasFeeCap:  big.NewInt(1),
		GasTipCap:  big.NewInt(1),
		To:         &testToAddr,
		Value:      big.NewInt(10),
		Data:       libcommon.FromHex("5544"),
		AccessList: accesses,
	})
	signedDynFeeTx, _ = types.SignTx(dynFeeTx, signer, privateKey)

	blobTx = types.NewTx(&types.BlobTx{
		ChainID:    uint256.MustFromBig(DefaultL2ChainID),
		Nonce:      3,
		Gas:        25000,
		GasFeeCap:  uint256.MustFromBig(big.NewInt(1)),
		GasTipCap:  uint256.MustFromBig(big.NewInt(1)),
		To:         testToAddr,
		Value:      uint256.MustFromBig(big.NewInt(10)),
		Data:       libcommon.FromHex("5544"),
		AccessList: accesses,
		BlobFeeCap: uint256.MustFromBig(big.NewInt(10)),
		BlobHashes: []libcommon.Hash{{0}},
	})
	signedBlobTx, _ = types.SignTx(blobTx, signer, privateKey)

	txReceipt = &types.Receipt{
		Type:              types.LegacyTxType,
		PostState:         libcommon.Hash{2}.Bytes(),
		CumulativeGasUsed: 3,
		Logs: []*types.Log{
			{Address: libcommon.BytesToAddress([]byte{0x22})},
			{Address: libcommon.BytesToAddress([]byte{0x02, 0x22})},
		},
		TxHash:          signedLegacyTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x02, 0x22, 0x22}),
		GasUsed:         2,
	}

	txInnerTxs = []*types.InnerTx{
		{
			Name:     "innerTx1",
			CallType: types.CALL_TYP,
		},
	}

	txChangeset = &realtimeTypes.Changeset{
		BalanceChanges: map[libcommon.Address]*uint256.Int{
			testToAddr: uint256.NewInt(10),
		},
	}

	accessListTxReceipt = &types.Receipt{
		Type:              types.AccessListTxType,
		PostState:         libcommon.Hash{3}.Bytes(),
		CumulativeGasUsed: 6,
		Logs: []*types.Log{
			{Address: libcommon.BytesToAddress([]byte{0x33})},
			{Address: libcommon.BytesToAddress([]byte{0x03, 0x33})},
		},
		TxHash:          signedAccessListTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x03, 0x33, 0x33}),
		GasUsed:         3,
	}

	dynFeeTxReceipt = &types.Receipt{
		Type:              types.DynamicFeeTxType,
		PostState:         libcommon.Hash{4}.Bytes(),
		CumulativeGasUsed: 10,
		Logs: []*types.Log{
			{Address: libcommon.BytesToAddress([]byte{0x33})},
			{Address: libcommon.BytesToAddress([]byte{0x03, 0x33})},
		},
		TxHash:          signedDynFeeTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x03, 0x33, 0x33}),
		GasUsed:         3,
	}

	blobTxReceipt = &types.Receipt{
		Type:              types.BlobTxType,
		PostState:         libcommon.Hash{2}.Bytes(),
		CumulativeGasUsed: 15,
		Logs: []*types.Log{
			{Address: libcommon.BytesToAddress([]byte{0x22})},
			{Address: libcommon.BytesToAddress([]byte{0x02, 0x22})},
		},
		TxHash:          signedBlobTx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x02, 0x22, 0x22}),
		GasUsed:         5,
	}
)

func AssertHeader(t *testing.T, header *types.Header, rcvHeader *types.Header) {
	if header == nil {
		if rcvHeader != nil {
			t.Fatalf("header is nil, but rcvHeader is not nil")
		}
		return
	}
	assert.Equal(t, header.ParentHash, rcvHeader.ParentHash)
	assert.Equal(t, header.UncleHash, rcvHeader.UncleHash)
	assert.Equal(t, header.Coinbase, rcvHeader.Coinbase)
	assert.Equal(t, header.Root, rcvHeader.Root)
	assert.Equal(t, header.TxHash, rcvHeader.TxHash)
	assert.Equal(t, header.ReceiptHash, rcvHeader.ReceiptHash)
	assert.Equal(t, header.Bloom, rcvHeader.Bloom)
	assert.Equal(t, header.Number.String(), rcvHeader.Number.String())
	assert.Equal(t, header.Difficulty.String(), rcvHeader.Difficulty.String())
	assert.Equal(t, header.GasLimit, rcvHeader.GasLimit)
	assert.Equal(t, header.GasUsed, rcvHeader.GasUsed)
	assert.Equal(t, header.Time, rcvHeader.Time)
	assert.Equal(t, string(header.Extra), string(rcvHeader.Extra))
	assert.Equal(t, header.BaseFee.String(), rcvHeader.BaseFee.String())
	assert.Equal(t, header.BlobGasUsed, rcvHeader.BlobGasUsed)
	assert.Equal(t, header.ExcessBlobGas, rcvHeader.ExcessBlobGas)
}

func AssertCommonTx(t *testing.T, msg kafkaTypes.TransactionMessage, tx *types.Transaction, blockNumber uint64, txType int) {
	assert.Equal(t, msg.BlockNumber, blockNumber)
	assert.Equal(t, int(msg.Type), txType)
	assert.Equal(t, msg.Hash, tx.Hash())
	assert.Equal(t, msg.ChainID.Uint64(), tx.ChainId().Uint64())
	assert.Equal(t, msg.Nonce, tx.Nonce())
	assert.Equal(t, msg.Gas, tx.Gas())
	assert.Equal(t, msg.To.String(), testToAddr.String())
	assert.Equal(t, msg.Value.String(), tx.Value().String())
	assert.Equal(t, string(msg.Data), string(tx.Data()))
	v, r, s := tx.RawSignatureValues()
	assert.Equal(t, msg.R.String(), r.String())
	assert.Equal(t, msg.S.String(), s.String())
	assert.Equal(t, msg.V.String(), v.String())

	// Check sender
	txSender, err := types.Sender(signer, tx)
	assert.NilError(t, err)
	assert.Equal(t, txSender, testFromAddr)
}

func AssertAccessList(t *testing.T, msgAccessList []kafkaTypes.AccessTupleMessage) {
	assert.Equal(t, len(msgAccessList), len(accesses))
	for idx, access := range msgAccessList {
		assert.Equal(t, access.Address, accesses[idx].Address.String())
		assert.Equal(t, len(access.StorageKeys), len(accesses[idx].StorageKeys))
		for i, storageKey := range access.StorageKeys {
			assert.Equal(t, storageKey, accesses[idx].StorageKeys[i].Hex())
		}
	}
}

func assertTxAccessList(t *testing.T, accessList types.AccessList) {
	assert.Equal(t, len(accessList), len(accesses))
	for idx, access := range accessList {
		assert.Equal(t, access.Address, accesses[idx].Address)
		assert.Equal(t, len(access.StorageKeys), len(accesses[idx].StorageKeys))
		for i, storageKey := range access.StorageKeys {
			assert.Equal(t, storageKey, accesses[idx].StorageKeys[i])
		}
	}
}

func AssertReceipt(t *testing.T, msg kafkaTypes.TransactionMessage, receipt *types.Receipt) {
	assert.Equal(t, msg.Receipt.Type, receipt.Type)
	assert.Equal(t, string(msg.Receipt.PostState), string(receipt.PostState))
	assert.Equal(t, msg.Receipt.Status, receipt.Status)
	assert.Equal(t, msg.Receipt.CumulativeGasUsed, receipt.CumulativeGasUsed)
	assert.Equal(t, msg.Receipt.Bloom, receipt.Bloom)
	assert.Equal(t, len(msg.Receipt.Logs), len(receipt.Logs))
	for i := range msg.Receipt.Logs {
		assert.Equal(t, msg.Receipt.Logs[i].Address.String(), receipt.Logs[i].Address.String())
		assert.Equal(t, len(msg.Receipt.Logs[i].Topics), len(receipt.Logs[i].Topics))
		for j := range msg.Receipt.Logs[i].Topics {
			assert.Equal(t, msg.Receipt.Logs[i].Topics[j].String(), receipt.Logs[i].Topics[j].String())
		}
		assert.Equal(t, string(msg.Receipt.Logs[i].Data), string(receipt.Logs[i].Data))
	}
	assert.Equal(t, msg.Receipt.TxHash, receipt.TxHash)
	assert.Equal(t, msg.Receipt.ContractAddress.String(), receipt.ContractAddress.String())
	assert.Equal(t, msg.Receipt.GasUsed, receipt.GasUsed)
	assert.Equal(t, msg.Receipt.BlockHash, receipt.BlockHash)
	assert.Equal(t, msg.Receipt.BlockNumber, receipt.BlockNumber)
	assert.Equal(t, msg.Receipt.TransactionIndex, receipt.TransactionIndex)
}

func AssertInnerTxs(t *testing.T, msg kafkaTypes.TransactionMessage, innerTxs []*types.InnerTx) {
	assert.Equal(t, len(msg.InnerTxs), len(innerTxs))
	for i := range msg.InnerTxs {
		assert.Equal(t, msg.InnerTxs[i].Dept.String(), innerTxs[i].Dept.String())
		assert.Equal(t, msg.InnerTxs[i].InternalIndex.String(), innerTxs[i].InternalIndex.String())
		assert.Equal(t, msg.InnerTxs[i].CallType, innerTxs[i].CallType)
		assert.Equal(t, msg.InnerTxs[i].Name, innerTxs[i].Name)
		assert.Equal(t, msg.InnerTxs[i].TraceAddress, innerTxs[i].TraceAddress)
		assert.Equal(t, msg.InnerTxs[i].CodeAddress, innerTxs[i].CodeAddress)
		assert.Equal(t, msg.InnerTxs[i].From, innerTxs[i].From)
		assert.Equal(t, msg.InnerTxs[i].To, innerTxs[i].To)
		assert.Equal(t, msg.InnerTxs[i].Input, innerTxs[i].Input)
		assert.Equal(t, msg.InnerTxs[i].Output, innerTxs[i].Output)
		assert.Equal(t, msg.InnerTxs[i].IsError, innerTxs[i].IsError)
		assert.Equal(t, msg.InnerTxs[i].Gas, innerTxs[i].Gas)
		assert.Equal(t, msg.InnerTxs[i].GasUsed, innerTxs[i].GasUsed)
		assert.Equal(t, msg.InnerTxs[i].Value, innerTxs[i].Value)
		assert.Equal(t, msg.InnerTxs[i].ValueWei, innerTxs[i].ValueWei)
		assert.Equal(t, msg.InnerTxs[i].CallValueWei, innerTxs[i].CallValueWei)
		assert.Equal(t, msg.InnerTxs[i].Error, innerTxs[i].Error)
	}
}

func AssertChangeseet(t *testing.T, msg kafkaTypes.TransactionMessage, changeset *realtimeTypes.Changeset) {
	assert.Equal(t, len(msg.Changeset.DeletedAccounts), len(changeset.DeletedAccounts))
	assert.Equal(t, len(msg.Changeset.BalanceChanges), len(changeset.BalanceChanges))
	assert.Equal(t, len(msg.Changeset.NonceChanges), len(changeset.NonceChanges))
	assert.Equal(t, len(msg.Changeset.CodeHashChanges), len(changeset.CodeHashChanges))
	assert.Equal(t, len(msg.Changeset.CodeChanges), len(changeset.CodeChanges))
	assert.Equal(t, len(msg.Changeset.StorageChanges), len(changeset.StorageChanges))

	for k, v := range msg.Changeset.DeletedAccounts {
		assert.Equal(t, v, changeset.DeletedAccounts[k])
	}
	for k, v := range changeset.DeletedAccounts {
		assert.Equal(t, v, msg.Changeset.DeletedAccounts[k])
	}

	for k, v := range msg.Changeset.BalanceChanges {
		assert.Equal(t, *v, *changeset.BalanceChanges[k])
	}
	for k, v := range changeset.BalanceChanges {
		assert.Equal(t, *v, *msg.Changeset.BalanceChanges[k])
	}

	for k, v := range msg.Changeset.NonceChanges {
		assert.Equal(t, v, changeset.NonceChanges[k])
	}
	for k, v := range changeset.NonceChanges {
		assert.Equal(t, v, msg.Changeset.NonceChanges[k])
	}

	for k, v := range msg.Changeset.CodeHashChanges {
		assert.Equal(t, v, changeset.CodeHashChanges[k])
	}
	for k, v := range changeset.CodeHashChanges {
		assert.Equal(t, v, msg.Changeset.CodeHashChanges[k])
	}

	for k, v := range msg.Changeset.CodeChanges {
		assert.Equal(t, hex.EncodeToString(v), hex.EncodeToString(changeset.CodeChanges[k]))
	}
	for k, v := range changeset.CodeChanges {
		assert.Equal(t, v, msg.Changeset.CodeChanges[k])
	}

	for k, v := range msg.Changeset.StorageChanges {
		for item, itemValue := range v {
			assert.Equal(t, itemValue, changeset.StorageChanges[k][item])
		}
	}
	for k, v := range changeset.StorageChanges {
		for item, itemValue := range v {
			assert.Equal(t, itemValue, msg.Changeset.StorageChanges[k][item])
		}
	}

}

func GetTestChainConfig(chainID uint64) *params.ChainConfig {
	return params.TestChainConfig
}

func getPrivateKeyFromHex(hexString string) (*ecdsa.PrivateKey, error) {
	keyBytes, err := hexutil.Decode(hexString)
	if err != nil {
		return nil, err
	}

	// Convert bytes to ECDSA private key
	privateKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}
