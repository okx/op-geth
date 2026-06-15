package utils

import (
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/urfave/cli/v2"
)

const (
	DefaultKMSNodeKeyName   = "op-geth.nodekeyhex"
	DefaultKMSJWTSecretName = "op-geth.jwtsecret"
)

var (
	KMSNodeKeyNameFlag = &cli.StringFlag{
		Name:     "kms.nodekey-name",
		Usage:    "KMS secret key name for devp2p node private key",
		Value:    DefaultKMSNodeKeyName,
		Category: flags.XLayerCategory,
		EnvVars:  []string{"KMS_NODEKEY_NAME"},
	}
	KMSJWTSecretNameFlag = &cli.StringFlag{
		Name:     "kms.jwtsecret-name",
		Usage:    "KMS secret key name for auth RPC JWT secret",
		Value:    DefaultKMSJWTSecretName,
		Category: flags.XLayerCategory,
		EnvVars:  []string{"KMS_JWTSECRET_NAME"},
	}

	KMSFlags = []cli.Flag{
		KMSNodeKeyNameFlag,
		KMSJWTSecretNameFlag,
	}
)
