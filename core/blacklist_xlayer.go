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
// L2BlacklistMirror list (XLOP-1100 cross-client contract). The node depends
// ONLY on this ABI, never on the contract's storage layout — so the contract
// may use any internal layout (e.g. OpenZeppelin EnumerableSet) and change it
// freely. This ABI is shared three ways with xlayer-reth and the contracts
// repo: all must use the identical method and any change is synchronized.
//
//	getBlacklist(uint256 start, uint256 limit)
//	    view returns (uint256 total, address[] addresses)
//
// One bounded call returns the total count AND a page of addresses atomically:
//   - total: total number of entries, independent of start/limit
//   - addresses: up to `limit` entries from index `start`; fewer if the tail is
//     shorter; empty when start >= total
//
// (never expose an unbounded values()).
const blacklistMirrorABIJSON = `[` +
	`{"type":"function","name":"getBlacklist","stateMutability":"view","inputs":[{"type":"uint256"},{"type":"uint256"}],"outputs":[{"type":"uint256"},{"type":"address[]"}]}` +
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
	// getBlacklist call.
	blacklistReadPageSize = 1024
	// blacklistReadGas is the (ample, fixed) gas budget for each read
	// staticcall. A view enumerating one page stays well under this.
	blacklistReadGas = 50_000_000
)

// maxSnapshotEntries bounds how many entries ReadBlacklistSnapshot will read in
// a single block, defending against a corrupt/pathological total word causing
// unbounded work. The cap is deterministic and MUST be identical on every
// client (XLOP-1100): a different cap would let one client read more entries
// than another and fork. It equals the PRD engineering upper bound (300k) and
// matches xlayer-reth; raising it requires a lockstep change in both clients.
const maxSnapshotEntries = 300000

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
// (getBlacklist) — NOT by reading raw storage slots. This decouples the node
// from the contract's storage layout (B-3 resolution): the contract may use any
// internal representation behind the fixed ABI.
//
// statedb MUST be the parent / block-head state (never mid-block live state), so
// an add landing in block N is only visible from block N+1 — no in-block delta
// (FR-4). The whole list is read once per block and reused for every tx.
//
// Behavior:
//   - chain not enabled (BlacklistMirror !ok)       → empty snapshot (no call)
//   - mirror not deployed (no code) / total==0      → empty snapshot, no-op
//   - non-empty list                                → populated snapshot
//   - view call / decode failure                    → empty snapshot + Error log
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

// readBlacklistSet enumerates the mirror via paged getBlacklist calls using the
// supplied caller, applies the deterministic cap, skips zero entries, and
// returns the snapshot. The first page yields the authoritative total; reads
// continue until `total` (capped) entries are consumed or the contract returns a
// short/empty page. Any call/decode failure yields an empty snapshot + Error log
// (deterministic across clients; a well-formed mirror never fails).
func readBlacklistSet(chainID uint64, call blacklistViewCall) *Snapshot {
	total, page, ok := getBlacklistPage(chainID, call, 0)
	if !ok {
		return emptySnapshot()
	}
	if total == 0 {
		return emptySnapshot()
	}
	n := total
	if n > maxSnapshotEntries {
		log.Warn("XLayer blacklist: mirror total exceeds cap, truncating",
			"chainID", chainID, "total", total, "cap", uint64(maxSnapshotEntries))
		n = maxSnapshotEntries
	}

	set := make(map[common.Address]struct{}, n)
	var read uint64
	accumulatePage(set, page, &read, n)
	// `read` (entries consumed so far) doubles as the next page offset. Driving
	// the offset off actual consumption — not a fixed PAGE_SIZE step — keeps the
	// read self-consistent even if the contract returns a short non-final page:
	// a client can never silently skip the [read, read+PAGE_SIZE) gap, which
	// would otherwise risk reading a different set than xlayer-reth (a different
	// pagination cursor on a misbehaving mirror = consensus fork).
	for read < n {
		_, page, ok := getBlacklistPage(chainID, call, read)
		if !ok {
			return emptySnapshot()
		}
		if len(page) == 0 {
			break // contract returned fewer than total; stop to avoid a spin
		}
		before := read
		accumulatePage(set, page, &read, n)
		if read == before {
			break // non-empty page consumed nothing (all beyond cap); avoid a spin
		}
	}
	return &Snapshot{set: set}
}

// getBlacklistPage performs one getBlacklist(start, pageSize) call and returns
// the reported total and the page of addresses. ok=false (with an Error log) on
// any call/decode failure, so the caller fails open to an empty snapshot.
func getBlacklistPage(chainID uint64, call blacklistViewCall, start uint64) (uint64, []common.Address, bool) {
	data, err := blacklistMirrorABI.Pack("getBlacklist", new(big.Int).SetUint64(start), new(big.Int).SetUint64(blacklistReadPageSize))
	if err != nil { // static ABI, cannot fail in practice
		panic("xlayer blacklist: pack getBlacklist: " + err.Error())
	}
	ret, err := call(data)
	if err != nil {
		log.Error("XLayer blacklist: getBlacklist call failed (treating as empty)", "chainID", chainID, "start", start, "err", err)
		return 0, nil, false
	}
	out, err := blacklistMirrorABI.Unpack("getBlacklist", ret)
	if err != nil || len(out) < 2 {
		log.Error("XLayer blacklist: getBlacklist decode failed (treating as empty)", "chainID", chainID, "start", start, "err", err)
		return 0, nil, false
	}
	total, ok := out[0].(*big.Int)
	if !ok || !total.IsUint64() {
		log.Error("XLayer blacklist: getBlacklist total invalid (treating as empty)", "chainID", chainID, "start", start)
		return 0, nil, false
	}
	addrs, ok := out[1].([]common.Address)
	if !ok {
		log.Error("XLayer blacklist: getBlacklist addresses invalid type (treating as empty)", "chainID", chainID, "start", start)
		return 0, nil, false
	}
	return total.Uint64(), addrs, true
}

// accumulatePage adds up to (n-*read) addresses from page into set, skipping
// zero entries, and advances *read by the number of entry slots consumed. The
// slot count (not the post-skip size) drives truncation so every client stops
// at the same entry index when n caps a list.
func accumulatePage(set map[common.Address]struct{}, page []common.Address, read *uint64, n uint64) {
	for _, a := range page {
		if *read >= n {
			return
		}
		*read++
		if a == (common.Address{}) {
			continue // zero entries are not valid members (defensive)
		}
		set[a] = struct{}{}
	}
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
