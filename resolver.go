package fhir

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// packageManifest is the subset of package.json we care about.
type packageManifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

// loadPackageJSON reads package.json from a package directory. The directory
// may contain package.json at its root or under a "package" subdirectory.
func loadPackageJSON(pkgDir string) (*packageManifest, error) {
	var candidates []string
	dir := pkgDir
	if filepath.Base(pkgDir) != "package" {
		candidates = []string{
			filepath.Join(pkgDir, "package", "package.json"),
			filepath.Join(pkgDir, "package.json"),
		}
	} else {
		candidates = []string{filepath.Join(pkgDir, "package.json")}
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			var m packageManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("%w: parsing %s: %v", ErrParseFailure, path, err)
			}
			return &m, nil
		}
	}
	if filepath.Base(dir) != "package" {
		dir = filepath.Join(dir, "package")
	}
	return nil, fmt.Errorf("%w: no package.json found in %s", ErrPackageNotFound, pkgDir)
}

// loadDir loads every .json resource in a directory into the registry.
// If the directory contains a "package" subdirectory (standard FHIR layout),
// that subdirectory is loaded instead.
func (r *Registry) loadDir(pkgDir string) error {
	dir := pkgDir
	if filepath.Base(dir) != "package" {
		sub := filepath.Join(dir, "package")
		if st, err := os.Stat(sub); err == nil && st.IsDir() {
			dir = sub
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("%w: reading package dir %s: %v", ErrPackageNotFound, dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("%w: reading %s: %v", ErrPackageNotFound, e.Name(), err)
		}
		if err := r.addResource(e.Name(), data); err != nil {
			return err
		}
	}
	return nil
}

// LoadPackageWithDeps loads a package directory and resolves its full
// dependency chain, downloading any missing packages from the registry.
func (r *Registry) LoadPackageWithDeps(ctx context.Context, pkgDir string, client *PackageClient) error {
	return r.loadPackageWithDeps(ctx, pkgDir, client, nil)
}

// loadPackageWithDeps is the recursive core; visited guards against circular
// or duplicate dependency processing.
func (r *Registry) loadPackageWithDeps(ctx context.Context, pkgDir string, client *PackageClient, visited map[string]bool) error {
	if visited == nil {
		visited = make(map[string]bool)
	}
	m, err := loadPackageJSON(pkgDir)
	if err != nil {
		return err
	}
	if m.Name != "" && visited[m.Name+"#"+m.Version] {
		return nil
	}
	if m.Name != "" {
		visited[m.Name+"#"+m.Version] = true
	}
	if err := r.loadDir(pkgDir); err != nil {
		return err
	}
	if len(m.Dependencies) == 0 {
		return nil
	}

	// Sort for deterministic processing order.
	names := make([]string, 0, len(m.Dependencies))
	for name := range m.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, depName := range names {
		ref := m.Dependencies[depName]
		version, err := client.ResolveVersion(ctx, depName, ref)
		if err != nil {
			return err
		}
		if visited[depName+"#"+version] {
			continue
		}
		depDir, err := client.Download(ctx, depName, version)
		if err != nil {
			return fmt.Errorf("dependency %s#%s: %w", depName, version, err)
		}
		if err := r.loadPackageWithDeps(ctx, depDir, client, visited); err != nil {
			return fmt.Errorf("dependency %s#%s: %w", depName, version, err)
		}
	}
	return nil
}
