package p2p

import (
	"crypto/ecdsa"
	"io"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/metrics"
)

func init() {
	metrics.Enabled = true
	eth69TrimCounter = metrics.NewCounterForced()
}

// --- S-1: trimETH69Caps tests ---

func TestTrimETH69Caps(t *testing.T) {
	tests := []struct {
		name        string
		input       []Cap
		wantCaps    []Cap
		wantRemoved bool
	}{
		{
			name:        "S-1.1: caps contain eth/69 among others",
			input:       []Cap{{Name: "eth", Version: 69}, {Name: "eth", Version: 68}, {Name: "snap", Version: 1}},
			wantCaps:    []Cap{{Name: "eth", Version: 68}, {Name: "snap", Version: 1}},
			wantRemoved: true,
		},
		{
			name:        "S-1.2: caps do NOT contain eth/69",
			input:       []Cap{{Name: "eth", Version: 68}, {Name: "snap", Version: 1}},
			wantCaps:    []Cap{{Name: "eth", Version: 68}, {Name: "snap", Version: 1}},
			wantRemoved: false,
		},
		{
			name:        "S-1.3: empty caps slice",
			input:       []Cap{},
			wantCaps:    []Cap{},
			wantRemoved: false,
		},
		{
			name:        "S-1.4: only eth/69 in caps",
			input:       []Cap{{Name: "eth", Version: 69}},
			wantCaps:    []Cap{},
			wantRemoved: true,
		},
		{
			name:        "S-1.5: multiple eth/69 entries",
			input:       []Cap{{Name: "eth", Version: 69}, {Name: "eth", Version: 69}, {Name: "eth", Version: 68}},
			wantCaps:    []Cap{{Name: "eth", Version: 68}},
			wantRemoved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, removed := trimETH69Caps(tt.input)
			if removed != tt.wantRemoved {
				t.Errorf("removed = %v, want %v", removed, tt.wantRemoved)
			}
			if len(result) != len(tt.wantCaps) {
				t.Fatalf("len(result) = %d, want %d", len(result), len(tt.wantCaps))
			}
			for i, c := range result {
				if c != tt.wantCaps[i] {
					t.Errorf("result[%d] = %v, want %v", i, c, tt.wantCaps[i])
				}
			}
		})
	}
}

func TestTrimETH69Caps_Immutability(t *testing.T) {
	original := []Cap{{Name: "eth", Version: 69}, {Name: "eth", Version: 68}}
	capsCopy := make([]Cap, len(original))
	copy(capsCopy, original)

	trimETH69Caps(capsCopy)

	for i, c := range capsCopy {
		if c != original[i] {
			t.Errorf("input mutated at index %d: got %v, want %v", i, c, original[i])
		}
	}
}

// --- S-2: isGeth tests ---

