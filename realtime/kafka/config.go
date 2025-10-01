package kafka

type KafkaConfig struct {
	BootstrapServers []string `toml:",omitempty"`
	BlockTopic       string   `toml:",omitempty"`
	TxTopic          string   `toml:",omitempty"`
	ErrorTopic       string   `toml:",omitempty"`
	ClientID         string   `toml:",omitempty"`
	GroupID          string   `toml:",omitempty"`
}
