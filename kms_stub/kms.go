// Package kms provides the ok-kms-go SDK interface for KMS secret management.
// This is a build stub — the real implementation is provided by
// gitlab.okg.com/okcoin-commons/ok-kms-go v1.0.9 in production builds.
package kms

import "errors"

// Init initializes the KMS SDK using environment variables (KMS_PROVIDER, KMS_SECRET_NAME, KMS_REGION).
// Must be called before GetSecretValue.
func Init() error {
	return errors.New("ok-kms-go stub: Init() not implemented — use real SDK in production")
}

// GetSecretValue retrieves a secret value by key name from KMS.
func GetSecretValue(key string) (string, error) {
	return "", errors.New("ok-kms-go stub: GetSecretValue() not implemented — use real SDK in production")
}
