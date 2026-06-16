package utils

import (
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/p2p"
)

// --- Test helpers for mocking KMS SDK ---

func mockKMSInit(retErr error) func() {
	orig := kmsInit
	kmsInit = func() error { return retErr }
	return func() { kmsInit = orig }
}

func mockKMSGetSecretValue(retVal string, retErr error) func() {
	orig := kmsGetSecretValue
	kmsGetSecretValue = func(key string) (string, error) { return retVal, retErr }
	return func() { kmsGetSecretValue = orig }
}

func resetKMSState() {
	kmsState.enabled = false
}

// --- DM-1: InitKMSIfEnabled tests ---

func TestInitKMSIfEnabled_AllSet_Success(t *testing.T) {
	resetKMSState()
	restore := mockKMSInit(nil)
	defer restore()

	t.Setenv(envKMSProvider, "ALIYUN")
	t.Setenv(envKMSSecretName, "test-secret")
	t.Setenv(envKMSRegion, "cn-hangzhou")

	err := InitKMSIfEnabled()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !IsKMSEnabled() {
		t.Fatal("expected KMS to be enabled")
	}
}

func TestInitKMSIfEnabled_NoneSet_Disabled(t *testing.T) {
	resetKMSState()

	t.Setenv(envKMSProvider, "")
	t.Setenv(envKMSSecretName, "")
	t.Setenv(envKMSRegion, "")

	err := InitKMSIfEnabled()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if IsKMSEnabled() {
		t.Fatal("expected KMS to be disabled")
	}
}

func TestInitKMSIfEnabled_PartialConfig_OnlyProvider(t *testing.T) {
	resetKMSState()

	t.Setenv(envKMSProvider, "ALIYUN")
	t.Setenv(envKMSSecretName, "")
	t.Setenv(envKMSRegion, "")

	err := InitKMSIfEnabled()
	if err == nil {
		t.Fatal("expected error for partial config")
	}
	if !strings.Contains(err.Error(), "KMS partially configured") {
		t.Fatalf("error should mention partial config, got: %v", err)
	}
	if !strings.Contains(err.Error(), "KMS_PROVIDER") {
		t.Fatalf("error should list env var names, got: %v", err)
	}
	if IsKMSEnabled() {
		t.Fatal("expected KMS to remain disabled")
	}
}

func TestInitKMSIfEnabled_PartialConfig_TwoOfThree(t *testing.T) {
	resetKMSState()

	t.Setenv(envKMSProvider, "ALIYUN")
	t.Setenv(envKMSSecretName, "")
	t.Setenv(envKMSRegion, "cn-hangzhou")

	err := InitKMSIfEnabled()
	if err == nil {
		t.Fatal("expected error for partial config")
	}
	if !strings.Contains(err.Error(), "KMS partially configured") {
		t.Fatalf("error should mention partial config, got: %v", err)
	}
}

func TestInitKMSIfEnabled_SDKInitFailure(t *testing.T) {
	resetKMSState()
	restore := mockKMSInit(errors.New("connection refused"))
	defer restore()

	t.Setenv(envKMSProvider, "ALIYUN")
	t.Setenv(envKMSSecretName, "test-secret")
	t.Setenv(envKMSRegion, "cn-hangzhou")

	err := InitKMSIfEnabled()
	if err == nil {
		t.Fatal("expected error when SDK init fails")
	}
	if !strings.Contains(err.Error(), "kms.Init() failed") {
		t.Fatalf("error should mention init failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error should wrap original, got: %v", err)
	}
	if IsKMSEnabled() {
		t.Fatal("expected KMS to remain disabled after init failure")
	}
}

// --- DM-2/3: GetKMSValue tests ---

func TestGetKMSValue_Success(t *testing.T) {
	validHex := strings.Repeat("ab", 32)
	restore := mockKMSGetSecretValue(validHex, nil)
	defer restore()

	val, err := GetKMSValue("test-key", "nodekeyhex")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if val != validHex {
		t.Fatalf("expected %q, got %q", validHex, val)
	}
}

func TestGetKMSValue_TrimsWhitespace(t *testing.T) {
	validHex := strings.Repeat("ab", 32)
	restore := mockKMSGetSecretValue("  "+validHex+"  \n", nil)
	defer restore()

	val, err := GetKMSValue("test-key", "nodekeyhex")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if val != validHex {
		t.Fatalf("expected trimmed value %q, got %q", validHex, val)
	}
}

