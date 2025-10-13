package realtime

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/cache"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

var (
	MaxMessageChanSize      = 10_000
	MaxMessageCacheSize     = 100
	MinRealtimeLoopWaitTime = 10 * time.Millisecond

	errorFlag    = atomic.Bool{}
	resetFlag    = atomic.Bool{}
	messageCache *cache.MessageCache
)

func ListenRealtimeProducer(
	ctx context.Context,
	kafkaProducer *kafka.KafkaProducer,
	blockInfoChan chan *realtimeTypes.BlockInfo,
	txInfoChan chan state.TxInfo,
	isRpc bool) {
	if isRpc {
		log.Info("[Realtime] KafkaProducer is disabled on realtime-rpc, skipping")
		return
	}
	if kafkaProducer == nil {
		return
	}

	for {
		currHeight := uint64(0)

		select {
		case <-ctx.Done():
			return
		case blockInfo := <-blockInfoChan:
			currHeight = blockInfo.Header.Number.Uint64()
			err := kafkaProducer.SendKafkaBlockInfo(blockInfo)
			if err != nil {
				log.Error(fmt.Sprintf("[Realtime] Failed to send kafka block info message. error: %v, currHeight: %d, blockHash: %x", err, currHeight, blockInfo.Hash))
				err = kafkaProducer.SendKafkaErrorTrigger(currHeight)
				if err != nil {
					log.Error(fmt.Sprintf("[Realtime] Failed to send error trigger message. error: %v, currHeight: %d", err, currHeight))
				}
			} else {
				log.Debug(fmt.Sprintf("[Realtime] Sent kafka confirmed block info message for block number %d, blockHash: %x", currHeight, blockInfo.Hash))
			}
		case txInfo := <-txInfoChan:
			currHeight = txInfo.BlockNumber
			changeset := state.CollectChangeset(txInfo.Entries)
			err := kafkaProducer.SendKafkaTransaction(txInfo.BlockNumber, txInfo.BlockTime, txInfo.Tx, txInfo.Receipt, txInfo.InnerTxs, changeset)
			if err != nil {
				log.Error(fmt.Sprintf("[Realtime] Failed to send kafka tx message. error: %v, currHeight: %d", err, currHeight))
				err = kafkaProducer.SendKafkaErrorTrigger(currHeight)
				if err != nil {
					log.Error(fmt.Sprintf("[Realtime] Failed to send error trigger message. error: %v, currHeight: %d", err, currHeight))
				}
			} else {
				log.Debug(fmt.Sprintf("[Realtime] Sent kafka tx message for block number %d with txHash %x", txInfo.BlockNumber, txInfo.Tx.Hash()))
			}
		}
	}
}

