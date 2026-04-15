package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultTruncateLen = 500

// ConversationMessage is a single human or assistant turn in the conversation view.
type ConversationMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ConversationOptions controls which messages are included and how they are formatted.
type ConversationOptions struct {
	// Role filters messages to "user" or "assistant". Empty means both.
	Role string
	// IncludeThinking includes thinking blocks in assistant messages when true.
	// Thinking content is typically empty in persisted sessions (redacted).
	IncludeThinking bool
	// IncludeToolResults includes tool result summaries in user messages when true.
	IncludeToolResults bool
	// Full disables the 500-character truncation on tool result content.
	Full bool
}

// ConversationFromEntries extracts human/assistant messages from entries in
// chronological order, applying the given options. Streaming assistant entries
// sharing the same message.id are deduplicated, keeping the last (most complete)
// entry. Meta messages (isMeta: true) are excluded.
func ConversationFromEntries(entries []Entry, opts ConversationOptions) ([]ConversationMessage, error) {
	// First pass: build a map from message.id to the last (most complete) entry,
	// and a map from tool_use_id to tool name for tool result labeling.
	assistantByMsgID := make(map[string]Entry)
	toolNameByUseID := make(map[string]string)
	for _, e := range entries {
		if e.Type != "assistant" || e.Message == nil {
			continue
		}
		msgID := e.Message.ID
		if msgID == "" {
			msgID = e.UUID
		}
		assistantByMsgID[msgID] = e

		// Extract tool names from tool_use blocks.
		blocks, err := ParseContentBlocks(e.Message.Content)
		if err == nil {
			for _, block := range blocks {
				if tu, ok := block.(ToolUseBlock); ok && tu.ID != "" {
					toolNameByUseID[tu.ID] = tu.Name
				}
			}
		}
	}

	// Second pass: walk entries in file order. For user entries emit directly;
	// for assistant entries emit the deduplicated (last) version the first time
	// that message.id is encountered.
	seenMsgIDs := make(map[string]bool)

	var msgs []ConversationMessage
	for _, e := range entries {
		switch e.Type {
		case "user":
			if e.IsMeta || e.Message == nil {
				continue
			}
			if opts.Role != "" && opts.Role != "user" {
				continue
			}
			msg, err := userMessage(e, opts, toolNameByUseID)
			if err != nil {
				// Skip malformed content rather than aborting.
				continue
			}
			if msg != nil {
				msgs = append(msgs, *msg)
			}

		case "assistant":
			if e.Message == nil {
				continue
			}
			if opts.Role != "" && opts.Role != "assistant" {
				continue
			}
			msgID := e.Message.ID
			if msgID == "" {
				msgID = e.UUID
			}
			if seenMsgIDs[msgID] {
				continue
			}
			seenMsgIDs[msgID] = true

			// Use the deduplicated (last) entry for this message ID.
			canonical := assistantByMsgID[msgID]
			msg, err := assistantMessage(canonical, opts)
			if err != nil {
				continue
			}
			if msg != nil {
				msgs = append(msgs, *msg)
			}
		}
	}

	return msgs, nil
}

// LoadConversation resolves a session by prefix and returns its conversation messages.
func LoadConversation(claudeHome, prefix string, opts ConversationOptions) ([]ConversationMessage, error) {
	s, err := Resolve(claudeHome, prefix)
	if err != nil {
		return nil, err
	}

	jsonlPath := filepath.Join(claudeHome, "projects", s.ProjectSlug, s.ID+".jsonl")
	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	entries, err := ParseEntries(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return ConversationFromEntries(entries, opts)
}

// userMessage converts a user entry into a ConversationMessage. Returns nil when
// the entry produces no visible content (e.g. it contains only tool results and
// opts.IncludeToolResults is false).
func userMessage(e Entry, opts ConversationOptions, toolNames map[string]string) (*ConversationMessage, error) {
	blocks, err := ParseContentBlocks(e.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user content blocks: %w", err)
	}

	var parts []string
	for _, block := range blocks {
		switch b := block.(type) {
		case TextBlock:
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case ToolResultBlock:
			if !opts.IncludeToolResults {
				continue
			}
			summary := toolResultSummary(b, opts.Full, toolNames)
			if summary != "" {
				parts = append(parts, summary)
			}
		}
	}

	if len(parts) == 0 {
		return nil, nil
	}
	return &ConversationMessage{
		Role:      "user",
		Content:   strings.Join(parts, "\n\n"),
		Timestamp: e.Timestamp,
	}, nil
}

// assistantMessage converts an assistant entry into a ConversationMessage. Returns
// nil when the entry produces no visible content after filtering.
func assistantMessage(e Entry, opts ConversationOptions) (*ConversationMessage, error) {
	blocks, err := ParseContentBlocks(e.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse assistant content blocks: %w", err)
	}

	var parts []string
	for _, block := range blocks {
		switch b := block.(type) {
		case TextBlock:
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case ThinkingBlock:
			if opts.IncludeThinking && b.Thinking != "" {
				parts = append(parts, b.Thinking)
			}
		}
		// ToolUseBlock is intentionally excluded from the conversation view.
	}

	if len(parts) == 0 {
		return nil, nil
	}
	return &ConversationMessage{
		Role:      "assistant",
		Content:   strings.Join(parts, "\n\n"),
		Timestamp: e.Timestamp,
	}, nil
}

// toolResultSummary produces a short summary string for a tool result block.
// The tool name is resolved from the toolNames map (built from assistant
// tool_use blocks). The content is truncated to defaultTruncateLen unless full
// is true.
func toolResultSummary(b ToolResultBlock, full bool, toolNames map[string]string) string {
	content := extractToolResultText(b.Content)
	if !full && len(content) > defaultTruncateLen {
		content = content[:defaultTruncateLen] + "..."
	}
	label := toolNames[b.ToolUseID]
	if label == "" {
		label = "tool"
	}
	if content == "" {
		return label
	}
	return fmt.Sprintf("%s: %s", label, content)
}

// extractToolResultText returns the text representation of a tool result's
// content field. The field may be a JSON string, a JSON object with stdout/stderr
// fields, or another structured value.
func extractToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Plain string.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}

	// Object with stdout/stderr (Bash tool).
	if raw[0] == '{' {
		var obj struct {
			Stdout string `json:"stdout"`
			Stderr string `json:"stderr"`
		}
		if err := json.Unmarshal(raw, &obj); err == nil {
			var parts []string
			if obj.Stdout != "" {
				parts = append(parts, obj.Stdout)
			}
			if obj.Stderr != "" {
				parts = append(parts, obj.Stderr)
			}
			return strings.Join(parts, "\n")
		}
	}

	// Fallback: return raw JSON.
	return string(raw)
}
