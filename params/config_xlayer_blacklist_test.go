package params

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestIsBlacklistEnabled(t *testing.T) {
	cases := []struct {
		name    string
		chainID uint64
		want    bool
	}{
		{"mainnet 196", XLayerMainnetChainID, true},
		{"testnet 1952", XLayerTestnetChainID, true},
		{"devnet 195", XLayerDevnetChainID, true},
		{"eth mainnet 1", 1, false},
		{"op mainnet 10", 10, false},
		{"zero", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBlacklistEnabled(tc.chainID); got != tc.want {
				t.Fatalf("IsBlacklistEnabled(%d) = %v, want %v", tc.chainID, got, tc.want)
			}
		})
	}
}

func TestBlacklistMirror(t *testing.T) {
	// returns (addr,true) for the three enabled chains, (_,false) otherwise.
	for _, id := range []uint64{XLayerDevnetChainID, XLayerTestnetChainID, XLayerMainnetChainID} {
		addr, ok := BlacklistMirror(id)
		if !ok {
			t.Fatalf("BlacklistMirror(%d) ok=false, want true", id)
		}
		if addr == (common.Address{}) {
			t.Fatalf("BlacklistMirror(%d) returned zero address", id)
		}
	}
	if _, ok := BlacklistMirror(1); ok {
		t.Fatalf("BlacklistMirror(1) ok=true, want false")
	}
}

func TestBlacklistMirrorAddressesDistinct(t *testing.T) {
	// Each enabled chain must map to a distinct mirror address.
	seen := map[common.Address]uint64{}
	for id, addr := range XLayerBlacklistMirrorAddress {
		if other, dup := seen[addr]; dup {
			t.Fatalf("chains %d and %d share mirror address %s", id, other, addr.Hex())
		}
		seen[addr] = id
	}
}

func TestIsDepositExemptSender(t *testing.T) {
	// system / L1-attributes deposit senders are exempt; others are not.
	if !IsDepositExemptSender(SystemAddress) {
		t.Fatalf("SystemAddress must be deposit-exempt")
	}
	if !IsDepositExemptSender(common.HexToAddress("0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAd0001")) {
		t.Fatalf("L1-attributes depositor must be deposit-exempt")
	}
	if IsDepositExemptSender(common.HexToAddress("0x00000000000000000000000000000000000000AA")) {
		t.Fatalf("arbitrary address must NOT be deposit-exempt")
	}
}
