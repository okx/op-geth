package kms

import (
	"errors"
	"os"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv(envProvider)
	os.Unsetenv(envSecretName)
	os.Unsetenv(envRegion)
}

func setFullEnv(t *testing.T) {
	t.Helper()
	os.Setenv(envProvider, "ALIYUN")
	os.Setenv(envSecretName, "test-secret")
	os.Setenv(envRegion, "cn-hangzhou")
}

func resetState() {
	enabled = false
	initialized = false
	initErr = nil
}

func TestIsEnabled_AllUnset(t *testing.T) {
	clearEnv(t)
	ok, err := IsEnabled()
	if ok {
		t.Fatal("expected false when no env vars set")
	}
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestIsEnabled_AllSet(t *testing.T) {
	setFullEnv(t)
	defer clearEnv(t)

	ok, err := IsEnabled()
	if !ok {
		t.Fatal("expected true when all env vars set")
	}
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestIsEnabled_PartialSet(t *testing.T) {
	clearEnv(t)
	os.Setenv(envProvider, "ALIYUN")
	defer clearEnv(t)

	ok, err := IsEnabled()
	if ok {
		t.Fatal("expected false on partial config")
	}
	if err == nil {
		t.Fatal("expected error on partial config")
	}
}

func TestIsEnabled_TwoOfThreeSet(t *testing.T) {
	clearEnv(t)
	os.Setenv(envProvider, "ALIYUN")
	os.Setenv(envRegion, "cn-hangzhou")
	defer clearEnv(t)

	ok, err := IsEnabled()
	if ok {
		t.Fatal("expected false on partial config")
	}
	if err == nil {
		t.Fatal("expected error on partial config")
	}
}

func TestInit_KMSDisabled(t *testing.T) {
	resetState()
	clearEnv(t)

	err := Init()
	if err != nil {
		t.Fatalf("expected no error when KMS disabled, got: %v", err)
	}
	if Enabled() {
		t.Fatal("expected Enabled() == false when KMS env not set")
	}
}

func TestInit_KMSEnabled_SDKSuccess(t *testing.T) {
	resetState()
	setFullEnv(t)
	defer clearEnv(t)

	origInit := sdkInit
	sdkInit = func() error { return nil }
	defer func() { sdkInit = origInit }()

	err := Init()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !Enabled() {
		t.Fatal("expected Enabled() == true after successful init")
	}
}

func TestInit_KMSEnabled_SDKFailure(t *testing.T) {
	resetState()
	setFullEnv(t)
	defer clearEnv(t)

	origInit := sdkInit
	sdkInit = func() error { return errors.New("connection refused") }
	defer func() { sdkInit = origInit }()

	err := Init()
	if err == nil {
		t.Fatal("expected error when SDK init fails")
	}
	if Enabled() {
		t.Fatal("expected Enabled() == false when SDK init fails")
	}
}

func TestInit_PartialConfig(t *testing.T) {
	resetState()
	clearEnv(t)
	os.Setenv(envProvider, "ALIYUN")
	defer clearEnv(t)

	err := Init()
	if err == nil {
		t.Fatal("expected error on partial config")
	}
	if Enabled() {
		t.Fatal("expected Enabled() == false on partial config")
	}
}

func TestGetSecret_Success(t *testing.T) {
	resetState()
	setFullEnv(t)
	defer clearEnv(t)

	origInit := sdkInit
	origGet := sdkGetSecretValue
	sdkInit = func() error { return nil }
	sdkGetSecretValue = func(key string) (string, error) {
		if key == "op-geth.nodekeyhex" {
			return "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", nil
		}
		return "", errors.New("unknown key")
	}
	defer func() {
		sdkInit = origInit
		sdkGetSecretValue = origGet
	}()

	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	val, err := GetSecret("op-geth.nodekeyhex")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if val != "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890" {
		t.Fatalf("unexpected value: %s", val)
	}
}

func TestGetSecret_EmptyValue(t *testing.T) {
	resetState()
	setFullEnv(t)
	defer clearEnv(t)

	origInit := sdkInit
	origGet := sdkGetSecretValue
	sdkInit = func() error { return nil }
	sdkGetSecretValue = func(key string) (string, error) { return "", nil }
	defer func() {
		sdkInit = origInit
		sdkGetSecretValue = origGet
	}()

	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err := GetSecret("op-geth.nodekeyhex")
	if err == nil {
		t.Fatal("expected error for empty value")
	}
}

func TestGetSecret_FetchError(t *testing.T) {
	resetState()
	setFullEnv(t)
	defer clearEnv(t)

	origInit := sdkInit
	origGet := sdkGetSecretValue
	sdkInit = func() error { return nil }
	sdkGetSecretValue = func(key string) (string, error) {
		return "", errors.New("network timeout")
	}
	defer func() {
		sdkInit = origInit
		sdkGetSecretValue = origGet
	}()

	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err := GetSecret("op-geth.nodekeyhex")
	if err == nil {
		t.Fatal("expected error for fetch failure")
	}
}

func TestGetSecret_NotEnabled(t *testing.T) {
	resetState()
	clearEnv(t)

	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err := GetSecret("any-key")
	if err == nil {
		t.Fatal("expected error when KMS not enabled")
	}
}

func TestInit_Idempotent(t *testing.T) {
	resetState()
	setFullEnv(t)
	defer clearEnv(t)

	callCount := 0
	origInit := sdkInit
	sdkInit = func() error {
		callCount++
		return nil
	}
	defer func() { sdkInit = origInit }()

	if err := Init(); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}
	if err := Init(); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected SDK Init called once, got %d", callCount)
	}
}
