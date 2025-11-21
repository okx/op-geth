package xlayer

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/xlayer/apollo"
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

	log.Info(fmt.Sprintf("[Debug] gasprice_scheduler_xlayer.go line 111"))

	updateTimer := time.NewTimer(5 * time.Second)
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
			log.Info("[Debug] gasprice_scheduler_xlayer.go line 122, updateTimer.C")

			test, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "gpo.maxprice", s.gpricer.GetConfig().MaxPrice.String())
			if !ok {
				log.Error("[Debug] line 130 failed to get maxprice from apollo")
			}
			log.Info(fmt.Sprintf("[Debug] gasprice_scheduler_xlayer.go line 59, test: %d", test))

			// Test 1: int64
			testI64, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.int64", int64(999))
			if !ok {
				log.Error("[Debug] line 138 failed to get int64 from apollo")
			}
			log.Info("[Lucas] [Debug] int64 - after", "result", testI64)

			// Test 2: uint64
			testU64, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.uint64", uint64(888))
			if !ok {
				log.Error("[Debug] line 142 failed to get uint64 from apollo")
			}
			log.Info("[Lucas] [Debug] uint64 - after", "result", testU64)

			// Test 3: bool
			testBool, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.bool", true)
			if !ok {
				log.Error("[Debug] line 146 failed to get bool from apollo")
			}
			log.Info("[Lucas] [Debug] bool - after", "result", testBool)

			// Test 4: string
			testStr, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.string", "default-value")
			if !ok {
				log.Error("[Debug] line 150 failed to get string from apollo")
			}
			log.Info("[Lucas] [Debug] string - after", "result", testStr)

			// Test 5: float64
			testF64, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.float64", 3.14159)
			if !ok {
				log.Error("[Debug] line 154 failed to get float64 from apollo")
			}
			log.Info("[Lucas] [Debug] float64 - after", "result", testF64)

			// Test 6: *big.Int (small)
			defaultSmall := big.NewInt(100000001)
			testBigSmall, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.bigint.small", defaultSmall.String())
			if !ok {
				log.Error("[Debug] line 158 failed to get *big.Int (small) from apollo")
			}
			newBigIntSmall, _ := new(big.Int).SetString(testBigSmall, 10)
			log.Info("[Lucas] [Debug] *big.Int (small) - after", "result", newBigIntSmall)

			// Test 7: *big.Int (large)
			defaultLarge := new(big.Int)
			defaultLarge.SetString("5000000000000000000003", 10)
			testBigLarge, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.bigint.large", defaultLarge.String())
			if !ok {
				log.Error("[Debug] line 162 failed to get *big.Int (large) from apollo")
			}
			newBigIntLarge, _ := new(big.Int).SetString(testBigLarge, 10)
			log.Info("[Lucas] [Debug] *big.Int (large) - after", "result", newBigIntLarge)

			// Test 8: Missing key (should return default)
			testMissing, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.nonexistent", int64(12345))
			if !ok {
				log.Error("[Debug] line 166 failed to get missing key from apollo")
			}
			log.Info("[Lucas] [Debug] missing key - after", "result", testMissing, "should_be_default", testMissing == 12345)

			// Test 9: array
			testArray, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.array", []int{1, 2, 3})
			if !ok {
				log.Error("[Debug] line 170 failed to get array from apollo")
			}
			log.Info("[Lucas] [Debug] array - after", "result", testArray)

			// Test 10: array of strings
			testArrayOfStrings, ok := apollo.ApolloConfigOr(apollo.L2GasPricer, "test.array.of.strings", []string{"a", "b", "c"})
			if !ok {
				log.Error("[Debug] line 174 failed to get array of strings from apollo")
			}
			log.Info("[Lucas] [Debug] array of strings - after", "result", testArrayOfStrings)

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
