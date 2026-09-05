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

// ConflictPolicy controls how package version conflicts are resolved when the
// same package is requested at different versions during dependency
// resolution.
type ConflictPolicy string

const (
	// ConflictPolicyStrict fails when a package is requested at a version that
	// conflicts with an already-selected version.
	ConflictPolicyStrict ConflictPolicy = "strict"
	// ConflictPolicyRootWins keeps the first-selected version and overrides
	// later conflicting requests.
	ConflictPolicyRootWins ConflictPolicy = "root-wins"
)

// PackageClient downloads FHIR packages from a registry server and caches
// them locally in a FHIR-standard layout: <cacheDir>/<name>#<version>/package/.
type PackageClient struct {
	// CacheDir is where extracted packages are stored.
	CacheDir string
	// HTTPClient is the transport used for all requests.
	HTTPClient *http.Client
	// LocalArchives maps a package key ("name@version") to a local .tgz/.tar.gz
	// archive path. When set, Download prefers these archives over the network.
	LocalArchives map[string]string
	// ConflictPolicy controls version-conflict resolution. Empty defaults to
	// ConflictPolicyRootWins.
	ConflictPolicy ConflictPolicy
	// registryURLs are the package registry roots, tried in order. The first
	// that returns usable metadata wins.
	registryURLs []string
	// selectedVersions records the version chosen for each package name during
	// dependency resolution, used to apply the conflict policy.
	selectedVersions map[string]string
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
		CacheDir:   filepath.Join(home, ".fhir", "packages"),
		HTTPClient: &http.Client{},
		registryURLs: []string{
			"https://packages2.fhir.org/packages",
			"https://packages.simplifier.net",
		},
	}, nil
}

// PackageDir returns the local directory for a package (even if not yet present).
func (c *PackageClient) PackageDir(name, version string) string {
	return filepath.Join(c.CacheDir, name+"#"+version)
}

// IndexLocalArchives scans a directory recursively for package archives
// (.tgz/.tar.gz), reads each archive's package.json manifest, and indexes the
// archives by "name@version" in LocalArchives. A non-existent directory is not
// an error (it indexes nothing). Existing entries are not overwritten.
func (c *PackageClient) IndexLocalArchives(dir string) error {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if c.LocalArchives == nil {
		c.LocalArchives = make(map[string]string)
	}
	return filepath.WalkDir(dir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		l := strings.ToLower(p)
		if !strings.HasSuffix(l, ".tgz") && !strings.HasSuffix(l, ".tar.gz") {
			return nil
		}
		manifest, err := readPackageManifestFromArchive(p)
		if err != nil {
			return nil // skip archives without a readable manifest
		}
		if manifest.Name == "" || manifest.Version == "" {
			return nil
		}
		key := manifest.Name + "@" + manifest.Version
		if _, exists := c.LocalArchives[key]; !exists {
			c.LocalArchives[key] = p
		}
		return nil
	})
}

// readPackageManifestFromArchive reads the package.json manifest from a .tgz
// archive.
func readPackageManifestFromArchive(archivePath string) (*packageManifest, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if strings.ToLower(filepath.Base(hdr.Name)) != "package.json" {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		var m packageManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil
	}
	return nil, fmt.Errorf("package.json not found in %s", archivePath)
}

// resolveLocalArchive returns the local archive path for a package name and
// version reference, resolving floating versions against the local index. It
// returns ("", false) when no local archive matches.
func (c *PackageClient) resolveLocalArchive(name, versionRef string) (string, string, bool) {
	if len(c.LocalArchives) == 0 {
		return "", "", false
	}
	// Exact version match first.
	if versionRef != "" && !isFloatingVersion(versionRef) {
		if p, ok := c.LocalArchives[name+"@"+versionRef]; ok {
			return versionRef, p, true
		}
	}
	// Floating or unspecified version: pick the single matching archive, or
	// the newest when multiple match.
	var bestPath string
	var bestVersion string
	for key, p := range c.LocalArchives {
		if !strings.HasPrefix(key, name+"@") {
			continue
		}
		v := strings.TrimPrefix(key, name+"@")
		if versionRef != "" && !isFloatingVersion(versionRef) && v != versionRef {
			continue
		}
		if bestVersion == "" || versionNewer(v, bestVersion) {
			bestVersion = v
			bestPath = p
		}
	}
	if bestPath == "" {
		return "", "", false
	}
	return bestVersion, bestPath, true
}

// Cached reports whether a package is already extracted in the cache.
func (c *PackageClient) Cached(name, version string) bool {
	_, err := os.Stat(filepath.Join(c.PackageDir(name, version), "package", "package.json"))
	return err == nil
}

// metadata is the shape of the registry metadata response.
type metadata struct {
	DistTags map[string]string `json:"dist-tags"`
	Versions map[string]struct {
		Dist struct {
			Tarball string `json:"tarball"`
		} `json:"dist"`
	} `json:"versions"`
}

// isFloatingVersion reports whether a version reference is a floating tag
// ("current", "latest", or "*") rather than a concrete version.
func isFloatingVersion(version string) bool {
	if version == "" {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(version))
	return v == "current" || v == "latest" || v == "*"
}

