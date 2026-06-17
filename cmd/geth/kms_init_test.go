package main

import (
	"flag"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/internal/kms"
	"github.com/urfave/cli/v2"
)

func TestKMSFlagsRegistered(t *testing.T) {
	// Verify KMS flags are registered in the app so ctx.String returns defaults
	found := map[string]bool{
		"kms.nodekey-name":    false,
		"kms.jwtsecret-name":  false,
	}
	for _, f := range app.Flags {
		for _, name := range f.Names() {
			if _, ok := found[name]; ok {
				found[name] = true
			}
		}
	}
	for name, registered := range found {
		if !registered {
			t.Errorf("flag --%s not registered in app.Flags", name)
		}
	}
}

func TestKMSNodeKeyNameFlagDefault(t *testing.T) {
	// Verify that when flag is registered, its default matches the KMS constant
	for _, f := range app.Flags {
		if sf, ok := f.(*cli.StringFlag); ok && sf.Name == "kms.nodekey-name" {
			if sf.Value != kms.DefaultNodeKeyHexKMSKey {
				t.Errorf("kms.nodekey-name default = %q, want %q", sf.Value, kms.DefaultNodeKeyHexKMSKey)
			}
			return
		}
	}
	t.Fatal("kms.nodekey-name flag not found")
}

func TestKMSJWTSecretNameFlagDefault(t *testing.T) {
	for _, f := range app.Flags {
		if sf, ok := f.(*cli.StringFlag); ok && sf.Name == "kms.jwtsecret-name" {
			if sf.Value != kms.DefaultJWTSecretKMSKey {
				t.Errorf("kms.jwtsecret-name default = %q, want %q", sf.Value, kms.DefaultJWTSecretKMSKey)
			}
			return
		}
	}
	t.Fatal("kms.jwtsecret-name flag not found")
}

func TestAppBeforeInitializesKMS(t *testing.T) {
	// Verify that app.Before calls kms.InitFromEnv so all subcommands get KMS
	kms.ResetForTest()
	kms.RegisterSDK(
		func() error { return nil },
		func(key string) (string, error) { return "val", nil },
	)
	os.Setenv("KMS_PROVIDER", "TEST")
	os.Setenv("KMS_SECRET_NAME", "test")
	os.Setenv("KMS_REGION", "test")
	defer func() {
		kms.ResetForTest()
		os.Unsetenv("KMS_PROVIDER")
		os.Unsetenv("KMS_SECRET_NAME")
		os.Unsetenv("KMS_REGION")
	}()

	// Simulate what cli does: create a context and call app.Before
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range app.Flags {
		f.Apply(set)
	}
	ctx := cli.NewContext(app, set, nil)
	// Before our fix, localConsole would bypass kms.InitFromEnv.
	// Now it's in app.Before, so all subcommands (geth, console, attach) get it.
	if err := app.Before(ctx); err != nil {
		t.Fatalf("app.Before failed: %v", err)
	}
	if !kms.IsEnabled() {
		t.Fatal("expected KMS to be enabled after app.Before with env vars set")
	}
}
