package ethconfig

type XLayerP2PConfig struct {
	ETH69Compat bool `toml:",omitempty"`
}

type XLayerConfig struct {
	P2P XLayerP2PConfig
}
