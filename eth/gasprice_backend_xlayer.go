package eth

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/eth/gasprice/xlayer"

	"github.com/ethereum/go-ethereum/core"
)

// xlayerEthereumBackend implements gasprice.EthereumBackend interface
type xlayerEthereumBackend struct {
	eth *Ethereum
}

// Blockchain returns the blockchain instance
func (b *xlayerEthereumBackend) Blockchain() *core.BlockChain {
	return b.eth.blockchain
}

// GasPriceOracle returns the gas price oracle
func (b *xlayerEthereumBackend) GasPriceOracle() xlayer.GasPriceOracle {
	if b.eth.APIBackend != nil {
		return b.eth.APIBackend.gpo
	}
	return nil
}

// GetPendingTxCount returns the number of pending transactions in the transaction pool
func (b *xlayerEthereumBackend) GetPendingTxCount(ctx context.Context) (int, error) {
	if b.eth.txPool == nil {
		return 0, fmt.Errorf("transaction pool is not available")
	}

	// Get pending and queued transaction counts
	pending, _ := b.eth.txPool.Stats()
	return pending, nil
}
