package txpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/eth/gasprice/xlayer"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/gasprice"

	"github.com/ethereum/go-ethereum/params"
)

// mockXLayerGpricer implements L2GasPricer for testing
type mockXLayerGpricer struct {
	minGasPrice *big.Int
	gasCache    *xlayer.GasPriceCache
}

func newMockXLayerGpricer(minGasPrice *big.Int) *mockXLayerGpricer {
	cache := xlayer.NewGasPriceCache()
	cache.SetLatestRawGP(minGasPrice)
	return &mockXLayerGpricer{
		minGasPrice: minGasPrice,
		gasCache:    cache,
	}
}

func (m *mockXLayerGpricer) UpdateGasPriceAvg(l1gp *big.Int) {
	// Mock implementation
}

func (m *mockXLayerGpricer) UpdateConfig(c gasprice.Config) {
	// Mock implementation
}

func (m *mockXLayerGpricer) GetLastRawGP() *big.Int {
	return new(big.Int).Set(m.minGasPrice)
}

func (m *mockXLayerGpricer) GetConfig() gasprice.Config {
	return gasprice.Config{
		XLayer: gasprice.XLayerGasPriceConfig{
			Default: m.minGasPrice,
		},
	}
}

func (m *mockXLayerGpricer) GetCtx() context.Context {
	return context.Background()
}

func (m *mockXLayerGpricer) GetGasCache() *xlayer.GasPriceCache {
	return m.gasCache
}

// mockBlockchainReader implements BlockchainReader for testing
type mockBlockchainReader struct {
	currentHeader *types.Header
}

func newMockBlockchainReader(header *types.Header) *mockBlockchainReader {
	return &mockBlockchainReader{
		currentHeader: header,
	}
}

func (m *mockBlockchainReader) CurrentBlock() *types.Header {
	return m.currentHeader
}

func TestNewXLayerFilter(t *testing.T) {
	tests := []struct {
		name           string
		config         gasprice.Config
		expectedMinGas *big.Int
	}{
		{
			name: "With default XLayer price",
			config: gasprice.Config{
				XLayer: gasprice.XLayerGasPriceConfig{
					Default: big.NewInt(10 * params.GWei),
				},
			},
			expectedMinGas: big.NewInt(10 * params.GWei),
		},
		{
			name: "With zero XLayer price",
			config: gasprice.Config{
				XLayer: gasprice.XLayerGasPriceConfig{
					Default: big.NewInt(0),
				},
			},
			expectedMinGas: gasprice.DefaultXLayerPrice,
		},
		{
			name: "With nil XLayer price",
			config: gasprice.Config{
				XLayer: gasprice.XLayerGasPriceConfig{
					Default: nil,
				},
			},
			expectedMinGas: gasprice.DefaultXLayerPrice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpricer := newMockXLayerGpricer(tt.expectedMinGas)
			blockchain := newMockBlockchainReader(&types.Header{})
			xlayerCache := xlayer.NewGasPriceCache()
			xlayerCache.SetLatestRawGP(tt.expectedMinGas)

			filter := NewXLayerFilter(tt.config, gpricer, blockchain, xlayerCache)
			if filter == nil {
				t.Fatal("Failed to create XLayerFilter")
			}

			if filter.minGasPrice.Cmp(tt.expectedMinGas) != 0 {
				t.Errorf("Expected minGasPrice %s, got %s", tt.expectedMinGas, filter.minGasPrice)
			}
		})
	}
}

