package p2p

import (
	"fmt"
	"strings"
)

// doProtoHandshakeLegacy performs the legacy protocol handshake where we read the remote
// handshake first, then send our own. This function includes special handling for Geth
// clients: if the remote peer is a Geth node, we filter out ETH69 from our capabilities
// to ensure compatibility.
func (t *rlpxTransport) doProtoHandshakeLegacy(our *protoHandshake) (their *protoHandshake, err error) {
	// Read the remote peer's handshake message first
	if their, err = readProtocolHandshake(t); err != nil {
		return nil, err
	}

	// Check if the remote peer is a Geth node by examining its client name
	// Geth nodes typically have "Geth" or "geth" in their client identifier
	handshakeToSend := our
	if isGeth(their.Name) {
		// For Geth clients, we need to filter out ETH69 protocol from our capabilities
		// This is because Geth nodes may not fully support ETH69, so we only advertise
		// ETH68 to ensure successful protocol negotiation
		handshakeToSend = trimETH69(our)
	}

	// Send our handshake message (filtered if Geth, original otherwise)
	err = Send(t, handshakeMsg, handshakeToSend)
	if err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}
	// If the protocol version supports Snappy encoding, upgrade immediately
	t.conn.SetSnappy(their.Version >= snappyProtocolVersion)

	if isGeth(their.Name) {
		their = trimETH69(their)
	}

	return their, nil
}

func isGeth(name string) bool {
	return strings.Contains(name, "Geth") || strings.Contains(name, "geth")
}

func trimETH69(phs *protoHandshake) *protoHandshake {
	newCaps := make([]Cap, 0, len(phs.Caps))
	for _, c := range phs.Caps {
		if c.Name == "eth" && c.Version == 69 {
			continue
		}
		newCaps = append(newCaps, c)
	}
	// Create a deep copy of the handshake with filtered caps
	filteredHandshake := *phs
	filteredHandshake.Caps = newCaps
	return &filteredHandshake
}
