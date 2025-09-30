package realtimeapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/log"
	realtimeSub "github.com/ethereum/go-ethereum/realtime/subscription"
	"github.com/ethereum/go-ethereum/rpc"
)

// Realtime send a notification each time when a transaction was received in real-time.
func (api *RealtimeAPIImpl) Realtime(ctx context.Context, criteria realtimeSub.StreamCriteria) (*rpc.Subscription, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		// Custom for realtime
		return &rpc.Subscription{}, ErrRealtimeNotEnabled
	}

	if api.subService == nil {
		// Custom for realtime
		return &rpc.Subscription{}, ErrRealtimeNotEnabled
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()
	msgChan, id, err := api.subService.SubscribeRealtime()
	if err != nil {
		return &rpc.Subscription{}, err
	}

	go func() {
		defer api.subService.UnsubscribeRealtime(id)

		for {
			select {
			case msg, ok := <-msgChan:
				if !ok {
					log.Warn("[Realtime] subscription txMsg channel closed")
					return
				}

				result := RealtimeSubResult{}
				if criteria.NewHeads && msg.BlockMsg != nil {
					if err != nil || msg.BlockMsg.Header == nil {
						log.Warn(fmt.Sprintf("[Realtime] subscription error: getting block info, err: %v", err))
						continue
					}
					result.Header = msg.BlockMsg.Header
					result.BlockTime = msg.BlockMsg.Header.Time
					err = notifier.Notify(rpcSub.ID, result)
					if err != nil {
						log.Warn("[realtime subscription] error while notifying subscription", "err", err)
					}
				}
				if msg.TxMsg != nil {
					_, tx, receipt, innerTxs, err := msg.TxMsg.GetAllTxData()
					if err != nil {
						log.Warn(fmt.Sprintf("[Realtime] subscription error: getting tx data, err: %v", err))
						continue
					}

					var txSender common.Address
					if tx.Type() == types.DepositTxType {
						txSender = tx.From()
					} else {
						signer := types.LatestSigner(api.b.ChainConfig())
						var err error
						txSender, err = types.Sender(signer, tx)
						if err != nil {
							log.Warn(fmt.Sprintf("[Realtime] subscription error: getting tx sender, err: %v", err))
							continue
						}
					}
					toAddress := tx.To()
					if len(criteria.SubscribedAddresses) != 0 {
						found := false
						for _, addr := range criteria.SubscribedAddresses {
							if addr == txSender || addr == *toAddress {
								found = true
								break
							}
						}
						if !found {
							continue
						}
					}

					result.TxHash = tx.Hash().Hex()
					result.BlockTime = msg.TxMsg.BlockTime
					if criteria.TransactionExtraInfo {
						result.TxData = tx
					}
					if criteria.TransactionReceipt {
						result.Receipt = receipt
					}
					if criteria.TransactionInnerTxs {
						result.InnerTxs = innerTxs
					}
					err = notifier.Notify(rpcSub.ID, result)
					if err != nil {
						log.Warn("[realtime subscription] error while notifying subscription", "err", err)
					}
				}
			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}

// Logs send a notification each time a new log appears in real-time.
func (api *RealtimeAPIImpl) Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.filterApi.Logs(ctx, crit)
	}

	if api.subService == nil {
		return api.filterApi.Logs(ctx, crit)
	}

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()
	logCh, id, err := api.subService.SubscribeRealtimeLogs(crit)
	if err != nil {
		return &rpc.Subscription{}, err
	}

	go func() {
		defer api.subService.UnsubscribeRealtimeLogs(id)

		for {
			select {
			case h, ok := <-logCh:
				if h != nil {
					if h.Topics == nil {
						h.Topics = make([]common.Hash, 0)
					}
					err := notifier.Notify(rpcSub.ID, h)
					if err != nil {
						log.Warn(fmt.Sprintf("[Realtime] subscription error while notifying, err: %v", err))
					}
				}
				if !ok {
					log.Warn("[Realtime] realtime log channel was closed")
					return
				}
			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}
