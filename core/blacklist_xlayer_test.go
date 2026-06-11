package core

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// newTestStateDB builds an empty in-memory StateDB for module tests.
func newTestStateDB(t *testing.T) *state.StateDB {
	t.Helper()
	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	return sdb
}

// fakeMirrorCall returns a blacklistViewCall backed by an in-memory address
// list, encoding getBlacklist(start,limit)->(total,addresses) exactly as the
// contract ABI would — so readBlacklistSet's decode/pagination logic is
// exercised without deploying contract bytecode.
func fakeMirrorCall(list []common.Address) blacklistViewCall {
	return func(input []byte) ([]byte, error) {
		if len(input) < 4 {
			return nil, errors.New("short input")
		}
		m := blacklistMirrorABI.Methods["getBlacklist"]
		if !bytes.Equal(input[:4], m.ID) {
			return nil, errors.New("unknown selector")
		}
		args, err := m.Inputs.Unpack(input[4:])
		if err != nil {
			return nil, err
		}
		start := args[0].(*big.Int).Uint64()
		limit := args[1].(*big.Int).Uint64()
		total := big.NewInt(int64(len(list)))
		n := uint64(len(list))
		if start >= n {
			return m.Outputs.Pack(total, []common.Address{})
		}
		end := min(start+limit, n)
		return m.Outputs.Pack(total, list[start:end])
	}
}

func addrN(i int) common.Address { return common.BigToAddress(big.NewInt(int64(i))) }

func TestReadBlacklistSet_NonEmpty(t *testing.T) {
	aaa := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	bbb := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	snap := readBlacklistSet(params.XLayerMainnetChainID, fakeMirrorCall([]common.Address{aaa, bbb}))
	if snap.Size() != 2 {
		t.Fatalf("Size = %d, want 2", snap.Size())
	}
	if !snap.Contains(aaa) || !snap.Contains(bbb) {
		t.Fatal("snapshot missing expected entries")
	}
	if snap.Contains(common.HexToAddress("0x00000000000000000000000000000000000000CC")) {
		t.Fatal("snapshot contains unexpected entry")
	}
}

func TestReadBlacklistSet_EmptyCount(t *testing.T) {
	if got := readBlacklistSet(params.XLayerMainnetChainID, fakeMirrorCall(nil)).Size(); got != 0 {
		t.Fatalf("empty-list Size = %d, want 0", got)
	}
}

func TestReadBlacklistSet_MultiPage(t *testing.T) {
	// One past a page boundary forces a second getBlacklist call.
	n := blacklistReadPageSize + 1
	list := make([]common.Address, n)
	for i := range n {
		list[i] = addrN(i + 1) // nonzero, distinct
	}
	snap := readBlacklistSet(params.XLayerMainnetChainID, fakeMirrorCall(list))
	if snap.Size() != n {
		t.Fatalf("Size = %d, want %d (must read across pages)", snap.Size(), n)
	}
	if !snap.Contains(addrN(1)) || !snap.Contains(addrN(n)) {
		t.Fatal("missing first/last entry across page boundary")
	}
}

func TestReadBlacklistSet_ZeroEntrySkipped(t *testing.T) {
	aaa := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	// list reports count=2 but includes a zero address that must be skipped.
	snap := readBlacklistSet(params.XLayerMainnetChainID, fakeMirrorCall([]common.Address{aaa, {}}))
	if snap.Size() != 1 {
		t.Fatalf("Size = %d, want 1 (zero entry skipped)", snap.Size())
	}
	if !snap.Contains(aaa) || snap.Contains(common.Address{}) {
		t.Fatal("zero address must be skipped, real address kept")
	}
}

func TestReadBlacklistSet_CallErrorEmpty(t *testing.T) {
	failing := func(input []byte) ([]byte, error) { return nil, errors.New("staticcall reverted") }
	if got := readBlacklistSet(params.XLayerMainnetChainID, failing).Size(); got != 0 {
		t.Fatalf("on call error Size = %d, want 0 (treat as empty)", got)
	}
}

