package session_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weirdgiraffe/claude-code-sessions-explorer/internal/session"
)

func TestParseEntries_Empty(t *testing.T) {
	entries, err := session.ParseEntries(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseEntries_BlankLines(t *testing.T) {
	input := "\n\n\n"
	entries, err := session.ParseEntries(strings.NewReader(input))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseEntries_PermissionMode(t *testing.T) {
	line := `{"type":"permission-mode","permissionMode":"default","sessionId":"049e42da-1111-2222-3333-444444444444","uuid":"u1","timestamp":"2026-03-23T13:57:52.000Z"}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "permission-mode", e.Type)
	assert.Equal(t, "default", e.PermissionMode)
	assert.Equal(t, "049e42da-1111-2222-3333-444444444444", e.SessionID)
}

func TestParseEntries_UserTextMessage(t *testing.T) {
	line := `{"type":"user","uuid":"7ecb98c1-aaaa-bbbb-cccc-dddddddddddd","parentUuid":"c06decdb-aaaa-bbbb-cccc-dddddddddddd","isSidechain":false,"isMeta":false,"sessionId":"110bd295-aaaa-bbbb-cccc-dddddddddddd","timestamp":"2026-03-23T13:57:53.475Z","slug":"iridescent-scribbling-noodle","message":{"role":"user","content":"Hello, Claude!"},"cwd":"/home/user/project","version":"2.1.81","gitBranch":"main"}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "user", e.Type)
	assert.Equal(t, "iridescent-scribbling-noodle", e.Slug)
	assert.False(t, e.IsMeta)
	assert.Equal(t, "main", e.GitBranch)
	require.NotNil(t, e.Message)
	assert.Equal(t, "user", e.Message.Role)

	wantTS, _ := time.Parse(time.RFC3339, "2026-03-23T13:57:53.475Z")
	assert.True(t, e.Timestamp.Equal(wantTS))
}

func TestParseEntries_AssistantMessage(t *testing.T) {
	line := `{"type":"assistant","uuid":"8fb7a993-aaaa-bbbb-cccc-dddddddddddd","sessionId":"110bd295-aaaa-bbbb-cccc-dddddddddddd","timestamp":"2026-03-23T13:58:00.116Z","requestId":"req_abc123","message":{"role":"assistant","model":"claude-opus-4-6","id":"msg_abc123","type":"message","stop_reason":"tool_use","content":[{"type":"text","text":"I will help."}],"usage":{"input_tokens":1000,"output_tokens":50,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "assistant", e.Type)
	assert.Equal(t, "req_abc123", e.RequestID)
	require.NotNil(t, e.Message)
	assert.Equal(t, "claude-opus-4-6", e.Message.Model)
	assert.Equal(t, "tool_use", *e.Message.StopReason)
	require.NotNil(t, e.Message.Usage)
	assert.Equal(t, 1000, e.Message.Usage.InputTokens)
	assert.Equal(t, 50, e.Message.Usage.OutputTokens)
}

func TestParseEntries_SystemTurnDuration(t *testing.T) {
	line := `{"type":"system","subtype":"turn_duration","durationMs":273714,"uuid":"s1","sessionId":"sid","timestamp":"2026-03-23T14:23:57.834Z"}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "system", e.Type)
	assert.Equal(t, "turn_duration", e.Subtype)
	assert.Equal(t, int64(273714), e.DurationMs)
}

func TestParseEntries_CustomTitle(t *testing.T) {
	line := `{"type":"custom-title","customTitle":"my-cool-session","sessionId":"sid"}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "my-cool-session", entries[0].CustomTitle)
}

func TestParseEntries_AgentName(t *testing.T) {
	line := `{"type":"agent-name","agentName":"my-agent","sessionId":"sid"}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "my-agent", entries[0].AgentName)
}

func TestParseEntries_LastPrompt(t *testing.T) {
	line := `{"type":"last-prompt","lastPrompt":"yes. looks correct","sessionId":"sid"}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "yes. looks correct", entries[0].LastPrompt)
}

func TestParseEntries_Progress(t *testing.T) {
	line := `{"type":"progress","uuid":"c4a6829e-aaaa-bbbb-cccc-dddddddddddd","toolUseID":"agent_msg_xyz","parentToolUseID":"toolu_abc","timestamp":"2026-03-23T13:58:00.118Z","data":{"type":"agent_progress","agentId":"a22b0b3973014b318","prompt":"Do something"}}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "progress", e.Type)
	assert.Equal(t, "toolu_abc", e.ParentToolUseID)
	assert.NotNil(t, e.Data)
}

