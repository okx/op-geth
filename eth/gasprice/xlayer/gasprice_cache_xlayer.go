package xlayer

import (
	"fmt"
	"math"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/eth/gasprice"
)

const (
	// maxCacheSize = 300sec (TTL) / 10sec (UpdatePeriod) = 30
	maxCacheSize = 30

	// minGPWindowSize defines the window size to be used when calculating the
	// MinGP from the cache
	minGPWindowSize = 27
)

// RawGPCache handles raw gas price caching with a fixed size circular buffer
type RawGPCache struct {
	values [maxCacheSize]*big.Int
	head   int // Points to the current head of the buffer
}

// NewRawGPCache initializes a RawGPCache with a fixed size cache
func NewRawGPCache() *RawGPCache {
	return &RawGPCache{
		head: 0,
	}
}

// Add adds an RGP to the cache and manages the head position
func (c *RawGPCache) Add(rgp *big.Int) {
	c.values[c.head] = new(big.Int).Set(rgp)
	c.head = (c.head + 1) % maxCacheSize
}

// GetMin returns the minimum RGP in the cache
func (c *RawGPCache) GetMin() (*big.Int, error) {
	isEmpty := true
	minRGP := big.NewInt(0).SetInt64(math.MaxInt64) // Initialize to maximum big.Int
	for _, value := range c.values {
		if value == nil {
			continue
		}
		isEmpty = false
		if value.Cmp(minRGP) < 0 {
			minRGP = value
		}
	}

	if isEmpty {
		return nil, fmt.Errorf("no values in cache")
	}

	return new(big.Int).Set(minRGP), nil
}

// GetMinGPMoreRecent returns the minimum RGP in the cache for the last minGPWindowSize elements
func (c *RawGPCache) GetMinGPMoreRecent() (*big.Int, error) {
	isEmpty := true
	minRGP := big.NewInt(0).SetInt64(math.MaxInt64) // Initialize to maximum big.Int

	for i := 1; i <= minGPWindowSize; i++ {
		index := (c.head - i + maxCacheSize) % maxCacheSize
		value := c.values[index]
		if value == nil {
			break
		}

		isEmpty = false
		if value.Cmp(minRGP) < 0 {
			minRGP = value
		}
	}

	if isEmpty {
		return nil, fmt.Errorf("no values in cache")
	}

	return new(big.Int).Set(minRGP), nil
}

// GasPriceCache handles gas price caching for XLayer
type GasPriceCache struct {
	latestPrice atomic.Pointer[big.Int]
	mtx         sync.RWMutex
	rawGPCache  *RawGPCache
}

// NewGasPriceCache creates a new gas price cache
func NewGasPriceCache() *GasPriceCache {
	// For X Layer, optimize gas price cache
	gpCache := &GasPriceCache{
		latestPrice: atomic.Pointer[big.Int]{},
		rawGPCache:  NewRawGPCache(),
	}
	gpCache.latestPrice.Store(big.NewInt(0))
	return gpCache
}

// GetLatest returns the latest cached gas price
func (c *GasPriceCache) GetLatest() *big.Int {
	// For X Layer, optimize gas price cache
	price := new(big.Int)
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	price.Set(c.latestPrice.Load()) // deep copy
	return price
}

// GetLatestPriceReadOnly returns the latest gas price for read-only access
func (c *GasPriceCache) GetLatestPriceReadOnly() *big.Int {
	return c.latestPrice.Load()
}

// SetLatest sets the latest gas price
func (c *GasPriceCache) SetLatest(price *big.Int) {
	c.mtx.Lock()
	// For X Layer, optimize gas price cache
	c.latestPrice.Store(price)
	c.mtx.Unlock()
}

// GetLatestRawGP returns the latest raw gas price
func (c *GasPriceCache) GetLatestRawGP() *big.Int {
	rgp, err := c.rawGPCache.GetMin()
	if err != nil {
		return gasprice.DefaultXLayerPrice
	}
	return rgp
}

// GetMinRawGPMoreRecent returns the minimum raw gas price from recent cache entries
func (c *GasPriceCache) GetMinRawGPMoreRecent() *big.Int {
	rgp, err := c.rawGPCache.GetMinGPMoreRecent()
	if err != nil {
		return gasprice.DefaultXLayerPrice
	}
	return rgp
}

// SetLatestRawGP sets the latest raw gas price
func (c *GasPriceCache) SetLatestRawGP(rgp *big.Int) {
	c.rawGPCache.Add(rgp)
}
