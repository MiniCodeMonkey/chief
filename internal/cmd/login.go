package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
)

const (
	defaultBaseURL = "https://chiefloop.com"
	pollInterval   = 5 * time.Second
	loginTimeout   = 5 * time.Minute
)

// LoginOptions contains configuration for the login command.
type LoginOptions struct {
	DeviceName string // Override device name (default: hostname)
	BaseURL    string // Override base URL (for testing)
}

// deviceCodeResponse is the response from the device code endpoint.
type deviceCodeResponse struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
}

// tokenResponse is the response from the token polling endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         string `json:"user"`
	Error        string `json:"error"`
}

// RunLogin performs the device OAuth login flow.
func RunLogin(opts LoginOptions) error {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	deviceName := opts.DeviceName
	if deviceName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			deviceName = "unknown"
		} else {
			deviceName = hostname
		}
	}

	// Check if already logged in
	existing, err := auth.LoadCredentials()
	if err == nil && existing != nil {
		fmt.Printf("Already logged in as %s (%s).\n", existing.User, existing.DeviceName)
		fmt.Print("Do you want to log in again? This will replace your existing credentials. [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Login cancelled.")
			return nil
		}
	}

	// Request device code
	codeReqBody, _ := json.Marshal(map[string]string{
		"device_name": deviceName,
	})

	resp, err := http.Post(baseURL+"/oauth/device/code", "application/json", bytes.NewReader(codeReqBody))
	if err != nil {
		return fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("requesting device code: server returned %s", resp.Status)
	}

	var codeResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&codeResp); err != nil {
		return fmt.Errorf("parsing device code response: %w", err)
	}

	// Display the user code and URL
	deviceURL := baseURL + "/device"
	fmt.Println()
	fmt.Println("To authenticate, open this URL in your browser:")
	fmt.Printf("\n  %s\n\n", deviceURL)
	fmt.Printf("And enter this code: %s\n\n", codeResp.UserCode)

	// Try to open browser automatically
	openBrowser(deviceURL)

	fmt.Println("Waiting for authorization...")

	// Poll for token
	creds, err := pollForToken(baseURL, codeResp.DeviceCode, deviceName)
	if err != nil {
		return err
	}

	// Save credentials
	if err := auth.SaveCredentials(creds); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Printf("\nLogged in as %s (%s)\n", creds.User, creds.DeviceName)
	return nil
}

// pollForToken polls the token endpoint until authorization is granted or timeout.
func pollForToken(baseURL, deviceCode, deviceName string) (*auth.Credentials, error) {
	deadline := time.Now().Add(loginTimeout)
	client := &http.Client{Timeout: 10 * time.Second}

	for {
		if time.Now().After(deadline) {
			return nil, errors.New("login timed out — you did not authorize the device within 5 minutes")
		}

		time.Sleep(pollInterval)

		reqBody, _ := json.Marshal(map[string]string{
			"device_code": deviceCode,
		})

		resp, err := client.Post(baseURL+"/oauth/device/token", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Network error while polling (will retry): %v\n", err)
			continue
		}

		var tokenResp tokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "Error parsing token response (will retry): %v\n", err)
			continue
		}
		resp.Body.Close()

		// Check for pending authorization
		if tokenResp.Error == "authorization_pending" {
			continue
		}

		// Check for other errors
		if tokenResp.Error != "" {
			return nil, fmt.Errorf("authorization failed: %s", tokenResp.Error)
		}

		// Check for successful token response
		if resp.StatusCode == http.StatusOK && tokenResp.AccessToken != "" {
			return &auth.Credentials{
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.RefreshToken,
				ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
				DeviceName:   deviceName,
				User:         tokenResp.User,
			}, nil
		}

		// Non-200 status without a recognized error
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Unexpected status %s (will retry)\n", resp.Status)
			continue
		}
	}
}

// openBrowser attempts to open the given URL in the default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	// Ignore errors — browser open is best-effort
	cmd.Start()
}
