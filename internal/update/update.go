package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultReleasesURL = "https://api.github.com/repos/MiniCodeMonkey/chief/releases/latest"
	downloadTimeout    = 5 * time.Minute
	checkTimeout       = 10 * time.Second
)

// ErrRateLimited is returned when the GitHub API rate limit is exceeded.
var ErrRateLimited = fmt.Errorf("GitHub API rate limit exceeded, try again later")

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckResult contains the result of a version check.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	UpdateAvailable bool
}

// Options configures the update checker.
type Options struct {
	ReleasesURL string // Override GitHub API URL (for testing)
}

func (o Options) releasesURL() string {
	if o.ReleasesURL != "" {
		return o.ReleasesURL
	}
	return defaultReleasesURL
}

// CheckForUpdate checks if a newer version is available.
func CheckForUpdate(currentVersion string, opts Options) (*CheckResult, error) {
	current := normalizeVersion(currentVersion)

	client := &http.Client{Timeout: checkTimeout}
	resp, err := client.Get(opts.releasesURL())
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	// Rate limited — silently assume current version is up to date
	if resp.StatusCode == http.StatusForbidden {
		return &CheckResult{
			CurrentVersion:  current,
			LatestVersion:   current,
			UpdateAvailable: false,
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release response: %w", err)
	}

	latest := normalizeVersion(release.TagName)

	return &CheckResult{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: baseVersion(current) != latest && current != "dev",
	}, nil
}

// PerformUpdate downloads and installs the latest version.
func PerformUpdate(currentVersion string, opts Options) (*CheckResult, error) {
	client := &http.Client{Timeout: checkTimeout}
	resp, err := client.Get(opts.releasesURL())
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release response: %w", err)
	}

	latest := normalizeVersion(release.TagName)
	current := normalizeVersion(currentVersion)

	if baseVersion(current) == latest {
		return &CheckResult{
			CurrentVersion:  current,
			LatestVersion:   latest,
			UpdateAvailable: false,
		}, nil
	}

	// Find the binary asset for this OS/arch
	binaryAsset, checksumAsset := findAssets(release.Assets, runtime.GOOS, runtime.GOARCH)
	if binaryAsset == nil {
		return nil, fmt.Errorf("no binary available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Get the current binary path
	binaryPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("finding current binary path: %w", err)
	}
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("resolving binary path: %w", err)
	}

	// Check write permissions
	dir := filepath.Dir(binaryPath)
	if err := checkWritePermission(dir); err != nil {
		return nil, fmt.Errorf("Permission denied. Run 'sudo chief update' to upgrade.")
	}

	// Download binary to temp file
	tmpFile, err := downloadToTemp(binaryAsset.BrowserDownloadURL, dir)
	if err != nil {
		return nil, fmt.Errorf("downloading update: %w", err)
	}
	defer os.Remove(tmpFile) // Clean up on failure

	// Verify checksum if available
	if checksumAsset != nil {
		if err := verifyChecksum(tmpFile, checksumAsset.BrowserDownloadURL); err != nil {
			return nil, fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Make the new binary executable
	if err := os.Chmod(tmpFile, 0o755); err != nil {
		return nil, fmt.Errorf("setting permissions on new binary: %w", err)
	}

	// Atomic rename to replace current binary
	if err := os.Rename(tmpFile, binaryPath); err != nil {
		return nil, fmt.Errorf("replacing binary: %w", err)
	}

	return &CheckResult{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: true,
	}, nil
}

// normalizeVersion strips the "v" prefix from version strings.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// baseVersion extracts the base semver from a git-describe version string.
// For example, "0.4.0-61-gd06835b" returns "0.4.0".
// Handles dirty builds too: "0.4.0-61-gd06835b-dirty" returns "0.4.0".
// A plain version like "0.4.0" is returned unchanged.
func baseVersion(v string) string {
	v = normalizeVersion(v)
	// Strip "-dirty" suffix from uncommitted builds.
	v = strings.TrimSuffix(v, "-dirty")
	// Git describe format: "0.4.0-61-gd06835b" (tag-commits-ghash)
	// Strip the "-N-gHASH" suffix to get the base semver.
	parts := strings.Split(v, "-")
	if len(parts) >= 3 {
		last := parts[len(parts)-1]
		if strings.HasPrefix(last, "g") {
			return strings.Join(parts[:len(parts)-2], "-")
		}
	}
	return v
}

// CompareVersions returns true if latest is different from the base version of current.
// Dev builds (e.g. "0.4.0-61-gd06835b") are compared by their base tag ("0.4.0").
func CompareVersions(current, latest string) bool {
	current = normalizeVersion(current)
	latest = normalizeVersion(latest)
	return baseVersion(current) != latest && current != "dev"
}

// findAssets locates the binary and checksum assets for the given OS/arch.
func findAssets(assets []Asset, goos, goarch string) (*Asset, *Asset) {
	binaryName := fmt.Sprintf("chief-%s-%s", goos, goarch)
	checksumName := binaryName + ".sha256"

	var binary, checksum *Asset
	for i := range assets {
		if assets[i].Name == binaryName {
			binary = &assets[i]
		}
		if assets[i].Name == checksumName {
			checksum = &assets[i]
		}
	}
	return binary, checksum
}

// checkWritePermission checks if we can write to the directory.
func checkWritePermission(dir string) error {
	tmp, err := os.CreateTemp(dir, ".chief-update-check-*")
	if err != nil {
		return err
	}
	tmp.Close()
	os.Remove(tmp.Name())
	return nil
}

// downloadToTemp downloads a URL to a temporary file in the specified directory.
func downloadToTemp(url, dir string) (string, error) {
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(dir, ".chief-update-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("writing download: %w", err)
	}
	tmp.Close()

	return tmp.Name(), nil
}

// verifyChecksum downloads the expected SHA256 checksum and verifies the file.
func verifyChecksum(filePath, checksumURL string) error {
	// Download checksum file
	client := &http.Client{Timeout: checkTimeout}
	resp, err := client.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("downloading checksum: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum download returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading checksum: %w", err)
	}

	// Parse checksum (format: "hash  filename" or just "hash")
	expectedHash := strings.Fields(strings.TrimSpace(string(body)))[0]

	// Calculate actual hash
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}
