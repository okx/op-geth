package test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	"github.com/ethereum/go-ethereum/realtime/relayer/streamclient"
	"github.com/ethereum/go-ethereum/realtime/relayer/streamer"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
	"github.com/stretchr/testify/require"
)

const KafkaLocalHostBootstrapServers = "localhost:9095"

func TestStreamServerClientStartAndStop(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Create stream server config
	streamerConfig := &streamer.StreamerConfig{
		Port:                   8001,
		HeartbeatCheckInterval: 30 * time.Second,
		InactivityTimeout:      5 * time.Minute,
		WriteTimeout:           10 * time.Second,
	}

	kafkaConfig := &kafka.KafkaConfig{
		BootstrapServers: []string{KafkaLocalHostBootstrapServers},
		GroupID:          "test-group",
		ClientID:         "test-client",
		BlockTopic:       "xlayer-header",
		TxTopic:          "xlayer-tx",
		ErrorTopic:       "xlayer-error",
	}

	// Start server
	server := streamer.NewServer(streamerConfig, kafkaConfig)
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)
	log.Info("Stream server started successfully")

	// Create client
	clientConfig := &streamclient.StreamClientConfig{
		RealtimeStreamerUrl:    "localhost:8001",
		RealtimeStreamerUseTLS: false,
	}

	ctx := context.Background()
	client := streamclient.NewStreamClient(ctx, clientConfig, kafkaConfig)
	err = client.Start()
	require.NoError(t, err)
	defer client.Stop()

	time.Sleep(100 * time.Millisecond)
	log.Info("Stream client started successfully")
}

func TestServerCanConsumeRawKafkaMessages(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	kafkaConfig := &kafka.KafkaConfig{
		BootstrapServers: []string{KafkaLocalHostBootstrapServers},
		GroupID:          "xlayer-consumer-3",
		ClientID:         "xlayer-consumer-3",
		BlockTopic:       "xlayer-header",
		TxTopic:          "xlayer-tx",
		ErrorTopic:       "xlayer-error",
	}

	rawMsgsChan := make(chan *sarama.ConsumerMessage, streamer.MaxMessageChanSize)
	errorChan := make(chan error, 1)

	kafkaConsumer, err := kafka.NewKafkaConsumer(*kafkaConfig, true)
	if err != nil {
		log.Warn("[Realtime] Failed to initialize kafka consumer", "error", err)
		return
	}

	go kafkaConsumer.ConsumeRawKafka(context.Background(), rawMsgsChan, errorChan)

	select {
	case rawMsg := <-rawMsgsChan:
		require.NotEmpty(t, rawMsg)
		log.Info("Raw message processed successfully")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestServerBroadcastAndClientProcessMessage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Setup server and client
	streamerConfig := &streamer.StreamerConfig{
		Port:                   8002,
		HeartbeatCheckInterval: 30 * time.Second,
		InactivityTimeout:      5 * time.Minute,
		WriteTimeout:           10 * time.Second,
	}

	kafkaConfig := &kafka.KafkaConfig{
		BootstrapServers: []string{}, // Empty kafka to test server broadcast message
		GroupID:          "test-group",
		ClientID:         "test-client",
		BlockTopic:       "xlayer-header",
		TxTopic:          "xlayer-tx",
		ErrorTopic:       "xlayer-error",
	}

	server := streamer.NewServer(streamerConfig, kafkaConfig)
	err := server.Start()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Create client
	clientConfig := &streamclient.StreamClientConfig{
		RealtimeStreamerUrl:          "localhost:8002",
		RealtimeStreamerUseTLS:       false,
		RealtimeStreamerReadTimeout:  30 * time.Second,
		RealtimeStreamerWriteTimeout: 30 * time.Second,
		RealtimeStreamerRetryDelay:   1 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := streamclient.NewStreamClient(ctx, clientConfig, kafkaConfig)
	err = client.Start()
	require.NoError(t, err)
	defer client.Stop()

	// Create channels for message processing
	blockMsgsChan := make(chan realtimeTypes.BlockInfo, 10)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, 10)
	errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, 10)
	errorChan := make(chan error, 1)

	// Start message processing
	go client.ConsumeRealtime(blockMsgsChan, txMsgsChan, errorMsgsChan, errorChan)

	zeroHash := func() string { return "0x" + strings.Repeat("0", 64) }   // 32 bytes
	zeroBloom := func() string { return "0x" + strings.Repeat("0", 512) } // 256 bytes

	msg, err := json.Marshal(map[string]any{
		"header": map[string]any{
			"number":           "0x7b",
			"hash":             zeroHash(),
			"parentHash":       zeroHash(),
			"timestamp":        "0x0",
			"sha3Uncles":       zeroHash(),
			"stateRoot":        zeroHash(),
			"transactionsRoot": zeroHash(),
			"receiptsRoot":     zeroHash(),
			"logsBloom":        zeroBloom(),
			"difficulty":       "0x0",
			"gasLimit":         "0x0",
			"gasUsed":          "0x0",
			"timeStamp":        "0x0",
			"extraData":        []byte{},
		},
		"hash": zeroHash(),
	})

	// Create test message
	testMsg := &sarama.ConsumerMessage{
		Topic: "xlayer-header",
		Value: msg,
	}

	serializeMessage := func(typeFlag streamer.TypeFlag, value []byte) []byte {
		lengthPrefix := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthPrefix, uint32(len(value))+1)
		return append(append(lengthPrefix, byte(typeFlag)), value...)
	}

	fromKafkaMessage := func(kafkaMessage *sarama.ConsumerMessage) []byte {
		typeFlag := streamer.TopicBlock
		value := kafkaMessage.Value

		return serializeMessage(typeFlag, value)
	}

	// Broadcast message
	time.Sleep(5 * time.Second)
	server.BroadcastMessage(fromKafkaMessage(testMsg))

	// Wait for message to be processed
	select {
	case blockMsg := <-blockMsgsChan:
		require.Equal(t, big.NewInt(123), blockMsg.Header.Number)
		fmt.Printf("Block message processed successfully: %v\n", blockMsg.Header.Number)
		server.Stop()
		client.Stop()
	case err := <-errorChan:
		t.Fatalf("Error processing message: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestClientDisconnectionOnMissingHeartbeats(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	streamerConfig := &streamer.StreamerConfig{
		Port:                   8003,
		HeartbeatCheckInterval: 1 * time.Second,
		InactivityTimeout:      5 * time.Second,
		WriteTimeout:           5 * time.Second,
	}

	kafkaConfig := &kafka.KafkaConfig{
		BootstrapServers: []string{KafkaLocalHostBootstrapServers},
		GroupID:          "test-group",
		ClientID:         "test-client",
		BlockTopic:       "xlayer-header",
		TxTopic:          "xlayer-tx",
		ErrorTopic:       "xlayer-error",
	}

	server := streamer.NewServer(streamerConfig, kafkaConfig)
	err := server.Start()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Connect client
	clientConfig := &streamclient.StreamClientConfig{
		RealtimeStreamerUrl:          "localhost:8003",
		RealtimeStreamerUseTLS:       false,
		RealtimeStreamerReadTimeout:  5 * time.Second,
		RealtimeStreamerWriteTimeout: 5 * time.Second,
		RealtimeStreamerRetryDelay:   1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := streamclient.NewStreamClient(ctx, clientConfig, kafkaConfig)

	// Start client
	go func() {
		blockMsgsChan := make(chan realtimeTypes.BlockInfo, 10)
		txMsgsChan := make(chan kafkaTypes.TransactionMessage, 10)
		errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, 10)
		errorChan := make(chan error, 1)

		client.ConsumeRealtime(blockMsgsChan, txMsgsChan, errorMsgsChan, errorChan)
	}()

	// Wait for connection
	time.Sleep(2 * time.Second)
	require.Equal(t, 1, server.GetConnectionsCount())

	cancel()

	// Wait for server to detect missing heartbeats and disconnect
	time.Sleep(8 * time.Second)
	require.Equal(t, 0, server.GetConnectionsCount())
}

