package ws

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

// ProtocolVersion is the current protocol version.
const ProtocolVersion = 1

// Message represents a protocol message envelope.
type Message struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// NewMessage creates a new message envelope with type, UUID, and ISO8601 timestamp.
func NewMessage(msgType string) Message {
	return Message{
		Type:      msgType,
		ID:        newUUID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// newUUID generates a random UUID v4 string.
func newUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	// Set version 4 bits.
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant bits.
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// Error codes for protocol error messages.
const (
	ErrCodeAuthFailed          = "AUTH_FAILED"
	ErrCodeProjectNotFound     = "PROJECT_NOT_FOUND"
	ErrCodePRDNotFound         = "PRD_NOT_FOUND"
	ErrCodeRunAlreadyActive    = "RUN_ALREADY_ACTIVE"
	ErrCodeRunNotActive        = "RUN_NOT_ACTIVE"
	ErrCodeSessionNotFound     = "SESSION_NOT_FOUND"
	ErrCodeCloneFailed         = "CLONE_FAILED"
	ErrCodeQuotaExhausted      = "QUOTA_EXHAUSTED"
	ErrCodeFilesystemError     = "FILESYSTEM_ERROR"
	ErrCodeClaudeError         = "CLAUDE_ERROR"
	ErrCodeUpdateFailed        = "UPDATE_FAILED"
	ErrCodeIncompatibleVersion = "INCOMPATIBLE_VERSION"
	ErrCodeRateLimited         = "RATE_LIMITED"
)

// Message type constants for the protocol catalog.
const (
	// Server → Web App message types.
	TypeHello                 = "hello"
	TypeStateSnapshot         = "state_snapshot"
	TypeProjectList           = "project_list"
	TypeProjectState          = "project_state"
	TypePRDContent            = "prd_content"
	TypeClaudeOutput          = "claude_output"
	TypeRunProgress           = "run_progress"
	TypeRunComplete           = "run_complete"
	TypeRunPaused             = "run_paused"
	TypeDiff                  = "diff"
	TypeDiffsResponse         = "diffs_response"
	TypePRDsResponse          = "prds_response"
	TypeCloneProgress         = "clone_progress"
	TypeCloneComplete         = "clone_complete"
	TypeError                 = "error"
	TypeQuotaExhausted        = "quota_exhausted"
	TypeLogLines              = "log_lines"
	TypeSessionTimeoutWarning = "session_timeout_warning"
	TypeSessionExpired        = "session_expired"
	TypeSettings              = "settings"
	TypeSettingsResponse      = "settings_response"
	TypeSettingsUpdated       = "settings_updated"
	TypeUpdateAvailable       = "update_available"
	TypePRDOutput             = "prd_output"
	TypePRDResponseComplete   = "prd_response_complete"

	// Web App → Server message types.
	TypeWelcome         = "welcome"
	TypeIncompatible    = "incompatible"
	TypeListProjects    = "list_projects"
	TypeGetProject      = "get_project"
	TypeGetPRD          = "get_prd"
	TypeGetPRDs         = "get_prds"
	TypeNewPRD          = "new_prd"
	TypePRDMessage      = "prd_message"
	TypeClosePRDSession = "close_prd_session"
	TypeStartRun        = "start_run"
	TypePauseRun        = "pause_run"
	TypeResumeRun       = "resume_run"
	TypeStopRun         = "stop_run"
	TypeCloneRepo       = "clone_repo"
	TypeCreateProject   = "create_project"
	TypeGetDiff         = "get_diff"
	TypeGetDiffs        = "get_diffs"
	TypeGetLogs         = "get_logs"
	TypeGetSettings     = "get_settings"
	TypeUpdateSettings  = "update_settings"
	TypeTriggerUpdate   = "trigger_update"
	TypePing            = "ping"

	// Bidirectional.
	TypePong = "pong"
)

// --- Server → Web App messages ---

// StateSnapshotMessage is sent on connect/reconnect with full state.
type StateSnapshotMessage struct {
	Type     string           `json:"type"`
	ID       string           `json:"id"`
	Timestamp string          `json:"timestamp"`
	Projects []ProjectSummary `json:"projects"`
	Runs     []RunState       `json:"runs"`
	Sessions []SessionState   `json:"sessions"`
}

// ProjectSummary describes a project in the workspace.
type ProjectSummary struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	HasChief bool       `json:"has_chief"`
	Branch   string     `json:"branch"`
	Commit   CommitInfo `json:"commit"`
	PRDs     []PRDInfo  `json:"prds"`
}

// CommitInfo describes a git commit.
type CommitInfo struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	Author    string `json:"author"`
	Timestamp string `json:"timestamp"`
}

