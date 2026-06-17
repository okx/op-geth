package kms

import (
	"errors"
	"os"
	"testing"
)

func setupTest(t *testing.T) {
	t.Helper()
	ResetForTest()
	os.Unsetenv("KMS_PROVIDER")
	os.Unsetenv("KMS_SECRET_NAME")
	os.Unsetenv("KMS_REGION")
	t.Cleanup(func() {
		ResetForTest()
		os.Unsetenv("KMS_PROVIDER")
		os.Unsetenv("KMS_SECRET_NAME")
		os.Unsetenv("KMS_REGION")
	})
}

func TestInitFromEnv_AllSet_InitSuccess(t *testing.T) {
	setupTest(t)
	RegisterSDK(
		func() error { return nil },
		func(key string) (string, error) { return "value", nil },
	)
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hongkong")

	err := InitFromEnv()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !IsEnabled() {
		t.Fatal("expected IsEnabled() to be true")
	}
}

func TestInitFromEnv_NoneSet(t *testing.T) {
	setupTest(t)

	err := InitFromEnv()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if IsEnabled() {
		t.Fatal("expected IsEnabled() to be false")
	}
}

func TestInitFromEnv_PartialConfig(t *testing.T) {
	setupTest(t)
	os.Setenv("KMS_PROVIDER", "ALIYUN")

	err := InitFromEnv()
	if err == nil {
		t.Fatal("expected error for partial config")
	}
	if !containsStr(err.Error(), "partial env config detected") {
		t.Fatalf("unexpected error: %v", err)
	}
	if IsEnabled() {
		t.Fatal("expected IsEnabled() to be false")
	}
}

func TestInitFromEnv_SDKInitFailure(t *testing.T) {
	setupTest(t)
	RegisterSDK(
		func() error { return errors.New("connection refused") },
		func(key string) (string, error) { return "", nil },
	)
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hongkong")

	err := InitFromEnv()
	if err == nil {
		t.Fatal("expected error when SDK Init fails")
	}
	if !containsStr(err.Error(), "Init() failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if IsEnabled() {
		t.Fatal("expected IsEnabled() to be false when Init fails")
	}
}

func TestInitFromEnv_Idempotent(t *testing.T) {
	setupTest(t)
	callCount := 0
	RegisterSDK(
		func() error { callCount++; return nil },
		func(key string) (string, error) { return "v", nil },
	)
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hongkong")

	_ = InitFromEnv()
	_ = InitFromEnv()
	if callCount != 1 {
		t.Fatalf("expected sdkInit called once, got %d", callCount)
	}
}

func TestGetSecretValue_Success(t *testing.T) {
	setupTest(t)
	RegisterSDK(
		func() error { return nil },
		func(key string) (string, error) { return "secret123", nil },
	)
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hongkong")
	_ = InitFromEnv()

	val, err := GetSecretValue("mykey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("expected 'secret123', got %q", val)
	}
}

func TestGetSecretValue_EmptyValue(t *testing.T) {
	setupTest(t)
	RegisterSDK(
		func() error { return nil },
		func(key string) (string, error) { return "", nil },
	)
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hongkong")
	_ = InitFromEnv()

	_, err := GetSecretValue("mykey")
	if err == nil {
		t.Fatal("expected error for empty value")
	}
	if !containsStr(err.Error(), "returned empty value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetSecretValue_SDKError(t *testing.T) {
	setupTest(t)
	RegisterSDK(
		func() error { return nil },
		func(key string) (string, error) { return "", errors.New("key not found") },
	)
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hongkong")
	_ = InitFromEnv()

	_, err := GetSecretValue("mykey")
	if err == nil {
		t.Fatal("expected error on SDK failure")
	}
	if !containsStr(err.Error(), "GetSecretValue") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetSecretValue_NotEnabled(t *testing.T) {
	setupTest(t)

	_ = InitFromEnv()
	_, err := GetSecretValue("mykey")
	if err == nil {
		t.Fatal("expected error when not enabled")
	}
	if !containsStr(err.Error(), "not enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetSecretValue_KeyPassedCorrectly(t *testing.T) {
	setupTest(t)
	var capturedKey string
	RegisterSDK(
		func() error { return nil },
		func(key string) (string, error) { capturedKey = key; return "val", nil },
	)
	os.Setenv("KMS_PROVIDER", "ALIYUN")
	os.Setenv("KMS_SECRET_NAME", "test-secret")
	os.Setenv("KMS_REGION", "cn-hongkong")
	_ = InitFromEnv()

	_, _ = GetSecretValue("custom.key.name")
	if capturedKey != "custom.key.name" {
		t.Fatalf("expected key 'custom.key.name', got %q", capturedKey)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
