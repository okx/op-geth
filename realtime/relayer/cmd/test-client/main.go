package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	"github.com/ethereum/go-ethereum/realtime/relayer/streamclient"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

func main() {
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true)))

	config := DefaultConfig()
	config.ParseFlags()

	// Create client configuration
	clientConfig := &streamclient.StreamClientConfig{
		RealtimeStreamerUrl:          config.RelayerURL,
		RealtimeStreamerUseTLS:       false,
		RealtimeStreamerReadTimeout:  30 * time.Second,
		RealtimeStreamerWriteTimeout: 30 * time.Second,
		RealtimeStreamerRetryDelay:   1 * time.Second,
	}

	kafkaConfig := &kafka.KafkaConfig{
		BootstrapServers: []string{config.KafkaURL},
		BlockTopic:       config.BlockTopic,
		TxTopic:          config.TxTopic,
		ErrorTopic:       config.ErrorTopic,
	}

	// Create stream client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := streamclient.NewStreamClient(ctx, clientConfig, kafkaConfig)

	// Create channels for different message types
	blockMsgsChan := make(chan realtimeTypes.BlockInfo, 1000)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, 1000)
	errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, 1000)
	errorChan := make(chan error, 1)

	// Start consuming from relayer
	go client.ConsumeRealtime(blockMsgsChan, txMsgsChan, errorMsgsChan, errorChan)

	// Start message processors
	go processBlockMessages(blockMsgsChan)
	go processTransactionMessages(txMsgsChan)
	go processErrorMessages(errorMsgsChan)
	go processErrors(errorChan)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Info("Shutting down client...")
}

func processBlockMessages(blockMsgsChan <-chan realtimeTypes.BlockInfo) {
	for blockMsg := range blockMsgsChan {
		// Add nil check for Header
		if blockMsg.Header == nil {
			log.Info("Received block message with nil header, skipping")
			continue
		}

		if blockMsg.IsConfirmedBlock() {
			log.Info("Received confirmed block ", "blockNum: ", blockMsg.Header.Number, " hash: ", blockMsg.Header.Hash)
		} else {
			log.Info("Received new block ", "blockNum: ", blockMsg.Header.Number, " hash: ", blockMsg.Header.Hash)
		}
	}
}

func processTransactionMessages(txMsgsChan <-chan kafkaTypes.TransactionMessage) {
	for txMsg := range txMsgsChan {
		log.Info("Received transaction ", "blockNum: ", txMsg.BlockNumber, " txHash: ", txMsg.Hash.Hex())
	}
}

func processErrorMessages(errorMsgsChan <-chan kafkaTypes.ErrorTriggerMessage) {
	for errorMsg := range errorMsgsChan {
		log.Error("Received error trigger ", "blockNum: ", errorMsg.BlockNumber)
	}
}

func processErrors(errorChan <-chan error) {
	for err := range errorChan {
		log.Error("Client error: ", err)
	}
}
