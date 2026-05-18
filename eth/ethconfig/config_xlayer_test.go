package ethconfig

import "testing"

func TestDefaultXLayerConfig_P2P_ETH69Compat(t *testing.T) {
	if !DefaultXLayerConfig.P2P.ETH69Compat {
		t.Fatal("DefaultXLayerConfig.P2P.ETH69Compat should be true by default")
	}
}
