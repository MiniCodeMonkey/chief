package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func schemasDir() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "contract", "schemas")); err == nil {
			return filepath.Join(dir, "contract", "schemas")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func newTestValidator(t *testing.T) *Validator {
	t.Helper()
	sd := schemasDir()
	if sd == "" {
		t.Skip("schemas directory not found")
	}
	v, err := NewValidator(sd)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func TestNewValidator(t *testing.T) {
	v := newTestValidator(t)
	if v.envelope == nil {
		t.Error("envelope schema not loaded")
	}
	if len(v.typeSchemas) == 0 {
		t.Error("no type-specific schemas loaded")
	}
}

func TestValidateValidFixtures(t *testing.T) {
	v := newTestValidator(t)
	fd := fixturesDir()
	if fd == "" {
		t.Skip("fixtures directory not found")
	}

	validFiles := []string{
		"envelope/valid_minimal.json",
		"envelope/valid_with_payload.json",
		"control/valid_welcome.json",
		"control/valid_ack.json",
		"control/valid_error.json",
		"state/valid_sync.json",
		"state/valid_prd_updated.json",
		"state/valid_run_completed.json",
		"cmd/valid_run_start.json",
		"cmd/valid_prd_create.json",
	}

	for _, f := range validFiles {
		t.Run(f, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fd, f))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if err := v.ValidateJSON(data); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateInvalidFixtures(t *testing.T) {
	v := newTestValidator(t)
	fd := fixturesDir()
	if fd == "" {
		t.Skip("fixtures directory not found")
	}

	invalidFiles := []string{
		"envelope/invalid_missing_type.json",
		"envelope/invalid_missing_id.json",
		"envelope/invalid_extra_field.json",
		"control/invalid_welcome_missing_payload.json",
		"control/invalid_ack_missing_ref.json",
		"control/invalid_error_missing_code.json",
		"state/invalid_sync_missing_projects.json",
		"state/invalid_run_completed_missing_result.json",
		"cmd/invalid_run_start_missing_prd_id.json",
		"cmd/invalid_prd_create_missing_title.json",
	}

	for _, f := range invalidFiles {
		t.Run(f, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fd, f))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if err := v.ValidateJSON(data); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestValidateEnvelopeStruct(t *testing.T) {
	v := newTestValidator(t)

	env := NewEnvelope(TypeSync, "device-123")
	env.Payload = json.RawMessage(`{"projects":[],"prds":[],"runs":[]}`)

	if err := v.Validate(env); err != nil {
		t.Errorf("expected valid envelope, got error: %v", err)
	}
}

func TestValidateEnvelopeStructInvalid(t *testing.T) {
	v := newTestValidator(t)

	// sync requires payload with projects, prds, runs
	env := NewEnvelope(TypeSync, "device-123")
	env.Payload = json.RawMessage(`{"prds":[],"runs":[]}`)

	if err := v.Validate(env); err == nil {
		t.Error("expected validation error for sync missing projects, got nil")
	}
}

func TestValidateDescriptiveErrors(t *testing.T) {
	v := newTestValidator(t)

	// Missing required fields should produce descriptive errors
	data := []byte(`{"device_id":"d1","timestamp":"2026-01-01T00:00:00Z"}`)
	err := v.ValidateJSON(data)
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if len(errStr) == 0 {
		t.Error("expected descriptive error message")
	}
}
