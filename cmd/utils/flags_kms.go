package utils

import (
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/internal/kms"
	"github.com/urfave/cli/v2"
)

var (
	KMSNodeKeyHexKeyFlag = &cli.StringFlag{
		Name:     "kms.nodekeyhex-key",
		Usage:    "KMS key name for the devp2p node private key (hex)",
		Value:    kms.DefaultKMSNodeKeyHexKey,
		Category: flags.NetworkingCategory,
		EnvVars:  []string{"OP_GETH_KMS_NODEKEYHEX_KEY"},
	}
	KMSJWTSecretKeyFlag = &cli.StringFlag{
		Name:     "kms.jwtsecret-key",
		Usage:    "KMS key name for the Engine API JWT secret (hex)",
		Value:    kms.DefaultKMSJWTSecretKey,
		Category: flags.APICategory,
		EnvVars:  []string{"OP_GETH_KMS_JWTSECRET_KEY"},
	}

	KMSFlags = []cli.Flag{
		KMSNodeKeyHexKeyFlag,
		KMSJWTSecretKeyFlag,
	}
)