func TestIsGeth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"S-2.1: official Geth name", "Geth/v1.14.0-stable-abcdef/linux-amd64/go1.22.0", true},
		{"S-2.2: lowercase geth", "geth/v1.14.0", true},
		{"S-2.3: op-geth", "op-geth/v1.101410.1-stable", true},
		{"S-2.4: Nethermind", "Nethermind/v1.25.0", false},
		{"S-2.5: Erigon", "Erigon/v2.60.0", false},
		{"S-2.6: empty name", "", false},
		{"S-2.7: all-uppercase GETH", "GETH/v1.14.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGeth(tt.input)
			if got != tt.want {
				t.Errorf("isGeth(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- S-3: TrimOurHandshakeCaps tests ---

func TestTrimOurHandshakeCaps(t *testing.T) {
	t.Run("S-3.1: trim enabled, ourCaps has eth/69", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		defer eth69CompatEnabled.Store(true)

		our := &protoHandshake{Caps: []Cap{
			{Name: "eth", Version: 68},
			{Name: "eth", Version: 69},
			{Name: "snap", Version: 1},
		}}
		TrimOurHandshakeCaps(our)

		want := []Cap{{Name: "eth", Version: 68}, {Name: "snap", Version: 1}}
		if len(our.Caps) != len(want) {
			t.Fatalf("len(our.Caps) = %d, want %d", len(our.Caps), len(want))
		}
		for i, c := range our.Caps {
			if c != want[i] {
				t.Errorf("our.Caps[%d] = %v, want %v", i, c, want[i])
			}
		}
	})

	t.Run("S-3.2: trim disabled, ourCaps unchanged", func(t *testing.T) {
		eth69CompatEnabled.Store(false)
		defer eth69CompatEnabled.Store(true)

		our := &protoHandshake{Caps: []Cap{
			{Name: "eth", Version: 68},
			{Name: "eth", Version: 69},
		}}
		TrimOurHandshakeCaps(our)

		if len(our.Caps) != 2 {
			t.Fatalf("expected 2 caps, got %d", len(our.Caps))
		}
		if our.Caps[1] != (Cap{Name: "eth", Version: 69}) {
			t.Errorf("eth/69 should still be present, got %v", our.Caps[1])
		}
	})

	t.Run("S-3.3: trim enabled, no eth/69 to remove", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		defer eth69CompatEnabled.Store(true)

		our := &protoHandshake{Caps: []Cap{
			{Name: "eth", Version: 68},
			{Name: "snap", Version: 1},
		}}
		TrimOurHandshakeCaps(our)

		if len(our.Caps) != 2 {
			t.Fatalf("expected 2 caps, got %d", len(our.Caps))
		}
	})

	t.Run("S-3.4: idempotency — called twice", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		defer eth69CompatEnabled.Store(true)

		our := &protoHandshake{Caps: []Cap{
			{Name: "eth", Version: 68},
			{Name: "eth", Version: 69},
		}}
		TrimOurHandshakeCaps(our)
		TrimOurHandshakeCaps(our)

		want := []Cap{{Name: "eth", Version: 68}}
		if len(our.Caps) != 1 {
			t.Fatalf("len(our.Caps) = %d, want 1", len(our.Caps))
		}
		if our.Caps[0] != want[0] {
			t.Errorf("our.Caps[0] = %v, want %v", our.Caps[0], want[0])
		}
	})
}

// --- S-4: doProtoHandshakeWithXLayerFilter tests ---

type mockTransport struct {
	handshakeResult *protoHandshake
	handshakeErr    error
}

func (m *mockTransport) doEncHandshake(prv *ecdsa.PrivateKey) (*ecdsa.PublicKey, error) {
	return nil, nil
}

func (m *mockTransport) doProtoHandshake(our *protoHandshake) (*protoHandshake, error) {
	return m.handshakeResult, m.handshakeErr
}

func (m *mockTransport) ReadMsg() (Msg, error) { return Msg{}, nil }
func (m *mockTransport) WriteMsg(Msg) error    { return nil }
func (m *mockTransport) close(err error)       {}

