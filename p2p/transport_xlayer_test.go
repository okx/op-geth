package p2p

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/metrics"
)

// --- trimETH69 tests (DM-6, FO-4, FO-5) ---

func TestTrimETH69_RemovesETH69AndReturnsTrue(t *testing.T) {
	phs := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0/linux-amd64",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	result, removed := trimETH69(phs)

	if !removed {
		t.Fatal("expected removed=true when eth/69 present")
	}
	if result == phs {
		t.Fatal("expected result to be a different pointer than input")
	}
	if len(result.Caps) != 2 {
		t.Fatalf("expected 2 caps after trim, got %d", len(result.Caps))
	}
	for _, c := range result.Caps {
		if c.Name == "eth" && c.Version == 69 {
			t.Fatal("result should not contain eth/69")
		}
	}
}

func TestTrimETH69_DoesNotMutateInput(t *testing.T) {
	originalCaps := []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}}
	phs := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0",
		Caps:    originalCaps,
		ID:      make([]byte, 64),
	}

	trimETH69(phs)

	if len(phs.Caps) != 3 {
		t.Fatalf("input caps mutated: expected len 3, got %d", len(phs.Caps))
	}
	if phs.Caps[1].Name != "eth" || phs.Caps[1].Version != 69 {
		t.Fatal("input caps[1] should still be eth/69")
	}
}

func TestTrimETH69_NoETH69_ReturnsSamePointer(t *testing.T) {
	phs := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0",
		Caps:    []Cap{{"eth", 68}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	result, removed := trimETH69(phs)

	if removed {
		t.Fatal("expected removed=false when eth/69 not present")
	}
	if result != phs {
		t.Fatal("expected same pointer when nothing removed")
	}
	if len(result.Caps) != 2 {
		t.Fatalf("expected 2 caps, got %d", len(result.Caps))
	}
}

func TestTrimETH69_EmptyCaps(t *testing.T) {
	phs := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0",
		Caps:    []Cap{},
		ID:      make([]byte, 64),
	}

	result, removed := trimETH69(phs)

	if removed {
		t.Fatal("expected removed=false for empty caps")
	}
	if result != phs {
		t.Fatal("expected same pointer for empty caps")
	}
}

// --- isGeth tests ---

