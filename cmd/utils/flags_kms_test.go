package utils

import (
	"flag"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/internal/kms"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/urfave/cli/v2"
)

func setupKMSFlagsTest(t *testing.T) {
	t.Helper()
	kms.ResetForTest()
	os.Unsetenv("KMS_PROVIDER")
	os.Unsetenv("KMS_SECRET_NAME")
	os.Unsetenv("KMS_REGION")
	t.Cleanup(func() {
		kms.ResetForTest()
		os.Unsetenv("KMS_PROVIDER")
		os.Unsetenv("KMS_SECRET_NAME")
		os.Unsetenv("KMS_REGION")
	})
}

func enableKMSForFlags(t *testing.T, getSecret func(key string) (string, error)) {
	t.Helper()
	kms.RegisterSDK(
		func() error { return nil },
		getSecret,
	)
	os.Setenv("KMS_PROVIDER", "TEST")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "test-region")
	if err := kms.InitFromEnv(); err != nil {
		t.Fatalf("KMS init failed: %v", err)
	}
}

func makeCliContext(flags []cli.Flag, args []string) *cli.Context {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range flags {
		f.Apply(set)
	}
	set.Parse(args)
	app := &cli.App{Flags: flags}
	return cli.NewContext(app, set, nil)
}

func TestSetNodeKey_KMSEnabled_ValidHex(t *testing.T) {
	setupKMSFlagsTest(t)
	// Valid secp256k1 private key hex (32 bytes = 64 hex chars)
	validKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	enableKMSForFlags(t, func(key string) (string, error) { return validKey, nil })

	ctx := makeCliContext([]cli.Flag{
		NodeKeyHexFlag,
		NodeKeyFileFlag,
		KMSNodeKeyNameFlag,
	}, nil)

	cfg := &p2p.Config{}
	setNodeKey(ctx, cfg)

	if cfg.PrivateKey == nil {
		t.Fatal("expected PrivateKey to be set")
	}
}

func TestSetNodeKey_KMSEnabled_With0xPrefix(t *testing.T) {
	setupKMSFlagsTest(t)
	validKey := "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	enableKMSForFlags(t, func(key string) (string, error) { return validKey, nil })

	ctx := makeCliContext([]cli.Flag{
		NodeKeyHexFlag,
		NodeKeyFileFlag,
		KMSNodeKeyNameFlag,
	}, nil)

	cfg := &p2p.Config{}
	setNodeKey(ctx, cfg)

	if cfg.PrivateKey == nil {
		t.Fatal("expected PrivateKey to be set with 0x prefix stripped")
	}
}

func TestSetNodeKey_KMSEnabled_With0XPrefix(t *testing.T) {
	setupKMSFlagsTest(t)
	validKey := "0Xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	enableKMSForFlags(t, func(key string) (string, error) { return validKey, nil })

	ctx := makeCliContext([]cli.Flag{
		NodeKeyHexFlag,
		NodeKeyFileFlag,
		KMSNodeKeyNameFlag,
	}, nil)

	cfg := &p2p.Config{}
	setNodeKey(ctx, cfg)

	if cfg.PrivateKey == nil {
		t.Fatal("expected PrivateKey to be set with 0X prefix stripped")
	}
}

func TestSetNodeKey_KMSEnabled_CustomKeyName(t *testing.T) {
	setupKMSFlagsTest(t)
	var capturedKey string
	validKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	enableKMSForFlags(t, func(key string) (string, error) {
		capturedKey = key
		return validKey, nil
	})

	ctx := makeCliContext([]cli.Flag{
		NodeKeyHexFlag,
		NodeKeyFileFlag,
		KMSNodeKeyNameFlag,
	}, []string{"--kms.nodekey-name=custom.nodekey"})

	cfg := &p2p.Config{}
	setNodeKey(ctx, cfg)

	if capturedKey != "custom.nodekey" {
		t.Fatalf("expected KMS key 'custom.nodekey', got %q", capturedKey)
	}
}

func TestSetNodeKey_KMSDisabled_NoKeySet(t *testing.T) {
	setupKMSFlagsTest(t)
	// KMS not enabled, no flags set — should not set PrivateKey
	ctx := makeCliContext([]cli.Flag{
		NodeKeyHexFlag,
		NodeKeyFileFlag,
		KMSNodeKeyNameFlag,
	}, nil)

	cfg := &p2p.Config{}
	setNodeKey(ctx, cfg)

	if cfg.PrivateKey != nil {
		t.Fatal("expected PrivateKey to be nil when KMS disabled and no flags set")
	}
}

func TestStripHexPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0xabcdef", "abcdef"},
		{"0Xabcdef", "abcdef"},
		{"abcdef", "abcdef"},
		{"0x", ""},
		{"0X", ""},
		{"", ""},
		{"0", "0"},
	}
	for _, tc := range tests {
		got := stripHexPrefix(tc.input)
		if got != tc.expected {
			t.Errorf("stripHexPrefix(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