// floatingVersionTag maps a floating version reference to the dist-tag to look
// up in registry metadata. "current" maps to "latest".
func floatingVersionTag(version string) string {
	v := strings.ToLower(strings.TrimSpace(version))
	if v == "current" {
		return "latest"
	}
	return v
}

// resolveVersionAndTarball resolves a version reference (exact, a patch
// wildcard like "4.0.x", or a floating tag like "current"/"latest") to an exact
// version and its tarball URL by querying the registry. It tries each registry
// URL in order until one returns usable metadata.
func (c *PackageClient) resolveVersionAndTarball(ctx context.Context, name, versionRef string) (string, string, error) {
	var errs []string
	for _, baseURL := range c.registryURLs {
		url := strings.TrimRight(baseURL, "/") + "/" + name
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", "", err
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", url, err))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			errs = append(errs, fmt.Sprintf("%s: status %d", url, resp.StatusCode))
			continue
		}
		var md metadata
		if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
			resp.Body.Close()
			errs = append(errs, fmt.Sprintf("%s: %v", url, err))
			continue
		}
		resp.Body.Close()

		version, tarball, err := resolveVersionFromMetadata(name, versionRef, &md)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", url, err))
			continue
		}
		return version, tarball, nil
	}
	return "", "", fmt.Errorf("%w: resolving %s %q: %s", ErrVersionNotFound, name, versionRef, strings.Join(errs, "; "))
}

// resolveVersionFromMetadata resolves a version reference against a single
// registry metadata document.
func resolveVersionFromMetadata(name, versionRef string, md *metadata) (string, string, error) {
	resolved := versionRef
	if isFloatingVersion(versionRef) {
		tag := floatingVersionTag(versionRef)
		if v, ok := md.DistTags[tag]; ok && v != "" {
			resolved = v
		} else if v, ok := md.DistTags["latest"]; ok && v != "" {
			resolved = v
		} else {
			return "", "", fmt.Errorf("floating version %q could not be resolved", versionRef)
		}
	} else if strings.Contains(versionRef, "x") {
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
			return "", "", fmt.Errorf("no version of %s matches %q", name, versionRef)
		}
		resolved = best
	}

	vm, ok := md.Versions[resolved]
	if !ok {
		return "", "", fmt.Errorf("version %q not found in registry metadata", resolved)
	}
	if vm.Dist.Tarball == "" {
		return "", "", fmt.Errorf("version %q missing dist.tarball", resolved)
	}
	return resolved, vm.Dist.Tarball, nil
}

// ResolveVersion resolves a version reference (exact, a patch wildcard like
// "4.0.x", or a floating tag like "current"/"latest") to an exact available
// version by querying the registry.
func (c *PackageClient) ResolveVersion(ctx context.Context, name, versionRef string) (string, error) {
	version, _, err := c.resolveVersionAndTarball(ctx, name, versionRef)
	return version, err
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

// Download resolves a version reference (exact, wildcard, or floating) to an
// exact version, fetches its tarball from the registry, and extracts it into
// the cache. It returns the resolved version and the cache directory. It is a
// no-op if the resolved package is already cached. When a matching local
// archive is indexed, it is used instead of the network.
func (c *PackageClient) Download(ctx context.Context, name, versionRef string) (resolvedVersion string, cacheDir string, err error) {
	// Apply the conflict policy: if a version was already selected for this
	// package, honour it.
	if c.selectedVersions == nil {
		c.selectedVersions = make(map[string]string)
	}
	if selected, ok := c.selectedVersions[name]; ok {
		if selected != versionRef && !isFloatingVersion(versionRef) {
			if c.ConflictPolicy == ConflictPolicyStrict {
				return "", "", fmt.Errorf("version conflict for package %s: %s vs %s", name, selected, versionRef)
			}
			// Root-wins: keep the selected version.
			versionRef = selected
		}
	}

	// Prefer a local archive when one matches.
	if localVersion, localPath, ok := c.resolveLocalArchive(name, versionRef); ok {
		c.selectedVersions[name] = localVersion
		dir := c.PackageDir(name, localVersion)
		if !c.Cached(name, localVersion) {
			if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
				return "", "", err
			}
			f, err := os.Open(localPath)
			if err != nil {
				return "", "", err
			}
			err = extractTGZToDir(f, dir)
			f.Close()
			if err != nil {
				return "", "", fmt.Errorf("extracting local archive %s: %w", localPath, err)
			}
		}
		return localVersion, dir, nil
	}

	resolvedVersion, tarballURL, err := c.resolveVersionAndTarball(ctx, name, versionRef)
	if err != nil {
		return "", "", err
	}
	c.selectedVersions[name] = resolvedVersion
	dir := c.PackageDir(name, resolvedVersion)
	if c.Cached(name, resolvedVersion) {
		return resolvedVersion, dir, nil
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tarballURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: downloading %s#%s: %v", ErrNetwork, name, resolvedVersion, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%w: downloading %s#%s: status %d", ErrNetwork, name, resolvedVersion, resp.StatusCode)
	}
	if err := extractTGZToDir(resp.Body, dir); err != nil {
		return "", "", fmt.Errorf("extracting %s#%s: %w", name, resolvedVersion, err)
	}
	return resolvedVersion, dir, nil
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
