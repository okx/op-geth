package types

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type HeaderInfo struct {
	Header    *types.Header `json:"header"`
	Changeset *Changeset    `json:"changeset,omitempty"`
}

func (msg HeaderInfo) Validate(executionHeight uint64) error {
	if msg.Header == nil {
		return fmt.Errorf("header is nil")
	}
	if msg.Header.Number.Uint64() == 0 {
		return fmt.Errorf("block number is 0")
	}
	if msg.Header.Number.Uint64() < executionHeight {
		// Ignore block msgs from previous blocks
		return fmt.Errorf("received old header message, blockNum: %d executionHeight: %d", msg.Header.Number.Uint64(), executionHeight)
	}
	return nil
}

type BlockInfo struct {
	Header      *types.Header      `json:"header"`
	Withdrawals *types.Withdrawals `json:"withdrawals,omitempty"`
	TxCount     int64              `json:"txCount"`
	Hash        common.Hash        `json:"hash"`
	Changeset   *Changeset         `json:"changeset,omitempty"`
}

func (msg BlockInfo) Validate(executionHeight uint64) error {
	if msg.Header == nil {
		return fmt.Errorf("header is nil")
	}
	if msg.Header.Number.Uint64() == 0 {
		return fmt.Errorf("block number is 0")
	}
	if msg.Header.Number.Uint64() < executionHeight {
		// Ignore block msgs from previous blocks
		return fmt.Errorf("received old block message, blockNum: %d executionHeight: %d", msg.Header.Number.Uint64(), executionHeight)
	}
	return nil
}
