package node

import (
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/internal/kms"
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

func TestObtainJWTSecretFromKMS_ValidHex(t *testing.T) {
	kms.ResetForTest()
	setupKMSEnv(t)
	defer clearKMSEnv(t)

	kms.SetTestSDK(
		func() error { return nil },
		func(key string) (string, error) {
			if key == "op-geth.jwtsecret" {
				return "0x7365637265743132333435363738393031323334353637383930313233343536", nil
			}
			return "", nil
		},
	)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	n := &Node{config: &Config{}}
	secret, err := n.obtainJWTSecretFromKMS()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(secret))
	}
}

func TestObtainJWTSecretFromKMS_InvalidLength(t *testing.T) {
	kms.ResetForTest()
	setupKMSEnv(t)
	defer clearKMSEnv(t)

	kms.SetTestSDK(
		func() error { return nil },
		func(key string) (string, error) {
			return "abcdef1234", nil // 5 bytes, not 32
		},
	)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	n := &Node{config: &Config{}}
	_, err := n.obtainJWTSecretFromKMS()
	if err == nil {
		t.Fatal("expected error for invalid length")
	}
}

func TestObtainJWTSecretFromKMS_CustomKeyName(t *testing.T) {
	kms.ResetForTest()
	setupKMSEnv(t)
	defer clearKMSEnv(t)

	queriedKey := ""
	kms.SetTestSDK(
		func() error { return nil },
		func(key string) (string, error) {
			queriedKey = key
			return "7365637265743132333435363738393031323334353637383930313233343536", nil
		},
	)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	n := &Node{config: &Config{KMSJWTSecretKey: "custom.jwt.key"}}
	_, err := n.obtainJWTSecretFromKMS()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if queriedKey != "custom.jwt.key" {
		t.Fatalf("expected KMS queried with 'custom.jwt.key', got %q", queriedKey)
	}
}

func TestObtainJWTSecretFromKMS_DefaultKeyName(t *testing.T) {
	kms.ResetForTest()
	setupKMSEnv(t)
	defer clearKMSEnv(t)

	queriedKey := ""
	kms.SetTestSDK(
		func() error { return nil },
		func(key string) (string, error) {
			queriedKey = key
			return "7365637265743132333435363738393031323334353637383930313233343536", nil
		},
	)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	n := &Node{config: &Config{}}
	_, err := n.obtainJWTSecretFromKMS()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if queriedKey != "op-geth.jwtsecret" {
		t.Fatalf("expected default key name 'op-geth.jwtsecret', got %q", queriedKey)
	}
}

func TestObtainJWTSecret_KMSDisabled_FallsBackToFile(t *testing.T) {
	kms.ResetForTest()
	clearKMSEnv(t)

	if err := kms.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	tmpDir := t.TempDir()
	gethDir := tmpDir + "/geth"
	if err := os.MkdirAll(gethDir, 0700); err != nil {
		t.Fatal(err)
	}
	n := &Node{config: &Config{DataDir: tmpDir, Name: "geth"}}
	secret, err := n.obtainJWTSecret("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(secret))
	}
}