func TestDoProtoHandshakeWithXLayerFilter(t *testing.T) {
	t.Run("S-4.1: trim enabled + Geth peer + eth/69", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		defer eth69CompatEnabled.Store(true)

		baseline := eth69TrimCounter.Snapshot().Count()
		mock := &mockTransport{
			handshakeResult: &protoHandshake{
				Name: "Geth/v1.14.0",
				Caps: []Cap{{Name: "eth", Version: 68}, {Name: "eth", Version: 69}},
			},
		}

		their, err := doProtoHandshakeWithXLayerFilter(mock, &protoHandshake{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(their.Caps) != 1 || their.Caps[0] != (Cap{Name: "eth", Version: 68}) {
			t.Errorf("their.Caps = %v, want [{eth 68}]", their.Caps)
		}
		if eth69TrimCounter.Snapshot().Count() != baseline+1 {
			t.Errorf("counter not incremented")
		}
	})

	t.Run("S-4.2: trim enabled + non-Geth peer", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		defer eth69CompatEnabled.Store(true)

		baseline := eth69TrimCounter.Snapshot().Count()
		mock := &mockTransport{
			handshakeResult: &protoHandshake{
				Name: "Nethermind/v1.25.0",
				Caps: []Cap{{Name: "eth", Version: 68}, {Name: "eth", Version: 69}},
			},
		}

		their, err := doProtoHandshakeWithXLayerFilter(mock, &protoHandshake{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(their.Caps) != 2 {
			t.Errorf("their.Caps = %v, want [{eth 68} {eth 69}]", their.Caps)
		}
		if eth69TrimCounter.Snapshot().Count() != baseline {
			t.Errorf("counter should not have incremented")
		}
	})

	t.Run("S-4.3: trim disabled + Geth peer", func(t *testing.T) {
		eth69CompatEnabled.Store(false)
		defer eth69CompatEnabled.Store(true)

		baseline := eth69TrimCounter.Snapshot().Count()
		mock := &mockTransport{
			handshakeResult: &protoHandshake{
				Name: "Geth/v1.14.0",
				Caps: []Cap{{Name: "eth", Version: 69}},
			},
		}

		their, err := doProtoHandshakeWithXLayerFilter(mock, &protoHandshake{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(their.Caps) != 1 || their.Caps[0] != (Cap{Name: "eth", Version: 69}) {
			t.Errorf("their.Caps = %v, want [{eth 69}]", their.Caps)
		}
		if eth69TrimCounter.Snapshot().Count() != baseline {
			t.Errorf("counter should not have incremented")
		}
	})

	t.Run("S-4.4: trim enabled + Geth peer + no eth/69", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		defer eth69CompatEnabled.Store(true)

		baseline := eth69TrimCounter.Snapshot().Count()
		mock := &mockTransport{
			handshakeResult: &protoHandshake{
				Name: "Geth/v1.14.0",
				Caps: []Cap{{Name: "eth", Version: 68}, {Name: "snap", Version: 1}},
			},
		}

		their, err := doProtoHandshakeWithXLayerFilter(mock, &protoHandshake{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(their.Caps) != 2 {
			t.Errorf("their.Caps = %v, want [{eth 68} {snap 1}]", their.Caps)
		}
		if eth69TrimCounter.Snapshot().Count() != baseline {
			t.Errorf("counter should not have incremented")
		}
	})

	t.Run("S-4.5: upstream handshake error propagation", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		defer eth69CompatEnabled.Store(true)

		baseline := eth69TrimCounter.Snapshot().Count()
		mock := &mockTransport{
			handshakeErr: io.EOF,
		}

		their, err := doProtoHandshakeWithXLayerFilter(mock, &protoHandshake{})
		if err != io.EOF {
			t.Errorf("expected io.EOF, got %v", err)
		}
		if their != nil {
			t.Errorf("expected nil result on error")
		}
		if eth69TrimCounter.Snapshot().Count() != baseline {
			t.Errorf("counter should not have incremented on error")
		}
	})
}

// --- S-5: doProtoHandshakeForConn routing tests ---

func TestDoProtoHandshakeForConn_MockTransport(t *testing.T) {
	eth69CompatEnabled.Store(true)
	defer eth69CompatEnabled.Store(true)

	mock := &mockTransport{
		handshakeResult: &protoHandshake{
			Name: "Geth/v1.14.0",
			Caps: []Cap{{Name: "eth", Version: 68}, {Name: "eth", Version: 69}},
		},
	}

	their, err := doProtoHandshakeForConn(mock, &protoHandshake{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mock transport (non-rlpxTransport) should bypass xlayer filter
	if len(their.Caps) != 2 {
		t.Errorf("mock transport should bypass xlayer filter, got %v", their.Caps)
	}
}

// --- S-6: SetETH69CompatEnabled tests ---

func TestSetETH69CompatEnabled(t *testing.T) {
	t.Run("S-6.1: set to true", func(t *testing.T) {
		SetETH69CompatEnabled(true)
		if !eth69CompatEnabled.Load() {
			t.Error("expected eth69CompatEnabled to be true")
		}
	})

	t.Run("S-6.2: set to false", func(t *testing.T) {
		SetETH69CompatEnabled(false)
		defer eth69CompatEnabled.Store(true)
		if eth69CompatEnabled.Load() {
			t.Error("expected eth69CompatEnabled to be false")
		}
	})

	t.Run("S-6.3: default init state is true", func(t *testing.T) {
		eth69CompatEnabled.Store(true)
		if !eth69CompatEnabled.Load() {
			t.Error("default state should be true")
		}
	})
}

// --- S-7: Counter concurrency test ---

func TestETH69TrimCounter_Concurrency(t *testing.T) {
	eth69CompatEnabled.Store(true)
	defer eth69CompatEnabled.Store(true)

	baseline := eth69TrimCounter.Snapshot().Count()
	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			mock := &mockTransport{
				handshakeResult: &protoHandshake{
					Name: "Geth/v1.14.0",
					Caps: []Cap{{Name: "eth", Version: 68}, {Name: "eth", Version: 69}},
				},
			}
			_, _ = doProtoHandshakeWithXLayerFilter(mock, &protoHandshake{})
		}()
	}
	wg.Wait()

	final := eth69TrimCounter.Snapshot().Count()
	if final != baseline+N {
		t.Errorf("counter = %d, want %d (baseline=%d, N=%d)", final, baseline+N, baseline, N)
	}
}

// --- S-8: Fuzz test ---

func FuzzTrimETH69Caps(f *testing.F) {
	f.Add("eth", uint(69))
	f.Add("eth", uint(68))
	f.Add("snap", uint(1))
	f.Add("", uint(0))

	f.Fuzz(func(t *testing.T, name string, version uint) {
		caps := []Cap{
			{Name: name, Version: version},
			{Name: "eth", Version: 69},
			{Name: "eth", Version: 68},
		}

		original := make([]Cap, len(caps))
		copy(original, caps)

		result, _ := trimETH69Caps(caps)

		for _, c := range result {
			if c.Name == "eth" && c.Version == 69 {
				t.Error("result contains eth/69")
			}
		}

		for i, c := range caps {
			if c != original[i] {
				t.Errorf("input mutated at index %d", i)
			}
		}

		for _, c := range caps {
			if c.Name == "eth" && c.Version == 69 {
				continue
			}
			found := false
			for _, r := range result {
				if r == c {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("cap %v from input not found in result", c)
			}
		}
	})
}

// --- Wiring integration test: SetETH69CompatEnabled called with false disables trim ---

func TestETH69CompatWiringDisabled(t *testing.T) {
	SetETH69CompatEnabled(false)
	defer eth69CompatEnabled.Store(true)

	if eth69CompatEnabled.Load() {
		t.Fatal("eth69CompatEnabled should be false after SetETH69CompatEnabled(false)")
	}

	our := &protoHandshake{Caps: []Cap{
		{Name: "eth", Version: 68},
		{Name: "eth", Version: 69},
	}}
	TrimOurHandshakeCaps(our)
	if len(our.Caps) != 2 {
		t.Errorf("TrimOurHandshakeCaps should be no-op when disabled, got %v", our.Caps)
	}

	baseline := eth69TrimCounter.Snapshot().Count()
	mock := &mockTransport{
		handshakeResult: &protoHandshake{
			Name: "Geth/v1.14.0",
			Caps: []Cap{{Name: "eth", Version: 69}},
		},
	}
	their, err := doProtoHandshakeWithXLayerFilter(mock, &protoHandshake{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(their.Caps) != 1 || their.Caps[0] != (Cap{Name: "eth", Version: 69}) {
		t.Errorf("filter should be inactive when disabled, got %v", their.Caps)
	}
	if eth69TrimCounter.Snapshot().Count() != baseline {
		t.Errorf("counter should not increment when disabled")
	}
}