func TestXLayerFilterFilterTx_NoXLayerConfig(t *testing.T) {
	config := gasprice.Config{
		XLayer: gasprice.XLayerGasPriceConfig{
			Type: "", // No XLayer type configured
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{})
	xlayerCache := xlayer.NewGasPriceCache()
	xlayerCache.SetLatestRawGP(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// Create a transaction with low gas price
	tx := types.NewTransaction(
		0,                        // nonce
		common.Address{},         // to
		big.NewInt(0),            // value
		21000,                    // gas
		big.NewInt(1*params.Wei), // gasPrice
		nil,                      // data
	)

	// Should allow transaction when XLayer is not configured
	result := filter.FilterTx(context.Background(), tx)
	if !result {
		t.Error("Expected transaction to be allowed when XLayer is not configured")
	}
}

func TestXLayerFilterFilterTx_LegacyTx(t *testing.T) {
	config := gasprice.Config{
		XLayer: gasprice.XLayerGasPriceConfig{
			Type:    gasprice.GasPriceDefaultType,
			Default: big.NewInt(10 * params.GWei),
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{})
	xlayerCache := xlayer.NewGasPriceCache()
	xlayerCache.SetLatestRawGP(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	tests := []struct {
		name     string
		gasPrice *big.Int
		want     bool
	}{
		{
			name:     "Sufficient gas price",
			gasPrice: big.NewInt(15 * params.GWei),
			want:     true,
		},
		{
			name:     "Insufficient gas price",
			gasPrice: big.NewInt(5 * params.GWei),
			want:     false,
		},
		{
			name:     "Equal to minimum gas price",
			gasPrice: big.NewInt(10 * params.GWei),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := types.NewTransaction(
				0,                // nonce
				common.Address{}, // to
				big.NewInt(0),    // value
				21000,            // gas
				tt.gasPrice,      // gasPrice
				nil,              // data
			)

			result := filter.FilterTx(context.Background(), tx)
			if result != tt.want {
				t.Errorf("FilterTx() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestXLayerFilterFilterTx_EIP1559(t *testing.T) {
	config := gasprice.Config{
		XLayer: gasprice.XLayerGasPriceConfig{
			Type:    gasprice.GasPriceDefaultType,
			Default: big.NewInt(10 * params.GWei),
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei) // minGasPrice is 10 GWei
	baseFee := big.NewInt(5 * params.GWei)      // baseFee is 5 GWei
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: baseFee,
	})
	xlayerCache := xlayer.NewGasPriceCache()
	xlayerCache.SetLatestRawGP(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	tests := []struct {
		name      string
		gasTipCap *big.Int
		gasFeeCap *big.Int
		want      bool
	}{
		{
			name:      "High tip, high cap",
			gasTipCap: big.NewInt(8 * params.GWei),  // tip + baseFee = 13 GWei
			gasFeeCap: big.NewInt(20 * params.GWei), // feeCap is 20 GWei, so effective gas price is 13 GWei > minGasPrice
			want:      true,
		},
		{
			name:      "Low tip, low cap",
			gasTipCap: big.NewInt(2 * params.GWei), // tip + baseFee = 7 GWei
			gasFeeCap: big.NewInt(5 * params.GWei), // feeCap is 5 GWei, so effective gas price is 5 GWei < minGasPrice
			want:      false,
		},
		{
			name:      "High tip, low cap",
			gasTipCap: big.NewInt(10 * params.GWei), // tip + baseFee = 15 GWei
			gasFeeCap: big.NewInt(8 * params.GWei),  // feeCap is 8 GWei, so effective gas price is 8 GWei < minGasPrice
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := types.NewTx(&types.DynamicFeeTx{
				ChainID:   big.NewInt(1),
				Nonce:     0,
				GasTipCap: tt.gasTipCap,
				GasFeeCap: tt.gasFeeCap,
				Gas:       21000,
				To:        &common.Address{},
				Value:     big.NewInt(0),
				Data:      nil,
			})

			result := filter.FilterTx(context.Background(), tx)
			if result != tt.want {
				t.Errorf("FilterTx() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestXLayerFilterFilterTx_NilMinPrice(t *testing.T) {
	config := gasprice.Config{
		XLayer: gasprice.XLayerGasPriceConfig{
			Type:    gasprice.GasPriceDefaultType,
			Default: big.NewInt(10 * params.GWei),
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{})
	xlayerCache := xlayer.NewGasPriceCache()
	// Intentionally not setting latest raw GP
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	tx := types.NewTransaction(
		0,                         // nonce
		common.Address{},          // to
		big.NewInt(0),             // value
		21000,                     // gas
		big.NewInt(5*params.GWei), // gasPrice - lower than min
		nil,                       // data
	)

	// Should allow transaction when minimum price is nil
	result := filter.FilterTx(context.Background(), tx)
	if !result {
		t.Error("Expected transaction to be allowed when minimum price is nil")
	}
}

func TestXLayerFilterUpdateConfig(t *testing.T) {
	initialConfig := gasprice.Config{
		XLayer: gasprice.XLayerGasPriceConfig{
			Type:    gasprice.GasPriceDefaultType,
			Default: big.NewInt(10 * params.GWei),
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{})
	xlayerCache := xlayer.NewGasPriceCache()
	xlayerCache.SetLatestRawGP(minGasPrice)
	filter := NewXLayerFilter(initialConfig, gpricer, blockchain, xlayerCache)

	tests := []struct {
		name          string
		newConfig     gasprice.Config
		expectedPrice *big.Int
	}{
		{
			name: "Update to higher price",
			newConfig: gasprice.Config{
				XLayer: gasprice.XLayerGasPriceConfig{
					Type:    gasprice.GasPriceDefaultType,
					Default: big.NewInt(20 * params.GWei),
				},
			},
			expectedPrice: big.NewInt(20 * params.GWei),
		},
		{
			name: "Update to zero price",
			newConfig: gasprice.Config{
				XLayer: gasprice.XLayerGasPriceConfig{
					Type:    gasprice.GasPriceDefaultType,
					Default: big.NewInt(0),
				},
			},
			expectedPrice: big.NewInt(20 * params.GWei), // Should keep previous price
		},
		{
			name: "Update with nil price",
			newConfig: gasprice.Config{
				XLayer: gasprice.XLayerGasPriceConfig{
					Type:    gasprice.GasPriceDefaultType,
					Default: nil,
				},
			},
			expectedPrice: big.NewInt(20 * params.GWei), // Should keep previous price
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter.UpdateConfig(tt.newConfig)
			if filter.GetMinGasPrice().Cmp(tt.expectedPrice) != 0 {
				t.Errorf("Expected minGasPrice %s, got %s", tt.expectedPrice, filter.GetMinGasPrice())
			}
		})
	}
}

func TestXLayerFilterFilterTx_EIP1559_And_UpdatePrice(t *testing.T) {
	config := gasprice.Config{
		XLayer: gasprice.XLayerGasPriceConfig{
			Type:    gasprice.GasPriceDefaultType,
			Default: big.NewInt(10 * params.GWei),
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei) // minGasPrice is 10 GWei
	baseFee := big.NewInt(5 * params.GWei)      // baseFee is 5 GWei
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: baseFee,
	})
	xlayerCache := xlayer.NewGasPriceCache()
	xlayerCache.SetLatestRawGP(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// effective gas price is 13 GWei
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		GasTipCap: big.NewInt(8 * params.GWei),  // tip + baseFee = 13 GWei
		GasFeeCap: big.NewInt(20 * params.GWei), // feeCap is 20 GWei, so effective gas price is 13 GWei
		Gas:       21000,
		To:        &common.Address{},
		Value:     big.NewInt(0),
		Data:      nil,
	})

	tests := []struct {
		name           string
		changeGasPrice *big.Int
		want           bool
	}{
		{
			name:           "Update to higher price",
			changeGasPrice: big.NewInt(20 * params.GWei), // change gas price to 20 GWei > tx.effectiveGasPrice, so it's not allowed
			want:           false,
		},
		{
			name:           "Update to lower price",
			changeGasPrice: big.NewInt(5 * params.GWei), // change gas price to 5 GWei < tx.effectiveGasPrice, so it's allowed
			want:           true,
		},
		{
			name:           "Update to same price",
			changeGasPrice: big.NewInt(13 * params.GWei), // change gas price to 13 GWei = tx.effectiveGasPrice, so it's allowed
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				// update 100 times to ensure the price has been slowly updated to tt.changeGasPrice
				xlayerCache.SetLatestRawGP(tt.changeGasPrice)
			}
			result := filter.FilterTx(context.Background(), tx)
			if result != tt.want {
				t.Errorf("FilterTx() = %v, want %v", result, tt.want)
			}
		})
	}
}
