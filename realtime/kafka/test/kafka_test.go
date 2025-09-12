package test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

var (
	difficulty, _ = new(big.Int).SetString("8398142613866510000000000000000000000000000000", 10)
	blockHeader   = &types.Header{
		ParentHash:  common.HexToHash("0x8b00fcf1e541d371a3a1b79cc999a85cc3db5ee5637b5159646e1acd3613fd15"),
		UncleHash:   common.HexToHash("1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
		Coinbase:    common.HexToAddress("0x571846e42308df2dad8ed792f44a8bfddf0acb4d"),
		Root:        common.HexToHash("0x351780124dae86b84998c6d4fe9a88acfb41b4856b4f2c56767b51a4e2f94dd4"),
		TxHash:      common.HexToHash("0x6a35133fbff7ea2cb5ee7635c9fb623f96d31d689d806a2bfe40a2b1d90ee99c"),
		ReceiptHash: common.HexToHash("0x324f54860e214ea896ea7a05bda30f85541be3157de77a9059a04fdb1e86badd"),
		Difficulty:  difficulty,
		Number:      big.NewInt(24679923),
		GasLimit:    30_000_000,
		GasUsed:     3_074_345,
		Time:        1666343339,
		Extra:       common.FromHex("0x1234"),
		BaseFee:     big.NewInt(7_000_000_000),
	}
	testHash  = common.HexToHash("0x1234567890abcdef")
	blockTime = uint64(1000)
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

	err := createKafkaTopics(cfg)
	assert.NilError(t, err)

	producer, err := kafka.NewKafkaProducer(cfg, context.Background(), nil)
	assert.NilError(t, err)

	for i := 1; i <= 10; i++ {
		err = producer.SendKafkaTransaction(uint64(i), blockTime, signedLegacyTx, txReceipt, txInnerTxs, txChangeset)
		assert.NilError(t, err)

		err = producer.SendKafkaBlockInfo(&realtimeTypes.BlockInfo{
			Header:  blockHeader,
			TxCount: int64(i),
			Hash:    testHash,
		})
		assert.NilError(t, err)

		err = producer.SendKafkaErrorTrigger(uint64(i))
		assert.NilError(t, err)
	}

	for i := 11; i <= 20; i++ {
		err = producer.SendKafkaTransaction(uint64(i), blockTime, signedAccessListTx, txReceipt, txInnerTxs, txChangeset)
		assert.NilError(t, err)
	}

	err = producer.Close()
	assert.NilError(t, err)

	consumer, err := kafka.NewKafkaConsumer(cfg, false)
	assert.NilError(t, err)
	ctx, ctxWithCancel := context.WithCancel(context.Background())
	headersChan := make(chan realtimeTypes.BlockInfo, 20)
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
			AssertCommonTxWithoutBlockNumber(t, txMsg, signedLegacyTx, types.LegacyTxType)
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
			AssertCommonTxWithoutBlockNumber(t, txMsg, signedAccessListTx, types.AccessListTxType)
			AssertAccessList(t, txMsg.AccessList)
			AssertReceipt(t, txMsg, txReceipt)
			AssertInnerTxs(t, txMsg, txInnerTxs)
			AssertChangeseet(t, txMsg, txChangeset)
		}
	}

	// Verify header messages
	for i := 1; i <= 10; i++ {
		select {
		case err := <-errorChan:
			t.Fatalf("Received error from consumer: %v", err)
		case rcvHeader := <-headersChan:
			AssertHeader(t, blockHeader, rcvHeader.Header)
			assert.Equal(t, rcvHeader.TxCount, int64(i))
			assert.Equal(t, rcvHeader.Hash, testHash)
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

func TestStressTestKafkaProducer(t *testing.T) {
	cfg := kafka.KafkaConfig{
		BootstrapServers: []string{"0.0.0.0:9095"},
		BlockTopic:       "xlayer-test-block",
		TxTopic:          "xlayer-test-tx",
		ErrorTopic:       "xlayer-test-error",
		ClientID:         "xlayer-test-consumer",
		GroupID:          "xlayer-test-consumer-1",
	}

	err := createKafkaTopics(cfg)
	assert.NilError(t, err)

	successChan := make(chan struct{}, 10000)
	producer, err := kafka.NewKafkaProducer(cfg, context.Background(), successChan)
	assert.NilError(t, err)

	startTime := time.Now()
	for i := 1; i <= 1000; i++ {
		err = producer.SendKafkaTransaction(uint64(i), blockTime, signedLegacyTx, txReceipt, txInnerTxs, txChangeset)
		assert.NilError(t, err)
	}

	// Sending 1000 messages should not be blocking, and should take less than 50ms
	elapsed := time.Since(startTime)
	fmt.Printf("Batch producer send took %s to dispatch 1000 messages\n", elapsed)
	require.Less(t, elapsed, 50*time.Millisecond)

	for i := 0; i < 1000; i++ {
		select {
		case <-successChan:
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout waiting for success message %d", i)
		}
	}
	elapsed = time.Since(startTime)
	fmt.Printf("Producer took %s to send 1000 messages to kafka broker\n", elapsed)
	require.Less(t, elapsed, 100*time.Millisecond)

	err = producer.Close()
	assert.NilError(t, err)
}

// createKafkaTopics creates the required Kafka topics for testing
func createKafkaTopics(config kafka.KafkaConfig) error {
	// Create admin client
	adminClient, err := sarama.NewClusterAdmin(config.BootstrapServers, nil)
	if err != nil {
		return err
	}
	defer adminClient.Close()

	// Define topics to create
	topics := []string{config.BlockTopic, config.TxTopic, config.ErrorTopic}

	for _, topic := range topics {
		// Check if topic already exists
		metadata, err := adminClient.DescribeConfig(sarama.ConfigResource{
			Type: sarama.TopicResource,
			Name: topic,
		})

		if err != nil {
			// Topic doesn't exist, create it
			err = adminClient.CreateTopic(topic, &sarama.TopicDetail{
				NumPartitions:     1,
				ReplicationFactor: 1,
			}, false)
			if err != nil {
				return err
			}
		} else {
			// Topic exists, just verify it's accessible
			_ = metadata
		}
	}
	time.Sleep(1 * time.Second)
	return nil
}
