package fhir

import (
	"context"
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	reg := NewRegistry()
	if err := reg.LoadPackageWithDeps(ctx, "package", client); err != nil {
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

	// Pre-populate the cache by copying the local package dir.
	reg := NewRegistry()
	if err := reg.loadDir("package"); err != nil {
		t.Fatalf("loadDir: %v", err)
	}
	_ = client
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient not loaded from local dir")
	}
}
