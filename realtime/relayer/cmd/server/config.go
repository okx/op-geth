package main

import "flag"

type Config struct {
	Port                   uint
	KafkaURL               string
	BlockTopic             string
	TxTopic                string
	ErrorTopic             string
	ClientID               string
	GroupID                string
	WriteTimeout           int
	InactivityTimeout      int
	HeartbeatCheckInterval int
}

func (c *Config) ParseFlags() {
	flag.UintVar((&c.Port), "port", c.Port, "WebSocket server port")
	flag.StringVar(&c.KafkaURL, "kafka", c.KafkaURL, "Kafka bootstrap servers")
	flag.StringVar(&c.BlockTopic, "block-topic", c.BlockTopic, "Block topic")
	flag.StringVar(&c.TxTopic, "tx-topic", c.TxTopic, "Transaction topic")
	flag.StringVar(&c.ErrorTopic, "error-topic", c.ErrorTopic, "Error topic")
	flag.StringVar(&c.ClientID, "client-id", c.ClientID, "Client ID")
	flag.StringVar(&c.GroupID, "group-id", c.GroupID, "Group ID")
	flag.IntVar(&c.WriteTimeout, "write-timeout", c.WriteTimeout, "Write timeout")
	flag.IntVar(&c.InactivityTimeout, "inactivity-timeout", c.InactivityTimeout, "Inactivity timeout")
	flag.IntVar(&c.HeartbeatCheckInterval, "heartbeat-check-interval", c.HeartbeatCheckInterval, "Heartbeat check interval")

	flag.Parse()
}

func DefaultConfig() *Config {
	return &Config{
		Port:                   8080,
		KafkaURL:               "localhost:9092",
		BlockTopic:             "xlayer-header",
		TxTopic:                "xlayer-tx",
		ErrorTopic:             "xlayer-error",
		ClientID:               "xlayer-consumer2",
		GroupID:                "xlayer-consumer-2",
		WriteTimeout:           10,
		InactivityTimeout:      60,
		HeartbeatCheckInterval: 30,
	}
}
