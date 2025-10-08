package vm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestTokenAddress(t *testing.T) {
	for _, env := range environments {
		InitEnvConfig(env.chainId)
		assert.Equal(t, CONFIG_CONTRACT_MANAGER_ADDRESS, common.HexToAddress(env.configContractMgrAddress))
		assert.Equal(t, TARGET_ADDRESS, common.HexToAddress(env.targetAddress))
	}
}
