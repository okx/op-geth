package p2p

import (
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var eth69CompatEnabled atomic.Bool

var eth69TrimCounter = metrics.NewRegisteredCounter("p2p/handshake/eth69/trimmed", nil)

func init() {
	eth69CompatEnabled.Store(true)
}

func SetETH69CompatEnabled(enabled bool) {
	eth69CompatEnabled.Store(enabled)
	if enabled {
		log.Warn("p2p: eth/69 compat trim active (op-geth<>geth peers will negotiate eth/68); disable with --p2p.eth69-compat=false once upstream is verified")
	}
}

func TrimOurHandshakeCaps(our *protoHandshake) {
	if !eth69CompatEnabled.Load() {
		return
	}
	trimmed, removed := trimETH69Caps(our.Caps)
	if removed {
		our.Caps = trimmed
	}
}

func isGeth(name string) bool {
	return strings.Contains(name, "Geth") || strings.Contains(name, "geth")
}

func trimETH69Caps(caps []Cap) ([]Cap, bool) {
	result := make([]Cap, 0, len(caps))
	removed := false
	for _, c := range caps {
		if c.Name == "eth" && c.Version == 69 {
			removed = true
			continue
		}
		result = append(result, c)
	}
	return result, removed
}

func doProtoHandshakeWithXLayerFilter(c transport, our *protoHandshake) (*protoHandshake, error) {
	their, err := c.doProtoHandshake(our)
	if err != nil {
		return nil, err
	}
	if eth69CompatEnabled.Load() && isGeth(their.Name) {
		filtered, removed := trimETH69Caps(their.Caps)
		if removed {
			their.Caps = filtered
			eth69TrimCounter.Inc(1)
		}
	}
	return their, nil
}

func doProtoHandshakeForConn(c transport, our *protoHandshake) (*protoHandshake, error) {
	if _, ok := c.(*rlpxTransport); ok {
		return doProtoHandshakeWithXLayerFilter(c, our)
	}
	return c.doProtoHandshake(our)
}
