package main

import (
	"flag"
)

type Config struct {
	RelayerURL string
	KafkaURL   string
	BlockTopic string
	TxTopic    string
	ErrorTopic string
}

func DefaultConfig() *Config {
	return &Config{
		RelayerURL: "localhost:9093",
		KafkaURL:   "localhost:9092",
		BlockTopic: "xlayer-header",
		TxTopic:    "xlayer-tx",
		ErrorTopic: "xlayer-error",
	}
}

func (c *Config) ParseFlags() {
	flag.StringVar(&c.RelayerURL, "relayer-url", c.RelayerURL, "Relayer server URL")
	flag.StringVar(&c.KafkaURL, "kafka-url", c.KafkaURL, "Kafka bootstrap servers")
	flag.StringVar(&c.BlockTopic, "block-topic", c.BlockTopic, "Block topic")
	flag.StringVar(&c.TxTopic, "tx-topic", c.TxTopic, "Transaction topic")
	flag.StringVar(&c.ErrorTopic, "error-topic", c.ErrorTopic, "Error topic")

	flag.Parse()
}