func TestParseEntries_FileHistorySnapshot(t *testing.T) {
	line := `{"type":"file-history-snapshot","messageId":"0b36d750-aaaa-bbbb-cccc-dddddddddddd","isSnapshotUpdate":false,"snapshot":{"messageId":"0b36d750-aaaa-bbbb-cccc-dddddddddddd","trackedFileBackups":{}}}`
	entries, err := session.ParseEntries(strings.NewReader(line))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "file-history-snapshot", entries[0].Type)
}

func TestParseEntries_MultipleLines(t *testing.T) {
	input := `{"type":"permission-mode","sessionId":"sid","permissionMode":"default","uuid":"u1","timestamp":"2026-01-01T00:00:00Z"}
{"type":"user","uuid":"u2","sessionId":"sid","timestamp":"2026-01-01T00:01:00Z","message":{"role":"user","content":"Hi"}}
{"type":"assistant","uuid":"u3","sessionId":"sid","timestamp":"2026-01-01T00:02:00Z","message":{"role":"assistant","model":"claude-opus-4-6","content":[]}}
`
	entries, err := session.ParseEntries(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, "permission-mode", entries[0].Type)
	assert.Equal(t, "user", entries[1].Type)
	assert.Equal(t, "assistant", entries[2].Type)
}

func TestParseEntries_MalformedLine(t *testing.T) {
	input := `{"type":"user","uuid":"u1"}
not valid json`
	_, err := session.ParseEntries(strings.NewReader(input))
	assert.Error(t, err)
}

// ---- Content block parsing ----

func TestParseContentBlocks_StringContent(t *testing.T) {
	raw := []byte(`"Hello, Claude!"`)
	blocks, err := session.ParseContentBlocks(raw)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	tb, ok := blocks[0].(session.TextBlock)
	require.True(t, ok, "expected TextBlock")
	assert.Equal(t, "Hello, Claude!", tb.Text)
	assert.Equal(t, "text", tb.BlockType())
}

func TestParseContentBlocks_EmptyRaw(t *testing.T) {
	blocks, err := session.ParseContentBlocks(nil)
	require.NoError(t, err)
	assert.Empty(t, blocks)
}

func TestParseContentBlocks_TextBlock(t *testing.T) {
	raw := []byte(`[{"type":"text","text":"I will help."}]`)
	blocks, err := session.ParseContentBlocks(raw)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	tb, ok := blocks[0].(session.TextBlock)
	require.True(t, ok)
	assert.Equal(t, "I will help.", tb.Text)
}

func TestParseContentBlocks_ToolUseBlock(t *testing.T) {
	raw := []byte(`[{"type":"tool_use","id":"toolu_abc","name":"Bash","input":{"command":"ls"}}]`)
	blocks, err := session.ParseContentBlocks(raw)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	tb, ok := blocks[0].(session.ToolUseBlock)
	require.True(t, ok)
	assert.Equal(t, "toolu_abc", tb.ID)
	assert.Equal(t, "Bash", tb.Name)
	assert.Equal(t, "tool_use", tb.BlockType())
}

func TestParseContentBlocks_ToolResultBlock(t *testing.T) {
	raw := []byte(`[{"type":"tool_result","tool_use_id":"toolu_abc","content":"output text","is_error":false}]`)
	blocks, err := session.ParseContentBlocks(raw)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	tb, ok := blocks[0].(session.ToolResultBlock)
	require.True(t, ok)
	assert.Equal(t, "toolu_abc", tb.ToolUseID)
	assert.False(t, tb.IsError)
	assert.Equal(t, "tool_result", tb.BlockType())
}

func TestParseContentBlocks_ThinkingBlock(t *testing.T) {
	raw := []byte(`[{"type":"thinking","thinking":"","signature":"BASE64SIG"}]`)
	blocks, err := session.ParseContentBlocks(raw)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	tb, ok := blocks[0].(session.ThinkingBlock)
	require.True(t, ok)
	assert.Equal(t, "", tb.Thinking)
	assert.Equal(t, "BASE64SIG", tb.Signature)
	assert.Equal(t, "thinking", tb.BlockType())
}

