package p2p

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var eth69CompatEnabled atomic.Bool

var eth69TrimmedCounter = metrics.NewRegisteredCounter("p2p/handshake/eth69/trimmed", nil)

func init() {
	eth69CompatEnabled.Store(true)
}

// SetETH69Compat is called once at startup to configure the trim behavior.
// When enabled=true (default), eth/69 is stripped for Geth peers.
// When enabled=false, no stripping occurs.
func SetETH69Compat(enabled bool) {
	eth69CompatEnabled.Store(enabled)
	if enabled {
		log.Warn("p2p: eth/69 compat trim active (op-geth<>geth peers will negotiate eth/68); disable with --p2p.eth69-compat=false once upstream is verified")
	}
}

func (t *rlpxTransport) doProtoHandshakeLegacy(our *protoHandshake) (their *protoHandshake, err error) {
	if their, err = readProtocolHandshake(t); err != nil {
		return nil, err
	}

	handshakeToSend := our
	trimmed := false

	if eth69CompatEnabled.Load() && isGeth(their.Name) {
		handshakeToSend, trimmed = trimETH69Counted(our)
	}

	err = Send(t, handshakeMsg, handshakeToSend)
	if err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}
	t.conn.SetSnappy(their.Version >= snappyProtocolVersion)

	if eth69CompatEnabled.Load() && isGeth(their.Name) {
		var theirTrimmed bool
		their, theirTrimmed = trimETH69Counted(their)
		if !trimmed {
			trimmed = theirTrimmed
		}
	}

	if trimmed {
		eth69TrimmedCounter.Inc(1)
	}

	return their, nil
}

// trimETH69Counted returns a copy of the handshake with eth/69 removed (if present).
// The second return value indicates whether eth/69 was actually found and removed.
// The original protoHandshake is NEVER mutated.
func trimETH69Counted(phs *protoHandshake) (*protoHandshake, bool) {
	found := false
	newCaps := make([]Cap, 0, len(phs.Caps))
	for _, c := range phs.Caps {
		if c.Name == "eth" && c.Version == 69 {
			found = true
			continue
		}
		newCaps = append(newCaps, c)
	}
	if !found {
		return phs, false
	}
	filteredHandshake := *phs
	filteredHandshake.Caps = newCaps
	return &filteredHandshake, true
}

func isGeth(name string) bool {
	return strings.Contains(name, "Geth") || strings.Contains(name, "geth")
}
