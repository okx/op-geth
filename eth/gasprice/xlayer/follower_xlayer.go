package xlayer

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/eth/gasprice"

	"github.com/ethereum/go-ethereum/log"
)

// FollowerGasPrice struct.
type FollowerGasPrice struct {
	cfg       gasprice.Config
	ctx       context.Context
	lastRawGP *big.Int
	kafkaPrc  *KafkaProcessor
	gasCache  *GasPriceCache
}

// newFollowerGasPriceSuggester inits l2 follower gas price suggester which is based on the l1 gas price.
func newFollowerGasPriceSuggester(ctx context.Context, cfg gasprice.Config) *FollowerGasPrice {
	return &FollowerGasPrice{
		cfg:       cfg,
		ctx:       ctx,
		lastRawGP: new(big.Int).Set(cfg.XLayer.Default),
		kafkaPrc:  newKafkaProcessor(cfg.XLayer, ctx),
		gasCache:  NewGasPriceCache(),
	}
}

// UpdateGasPriceAvg updates the gas price in wei.
func (f *FollowerGasPrice) UpdateGasPriceAvg(l1GasPrice *big.Int) {
	if big.NewInt(0).Cmp(l1GasPrice) == 0 {
		log.Warn("gas price 0 received. Skipping update...")
		return
	}

	// Apply gasPrice factor
	factor := big.NewFloat(0).SetFloat64(f.cfg.XLayer.Factor)
	res := new(big.Float).Mul(factor, big.NewFloat(0).SetInt(l1GasPrice))

	// Get L1 and L2 coin prices
	l1CoinPrice, l2CoinPrice := f.kafkaPrc.GetL1L2CoinPrice()
	log.Debug("UpdateGasPriceAvg", "l1CoinPrice", l1CoinPrice, "l2CoinPrice", l2CoinPrice)
	if l1CoinPrice < minUSDTPrice {
		log.Warn("update gas price average failed, the L1 native coin price is too small")
		return
	}
	if l2CoinPrice < minUSDTPrice {
		log.Warn("update gas price average failed, the L2 native coin price is too small")
		return
	}

	// Convert L1 gasPrice in Eth to L2 gasPrice in OKB
	res = new(big.Float).Mul(big.NewFloat(0).SetFloat64(l1CoinPrice/l2CoinPrice), res)
	log.Debug(fmt.Sprintf("L2 pre gas price value: %s. L1 coin price: %f. L2 coin price: %f", res.String(), l1CoinPrice, l2CoinPrice))

	// Check for min/max L2 gasPrice
	result := new(big.Int)
	res.Int(result)
	minGasPrice := new(big.Int).Set(f.cfg.XLayer.Default)
	if minGasPrice.Cmp(result) == 1 { // minGasPrice > result
		log.Info(fmt.Sprintf("Follower mode, setting DefaultGasPrice for L2: %s, result:%v", f.cfg.XLayer.Default.String(), result.String()))
		result = minGasPrice
	}
	maxGasPrice := new(big.Int).Set(f.cfg.MaxPrice)
	if maxGasPrice.Int64() > 0 && result.Cmp(maxGasPrice) == 1 { // result > maxGasPrice
		log.Warn("setting MaxGasPriceWei for L2")
		result = maxGasPrice
	}
	var truncateValue *big.Int
	log.Debug(fmt.Sprintf("Full L2 gas price value: %s. Length: %d", result.String(), len(result.String())))
	numLength := len(result.String())
	if numLength > 3 { //nolint:gomnd
		aux := "%0" + strconv.Itoa(numLength-3) + "d" //nolint:gomnd
		var ok bool
		value := result.String()[:3] + fmt.Sprintf(aux, 0)
		truncateValue, ok = new(big.Int).SetString(value, 10)
		if !ok {
			log.Error(fmt.Sprintf("error converting: %s", value))
		}
	} else {
		truncateValue = result
	}

	// Cache L2 gasPrice calculated
	if truncateValue != nil {
		log.Info(fmt.Sprintf("Set l2 raw gas price: %d", truncateValue.Uint64()))
		f.lastRawGP = truncateValue
	} else {
		log.Error("nil value detected. Skipping...")
	}
}

func (f *FollowerGasPrice) UpdateConfig(c gasprice.Config) {
	f.cfg = c
}

func (f *FollowerGasPrice) GetLastRawGP() *big.Int {
	return f.lastRawGP
}

func (f *FollowerGasPrice) GetConfig() gasprice.Config {
	return f.cfg
}

func (f *FollowerGasPrice) GetCtx() context.Context {
	return f.ctx
}

// GetGasCache returns the gas price cache
func (f *FollowerGasPrice) GetGasCache() *GasPriceCache {
	return f.gasCache
}
