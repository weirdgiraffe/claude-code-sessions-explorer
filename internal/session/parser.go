package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ParseEntries reads JSONL entries line by line from r and returns all parsed
// entries. Blank lines are skipped. Returns an error on any malformed line.
func ParseEntries(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10 MiB max line
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("failed to parse JSONL line %d: %w", lineNum, err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read JSONL: %w", err)
	}
	return entries, nil
}

// ParseContentBlocks parses a message's raw content field into typed ContentBlock
// values. The content field may be a JSON string (plain text) or a JSON array of
// typed block objects.
func ParseContentBlocks(raw json.RawMessage) ([]ContentBlock, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Detect whether the content is a plain string or an array.
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, fmt.Errorf("failed to parse string content: %w", err)
		}
		return []ContentBlock{TextBlock{Text: text}}, nil
	}

	if raw[0] != '[' {
		return nil, fmt.Errorf("unexpected content JSON: expected string or array, got %q", string(raw[:min(len(raw), 20)]))
	}

	var rawBlocks []json.RawMessage
	if err := json.Unmarshal(raw, &rawBlocks); err != nil {
		return nil, fmt.Errorf("failed to parse content array: %w", err)
	}

	blocks := make([]ContentBlock, 0, len(rawBlocks))
	for i, rb := range rawBlocks {
		block, err := parseContentBlock(rb)
		if err != nil {
			return nil, fmt.Errorf("failed to parse content block %d: %w", i, err)
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// parseContentBlock unmarshals a single content block JSON object into the
// appropriate ContentBlock concrete type.
func parseContentBlock(raw json.RawMessage) (ContentBlock, error) {
	var discriminator struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &discriminator); err != nil {
		return nil, fmt.Errorf("failed to read block type: %w", err)
	}

	switch discriminator.Type {
	case "text":
		var b TextBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("failed to parse text block: %w", err)
		}
		return b, nil

	case "tool_use":
		var b ToolUseBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("failed to parse tool_use block: %w", err)
		}
		return b, nil

	case "tool_result":
		var b ToolResultBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("failed to parse tool_result block: %w", err)
		}
		return b, nil

	case "thinking":
		var b ThinkingBlock
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("failed to parse thinking block: %w", err)
		}
		return b, nil

	default:
		// Unknown block type — return a TextBlock with empty text rather than
		// failing so that future block types don't break existing callers.
		return TextBlock{}, nil
	}
}

// OverviewFromEntries aggregates a slice of parsed entries into an Overview.
// Streaming assistant entries sharing the same message.id are deduplicated,
// keeping the last entry (which has the most complete content and stop_reason set).
func OverviewFromEntries(sessionID string, entries []Entry) (Overview, error) {
	// Deduplicate streaming assistant entries by message.id.
	// We process entries in order; for each message.id we overwrite with the
	// latest entry, preserving insertion order via a separate slice of seen IDs.
	assistantByMsgID := make(map[string]Entry)
	var assistantOrder []string // message IDs in first-seen order

	for _, e := range entries {
		if e.Type != "assistant" || e.Message == nil {
			continue
		}
		msgID := e.Message.ID
		if msgID == "" {
			// No message ID — treat as unique.
			msgID = e.UUID
		}
		if _, seen := assistantByMsgID[msgID]; !seen {
			assistantOrder = append(assistantOrder, msgID)
		}
		assistantByMsgID[msgID] = e
	}

	var (
		model         string
		msgCountUser  int
		msgCountAsst  int
		totalUsage    TokenUsage
		tools         []string
		subagentCount int
		durationSecs  float64
	)

	// Accumulate turn_duration from system entries.
	var totalDurationMs int64
	for _, e := range entries {
		if e.Type == "system" && e.Subtype == "turn_duration" {
			totalDurationMs += e.DurationMs
		}
	}
	if totalDurationMs > 0 {
		durationSecs = float64(totalDurationMs) / 1000.0
	} else if len(entries) > 0 {
		// Fallback: compute from first/last non-zero entry timestamps.
		var firstTS, lastTS time.Time
		for _, e := range entries {
			if e.Timestamp.IsZero() {
				continue
			}
			if firstTS.IsZero() || e.Timestamp.Before(firstTS) {
				firstTS = e.Timestamp
			}
			if lastTS.IsZero() || e.Timestamp.After(lastTS) {
				lastTS = e.Timestamp
			}
		}
		if !firstTS.IsZero() && lastTS.After(firstTS) {
			durationSecs = lastTS.Sub(firstTS).Seconds()
		}
	}

	// Count user messages (non-meta).
	for _, e := range entries {
		if e.Type == "user" && !e.IsMeta {
			msgCountUser++
		}
	}

	// Count subagents from progress entries with data.type == "agent_progress".
	seenAgentIDs := make(map[string]bool)
	for _, e := range entries {
		if e.Type != "progress" || len(e.Data) == 0 {
			continue
		}
		var data struct {
			Type    string `json:"type"`
			AgentID string `json:"agentId"`
		}
		if err := json.Unmarshal(e.Data, &data); err != nil {
			continue
		}
		if data.Type == "agent_progress" && data.AgentID != "" && !seenAgentIDs[data.AgentID] {
			seenAgentIDs[data.AgentID] = true
			subagentCount++
		}
	}

	// Process deduplicated assistant entries in first-seen order.
	for _, msgID := range assistantOrder {
		e := assistantByMsgID[msgID]

		msgCountAsst++

		if model == "" && e.Message.Model != "" {
			model = e.Message.Model
		}

		if e.Message.Usage != nil {
			totalUsage.InputTokens += e.Message.Usage.InputTokens
			totalUsage.OutputTokens += e.Message.Usage.OutputTokens
			totalUsage.CacheCreationInputTokens += e.Message.Usage.CacheCreationInputTokens
			totalUsage.CacheReadInputTokens += e.Message.Usage.CacheReadInputTokens
		}

		blocks, err := ParseContentBlocks(e.Message.Content)
		if err != nil {
			// Skip malformed content rather than aborting the whole overview.
			continue
		}
		for _, block := range blocks {
			if tu, ok := block.(ToolUseBlock); ok {
				tools = append(tools, tu.Name)
			}
		}
	}

	if tools == nil {
		tools = []string{}
	}

	return Overview{
		SessionID:     sessionID,
		Model:         model,
		DurationSecs:  durationSecs,
		MessageCount:  MessageCount{User: msgCountUser, Assistant: msgCountAsst, Total: msgCountUser + msgCountAsst},
		TokenUsage:    totalUsage,
		Tools:         tools,
		SubagentCount: subagentCount,
	}, nil
}
