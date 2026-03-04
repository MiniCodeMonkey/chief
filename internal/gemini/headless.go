package gemini

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type HeadlessResult struct {
	Response string          `json:"response"`
	Stats    json.RawMessage `json:"stats,omitempty"`
	Error    json.RawMessage `json:"error,omitempty"`
}

func EnsureAuth() error {
	if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" {
		return nil
	}
	if os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") != "" || os.Getenv("GOOGLE_CLOUD_PROJECT") != "" || os.Getenv("GOOGLE_VERTEX_PROJECT") != "" {
		return nil
	}
	return errors.New("Gemini authentication is not configured: set GEMINI_API_KEY (or Vertex AI auth environment variables) before running Melliza")
}

func BuildHeadlessArgs(prompt, model string, yolo bool) []string {
	args := []string{"--output-format", "json", "-e", "none"}
	if yolo {
		args = append(args, "-y")
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--", prompt)
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
