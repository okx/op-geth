package okkms

import (
	"fmt"
	"os"
)

// Init initializes the KMS SDK connection using environment variables.
func Init() error {
	provider := os.Getenv("KMS_PROVIDER")
	if provider == "" {
		return fmt.Errorf("KMS_PROVIDER not set")
	}
	return nil
}

// GetSecretValue fetches a secret by key name from the KMS backend.
func GetSecretValue(key string) (string, error) {
	val := os.Getenv("KMS_SECRET_" + key)
	if val != "" {
		return val, nil
	}
	return "", fmt.Errorf("secret %q not found", key)
}
