package p2p

import "testing"

func TestIsGeth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// FR-1: prefix match — true cases
		{name: "upstream Geth full name", input: "Geth/v1.14.11-stable-abc123/linux-amd64/go1.21", expected: true},
		{name: "bare Geth/ prefix", input: "Geth/", expected: true},

		// FR-1: prefix match — false cases
		{name: "fork node l2-geth", input: "l2-geth/v1.101701.0/darwin-arm64/go1.26.3", expected: false},
		{name: "fork node op-geth", input: "op-geth/v1.101701.0/linux-amd64/go1.26.3", expected: false},
		{name: "empty string", input: "", expected: false},
		{name: "interior substring XLayer/Geth", input: "XLayer/Geth/v1.0/linux", expected: false},
		{name: "dash separator Geth-legacy", input: "Geth-legacy/v1.0/linux", expected: false},

		// FR-2: case sensitivity
		{name: "lowercase geth/", input: "geth/v1.0/linux-amd64/go1.21", expected: false},
		{name: "uppercase OPGETH/", input: "OPGETH/v1.0", expected: false},
		{name: "mixed case OpGeth/", input: "OpGeth/v1.0", expected: false},
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

func TestTrimETH69(t *testing.T) {
	t.Run("removes eth/69 and preserves other caps", func(t *testing.T) {
		input := &protoHandshake{
			Version: 5,
			Name:    "test-node",
			Caps:    []Cap{{Name: "eth", Version: 69}, {Name: "snap", Version: 1}},
		}
		result := trimETH69(input)

		if len(result.Caps) != 1 {
			t.Fatalf("expected 1 cap, got %d", len(result.Caps))
		}
		if result.Caps[0].Name != "snap" || result.Caps[0].Version != 1 {
			t.Errorf("expected snap/1, got %s/%d", result.Caps[0].Name, result.Caps[0].Version)
		}
	})

	t.Run("does not mutate original", func(t *testing.T) {
		input := &protoHandshake{
			Version: 5,
			Name:    "test-node",
			Caps:    []Cap{{Name: "eth", Version: 69}, {Name: "snap", Version: 1}},
		}
		trimETH69(input)

		if len(input.Caps) != 2 {
			t.Fatalf("original mutated: expected 2 caps, got %d", len(input.Caps))
		}
	})

	t.Run("no eth/69 caps returns same set", func(t *testing.T) {
		input := &protoHandshake{
			Version: 5,
			Name:    "test-node",
			Caps:    []Cap{{Name: "snap", Version: 1}, {Name: "eth", Version: 68}},
		}
		result := trimETH69(input)

		if len(result.Caps) != 2 {
			t.Fatalf("expected 2 caps, got %d", len(result.Caps))
		}
	})
}