func ListenRealtimeConsumer(
	ctx context.Context,
	cfg *RealtimeConfig,
	realtimeCache *cache.RealtimeCache,
	finishChan chan realtimeTypes.FinishedEntry,
	isRpc bool) {
	if !isRpc {
		log.Info("[Realtime] RealtimeConsumer is disabled on non realtime-rpc, skipping")
		return
	}
	if realtimeCache == nil {
		return
	}

	// Initialize realtime message cache
	var err error
	messageCache, err = cache.NewMessageCache(MaxMessageCacheSize)
	if err != nil {
		log.Error(fmt.Sprintf("[Realtime] Failed to initialize realtime message cache. error: %v", err))
		return
	}
	errorFlag.Store(false)
	blockMsgsChan := make(chan realtimeTypes.BlockInfo, MaxMessageChanSize)
	txMsgsChan := make(chan kafkaTypes.TransactionMessage, MaxMessageChanSize)
	errorMsgsChan := make(chan kafkaTypes.ErrorTriggerMessage, MaxMessageChanSize)
	errorChan := make(chan error, 1)

	// Init realtime consumer
	if cfg.SubscribeKafka {
		kafkaConsumer, err := kafka.NewKafkaConsumer(cfg.Kafka, true)
		if err != nil {
			log.Warn("[Realtime] Failed to initialize kafka consumer", "error", err)
			return
		}
		// Start the kafka consumer
		go kafkaConsumer.ConsumeKafka(ctx, blockMsgsChan, txMsgsChan, errorMsgsChan, errorChan)
	} else if cfg.SubscribeWebsocket {
		// TODO: Add ws consumer consume logic
	} else {
		log.Error("[Realtime] RealtimeConsumer disabled, no realtime kafka or websocket consumer specified")
		return
	}

	// Start realtime loop
	go realtimeLoop(ctx, realtimeCache)

	for {
		select {
		case <-ctx.Done():
			return
		case finishEntry := <-finishChan:
			if finishEntry.Height < realtimeCache.GetExecutionHeight() {
				// Chain rollback. Reset realtime cache
				resetFlag.Store(true)
				log.Error(fmt.Sprintf("[Realtime] Chain rollback detected, resetting realtime cache. finishHeight: %d", finishEntry.Height))
			}
			err := realtimeCache.UpdateExecution(finishEntry)
			if err != nil {
				log.Error(fmt.Sprintf("[Realtime] Failed to update execution. error: %v", err))
				resetFlag.Store(true)
			}
			log.Debug(fmt.Sprintf("[Realtime] Received finish signal from execution. finishHeight: %d", finishEntry.Height))
		case blockMsg := <-blockMsgsChan:
			// Confirmed block msg
			if err := blockMsg.Validate(realtimeCache.GetExecutionHeight()); err != nil {
				log.Error(fmt.Sprintf("[Realtime] Failed to consume block message from realtime consumer. error: %v", err))
				continue
			}
			if blockMsg.IsConfirmedBlock() {
				// Confirmed block msg
				messageCache.ConfirmedBlockMsgCache.Add(&blockMsg)
				log.Debug(fmt.Sprintf("[Realtime] Received confirmed block message. blockNum: %d", blockMsg.Header.Number))
			} else {
				// New pending block msg
				messageCache.NewBlockMsgCache.Add(&blockMsg)
				log.Debug(fmt.Sprintf("[Realtime] Received new block message. blockNum: %d", blockMsg.Header.Number))
			}
			log.Debug(fmt.Sprintf("[Realtime] Received confirmed block message. blockNum: %d", blockMsg.Header.Number))
		case txMsg := <-txMsgsChan:
			if err := txMsg.Validate(); err != nil {
				log.Error(fmt.Sprintf("[Realtime] Failed to consume transaction message from realtime consumer. error: %v", err))
				continue
			}
			if txMsg.BlockNumber <= realtimeCache.GetExecutionHeight() {
				// Ignore txs from previous blocks
				log.Debug(fmt.Sprintf("[Realtime] Ignoring transaction message from previous block. blockNum: %d", txMsg.BlockNumber))
				continue
			}
			messageCache.TxMsgCache.Add(&txMsg)
			log.Debug(fmt.Sprintf("[Realtime] Received transaction message. blockNum: %d, txHash: %x", txMsg.BlockNumber, txMsg.Hash))
		case errorTriggerMsg := <-errorMsgsChan:
			resetFlag.Store(true)
			triggerHeight := errorTriggerMsg.BlockNumber
			log.Error(fmt.Sprintf("[Realtime] Received error trigger message, flushing realtime cache. triggerHeight: %d", triggerHeight))
		case err := <-errorChan:
			errorFlag.Store(true)
			log.Error(fmt.Sprintf("[Realtime] Realtime consumer failed. error: %v", err))
			return
		}
	}
}

