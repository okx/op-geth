package kms

// ResetForTest resets internal KMS state for use in tests.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	enabled = false
	initialized = false
	initErr = nil
}

// SetTestSDK overrides the SDK functions for testing.
func SetTestSDK(initFn func() error, getFn func(string) (string, error)) {
	sdkInit = initFn
	sdkGetSecretValue = getFn
}
