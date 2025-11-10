package xlayer

import (
	"context"

	"github.com/ethereum/go-ethereum/eth/gasprice"

	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// L2GasPricer interface for gas price suggester.
type L2GasPricer interface {
	UpdateGasPriceAvg(*big.Int)
	UpdateConfig(c gasprice.Config)
	GetLastRawGP() *big.Int
	GetConfig() gasprice.Config
	GetCtx() context.Context
	GetGasCache() *GasPriceCache
}

// NewL2GasPriceSuggester init.
func NewL2GasPriceSuggester(ctx context.Context, cfg gasprice.Config) L2GasPricer {
	var gpricer L2GasPricer
	switch cfg.XLayer.Type {
	case gasprice.GasPriceFollowerType:
		log.Info("Follower type selected")
		gpricer = newFollowerGasPriceSuggester(ctx, cfg)
	case gasprice.GasPriceDefaultType:
		log.Info("Default type selected")
		gpricer = newDefaultGasPriceSuggester(ctx, cfg)
	case gasprice.GasPriceFixedType:
		log.Info("Fixed type selected")
		gpricer = newFixedGasPriceSuggester(ctx, cfg)
	default:
		log.Error(fmt.Sprintf("unknown l2 gas price suggester type %v. Please specify a valid one: 'follower', 'fixed' or 'default'", cfg.XLayer.Type))
	}

	return gpricer
}

func GetL1GasPrice(blockchain *core.BlockChain) (*big.Int, error) {
	statedb, err := blockchain.State()
	if err != nil {
		return nil, fmt.Errorf("failed to get current state: %v", err)
	}
	// Get L1 base fee from state instead of network RPC
	l1BaseFee := statedb.GetState(types.L1BlockAddr, types.L1BaseFeeSlot).Big()

	// Return the L1 base fee as the gas price
	if l1BaseFee == nil || l1BaseFee.Sign() == 0 {
		return nil, fmt.Errorf("l1 base fee is not available in L1Block contract: L1BlockAddr=%v, L1BaseFeeSlot=%v", types.L1BlockAddr, types.L1BaseFeeSlot)
	}
	log.Trace("L1 gas price", "l1BaseFee", l1BaseFee.String())

	return l1BaseFee, nil
}