// PRDInfo describes a PRD in a project.
type PRDInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	StoryCount       int    `json:"story_count"`
	CompletionStatus string `json:"completion_status"`
}

// RunState describes an active run.
type RunState struct {
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
	StoryID   string `json:"story_id"`
	Status    string `json:"status"`
	Iteration int    `json:"iteration"`
}

// SessionState describes an active Claude session.
type SessionState struct {
	SessionID string `json:"session_id"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
}

// ProjectListMessage lists all discovered projects.
type ProjectListMessage struct {
	Type      string           `json:"type"`
	ID        string           `json:"id"`
	Timestamp string           `json:"timestamp"`
	Projects  []ProjectSummary `json:"projects"`
}

// ProjectStateMessage returns state for a single project.
type ProjectStateMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Project   ProjectSummary `json:"project"`
}

// PRDContentMessage returns PRD content and state.
type PRDContentMessage struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Timestamp string      `json:"timestamp"`
	Project   string      `json:"project"`
	PRDID     string      `json:"prd_id"`
	Content   string      `json:"content"`
	State     interface{} `json:"state"`
}

// ClaudeOutputMessage streams Claude output.
type ClaudeOutputMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id,omitempty"`
	Project   string `json:"project,omitempty"`
	PRDID     string `json:"prd_id,omitempty"`
	StoryID   string `json:"story_id,omitempty"`
	Data      string `json:"data"`
	Done      bool   `json:"done"`
}

// PRDOutputMessage streams PRD session output (text chunks from Claude).
type PRDOutputMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id"`
	Project   string `json:"project"`
	Text      string `json:"text"`
}

// PRDResponseCompleteMessage signals that a PRD session's Claude process has finished.
type PRDResponseCompleteMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id"`
	Project   string `json:"project"`
}

// RunProgressMessage reports run state changes.
type RunProgressMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
	StoryID   string `json:"story_id"`
	Status    string `json:"status"`
	Iteration int    `json:"iteration"`
	Attempt   int    `json:"attempt"`
}

// RunCompleteMessage reports run completion.
type RunCompleteMessage struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	Timestamp        string `json:"timestamp"`
	Project          string `json:"project"`
	PRDID            string `json:"prd_id"`
	StoriesCompleted int    `json:"stories_completed"`
	Duration         string `json:"duration"`
	PassCount        int    `json:"pass_count"`
	FailCount        int    `json:"fail_count"`
}

// RunPausedMessage reports a paused run.
type RunPausedMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
	StoryID   string `json:"story_id"`
	Reason    string `json:"reason"`
}

// DiffMessage contains a story's diff.
type DiffMessage struct {
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	Project   string   `json:"project"`
	PRDID     string   `json:"prd_id"`
	StoryID   string   `json:"story_id"`
	Files     []string `json:"files"`
	DiffText  string   `json:"diff_text"`
}

// CloneProgressMessage reports git clone progress.
type CloneProgressMessage struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	URL          string `json:"url"`
	ProgressText string `json:"progress_text"`
	Percent      int    `json:"percent"`
}

// CloneCompleteMessage reports clone completion.
type CloneCompleteMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	URL       string `json:"url"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Project   string `json:"project,omitempty"`
}

// ErrorMessage reports an error.
type ErrorMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// QuotaExhaustedMessage reports quota exhaustion.
type QuotaExhaustedMessage struct {
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	Runs      []string `json:"runs"`
	Sessions  []string `json:"sessions"`
}

// LogLinesMessage returns log content.
type LogLinesMessage struct {
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	Project   string   `json:"project"`
	PRDID     string   `json:"prd_id"`
	StoryID   string   `json:"story_id"`
	Lines     []string `json:"lines"`
	Level     string   `json:"level"`
}

// SessionTimeoutWarningMessage warns of impending session timeout.
type SessionTimeoutWarningMessage struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	Timestamp        string `json:"timestamp"`
	SessionID        string `json:"session_id"`
	MinutesRemaining int    `json:"minutes_remaining"`
}

// SessionExpiredMessage reports that a session has timed out.
type SessionExpiredMessage struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	SessionID  string `json:"session_id"`
	SavedState string `json:"saved_state,omitempty"`
}

// SettingsMessage returns project settings.
type SettingsMessage struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	Project       string `json:"project"`
	MaxIterations int    `json:"max_iterations"`
	AutoCommit    bool   `json:"auto_commit"`
	CommitPrefix  string `json:"commit_prefix"`
	ClaudeModel   string `json:"claude_model"`
	TestCommand   string `json:"test_command"`
}

// UpdateAvailableMessage reports an available update.
type UpdateAvailableMessage struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	Timestamp      string `json:"timestamp"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
}