func TestReadBlacklistSnapshot_NotDeployed(t *testing.T) {
	// No code at the mirror address → empty snapshot, no view call.
	sdb := newTestStateDB(t)
	hdr := &types.Header{Number: big.NewInt(1), Time: 1}
	cfg := buildPathTestConfig()
	if got := ReadBlacklistSnapshot(sdb, hdr, cfg, params.XLayerMainnetChainID).Size(); got != 0 {
		t.Fatalf("not-deployed Size = %d, want 0", got)
	}
}

func TestReadBlacklistSnapshot_DisabledChain(t *testing.T) {
	sdb := newTestStateDB(t)
	hdr := &types.Header{Number: big.NewInt(1), Time: 1}
	cfg := buildPathTestConfig()
	if got := ReadBlacklistSnapshot(sdb, hdr, cfg, 1 /* eth mainnet, not enabled */).Size(); got != 0 {
		t.Fatalf("disabled-chain Size = %d, want 0", got)
	}
}

// mirrorStubRuntimeHex is the runtime bytecode of a minimal MirrorStub
// (solc 0.8.30, --evm-version shanghai) implementing the XLOP-1100 contract:
// getBlacklist(start,limit) -> (total=2, [0xAA,0xBB] sliced by start/limit).
// Compiled with PUSH0 so this test also guards the merge/Random regression —
// under a wrongly pre-Shanghai EVM the PUSH0 reverts and the read fails.
const mirrorStubRuntimeHex = "608060405234801561000f575f5ffd5b5060043610610029575f3560e01c8063f1c0c3731461002d575b5f5ffd5b61004061003b366004610162565b610057565b60405161004e929190610182565b60405180910390f35b6040805180820190915260aa815260bb602082015260029060609082851061008e575050604080515f81526020810190915261015b565b5f6100998587610200565b905060028111156100a8575060025b5f6100b38783610219565b90508067ffffffffffffffff8111156100ce576100ce6101d8565b6040519080825280602002602001820160405280156100f7578160200160208202803683370190505b5093505f5b81811015610156578361010f828a610200565b6002811061011f5761011f61022c565b60200201518582815181106101365761013661022c565b6001600160a01b03909216602092830291909101909101526001016100fc565b505050505b9250929050565b5f5f60408385031215610173575f5ffd5b50508035926020909101359150565b5f60408201848352604060208401528084518083526060850191506020860192505f5b818110156101cc5783516001600160a01b03168352602093840193909201916001016101a5565b50909695505050505050565b634e487b7160e01b5f52604160045260245ffd5b634e487b7160e01b5f52601160045260245ffd5b80820180821115610213576102136101ec565b92915050565b81810381811115610213576102136101ec565b634e487b7160e01b5f52603260045260245ffdfea2646970667358221220d0ef18377e6092f1e40ad9bba5331ff0fa61679d6304509867922df8ebab8b7b64736f6c634300081e0033"

// TestReadBlacklistSnapshot_ViaDeployedMirror is the end-to-end read test: it
// deploys a real MirrorStub at the mirror address and drives the full
// ReadBlacklistSnapshot path (newReadOnlyEVM → evm.StaticCall → ABI round-trip).
// The config is merged (header Difficulty==0 → Random set) and Shanghai-active so
// the modern stub bytecode runs; this both validates the view read and guards the
// instruction-set/Random regression (a pre-Shanghai EVM would revert on PUSH0).
func TestReadBlacklistSnapshot_ViaDeployedMirror(t *testing.T) {
	sdb := newTestStateDB(t)
	mirror, ok := params.BlacklistMirror(params.XLayerMainnetChainID)
	if !ok {
		t.Fatal("chain not blacklist-enabled")
	}
	sdb.SetCode(mirror, common.FromHex(mirrorStubRuntimeHex), tracing.CodeChangeGenesis)

	cfg := &params.ChainConfig{
		ChainID:             big.NewInt(int64(params.XLayerMainnetChainID)),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		ShanghaiTime:        u64(0),
	}
	// Difficulty==0 → newReadOnlyEVM sets Random → isMerge true → Shanghai active.
	hdr := &types.Header{Number: big.NewInt(1), Time: 1, Difficulty: big.NewInt(0)}

	snap := ReadBlacklistSnapshot(sdb, hdr, cfg, params.XLayerMainnetChainID)
	if snap.Size() != 2 {
		t.Fatalf("Size = %d, want 2 (read from deployed mirror via view call)", snap.Size())
	}
	if !snap.Contains(common.BigToAddress(big.NewInt(0xAA))) || !snap.Contains(common.BigToAddress(big.NewInt(0xBB))) {
		t.Fatal("deployed-mirror snapshot missing expected entries")
	}
}

