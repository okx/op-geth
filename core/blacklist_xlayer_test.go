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
// list, encoding entryCount/valuesPaginated outputs exactly as the contract ABI
// would — so readBlacklistSet's decode/pagination logic is exercised without
// deploying contract bytecode.
func fakeMirrorCall(list []common.Address) blacklistViewCall {
	return func(input []byte) ([]byte, error) {
		if len(input) < 4 {
			return nil, errors.New("short input")
		}
		switch {
		case bytes.Equal(input[:4], blacklistMirrorABI.Methods["entryCount"].ID):
			return blacklistMirrorABI.Methods["entryCount"].Outputs.Pack(big.NewInt(int64(len(list))))
		case bytes.Equal(input[:4], blacklistMirrorABI.Methods["valuesPaginated"].ID):
			args, err := blacklistMirrorABI.Methods["valuesPaginated"].Inputs.Unpack(input[4:])
			if err != nil {
				return nil, err
			}
			start := args[0].(*big.Int).Uint64()
			limit := args[1].(*big.Int).Uint64()
			n := uint64(len(list))
			if start >= n {
				return blacklistMirrorABI.Methods["valuesPaginated"].Outputs.Pack([]common.Address{})
			}
			end := min(start+limit, n)
			return blacklistMirrorABI.Methods["valuesPaginated"].Outputs.Pack(list[start:end])
		}
		return nil, errors.New("unknown selector")
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
	// One past a page boundary forces a second valuesPaginated call.
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
// (solc 0.8.30, --evm-version shanghai): entryCount()->2,
// valuesPaginated(start,limit)->[0xAA,0xBB] sliced. Compiled with PUSH0 so this
// test also guards the merge/Random regression — under a wrongly pre-Shanghai
// EVM the PUSH0 reverts and the read fails.
const mirrorStubRuntimeHex = "608060405234801561000f575f5ffd5b5060043610610034575f3560e01c80630cbb0f83146100385780639734aacd1461004c575b5f5ffd5b604051600281526020015b60405180910390f35b61005f61005a36600461016f565b61006c565b604051610043919061018f565b6040805180820190915260aa815260bb6020820152606090600284106100a1575050604080515f815260208101909152610169565b5f6100ac8486610202565b905060028111156100bb575060025b6100c58582610215565b67ffffffffffffffff8111156100dd576100dd6101da565b604051908082528060200260200182016040528015610106578160200160208202803683370190505b509250845b818110156101655782816002811061012557610125610228565b6020020151846101358884610215565b8151811061014557610145610228565b6001600160a01b039092166020928302919091019091015260010161010b565b5050505b92915050565b5f5f60408385031215610180575f5ffd5b50508035926020909101359150565b602080825282518282018190525f918401906040840190835b818110156101cf5783516001600160a01b03168352602093840193909201916001016101a8565b509095945050505050565b634e487b7160e01b5f52604160045260245ffd5b634e487b7160e01b5f52601160045260245ffd5b80820180821115610169576101696101ee565b81810381811115610169576101696101ee565b634e487b7160e01b5f52603260045260245ffdfea2646970667358221220c041959d685fce9661dd53ccc04a009113cfe11b0aea3d0571bc9524d7c5a07064736f6c634300081e0033"

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
