package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/minicodemonkey/chief/internal/auth"
	"github.com/minicodemonkey/chief/internal/ws"
)

// ServeOptions contains configuration for the serve command.
type ServeOptions struct {
	Workspace  string          // Path to workspace directory
	DeviceName string          // Override device name (default: from credentials)
	LogFile    string          // Path to log file (default: stdout)
	BaseURL    string          // Override base URL (for testing)
	WSURL      string          // Override WebSocket URL (for testing/dev)
	Version    string          // Chief version string
	Ctx        context.Context // Optional context for cancellation (for testing)
}

// RunServe starts the headless serve daemon.
func RunServe(opts ServeOptions) error {
	// Validate workspace directory exists
	info, err := os.Stat(opts.Workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workspace directory does not exist: %s", opts.Workspace)
		}
		return fmt.Errorf("checking workspace directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace path is not a directory: %s", opts.Workspace)
	}

	// Set up logging
	if opts.LogFile != "" {
		f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	// Check for credentials
	creds, err := auth.LoadCredentials()
	if err != nil {
		if errors.Is(err, auth.ErrNotLoggedIn) {
			return fmt.Errorf("Not logged in. Run 'chief login' first.")
		}
		return fmt.Errorf("loading credentials: %w", err)
	}

	// Refresh token if near-expiry
	if creds.IsNearExpiry(5 * time.Minute) {
		log.Println("Access token near expiry, refreshing...")
		creds, err = auth.RefreshToken(opts.BaseURL)
		if err != nil {
			return fmt.Errorf("refreshing token: %w", err)
		}
		log.Println("Token refreshed successfully")
	}

	// Determine device name
	deviceName := opts.DeviceName
	if deviceName == "" {
		deviceName = creds.DeviceName
	}

	// Determine WebSocket URL
	wsURL := opts.WSURL
	if wsURL == "" {
		wsURL = ws.DefaultURL
	}

	log.Printf("Starting chief serve (workspace: %s, device: %s)", opts.Workspace, deviceName)
	log.Printf("Connecting to %s", wsURL)

	// Set up context with cancellation for clean shutdown
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create WebSocket client
	client := ws.New(wsURL, ws.WithOnReconnect(func() {
		log.Println("WebSocket reconnected, re-sending state snapshot")
	}))

	// Connect to WebSocket server
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connecting to WebSocket server: %w", err)
	}
	log.Println("WebSocket connected")

	// Perform protocol handshake
	version := opts.Version
	if version == "" {
		version = "dev"
	}

	if err := client.Handshake(creds.AccessToken, version, deviceName); err != nil {
		// Cancel context first to stop reconnection attempts before closing
		cancel()
		client.Close()
		var incompErr *ws.ErrIncompatible
		if errors.As(err, &incompErr) {
			return fmt.Errorf("incompatible version: %s", incompErr.Message)
		}
		if errors.Is(err, ws.ErrAuthFailed) {
			return fmt.Errorf("Device deauthorized. Run 'chief login' to re-authenticate.")
		}
		return fmt.Errorf("handshake failed: %w", err)
	}
	log.Println("Handshake complete")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	log.Println("Serve is running. Press Ctrl+C to stop.")

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down...")
			return serveShutdown(client)

		case sig := <-sigCh:
			log.Printf("Received signal %s, shutting down...", sig)
			return serveShutdown(client)

		case msg, ok := <-client.Receive():
			if !ok {
				// Channel closed, connection lost permanently
				log.Println("WebSocket connection closed permanently")
				return serveShutdown(client)
			}
			handleMessage(client, msg)
		}
	}
}

// handleMessage routes incoming WebSocket messages.
func handleMessage(client *ws.Client, msg ws.Message) {
	switch msg.Type {
	case ws.TypePing:
		pong := ws.NewMessage(ws.TypePong)
		if err := client.Send(pong); err != nil {
			log.Printf("Error sending pong: %v", err)
		}
	default:
		log.Printf("Received message type: %s", msg.Type)
	}
}

// serveShutdown performs clean shutdown of the serve command.
func serveShutdown(client *ws.Client) error {
	log.Println("Shutting down...")

	// Close WebSocket connection
	if err := client.Close(); err != nil {
		log.Printf("Error closing WebSocket: %v", err)
	}

	log.Println("Goodbye.")
	return nil
}
