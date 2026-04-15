package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ToolCall is a paired tool_use block with its matching tool_result response.
// Input and Result are truncated to 500 chars unless Full was requested.
type ToolCall struct {
	Name    string `json:"name"`
	Input   string `json:"input"`
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
}

// ToolsOptions controls which tool calls are returned and how they are formatted.
type ToolsOptions struct {
	// Name filters to a specific tool name. Empty means all tools.
	Name string
	// SequenceOnly returns just tool names in call order, omitting input/result.
	SequenceOnly bool
	// Full disables the 500-character truncation on input and result content.
	Full bool
}

// ToolsFromEntries extracts paired tool_use/tool_result entries from a session
// in chronological order. Streaming assistant entries are deduplicated (last
// wins). Tool results are matched to tool_use blocks by tool_use_id.
func ToolsFromEntries(entries []Entry, opts ToolsOptions) ([]ToolCall, error) {
	// First pass: deduplicate streaming assistant entries by message.id.
	// Last entry for a given message.id wins (most complete content).
	assistantByMsgID := make(map[string]Entry)
	for _, e := range entries {
		if e.Type != "assistant" || e.Message == nil {
			continue
		}
		msgID := e.Message.ID
		if msgID == "" {
			msgID = e.UUID
		}
		assistantByMsgID[msgID] = e
	}

	// Second pass: walk entries in file order to collect tool_use blocks.
	// For each assistant message, use the deduplicated (last) entry's content,
	// but preserve the first-seen order of message IDs.
	type toolUseRecord struct {
		id   string
		name string
		raw  json.RawMessage
	}
	var toolUses []toolUseRecord
	seenMsgIDs := make(map[string]bool)
	seenToolUseIDs := make(map[string]bool)

	for _, e := range entries {
		if e.Type != "assistant" || e.Message == nil {
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

		canonical := assistantByMsgID[msgID]
		blocks, err := ParseContentBlocks(canonical.Message.Content)
		if err != nil {
			continue
		}
		for _, block := range blocks {
			tu, ok := block.(ToolUseBlock)
			if !ok || tu.ID == "" {
				continue
			}
			if seenToolUseIDs[tu.ID] {
				continue
			}
			seenToolUseIDs[tu.ID] = true
			toolUses = append(toolUses, toolUseRecord{
				id:   tu.ID,
				name: tu.Name,
				raw:  tu.Input,
			})
		}
	}

	// Third pass: collect tool_result blocks keyed by tool_use_id.
	type toolResultRecord struct {
		raw     json.RawMessage
		isError bool
	}
	resultByUseID := make(map[string]toolResultRecord)
	for _, e := range entries {
		if e.Type != "user" || e.Message == nil {
			continue
		}
		blocks, err := ParseContentBlocks(e.Message.Content)
		if err != nil {
			continue
		}
		for _, block := range blocks {
			tr, ok := block.(ToolResultBlock)
			if !ok || tr.ToolUseID == "" {
				continue
			}
			// Later entries for the same tool_use_id overwrite earlier ones.
			resultByUseID[tr.ToolUseID] = toolResultRecord{
				raw:     tr.Content,
				isError: tr.IsError,
			}
		}
	}

	// Pair tool_use records with their results and apply filters.
	var calls []ToolCall
	for _, tu := range toolUses {
		if opts.Name != "" && tu.name != opts.Name {
			continue
		}

		result, hasResult := resultByUseID[tu.id]

		inputStr := formatRawJSON(tu.raw)
		var resultStr string
		var isError bool
		if hasResult {
			resultStr = extractToolResultText(result.raw)
			isError = result.isError
		}

		if !opts.Full {
			inputStr = truncate(inputStr, defaultTruncateLen)
			resultStr = truncate(resultStr, defaultTruncateLen)
		}

		calls = append(calls, ToolCall{
			Name:    tu.name,
			Input:   inputStr,
			Result:  resultStr,
			IsError: isError,
		})
	}

	if calls == nil {
		calls = []ToolCall{}
	}
	return calls, nil
}

// LoadTools resolves a session by prefix and returns its tool calls.
func LoadTools(claudeHome, prefix string, opts ToolsOptions) ([]ToolCall, error) {
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

	return ToolsFromEntries(entries, opts)
}

// ToolSequence extracts just the tool names in call order from a slice of ToolCall.
func ToolSequence(calls []ToolCall) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
	}
	return names
}

// formatRawJSON converts a json.RawMessage to a compact string for display.
// Falls back to the raw bytes if marshalling fails.
func formatRawJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Compact and re-encode to normalise whitespace.
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

// truncate shortens s to at most maxLen runes, appending "..." when cut.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
