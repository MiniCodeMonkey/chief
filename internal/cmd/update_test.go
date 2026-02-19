package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/minicodemonkey/chief/internal/update"
)

func TestRunUpdate_AlreadyLatest(t *testing.T) {
	release := update.Release{TagName: "v0.5.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	err := RunUpdate(UpdateOptions{
		Version:     "0.5.0",
		ReleasesURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := RunUpdate(UpdateOptions{
		Version:     "0.5.0",
		ReleasesURL: srv.URL,
	})
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestCheckVersionForServe_UpdateAvailable(t *testing.T) {
	release := update.Release{TagName: "v0.6.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result := CheckVersionForServe("0.5.0", srv.URL)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.UpdateAvailable {
		t.Error("expected update to be available")
	}
	if result.LatestVersion != "0.6.0" {
		t.Errorf("expected latest version 0.6.0, got %s", result.LatestVersion)
	}
}

func TestCheckVersionForServe_NoUpdate(t *testing.T) {
	release := update.Release{TagName: "v0.5.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result := CheckVersionForServe("0.5.0", srv.URL)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.UpdateAvailable {
		t.Error("expected no update available")
	}
}

func TestCheckVersionForServe_APIFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	result := CheckVersionForServe("0.5.0", srv.URL)
	if result != nil {
		t.Error("expected nil result on API failure")
	}
}

func TestCheckVersionForServe_DevVersion(t *testing.T) {
	release := update.Release{TagName: "v1.0.0"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	result := CheckVersionForServe("dev", srv.URL)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.UpdateAvailable {
		t.Error("dev version should not report update available")
	}
}
