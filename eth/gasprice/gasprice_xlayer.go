package gasprice

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/params"
)

// XLayer gas price types
const (
	GasPriceDefaultType  = "default"  // Default gas price from config
	GasPriceFollowerType = "follower" // Calculate gas price based on L1 gas price
	GasPriceFixedType    = "fixed"    // Fixed gas price in USDT
)

// XLayerGasPriceConfig is the X Layer gas price config
type XLayerGasPriceConfig struct {
	Type         string        `toml:",omitempty"`
	UpdatePeriod time.Duration `toml:",omitempty"`
	Factor       float64       `toml:",omitempty"`
	KafkaURL     string        `toml:",omitempty"`
	Topic        string        `toml:",omitempty"`
	GroupID      string        `toml:",omitempty"`
	Username     string        `toml:",omitempty"`
	Password     string        `toml:",omitempty"`
	RootCAPath   string        `toml:",omitempty"`
	L1CoinId     int           `toml:",omitempty"`
	L2CoinId     int           `toml:",omitempty"`
	// DefaultL1CoinPrice is the L1 token's coin price
	DefaultL1CoinPrice float64 `toml:",omitempty"`
	// DefaultL2CoinPrice is the native token's coin price
	DefaultL2CoinPrice float64 `toml:",omitempty"`
	GasPriceUsdt       float64 `toml:",omitempty"`

	CongestionThreshold int `toml:",omitempty"`

	// Default is the default gas price for X Layer
	Default *big.Int `toml:",omitempty"`
}

var (
	DefaultXLayerPrice = big.NewInt(1 * params.GWei)
)