func TestSnapshotNilSafety(t *testing.T) {
	var s *Snapshot
	if s.Size() != 0 || s.Contains(common.Address{}) {
		t.Fatal("nil snapshot must be empty and contain nothing")
	}
}

// makeAddrs builds n distinct nonzero addresses (addrN(1..n)).
func makeAddrs(n int) []common.Address {
	out := make([]common.Address, n)
	for i := range n {
		out[i] = addrN(i + 1)
	}
	return out
}

// customMirrorCall decouples the reported total from the actually returned list
// and can inject a failure on the k-th call (failOnCall, 1-based; 0 = never).
// This makes the truncation / oversized-total / mid-page-failure branches —
// which fakeMirrorCall cannot reach (its total always equals len(list)) —
// testable.
func customMirrorCall(total *big.Int, list []common.Address, failOnCall int) blacklistViewCall {
	calls := 0
	return func(input []byte) ([]byte, error) {
		calls++
		if failOnCall > 0 && calls == failOnCall {
			return nil, errors.New("injected getBlacklist failure")
		}
		m := blacklistMirrorABI.Methods["getBlacklist"]
		if len(input) < 4 || !bytes.Equal(input[:4], m.ID) {
			return nil, errors.New("unknown selector")
		}
		args, err := m.Inputs.Unpack(input[4:])
		if err != nil {
			return nil, err
		}
		start := args[0].(*big.Int).Uint64()
		limit := args[1].(*big.Int).Uint64()
		n := uint64(len(list))
		if start >= n {
			return m.Outputs.Pack(total, []common.Address{})
		}
		end := min(start+limit, n)
		return m.Outputs.Pack(total, list[start:end])
	}
}

// TestAccumulatePage_SlotTruncation locks the consensus-critical truncation
// semantics (F2): the cap counts entry SLOTS, not post-skip set size, so every
// client stops at the same entry index regardless of how many zero entries it
// skipped. n=3 over [addr, zero, addr, addr, addr] must consume exactly 3 slots
// and keep the two nonzero entries among the first 3.
func TestAccumulatePage_SlotTruncation(t *testing.T) {
	set := make(map[common.Address]struct{})
	var read uint64
	page := []common.Address{addrN(1), {}, addrN(2), addrN(3), addrN(4)}
	accumulatePage(set, page, &read, 3)
	if read != 3 {
		t.Fatalf("read = %d, want 3 (slot count, not post-skip size)", read)
	}
	if len(set) != 2 {
		t.Fatalf("set size = %d, want 2", len(set))
	}
	if _, ok := set[addrN(1)]; !ok {
		t.Fatal("addrN(1) (slot 1) must be included")
	}
	if _, ok := set[addrN(2)]; !ok {
		t.Fatal("addrN(2) (slot 3) must be included")
	}
	if _, ok := set[addrN(3)]; ok {
		t.Fatal("addrN(3) (slot 4, beyond cap) must NOT be included")
	}
}

