package types

import (
	libcommon "github.com/ethereum/go-ethereum/common"
)

type FinishedEntry struct {
	Height uint64
	Root   libcommon.Hash
}
