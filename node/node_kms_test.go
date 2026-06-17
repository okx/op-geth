package node

import (
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/internal/kms"
)

func setupKMSTest(t *testing.T) {
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

func enableKMS(t *testing.T, getSecret func(key string) (string, error)) {
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

func TestObtainJWTSecretFromKMS_ValidHex(t *testing.T) {
	setupKMSTest(t)
	// 64 hex chars = 32 bytes
	validHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enableKMS(t, func(key string) (string, error) { return validHex, nil })

	n := &Node{config: &Config{}}
	secret, err := n.obtainJWTSecret("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(secret))
	}
}

func TestObtainJWTSecretFromKMS_With0xPrefix(t *testing.T) {
	setupKMSTest(t)
	validHex := "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enableKMS(t, func(key string) (string, error) { return validHex, nil })

	n := &Node{config: &Config{}}
	secret, err := n.obtainJWTSecret("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(secret))
	}
}

func TestObtainJWTSecretFromKMS_InvalidLength(t *testing.T) {
	setupKMSTest(t)
	shortHex := "0123456789abcdef0123456789abcdef" // 16 bytes
	enableKMS(t, func(key string) (string, error) { return shortHex, nil })

	n := &Node{config: &Config{}}
	_, err := n.obtainJWTSecret("")
	if err == nil {
		t.Fatal("expected error for invalid length")
	}
	if !strings.Contains(err.Error(), "invalid length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestObtainJWTSecretFromKMS_EmptyValue(t *testing.T) {
	setupKMSTest(t)
	enableKMS(t, func(key string) (string, error) { return "", nil })

	n := &Node{config: &Config{}}
	_, err := n.obtainJWTSecret("")
	if err == nil {
		t.Fatal("expected error for empty value")
	}
	if !strings.Contains(err.Error(), "retrieval failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestObtainJWTSecretFromKMS_CustomKeyName(t *testing.T) {
	setupKMSTest(t)
	var capturedKey string
	validHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enableKMS(t, func(key string) (string, error) {
		capturedKey = key
		return validHex, nil
	})

	n := &Node{config: &Config{KMSJWTSecretName: "custom.jwt.key"}}
	_, err := n.obtainJWTSecret("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedKey != "custom.jwt.key" {
		t.Fatalf("expected key 'custom.jwt.key', got %q", capturedKey)
	}
}

func TestObtainJWTSecretFromKMS_DefaultKeyName(t *testing.T) {
	setupKMSTest(t)
	var capturedKey string
	validHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enableKMS(t, func(key string) (string, error) {
		capturedKey = key
		return validHex, nil
	})

	n := &Node{config: &Config{}}
	_, err := n.obtainJWTSecret("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedKey != kms.DefaultJWTSecretKMSKey {
		t.Fatalf("expected default key %q, got %q", kms.DefaultJWTSecretKMSKey, capturedKey)
	}
}

func TestObtainJWTSecret_KMSDisabled_FallsBackToFile(t *testing.T) {
	setupKMSTest(t)
	// KMS not enabled, should use file path
	dir := t.TempDir()
	validHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	jwtFile := dir + "/jwtsecret"
	os.WriteFile(jwtFile, []byte(validHex), 0600)

	n := &Node{config: &Config{}}
	secret, err := n.obtainJWTSecret(jwtFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(secret))
	}
}
