package p2p

import (
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/pipes"
)

// --- Slice 1: trimETH69Counted pure function tests ---

func TestTrimETH69Counted_RemovesETH69(t *testing.T) {
	input := &protoHandshake{
		Version: 5,
		Name:    "test",
		Caps:    []Cap{{"eth", 67}, {"eth", 68}, {"eth", 69}, {"snap", 1}},
	}

	result, found := trimETH69Counted(input)

	if !found {
		t.Fatal("expected found=true when eth/69 is present")
	}
	for _, c := range result.Caps {
		if c.Name == "eth" && c.Version == 69 {
			t.Fatal("eth/69 should have been removed from result")
		}
	}
	if len(result.Caps) != 3 {
		t.Fatalf("expected 3 caps in result, got %d", len(result.Caps))
	}
}

func TestTrimETH69Counted_DoesNotMutateInput(t *testing.T) {
	input := &protoHandshake{
		Version: 5,
		Name:    "test",
		Caps:    []Cap{{"eth", 67}, {"eth", 68}, {"eth", 69}, {"snap", 1}},
	}
	originalLen := len(input.Caps)

	result, found := trimETH69Counted(input)

	if !found {
		t.Fatal("expected found=true")
	}
	if len(input.Caps) != originalLen {
		t.Fatalf("input.Caps mutated: expected len %d, got %d", originalLen, len(input.Caps))
	}
	if result == input {
		t.Fatal("returned pointer should differ from input when eth/69 was removed")
	}
}

func TestTrimETH69Counted_NoETH69Present(t *testing.T) {
	input := &protoHandshake{
		Version: 5,
		Name:    "test",
		Caps:    []Cap{{"eth", 68}, {"snap", 1}},
	}

	result, found := trimETH69Counted(input)

	if found {
		t.Fatal("expected found=false when eth/69 is not present")
	}
	if result != input {
		t.Fatal("should return original pointer when nothing removed")
	}
}

func TestTrimETH69Counted_OnlyETH69(t *testing.T) {
	input := &protoHandshake{
		Version: 5,
		Name:    "test",
		Caps:    []Cap{{"eth", 69}},
	}

	result, found := trimETH69Counted(input)

	if !found {
		t.Fatal("expected found=true")
	}
	if len(result.Caps) != 0 {
		t.Fatalf("expected empty caps, got %d", len(result.Caps))
	}
	if len(input.Caps) != 1 {
		t.Fatal("original must still have 1 cap")
	}
}

func TestTrimETH69Counted_PreservesOrder(t *testing.T) {
	input := &protoHandshake{
		Version: 5,
		Name:    "test",
		Caps:    []Cap{{"snap", 1}, {"eth", 68}, {"eth", 69}, {"les", 4}},
	}

	result, found := trimETH69Counted(input)

	if !found {
		t.Fatal("expected found=true")
	}
	expected := []Cap{{"snap", 1}, {"eth", 68}, {"les", 4}}
	if len(result.Caps) != len(expected) {
		t.Fatalf("expected %d caps, got %d", len(expected), len(result.Caps))
	}
	for i, c := range result.Caps {
		if c != expected[i] {
			t.Fatalf("cap[%d] mismatch: got %v, want %v", i, c, expected[i])
		}
	}
}

// --- Slice 2: isGeth tests ---

