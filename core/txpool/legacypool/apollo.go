// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package legacypool

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

// UpdateAccountSlots dynamically updates the AccountSlots configuration
// This triggers immediate rebalancing of the transaction pool
func (pool *LegacyPool) UpdateAccountSlots(newValue uint64) error {
	if newValue < 1 {
		return fmt.Errorf("account slots must be at least 1, got %d", newValue)
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()
	oldValue := pool.config.AccountSlots
	if oldValue == newValue {
		return nil
	}
	pool.config.AccountSlots = newValue

	log.Info("Updated txpool AccountSlots configuration",
		"old", oldValue, "new", newValue)

	// Trigger immediate rebalancing to apply the new limit
	pool.requestPromoteExecutables(newAccountSet(pool.signer))

	return nil
}

// UpdateGlobalSlots dynamically updates the GlobalSlots configuration
func (pool *LegacyPool) UpdateGlobalSlots(newValue uint64) error {
	if newValue < 1 {
		return fmt.Errorf("global slots must be at least 1, got %d", newValue)
	}

	// Check if value changed
	oldValue := pool.config.GlobalSlots
	if oldValue == newValue {
		return nil
	}

	pool.mu.Lock()
	pool.config.GlobalSlots = newValue
	pool.mu.Unlock()

	log.Info("Updated txpool GlobalSlots configuration",
		"old", oldValue, "new", newValue)

	// Trigger immediate rebalancing
	pool.requestPromoteExecutables(newAccountSet(pool.signer))

	return nil
}

// UpdateAccountQueue dynamically updates the AccountQueue configuration
func (pool *LegacyPool) UpdateAccountQueue(newValue uint64) error {
	if newValue < 1 {
		return fmt.Errorf("account queue must be at least 1, got %d", newValue)
	}

	// Check if value changed
	oldValue := pool.config.AccountQueue
	if oldValue == newValue {
		return nil
	}

	pool.mu.Lock()
	pool.config.AccountQueue = newValue
	pool.mu.Unlock()

	log.Info("Updated txpool AccountQueue configuration",
		"old", oldValue, "new", newValue)

	// Trigger queue cleanup
	pool.requestPromoteExecutables(newAccountSet(pool.signer))

	return nil
}

// UpdateGlobalQueue dynamically updates the GlobalQueue configuration
func (pool *LegacyPool) UpdateGlobalQueue(newValue uint64) error {
	if newValue < 1 {
		return fmt.Errorf("global queue must be at least 1, got %d", newValue)
	}

	// Check if value changed
	oldValue := pool.config.GlobalQueue
	if oldValue == newValue {
		return nil
	}

	pool.mu.Lock()
	pool.config.GlobalQueue = newValue
	pool.mu.Unlock()

	log.Info("Updated txpool GlobalQueue configuration",
		"old", oldValue, "new", newValue)

	// Trigger queue cleanup
	pool.requestPromoteExecutables(newAccountSet(pool.signer))

	return nil
}

// UpdatePriceLimit dynamically updates the PriceLimit configuration
func (pool *LegacyPool) UpdatePriceLimit(newValue uint64) error {
	if newValue < 1 {
		return fmt.Errorf("price limit must be at least 1, got %d", newValue)
	}

	// Check if value changed
	oldValue := pool.config.PriceLimit
	if oldValue == newValue {
		return nil
	}

	pool.mu.Lock()
	pool.config.PriceLimit = newValue
	pool.mu.Unlock()

	log.Info("Updated txpool PriceLimit configuration",
		"old", oldValue, "new", newValue)

	// Trigger price-based filtering
	pool.requestPromoteExecutables(newAccountSet(pool.signer))

	return nil
}

// UpdatePriceBump dynamically updates the PriceBump configuration
func (pool *LegacyPool) UpdatePriceBump(newValue uint64) error {
	if newValue < 1 {
		return fmt.Errorf("price bump must be at least 1, got %d", newValue)
	}

	// Check if value changed
	oldValue := pool.config.PriceBump
	if oldValue == newValue {
		return nil
	}

	pool.mu.Lock()
	pool.config.PriceBump = newValue
	pool.mu.Unlock()

	log.Info("Updated txpool PriceBump configuration",
		"old", oldValue, "new", newValue)

	// PriceBump doesn't require immediate action as it affects future replacements
	return nil
}

// UpdateLifetime dynamically updates the Lifetime configuration
func (pool *LegacyPool) UpdateLifetime(newValue time.Duration) error {
	if newValue < time.Second {
		return fmt.Errorf("lifetime must be at least 1 second, got %v", newValue)
	}

	// Check if value changed
	oldValue := pool.config.Lifetime
	if oldValue == newValue {
		return nil
	}

	pool.mu.Lock()
	pool.config.Lifetime = newValue
	pool.mu.Unlock()

	log.Info("Updated txpool Lifetime configuration",
		"old", oldValue, "new", newValue)

	// Lifetime changes don't require immediate action
	return nil
}
