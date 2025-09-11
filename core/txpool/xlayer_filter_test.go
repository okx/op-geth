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
}

func newMockXLayerGpricer(minGasPrice *big.Int) *mockXLayerGpricer {
	return &mockXLayerGpricer{
		minGasPrice: minGasPrice,
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
		Default: m.minGasPrice,
	}
}

func (m *mockXLayerGpricer) GetCtx() context.Context {
	return context.Background()
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

// mockGasPriceCache implements GasPriceCache for testing
type mockGasPriceCache struct {
	latestRawGP *big.Int
}

func newMockGasPriceCache(latestRawGP *big.Int) *xlayer.GasPriceCache {
	// Create a real GasPriceCache and set the latest raw GP
	cache := xlayer.NewGasPriceCache()
	cache.SetLatestRawGP(latestRawGP)
	return cache
}

func TestNewXLayerFilter(t *testing.T) {
	config := gasprice.Config{
		Default: big.NewInt(10 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.DefaultType,
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(0),
	})
	xlayerCache := newMockGasPriceCache(minGasPrice)

	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)
	if filter == nil {
		t.Fatal("Failed to create XLayerFilter")
	}

	if filter.minGasPrice.Cmp(minGasPrice) != 0 {
		t.Errorf("Expected minGasPrice %s, got %s", minGasPrice, filter.minGasPrice)
	}
}

func TestXLayerFilterFilterTx_NoXLayerConfig(t *testing.T) {
	config := gasprice.Config{
		XLayer: gasprice.XLayerConfig{
			Type: "", // No XLayer type configured
		},
	}

	gpricer := newMockXLayerGpricer(big.NewInt(10 * params.GWei))
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(0),
	})
	xlayerCache := newMockGasPriceCache(big.NewInt(10 * params.GWei))
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// Create a transaction with low gas price
	tx := types.NewTransaction(
		0,                        // nonce
		[20]byte{},               // to
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

func TestXLayerFilterFilterTx_DefaultType(t *testing.T) {
	config := gasprice.Config{
		Default: big.NewInt(10 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.DefaultType,
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(0),
	})
	xlayerCache := newMockGasPriceCache(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// Test transaction with sufficient gas price
	tx := types.NewTransaction(
		0,                          // nonce
		[20]byte{},                 // to
		big.NewInt(0),              // value
		21000,                      // gas
		big.NewInt(15*params.GWei), // gasPrice - higher than min
		nil,                        // data
	)

	result := filter.FilterTx(context.Background(), tx)
	if !result {
		t.Error("Expected transaction with sufficient gas price to be allowed")
	}

	// Test transaction with insufficient gas price
	tx = types.NewTransaction(
		0,                         // nonce
		[20]byte{},                // to
		big.NewInt(0),             // value
		21000,                     // gas
		big.NewInt(5*params.GWei), // gasPrice - lower than min
		nil,                       // data
	)

	result = filter.FilterTx(context.Background(), tx)
	if result {
		t.Error("Expected transaction with insufficient gas price to be rejected")
	}
}

func TestXLayerFilterFilterTx_EIP1559(t *testing.T) {
	config := gasprice.Config{
		Default: big.NewInt(10 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.DefaultType,
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(5 * params.GWei), // Set a base fee for EIP-1559 testing
	})
	xlayerCache := newMockGasPriceCache(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// Test EIP-1559 transaction with sufficient fee cap
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		GasTipCap: big.NewInt(8 * params.GWei),  // 8 + 5 = 13 GWei > 10 GWei
		GasFeeCap: big.NewInt(20 * params.GWei), // Higher than min
		Gas:       21000,
		To:        &common.Address{},
		Value:     big.NewInt(0),
		Data:      nil,
	})

	result := filter.FilterTx(context.Background(), tx)
	if !result {
		t.Error("Expected EIP-1559 transaction with sufficient fee cap to be allowed")
	}

	// Test EIP-1559 transaction with insufficient fee cap
	tx = types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		GasTipCap: big.NewInt(2 * params.GWei),
		GasFeeCap: big.NewInt(5 * params.GWei), // Lower than min
		Gas:       21000,
		To:        &common.Address{},
		Value:     big.NewInt(0),
		Data:      nil,
	})

	result = filter.FilterTx(context.Background(), tx)
	if result {
		t.Error("Expected EIP-1559 transaction with insufficient fee cap to be rejected")
	}
}

func TestXLayerFilterFilterTx_FollowerType(t *testing.T) {
	config := gasprice.Config{
		Default: big.NewInt(10 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.FollowerType,
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(0),
	})
	xlayerCache := newMockGasPriceCache(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// Test transaction with sufficient gas price
	tx := types.NewTransaction(
		0,                          // nonce
		[20]byte{},                 // to
		big.NewInt(0),              // value
		21000,                      // gas
		big.NewInt(15*params.GWei), // gasPrice - higher than min
		nil,                        // data
	)

	result := filter.FilterTx(context.Background(), tx)
	if !result {
		t.Error("Expected transaction with sufficient gas price to be allowed in follower type")
	}
}

func TestXLayerFilterFilterTx_FixedType(t *testing.T) {
	config := gasprice.Config{
		Default: big.NewInt(10 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.FixedType,
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(0),
	})
	xlayerCache := newMockGasPriceCache(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// Test transaction with sufficient gas price
	tx := types.NewTransaction(
		0,                          // nonce
		[20]byte{},                 // to
		big.NewInt(0),              // value
		21000,                      // gas
		big.NewInt(15*params.GWei), // gasPrice - higher than min
		nil,                        // data
	)

	result := filter.FilterTx(context.Background(), tx)
	if !result {
		t.Error("Expected transaction with sufficient gas price to be allowed in fixed type")
	}
}

func TestXLayerFilterGetMinGasPrice(t *testing.T) {
	config := gasprice.Config{
		Default: big.NewInt(10 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.DefaultType,
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(0),
	})
	xlayerCache := newMockGasPriceCache(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	result := filter.GetMinGasPrice()
	if result.Cmp(minGasPrice) != 0 {
		t.Errorf("Expected minGasPrice %s, got %s", minGasPrice, result)
	}
}

func TestXLayerFilterUpdateConfig(t *testing.T) {
	config := gasprice.Config{
		Default: big.NewInt(10 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.DefaultType,
		},
	}

	minGasPrice := big.NewInt(10 * params.GWei)
	gpricer := newMockXLayerGpricer(minGasPrice)
	blockchain := newMockBlockchainReader(&types.Header{
		BaseFee: big.NewInt(0),
	})
	xlayerCache := newMockGasPriceCache(minGasPrice)
	filter := NewXLayerFilter(config, gpricer, blockchain, xlayerCache)

	// Update config with new default gas price
	newConfig := gasprice.Config{
		Default: big.NewInt(20 * params.GWei),
		XLayer: gasprice.XLayerConfig{
			Type: gasprice.DefaultType,
		},
	}

	filter.UpdateConfig(newConfig)

	expected := big.NewInt(20 * params.GWei)
	result := filter.GetMinGasPrice()
	if result.Cmp(expected) != 0 {
		t.Errorf("Expected minGasPrice %s, got %s", expected, result)
	}
}
