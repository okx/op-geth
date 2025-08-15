package test

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	"gotest.tools/assert"
)

func TestLegacyTx(t *testing.T) {
	// Test from
	blockNumber := uint64(100)
	emptyMsg, err := kafkaTypes.ToKafkaTransactionMessage(signedEmptyTx, emptyTxReceipt, nil, nil, blockNumber)
	assert.NilError(t, err)
	AssertCommonTx(t, emptyMsg, signedEmptyTx, blockNumber, types.LegacyTxType)
	assert.Equal(t, emptyMsg.GasPrice.String(), signedEmptyTx.GasPrice().String())
	AssertReceipt(t, emptyMsg, emptyTxReceipt)
	AssertInnerTxs(t, emptyMsg, nil)

	msg, err := kafkaTypes.ToKafkaTransactionMessage(signedLegacyTx, txReceipt, txInnerTxs, txChangeset, blockNumber)
	assert.NilError(t, err)
	AssertCommonTx(t, msg, signedLegacyTx, blockNumber, types.LegacyTxType)
	assert.Equal(t, msg.GasPrice.String(), signedLegacyTx.GasPrice().String())
	AssertReceipt(t, msg, txReceipt)
	AssertInnerTxs(t, msg, txInnerTxs)
	AssertChangeseet(t, msg, txChangeset)

	// Test to
	convertEmptyTx, convertBlockNumber, err := emptyMsg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	AssertCommonTx(t, emptyMsg, convertEmptyTx, convertBlockNumber, types.LegacyTxType)
	assert.Equal(t, emptyMsg.GasPrice.String(), convertEmptyTx.GasPrice().String())

	convertLegacyTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	AssertCommonTx(t, msg, convertLegacyTx, convertBlockNumber, types.LegacyTxType)
	assert.Equal(t, msg.GasPrice.String(), convertLegacyTx.GasPrice().String())
}

func TestAccessListTx(t *testing.T) {
	// Test from
	blockNumber := uint64(100)
	msg, err := kafkaTypes.ToKafkaTransactionMessage(signedAccessListTx, accessListTxReceipt, txInnerTxs, txChangeset, blockNumber)
	assert.NilError(t, err)
	AssertCommonTx(t, msg, signedAccessListTx, blockNumber, types.AccessListTxType)
	assert.Equal(t, msg.GasPrice.String(), signedAccessListTx.GasPrice().String())
	AssertReceipt(t, msg, accessListTxReceipt)
	AssertInnerTxs(t, msg, txInnerTxs)
	AssertAccessList(t, msg.AccessList)
	AssertChangeseet(t, msg, txChangeset)

	// Test to
	convertAccessListTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	AssertCommonTx(t, msg, convertAccessListTx, convertBlockNumber, types.AccessListTxType)
	assertTxAccessList(t, convertAccessListTx.AccessList())
	assert.Equal(t, msg.GasPrice.String(), convertAccessListTx.GasPrice().String())
}

func TestDynamicFeeTx(t *testing.T) {
	// Test from
	blockNumber := uint64(100)
	msg, err := kafkaTypes.ToKafkaTransactionMessage(signedDynFeeTx, dynFeeTxReceipt, txInnerTxs, txChangeset, blockNumber)
	assert.NilError(t, err)
	AssertCommonTx(t, msg, signedDynFeeTx, blockNumber, types.DynamicFeeTxType)
	assert.Equal(t, msg.Tip.String(), signedDynFeeTx.GasTipCap().String())
	assert.Equal(t, msg.FeeCap.String(), signedDynFeeTx.GasFeeCap().String())
	AssertReceipt(t, msg, dynFeeTxReceipt)
	AssertInnerTxs(t, msg, txInnerTxs)
	AssertAccessList(t, msg.AccessList)
	AssertChangeseet(t, msg, txChangeset)

	// Test to
	convertDynFeeTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	assert.Equal(t, convertBlockNumber, blockNumber)
	AssertCommonTx(t, msg, convertDynFeeTx, convertBlockNumber, types.DynamicFeeTxType)
	assertTxAccessList(t, convertDynFeeTx.AccessList())
	assert.Equal(t, msg.Tip.String(), convertDynFeeTx.GasTipCap().String())
	assert.Equal(t, msg.FeeCap.String(), convertDynFeeTx.GasFeeCap().String())
}

func TestFromBlobTx(t *testing.T) {
	// Test from
	blockNumber := uint64(100)
	msg, err := kafkaTypes.ToKafkaTransactionMessage(signedBlobTx, blobTxReceipt, txInnerTxs, txChangeset, blockNumber)
	assert.NilError(t, err)
	AssertCommonTx(t, msg, signedBlobTx, blockNumber, types.BlobTxType)
	assert.Equal(t, msg.Tip.String(), signedBlobTx.GasTipCap().String())
	assert.Equal(t, msg.FeeCap.String(), signedBlobTx.GasFeeCap().String())
	AssertReceipt(t, msg, blobTxReceipt)
	AssertInnerTxs(t, msg, txInnerTxs)
	AssertAccessList(t, msg.AccessList)

	assert.Equal(t, msg.MaxFeePerBlobGas.String(), "10")
	assert.Equal(t, len(msg.BlobVersionedHashes), 1)
	for _, hash := range msg.BlobVersionedHashes {
		assert.Equal(t, hash, "0x0000000000000000000000000000000000000000000000000000000000000000")
	}

	// Test to
	convertBlobTx, convertBlockNumber, err := msg.GetTransaction()
	assert.NilError(t, err)
	AssertCommonTx(t, msg, convertBlobTx, convertBlockNumber, types.BlobTxType)
	assert.Equal(t, msg.Tip.String(), convertBlobTx.GasTipCap().String())
	assert.Equal(t, msg.FeeCap.String(), convertBlobTx.GasFeeCap().String())
	assertTxAccessList(t, convertBlobTx.AccessList())
}
