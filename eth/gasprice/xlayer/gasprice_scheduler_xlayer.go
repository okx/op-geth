package xlayer

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
)

// GasPriceOracle defines the interface for gas price suggestions
type GasPriceOracle interface {
	SuggestTipCap(ctx context.Context) (*big.Int, error)
}

// EthereumBackend defines the interface for accessing Ethereum backend services
type EthereumBackend interface {
	Blockchain() *core.BlockChain
	GasPriceOracle() GasPriceOracle
	GetPendingTxCount(ctx context.Context) (int, error)
}

// XLayerScheduler handles the scheduling of gas price updates for XLayer
type XLayerScheduler struct {
	ctx       context.Context
	gpricer   L2GasPricer
	eth       EthereumBackend
	stopChan  chan struct{}
	isRunning bool
	mu        sync.RWMutex
}

// NewXLayerScheduler creates a new XLayer gas price scheduler
func NewXLayerScheduler(ctx context.Context, gpricer L2GasPricer, eth EthereumBackend) *XLayerScheduler {
	return &XLayerScheduler{
		ctx:      ctx,
		gpricer:  gpricer,
		eth:      eth,
		stopChan: make(chan struct{}),
	}
}

// Start starts the gas price scheduler
func (s *XLayerScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return
	}

	s.isRunning = true

	// Set default gas price
	s.gpricer.GetGasCache().SetLatest(s.gpricer.GetConfig().XLayer.Default)
	s.gpricer.GetGasCache().SetLatestRawGP(s.gpricer.GetConfig().XLayer.Default)

	go s.runL2GasPriceSuggester()

	log.Info("XLayer gas price scheduler started")
}

// Stop stops the gas price scheduler
func (s *XLayerScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.isRunning = false
	close(s.stopChan)

	log.Info("XLayer gas price scheduler stopped")
}

// IsRunning returns whether the scheduler is running
func (s *XLayerScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetGasCache returns the gas price cache
func (s *XLayerScheduler) GetGasCache() *GasPriceCache {
	return s.gpricer.GetGasCache()
}

// runL2GasPriceSuggester runs the L2 gas price suggester in a loop
func (s *XLayerScheduler) runL2GasPriceSuggester() {
	ctx := s.gpricer.GetCtx()

	// Check if eth and blockchain are available
	if s.eth == nil || s.eth.Blockchain() == nil {
		log.Error("blockchain is not available")
		return
	}

	// Get current state and fetch L1 gas price
	if l1gp, err := GetL1GasPrice(s.eth.Blockchain()); err == nil {
		s.gpricer.UpdateGasPriceAvg(l1gp)
	} else {
		log.Debug("L1 gas price has not been set, please start op-node", "err", err)
	}

	updateTimer := time.NewTimer(s.gpricer.GetConfig().XLayer.UpdatePeriod)
	defer updateTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Finishing l2 gas price suggester...")
			return
		case <-s.stopChan:
			log.Info("Stopping l2 gas price suggester...")
			return
		case <-updateTimer.C:
			// Check if eth and blockchain are available
			if s.eth == nil || s.eth.Blockchain() == nil {
				log.Error("blockchain is not available")
				updateTimer.Reset(s.gpricer.GetConfig().XLayer.UpdatePeriod)
				return
			}

			// Get current state and fetch L1 gas price
			if l1gp, err := GetL1GasPrice(s.eth.Blockchain()); err == nil {
				s.gpricer.UpdateGasPriceAvg(l1gp)
				s.gpricer.GetGasCache().SetLatestRawGP(s.gpricer.GetLastRawGP())
			} else {
				log.Debug("L1 gas price has not been set, please start op-node", "err", err)
			}

			s.updateDynamicGP(ctx)

			updateTimer.Reset(s.gpricer.GetConfig().XLayer.UpdatePeriod)
		}
	}
}

// updateDynamicGP updates the dynamic gas price based on current conditions
func (s *XLayerScheduler) updateDynamicGP(ctx context.Context) {
	// Check if eth and gasOracle are available
	if s.eth == nil || s.eth.GasPriceOracle() == nil {
		log.Error("gasOracle is not available")
		return
	}

	tipcap, err := s.eth.GasPriceOracle().SuggestTipCap(ctx) // SuggestTipCap provides gas price suggestion
	if err != nil {
		log.Error(fmt.Sprintf("error SuggestTipCap: %v", err))
		return
	}

	// get baseFee
	baseFee := s.eth.Blockchain().CurrentBlock().BaseFee
	if baseFee == nil {
		log.Error("baseFee is not available")
		return
	}
	gasResult := tipcap.Add(tipcap, baseFee)

	if gasResult.Cmp(s.gpricer.GetConfig().XLayer.Default) < 0 {
		log.Debug("GasPriceOracle suggested gas price is less than xlayer default, setting to xlayer default", "suggestedGasPrice", gasResult.String(), "default", s.gpricer.GetConfig().XLayer.Default.String())
		gasResult = new(big.Int).Set(s.gpricer.GetConfig().XLayer.Default)
	}

	rgp := s.gpricer.GetLastRawGP()
	if gasResult.Cmp(rgp) < 0 {
		log.Debug("gasResult is less than rgp, setting gasResult to recommendedGasPrice", "gasResult", gasResult.String(), "recommendedGasPrice", rgp.String())
		gasResult = new(big.Int).Set(rgp)
	}

	if !s.isCongested(ctx) {
		log.Debug("not congested, setting gasResult to avg of recommendedGasPrice and suggestGasPrice", "recommendedGasPrice", rgp.String(), "gasResult", gasResult.String())
		gasResult = getAvgPrice(rgp, gasResult)
	}

	s.gpricer.GetGasCache().SetLatest(gasResult)
	log.Info(fmt.Sprintf("Updated gas price: %s", gasResult.String()))
}

// isCongested checks if the network is congested
func (s *XLayerScheduler) isCongested(ctx context.Context) bool {
	latestBlockTxNum, err := getLatestBlockTxNum(s.eth.Blockchain())
	if err != nil {
		return false
	}
	isLatestBlockEmpty := latestBlockTxNum <= 1 // op-stack will have at least 1 tx(DespositTx) in the latest block

	pendingCount, err := s.eth.GetPendingTxCount(ctx)
	if err != nil {
		return false
	}

	isPendingTxCongested := pendingCount >= s.gpricer.GetConfig().XLayer.CongestionThreshold

	return !isLatestBlockEmpty && isPendingTxCongested
}

func getLatestBlockTxNum(blockchain *core.BlockChain) (int, error) {
	header := blockchain.CurrentBlock()
	if header == nil {
		return 0, fmt.Errorf("Could not get current block header during getLatestBlockTxNum")
	}
	body := blockchain.GetBody(header.Hash())
	if body == nil {
		return 0, fmt.Errorf("Could not get current block body during getLatestBlockTxNum")
	}
	return len(body.Transactions), nil
}

// getAvgPrice calculates the average price between low and high
func getAvgPrice(low *big.Int, high *big.Int) *big.Int {
	avg := new(big.Int).Add(low, high)
	avg = avg.Quo(avg, big.NewInt(2)) //nolint:gomnd
	return avg
}
