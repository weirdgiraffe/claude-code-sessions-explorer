// Package session provides types and utilities for reading Claude Code session data.
package session

import (
	"encoding/json"
	"time"
)

// Entry is a single line from a Claude Code session JSONL file.
// The Type field discriminates the kind of entry; additional fields are populated
// according to the type. Content blocks (message.content) are stored as
// json.RawMessage to defer deep parsing.
type Entry struct {
	Type        string    `json:"type"`
	UUID        string    `json:"uuid"`
	ParentUUID  *string   `json:"parentUuid"`
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"sessionId"`
	IsSidechain bool      `json:"isSidechain"`
	UserType    string    `json:"userType"`
	Entrypoint  string    `json:"entrypoint"`
	CWD         string    `json:"cwd"`
	Version     string    `json:"version"`
	GitBranch   string    `json:"gitBranch"`
	Slug        string    `json:"slug"`
	IsMeta      bool      `json:"isMeta"`

	// user / assistant entries
	Message *Message `json:"message,omitempty"`

	// permission-mode entries
	PermissionMode string `json:"permissionMode,omitempty"`

	// system entries
	Subtype    string `json:"subtype,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`

	// custom-title entries
	CustomTitle string `json:"customTitle,omitempty"`

	// agent-name entries
	AgentName string `json:"agentName,omitempty"`

	// last-prompt entries
	LastPrompt string `json:"lastPrompt,omitempty"`

	// progress entries
	Data            json.RawMessage `json:"data,omitempty"`
	ToolUseID       string          `json:"toolUseID,omitempty"`
	ParentToolUseID string          `json:"parentToolUseID,omitempty"`

	// file-history-snapshot entries — stored raw, not needed for metadata
	Snapshot json.RawMessage `json:"snapshot,omitempty"`

	// user entries — tool result source
	SourceToolAssistantUUID string `json:"sourceToolAssistantUUID,omitempty"`
	PromptID                string `json:"promptId,omitempty"`
	AgentID                 string `json:"agentId,omitempty"`

	// assistant entries
	RequestID string `json:"requestId,omitempty"`
}

// Message is the message payload inside a user or assistant entry.
type Message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Model      string          `json:"model,omitempty"`
	ID         string          `json:"id,omitempty"`
	Type       string          `json:"type,omitempty"`
	StopReason *string         `json:"stop_reason,omitempty"`
	Usage      *TokenUsage     `json:"usage,omitempty"`
}

// TokenUsage holds token accounting data from an assistant entry.
type TokenUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// ContentBlock is the discriminated union for a single block inside a message's
// content array. Use the BlockType() method to determine the concrete type, then
// cast with the typed accessors or a type switch.
type ContentBlock interface {
	BlockType() string
}

// TextBlock is a plain-text content block (type "text").
type TextBlock struct {
	Text string `json:"text"`
}

// BlockType implements ContentBlock.
func (b TextBlock) BlockType() string { return "text" }

// ToolUseBlock is a tool invocation block (type "tool_use", assistant only).
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// BlockType implements ContentBlock.
func (b ToolUseBlock) BlockType() string { return "tool_use" }

// ToolResultBlock is the result of a tool call (type "tool_result", user only).
type ToolResultBlock struct {
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// BlockType implements ContentBlock.
func (b ToolResultBlock) BlockType() string { return "tool_result" }

// ThinkingBlock is an extended thinking block (type "thinking", assistant only).
// The Thinking field is empty in persisted sessions (redacted); only Signature is stored.
type ThinkingBlock struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// BlockType implements ContentBlock.
func (b ThinkingBlock) BlockType() string { return "thinking" }

// Session is the aggregated metadata for a single Claude Code session.
type Session struct {
	ID           string    `json:"id"`
	ProjectSlug  string    `json:"project_slug"`
	Slug         string    `json:"slug"`
	Model        string    `json:"model"`
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at"`
	DurationSecs float64   `json:"duration_secs"`
	MessageCount int       `json:"message_count"`
}

// Overview is the aggregated summary of a single Claude Code session.
type Overview struct {
	SessionID     string       `json:"session_id"`
	Model         string       `json:"model"`
	DurationSecs  float64      `json:"duration_secs"`
	MessageCount  MessageCount `json:"message_count"`
	TokenUsage    TokenUsage   `json:"token_usage"`
	Tools         []string     `json:"tools"`
	SubagentCount int          `json:"subagent_count"`
}

// MessageCount breaks down message counts by role.
type MessageCount struct {
	User      int `json:"user"`
	Assistant int `json:"assistant"`
	Total     int `json:"total"`
}
