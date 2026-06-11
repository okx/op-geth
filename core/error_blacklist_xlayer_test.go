package core

import "testing"

// TestErrBlacklisted_Message pins the externally-observable error string (FR-7).
// eth_sendRawTransaction surfaces this verbatim; it must carry no address or
// list-size (no dynamic fields) to avoid leaking blacklist contents.
func TestErrBlacklisted_Message(t *testing.T) {
	const want = "xlayer-blacklist: sender or recipient is on the blacklist"
	if got := ErrBlacklisted.Error(); got != want {
		t.Fatalf("ErrBlacklisted.Error() = %q, want %q", got, want)
	}
}

// TestErrBlacklisted_NoCustomErrorCode is a contract proxy for "RPC code ==
// -32000". The JSON-RPC layer (rpc/json.go) assigns errcodeDefault (-32000) to
// any returned error that does NOT implement an ErrorCode() int method. By
// asserting ErrBlacklisted does not implement that interface, we lock in the
// -32000 mapping without importing rpc (avoids an import cycle). The exact -32000
// value is additionally covered in package rpc.
func TestErrBlacklisted_NoCustomErrorCode(t *testing.T) {
	type errorCoder interface{ ErrorCode() int }
	if _, ok := error(ErrBlacklisted).(errorCoder); ok {
		t.Fatal("ErrBlacklisted implements ErrorCode(); it must NOT, so RPC defaults to -32000")
	}
}