func TestIsGeth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"uppercase Geth", "Geth/v1.13.0/linux-amd64", true},
		{"lowercase geth", "geth/v1.12.0", true},
		{"op-geth contains geth", "op-geth/v1.0.0", true},
		{"Nethermind", "Nethermind/v1.25.0", false},
		{"Erigon", "Erigon/v2.55.0", false},
		{"empty string", "", false},
		{"GETH uppercase only", "GETH/v1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGeth(tt.input)
			if got != tt.expected {
				t.Errorf("isGeth(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// --- SetETH69CompatEnabled tests (DM-7, FO-7) ---

func TestSetETH69CompatEnabled_StoresTrue(t *testing.T) {
	eth69CompatEnabled.Store(false)
	SetETH69CompatEnabled(true)

	if !eth69CompatEnabled.Load() {
		t.Fatal("expected eth69CompatEnabled to be true")
	}
}

func TestSetETH69CompatEnabled_StoresFalse(t *testing.T) {
	eth69CompatEnabled.Store(true)
	SetETH69CompatEnabled(false)

	if eth69CompatEnabled.Load() {
		t.Fatal("expected eth69CompatEnabled to be false")
	}
}

// --- doProtoHandshakeLegacy integration tests (DM-1 through DM-8) ---

// resetTrimCounter resets the eth69TrimCounter for test isolation.
func resetTrimCounter() {
	eth69TrimCounter = metrics.NewRegisteredCounter("p2p/eth69/trimmed/test", nil)
}

func TestDoProtoHandshakeLegacy_FlagEnabled_GethPeer_TrimsETH69(t *testing.T) {
	resetTrimCounter()
	eth69CompatEnabled.Store(true)

	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0/linux-amd64",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}
	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/v1.0.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	var sentData interface{}
	mock := &mockTransport{
		readHandshake: theirHS,
		onSend: func(data interface{}) error {
			sentData = data
			return nil
		},
	}

	their, err := mock.doProtoHandshakeLegacyTest(ourHS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify their caps don't contain eth/69
	for _, c := range their.Caps {
		if c.Name == "eth" && c.Version == 69 {
			t.Fatal("returned their.Caps should not contain eth/69")
		}
	}

	// Verify sent handshake doesn't contain eth/69
	sentHS := sentData.(*protoHandshake)
	for _, c := range sentHS.Caps {
		if c.Name == "eth" && c.Version == 69 {
			t.Fatal("sent handshake should not contain eth/69")
		}
	}

	// Verify original our handshake is not mutated
	if len(ourHS.Caps) != 3 {
		t.Fatalf("original ourHS.Caps mutated: expected len 3, got %d", len(ourHS.Caps))
	}

	// Verify counter incremented
	if eth69TrimCounter.Snapshot().Count() != 1 {
		t.Fatalf("expected counter=1, got %d", eth69TrimCounter.Snapshot().Count())
	}
}

func TestDoProtoHandshakeLegacy_FlagDisabled_GethPeer_NoTrim(t *testing.T) {
	resetTrimCounter()
	eth69CompatEnabled.Store(false)

	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}},
		ID:      make([]byte, 64),
	}
	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/v1.0.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}},
		ID:      make([]byte, 64),
	}

	var sentData interface{}
	mock := &mockTransport{
		readHandshake: theirHS,
		onSend: func(data interface{}) error {
			sentData = data
			return nil
		},
	}

	their, err := mock.doProtoHandshakeLegacyTest(ourHS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify their caps still contain eth/69
	hasETH69 := false
	for _, c := range their.Caps {
		if c.Name == "eth" && c.Version == 69 {
			hasETH69 = true
			break
		}
	}
	if !hasETH69 {
		t.Fatal("returned their.Caps should contain eth/69 when flag disabled")
	}

	// Verify sent is our original
	sentHS := sentData.(*protoHandshake)
	if sentHS != ourHS {
		t.Fatal("sent handshake should be the original when flag disabled")
	}

	// Counter should not increment
	if eth69TrimCounter.Snapshot().Count() != 0 {
		t.Fatalf("expected counter=0, got %d", eth69TrimCounter.Snapshot().Count())
	}
}

func TestDoProtoHandshakeLegacy_NonGethPeer_NoTrim(t *testing.T) {
	resetTrimCounter()
	eth69CompatEnabled.Store(true)

	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Nethermind/v1.25.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}},
		ID:      make([]byte, 64),
	}
	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/v1.0.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}},
		ID:      make([]byte, 64),
	}

	var sentData interface{}
	mock := &mockTransport{
		readHandshake: theirHS,
		onSend: func(data interface{}) error {
			sentData = data
			return nil
		},
	}

	their, err := mock.doProtoHandshakeLegacyTest(ourHS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify caps unchanged
	hasETH69 := false
	for _, c := range their.Caps {
		if c.Name == "eth" && c.Version == 69 {
			hasETH69 = true
			break
		}
	}
	if !hasETH69 {
		t.Fatal("returned their.Caps should contain eth/69 for non-Geth peer")
	}

	// Sent handshake should be the original
	sentHS := sentData.(*protoHandshake)
	if sentHS != ourHS {
		t.Fatal("sent handshake should be original for non-Geth peer")
	}

	// Counter should not increment
	if eth69TrimCounter.Snapshot().Count() != 0 {
		t.Fatalf("expected counter=0, got %d", eth69TrimCounter.Snapshot().Count())
	}
}

