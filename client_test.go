package fhir

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	if client.HTTPClient == nil {
		t.Errorf("HTTPClient not set: %+v", client)
	}
	if len(client.registryURLs) == 0 {
		t.Errorf("registryURLs not set: %+v", client)
	}
	if client.registryURLs[0] != "https://packages2.fhir.org/packages" {
		t.Errorf("primary registryURL = %q, want packages2.fhir.org", client.registryURLs[0])
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

// TestIsFloatingVersion verifies detection of floating version references.
func TestIsFloatingVersion(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"current", true},
		{"latest", true},
		{"*", true},
		{"CURRENT", true},
		{"Current", true},
		{"  current  ", true},
		{"4.0.1", false},
		{"4.0.x", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isFloatingVersion(tc.v); got != tc.want {
			t.Errorf("isFloatingVersion(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestFloatingVersionTag verifies mapping of floating references to dist-tags.
func TestFloatingVersionTag(t *testing.T) {
	cases := []struct {
		v    string
		want string
	}{
		{"current", "latest"},
		{"latest", "latest"},
		{"*", "*"},
		{"CURRENT", "latest"},
	}
	for _, tc := range cases {
		if got := floatingVersionTag(tc.v); got != tc.want {
			t.Errorf("floatingVersionTag(%q) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

// TestResolveVersionAndTarball verifies version resolution against a mock
// registry, covering exact, wildcard, and floating version references.
func TestResolveVersionAndTarball(t *testing.T) {
	meta := `{
		"dist-tags": {"latest": "4.0.1"},
		"versions": {
			"4.0.0": {"dist": {"tarball": "https://reg.example/4.0.0.tgz"}},
			"4.0.1": {"dist": {"tarball": "https://reg.example/4.0.1.tgz"}},
			"4.0.2": {"dist": {"tarball": "https://reg.example/4.0.2.tgz"}}
		}
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(meta))
	}))
	defer srv.Close()

	client := &PackageClient{
		HTTPClient:   srv.Client(),
		registryURLs: []string{srv.URL},
	}
	ctx := context.Background()

	cases := []struct {
		name, ref   string
		wantVersion string
		wantTarball string
	}{
		{"pkg", "4.0.1", "4.0.1", "https://reg.example/4.0.1.tgz"},
		{"pkg", "4.0.x", "4.0.2", "https://reg.example/4.0.2.tgz"},
		{"pkg", "current", "4.0.1", "https://reg.example/4.0.1.tgz"},
		{"pkg", "latest", "4.0.1", "https://reg.example/4.0.1.tgz"},
	}
	for _, tc := range cases {
		version, tarball, err := client.resolveVersionAndTarball(ctx, tc.name, tc.ref)
		if err != nil {
			t.Errorf("resolveVersionAndTarball(%q): %v", tc.ref, err)
			continue
		}
		if version != tc.wantVersion {
			t.Errorf("resolveVersionAndTarball(%q) version = %q, want %q", tc.ref, version, tc.wantVersion)
		}
		if tarball != tc.wantTarball {
			t.Errorf("resolveVersionAndTarball(%q) tarball = %q, want %q", tc.ref, tarball, tc.wantTarball)
		}
	}
}

// TestResolveVersionAndTarballFallback verifies that the client tries each
// registry URL in order until one succeeds.
func TestResolveVersionAndTarballFallback(t *testing.T) {
	meta := `{"dist-tags":{"latest":"1.0.0"},"versions":{"1.0.0":{"dist":{"tarball":"https://reg.example/1.0.0.tgz"}}}}`
	var first, second *httptest.Server
	first = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer first.Close()
	second = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(meta))
	}))
	defer second.Close()

	client := &PackageClient{
		HTTPClient:   second.Client(),
		registryURLs: []string{first.URL, second.URL},
	}
	version, tarball, err := client.resolveVersionAndTarball(context.Background(), "pkg", "1.0.0")
	if err != nil {
		t.Fatalf("resolveVersionAndTarball: %v", err)
	}
	if version != "1.0.0" || tarball != "https://reg.example/1.0.0.tgz" {
		t.Errorf("got version=%q tarball=%q, want 1.0.0 / https://reg.example/1.0.0.tgz", version, tarball)
	}
}

// TestResolveVersionAndTarballAllFail verifies an error when every registry
// fails.
func TestResolveVersionAndTarballAllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	client := &PackageClient{
		HTTPClient:   srv.Client(),
		registryURLs: []string{srv.URL},
	}
	if _, _, err := client.resolveVersionAndTarball(context.Background(), "pkg", "1.0.0"); err == nil {
		t.Error("expected error when all registries fail")
	}
}
