package xlayer

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/eth/gasprice"
)

// DefaultGasPricer gas price from config is set.
type DefaultGasPricer struct {
	cfg       gasprice.Config
	ctx       context.Context
	lastRawGP *big.Int
	gasCache  *GasPriceCache
}

// newDefaultGasPriceSuggester inits l2 default gas price suggester.
func newDefaultGasPriceSuggester(ctx context.Context, cfg gasprice.Config) *DefaultGasPricer {
	return &DefaultGasPricer{
		cfg:       cfg,
		ctx:       ctx,
		lastRawGP: new(big.Int).Set(cfg.XLayer.Default),
		gasCache:  NewGasPriceCache(),
	}
}

// UpdateGasPriceAvg not needed for default strategy.
func (d *DefaultGasPricer) UpdateGasPriceAvg(l1GasPrice *big.Int) {
	d.lastRawGP = new(big.Int).Set(d.cfg.XLayer.Default)
}

func (d *DefaultGasPricer) UpdateConfig(c gasprice.Config) {
	d.cfg = c
}

func (d *DefaultGasPricer) GetLastRawGP() *big.Int {
	return d.lastRawGP
}

func (d *DefaultGasPricer) GetConfig() gasprice.Config {
	return d.cfg
}

func (d *DefaultGasPricer) GetCtx() context.Context {
	return d.ctx
}

// GetGasCache returns the gas price cache
func (d *DefaultGasPricer) GetGasCache() *GasPriceCache {
	return d.gasCache
}