func TestClientReconnectionToStoppedServer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	streamerConfig := &streamer.StreamerConfig{
		Port:                   8004,
		HeartbeatCheckInterval: 1 * time.Second,
		InactivityTimeout:      5 * time.Second,
		WriteTimeout:           5 * time.Second,
	}

	kafkaConfig := &kafka.KafkaConfig{
		BootstrapServers: []string{KafkaLocalHostBootstrapServers},
		GroupID:          "test-group",
		ClientID:         "test-client",
		BlockTopic:       "xlayer-header",
		TxTopic:          "xlayer-tx",
		ErrorTopic:       "xlayer-error",
	}

	server := streamer.NewServer(streamerConfig, kafkaConfig)
	err := server.Start()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Connect client
	clientConfig := &streamclient.StreamClientConfig{
		RealtimeStreamerUrl:          "localhost:8004",
		RealtimeStreamerUseTLS:       false,
		RealtimeStreamerReadTimeout:  30 * time.Second,
		RealtimeStreamerWriteTimeout: 30 * time.Second,
		RealtimeStreamerRetryDelay:   1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := streamclient.NewStreamClient(ctx, clientConfig, kafkaConfig)

	errorChan := make(chan error, 1)
	// Start client
	go func() {
		blockMsgsChan := make(chan realtimeTypes.BlockInfo, 10)
		txMsgsChan := make(chan kafkaTypes.TransactionMessage, 10)
		errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, 10)

		client.ConsumeRealtime(blockMsgsChan, txMsgsChan, errorMsgsChan, errorChan)
	}()

	// Wait for connection
	time.Sleep(2 * time.Second)
	require.Equal(t, 1, server.GetConnectionsCount())

	// stop the server
	server.Stop()

	time.Sleep(2 * time.Second)

	// restart the server
	server = streamer.NewServer(streamerConfig, kafkaConfig)
	err = server.Start()
	require.NoError(t, err)

	time.Sleep(4 * time.Second)
	require.Equal(t, 1, server.GetConnectionsCount())
}
