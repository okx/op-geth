// XLayer emergency-freeze blacklist — on-chain data source (XLOP-1099, FR-4).
//
// This file is a fork-local XLayer extension (see KG naming rule: XLayer
// additions to core live in a dedicated _xlayer.go file). It must not contain
// upstream go-ethereum logic.

package core

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// blacklistMirrorABIJSON is the read-only ABI the node uses to enumerate the
// L2BlacklistMirror list. The node depends ONLY on this ABI, never on the
// contract's storage layout — so the contract may use any internal layout
// (e.g. OpenZeppelin EnumerableSet) and change it freely. This ABI is a shared
// contract with xlayer-reth and the contracts repo: both clients must call the
// identical functions and any change must be synchronized three ways.
//   - entryCount() -> uint256
//   - valuesPaginated(start, limit) -> address[]  (bounded; never expose values())
const blacklistMirrorABIJSON = `[` +
	`{"type":"function","name":"entryCount","stateMutability":"view","inputs":[],"outputs":[{"type":"uint256"}]},` +
	`{"type":"function","name":"valuesPaginated","stateMutability":"view","inputs":[{"type":"uint256"},{"type":"uint256"}],"outputs":[{"type":"address[]"}]}` +
	`]`

var blacklistMirrorABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(blacklistMirrorABIJSON))
	if err != nil {
		panic("xlayer blacklist: invalid mirror ABI: " + err.Error())
	}
	return parsed
}()

// Deterministic read parameters — MUST be identical across op-geth and
// xlayer-reth (different values would build different sets → consensus fork).
const (
	// blacklistReadPageSize is how many addresses are requested per
	// valuesPaginated call.
	blacklistReadPageSize = 1024
	// blacklistReadGas is the (ample, fixed) gas budget for each read
	// staticcall. A view enumerating one page stays well under this.
	blacklistReadGas = 50_000_000
)

// maxSnapshotEntries bounds how many entries ReadBlacklistSnapshot will read in
// a single block, defending against a corrupt/pathological length word causing
// unbounded work. The cap is deterministic (identical on every client) and far
// above the PRD engineering upper bound of 300k addresses, so it never affects
// well-formed lists. If a list legitimately exceeds this, it must be raised in
// lockstep across op-geth and xlayer-reth.
const maxSnapshotEntries = 1 << 20

// Snapshot is an immutable, block-head view of the blacklist address set. It is
// safe for concurrent reads (never mutated after construction).
type Snapshot struct {
	set       map[common.Address]struct{}
	blockHash common.Hash
}

// NewSnapshot builds a Snapshot from the given addresses. Used by the ingress
// filter refresh path and by tests; the execution gate uses
// ReadBlacklistSnapshot.
func NewSnapshot(addrs []common.Address) *Snapshot {
	set := make(map[common.Address]struct{}, len(addrs))
	for _, a := range addrs {
		set[a] = struct{}{}
	}
	return &Snapshot{set: set}
}

// Contains reports whether the address is on the blacklist snapshot. A nil
// snapshot contains nothing (treated as empty / no-op).
func (s *Snapshot) Contains(a common.Address) bool {
	if s == nil {
		return false
	}
	_, ok := s.set[a]
	return ok
}

// Size returns the number of blacklisted addresses in the snapshot. A nil
// snapshot has size 0.
func (s *Snapshot) Size() int {
	if s == nil {
		return 0
	}
	return len(s.set)
}

// BlockHash returns the block hash this snapshot was read at (zero if unset).
func (s *Snapshot) BlockHash() common.Hash {
	if s == nil {
		return common.Hash{}
	}
	return s.blockHash
}

// ReadBlacklistSnapshot reads the blacklist address set from the L2BlacklistMirror
// contract for the given chain_id, by calling its read-only view ABI
// (entryCount + valuesPaginated) — NOT by reading raw storage slots. This
// decouples the node from the contract's storage layout (B-3 resolution): the
// contract may use any internal representation behind the fixed ABI.
//
// statedb MUST be the parent / block-head state (never mid-block live state), so
// an add landing in block N is only visible from block N+1 — no in-block delta
// (FR-4). The whole list is read once per block and reused for every tx.
//
// Behavior:
//   - chain not enabled (BlacklistMirror !ok)        → empty snapshot (no call)
//   - mirror not deployed (no code) / entryCount==0  → empty snapshot, no-op
//   - non-empty list                                 → populated snapshot
//   - view call / decode failure                     → empty snapshot + Error log
//     (deterministic across clients; a well-formed mirror never fails. The exact
//     failure policy is a cross-client agreed item — keep op-geth and reth identical.)
func ReadBlacklistSnapshot(statedb vm.StateDB, header *types.Header, config *params.ChainConfig, chainID uint64) *Snapshot {
	mirror, ok := params.BlacklistMirror(chainID)
	if !ok {
		return emptySnapshot()
	}
	// Not deployed → a staticcall to a code-less account returns empty; short-circuit.
	if statedb.GetCodeSize(mirror) == 0 {
		return emptySnapshot()
	}
	evm := newReadOnlyEVM(statedb, header, config)
	call := func(input []byte) ([]byte, error) {
		ret, _, err := evm.StaticCall(params.SystemAddress, mirror, input, blacklistReadGas)
		return ret, err
	}
	return readBlacklistSet(chainID, call)
}

