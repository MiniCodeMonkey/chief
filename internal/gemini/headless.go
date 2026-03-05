package gemini

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type HeadlessResult struct {
	Response string          `json:"response"`
	Stats    json.RawMessage `json:"stats,omitempty"`
	Error    json.RawMessage `json:"error,omitempty"`
}

// EnsureAuth is a no-op kept for backward compatibility. Authentication is
// handled by the Gemini CLI itself (API key env vars, Vertex AI env vars,
// or token stored by `gemini login`). Melliza should not gate on env vars
// because that rejects CLI-authenticated sessions.
func EnsureAuth() error {
	return nil
}

func BuildHeadlessArgs(prompt, model string, yolo bool) []string {
	args := []string{"--output-format", "json", "-e", "none"}
	if yolo {
		args = append(args, "-y")
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "-p", prompt)
	return args
}

// BuildStreamArgs builds args for streaming mode (stream-json output).
// This allows real-time parsing of Gemini's output as it works.
func BuildStreamArgs(prompt, model string, yolo bool) []string {
	args := []string{"--output-format", "stream-json", "-e", "none"}
	if yolo {
		args = append(args, "-y")
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "-p", prompt)
	return args
}

func ParseSingleJSONObject(stdout []byte) (*HeadlessResult, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, errors.New("gemini produced empty stdout")
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var result HeadlessResult
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid JSON output from gemini: %w", err)
	}
	if tail := bytes.TrimSpace(trimmed[dec.InputOffset():]); len(tail) > 0 {
		return nil, errors.New("gemini output contained trailing content")
	}
	if result.Response == "" && len(result.Error) == 0 {
		return nil, errors.New("gemini JSON missing both response and error fields")
	}
	return &result, nil
}

func Command(prompt, model string) *exec.Cmd {
	return exec.Command("gemini", BuildHeadlessArgs(prompt, model, false)...)
}
