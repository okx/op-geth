package ethconfig

import "github.com/ethereum/go-ethereum/okpay"

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	IsSequencer bool              `toml:",omitempty"`
	OkPay       okpay.OkPayConfig `toml:",omitempty"`
}
