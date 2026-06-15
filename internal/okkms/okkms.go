package okkms

import (
	"fmt"
	"os"
	"sync"
)

var (
	once       sync.Once
	initErr    error
	secretName string
)

func Init() error {
	once.Do(func() {
		secretName = os.Getenv("KMS_SECRET_NAME")
		if secretName == "" {
			initErr = fmt.Errorf("KMS_SECRET_NAME not set")
		}
	})
	return initErr
}

func GetSecretValue(key string) (string, error) {
	val := os.Getenv("KMS_SECRET_" + key)
	if val != "" {
		return val, nil
	}
	return "", fmt.Errorf("secret %q not found", key)
}
