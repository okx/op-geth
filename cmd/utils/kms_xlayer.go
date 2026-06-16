package utils

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	kms "gitlab.okg.com/okcoin-commons/ok-kms-go"
)

const (
	envKMSProvider   = "KMS_PROVIDER"
	envKMSSecretName = "KMS_SECRET_NAME"
	envKMSRegion     = "KMS_REGION"
)

const (
	DefaultKMSKeyNodeKey   = "op-geth.nodekeyhex"
	DefaultKMSKeyJWTSecret = "op-geth.jwtsecret"
)

// Package-level function variables for testability (addresses adversarial review finding #3).
var (
	kmsInit           = kms.Init
	kmsGetSecretValue = kms.GetSecretValue
)

var kmsState struct {
	enabled bool
}

// IsKMSEnabled returns whether KMS has been successfully initialized.
func IsKMSEnabled() bool {
	return kmsState.enabled
}

// InitKMSIfEnabled checks KMS env vars and initializes the SDK.
// Returns nil if KMS is not configured (all 3 envs unset).
// Returns error if partially configured or Init fails.
func InitKMSIfEnabled() error {
	provider := os.Getenv(envKMSProvider)
	secretName := os.Getenv(envKMSSecretName)
	region := os.Getenv(envKMSRegion)

	allSet := provider != "" && secretName != "" && region != ""
	noneSet := provider == "" && secretName == "" && region == ""

	if noneSet {
		kmsState.enabled = false
		return nil
	}
	if !allSet {
		return fmt.Errorf("KMS partially configured: KMS_PROVIDER=%q, KMS_SECRET_NAME=%q, KMS_REGION=%q; all three must be set or all unset", provider, secretName, region)
	}

	if err := kmsInit(); err != nil {
		return fmt.Errorf("kms.Init() failed: %w", err)
	}
	kmsState.enabled = true
	log.Info("KMS initialized successfully", "provider", provider, "region", region)
	return nil
}

// GetKMSValue retrieves a secret value from KMS by key name.
func GetKMSValue(keyName, itemDesc string) (string, error) {
	val, err := kmsGetSecretValue(keyName)
	if err != nil {
		return "", fmt.Errorf("KMS GetSecretValue failed for %s (key=%q): %w", itemDesc, keyName, err)
	}
	if strings.TrimSpace(val) == "" {
		return "", fmt.Errorf("KMS returned empty value for %s (key=%q)", itemDesc, keyName)
	}
	return strings.TrimSpace(val), nil
}

// ParseHexPrivateKey parses a hex-encoded secp256k1 private key (with optional 0x prefix).
func ParseHexPrivateKey(hexStr string) ([]byte, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	return b, nil
}

// ParseJWTSecret parses a hex-encoded JWT secret (with optional 0x prefix).
func ParseJWTSecret(hexStr string) ([]byte, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT hex: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("JWT secret must be 32 bytes, got %d", len(b))
	}
	return b, nil
}

// SetNodeKeyFromKMS retrieves the node private key from KMS and sets it in P2P config.
// Calls log.Crit (exits process) on any failure.
func SetNodeKeyFromKMS(p2pCfg *p2p.Config, keyName string) {
	hexVal, err := GetKMSValue(keyName, "nodekeyhex")
	if err != nil {
		log.Crit("Failed to get node key from KMS", "key", keyName, "err", err)
	}
	keyBytes, err := ParseHexPrivateKey(hexVal)
	if err != nil {
		log.Crit("Invalid node key from KMS", "key", keyName, "err", err)
	}
	privKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		log.Crit("Failed to parse node key as ECDSA", "key", keyName, "err", err)
	}
	p2pCfg.PrivateKey = privKey
	log.Info("Node key loaded from KMS", "key", keyName)
}

// GetJWTSecretFromKMS retrieves the JWT secret from KMS, parses it, and returns the 32-byte value.
// Calls log.Crit (exits process) on any failure.
func GetJWTSecretFromKMS(keyName string) []byte {
	hexVal, err := GetKMSValue(keyName, "JWTSecret")
	if err != nil {
		log.Crit("Failed to get JWT secret from KMS", "key", keyName, "err", err)
	}
	secret, err := ParseJWTSecret(hexVal)
	if err != nil {
		log.Crit("Invalid JWT secret from KMS", "key", keyName, "err", err)
	}
	log.Info("JWT secret loaded from KMS", "key", keyName)
	return secret
}
