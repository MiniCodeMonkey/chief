package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minicodemonkey/chief/embed"
	"github.com/minicodemonkey/chief/internal/loop"
	"github.com/minicodemonkey/chief/internal/prd"
)

// runConversionWithProvider runs the agent to convert prd.md to JSON.
func runConversionWithProvider(provider loop.Provider, absPRDDir string) (string, error) {
	content, err := os.ReadFile(filepath.Join(absPRDDir, "prd.md"))
	if err != nil {
		return "", fmt.Errorf("failed to read prd.md: %w", err)
	}
	prompt := embed.GetConvertPrompt(string(content))
	cmd, mode, outPath, err := provider.ConvertCommand(absPRDDir, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to prepare conversion command: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if mode == loop.OutputStdout {
		cmd.Stdout = &stdout
	} else {
		cmd.Stdout = &bytes.Buffer{}
	}
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		if outPath != "" {
			_ = os.Remove(outPath)
		}
		return "", fmt.Errorf("failed to start %s: %w", provider.Name(), err)
	}
	if err := prd.WaitWithPanel(cmd, "Converting PRD", "Analyzing PRD...", &stderr); err != nil {
		if outPath != "" {
			os.Remove(outPath)
		}
		return "", err
	}
	if mode == loop.OutputFromFile && outPath != "" {
		defer os.Remove(outPath)
		data, err := os.ReadFile(outPath)
		if err != nil {
			return "", fmt.Errorf("failed to read conversion output: %w", err)
		}
		return string(data), nil
	}
	return stdout.String(), nil
}

// runFixJSONWithProvider runs the agent to fix invalid JSON.
func runFixJSONWithProvider(provider loop.Provider, prompt string) (string, error) {
	cmd, mode, outPath, err := provider.FixJSONCommand(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to prepare fix command: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if mode == loop.OutputStdout {
		cmd.Stdout = &stdout
	} else {
		cmd.Stdout = &bytes.Buffer{}
	}
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		if outPath != "" {
			_ = os.Remove(outPath)
		}
		return "", fmt.Errorf("failed to start %s: %w", provider.Name(), err)
	}
	if err := prd.WaitWithSpinner(cmd, "Fixing JSON", "Fixing prd.json...", &stderr); err != nil {
		if outPath != "" {
			os.Remove(outPath)
		}
		return "", err
	}
	if mode == loop.OutputFromFile && outPath != "" {
		defer os.Remove(outPath)
		data, err := os.ReadFile(outPath)
		if err != nil {
			return "", fmt.Errorf("failed to read fix output: %w", err)
		}
		return string(data), nil
	}
	return stdout.String(), nil
}