func realtimeLoop(ctx context.Context, realtimeCache *cache.RealtimeCache) {
	log.Info("[Realtime] Starting realtime loop")
	for {
		select {
		case <-ctx.Done():
			log.Info("[Realtime] context done, stopping realtime loop")
			return
		default:
		}

		startTime := time.Now()

		// Check for realtime consumer error
		if errorFlag.Load() {
			realtimeCache.ReadyFlag.Store(false)
			log.Error("[Realtime] Realtime consumer error, stopping realtime loop")
			return
		}

		// Check for reset trigger
		if resetFlag.Load() {
			resetRealtimeCache(realtimeCache)
			continue
		}

		// Check if realtime cache is ready
		if !realtimeCache.ReadyFlag.Load() {
			if ok := tryInitRealtimeCache(realtimeCache); !ok {
				time.Sleep(10 * time.Second)
			}
			continue
		}

		// Check for corrupted cache
		pendingHeight := realtimeCache.GetNextPendingHeight()
		lastExecutionHeight := realtimeCache.GetExecutionHeight()
		if pendingHeight != 0 && pendingHeight < lastExecutionHeight {
			// Execution is ahead of pending cache. This should not happen
			resetFlag.Store(true)
			log.Error(fmt.Sprintf("[Realtime] Execution height is ahead of cache confirm height. pendingHeight: %d, lastExecutionHeight: %d", pendingHeight, lastExecutionHeight))
			continue
		}

		// Handle new block msg
		highestPendingHeight := realtimeCache.GetHighestPendingHeight()
		if highestPendingHeight == 0 {
			highestPendingHeight = realtimeCache.GetHighestConfirmHeight()
		}
		nextHeight := highestPendingHeight + 1
		newBlockMsg, ok := messageCache.NewBlockMsgCache.Get(nextHeight)
		if ok {
			err := realtimeCache.TryApplyNewBlockMsg(newBlockMsg.Header.Number.Uint64(), newBlockMsg)
			if err != nil {
				// Apply state error. Reset cache
				resetFlag.Store(true)
				log.Error(fmt.Sprintf("[Realtime] Failed to apply new block msg. error: %v, blockHeight: %d", err, newBlockMsg.Header.Number.Uint64()))
			}
			messageCache.NewBlockMsgCache.Flush(nextHeight)
		}

		// Handle pending blocks
		err := realtimeCache.HandlePendingBlocks(messageCache)
		if err != nil {
			// Handle pending blocks error. Reset cache
			resetFlag.Store(true)
			log.Error(fmt.Sprintf("[Realtime] Failed to handle pending blocks. error: %v", err))
		}

		// Handle confirmed block msg
		highestPendingHeight = realtimeCache.GetHighestPendingHeight()
		if highestPendingHeight != 0 {
			confirmBlockMsg, ok := messageCache.ConfirmedBlockMsgCache.Get(pendingHeight)
			if ok {
				err := realtimeCache.TryCloseBlockFromConfirmedBlockMsg(pendingHeight, confirmBlockMsg)
				if err != nil {
					// Apply state error. Reset cache
					resetFlag.Store(true)
					log.Error(fmt.Sprintf("[Realtime] Failed to apply confirm block msg. error: %v, blockHeight: %d", err, pendingHeight))
				} else {
					messageCache.ConfirmedBlockMsgCache.Flush(pendingHeight)
				}
			}
		}

		duration := time.Since(startTime)
		if duration < MinRealtimeLoopWaitTime {
			time.Sleep(MinRealtimeLoopWaitTime - duration)
		}
	}
}

// tryInitRealtimeCache checks if the realtime cache can be initialized by comparing
// the current execution height with the lowest message cache height.
func tryInitRealtimeCache(realtimeCache *cache.RealtimeCache) bool {
	log.Debug("[Realtime] Trying to initialize realtime cache")
	executionHeight := realtimeCache.GetExecutionHeight()
	lowestMessageHeight := messageCache.GetLowestNewBlockHeight()
	if executionHeight == 0 || lowestMessageHeight == 0 {
		// No message message or rpc execution. Skip init
		log.Error(fmt.Sprintf("[Realtime] Init realtime cache failed, no realtime message or rpc execution. lowestMessageHeight: %d, executionHeight: %d", lowestMessageHeight, executionHeight))
		return false
	}

	if lowestMessageHeight > executionHeight {
		// The current execution height is behind message cache height. We will wait for the execution
		// height to catch up to message cache height before re-initializing the state cache.
		log.Error(fmt.Sprintf("[Realtime] Init realtime cache failed, waiting for execution height to catch up to message cache height. lowestMessageHeight: %d, executionHeight: %d", lowestMessageHeight, executionHeight))
		return false
	}

	realtimeCache.Clear()
	err := realtimeCache.TryInitStateCache(executionHeight)
	if err != nil {
		log.Error(fmt.Sprintf("[Realtime] Failed to initialize state cache. error: %v", err))
		return false
	}

	// Flush all realtime message data less than or equal to state cache height
	messageCache.Flush(executionHeight)
	realtimeCache.ReadyFlag.Store(true)
	log.Info(fmt.Sprintf("[Realtime] Realtime cache initialized. executionHeight: %d", executionHeight))

	return true
}

// resetRealtimeCache clears the realtime cache and resets the state flags
func resetRealtimeCache(realtimeCache *cache.RealtimeCache) {
	// Reset and clear realtime cache
	log.Info("[Realtime] Resetting realtime cache")
	realtimeCache.ReadyFlag.Store(false)
	realtimeCache.Clear()

	resetFlag.Store(false)
}
