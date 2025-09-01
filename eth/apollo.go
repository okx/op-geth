package eth

import (
	"fmt"

	"github.com/ethereum/go-ethereum/log"
)

// UpdateTxGossipSetting dynamically updates the transaction gossip setting
// OP-Stack addition for Apollo configuration support
func (s *Ethereum) UpdateTxGossipSetting(disable bool) error {
	oldValue := s.handler.noTxGossip
	if oldValue == disable {
		return nil // No change needed
	}

	s.handler.noTxGossip = disable
	log.Info("Updated transaction gossip setting",
		"disabled", disable, "previous", oldValue)

	return nil
}

func (s *Ethereum) UpdateTxPoolAdmissionSetting(enable bool) error {
	oldValue := s.APIBackend.disableTxPool
	if oldValue == !enable {
		return nil // No change needed
	}

	if s.config.RollupSequencerHTTP == "" {
		return fmt.Errorf("rollup sequencer HTTP endpoint is not set")
	}

	s.APIBackend.disableTxPool = !enable
	log.Info("Updated transaction pool admission setting",
		"disabled", !enable, "previous", oldValue)

	return nil
}
