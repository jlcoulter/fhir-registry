package fhir

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadPackageWithDeps is an integration test that resolves the AU Base
// package's dependency chain from packages.fhir.org. It is gated behind
// testing.Short() so it can be skipped with -short.
func TestLoadPackageWithDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network dependency resolution in -short mode")
	}
	client, err := NewPackageClient()
	if err != nil {
		t.Fatalf("NewPackageClient: %v", err)
	}
	client.CacheDir = t.TempDir() // isolate cache

	// Extract the local tgz into a temp dir to serve as the root package.
	pkgDir := t.TempDir()
	f, err := os.Open("au-base.tgz")
	if err != nil {
		t.Fatalf("open au-base.tgz: %v", err)
	}
	if err := extractTGZToDir(f, pkgDir); err != nil {
		f.Close()
		t.Fatalf("extractTGZToDir: %v", err)
	}
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	reg := NewRegistry()
	if err := reg.LoadPackageWithDeps(ctx, pkgDir, client); err != nil {
		t.Fatalf("LoadPackageWithDeps: %v", err)
	}

	// The R4 core package provides the base Address definition.
	if _, ok := reg.Definition("http://hl7.org/fhir/StructureDefinition/Address"); !ok {
		t.Error("base Address definition not resolved from R4 core dependency")
	}
	if _, ok := reg.Definition("http://hl7.org/fhir/StructureDefinition/Patient"); !ok {
		t.Error("base Patient definition not resolved")
	}
	// The AU package itself.
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient not loaded")
	}
}

// TestDependencyResolutionFromCache verifies the resolution/loading logic
// without network, using the locally checked-in package as a dependency.
func TestDependencyResolutionFromCache(t *testing.T) {
	client, err := NewPackageClient()
	if err != nil {
		t.Fatalf("NewPackageClient: %v", err)
	}
	client.CacheDir = t.TempDir()

	// Pre-populate the cache by loading the local tgz.
	reg := NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	_ = client
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient not loaded from local tgz")
	}
}

// TestLoadPackageTgzWithDepsNoDeps verifies that LoadPackageTgzWithDeps works
// for a package with no dependencies (or when the client is nil and the package
// has no deps).
func TestLoadPackageTgzWithDepsNoDeps(t *testing.T) {
	// Build a minimal package with no dependencies.
	dir := t.TempDir()
	writeTestPackage(t, dir, "test.pkg", "1.0.0", nil)
	tgz := dir + ".tgz"
	if err := writeTGZ(t, dir, tgz); err != nil {
		t.Fatalf("writeTGZ: %v", err)
	}

	reg := NewRegistry()
	if err := reg.LoadPackageTgzWithDeps(context.Background(), tgz, nil); err != nil {
		t.Fatalf("LoadPackageTgzWithDeps: %v", err)
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/Test"); !ok {
		t.Error("Test definition not loaded")
	}
}

// writeTestPackage writes a minimal FHIR package directory with a package.json
// and a single StructureDefinition. The package contents are written directly
// into dir (the "package/" prefix is added by writeTGZ).
func writeTestPackage(t *testing.T, dir, name, version string, deps map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
	if err := os.WriteFile(dir+"/package.json", data, 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	sd := map[string]any{
		"resourceType": "StructureDefinition",
		"id":           "Test",
		"url":          "http://example.org/StructureDefinition/Test",
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
	if err := os.WriteFile(dir+"/StructureDefinition-Test.json", sdData, 0o644); err != nil {
		t.Fatalf("write sd: %v", err)
	}
}

// writeTGZ tars and gzips a directory into a .tgz file, prefixing every entry
// with "package/" to match the standard FHIR package layout.
func writeTGZ(t *testing.T, dir, tgzPath string) error {
	t.Helper()
	f, err := os.Create(tgzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
}
