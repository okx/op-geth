package kms

import (
	"fmt"
	"os"
	"sync"
)

const (
	envProvider   = "KMS_PROVIDER"
	envSecretName = "KMS_SECRET_NAME"
	envRegion     = "KMS_REGION"

	DefaultKMSNodeKeyHexKey = "op-geth.nodekeyhex"
	DefaultKMSJWTSecretKey  = "op-geth.jwtsecret"
)

var (
	mu          sync.Mutex
	enabled     bool
	initialized bool
	initErr     error

	sdkInit           func() error                          = defaultSDKInit
	sdkGetSecretValue func(key string) (string, error)      = defaultSDKGetSecretValue
)

func defaultSDKInit() error {
	return nil
}

func defaultSDKGetSecretValue(key string) (string, error) {
	return "", fmt.Errorf("KMS SDK not linked")
}

// IsEnabled returns true if all three KMS env vars are set (non-empty).
// If partially set, returns an error (config mistake).
func IsEnabled() (bool, error) {
	provider := os.Getenv(envProvider)
	secretName := os.Getenv(envSecretName)
	region := os.Getenv(envRegion)

	set := 0
	if provider != "" {
		set++
	}
	if secretName != "" {
		set++
	}
	if region != "" {
		set++
	}

	switch set {
	case 0:
		return false, nil
	case 3:
		return true, nil
	default:
		return false, fmt.Errorf("KMS partially configured: KMS_PROVIDER=%q, KMS_SECRET_NAME=%q, KMS_REGION=%q — all three must be set or all unset", provider, secretName, region)
	}
}

// Init initializes the KMS SDK. Must be called once before GetSecret.
func Init() error {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return initErr
	}
	initialized = true

	en, err := IsEnabled()
	if err != nil {
		initErr = err
		return initErr
	}
	if !en {
		return nil
	}

	initErr = sdkInit()
	if initErr == nil {
		enabled = true
	}
	return initErr
}

// Enabled returns whether KMS is active (Init must have been called).
func Enabled() bool {
	return enabled
}

// GetSecret fetches a secret value from KMS by key name.
func GetSecret(keyName string) (string, error) {
	if !enabled {
		return "", fmt.Errorf("KMS not enabled")
	}
	val, err := sdkGetSecretValue(keyName)
	if err != nil {
		return "", fmt.Errorf("KMS fetch failed for key %q: %w", keyName, err)
	}
	if val == "" {
		return "", fmt.Errorf("KMS key %q returned empty value", keyName)
	}
	return val, nil
}
