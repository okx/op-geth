package p2p

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var (
	eth69CompatEnabled atomic.Bool

	eth69TrimCounter = metrics.NewRegisteredCounter("p2p/eth69/trimmed", nil)
)

func init() {
	eth69CompatEnabled.Store(true)
}

// SetETH69CompatEnabled sets the ETH69 compatibility trim state.
// Called once at process startup from cmd/utils.
func SetETH69CompatEnabled(enabled bool) {
	eth69CompatEnabled.Store(enabled)
	if enabled {
		log.Warn("P2P eth/69 compat trim active (op-geth<>geth peers will negotiate eth/68); disable with --p2p.eth69-compat=false once upstream is verified")
	}
}

func (t *rlpxTransport) doProtoHandshakeLegacy(our *protoHandshake) (their *protoHandshake, err error) {
	if their, err = readProtocolHandshake(t); err != nil {
		return nil, err
	}

	handshakeToSend := our
	trimmed := false

	if eth69CompatEnabled.Load() && isGeth(their.Name) {
		var ourTrimmed, theirTrimmed bool
		handshakeToSend, ourTrimmed = trimETH69(our)
		their, theirTrimmed = trimETH69(their)
		trimmed = ourTrimmed || theirTrimmed
	}

	err = Send(t, handshakeMsg, handshakeToSend)
	if err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}
	t.conn.SetSnappy(their.Version >= snappyProtocolVersion)

	if trimmed {
		eth69TrimCounter.Inc(1)
	}

	return their, nil
}

func isGeth(name string) bool {
	return strings.Contains(name, "Geth") || strings.Contains(name, "geth")
}

// trimETH69 returns a copy of the handshake with eth/69 removed from caps.
// The second return value indicates whether eth/69 was actually present and removed.
// The input handshake is never mutated.
func trimETH69(phs *protoHandshake) (*protoHandshake, bool) {
	for _, c := range phs.Caps {
		if c.Name == "eth" && c.Version == 69 {
			newCaps := make([]Cap, 0, len(phs.Caps)-1)
			for _, cc := range phs.Caps {
				if cc.Name == "eth" && cc.Version == 69 {
					continue
				}
				newCaps = append(newCaps, cc)
			}
			filteredHandshake := *phs
			filteredHandshake.Caps = newCaps
			return &filteredHandshake, true
		}
	}
	return phs, false
}