// --- Web App → Server messages ---

// ListProjectsMessage requests the project list.
type ListProjectsMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}

// GetProjectMessage requests a single project's state.
type GetProjectMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
}

// GetPRDsMessage requests a list of all PRDs for a project.
type GetPRDsMessage struct {
	Project string `json:"project"`
}

// PRDsResponseMessage returns a list of PRDs for a project.
type PRDsResponseMessage struct {
	Type    string               `json:"type"`
	Payload PRDsResponsePayload  `json:"payload"`
}

// PRDsResponsePayload is the payload of a PRDs response.
type PRDsResponsePayload struct {
	Project string    `json:"project"`
	PRDs    []PRDItem `json:"prds"`
}

// PRDItem describes a PRD in the response list.
type PRDItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	StoryCount int    `json:"story_count"`
	Status     string `json:"status"`
}

// SettingsResponseMessage wraps settings for browser delivery.
type SettingsResponseMessage struct {
	Type    string                  `json:"type"`
	Payload SettingsResponsePayload `json:"payload"`
}

// SettingsResponsePayload is the payload of a settings response.
type SettingsResponsePayload struct {
	Project  string       `json:"project"`
	Settings SettingsData `json:"settings"`
}

// SettingsData contains project settings fields.
type SettingsData struct {
	MaxIterations int    `json:"max_iterations"`
	AutoCommit    bool   `json:"auto_commit"`
	CommitPrefix  string `json:"commit_prefix"`
	ClaudeModel   string `json:"claude_model"`
	TestCommand   string `json:"test_command"`
}

// DiffsResponseMessage wraps diff data for browser delivery.
type DiffsResponseMessage struct {
	Type    string               `json:"type"`
	Payload DiffsResponsePayload `json:"payload"`
}

// DiffsResponsePayload is the payload of a diffs response.
type DiffsResponsePayload struct {
	Project string           `json:"project"`
	StoryID string           `json:"story_id"`
	Files   []DiffFileDetail `json:"files"`
}

// DiffFileDetail represents a single file's diff information.
type DiffFileDetail struct {
	Filename  string `json:"filename"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

// GetDiffsMessage requests diffs for a story (without requiring prd_id).
type GetDiffsMessage struct {
	Project string `json:"project"`
	StoryID string `json:"story_id"`
}

// GetPRDMessage requests a PRD's content.
type GetPRDMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
}

// NewPRDMessage requests creation of a new PRD via Claude.
type NewPRDMessage struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	Timestamp      string `json:"timestamp"`
	Project        string `json:"project"`
	SessionID      string `json:"session_id"`
	InitialMessage string `json:"initial_message"`
}

// PRDMessageMessage sends a user message to an active PRD session.
type PRDMessageMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
}

// ClosePRDSessionMessage closes a PRD session.
type ClosePRDSessionMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"session_id"`
	Save      bool   `json:"save"`
}

// StartRunMessage starts a Ralph loop.
type StartRunMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
}

// PauseRunMessage pauses a running loop.
type PauseRunMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
}

// ResumeRunMessage resumes a paused loop.
type ResumeRunMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
}

// StopRunMessage stops a running loop.
type StopRunMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
}

// CloneRepoMessage requests a git clone.
type CloneRepoMessage struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	URL           string `json:"url"`
	DirectoryName string `json:"directory_name,omitempty"`
}

// CreateProjectMessage creates a new project.
type CreateProjectMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Name      string `json:"name"`
	GitInit   bool   `json:"git_init"`
}

// GetDiffMessage requests a story's diff.
type GetDiffMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
	StoryID   string `json:"story_id"`
}

// GetLogsMessage requests log lines.
type GetLogsMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
	PRDID     string `json:"prd_id"`
	StoryID   string `json:"story_id,omitempty"`
	Lines     int    `json:"lines,omitempty"`
}

// GetSettingsMessage requests project settings.
type GetSettingsMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Project   string `json:"project"`
}

// UpdateSettingsMessage updates project settings.
type UpdateSettingsMessage struct {
	Type          string  `json:"type"`
	ID            string  `json:"id"`
	Timestamp     string  `json:"timestamp"`
	Project       string  `json:"project"`
	MaxIterations *int    `json:"max_iterations,omitempty"`
	AutoCommit    *bool   `json:"auto_commit,omitempty"`
	CommitPrefix  *string `json:"commit_prefix,omitempty"`
	ClaudeModel   *string `json:"claude_model,omitempty"`
	TestCommand   *string `json:"test_command,omitempty"`
}

// TriggerUpdateMessage requests a self-update.
type TriggerUpdateMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}

// PingMessage is a keepalive ping.
type PingMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}

// PongMessage is a keepalive pong response.
type PongMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}
