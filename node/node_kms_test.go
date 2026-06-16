package node

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestObtainJWTSecret_PreInjected(t *testing.T) {
	expected := bytes.Repeat([]byte{0xab}, 32)
	cfg := &Config{
		JWTSecretValue: expected,
		DataDir:        t.TempDir(),
	}

	n := &Node{config: cfg}
	got, err := n.obtainJWTSecret("/nonexistent/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("expected pre-injected secret, got different value")
	}
}

func TestObtainJWTSecret_FallbackToFile(t *testing.T) {
	dir := t.TempDir()
	jwtFile := filepath.Join(dir, "jwt.hex")
	// Write a valid 32-byte hex secret (with 0x prefix as geth expects)
	hexSecret := "0x" + "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd"
	if err := os.WriteFile(jwtFile, []byte(hexSecret), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		JWTSecretValue: nil,
		DataDir:        dir,
	}

	n := &Node{config: cfg}
	got, err := n.obtainJWTSecret(jwtFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("expected 32-byte secret, got %d bytes", len(got))
	}
}

func TestObtainJWTSecret_NoPreInjected_NoFile_Generates(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		JWTSecretValue: nil,
		DataDir:        dir,
	}

	n := &Node{config: cfg, dirLock: nil}
	// point to a non-existent file in temp dir — ObtainJWTSecret will generate one
	jwtFile := filepath.Join(dir, "jwtsecret")
	got, err := n.obtainJWTSecret(jwtFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("expected generated 32-byte secret, got %d bytes", len(got))
	}
	// Verify file was written
	if _, err := os.Stat(jwtFile); os.IsNotExist(err) {
		t.Fatal("expected JWT file to be generated")
	}
}
