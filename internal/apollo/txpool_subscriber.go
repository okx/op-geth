package apollo

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
)

// TxPoolConfigSubscriber handles Apollo configuration changes for TxPool
type TxPoolConfigSubscriber struct {
	ethService *eth.Ethereum
	logger     log.Logger
}

// NewTxPoolConfigSubscriber creates a new TxPool configuration subscriber
func NewTxPoolConfigSubscriber(backend interface{}) *TxPoolConfigSubscriber {
	ethService, ok := backend.(*eth.Ethereum)
	if !ok {
		log.Error("Invalid backend type for TxPoolConfigSubscriber", "type", fmt.Sprintf("%T", backend))
		return nil
	}
	return &TxPoolConfigSubscriber{
		ethService: ethService,
		logger:     log.New("module", "apollo-txpool"),
	}
}

// HandleConfigItem handles a specific configuration item change
func (tcs *TxPoolConfigSubscriber) HandleConfigItem(key, value string) error {
	// Only handle txpool related configurations
	if !strings.HasPrefix(key, "txpool.") {
		return nil
	}

	configKey := strings.TrimPrefix(key, "txpool.")
	tcs.logger.Info("Received TxPool configuration change", "key", configKey, "value", value)

	return tcs.handleTxPool(key, value)
}

func (tcs *TxPoolConfigSubscriber) handleTxPool(key, value string) error {
	switch key {
	case "accountslots":
		return tcs.handleAccountSlots(value)
	case "globalslots":
		return tcs.handleGlobalSlots(value)
	case "accountqueue":
		return tcs.handleAccountQueue(value)
	case "globalqueue":
		return tcs.handleGlobalQueue(value)
	case "pricelimit":
		return tcs.handlePriceLimit(value)
	case "pricebump":
		return tcs.handlePriceBump(value)
	case "lifetime":
		return tcs.handleLifetime(value)
	}
	return nil
}

// getLegacyPool finds and returns the LegacyPool from TxPool subpools
func (tcs *TxPoolConfigSubscriber) getLegacyPool() (txpool.LegacyPool, error) {
	txPool := tcs.ethService.TxPool()
	if txPool == nil {
		return nil, fmt.Errorf("TxPool is nil")
	}

	legacyPool := txPool.GetLegacyPool()
	if legacyPool == nil {
		return nil, fmt.Errorf("LegacyPool not found")
	}

	return legacyPool, nil
}

// getBlobPool finds and returns the BlobPool from TxPool subpools

// handleAccountSlots handles accountslots configuration and returns error
func (tcs *TxPoolConfigSubscriber) handleAccountSlots(value string) error {
	newValue, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid AccountSlots value %s: %w", value, err)
	}

	if newValue < 1 {
		return fmt.Errorf("AccountSlots must be at least 1, got %d", newValue)
	}

	legacyPool, err := tcs.getLegacyPool()
	if err != nil {
		return err
	}
	return legacyPool.UpdateAccountSlots(newValue)
}

// handleGlobalSlots handles globalslots configuration and returns error
func (tcs *TxPoolConfigSubscriber) handleGlobalSlots(value string) error {
	newValue, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid GlobalSlots value %s: %w", value, err)
	}

	if newValue < 1 {
		return fmt.Errorf("GlobalSlots must be at least 1, got %d", newValue)
	}

	legacyPool, err := tcs.getLegacyPool()
	if err != nil {
		return err
	}
	return legacyPool.UpdateGlobalSlots(newValue)
}

// handleAccountQueue handles accountqueue configuration and returns error
func (tcs *TxPoolConfigSubscriber) handleAccountQueue(value string) error {
	newValue, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid AccountQueue value %s: %w", value, err)
	}

	if newValue < 1 {
		return fmt.Errorf("AccountQueue must be at least 1, got %d", newValue)
	}

	legacyPool, err := tcs.getLegacyPool()
	if err != nil {
		return err
	}
	return legacyPool.UpdateAccountQueue(newValue)
}

// handleGlobalQueue handles globalqueue configuration and returns error
func (tcs *TxPoolConfigSubscriber) handleGlobalQueue(value string) error {
	newValue, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid GlobalQueue value %s: %w", value, err)
	}

	if newValue < 1 {
		return fmt.Errorf("GlobalQueue must be at least 1, got %d", newValue)
	}

	legacyPool, err := tcs.getLegacyPool()
	if err != nil {
		return err
	}
	return legacyPool.UpdateGlobalQueue(newValue)
}

// handlePriceLimit handles pricelimit configuration and returns error
func (tcs *TxPoolConfigSubscriber) handlePriceLimit(value string) error {
	newValue, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid PriceLimit value %s: %w", value, err)
	}

	if newValue < 1 {
		return fmt.Errorf("PriceLimit must be at least 1, got %d", newValue)
	}

	legacyPool, err := tcs.getLegacyPool()
	if err != nil {
		return err
	}
	return legacyPool.UpdatePriceLimit(newValue)
}

// handlePriceBump handles pricebump configuration and returns error
func (tcs *TxPoolConfigSubscriber) handlePriceBump(value string) error {
	newValue, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid PriceBump value %s: %w", value, err)
	}

	if newValue < 1 {
		return fmt.Errorf("PriceBump must be at least 1, got %d", newValue)
	}

	legacyPool, err := tcs.getLegacyPool()
	if err != nil {
		return err
	}
	return legacyPool.UpdatePriceBump(newValue)
}

// handleLifetime handles lifetime configuration and returns error
func (tcs *TxPoolConfigSubscriber) handleLifetime(value string) error {
	var duration time.Duration
	var err error

	if duration, err = time.ParseDuration(value); err != nil {
		if seconds, parseErr := strconv.ParseInt(value, 10, 64); parseErr == nil {
			duration = time.Duration(seconds) * time.Second
		} else {
			return fmt.Errorf("invalid Lifetime value %s: %w", value, err)
		}
	}

	if duration < time.Second {
		return fmt.Errorf("lifetime must be at least 1 second, got %v", duration)
	}

	legacyPool, err := tcs.getLegacyPool()
	if err != nil {
		return err
	}
	return legacyPool.UpdateLifetime(duration)
}

// GetSupportedKeys returns all TxPool configuration keys supported by this subscriber
func (tcs *TxPoolConfigSubscriber) GetSupportedKeys() []string {
	return []string{
		"txpool.accountslots",
		"txpool.globalslots",
		"txpool.accountqueue",
		"txpool.globalqueue",
		"txpool.pricelimit",
		"txpool.pricebump",
		"txpool.lifetime",
	}
}
