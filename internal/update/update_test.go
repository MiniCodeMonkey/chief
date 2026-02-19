package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCheckForUpdate_UpdateAvailable(t *testing.T) {
	release := Release{
		TagName: "v0.5.1",
		Assets:  []Asset{{Name: "chief-linux-amd64", BrowserDownloadURL: "http://example.com/chief"}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result, err := CheckForUpdate("0.5.0", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.UpdateAvailable {
		t.Error("expected update to be available")
	}
	if result.LatestVersion != "0.5.1" {
		t.Errorf("expected latest version 0.5.1, got %s", result.LatestVersion)
	}
	if result.CurrentVersion != "0.5.0" {
		t.Errorf("expected current version 0.5.0, got %s", result.CurrentVersion)
	}
}

func TestCheckForUpdate_AlreadyLatest(t *testing.T) {
	release := Release{TagName: "v0.5.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result, err := CheckForUpdate("v0.5.0", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("expected no update available")
	}
}

func TestCheckForUpdate_DevVersion(t *testing.T) {
	release := Release{TagName: "v1.0.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result, err := CheckForUpdate("dev", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("dev version should not report update available")
	}
}

func TestCheckForUpdate_DevBuildSameTag(t *testing.T) {
	release := Release{TagName: "v0.4.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result, err := CheckForUpdate("v0.4.0-61-gd06835b", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("dev build ahead of the same tag should not report update available")
	}
}

func TestCheckForUpdate_DevBuildOlderTag(t *testing.T) {
	release := Release{TagName: "v0.5.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result, err := CheckForUpdate("v0.4.0-61-gd06835b", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.UpdateAvailable {
		t.Error("dev build should report update available when a newer release exists")
	}
}

func TestCheckForUpdate_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	result, err := CheckForUpdate("0.5.0", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("expected no error for rate-limited response, got: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("expected no update when rate limited")
	}
	if result.CurrentVersion != "0.5.0" {
		t.Errorf("expected current version 0.5.0, got %s", result.CurrentVersion)
	}
}

func TestCheckForUpdate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := CheckForUpdate("0.5.0", Options{ReleasesURL: srv.URL})
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestCheckForUpdate_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := CheckForUpdate("0.5.0", Options{ReleasesURL: srv.URL})
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"v0.5.0", "0.5.0"},
		{"0.5.0", "0.5.0"},
		{"v1.0.0-beta", "1.0.0-beta"},
		{"dev", "dev"},
	}

	for _, tc := range tests {
		result := normalizeVersion(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestBaseVersion(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"v0.4.0-61-gd06835b", "0.4.0"},
		{"0.4.0-61-gd06835b", "0.4.0"},
		{"v0.5.0", "0.5.0"},
		{"0.5.0", "0.5.0"},
		{"1.0.0-beta", "1.0.0-beta"},
		{"1.0.0-beta-3-gabcdef1", "1.0.0-beta"},
		{"dev", "dev"},
		{"v0.4.0-dirty", "0.4.0"},
		{"0.4.0-81-g1d1ebf3-dirty", "0.4.0"},
	}

	for _, tc := range tests {
		result := baseVersion(tc.input)
		if result != tc.expected {
			t.Errorf("baseVersion(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current, latest string
		expected        bool
	}{
		{"0.5.0", "0.5.1", true},
		{"0.5.0", "0.5.0", false},
		{"v0.5.0", "v0.5.0", false},
		{"dev", "1.0.0", false},
		{"1.0.0", "2.0.0", true},
		// Dev builds ahead of a tag should not report update available
		{"0.4.0-61-gd06835b", "0.4.0", false},
		{"v0.4.0-61-gd06835b", "v0.4.0", false},
		// Dev builds should report update when a newer release exists
		{"0.4.0-61-gd06835b", "0.5.0", true},
	}

	for _, tc := range tests {
		result := CompareVersions(tc.current, tc.latest)
		if result != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %v, want %v", tc.current, tc.latest, result, tc.expected)
		}
	}
}

func TestFindAssets(t *testing.T) {
	assets := []Asset{
		{Name: "chief-linux-amd64", BrowserDownloadURL: "http://example.com/chief-linux-amd64"},
		{Name: "chief-linux-amd64.sha256", BrowserDownloadURL: "http://example.com/chief-linux-amd64.sha256"},
		{Name: "chief-darwin-arm64", BrowserDownloadURL: "http://example.com/chief-darwin-arm64"},
	}

	binary, checksum := findAssets(assets, "linux", "amd64")
	if binary == nil {
		t.Fatal("expected to find binary asset")
	}
	if binary.Name != "chief-linux-amd64" {
		t.Errorf("expected chief-linux-amd64, got %s", binary.Name)
	}
	if checksum == nil {
		t.Fatal("expected to find checksum asset")
	}
	if checksum.Name != "chief-linux-amd64.sha256" {
		t.Errorf("expected chief-linux-amd64.sha256, got %s", checksum.Name)
	}
}

func TestFindAssets_NoMatch(t *testing.T) {
	assets := []Asset{
		{Name: "chief-linux-amd64", BrowserDownloadURL: "http://example.com/chief-linux-amd64"},
	}

	binary, _ := findAssets(assets, "windows", "amd64")
	if binary != nil {
		t.Error("expected no binary for windows/amd64")
	}
}

func TestFindAssets_NoChecksum(t *testing.T) {
	assets := []Asset{
		{Name: "chief-linux-amd64", BrowserDownloadURL: "http://example.com/chief-linux-amd64"},
	}

	binary, checksum := findAssets(assets, "linux", "amd64")
	if binary == nil {
		t.Fatal("expected to find binary")
	}
	if checksum != nil {
		t.Error("expected no checksum")
	}
}

func TestCheckWritePermission_Success(t *testing.T) {
	dir := t.TempDir()
	if err := checkWritePermission(dir); err != nil {
		t.Errorf("expected write permission check to pass: %v", err)
	}
}

func TestCheckWritePermission_Fail(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755) // restore for cleanup

	if err := checkWritePermission(dir); err == nil {
		t.Error("expected write permission check to fail")
	}
}

func TestDownloadToTemp(t *testing.T) {
	content := "binary content here"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer srv.Close()

	dir := t.TempDir()
	tmpFile, err := downloadToTemp(srv.URL, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpFile)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("reading temp file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}

	// Verify temp file is in the right directory
	if filepath.Dir(tmpFile) != dir {
		t.Errorf("temp file should be in %s, got %s", dir, filepath.Dir(tmpFile))
	}
}

func TestDownloadToTemp_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	_, err := downloadToTemp(srv.URL, dir)
	if err == nil {
		t.Error("expected error for server error")
	}
}

func TestVerifyChecksum(t *testing.T) {
	// Create a temp file with known content
	dir := t.TempDir()
	filePath := filepath.Join(dir, "binary")
	content := []byte("test binary content")
	os.WriteFile(filePath, content, 0o644)

	// Calculate expected hash
	h := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(h[:])

	// Serve checksum
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  binary\n", expectedHash)
	}))
	defer srv.Close()

	if err := verifyChecksum(filePath, srv.URL); err != nil {
		t.Errorf("expected checksum verification to pass: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "binary")
	os.WriteFile(filePath, []byte("content"), 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "0000000000000000000000000000000000000000000000000000000000000000  binary\n")
	}))
	defer srv.Close()

	if err := verifyChecksum(filePath, srv.URL); err == nil {
		t.Error("expected checksum verification to fail")
	}
}

