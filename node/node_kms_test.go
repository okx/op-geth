package node

import (
	"bytes"
	"testing"
)

func TestObtainJWTSecret_KMSProvided(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}

	n := &Node{
		config: &Config{
			KMSJWTSecret: secret,
		},
	}

	got, err := n.obtainJWTSecret("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("expected KMS secret, got different value")
	}
}

func TestObtainJWTSecret_KMSNil_FallsThrough(t *testing.T) {
	dir := t.TempDir()
	n := &Node{
		config: &Config{
			KMSJWTSecret: nil,
			DataDir:      dir,
			Name:         "test",
		},
	}

	// ObtainJWTSecret auto-generates when file does not exist, but needs the parent dir.
	// When no cliParam is set, ResolvePath constructs the path under the instance dir.
	// Use a cliParam pointing to the temp dir instead to avoid filesystem structure issues.
	jwtPath := dir + "/jwtsecret"
	got, err := n.obtainJWTSecret(jwtPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("expected 32-byte auto-generated secret, got %d bytes", len(got))
	}
}
