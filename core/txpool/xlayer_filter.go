package txpool

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/eth/gasprice/xlayer"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/gasprice"

	"github.com/ethereum/go-ethereum/log"
)

// XLayerFilter is an IngressFilter that filters transactions based on X Layer gas price requirements
type XLayerFilter struct {
	config      gasprice.Config
	gpricer     xlayer.L2GasPricer
	minGasPrice *big.Int
	blockchain  BlockchainReader
	xlayerCache *xlayer.GasPriceCache
}

// BlockchainReader interface for accessing blockchain data
type BlockchainReader interface {
	CurrentBlock() *types.Header
}

// NewXLayerFilter creates a new XLayer IngressFilter
func NewXLayerFilter(config gasprice.Config, gpricer xlayer.L2GasPricer, blockchain BlockchainReader, xlayerCache *xlayer.GasPriceCache) *XLayerFilter {
	minGasPrice := config.XLayer.Default
	if minGasPrice == nil || minGasPrice.Int64() <= 0 {
		minGasPrice = gasprice.DefaultXLayerPrice
	}

	return &XLayerFilter{
		config:      config,
		gpricer:     gpricer,
		minGasPrice: minGasPrice,
		blockchain:  blockchain,
		xlayerCache: xlayerCache,
	}
}

// FilterTx implements IngressFilter.FilterTx
// It filters transactions based on X Layer gas price requirements
func (f *XLayerFilter) FilterTx(ctx context.Context, tx *types.Transaction) bool {
	// Skip filtering if XLayer is not configured
	if f.config.XLayer.Type == "" {
		return true
	}

	// Get the minimum gas price from xlayer_cache
	minPrice := f.xlayerCache.GetLatestRawGP()
	if minPrice == nil {
		log.Warn("XLayerFilter: Unable to get minimum gas price, allowing transaction")
		return true
	}

	// Check if transaction meets minimum gas price requirement
	txGasPrice := tx.GasPrice()
	if tx.Type() == types.DynamicFeeTxType {
		// For EIP-1559 transactions, check fee cap
		// min of tip + base fee and fee cap
		baseFee := big.NewInt(0)
		if f.blockchain != nil {
			if currentHeader := f.blockchain.CurrentBlock(); currentHeader != nil {
				baseFee = currentHeader.BaseFee
			}
		}

		// Calculate effective gas price: min(tip + baseFee, feeCap)
		tipPlusBaseFee := new(big.Int).Add(tx.GasTipCap(), baseFee)
		if tipPlusBaseFee.Cmp(tx.GasFeeCap()) < 0 {
			txGasPrice = tipPlusBaseFee
		} else {
			txGasPrice = tx.GasFeeCap()
		}
		log.Debug("XLayerFilter: Transaction effective gas price",
			"txHash", tx.Hash().Hex(),
			"txGasPrice=min(baseFee+tip, feeCap)", txGasPrice.String(),
			"baseFee", baseFee.String(),
			"tip", tx.GasTipCap().String(),
			"feeCap", tx.GasFeeCap().String())
	}

	if txGasPrice.Cmp(minPrice) < 0 {
		log.Info("XLayerFilter: Transaction rejected due to insufficient gas price",
			"txHash", tx.Hash().Hex(),
			"txGasPrice", txGasPrice.String(),
			"minGasPrice", minPrice.String())
		return false
	}

	// Additional filtering based on XLayer type
	switch f.config.XLayer.Type {
	case gasprice.GasPriceFollowerType:
		return f.filterFollowerTx(ctx, tx)
	case gasprice.GasPriceFixedType:
		return f.filterFixedTx(ctx, tx)
	default:
		// Default type - no additional filtering
		return true
	}
}

// filterFollowerTx applies additional filtering for follower type
func (f *XLayerFilter) filterFollowerTx(ctx context.Context, tx *types.Transaction) bool {
	// For follower type, we could add additional logic based on L1 gas price
	// For now, just check if the transaction meets basic requirements
	return true
}

// filterFixedTx applies additional filtering for fixed type
func (f *XLayerFilter) filterFixedTx(ctx context.Context, tx *types.Transaction) bool {
	// For fixed type, ensure the gas price is within acceptable bounds
	// This could include checks against the configured USDT gas price
	return true
}

// GetMinGasPrice returns the minimum gas price required by this filter
func (f *XLayerFilter) GetMinGasPrice() *big.Int {
	return new(big.Int).Set(f.minGasPrice)
}

// UpdateConfig updates the filter configuration
func (f *XLayerFilter) UpdateConfig(config gasprice.Config) {
	f.config = config
	if config.XLayer.Default != nil && config.XLayer.Default.Int64() > 0 {
		f.minGasPrice = new(big.Int).Set(config.XLayer.Default)
	}
}