func TestGetKMSValue_EmptyValue(t *testing.T) {
	restore := mockKMSGetSecretValue("", nil)
	defer restore()

	_, err := GetKMSValue("test-key", "nodekeyhex")
	if err == nil {
		t.Fatal("expected error for empty value")
	}
	if !strings.Contains(err.Error(), "KMS returned empty value") {
		t.Fatalf("error should mention empty value, got: %v", err)
	}
}

func TestGetKMSValue_WhitespaceOnlyValue(t *testing.T) {
	restore := mockKMSGetSecretValue("   \t\n", nil)
	defer restore()

	_, err := GetKMSValue("test-key", "JWTSecret")
	if err == nil {
		t.Fatal("expected error for whitespace-only value")
	}
	if !strings.Contains(err.Error(), "KMS returned empty value") {
		t.Fatalf("error should mention empty value, got: %v", err)
	}
}

func TestGetKMSValue_SDKError(t *testing.T) {
	restore := mockKMSGetSecretValue("", errors.New("timeout"))
	defer restore()

	_, err := GetKMSValue("test-key", "nodekeyhex")
	if err == nil {
		t.Fatal("expected error for SDK failure")
	}
	if !strings.Contains(err.Error(), "KMS GetSecretValue failed") {
		t.Fatalf("error should mention failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error should wrap original, got: %v", err)
	}
}

// --- ParseHexPrivateKey tests ---

func TestParseHexPrivateKey_Valid64Chars(t *testing.T) {
	hexStr := strings.Repeat("ab", 32)
	b, err := ParseHexPrivateKey(hexStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(b))
	}
}

func TestParseHexPrivateKey_With0xPrefix(t *testing.T) {
	hexStr := "0x" + strings.Repeat("cd", 32)
	b, err := ParseHexPrivateKey(hexStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(b))
	}
}

func TestParseHexPrivateKey_WrongLength_Short(t *testing.T) {
	hexStr := strings.Repeat("ab", 16) // 16 bytes
	_, err := ParseHexPrivateKey(hexStr)
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
	if !strings.Contains(err.Error(), "expected 32 bytes, got 16") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseHexPrivateKey_WrongLength_Long(t *testing.T) {
	hexStr := strings.Repeat("ab", 64) // 64 bytes
	_, err := ParseHexPrivateKey(hexStr)
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
	if !strings.Contains(err.Error(), "expected 32 bytes, got 64") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseHexPrivateKey_InvalidHex(t *testing.T) {
	_, err := ParseHexPrivateKey("GHIJKLMNOPQRSTUVWXYZ0123456789abcdef0123456789abcdef0123456789abcde")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
	if !strings.Contains(err.Error(), "invalid hex") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseHexPrivateKey_Empty(t *testing.T) {
	_, err := ParseHexPrivateKey("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "expected 32 bytes, got 0") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// --- ParseJWTSecret tests ---

func TestParseJWTSecret_Valid64Chars(t *testing.T) {
	hexStr := strings.Repeat("ef", 32)
	b, err := ParseJWTSecret(hexStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(b))
	}
}

func TestParseJWTSecret_With0xPrefix(t *testing.T) {
	hexStr := "0x" + strings.Repeat("12", 32)
	b, err := ParseJWTSecret(hexStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(b))
	}
}

func TestParseJWTSecret_WrongLength(t *testing.T) {
	hexStr := strings.Repeat("ab", 24) // 24 bytes
	_, err := ParseJWTSecret(hexStr)
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
	if !strings.Contains(err.Error(), "JWT secret must be 32 bytes, got 24") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseJWTSecret_InvalidHex(t *testing.T) {
	_, err := ParseJWTSecret("not-valid-hex-string-at-all-!!!")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
	if !strings.Contains(err.Error(), "invalid JWT hex") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestParseJWTSecret_Empty(t *testing.T) {
	_, err := ParseJWTSecret("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "JWT secret must be 32 bytes, got 0") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// --- SetNodeKeyFromKMS tests ---

func TestSetNodeKeyFromKMS_Success(t *testing.T) {
	// Use a known valid secp256k1 private key (non-zero, valid curve point)
	validKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	restore := mockKMSGetSecretValue(validKey, nil)
	defer restore()

	cfg := &p2p.Config{}
	SetNodeKeyFromKMS(cfg, "op-geth.nodekeyhex")
	if cfg.PrivateKey == nil {
		t.Fatal("expected PrivateKey to be set")
	}
}

// --- GetJWTSecretFromKMS tests ---

func TestGetJWTSecretFromKMS_Success(t *testing.T) {
	validJWT := strings.Repeat("aa", 32)
	restore := mockKMSGetSecretValue(validJWT, nil)
	defer restore()

	secret := GetJWTSecretFromKMS("op-geth.jwtsecret")
	if len(secret) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(secret))
	}
}
