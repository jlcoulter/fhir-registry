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

// TestVersionNewer verifies version comparison, including the rule that an
// unlabeled version is preferred over a labeled one with the same core.
func TestVersionNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.2.3", "1.2.2", true},
		{"1.2.2", "1.2.3", false},
		{"1.2.3", "1.2.3", false},
		{"2.0.0", "1.9.9", true},
		{"1.3.0", "1.2.9", true},
		{"1.2.3", "1.2.3-ballot", true}, // unlabeled beats labeled
		{"1.2.3-ballot", "1.2.3", false},
		{"1.2.3", "1.2.3-snapshot", true},
		{"4.0.1", "4.0.0", true},
		{"1.2.3", "1.2.3.4", true}, // equal core; shorter string preferred
		{"1.2.3.4", "1.2.3", false},
	}
	for _, tc := range cases {
		if got := versionNewer(tc.a, tc.b); got != tc.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestVersionParts verifies parsing of version strings into numeric parts,
// with non-numeric segments defaulting to 0.
func TestVersionParts(t *testing.T) {
	cases := []struct {
		v    string
		want [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"4.0.1", [3]int{4, 0, 1}},
		{"1.2.3-ballot", [3]int{1, 2, 3}},
		{"1.2", [3]int{1, 2, 0}},
		{"1", [3]int{1, 0, 0}},
		{"", [3]int{0, 0, 0}},
		{"a.b.c", [3]int{0, 0, 0}},
		{"1.x.3", [3]int{1, 0, 3}},
	}
	for _, tc := range cases {
		if got := versionParts(tc.v); got != tc.want {
			t.Errorf("versionParts(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}