func TestParseContentBlocks_MixedBlocks(t *testing.T) {
	raw := []byte(`[
		{"type":"thinking","thinking":"","signature":"SIG"},
		{"type":"text","text":"Let me check."},
		{"type":"tool_use","id":"toolu_1","name":"Read","input":{}}
	]`)
	blocks, err := session.ParseContentBlocks(raw)
	require.NoError(t, err)
	require.Len(t, blocks, 3)
	assert.Equal(t, "thinking", blocks[0].BlockType())
	assert.Equal(t, "text", blocks[1].BlockType())
	assert.Equal(t, "tool_use", blocks[2].BlockType())
}

// ---- Overview aggregation ----

func sampleEntries() []session.Entry {
	mkPtr := func(s string) *string { return &s }

	ts1, _ := time.Parse(time.RFC3339, "2026-03-23T13:57:52.000Z")
	ts2, _ := time.Parse(time.RFC3339, "2026-03-23T14:00:00.000Z")
	ts3, _ := time.Parse(time.RFC3339, "2026-03-23T14:02:00.000Z")
	ts4, _ := time.Parse(time.RFC3339, "2026-03-23T14:04:00.000Z")

	return []session.Entry{
		{
			Type:      "permission-mode",
			UUID:      "perm1",
			Timestamp: ts1,
			SessionID: "sid",
		},
		{
			Type:      "user",
			UUID:      "u1",
			Timestamp: ts2,
			SessionID: "sid",
			IsMeta:    false,
			Message:   &session.Message{Role: "user", Content: []byte(`"Hello"`)},
		},
		{
			// Streaming chunk — same message.id, stop_reason null, incomplete.
			Type:      "assistant",
			UUID:      "a1-chunk1",
			Timestamp: ts3,
			SessionID: "sid",
			RequestID: "req1",
			Message: &session.Message{
				Role:       "assistant",
				Model:      "claude-opus-4-6",
				ID:         "msg_001",
				StopReason: nil,
				Content:    []byte(`[{"type":"text","text":"Let me"}]`),
				Usage:      &session.TokenUsage{InputTokens: 100, OutputTokens: 5},
			},
		},
		{
			// Final chunk — same message.id, stop_reason set, full content.
			Type:      "assistant",
			UUID:      "a1-chunk2",
			Timestamp: ts3,
			SessionID: "sid",
			RequestID: "req1",
			Message: &session.Message{
				Role:       "assistant",
				Model:      "claude-opus-4-6",
				ID:         "msg_001",
				StopReason: mkPtr("tool_use"),
				Content: []byte(`[
					{"type":"text","text":"I will help."},
					{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/tmp/x"}}
				]`),
				Usage: &session.TokenUsage{InputTokens: 1000, OutputTokens: 50},
			},
		},
		{
			Type:      "user",
			UUID:      "u2",
			Timestamp: ts3,
			SessionID: "sid",
			IsMeta:    false,
			Message: &session.Message{
				Role:    "user",
				Content: []byte(`[{"type":"tool_result","tool_use_id":"toolu_1","content":"file content","is_error":false}]`),
			},
		},
		{
			Type:      "assistant",
			UUID:      "a2",
			Timestamp: ts4,
			SessionID: "sid",
			RequestID: "req2",
			Message: &session.Message{
				Role:       "assistant",
				Model:      "claude-opus-4-6",
				ID:         "msg_002",
				StopReason: mkPtr("end_turn"),
				Content:    []byte(`[{"type":"text","text":"Done."}]`),
				Usage:      &session.TokenUsage{InputTokens: 500, OutputTokens: 30},
			},
		},
		{
			Type:       "system",
			UUID:       "sys1",
			Subtype:    "turn_duration",
			DurationMs: 60000,
			Timestamp:  ts4,
			SessionID:  "sid",
		},
		{
			Type:      "progress",
			UUID:      "prog1",
			Timestamp: ts3,
			SessionID: "sid",
			Data:      []byte(`{"type":"agent_progress","agentId":"agent-abc","prompt":"Do something"}`),
		},
	}
}

func TestOverviewFromEntries_MessageCounts(t *testing.T) {
	entries := sampleEntries()
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	// 2 non-meta user messages
	assert.Equal(t, 2, ov.MessageCount.User)
	// 2 deduplicated assistant messages (msg_001 + msg_002)
	assert.Equal(t, 2, ov.MessageCount.Assistant)
	assert.Equal(t, 4, ov.MessageCount.Total)
}

