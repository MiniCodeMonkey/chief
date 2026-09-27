package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/auth/device/request" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["device_name"] != "my-laptop" {
			t.Errorf("expected device_name my-laptop, got %s", body["device_name"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode: "dev-code-123",
			UserCode:   "ABCD-1234",
			VerifyURL:  "https://uplink.example.com/verify",
		})
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	resp, err := flow.RequestCode("my-laptop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeviceCode != "dev-code-123" {
		t.Errorf("expected dev-code-123, got %s", resp.DeviceCode)
	}
	if resp.UserCode != "ABCD-1234" {
		t.Errorf("expected ABCD-1234, got %s", resp.UserCode)
	}
	if resp.VerifyURL != "https://uplink.example.com/verify" {
		t.Errorf("expected verify URL, got %s", resp.VerifyURL)
	}
}

func TestRequestCodeServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	_, err := flow.RequestCode("my-laptop")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestPollForTokenSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/auth/device/verify" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["device_code"] != "dev-code-123" {
			t.Errorf("expected device_code dev-code-123, got %s", body["device_code"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "access-tok",
			RefreshToken: "refresh-tok",
			ExpiresIn:    3600,
			DeviceID:     42,
		})
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	resp, err := flow.PollForToken("dev-code-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != PollSuccess {
		t.Errorf("expected PollSuccess, got %d", resp.Result)
	}
	if resp.Token.AccessToken != "access-tok" {
		t.Errorf("expected access-tok, got %s", resp.Token.AccessToken)
	}
	if resp.Token.RefreshToken != "refresh-tok" {
		t.Errorf("expected refresh-tok, got %s", resp.Token.RefreshToken)
	}
	if resp.Token.ExpiresIn != 3600 {
		t.Errorf("expected 3600, got %d", resp.Token.ExpiresIn)
	}
	if resp.Token.DeviceID != 42 {
		t.Errorf("expected 42, got %d", resp.Token.DeviceID)
	}
}

func TestPollForTokenPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	resp, err := flow.PollForToken("dev-code-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != PollPending {
		t.Errorf("expected PollPending, got %d", resp.Result)
	}
	if resp.Token != nil {
		t.Error("expected nil token for pending response")
	}
}

func TestPollForTokenDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	resp, err := flow.PollForToken("dev-code-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != PollDenied {
		t.Errorf("expected PollDenied, got %d", resp.Result)
	}
}

func TestPollForTokenUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	_, err := flow.PollForToken("dev-code-123")
	if err == nil {
		t.Fatal("expected error for unexpected status")
	}
}

func TestRefreshAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/auth/token/refresh" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["refresh_token"] != "refresh-tok" {
			t.Errorf("expected refresh_token refresh-tok, got %s", body["refresh_token"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "new-access-tok",
			RefreshToken: "new-refresh-tok",
			ExpiresIn:    7200,
		})
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	token, err := flow.RefreshAccessToken("refresh-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "new-access-tok" {
		t.Errorf("expected new-access-tok, got %s", token.AccessToken)
	}
	if token.RefreshToken != "new-refresh-tok" {
		t.Errorf("expected new-refresh-tok, got %s", token.RefreshToken)
	}
	if token.ExpiresIn != 7200 {
		t.Errorf("expected 7200, got %d", token.ExpiresIn)
	}
}

func TestRefreshAccessTokenFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	_, err := flow.RefreshAccessToken("bad-token")
	if err == nil {
		t.Fatal("expected error for failed refresh")
	}
}

func TestRevokeDevice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/auth/device/device-42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-tok" {
			t.Errorf("expected Bearer access-tok, got %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	err := flow.RevokeDevice("device-42", "access-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevokeDeviceFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	flow := NewDeviceFlow(server.URL)
	err := flow.RevokeDevice("unknown-device", "access-tok")
	if err == nil {
		t.Fatal("expected error for failed revoke")
	}
}
