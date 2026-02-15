package ws

import (
	"encoding/json"
	"testing"
)

func TestStateSnapshotRoundTrip(t *testing.T) {
	msg := StateSnapshotMessage{
		Type:      TypeStateSnapshot,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Projects: []ProjectSummary{
			{
				Name:     "my-project",
				Path:     "/home/user/projects/my-project",
				HasChief: true,
				Branch:   "main",
				Commit: CommitInfo{
					Hash:      "abc123",
					Message:   "initial commit",
					Author:    "dev",
					Timestamp: "2026-02-15T09:00:00Z",
				},
				PRDs: []PRDInfo{
					{ID: "auth", Name: "Authentication", StoryCount: 5, CompletionStatus: "3/5"},
				},
			},
		},
		Runs:     []RunState{{Project: "my-project", PRDID: "auth", StoryID: "US-003", Status: "running", Iteration: 2}},
		Sessions: []SessionState{{SessionID: "sess-1", Project: "my-project", PRDID: "auth"}},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got StateSnapshotMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != TypeStateSnapshot {
		t.Errorf("type = %q, want %q", got.Type, TypeStateSnapshot)
	}
	if len(got.Projects) != 1 {
		t.Fatalf("projects count = %d, want 1", len(got.Projects))
	}
	if got.Projects[0].Name != "my-project" {
		t.Errorf("project name = %q, want %q", got.Projects[0].Name, "my-project")
	}
	if got.Projects[0].Commit.Hash != "abc123" {
		t.Errorf("commit hash = %q, want %q", got.Projects[0].Commit.Hash, "abc123")
	}
	if len(got.Projects[0].PRDs) != 1 || got.Projects[0].PRDs[0].StoryCount != 5 {
		t.Errorf("unexpected PRD info: %+v", got.Projects[0].PRDs)
	}
	if len(got.Runs) != 1 || got.Runs[0].Iteration != 2 {
		t.Errorf("unexpected run state: %+v", got.Runs)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].SessionID != "sess-1" {
		t.Errorf("unexpected session state: %+v", got.Sessions)
	}
}

func TestClaudeOutputRoundTrip(t *testing.T) {
	msg := ClaudeOutputMessage{
		Type:      TypeClaudeOutput,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		SessionID: "session-123",
		Project:   "my-project",
		Data:      "Hello from Claude!\nLine 2.",
		Done:      false,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ClaudeOutputMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != TypeClaudeOutput {
		t.Errorf("type = %q, want %q", got.Type, TypeClaudeOutput)
	}
	if got.SessionID != "session-123" {
		t.Errorf("session_id = %q, want %q", got.SessionID, "session-123")
	}
	if got.Data != "Hello from Claude!\nLine 2." {
		t.Errorf("data = %q, want %q", got.Data, "Hello from Claude!\nLine 2.")
	}
	if got.Done {
		t.Error("done should be false")
	}
}

func TestRunProgressRoundTrip(t *testing.T) {
	msg := RunProgressMessage{
		Type:      TypeRunProgress,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
		StoryID:   "US-003",
		Status:    "running",
		Iteration: 3,
		Attempt:   1,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunProgressMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Project != "my-project" {
		t.Errorf("project = %q, want %q", got.Project, "my-project")
	}
	if got.StoryID != "US-003" {
		t.Errorf("story_id = %q, want %q", got.StoryID, "US-003")
	}
	if got.Iteration != 3 {
		t.Errorf("iteration = %d, want 3", got.Iteration)
	}
	if got.Attempt != 1 {
		t.Errorf("attempt = %d, want 1", got.Attempt)
	}
}

func TestRunCompleteRoundTrip(t *testing.T) {
	msg := RunCompleteMessage{
		Type:             TypeRunComplete,
		ID:               newUUID(),
		Timestamp:        "2026-02-15T10:00:00Z",
		Project:          "my-project",
		PRDID:            "auth",
		StoriesCompleted: 5,
		Duration:         "12m34s",
		PassCount:        4,
		FailCount:        1,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunCompleteMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.StoriesCompleted != 5 {
		t.Errorf("stories_completed = %d, want 5", got.StoriesCompleted)
	}
	if got.Duration != "12m34s" {
		t.Errorf("duration = %q, want %q", got.Duration, "12m34s")
	}
	if got.PassCount != 4 || got.FailCount != 1 {
		t.Errorf("pass/fail = %d/%d, want 4/1", got.PassCount, got.FailCount)
	}
}

func TestErrorMessageRoundTrip(t *testing.T) {
	msg := ErrorMessage{
		Type:      TypeError,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Code:      ErrCodeProjectNotFound,
		Message:   "Project 'foobar' not found",
		RequestID: "req-456",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ErrorMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Code != ErrCodeProjectNotFound {
		t.Errorf("code = %q, want %q", got.Code, ErrCodeProjectNotFound)
	}
	if got.Message != "Project 'foobar' not found" {
		t.Errorf("message = %q", got.Message)
	}
	if got.RequestID != "req-456" {
		t.Errorf("request_id = %q, want %q", got.RequestID, "req-456")
	}
}

func TestErrorMessageWithoutRequestID(t *testing.T) {
	msg := ErrorMessage{
		Type:      TypeError,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Code:      ErrCodeClaudeError,
		Message:   "Claude process crashed",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify request_id is omitted.
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, ok := raw["request_id"]; ok {
		t.Error("request_id should be omitted when empty")
	}
}

func TestDiffMessageRoundTrip(t *testing.T) {
	msg := DiffMessage{
		Type:      TypeDiff,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
		StoryID:   "US-003",
		Files:     []string{"internal/auth/auth.go", "internal/auth/auth_test.go"},
		DiffText:  "--- a/internal/auth/auth.go\n+++ b/internal/auth/auth.go\n@@ -1,3 +1,5 @@\n+// new code\n",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got DiffMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Files) != 2 {
		t.Fatalf("files count = %d, want 2", len(got.Files))
	}
	if got.Files[0] != "internal/auth/auth.go" {
		t.Errorf("files[0] = %q", got.Files[0])
	}
	if got.DiffText == "" {
		t.Error("diff_text should not be empty")
	}
}

func TestStartRunRoundTrip(t *testing.T) {
	msg := StartRunMessage{
		Type:      TypeStartRun,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got StartRunMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != TypeStartRun {
		t.Errorf("type = %q, want %q", got.Type, TypeStartRun)
	}
	if got.Project != "my-project" {
		t.Errorf("project = %q", got.Project)
	}
	if got.PRDID != "auth" {
		t.Errorf("prd_id = %q", got.PRDID)
	}
}

func TestNewPRDRoundTrip(t *testing.T) {
	msg := NewPRDMessage{
		Type:           TypeNewPRD,
		ID:             newUUID(),
		Timestamp:      "2026-02-15T10:00:00Z",
		Project:        "my-project",
		SessionID:      "session-abc",
		InitialMessage: "Build an authentication system",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got NewPRDMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SessionID != "session-abc" {
		t.Errorf("session_id = %q", got.SessionID)
	}
	if got.InitialMessage != "Build an authentication system" {
		t.Errorf("initial_message = %q", got.InitialMessage)
	}
}

func TestCloneRepoRoundTrip(t *testing.T) {
	msg := CloneRepoMessage{
		Type:          TypeCloneRepo,
		ID:            newUUID(),
		Timestamp:     "2026-02-15T10:00:00Z",
		URL:           "git@github.com:user/repo.git",
		DirectoryName: "my-repo",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got CloneRepoMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.URL != "git@github.com:user/repo.git" {
		t.Errorf("url = %q", got.URL)
	}
	if got.DirectoryName != "my-repo" {
		t.Errorf("directory_name = %q", got.DirectoryName)
	}
}

func TestCloneRepoOmitsEmptyDirectoryName(t *testing.T) {
	msg := CloneRepoMessage{
		Type:      TypeCloneRepo,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		URL:       "git@github.com:user/repo.git",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, ok := raw["directory_name"]; ok {
		t.Error("directory_name should be omitted when empty")
	}
}

func TestUpdateSettingsPartialFields(t *testing.T) {
	// Only updating max_iterations and auto_commit.
	maxIter := 10
	autoCommit := false
	msg := UpdateSettingsMessage{
		Type:          TypeUpdateSettings,
		ID:            newUUID(),
		Timestamp:     "2026-02-15T10:00:00Z",
		Project:       "my-project",
		MaxIterations: &maxIter,
		AutoCommit:    &autoCommit,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got UpdateSettingsMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.MaxIterations == nil || *got.MaxIterations != 10 {
		t.Errorf("max_iterations = %v, want 10", got.MaxIterations)
	}
	if got.AutoCommit == nil || *got.AutoCommit != false {
		t.Errorf("auto_commit = %v, want false", got.AutoCommit)
	}
	if got.CommitPrefix != nil {
		t.Errorf("commit_prefix should be nil, got %v", got.CommitPrefix)
	}
	if got.ClaudeModel != nil {
		t.Errorf("claude_model should be nil, got %v", got.ClaudeModel)
	}
	if got.TestCommand != nil {
		t.Errorf("test_command should be nil, got %v", got.TestCommand)
	}
}

func TestSessionTimeoutWarningRoundTrip(t *testing.T) {
	msg := SessionTimeoutWarningMessage{
		Type:             TypeSessionTimeoutWarning,
		ID:               newUUID(),
		Timestamp:        "2026-02-15T10:00:00Z",
		SessionID:        "session-xyz",
		MinutesRemaining: 5,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SessionTimeoutWarningMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.MinutesRemaining != 5 {
		t.Errorf("minutes_remaining = %d, want 5", got.MinutesRemaining)
	}
	if got.SessionID != "session-xyz" {
		t.Errorf("session_id = %q", got.SessionID)
	}
}

func TestQuotaExhaustedRoundTrip(t *testing.T) {
	msg := QuotaExhaustedMessage{
		Type:      TypeQuotaExhausted,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Runs:      []string{"run-1", "run-2"},
		Sessions:  []string{"session-1"},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got QuotaExhaustedMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Runs) != 2 {
		t.Errorf("runs count = %d, want 2", len(got.Runs))
	}
	if len(got.Sessions) != 1 {
		t.Errorf("sessions count = %d, want 1", len(got.Sessions))
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	msg := SettingsMessage{
		Type:          TypeSettings,
		ID:            newUUID(),
		Timestamp:     "2026-02-15T10:00:00Z",
		Project:       "my-project",
		MaxIterations: 5,
		AutoCommit:    true,
		CommitPrefix:  "feat:",
		ClaudeModel:   "claude-opus-4-6",
		TestCommand:   "go test ./...",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SettingsMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.MaxIterations != 5 {
		t.Errorf("max_iterations = %d, want 5", got.MaxIterations)
	}
	if !got.AutoCommit {
		t.Error("auto_commit should be true")
	}
	if got.TestCommand != "go test ./..." {
		t.Errorf("test_command = %q", got.TestCommand)
	}
}

func TestLogLinesRoundTrip(t *testing.T) {
	msg := LogLinesMessage{
		Type:      TypeLogLines,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
		StoryID:   "US-003",
		Lines:     []string{"Starting iteration 1...", "Running tests...", "All tests passed."},
		Level:     "info",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got LogLinesMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Lines) != 3 {
		t.Fatalf("lines count = %d, want 3", len(got.Lines))
	}
	if got.Level != "info" {
		t.Errorf("level = %q, want %q", got.Level, "info")
	}
}

func TestRunPausedRoundTrip(t *testing.T) {
	msg := RunPausedMessage{
		Type:      TypeRunPaused,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
		StoryID:   "US-003",
		Reason:    "quota_exhausted",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunPausedMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Reason != "quota_exhausted" {
		t.Errorf("reason = %q, want %q", got.Reason, "quota_exhausted")
	}
}

func TestCloneProgressRoundTrip(t *testing.T) {
	msg := CloneProgressMessage{
		Type:         TypeCloneProgress,
		ID:           newUUID(),
		Timestamp:    "2026-02-15T10:00:00Z",
		URL:          "git@github.com:user/repo.git",
		ProgressText: "Receiving objects: 45%",
		Percent:      45,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got CloneProgressMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Percent != 45 {
		t.Errorf("percent = %d, want 45", got.Percent)
	}
	if got.ProgressText != "Receiving objects: 45%" {
		t.Errorf("progress_text = %q", got.ProgressText)
	}
}

func TestUpdateAvailableRoundTrip(t *testing.T) {
	msg := UpdateAvailableMessage{
		Type:           TypeUpdateAvailable,
		ID:             newUUID(),
		Timestamp:      "2026-02-15T10:00:00Z",
		CurrentVersion: "0.5.0",
		LatestVersion:  "0.5.1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got UpdateAvailableMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.CurrentVersion != "0.5.0" {
		t.Errorf("current_version = %q", got.CurrentVersion)
	}
	if got.LatestVersion != "0.5.1" {
		t.Errorf("latest_version = %q", got.LatestVersion)
	}
}

func TestPingPongRoundTrip(t *testing.T) {
	ping := PingMessage{
		Type:      TypePing,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
	}

	data, err := json.Marshal(ping)
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}

	var gotPing PingMessage
	if err := json.Unmarshal(data, &gotPing); err != nil {
		t.Fatalf("unmarshal ping: %v", err)
	}

	if gotPing.Type != TypePing {
		t.Errorf("ping type = %q, want %q", gotPing.Type, TypePing)
	}

	pong := PongMessage{
		Type:      TypePong,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
	}

	data, err = json.Marshal(pong)
	if err != nil {
		t.Fatalf("marshal pong: %v", err)
	}

	var gotPong PongMessage
	if err := json.Unmarshal(data, &gotPong); err != nil {
		t.Fatalf("unmarshal pong: %v", err)
	}

	if gotPong.Type != TypePong {
		t.Errorf("pong type = %q, want %q", gotPong.Type, TypePong)
	}
}

func TestGenericMessageEnvelopeParsing(t *testing.T) {
	// Verify that any message can be parsed as the generic Message type for routing.
	msg := RunProgressMessage{
		Type:      TypeRunProgress,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
		StoryID:   "US-003",
		Status:    "running",
		Iteration: 2,
		Attempt:   1,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var envelope Message
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if envelope.Type != TypeRunProgress {
		t.Errorf("type = %q, want %q", envelope.Type, TypeRunProgress)
	}
	if envelope.ID == "" {
		t.Error("id should be set")
	}
	if envelope.Timestamp == "" {
		t.Error("timestamp should be set")
	}
}

func TestClosePRDSessionRoundTrip(t *testing.T) {
	msg := ClosePRDSessionMessage{
		Type:      TypeClosePRDSession,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		SessionID: "session-abc",
		Save:      true,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ClosePRDSessionMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !got.Save {
		t.Error("save should be true")
	}
	if got.SessionID != "session-abc" {
		t.Errorf("session_id = %q", got.SessionID)
	}
}

func TestSessionExpiredRoundTrip(t *testing.T) {
	msg := SessionExpiredMessage{
		Type:       TypeSessionExpired,
		ID:         newUUID(),
		Timestamp:  "2026-02-15T10:00:00Z",
		SessionID:  "session-abc",
		SavedState: "partial PRD content here",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SessionExpiredMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SavedState != "partial PRD content here" {
		t.Errorf("saved_state = %q", got.SavedState)
	}
}

func TestCreateProjectRoundTrip(t *testing.T) {
	msg := CreateProjectMessage{
		Type:      TypeCreateProject,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Name:      "new-project",
		GitInit:   true,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got CreateProjectMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Name != "new-project" {
		t.Errorf("name = %q", got.Name)
	}
	if !got.GitInit {
		t.Error("git_init should be true")
	}
}

func TestGetLogsWithOptionalFields(t *testing.T) {
	// With all fields.
	msg := GetLogsMessage{
		Type:      TypeGetLogs,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
		StoryID:   "US-003",
		Lines:     100,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got GetLogsMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.StoryID != "US-003" {
		t.Errorf("story_id = %q", got.StoryID)
	}
	if got.Lines != 100 {
		t.Errorf("lines = %d, want 100", got.Lines)
	}

	// Without optional fields.
	msg2 := GetLogsMessage{
		Type:      TypeGetLogs,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
	}

	data2, _ := json.Marshal(msg2)
	var raw map[string]interface{}
	json.Unmarshal(data2, &raw)

	if _, ok := raw["story_id"]; ok {
		t.Error("story_id should be omitted when empty")
	}
	if v, ok := raw["lines"]; ok && v != float64(0) {
		t.Error("lines should be omitted when zero")
	}
}

func TestCloneCompleteRoundTrip(t *testing.T) {
	// Success case.
	msg := CloneCompleteMessage{
		Type:      TypeCloneComplete,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		URL:       "git@github.com:user/repo.git",
		Success:   true,
		Project:   "repo",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got CloneCompleteMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !got.Success {
		t.Error("success should be true")
	}
	if got.Project != "repo" {
		t.Errorf("project = %q", got.Project)
	}

	// Failure case.
	msg2 := CloneCompleteMessage{
		Type:      TypeCloneComplete,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		URL:       "git@github.com:user/repo.git",
		Success:   false,
		Error:     "repository not found",
	}

	data2, _ := json.Marshal(msg2)
	var got2 CloneCompleteMessage
	json.Unmarshal(data2, &got2)

	if got2.Success {
		t.Error("success should be false")
	}
	if got2.Error != "repository not found" {
		t.Errorf("error = %q", got2.Error)
	}
}

func TestPRDContentRoundTrip(t *testing.T) {
	msg := PRDContentMessage{
		Type:      TypePRDContent,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
		Content:   "# Authentication PRD\n\nBuild a login system.",
		State:     map[string]interface{}{"stories": 5, "completed": 3},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got PRDContentMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Content != "# Authentication PRD\n\nBuild a login system." {
		t.Errorf("content = %q", got.Content)
	}
	if got.PRDID != "auth" {
		t.Errorf("prd_id = %q", got.PRDID)
	}
}

func TestProjectListRoundTrip(t *testing.T) {
	msg := ProjectListMessage{
		Type:      TypeProjectList,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Projects: []ProjectSummary{
			{Name: "project-a", Path: "/home/user/projects/project-a", HasChief: true, Branch: "main"},
			{Name: "project-b", Path: "/home/user/projects/project-b", HasChief: false, Branch: "develop"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ProjectListMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Projects) != 2 {
		t.Fatalf("projects count = %d, want 2", len(got.Projects))
	}
	if got.Projects[1].Branch != "develop" {
		t.Errorf("projects[1].branch = %q", got.Projects[1].Branch)
	}
}

func TestPRDMessageRoundTrip(t *testing.T) {
	msg := PRDMessageMessage{
		Type:      TypePRDMessage,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		SessionID: "session-abc",
		Content:   "Add OAuth support to the PRD",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got PRDMessageMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Content != "Add OAuth support to the PRD" {
		t.Errorf("content = %q", got.Content)
	}
}
