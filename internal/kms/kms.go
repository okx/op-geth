package kms

import (
	"fmt"
	"os"
	"sync"
)

var (
	once    sync.Once
	enabled bool
	initErr error

	// SDK function variables — overridden by RegisterSDK or in tests.
	sdkInit           func() error                         = func() error { return fmt.Errorf("kms: SDK not linked") }
	sdkGetSecretValue func(key string) (string, error)     = func(key string) (string, error) { return "", fmt.Errorf("kms: SDK not linked") }
)

// RegisterSDK wires the real KMS SDK functions. Must be called before InitFromEnv.
func RegisterSDK(initFn func() error, getSecretFn func(key string) (string, error)) {
	sdkInit = initFn
	sdkGetSecretValue = getSecretFn
}

// ResetForTest resets internal state for testing. Not safe for concurrent use.
func ResetForTest() {
	once = sync.Once{}
	enabled = false
	initErr = nil
}

func InitFromEnv() error {
	once.Do(func() {
		provider := os.Getenv("KMS_PROVIDER")
		secretName := os.Getenv("KMS_SECRET_NAME")
		region := os.Getenv("KMS_REGION")

		allSet := provider != "" && secretName != "" && region != ""
		noneSet := provider == "" && secretName == "" && region == ""

		if noneSet {
			enabled = false
			return
		}
		if !allSet {
			initErr = fmt.Errorf("kms: partial env config detected (KMS_PROVIDER=%q, KMS_SECRET_NAME=%q, KMS_REGION=%q); all three must be set or all unset", provider, secretName, region)
			return
		}

		if err := sdkInit(); err != nil {
			initErr = fmt.Errorf("kms: Init() failed: %w", err)
			return
		}
		enabled = true
	})
	return initErr
}

func IsEnabled() bool {
	return enabled
}

func GetSecretValue(key string) (string, error) {
	if !enabled {
		return "", fmt.Errorf("kms: not enabled")
	}
	val, err := sdkGetSecretValue(key)
	if err != nil {
		return "", fmt.Errorf("kms: GetSecretValue(%q) failed: %w", key, err)
	}
	if val == "" {
		return "", fmt.Errorf("kms: GetSecretValue(%q) returned empty value", key)
	}
	return val, nil
}
