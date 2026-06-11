package rpc

import (
	"testing"

	"github.com/ethereum/go-ethereum/core"
)

// TestErrorMessage_BlacklistDefaultsTo32000 locks the FR-7 contract end-to-end:
// core.ErrBlacklisted, carrying no custom ErrorCode(), is encoded by the
// JSON-RPC layer with code -32000 (errcodeDefault) and its message verbatim. A
// blacklist-rejected eth_sendRawTransaction thus returns -32000 with the fixed
// message and no dynamic fields.
func TestErrorMessage_BlacklistDefaultsTo32000(t *testing.T) {
	msg := errorMessage(core.ErrBlacklisted)
	if msg.Error == nil {
		t.Fatal("errorMessage produced no Error")
	}
	if msg.Error.Code != errcodeDefault {
		t.Fatalf("code = %d, want %d (errcodeDefault)", msg.Error.Code, errcodeDefault)
	}
	if msg.Error.Code != -32000 {
		t.Fatalf("code = %d, want -32000", msg.Error.Code)
	}
	const want = "xlayer-blacklist: sender or recipient is on the blacklist"
	if msg.Error.Message != want {
		t.Fatalf("message = %q, want %q", msg.Error.Message, want)
	}
}
