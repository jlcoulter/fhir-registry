package fhir

import (
	"path/filepath"
	"testing"
)

// TestNewPackageClient verifies the client is built with sensible defaults
// and a cache directory rooted at the user's home directory.
func TestNewPackageClient(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	client, err := NewPackageClient()
	if err != nil {
		t.Fatalf("NewPackageClient: %v", err)
	}
	if client == nil {
		t.Fatal("NewPackageClient returned nil client")
	}
	want := filepath.Join(home, ".fhir", "packages")
	if client.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q", client.CacheDir, want)
	}
	if client.RegistryURL == "" || client.TarballURL == "" || client.HTTPClient == nil {
		t.Errorf("defaults not set: %+v", client)
	}
}

// TestNewPackageClientHomeUnset verifies that a failure to resolve the user's
// home directory surfaces as an error rather than silently producing a cache
// path rooted at the filesystem root.
func TestNewPackageClientHomeUnset(t *testing.T) {
	t.Setenv("HOME", "")

	client, err := NewPackageClient()
	if err == nil {
		t.Fatalf("NewPackageClient() = %+v, want error when HOME is unset", client)
	}
}
