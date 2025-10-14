package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	"github.com/ethereum/go-ethereum/realtime/relayer/streamer"
)

func main() {
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true)))

	config := DefaultConfig()
	config.ParseFlags()

	kafkaConfig := &kafka.KafkaConfig{
		BootstrapServers: []string{config.KafkaURL},
		BlockTopic:       config.BlockTopic,
		TxTopic:          config.TxTopic,
		ErrorTopic:       config.ErrorTopic,
		ClientID:         config.ClientID,
		GroupID:          config.GroupID,
	}

	streamerConfig := &streamer.StreamerConfig{
		Port:                   uint16(config.Port),
		WriteTimeout:           time.Duration(config.WriteTimeout) * time.Second,
		InactivityTimeout:      time.Duration(config.InactivityTimeout) * time.Second,
		HeartbeatCheckInterval: time.Duration(config.HeartbeatCheckInterval) * time.Second,
	}

	// Create and start server
	server := streamer.NewServer(streamerConfig, kafkaConfig)

	// Start server
	if err := server.Start(); err != nil {
		log.Error("Failed to start server", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Realtime relayer service started. ", "port: ", config.Port)
	<-sigChan

	log.Info("Shutting down relayer service...")
}