func emptySnapshot() *Snapshot { return &Snapshot{set: map[common.Address]struct{}{}} }

// blacklistViewCall performs a read-only call to the mirror with the given ABI
// calldata and returns the raw output. Abstracted so the ABI decode + pagination
// logic (readBlacklistSet) is unit-testable without deploying contract bytecode.
type blacklistViewCall func(input []byte) ([]byte, error)

// readBlacklistSet enumerates the mirror via entryCount + valuesPaginated using
// the supplied caller, applies the deterministic cap, skips zero entries, and
// returns the snapshot. Any call/decode failure yields an empty snapshot + Error
// log (deterministic across clients; a well-formed mirror never fails).
func readBlacklistSet(chainID uint64, call blacklistViewCall) *Snapshot {
	countData, err := blacklistMirrorABI.Pack("entryCount")
	if err != nil { // static ABI, cannot fail in practice
		panic("xlayer blacklist: pack entryCount: " + err.Error())
	}
	ret, err := call(countData)
	if err != nil {
		log.Error("XLayer blacklist: entryCount call failed (treating as empty)", "chainID", chainID, "err", err)
		return emptySnapshot()
	}
	out, err := blacklistMirrorABI.Unpack("entryCount", ret)
	if err != nil || len(out) == 0 {
		log.Error("XLayer blacklist: entryCount decode failed (treating as empty)", "chainID", chainID, "err", err)
		return emptySnapshot()
	}
	count, ok := out[0].(*big.Int)
	if !ok || count.Sign() == 0 {
		return emptySnapshot()
	}

	n := count.Uint64()
	if !count.IsUint64() || n > maxSnapshotEntries {
		log.Warn("XLayer blacklist: mirror entryCount exceeds cap, truncating",
			"chainID", chainID, "count", count.String(), "cap", maxSnapshotEntries)
		n = maxSnapshotEntries
	}

	set := make(map[common.Address]struct{}, n)
	for start := uint64(0); start < n; start += blacklistReadPageSize {
		limit := uint64(blacklistReadPageSize)
		if start+limit > n {
			limit = n - start
		}
		pageData, err := blacklistMirrorABI.Pack("valuesPaginated", new(big.Int).SetUint64(start), new(big.Int).SetUint64(limit))
		if err != nil {
			panic("xlayer blacklist: pack valuesPaginated: " + err.Error())
		}
		pret, err := call(pageData)
		if err != nil {
			log.Error("XLayer blacklist: valuesPaginated call failed (treating as empty)", "chainID", chainID, "start", start, "err", err)
			return emptySnapshot()
		}
		pout, err := blacklistMirrorABI.Unpack("valuesPaginated", pret)
		if err != nil || len(pout) == 0 {
			log.Error("XLayer blacklist: valuesPaginated decode failed (treating as empty)", "chainID", chainID, "start", start, "err", err)
			return emptySnapshot()
		}
		addrs, ok := pout[0].([]common.Address)
		if !ok {
			log.Error("XLayer blacklist: valuesPaginated unexpected type (treating as empty)", "chainID", chainID)
			return emptySnapshot()
		}
		for _, a := range addrs {
			if a == (common.Address{}) {
				continue // zero entries are not valid members (defensive)
			}
			set[a] = struct{}{}
		}
		if len(addrs) == 0 {
			break // contract returned fewer than expected; stop to avoid a spin
		}
	}
	return &Snapshot{set: set}
}

// newReadOnlyEVM builds a minimal EVM over the given (block-head) state for a
// read-only staticcall. It hand-builds the BlockContext (no ChainContext
// needed) so all call sites — including the txpool, which has no EVM — can use
// it. GetHash is a stub: a view enumerator never executes BLOCKHASH.
func newReadOnlyEVM(statedb vm.StateDB, header *types.Header, config *params.ChainConfig) *vm.EVM {
	baseFee := new(big.Int)
	if header.BaseFee != nil {
		baseFee = new(big.Int).Set(header.BaseFee)
	}
	blockCtx := vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		Coinbase:    header.Coinbase,
		GasLimit:    header.GasLimit,
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        header.Time,
		BaseFee:     baseFee,
	}
	// CRITICAL: vm.NewEVM derives the fork rules (and thus the instruction set)
	// from `Random != nil` (= isMerge). A post-merge block has Difficulty==0 and
	// carries randomness in MixDigest; without Random set, isMerge=false would
	// drop the EVM to a pre-Shanghai/pre-Bedrock instruction set and a modern
	// mirror (PUSH0 etc.) would hit an invalid opcode → staticcall revert →
	// fail-open empty list. Mirror core.NewEVMBlockContext exactly.
	if header.Difficulty == nil || header.Difficulty.Sign() == 0 {
		random := header.MixDigest
		blockCtx.Random = &random
	} else {
		blockCtx.Difficulty = new(big.Int).Set(header.Difficulty)
	}
	return vm.NewEVM(blockCtx, statedb, config, vm.Config{})
}
