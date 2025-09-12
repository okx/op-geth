package types

import "github.com/ethereum/go-ethereum/common"

type FinishedEntry struct {
	Height uint64
	Root   common.Hash
}
