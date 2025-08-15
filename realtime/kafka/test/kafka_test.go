package test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
	"gotest.tools/v3/assert"
)

var (
	difficulty, _ = new(big.Int).SetString("8398142613866510000000000000000000000000000000", 10)
	blockHeader   = &types.Header{
		ParentHash:  libcommon.HexToHash("0x8b00fcf1e541d371a3a1b79cc999a85cc3db5ee5637b5159646e1acd3613fd15"),
		UncleHash:   libcommon.HexToHash("1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
		Coinbase:    libcommon.HexToAddress("0x571846e42308df2dad8ed792f44a8bfddf0acb4d"),
		Root:        libcommon.HexToHash("0x351780124dae86b84998c6d4fe9a88acfb41b4856b4f2c56767b51a4e2f94dd4"),
		TxHash:      libcommon.HexToHash("0x6a35133fbff7ea2cb5ee7635c9fb623f96d31d689d806a2bfe40a2b1d90ee99c"),
		ReceiptHash: libcommon.HexToHash("0x324f54860e214ea896ea7a05bda30f85541be3157de77a9059a04fdb1e86badd"),
		Difficulty:  difficulty,
		Number:      big.NewInt(24679923),
		GasLimit:    30_000_000,
		GasUsed:     3_074_345,
		Time:        1666343339,
		Extra:       common.FromHex("0x1234"),
		BaseFee:     big.NewInt(7_000_000_000),
	}
	testHash = libcommon.HexToHash("0x1234567890abcdef")
)

func TestKafka(t *testing.T) {
	cfg := kafka.KafkaConfig{
		BootstrapServers: []string{"0.0.0.0:9095"},
		BlockTopic:       "xlayer-test-block",
		TxTopic:          "xlayer-test-tx",
		ErrorTopic:       "xlayer-test-error",
		ClientID:         "xlayer-test-consumer",
		GroupID:          "xlayer-test-consumer-1",
	}
	producer, err := kafka.NewKafkaProducer(cfg, context.Background(), nil)
	assert.NilError(t, err)

	currBlockHeader := types.CopyHeader(blockHeader)
	for i := 1; i <= 10; i++ {
		err = producer.SendKafkaTransaction(uint64(i), signedLegacyTx, txReceipt, txInnerTxs, txChangeset)
		assert.NilError(t, err)

		var prevBlockHeader *types.Header
		if i != 1 {
			prevBlockHeader = types.CopyHeader(currBlockHeader)
		}
		currBlockHeader.Number = big.NewInt(int64(i))
		blockMsg := kafkaTypes.BlockMessage{
			Header: currBlockHeader,
			PrevBlockInfo: &realtimeTypes.BlockInfo{
				Header:  prevBlockHeader,
				TxCount: int64(i),
				Hash:    testHash,
			},
		}
		assert.NilError(t, err)
		err = producer.SendKafkaBlockMessage(blockMsg)
		assert.NilError(t, err)

		err = producer.SendKafkaErrorTrigger(uint64(i))
		assert.NilError(t, err)
	}

	for i := 11; i <= 20; i++ {
		err = producer.SendKafkaTransaction(uint64(i), signedAccessListTx, txReceipt, txInnerTxs, txChangeset)
		assert.NilError(t, err)
	}

	err = producer.Close()
	assert.NilError(t, err)

	consumer, err := kafka.NewKafkaConsumer(cfg, false)
	assert.NilError(t, err)
	ctx, ctxWithCancel := context.WithCancel(context.Background())
	headersChan := make(chan kafkaTypes.BlockMessage, 20)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, 20)
	errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, 20)
	errorChan := make(chan error, 10)
	go consumer.ConsumeKafka(ctx, headersChan, txMsgsChan, errorMsgsChan, errorChan)

	// Verify tx messages
	for i := 1; i <= 10; i++ {
		select {
		case err := <-errorChan:
			t.Fatalf("Received error from consumer: %v", err)
		case txMsg := <-txMsgsChan:
			AssertCommonTx(t, txMsg, signedLegacyTx, uint64(i), types.LegacyTxType)
			AssertReceipt(t, txMsg, txReceipt)
			AssertInnerTxs(t, txMsg, txInnerTxs)
			AssertChangeseet(t, txMsg, txChangeset)
		}
	}

	for i := 11; i <= 20; i++ {
		select {
		case err := <-errorChan:
			t.Fatalf("Received error from consumer: %v", err)
		case txMsg := <-txMsgsChan:
			AssertCommonTx(t, txMsg, signedAccessListTx, uint64(i), types.AccessListTxType)
			AssertAccessList(t, txMsg.AccessList)
			AssertReceipt(t, txMsg, txReceipt)
			AssertInnerTxs(t, txMsg, txInnerTxs)
			AssertChangeseet(t, txMsg, txChangeset)
		}
	}

	// Verify header messages
	currBlockHeader = types.CopyHeader(blockHeader)
	for i := 1; i <= 10; i++ {
		select {
		case err := <-errorChan:
			t.Fatalf("Received error from consumer: %v", err)
		case rcvHeader := <-headersChan:
			var prevBlockHeader *types.Header
			if i != 1 {
				prevBlockHeader = types.CopyHeader(currBlockHeader)
			}
			currBlockHeader.Number = big.NewInt(int64(i))
			header, prevBlockInfo, err := rcvHeader.GetBlockInfo()
			assert.NilError(t, err)
			AssertHeader(t, currBlockHeader, header)
			AssertHeader(t, prevBlockHeader, prevBlockInfo.Header)
			assert.Equal(t, prevBlockInfo.TxCount, int64(i))
			assert.Equal(t, prevBlockInfo.Hash, testHash)
		}
	}

	// Verify error trigger messages
	for i := 1; i <= 10; i++ {
		select {
		case err := <-errorChan:
			t.Fatalf("Received error from consumer: %v", err)
		case errorMsg := <-errorMsgsChan:
			assert.Equal(t, errorMsg.BlockNumber, uint64(i))
		}
	}

	ctxWithCancel()
	err = consumer.Close()
	assert.NilError(t, err)
}