// TestReadBlacklistSet_SecondPageFailureEmpty (F3): a call failure on a later
// page (not just the first) must fail open to an empty snapshot.
func TestReadBlacklistSet_SecondPageFailureEmpty(t *testing.T) {
	list := makeAddrs(blacklistReadPageSize) // full first page
	total := big.NewInt(int64(blacklistReadPageSize + 10))
	snap := readBlacklistSet(params.XLayerMainnetChainID, customMirrorCall(total, list, 2))
	if snap.Size() != 0 {
		t.Fatalf("Size = %d, want 0 (fail-open on second-page failure)", snap.Size())
	}
}

// TestReadBlacklistSet_OverstatedTotalStops (F4): when the reported total
// exceeds what the contract can actually return, the read stops at the real end
// (empty page → break) without spinning.
func TestReadBlacklistSet_OverstatedTotalStops(t *testing.T) {
	list := makeAddrs(blacklistReadPageSize) // 1024 real entries
	snap := readBlacklistSet(params.XLayerMainnetChainID, customMirrorCall(big.NewInt(5000), list, 0))
	if snap.Size() != blacklistReadPageSize {
		t.Fatalf("Size = %d, want %d (stop at real end, no spin)", snap.Size(), blacklistReadPageSize)
	}
}

// TestReadBlacklistSet_OversizedTotalEmpty (F5): a total word that does not fit
// in uint64 (corrupt/pathological) is rejected → empty snapshot.
func TestReadBlacklistSet_OversizedTotalEmpty(t *testing.T) {
	huge := new(big.Int).Lsh(big.NewInt(1), 65) // 2^65 > uint64 max
	snap := readBlacklistSet(params.XLayerMainnetChainID, customMirrorCall(huge, makeAddrs(3), 0))
	if snap.Size() != 0 {
		t.Fatalf("Size = %d, want 0 (oversized total rejected)", snap.Size())
	}
}

// TestReadBlacklistSet_PageBoundaries (F6): exact page multiple (2 pages) and a
// non-multiple spanning 3+ pages both read the full set with correct ends.
func TestReadBlacklistSet_PageBoundaries(t *testing.T) {
	for _, n := range []int{blacklistReadPageSize * 2, blacklistReadPageSize*3 + 7} {
		list := makeAddrs(n)
		snap := readBlacklistSet(params.XLayerMainnetChainID, customMirrorCall(big.NewInt(int64(n)), list, 0))
		if snap.Size() != n {
			t.Fatalf("n=%d: Size = %d, want %d", n, snap.Size(), n)
		}
		if !snap.Contains(addrN(1)) || !snap.Contains(addrN(n)) {
			t.Fatalf("n=%d: missing first/last entry", n)
		}
	}
}

// TestReadBlacklistSet_ShortMidPageNoGap (F1): a misbehaving mirror returns a
// short NON-final first page (500 < PAGE_SIZE). Because the next offset is the
// consumed count (not a fixed PAGE_SIZE step), no entry in the would-be gap is
// skipped. Under the old fixed-step cursor this read would drop [500,1024).
func TestReadBlacklistSet_ShortMidPageNoGap(t *testing.T) {
	const totalN = 1500
	list := makeAddrs(totalN)
	total := big.NewInt(totalN)
	short := true
	call := func(input []byte) ([]byte, error) {
		m := blacklistMirrorABI.Methods["getBlacklist"]
		args, err := m.Inputs.Unpack(input[4:])
		if err != nil {
			return nil, err
		}
		start := args[0].(*big.Int).Uint64()
		n := uint64(len(list))
		if start >= n {
			return m.Outputs.Pack(total, []common.Address{})
		}
		step := uint64(blacklistReadPageSize)
		if short { // first page deliberately short though not final
			short = false
			step = 500
		}
		end := min(start+step, n)
		return m.Outputs.Pack(total, list[start:end])
	}
	snap := readBlacklistSet(params.XLayerMainnetChainID, call)
	if snap.Size() != totalN {
		t.Fatalf("Size = %d, want %d (no gap despite short middle page)", snap.Size(), totalN)
	}
	if !snap.Contains(addrN(501)) {
		t.Fatal("entry right after the short page was skipped (gap bug)")
	}
}
