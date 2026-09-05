package fhir

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PackageClient downloads FHIR packages from a registry server and caches
// them locally in a FHIR-standard layout: <cacheDir>/<name>#<version>/package/.
type PackageClient struct {
	// RegistryURL is the package registry root, e.g. "https://packages.fhir.org".
	RegistryURL string
	// TarballURL is the base URL used to fetch tarballs. If empty it is derived
	// from the metadata response (dist.tarball). Set to "https://packages.simplifier.net"
	// to avoid a metadata round-trip for known versions.
	TarballURL string
	// CacheDir is where extracted packages are stored.
	CacheDir string
	// HTTPClient is the transport used for all requests.
	HTTPClient *http.Client
}

// NewPackageClient returns a client with sensible defaults. It returns an
// error if the user's home directory cannot be resolved, since the cache
// directory is rooted there.
func NewPackageClient() (*PackageClient, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	return &PackageClient{
		RegistryURL: "https://packages.fhir.org",
		TarballURL:  "https://packages.simplifier.net",
		CacheDir:    filepath.Join(home, ".fhir", "packages"),
		HTTPClient:  &http.Client{},
	}, nil
}

// PackageDir returns the local directory for a package (even if not yet present).
func (c *PackageClient) PackageDir(name, version string) string {
	return filepath.Join(c.CacheDir, name+"#"+version)
}

// Cached reports whether a package is already extracted in the cache.
func (c *PackageClient) Cached(name, version string) bool {
	_, err := os.Stat(filepath.Join(c.PackageDir(name, version), "package", "package.json"))
	return err == nil
}

// metadata is the shape of the registry metadata response.
type metadata struct {
	Versions map[string]struct {
		Dist struct {
			Tarball string `json:"tarball"`
		} `json:"dist"`
	} `json:"versions"`
}

// ResolveVersion resolves a version reference (exact, or a patch wildcard like
// "4.0.x") to an exact available version by querying the registry.
func (c *PackageClient) ResolveVersion(ctx context.Context, name, versionRef string) (string, error) {
	if !strings.Contains(versionRef, "x") {
		return versionRef, nil
	}
	url := c.RegistryURL + "/" + name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: querying registry for %s: %v", ErrNetwork, name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: querying registry for %s: status %d", ErrNetwork, name, resp.StatusCode)
	}
	var md metadata
	if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
		return "", fmt.Errorf("%w: decoding registry metadata for %s: %v", ErrParseFailure, name, err)
	}
	prefix := strings.TrimSuffix(versionRef, "x")
	var best string
	for v := range md.Versions {
		if strings.HasPrefix(v, prefix) {
			if best == "" || versionNewer(v, best) {
				best = v
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("%w: no version of %s matches %q", ErrVersionNotFound, name, versionRef)
	}
	return best, nil
}

// versionNewer compares two version strings, ignoring trailing labels
// (e.g. "1.2.3" is newer than "1.2.3-ballot").
func versionNewer(a, b string) bool {
	pa := versionParts(a)
	pb := versionParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	// Equal core: an unlabeled version is preferred over a labeled one.
	return len(a) < len(b)
}

func versionParts(v string) [3]int {
	var parts [3]int
	fields := strings.SplitN(v, "-", 2)[0]
	for i, s := range strings.SplitN(fields, ".", 3) {
		n, err := strconv.Atoi(s)
		if err != nil {
			n = 0
		}
		parts[i] = n
	}
	return parts
}

// Download fetches and extracts a package by name and version into the cache.
// It is a no-op if the package is already cached.
func (c *PackageClient) Download(ctx context.Context, name, version string) (string, error) {
	dir := c.PackageDir(name, version)
	if c.Cached(name, version) {
		return dir, nil
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return "", err
	}

	tarballURL := c.TarballURL + "/" + name + "/" + version
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tarballURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: downloading %s#%s: %v", ErrNetwork, name, version, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: downloading %s#%s: status %d", ErrNetwork, name, version, resp.StatusCode)
	}
	if err := extractTGZToDir(resp.Body, dir); err != nil {
		return "", fmt.Errorf("extracting %s#%s: %w", name, version, err)
	}
	return dir, nil
}

// extractTGZToDir extracts a gzipped tar stream to a destination directory,
// writing to a temp dir first then renaming, so a partially-written package
// is never mistaken for a complete one.
func extractTGZToDir(r io.Reader, dest string) error {
	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Strip the leading "package/" directory component.
		rel, ok := strings.CutPrefix(hdr.Name, "package/")
		if !ok || rel == "" {
			continue
		}
		path := filepath.Join(tmp, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			out, err := os.Create(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
