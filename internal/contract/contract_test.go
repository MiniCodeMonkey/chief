package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/minicodemonkey/chief/internal/uplink"
	"github.com/minicodemonkey/chief/internal/ws"
)

// fixturesDir returns the absolute path to contract/fixtures relative to the repo root.
func fixturesDir(t *testing.T) string {
	t.Helper()
	// Determine repo root from this test file's location:
	// internal/contract/contract_test.go → ../../contract/fixtures
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "contract", "fixtures")
}

func loadFixture(t *testing.T, relPath string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir(t), relPath))
	if err != nil {
		t.Fatalf("loading fixture %s: %v", relPath, err)
	}
	return data
}

// --- server-to-cli fixtures ---

func TestWelcomeResponse_Deserialize(t *testing.T) {
	data := loadFixture(t, "server-to-cli/welcome_response.json")

	var welcome uplink.WelcomeResponse
	if err := json.Unmarshal(data, &welcome); err != nil {
		t.Fatalf("failed to unmarshal welcome_response.json: %v", err)
	}

	if welcome.Type != "welcome" {
		t.Errorf("type = %q, want %q", welcome.Type, "welcome")
	}
	if welcome.ProtocolVersion != 1 {
		t.Errorf("protocol_version = %d, want 1", welcome.ProtocolVersion)
	}
	if welcome.DeviceID != 42 {
		t.Errorf("device_id = %d, want 42", welcome.DeviceID)
	}
	if welcome.SessionID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("session_id = %q, want UUID", welcome.SessionID)
	}

	// Reverb config — port MUST be an int, not a string
	if welcome.Reverb.Port != 8080 {
		t.Errorf("reverb.port = %d, want 8080", welcome.Reverb.Port)
	}
	if welcome.Reverb.Key != "test-app-key" {
		t.Errorf("reverb.key = %q, want %q", welcome.Reverb.Key, "test-app-key")
	}
	if welcome.Reverb.Host != "127.0.0.1" {
		t.Errorf("reverb.host = %q, want %q", welcome.Reverb.Host, "127.0.0.1")
	}
	if welcome.Reverb.Scheme != "https" {
		t.Errorf("reverb.scheme = %q, want %q", welcome.Reverb.Scheme, "https")
	}
}

func TestWelcomeResponse_PortIsInt(t *testing.T) {
	// Regression: PHP env() returns strings — verify port decodes as int.
	data := loadFixture(t, "server-to-cli/welcome_response.json")

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	var reverb map[string]json.RawMessage
	json.Unmarshal(raw["reverb"], &reverb)

	// Verify port is a JSON number, not a string
	portStr := string(reverb["port"])
	if portStr == `"8080"` {
		t.Fatal("reverb.port is a JSON string — must be a number")
	}
	if portStr != "8080" {
		t.Errorf("reverb.port raw JSON = %s, want 8080", portStr)
	}
}

func TestCommandCreateProject_PayloadWrapper(t *testing.T) {
	data := loadFixture(t, "server-to-cli/command_create_project.json")

	// Verify the envelope has type + payload
	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("failed to unmarshal command envelope: %v", err)
	}

	if env.Type != "create_project" {
		t.Errorf("envelope type = %q, want %q", env.Type, "create_project")
	}
	if len(env.Payload) == 0 {
		t.Fatal("envelope payload is empty — commands must have payload wrapper")
	}

	// The payload itself should parse into CreateProjectMessage fields
	var req ws.CreateProjectMessage
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		t.Fatalf("failed to unmarshal payload into CreateProjectMessage: %v", err)
	}

	if req.Name != "new-project" {
		t.Errorf("payload.name = %q, want %q", req.Name, "new-project")
	}
	if !req.GitInit {
		t.Error("payload.git_init = false, want true")
	}
}

func TestCommandStartRun_PayloadWrapper(t *testing.T) {
	data := loadFixture(t, "server-to-cli/command_start_run.json")

	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("failed to unmarshal command envelope: %v", err)
	}

	if env.Type != "start_run" {
		t.Errorf("envelope type = %q, want %q", env.Type, "start_run")
	}

	var req ws.StartRunMessage
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		t.Fatalf("failed to unmarshal payload into StartRunMessage: %v", err)
	}

	if req.Project != "my-project" {
		t.Errorf("payload.project = %q, want %q", req.Project, "my-project")
	}
	if req.PRDID != "feature-auth" {
		t.Errorf("payload.prd_id = %q, want %q", req.PRDID, "feature-auth")
	}
}

