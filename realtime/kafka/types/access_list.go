package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type AccessTupleMessage struct {
	Address     string   `json:"address"`
	StorageKeys []string `json:"storageKeys"`
}

func fromAccessTuple(tuple types.AccessTuple) AccessTupleMessage {
	storageKeys := make([]string, 0, len(tuple.StorageKeys))
	for _, key := range tuple.StorageKeys {
		storageKeys = append(storageKeys, key.String())
	}
	return AccessTupleMessage{
		Address:     tuple.Address.String(),
		StorageKeys: storageKeys,
	}
}

func (msg AccessTupleMessage) toAccessTuple() types.AccessTuple {
	storageKeys := make([]common.Hash, 0, len(msg.StorageKeys))
	for _, key := range msg.StorageKeys {
		storageKeys = append(storageKeys, common.HexToHash(key))
	}

	return types.AccessTuple{
		Address:     common.HexToAddress(msg.Address),
		StorageKeys: storageKeys,
	}
}
