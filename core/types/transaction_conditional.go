package types

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
)

const TransactionConditionalMaxCost = 1000

// KnownAccounts represents a set of KnownAccounts
type KnownAccounts map[common.Address]KnownAccount

// EncodeRLP will encode the known account into rlp bytes
func (ka KnownAccounts) EncodeRLP(w io.Writer) error {
	type accountKV struct {
		Key   common.Address
		Value KnownAccount
	}
	accounts := make([]accountKV, 0, len(ka))
	for k, v := range ka {
		accounts = append(accounts, accountKV{Key: k, Value: v})
	}
	return rlp.Encode(w, accounts)
}

// DecodeRLP will decode the known account into rlp bytes
func (ka *KnownAccounts) DecodeRLP(s *rlp.Stream) error {
	type accountKV struct {
		Key   common.Address
		Value KnownAccount
	}
	var accounts []accountKV
	if err := s.Decode(&accounts); err != nil {
		return err
	}

	kamap := *ka
	for _, account := range accounts {
		kamap[account.Key] = account.Value
	}
	return nil
}

// KnownAccount allows for a user to express their preference of a known
// prestate at a particular account. Only one of the storage root or
// storage slots is allowed to be set. If the storage root is set, then
// the user prefers their transaction to only be included in a block if
// the account's storage root matches. If the storage slots are set,
// then the user prefers their transaction to only be included if the
// particular storage slot values from state match.
type KnownAccount struct {
	StorageRoot  *common.Hash `rlp:"nil"`
	StorageSlots map[common.Hash]common.Hash
}

// UnmarshalJSON will parse the JSON bytes into a KnownAccount struct.
func (ka *KnownAccount) UnmarshalJSON(data []byte) error {
	var hash common.Hash
	if err := json.Unmarshal(data, &hash); err == nil {
		ka.StorageRoot = &hash
		ka.StorageSlots = make(map[common.Hash]common.Hash)
		return nil
	}

	var mapping map[common.Hash]common.Hash
	if err := json.Unmarshal(data, &mapping); err != nil {
		return err
	}
	ka.StorageSlots = mapping
	return nil
}

// MarshalJSON will serialize the KnownAccount into JSON bytes.
func (ka *KnownAccount) MarshalJSON() ([]byte, error) {
	if ka.StorageRoot != nil {
		return json.Marshal(ka.StorageRoot)
	}
	return json.Marshal(ka.StorageSlots)
}

// EncodeRLP will serialize the KnownAccount into rlp bytes
func (ka KnownAccount) EncodeRLP(w io.Writer) error {
	if ka.StorageRoot != nil {
		return rlp.Encode(w, ka.StorageRoot)
	}
	type slotKV struct {
		Key   common.Hash
		Value common.Hash
	}
	slots := make([]slotKV, 0, len(ka.StorageSlots))
	for k, v := range ka.StorageSlots {
		slots = append(slots, slotKV{Key: k, Value: v})
	}
	return rlp.Encode(w, slots)
}

// DecodeRLP will decode the KnownAccount from rlp bytes
func (ka *KnownAccount) DecodeRLP(s *rlp.Stream) error {
	ka.StorageSlots = make(map[common.Hash]common.Hash)
	type slotKV struct {
		Key   common.Hash
		Value common.Hash
	}
	_, size, err := s.Kind()
	switch {
	case err != nil:
		return err
	case size == 0:
		return nil
	case size == 32:
		// storage root
		return s.Decode(&ka.StorageRoot)
	default:
		// storage slots
		slots := []slotKV{}
		if err := s.Decode(&slots); err != nil {
			return err
		}
		for _, slot := range slots {
			ka.StorageSlots[slot.Key] = slot.Value
		}
		return nil
	}
}

// Root will return the storage root and true when the user prefers
// execution against an account's storage root, otherwise it will
// return false.
func (ka *KnownAccount) Root() (common.Hash, bool) {
	if ka.StorageRoot == nil {
		return common.Hash{}, false
	}
	return *ka.StorageRoot, true
}

// Slots will return the storage slots and true when the user prefers
// execution against an account's particular storage slots, StorageRoot == nil,
// otherwise it will return false.
func (ka *KnownAccount) Slots() (map[common.Hash]common.Hash, bool) {
	if ka.StorageRoot != nil {
		return ka.StorageSlots, false
	}
	return ka.StorageSlots, true
}

