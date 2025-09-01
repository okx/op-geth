package apollo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
)

// RollupConfigSubscriber handles Apollo configuration changes for Rollup
type RollupConfigSubscriber struct {
	ethService *eth.Ethereum
	logger     log.Logger
}

// NewRollupConfigSubscriber creates a new Rollup configuration subscriber
func NewRollupConfigSubscriber(backend interface{}) *RollupConfigSubscriber {
	ethService, ok := backend.(*eth.Ethereum)
	if !ok {
		log.Error("Invalid backend type for RollupConfigSubscriber", "type", fmt.Sprintf("%T", backend))
		return nil
	}
	return &RollupConfigSubscriber{
		ethService: ethService,
		logger:     log.New("module", "apollo-rollup"),
	}
}

// HandleConfigItem handles a specific configuration item change
func (rcs *RollupConfigSubscriber) HandleConfigItem(key, value string) error {
	// Only handle rollup related configurations
	if !strings.HasPrefix(key, "rollup.") {
		return nil
	}

	configKey := strings.TrimPrefix(key, "rollup.")
	rcs.logger.Info("Received Rollup configuration change", "key", configKey, "value", value)

	return rcs.handleRollup(key, value)
}

func (rcs *RollupConfigSubscriber) handleRollup(key, value string) error {
	switch key {
	case "rollup.disabletxpoolgossip":
		return rcs.handleTxPoolGossip(value)
	case "rollup.enabletxpooladmission":
		return rcs.handleTxPoolAdmission(value)
	}
	return nil
}

// updateTxGossipSetting updates the transaction gossip setting
func (rcs *RollupConfigSubscriber) handleTxPoolGossip(value string) error {
	disable, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("invalid boolean value for %s: %w", value, err)
	}

	return rcs.ethService.UpdateTxGossipSetting(disable)
}

func (rcs *RollupConfigSubscriber) handleTxPoolAdmission(value string) error {
	enable, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("invalid boolean value for %s: %w", value, err)
	}

	return rcs.ethService.UpdateTxPoolAdmissionSetting(enable)
}

// GetSupportedKeys returns all TxPool Gossip configuration keys supported by this subscriber
func (rcs *RollupConfigSubscriber) GetSupportedKeys() []string {
	return []string{
		"rollup.disabletxpoolgossip",
		"rollup.enabletxpooladmission",
	}
}
