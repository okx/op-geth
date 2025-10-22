package miner

import (
	"math/big"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

func (payload *Payload) resolveRealtime() *engine.ExecutionPayloadEnvelope {
	payload.lock.Lock()
	defer payload.lock.Unlock()

	// We interrupt any active building block to prevent it from adding more transactions,
	// and if it is an update, don't attempt to seal the block.
	payload.interruptBuilding()

	// Realtime incremental building requires the current block building, incremental or
	// on generateWork, to be fully completed to ensure all transactions pre-confirmed on
	// the realtime layer to be included into the payload.
	// We block until the block is fully built before returning.
	select {
	case <-payload.stop:
		return nil
	default:
	}
	payload.cond.Wait()

	// Signal the building routine to stop
	payload.stopBuilding()

	if payload.full != nil {
		envelope := engine.BlockToExecutableData(payload.full, payload.fullFees, payload.sidecars, payload.requests)
		// For X Layer, realtime
		envelope.Changeset = payload.finalizeBlockChangeset
		if payload.fullWitness != nil {
			envelope.Witness = new(hexutil.Bytes)
			*envelope.Witness, _ = rlp.EncodeToBytes(payload.fullWitness) // cannot fail
		}
		return envelope
	} else if payload.empty != nil {
		envelope := engine.BlockToExecutableData(payload.empty, big.NewInt(0), nil, payload.emptyRequests)
		if payload.emptyWitness != nil {
			envelope.Witness = new(hexutil.Bytes)
			*envelope.Witness, _ = rlp.EncodeToBytes(payload.emptyWitness) // cannot fail
		}
	} else if err := payload.err; err != nil {
		log.Error("Error building any payload", "id", payload.id, "err", err)
	}
	return nil
}
