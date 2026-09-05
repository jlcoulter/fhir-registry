package fhir

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeTestPackageArchive writes a minimal FHIR package .tgz archive with a
// package.json manifest and a single StructureDefinition, returning the path.
func writeTestPackageArchive(t *testing.T, dir, name, version string, deps map[string]string) string {
	t.Helper()
	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := map[string]any{
		"name":         name,
		"version":      version,
		"dependencies": deps,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), data, 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	sd := map[string]any{
		"resourceType": "StructureDefinition",
		"id":           "Test",
		"url":          "http://example.org/StructureDefinition/" + name,
		"name":         "Test",
		"type":         "Test",
		"kind":         "resource",
		"abstract":     false,
		"snapshot": map[string]any{
			"element": []map[string]any{
				{"id": "Test", "path": "Test", "min": 0, "max": "1"},
			},
		},
	}
	sdData, err := json.Marshal(sd)
	if err != nil {
		t.Fatalf("marshal sd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "StructureDefinition-Test.json"), sdData, 0o644); err != nil {
		t.Fatalf("write sd: %v", err)
	}

	archivePath := filepath.Join(dir, name+"-"+version+".tgz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	err = filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(pkgDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = "package/" + rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(content)
		return err
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return archivePath
}

// TestIndexLocalArchives verifies that IndexLocalArchives scans a directory
// for package archives and indexes them by name@version.
func TestIndexLocalArchives(t *testing.T) {
	dir := t.TempDir()
	writeTestPackageArchive(t, dir, "pkg.a", "1.0.0", nil)
	writeTestPackageArchive(t, dir, "pkg.b", "2.0.0", nil)

	client := &PackageClient{}
	if err := client.IndexLocalArchives(dir); err != nil {
		t.Fatalf("IndexLocalArchives: %v", err)
	}
	if len(client.LocalArchives) != 2 {
		t.Fatalf("got %d local archives, want 2", len(client.LocalArchives))
	}
	if _, ok := client.LocalArchives["pkg.a@1.0.0"]; !ok {
		t.Error("pkg.a@1.0.0 not indexed")
	}
	if _, ok := client.LocalArchives["pkg.b@2.0.0"]; !ok {
		t.Error("pkg.b@2.0.0 not indexed")
	}
}

// TestIndexLocalArchivesMissingDir verifies that a non-existent directory is
// not an error (it simply indexes nothing).
func TestIndexLocalArchivesMissingDir(t *testing.T) {
	client := &PackageClient{}
	if err := client.IndexLocalArchives(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("IndexLocalArchives on missing dir: %v", err)
	}
	if len(client.LocalArchives) != 0 {
		t.Fatalf("got %d archives, want 0", len(client.LocalArchives))
	}
}

// TestDownloadPrefersLocalArchive verifies that Download uses a local archive
// when one is indexed, without hitting the network.
func TestDownloadPrefersLocalArchive(t *testing.T) {
	dir := t.TempDir()
	archive := writeTestPackageArchive(t, dir, "pkg.a", "1.0.0", nil)

	client := &PackageClient{
		CacheDir:      filepath.Join(t.TempDir(), "cache"),
		LocalArchives: map[string]string{"pkg.a@1.0.0": archive},
	}
	version, cacheDir, err := client.Download(context.Background(), "pkg.a", "1.0.0")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", version)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "package.json")); err != nil {
		t.Errorf("local archive not extracted to cache: %v", err)
	}
}

// TestDownloadLocalArchiveFloatingVersion verifies that a floating version
// reference resolves against a local archive index.
func TestDownloadLocalArchiveFloatingVersion(t *testing.T) {
	dir := t.TempDir()
	archive := writeTestPackageArchive(t, dir, "pkg.a", "1.0.0", nil)

	client := &PackageClient{
		CacheDir:      filepath.Join(t.TempDir(), "cache"),
		LocalArchives: map[string]string{"pkg.a@1.0.0": archive},
	}
	version, _, err := client.Download(context.Background(), "pkg.a", "current")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", version)
	}
}

// TestLoadPackageTgzWithDepsLocal verifies that LoadPackageTgzWithDeps resolves
// dependencies from local archives without network access.
func TestLoadPackageTgzWithDepsLocal(t *testing.T) {
	dir := t.TempDir()
	// dep depends on nothing; root depends on dep.
	depArchive := writeTestPackageArchive(t, dir, "pkg.dep", "1.0.0", nil)
	rootArchive := writeTestPackageArchive(t, dir, "pkg.root", "1.0.0", map[string]string{"pkg.dep": "1.0.0"})

	client := &PackageClient{
		CacheDir:      filepath.Join(t.TempDir(), "cache"),
		LocalArchives: map[string]string{"pkg.dep@1.0.0": depArchive},
	}
	reg := NewRegistry()
	if err := reg.LoadPackageTgzWithDeps(context.Background(), rootArchive, client); err != nil {
		t.Fatalf("LoadPackageTgzWithDeps: %v", err)
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/pkg.root"); !ok {
		t.Error("root definition not loaded")
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/pkg.dep"); !ok {
		t.Error("dep definition not loaded")
	}
}

// TestConflictPolicyStrict verifies that strict policy errors on a version
// conflict.
func TestConflictPolicyStrict(t *testing.T) {
	dir := t.TempDir()
	depA := writeTestPackageArchive(t, dir, "pkg.dep", "1.0.0", nil)
	depB := writeTestPackageArchive(t, dir, "pkg.dep", "2.0.0", nil)

	client := &PackageClient{
		CacheDir:      filepath.Join(t.TempDir(), "cache"),
		LocalArchives: map[string]string{"pkg.dep@1.0.0": depA, "pkg.dep@2.0.0": depB},
		ConflictPolicy: ConflictPolicyStrict,
	}
	// First download selects 1.0.0.
	if _, _, err := client.Download(context.Background(), "pkg.dep", "1.0.0"); err != nil {
		t.Fatalf("first Download: %v", err)
	}
	// Second download of a different version must fail under strict.
	if _, _, err := client.Download(context.Background(), "pkg.dep", "2.0.0"); err == nil {
		t.Error("expected conflict error under strict policy")
	}
}

// TestConflictPolicyRootWins verifies that root-wins policy keeps the first
// selected version.
func TestConflictPolicyRootWins(t *testing.T) {
	dir := t.TempDir()
	depA := writeTestPackageArchive(t, dir, "pkg.dep", "1.0.0", nil)
	depB := writeTestPackageArchive(t, dir, "pkg.dep", "2.0.0", nil)

	client := &PackageClient{
		CacheDir:      filepath.Join(t.TempDir(), "cache"),
		LocalArchives: map[string]string{"pkg.dep@1.0.0": depA, "pkg.dep@2.0.0": depB},
		ConflictPolicy: ConflictPolicyRootWins,
	}
	if _, _, err := client.Download(context.Background(), "pkg.dep", "1.0.0"); err != nil {
		t.Fatalf("first Download: %v", err)
	}
	version, _, err := client.Download(context.Background(), "pkg.dep", "2.0.0")
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0 (root-wins keeps first)", version)
	}
}
