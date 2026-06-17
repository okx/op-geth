package kms

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
	okkms "gitlab.okg.com/okcoin-commons/ok-kms-go"
)

type Provider interface {
	Init() error
	GetSecretValue(key string) (string, error)
}

type sdkProvider struct{}

func (s *sdkProvider) Init() error                              { return okkms.Init() }
func (s *sdkProvider) GetSecretValue(key string) (string, error) { return okkms.GetSecretValue(key) }

var (
	once      sync.Once
	enabled   bool
	initError error
	provider  Provider = &sdkProvider{}
)

func ResetForTesting(p Provider) {
	once = sync.Once{}
	enabled = false
	initError = nil
	if p != nil {
		provider = p
	} else {
		provider = &sdkProvider{}
	}
}

func IsEnabled() bool {
	return enabled
}

func Init() error {
	once.Do(func() {
		p := os.Getenv("KMS_PROVIDER")
		secretName := os.Getenv("KMS_SECRET_NAME")
		region := os.Getenv("KMS_REGION")

		if p == "" && secretName == "" && region == "" {
			enabled = false
			return
		}
		if p == "" || secretName == "" || region == "" {
			enabled = true
			initError = fmt.Errorf("incomplete KMS configuration: KMS_PROVIDER=%q, KMS_SECRET_NAME=%q, KMS_REGION=%q — all three must be set", p, secretName, region)
			return
		}
		enabled = true
		initError = provider.Init()
	})
	return initError
}

func GetSecretValue(key string) (string, error) {
	if !enabled {
		return "", fmt.Errorf("KMS is not enabled")
	}
	val, err := provider.GetSecretValue(key)
	if err != nil {
		return "", fmt.Errorf("KMS GetSecretValue(%q): %w", key, err)
	}
	if val == "" {
		return "", fmt.Errorf("KMS key %q returned empty value", key)
	}
	return val, nil
}

func MustGetHexBytes(key string, expectedLen int) ([]byte, error) {
	raw, err := GetSecretValue(key)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")

	b, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("KMS key %q: invalid hex: %w", key, err)
	}
	if expectedLen > 0 && len(b) != expectedLen {
		return nil, fmt.Errorf("KMS key %q: expected %d bytes, got %d", key, expectedLen, len(b))
	}
	return b, nil
}

func MustGetPrivateKey(key string) (*ecdsa.PrivateKey, error) {
	raw, err := GetSecretValue(key)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")

	pk, err := crypto.HexToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("KMS key %q: invalid private key: %w", key, err)
	}
	return pk, nil
}