func TestPerformUpdate_AlreadyLatest(t *testing.T) {
	release := Release{TagName: "v0.5.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result, err := PerformUpdate("0.5.0", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("expected no update available when already on latest")
	}
}

func TestPerformUpdate_FullFlow(t *testing.T) {
	// Create a fake "current binary"
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "chief")
	os.WriteFile(binaryPath, []byte("old binary"), 0o755)

	// New binary content
	newContent := []byte("new binary v0.6.0")
	h := sha256.Sum256(newContent)
	expectedHash := hex.EncodeToString(h[:])

	binaryName := fmt.Sprintf("chief-%s-%s", runtime.GOOS, runtime.GOARCH)

	// Set up download server
	downloadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/binary" {
			w.Write(newContent)
		} else if r.URL.Path == "/checksum" {
			fmt.Fprintf(w, "%s  %s\n", expectedHash, binaryName)
		}
	}))
	defer downloadSrv.Close()

	release := Release{
		TagName: "v0.6.0",
		Assets: []Asset{
			{Name: binaryName, BrowserDownloadURL: downloadSrv.URL + "/binary"},
			{Name: binaryName + ".sha256", BrowserDownloadURL: downloadSrv.URL + "/checksum"},
		},
	}
	releaseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer releaseSrv.Close()

	// We can't easily test PerformUpdate because it calls os.Executable()
	// Instead, test the components used by PerformUpdate individually
	// (CheckForUpdate, findAssets, downloadToTemp, verifyChecksum are all tested above)

	// Test the download + checksum flow manually
	tmpFile, err := downloadToTemp(downloadSrv.URL+"/binary", dir)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer os.Remove(tmpFile)

	if err := verifyChecksum(tmpFile, downloadSrv.URL+"/checksum"); err != nil {
		t.Fatalf("checksum failed: %v", err)
	}

	// Verify file content
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != string(newContent) {
		t.Errorf("downloaded content mismatch")
	}
}

func TestCheckForUpdate_VersionWithVPrefix(t *testing.T) {
	release := Release{TagName: "v0.5.1"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	// Passing version with v prefix should still work
	result, err := CheckForUpdate("v0.5.0", Options{ReleasesURL: srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.UpdateAvailable {
		t.Error("expected update to be available")
	}
	if result.CurrentVersion != "0.5.0" {
		t.Errorf("expected normalized version 0.5.0, got %s", result.CurrentVersion)
	}
}