func TestCommandListProjects_PayloadWrapper(t *testing.T) {
	data := loadFixture(t, "server-to-cli/command_list_projects.json")

	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("failed to unmarshal command envelope: %v", err)
	}

	if env.Type != "list_projects" {
		t.Errorf("envelope type = %q, want %q", env.Type, "list_projects")
	}
}

// --- cli-to-server fixtures ---

func TestStateSnapshot_Roundtrip(t *testing.T) {
	data := loadFixture(t, "cli-to-server/state_snapshot.json")

	// Unmarshal into the Go struct
	var snapshot ws.StateSnapshotMessage
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("failed to unmarshal state_snapshot.json: %v", err)
	}

	if snapshot.Type != "state_snapshot" {
		t.Errorf("type = %q, want %q", snapshot.Type, "state_snapshot")
	}
	if len(snapshot.Projects) != 1 {
		t.Fatalf("projects count = %d, want 1", len(snapshot.Projects))
	}

	// Verify project uses "name" field, not "project_slug"
	proj := snapshot.Projects[0]
	if proj.Name != "my-project" {
		t.Errorf("project.name = %q, want %q", proj.Name, "my-project")
	}
	if proj.Path != "/home/user/projects/my-project" {
		t.Errorf("project.path = %q", proj.Path)
	}
	if !proj.HasChief {
		t.Error("project.has_chief = false, want true")
	}
	if proj.Branch != "main" {
		t.Errorf("project.branch = %q, want %q", proj.Branch, "main")
	}
	if proj.Commit.Hash != "abc1234" {
		t.Errorf("project.commit.hash = %q, want %q", proj.Commit.Hash, "abc1234")
	}

	// Re-marshal and verify it round-trips cleanly
	remarshaled, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("failed to re-marshal: %v", err)
	}

	var roundtrip ws.StateSnapshotMessage
	if err := json.Unmarshal(remarshaled, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}
	if roundtrip.Projects[0].Name != "my-project" {
		t.Errorf("round-trip project.name = %q, want %q", roundtrip.Projects[0].Name, "my-project")
	}
}

func TestStateSnapshot_NameFieldNotProjectSlug(t *testing.T) {
	// Regression: CLI sends "name", not "project_slug".
	data := loadFixture(t, "cli-to-server/state_snapshot.json")

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	var projects []map[string]json.RawMessage
	json.Unmarshal(raw["projects"], &projects)

	if len(projects) == 0 {
		t.Fatal("no projects in fixture")
	}

	proj := projects[0]
	if _, hasName := proj["name"]; !hasName {
		t.Error("project should have 'name' field")
	}
	if _, hasSlug := proj["project_slug"]; hasSlug {
		t.Error("project should NOT have 'project_slug' field — CLI uses 'name'")
	}
}

func TestConnectRequest_Deserialize(t *testing.T) {
	data := loadFixture(t, "cli-to-server/connect_request.json")

	var req struct {
		ChiefVersion    string `json:"chief_version"`
		DeviceName      string `json:"device_name"`
		OS              string `json:"os"`
		Arch            string `json:"arch"`
		ProtocolVersion int    `json:"protocol_version"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal connect_request.json: %v", err)
	}

	if req.ChiefVersion != "1.0.0" {
		t.Errorf("chief_version = %q, want %q", req.ChiefVersion, "1.0.0")
	}
	if req.ProtocolVersion != 1 {
		t.Errorf("protocol_version = %d, want 1", req.ProtocolVersion)
	}
	if req.OS == "" {
		t.Error("os should not be empty")
	}
}

func TestMessagesBatch_Deserialize(t *testing.T) {
	data := loadFixture(t, "cli-to-server/messages_batch.json")

	var batch struct {
		BatchID  string            `json:"batch_id"`
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(data, &batch); err != nil {
		t.Fatalf("failed to unmarshal messages_batch.json: %v", err)
	}

	if batch.BatchID == "" {
		t.Error("batch_id should not be empty")
	}
	if len(batch.Messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(batch.Messages))
	}

	// First message should be a state_snapshot
	var msg ws.StateSnapshotMessage
	if err := json.Unmarshal(batch.Messages[0], &msg); err != nil {
		t.Fatalf("failed to unmarshal first message: %v", err)
	}
	if msg.Type != "state_snapshot" {
		t.Errorf("first message type = %q, want %q", msg.Type, "state_snapshot")
	}
}
