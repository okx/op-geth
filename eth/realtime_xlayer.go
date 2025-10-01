package eth

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime"
	realtimeCache "github.com/ethereum/go-ethereum/realtime/cache"
	realtimeKafka "github.com/ethereum/go-ethereum/realtime/kafka"
	"github.com/ethereum/go-ethereum/realtime/realtimeapi"
	realtimeSub "github.com/ethereum/go-ethereum/realtime/subscription"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
	"github.com/ethereum/go-ethereum/rpc"
)

func (eth *Ethereum) RealtimeEnabled() bool {
	return eth.config.XLayer.Realtime.Enable
}

func (eth *Ethereum) GetRealtimeHeaderInfoChan() chan *realtimeTypes.HeaderInfo {
	if eth.config.XLayer.Realtime.Enable {
		return eth.kafkaHeaderInfoChan
	}
	return nil
}

func (eth *Ethereum) GetRealtimeTxInfoChan() chan state.TxInfo {
	return eth.kafkaTxInfoChan
}

func (eth *Ethereum) GetFinishChan() chan realtimeTypes.FinishedEntry {
	if eth.RealtimeEnabled() {
		return eth.finishChan
	}
	return nil
}

func (eth *Ethereum) InitRealtime() {
	if eth.config.XLayer.Realtime.Enable {
		if !eth.config.XLayer.Realtime.RealtimeRpc {
			// Sequencer execution mode
			kafkaProducer, err := realtimeKafka.NewKafkaProducer(eth.config.XLayer.Realtime.Kafka, context.Background(), nil)
			if err != nil {
				eth.kafkaProducer = nil
				log.Warn("[Realtime] Failed to initialize kafka producer", "error", err)
			} else {
				eth.kafkaProducer = kafkaProducer
				eth.kafkaHeaderInfoChan = make(chan *realtimeTypes.HeaderInfo, realtimeKafka.DefaultKafkaBufferSize)
				eth.kafkaTxInfoChan = make(chan state.TxInfo, realtimeKafka.DefaultKafkaBufferSize)

				// Send error trigger message on EL restart
				if err := eth.kafkaProducer.SendKafkaErrorTrigger(0); err != nil {
					log.Error(fmt.Sprintf("[Realtime] Failed to send kafka error trigger message. error: %v", err))
				}
			}
		} else {
			// Rpc execution mode
			eth.finishChan = make(chan realtimeTypes.FinishedEntry)
			eth.blockchain.SetRealtimeFinishChan(eth.finishChan)
			if eth.config.XLayer.Realtime.EnableSubscribe {
				eth.realtimeSub = realtimeSub.NewRealtimeSubscription()
				eth.realtimeSub.Start(context.Background())
			}
			eth.realtimeCache = realtimeCache.NewRealtimeCache(context.Background(), eth.blockchain, eth.realtimeSub, eth.config.XLayer.Realtime.CacheDumpPath, eth.config.XLayer.Realtime.CacheHeightThreshold)

		}
	}
}

func (eth *Ethereum) StartRealtime() {
	if eth.RealtimeEnabled() {
		go realtime.ListenRealtimeConsumer(context.Background(), &eth.config.XLayer.Realtime, eth.realtimeCache, eth.finishChan, eth.config.XLayer.Realtime.RealtimeRpc)
		go realtime.ListenRealtimeProducer(context.Background(), eth.kafkaProducer, eth.kafkaHeaderInfoChan, nil, eth.kafkaTxInfoChan, eth.config.XLayer.Realtime.RealtimeRpc)
	}
}

func (eth *Ethereum) StopRealtime() {
	if eth.RealtimeEnabled() && !eth.config.XLayer.Realtime.RealtimeRpc {
		if err := eth.kafkaProducer.SendKafkaErrorTrigger(0); err != nil {
			log.Error(fmt.Sprintf("[Realtime] Failed to send kafka error trigger message. error: %v", err))
		}
	}
}

func (eth *Ethereum) TryGetRealtimeAPIs(filterApi *filters.FilterAPI) []rpc.API {
	if eth.RealtimeEnabled() {
		return []rpc.API{
			{
				Namespace: "eth",
				Service:   realtimeapi.NewRealtimeAPI(eth.realtimeCache, eth.realtimeSub, eth.APIBackend, filterApi),
			},
			{
				Namespace: "debug",
				Service:   realtimeapi.NewRealtimeDebugAPI(eth.realtimeCache, eth.APIBackend),
			},
		}
	}
	return nil
}
