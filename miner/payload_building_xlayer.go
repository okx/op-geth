package miner

import "github.com/ethereum/go-ethereum/beacon/engine"

func (payload *Payload) resolveRealtime() *engine.ExecutionPayloadEnvelope {
	// Realtime incremental building requires block building to be completed.
	// We block until the block is fully built before returning.
	payload.lock.Lock()
	defer payload.lock.Unlock()
	return payload.resolve(true)
}