func TestOverviewFromEntries_ToolSequence(t *testing.T) {
	entries := sampleEntries()
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	// Only one tool_use block in the deduped set: "Read" from msg_001
	assert.Equal(t, []string{"Read"}, ov.Tools)
}

func TestOverviewFromEntries_TokenTotals(t *testing.T) {
	entries := sampleEntries()
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	// msg_001 final chunk: 1000 in, 50 out
	// msg_002: 500 in, 30 out
	// The streaming chunk (msg_001 chunk1) is deduplicated — not counted.
	assert.Equal(t, 1500, ov.TokenUsage.InputTokens)
	assert.Equal(t, 80, ov.TokenUsage.OutputTokens)
}

func TestOverviewFromEntries_SubagentCount(t *testing.T) {
	entries := sampleEntries()
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	assert.Equal(t, 1, ov.SubagentCount)
}

func TestOverviewFromEntries_Duration_FromSystemEntries(t *testing.T) {
	entries := sampleEntries()
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	// turn_duration 60000ms = 60s
	assert.InDelta(t, 60.0, ov.DurationSecs, 0.001)
}

func TestOverviewFromEntries_Duration_Fallback(t *testing.T) {
	// Entries without any system/turn_duration — fall back to first/last timestamp.
	ts1, _ := time.Parse(time.RFC3339, "2026-03-23T13:00:00.000Z")
	ts2, _ := time.Parse(time.RFC3339, "2026-03-23T13:02:30.000Z")
	mkPtr := func(s string) *string { return &s }
	entries := []session.Entry{
		{
			Type:      "user",
			UUID:      "u1",
			Timestamp: ts1,
			SessionID: "sid",
			Message:   &session.Message{Role: "user", Content: []byte(`"Hi"`)},
		},
		{
			Type:      "assistant",
			UUID:      "a1",
			Timestamp: ts2,
			SessionID: "sid",
			Message: &session.Message{
				Role:       "assistant",
				Model:      "claude-opus-4-6",
				ID:         "msg_x",
				StopReason: mkPtr("end_turn"),
				Content:    []byte(`[{"type":"text","text":"Hello!"}]`),
				Usage:      &session.TokenUsage{InputTokens: 10, OutputTokens: 5},
			},
		},
	}
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	assert.InDelta(t, 150.0, ov.DurationSecs, 0.001) // 2m30s = 150s
}

func TestOverviewFromEntries_Model(t *testing.T) {
	entries := sampleEntries()
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-4-6", ov.Model)
}

func TestOverviewFromEntries_DeduplicatesStreamingEntries(t *testing.T) {
	// Two streaming chunks with the same message.id — only the last should count.
	mkPtr := func(s string) *string { return &s }
	ts1, _ := time.Parse(time.RFC3339, "2026-03-23T13:00:00.000Z")
	entries := []session.Entry{
		{
			Type:      "user",
			UUID:      "u1",
			Timestamp: ts1,
			SessionID: "sid",
			IsMeta:    false,
			Message:   &session.Message{Role: "user", Content: []byte(`"Hi"`)},
		},
		{
			Type:      "assistant",
			UUID:      "a1",
			Timestamp: ts1,
			SessionID: "sid",
			Message: &session.Message{
				Role:       "assistant",
				Model:      "claude-opus-4-6",
				ID:         "msg_stream",
				StopReason: nil,
				Content:    []byte(`[{"type":"text","text":"Hello"}]`),
				Usage:      &session.TokenUsage{InputTokens: 999, OutputTokens: 999},
			},
		},
		{
			Type:      "assistant",
			UUID:      "a2",
			Timestamp: ts1,
			SessionID: "sid",
			Message: &session.Message{
				Role:       "assistant",
				Model:      "claude-opus-4-6",
				ID:         "msg_stream",
				StopReason: mkPtr("end_turn"),
				Content:    []byte(`[{"type":"text","text":"Hello world!"}]`),
				Usage:      &session.TokenUsage{InputTokens: 42, OutputTokens: 7},
			},
		},
	}
	ov, err := session.OverviewFromEntries("sid", entries)
	require.NoError(t, err)
	assert.Equal(t, 1, ov.MessageCount.Assistant)
	assert.Equal(t, 42, ov.TokenUsage.InputTokens)
	assert.Equal(t, 7, ov.TokenUsage.OutputTokens)
}