//go:generate go run github.com/fjl/gencodec -type TransactionConditional -field-override transactionConditionalMarshalling -out gen_transaction_conditional_json.go

// TransactionConditional represents the preconditions that determine the
// inclusion of the transaction, enforced out-of-protocol by the sequencer.
type TransactionConditional struct {
	// KnownAccounts represents account prestate conditions
	KnownAccounts KnownAccounts `json:"knownAccounts"`

	// Header state conditionals
	BlockNumberMin *big.Int `json:"blockNumberMin,omitempty"`
	BlockNumberMax *big.Int `json:"blockNumberMax,omitempty"`
	TimestampMin   *uint64  `json:"timestampMin,omitempty"`
	TimestampMax   *uint64  `json:"timestampMax,omitempty"`

	// Tracked internally for metrics purposes. Exported such that it's
	// rlp encoded and gossiped to peers from the originating replica but
	// ignored when deserialized from the external json api
	SubmissionTime time.Time `json:"-"`
}

// field type overrides for gencodec
type transactionConditionalMarshalling struct {
	BlockNumberMax *hexutil.Big
	BlockNumberMin *hexutil.Big
	TimestampMin   *hexutil.Uint64
	TimestampMax   *hexutil.Uint64
}

// EncodeRLP will encode the TransactionConditional into RLP bytes
func (cond TransactionConditional) EncodeRLP(w io.Writer) error {
	type TransactionConditional struct {
		KnownAccounts  KnownAccounts
		BlockNumberMin *big.Int
		BlockNumberMax *big.Int
		TimestampMin   *uint64
		TimestampMax   *uint64
		SubmissionTime uint64
	}
	var enc TransactionConditional
	enc.KnownAccounts = cond.KnownAccounts
	enc.BlockNumberMin = cond.BlockNumberMin
	enc.BlockNumberMax = cond.BlockNumberMax
	enc.TimestampMin = cond.TimestampMin
	enc.TimestampMax = cond.TimestampMax
	enc.SubmissionTime = uint64(cond.SubmissionTime.Unix())
	return rlp.Encode(w, enc)
}

// DecodeRLP will decode the TransactionConditional from RLP bytes
func (cond *TransactionConditional) DecodeRLP(s *rlp.Stream) error {
	type TransactionConditional struct {
		KnownAccounts  KnownAccounts
		BlockNumberMin *big.Int `rlp:"nil"`
		BlockNumberMax *big.Int `rlp:"nil"`
		TimestampMin   *uint64  `rlp:"nil"`
		TimestampMax   *uint64  `rlp:"nil"`
		SubmissionTime uint64
	}
	dec := TransactionConditional{KnownAccounts: make(map[common.Address]KnownAccount)}
	if err := s.Decode(&dec); err != nil {
		return err
	}
	cond.KnownAccounts = dec.KnownAccounts
	cond.BlockNumberMin = dec.BlockNumberMin
	cond.BlockNumberMax = dec.BlockNumberMax
	cond.TimestampMin = dec.TimestampMin
	cond.TimestampMax = dec.TimestampMax
	cond.SubmissionTime = time.Unix(int64(dec.SubmissionTime), 0)
	return nil
}

// Validate will perform sanity checks on the specified preconditions
func (cond *TransactionConditional) Validate() error {
	if cond.BlockNumberMin != nil && cond.BlockNumberMax != nil && cond.BlockNumberMin.Cmp(cond.BlockNumberMax) > 0 {
		return fmt.Errorf("block number minimum constraint must be less than the maximum")
	}
	if cond.TimestampMin != nil && cond.TimestampMax != nil && *cond.TimestampMin > *cond.TimestampMax {
		return fmt.Errorf("timestamp minimum constraint must be less than the maximum")
	}
	return nil
}

// Cost computes the cost of checking the conditional -- total number of storage lookups required
func (opts *TransactionConditional) Cost() int {
	cost := 0
	for _, account := range opts.KnownAccounts {
		if _, isRoot := account.Root(); isRoot {
			cost += 1
		}
		if slots, isSlots := account.Slots(); isSlots {
			cost += len(slots)
		}
	}
	if opts.BlockNumberMin != nil || opts.BlockNumberMax != nil {
		cost += 1
	}
	if opts.TimestampMin != nil || opts.TimestampMax != nil {
		cost += 1
	}
	return cost
}

func (opts *TransactionConditional) SetSubmissionTime(t time.Time) {
	opts.SubmissionTime = t
}