func TestIsGeth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"uppercase Geth", "Geth/v1.14.0/linux/go1.22", true},
		{"lowercase geth", "geth/v1.13.0/linux/go1.21", true},
		{"op-Geth", "op-Geth/v1.101.0", true},
		{"Nethermind", "Nethermind/v1.25.0", false},
		{"Besu", "Besu/v24.1.0", false},
		{"Erigon", "Erigon/3.0.0", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGeth(tt.input); got != tt.expected {
				t.Errorf("isGeth(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// --- Slice 3: SetETH69Compat and default state tests ---

func TestETH69CompatEnabled_DefaultTrue(t *testing.T) {
	// The init() function sets this to true
	if !eth69CompatEnabled.Load() {
		t.Fatal("expected eth69CompatEnabled default to be true")
	}
}

func TestSetETH69Compat_DisablesFlag(t *testing.T) {
	// Save original and restore after test
	original := eth69CompatEnabled.Load()
	defer eth69CompatEnabled.Store(original)

	SetETH69Compat(false)
	if eth69CompatEnabled.Load() {
		t.Fatal("expected eth69CompatEnabled to be false after SetETH69Compat(false)")
	}
}

func TestSetETH69Compat_EnablesFlag(t *testing.T) {
	// Save original and restore after test
	original := eth69CompatEnabled.Load()
	defer eth69CompatEnabled.Store(original)

	SetETH69Compat(false)
	SetETH69Compat(true)
	if !eth69CompatEnabled.Load() {
		t.Fatal("expected eth69CompatEnabled to be true after SetETH69Compat(true)")
	}
}

// --- Slice 4: doProtoHandshakeLegacy integration tests ---

func TestDoProtoHandshakeLegacy_FlagEnabled_GethPeer_WithETH69(t *testing.T) {
	// Save and restore flag state
	original := eth69CompatEnabled.Load()
	defer eth69CompatEnabled.Store(original)
	eth69CompatEnabled.Store(true)

	initialCount := eth69TrimmedCounter.Snapshot().Count()

	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/test/v1.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}
	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.14.0/linux/go1.22",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	their, sentHS := doLegacyHandshakeTest(t, ourHS, theirHS)

	// Verify: our sent handshake should NOT contain eth/69
	for _, c := range sentHS.Caps {
		if c.Name == "eth" && c.Version == 69 {
			t.Fatal("sent handshake should not contain eth/69 when flag=true and peer is Geth")
		}
	}

	// Verify: their returned caps should NOT contain eth/69
	for _, c := range their.Caps {
		if c.Name == "eth" && c.Version == 69 {
			t.Fatal("their caps should not contain eth/69 when flag=true and peer is Geth")
		}
	}

	// Counter should increment by exactly 1
	newCount := eth69TrimmedCounter.Snapshot().Count()
	if newCount-initialCount != 1 {
		t.Fatalf("counter should increment by 1, got delta=%d", newCount-initialCount)
	}
}

func TestDoProtoHandshakeLegacy_FlagEnabled_NonGethPeer(t *testing.T) {
	original := eth69CompatEnabled.Load()
	defer eth69CompatEnabled.Store(original)
	eth69CompatEnabled.Store(true)

	initialCount := eth69TrimmedCounter.Snapshot().Count()

	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/test/v1.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}
	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Nethermind/v1.25.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	their, sentHS := doLegacyHandshakeTest(t, ourHS, theirHS)

	// Verify: our sent handshake SHOULD contain eth/69
	hasETH69 := false
	for _, c := range sentHS.Caps {
		if c.Name == "eth" && c.Version == 69 {
			hasETH69 = true
			break
		}
	}
	if !hasETH69 {
		t.Fatal("sent handshake should contain eth/69 for non-Geth peer")
	}

	// Verify: their returned caps SHOULD contain eth/69
	hasETH69 = false
	for _, c := range their.Caps {
		if c.Name == "eth" && c.Version == 69 {
			hasETH69 = true
			break
		}
	}
	if !hasETH69 {
		t.Fatal("their caps should contain eth/69 for non-Geth peer")
	}

	// Counter should NOT increment
	newCount := eth69TrimmedCounter.Snapshot().Count()
	if newCount-initialCount != 0 {
		t.Fatalf("counter should not increment for non-Geth peer, got delta=%d", newCount-initialCount)
	}
}

func TestDoProtoHandshakeLegacy_FlagDisabled_GethPeer(t *testing.T) {
	original := eth69CompatEnabled.Load()
	defer eth69CompatEnabled.Store(original)
	eth69CompatEnabled.Store(false)

	initialCount := eth69TrimmedCounter.Snapshot().Count()

	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/test/v1.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}
	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.14.0/linux/go1.22",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	their, sentHS := doLegacyHandshakeTest(t, ourHS, theirHS)

	// Verify: sent handshake SHOULD contain eth/69
	hasETH69 := false
	for _, c := range sentHS.Caps {
		if c.Name == "eth" && c.Version == 69 {
			hasETH69 = true
			break
		}
	}
	if !hasETH69 {
		t.Fatal("sent handshake should contain eth/69 when flag=false")
	}

	// Verify: their caps SHOULD contain eth/69
	hasETH69 = false
	for _, c := range their.Caps {
		if c.Name == "eth" && c.Version == 69 {
			hasETH69 = true
			break
		}
	}
	if !hasETH69 {
		t.Fatal("their caps should contain eth/69 when flag=false")
	}

	// Counter should NOT increment
	newCount := eth69TrimmedCounter.Snapshot().Count()
	if newCount-initialCount != 0 {
		t.Fatalf("counter should not increment when flag=false, got delta=%d", newCount-initialCount)
	}
}

