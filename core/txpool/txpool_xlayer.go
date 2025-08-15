package txpool

import "time"

type LegacyPool interface {
	UpdateAccountSlots(uint64) error
	UpdateGlobalSlots(uint64) error
	UpdateAccountQueue(uint64) error
	UpdateGlobalQueue(uint64) error
	UpdatePriceLimit(uint64) error
	UpdatePriceBump(uint64) error
	UpdateLifetime(time.Duration) error
}

// GetLegacyPool returns the LegacyPool subpool if it exists.
//
// OP-Stack addition for Apollo configuration support.
func (p *TxPool) GetLegacyPool() LegacyPool {
	for _, subpool := range p.subpools {
		// Check if this subpool is a LegacyPool by testing for its specific methods
		if lpool, ok := subpool.(LegacyPool); ok {
			return lpool
		}
	}
	return nil
}