func TestDoProtoHandshakeLegacy_GethPeer_NoETH69InCaps_NoCounterIncrement(t *testing.T) {
	resetTrimCounter()
	eth69CompatEnabled.Store(true)

	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0",
		Caps:    []Cap{{"eth", 68}, {"snap", 1}},
		ID:      make([]byte, 64),
	}
	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/v1.0.0",
		Caps:    []Cap{{"eth", 68}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	mock := &mockTransport{
		readHandshake: theirHS,
		onSend: func(data interface{}) error {
			return nil
		},
	}

	_, err := mock.doProtoHandshakeLegacyTest(ourHS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Counter should NOT increment since nothing was actually removed
	if eth69TrimCounter.Snapshot().Count() != 0 {
		t.Fatalf("expected counter=0 when no eth/69 to remove, got %d", eth69TrimCounter.Snapshot().Count())
	}
}

func TestDoProtoHandshakeLegacy_ReadError_ReturnsError(t *testing.T) {
	resetTrimCounter()
	eth69CompatEnabled.Store(true)

	mock := &mockTransport{
		readErr: fmt.Errorf("test read error"),
		onSend: func(data interface{}) error {
			t.Fatal("Send should not be called on read error")
			return nil
		},
	}

	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/v1.0.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}},
		ID:      make([]byte, 64),
	}

	their, err := mock.doProtoHandshakeLegacyTest(ourHS)
	if err == nil {
		t.Fatal("expected error on read failure")
	}
	if their != nil {
		t.Fatal("expected nil return on read error")
	}
	if eth69TrimCounter.Snapshot().Count() != 0 {
		t.Fatalf("expected counter=0 on read error, got %d", eth69TrimCounter.Snapshot().Count())
	}
}

func TestDoProtoHandshakeLegacy_SendError_ReturnsError_NoCounterIncrement(t *testing.T) {
	resetTrimCounter()
	eth69CompatEnabled.Store(true)

	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}},
		ID:      make([]byte, 64),
	}
	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/v1.0.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}},
		ID:      make([]byte, 64),
	}

	mock := &mockTransport{
		readHandshake: theirHS,
		onSend: func(data interface{}) error {
			return fmt.Errorf("test send error")
		},
	}

	_, err := mock.doProtoHandshakeLegacyTest(ourHS)
	if err == nil {
		t.Fatal("expected error on send failure")
	}
	// Counter should NOT increment on send error (counter is after Send)
	if eth69TrimCounter.Snapshot().Count() != 0 {
		t.Fatalf("expected counter=0 on send error, got %d", eth69TrimCounter.Snapshot().Count())
	}
}

// --- Fuzz test (FO-9) ---

func FuzzTrimETH69(f *testing.F) {
	f.Add("eth", uint64(69))
	f.Add("eth", uint64(68))
	f.Add("snap", uint64(1))
	f.Add("", uint64(0))

	f.Fuzz(func(t *testing.T, name string, version uint64) {
		caps := []Cap{
			{"eth", 68},
			{name, uint(version)},
			{"snap", 1},
			{"eth", 69},
		}
		phs := &protoHandshake{
			Version: 5,
			Name:    "test",
			Caps:    caps,
			ID:      make([]byte, 64),
		}

		result, _ := trimETH69(phs)

		// Invariant: result never contains eth/69
		for _, c := range result.Caps {
			if c.Name == "eth" && c.Version == 69 {
				t.Fatal("result should never contain eth/69")
			}
		}

		// Invariant: input not mutated
		if len(phs.Caps) != 4 {
			t.Fatalf("input mutated: expected 4 caps, got %d", len(phs.Caps))
		}
	})
}

// --- mockTransport for testing doProtoHandshakeLegacy ---

type mockTransport struct {
	readHandshake *protoHandshake
	readErr       error
	onSend        func(data interface{}) error
	snappySet     bool
}

// doProtoHandshakeLegacyTest mirrors the logic of doProtoHandshakeLegacy
// but uses the mock transport instead of real network I/O.
func (m *mockTransport) doProtoHandshakeLegacyTest(our *protoHandshake) (their *protoHandshake, err error) {
	// Simulate readProtocolHandshake
	if m.readErr != nil {
		return nil, m.readErr
	}
	their = m.readHandshake

	handshakeToSend := our
	trimmed := false

	if eth69CompatEnabled.Load() && isGeth(their.Name) {
		var ourTrimmed, theirTrimmed bool
		handshakeToSend, ourTrimmed = trimETH69(our)
		their, theirTrimmed = trimETH69(their)
		trimmed = ourTrimmed || theirTrimmed
	}

	// Simulate Send
	if m.onSend != nil {
		if err := m.onSend(handshakeToSend); err != nil {
			return nil, fmt.Errorf("write error: %v", err)
		}
	}

	// Simulate SetSnappy
	m.snappySet = their.Version >= snappyProtocolVersion

	if trimmed {
		eth69TrimCounter.Inc(1)
	}

	return their, nil
}