func TestDoProtoHandshakeLegacy_FlagEnabled_GethPeer_NoETH69(t *testing.T) {
	original := eth69CompatEnabled.Load()
	defer eth69CompatEnabled.Store(original)
	eth69CompatEnabled.Store(true)

	initialCount := eth69TrimmedCounter.Snapshot().Count()

	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/test/v1.0",
		Caps:    []Cap{{"eth", 68}, {"snap", 1}},
		ID:      make([]byte, 64),
	}
	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.13.0/linux/go1.21",
		Caps:    []Cap{{"eth", 68}},
		ID:      make([]byte, 64),
	}

	_, _ = doLegacyHandshakeTest(t, ourHS, theirHS)

	// Counter should NOT increment (nothing to remove)
	newCount := eth69TrimmedCounter.Snapshot().Count()
	if newCount-initialCount != 0 {
		t.Fatalf("counter should not increment when no eth/69 to remove, got delta=%d", newCount-initialCount)
	}
}

func TestDoProtoHandshakeLegacy_CounterIncrementsOncePerConnection(t *testing.T) {
	original := eth69CompatEnabled.Load()
	defer eth69CompatEnabled.Store(original)
	eth69CompatEnabled.Store(true)

	initialCount := eth69TrimmedCounter.Snapshot().Count()

	// Both sides have eth/69, so trimETH69Counted is called on both our and their
	ourHS := &protoHandshake{
		Version: 5,
		Name:    "op-geth/test/v1.0",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}
	theirHS := &protoHandshake{
		Version: 5,
		Name:    "Geth/v1.14.0/linux/go1.22",
		Caps:    []Cap{{"eth", 68}, {"eth", 69}, {"snap", 1}},
		ID:      make([]byte, 64),
	}

	_, _ = doLegacyHandshakeTest(t, ourHS, theirHS)

	// Despite trimming both our and their, counter increments exactly once
	newCount := eth69TrimmedCounter.Snapshot().Count()
	if newCount-initialCount != 1 {
		t.Fatalf("counter should increment exactly once per connection, got delta=%d", newCount-initialCount)
	}
}

// doLegacyHandshakeTest sets up a TCP pipe with encryption handshake and runs
// doProtoHandshakeLegacy on one side. Returns the received 'their' handshake
// and the handshake that was actually sent (captured from the remote side).
func doLegacyHandshakeTest(t *testing.T, ourHS, theirHS *protoHandshake) (their *protoHandshake, sentHS *protoHandshake) {
	t.Helper()

	fd0, fd1, err := pipes.TCPPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer fd0.Close()
	defer fd1.Close()

	prv0, _ := crypto.GenerateKey()
	prv1, _ := crypto.GenerateKey()

	// Fill IDs from keys
	ourHS.ID = crypto.FromECDSAPub(&prv0.PublicKey)[1:]
	theirHS.ID = crypto.FromECDSAPub(&prv1.PublicKey)[1:]

	var (
		wg        sync.WaitGroup
		dialErr   error
		listenErr error
	)

	// Dial side (fd0): will call doProtoHandshakeLegacy
	wg.Add(1)
	go func() {
		defer wg.Done()
		frame := newRLPX(fd0, &prv1.PublicKey)
		if _, err := frame.doEncHandshake(prv0); err != nil {
			dialErr = err
			return
		}
		their, dialErr = frame.doProtoHandshakeLegacy(ourHS)
	}()

	// Listen side (fd1): sends theirHS, reads what we sent
	wg.Add(1)
	go func() {
		defer wg.Done()
		frame := newRLPX(fd1, nil)
		if _, err := frame.doEncHandshake(prv1); err != nil {
			listenErr = err
			return
		}
		// Send their handshake (simulating the remote peer's protocol handshake)
		if err := Send(frame, handshakeMsg, theirHS); err != nil {
			listenErr = err
			return
		}
		// Read what our side sent (this is the handshake we want to inspect)
		var received protoHandshake
		msg, err := frame.ReadMsg()
		if err != nil {
			listenErr = err
			return
		}
		if err := msg.Decode(&received); err != nil {
			listenErr = err
			return
		}
		sentHS = &received
	}()

	wg.Wait()

	if dialErr != nil {
		t.Fatalf("dial side error: %v", dialErr)
	}
	if listenErr != nil {
		t.Fatalf("listen side error: %v", listenErr)
	}
	if their == nil {
		t.Fatal("their handshake is nil")
	}
	if sentHS == nil {
		t.Fatal("sentHS is nil")
	}
	return their, sentHS
}
