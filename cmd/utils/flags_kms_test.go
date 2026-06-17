package utils

import (
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/internal/kms"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/urfave/cli/v2"
)

func setupKMSEnv(t *testing.T) {
	t.Helper()
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hangzhou")
}

func clearKMSEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv("KMS_PROVIDER")
	os.Unsetenv("KMS_SECRET_NAME")
	os.Unsetenv("KMS_REGION")
}


func TestSetNodeKey_KMSEnabled_ValidHex(t *testing.T) {
	kms.ResetForTest()
	setupKMSEnv(t)
	defer clearKMSEnv(t)

	validHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	kms.SetTestSDK(
		func() error { return nil },
		func(key string) (string, error) {
			if key == "op-geth.nodekeyhex" {
				return validHex, nil
			}
			return "", nil
		},
	)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	app := &cli.App{
		Flags: KMSFlags,
		Action: func(ctx *cli.Context) error {
			cfg := &p2p.Config{}
			setNodeKey(ctx, cfg)
			if cfg.PrivateKey == nil {
				t.Fatal("expected PrivateKey to be set")
			}
			return nil
		},
	}
	if err := app.Run([]string{"test"}); err != nil {
		t.Fatal(err)
	}
}

func TestSetNodeKey_KMSEnabled_WithOxPrefix(t *testing.T) {
	kms.ResetForTest()
	setupKMSEnv(t)
	defer clearKMSEnv(t)

	validHex := "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	kms.SetTestSDK(
		func() error { return nil },
		func(key string) (string, error) {
			return validHex, nil
		},
	)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	app := &cli.App{
		Flags: KMSFlags,
		Action: func(ctx *cli.Context) error {
			cfg := &p2p.Config{}
			setNodeKey(ctx, cfg)
			if cfg.PrivateKey == nil {
				t.Fatal("expected PrivateKey to be set with 0x prefix")
			}
			return nil
		},
	}
	if err := app.Run([]string{"test"}); err != nil {
		t.Fatal(err)
	}
}

func TestSetNodeKey_KMSEnabled_CustomKeyName(t *testing.T) {
	kms.ResetForTest()
	setupKMSEnv(t)
	defer clearKMSEnv(t)

	validHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	queriedKey := ""
	kms.SetTestSDK(
		func() error { return nil },
		func(key string) (string, error) {
			queriedKey = key
			return validHex, nil
		},
	)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	app := &cli.App{
		Flags: KMSFlags,
		Action: func(ctx *cli.Context) error {
			cfg := &p2p.Config{}
			setNodeKey(ctx, cfg)
			if queriedKey != "custom.nodekey" {
				t.Fatalf("expected query key 'custom.nodekey', got %q", queriedKey)
			}
			return nil
		},
	}
	if err := app.Run([]string{"test", "--kms.nodekeyhex-key", "custom.nodekey"}); err != nil {
		t.Fatal(err)
	}
}

func TestSetNodeKey_KMSDisabled_OriginalBehavior(t *testing.T) {
	kms.ResetForTest()
	clearKMSEnv(t)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	validHex := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	app := &cli.App{
		Flags: append(KMSFlags, NodeKeyHexFlag),
		Action: func(ctx *cli.Context) error {
			cfg := &p2p.Config{}
			setNodeKey(ctx, cfg)
			if cfg.PrivateKey == nil {
				t.Fatal("expected PrivateKey to be set from --nodekeyhex")
			}
			return nil
		},
	}
	if err := app.Run([]string{"test", "--nodekeyhex", validHex}); err != nil {
		t.Fatal(err)
	}
}

func TestSetNodeKey_KMSDisabled_NoFlags_NilKey(t *testing.T) {
	kms.ResetForTest()
	clearKMSEnv(t)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	app := &cli.App{
		Flags: append(KMSFlags, NodeKeyHexFlag, NodeKeyFileFlag),
		Action: func(ctx *cli.Context) error {
			cfg := &p2p.Config{}
			setNodeKey(ctx, cfg)
			if cfg.PrivateKey != nil {
				t.Fatal("expected PrivateKey to be nil when no flags set")
			}
			return nil
		},
	}
	if err := app.Run([]string{"test"}); err != nil {
		t.Fatal(err)
	}
}

func TestKMSFlags_DefaultValues(t *testing.T) {
	if KMSNodeKeyHexKeyFlag.Value != "op-geth.nodekeyhex" {
		t.Fatalf("unexpected default for nodekeyhex-key: %s", KMSNodeKeyHexKeyFlag.Value)
	}
	if KMSJWTSecretKeyFlag.Value != "op-geth.jwtsecret" {
		t.Fatalf("unexpected default for jwtsecret-key: %s", KMSJWTSecretKeyFlag.Value)
	}
}

func TestKMSFlags_EnvVars(t *testing.T) {
	if len(KMSNodeKeyHexKeyFlag.EnvVars) == 0 || KMSNodeKeyHexKeyFlag.EnvVars[0] != "OP_GETH_KMS_NODEKEYHEX_KEY" {
		t.Fatal("expected OP_GETH_KMS_NODEKEYHEX_KEY env var")
	}
	if len(KMSJWTSecretKeyFlag.EnvVars) == 0 || KMSJWTSecretKeyFlag.EnvVars[0] != "OP_GETH_KMS_JWTSECRET_KEY" {
		t.Fatal("expected OP_GETH_KMS_JWTSECRET_KEY env var")
	}
}
