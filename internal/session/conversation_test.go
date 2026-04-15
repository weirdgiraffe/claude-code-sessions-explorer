package session_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weirdgiraffe/claude-code-sessions-explorer/internal/session"
)

// conversationEntries returns a representative slice of entries for conversation tests.
// Layout:
//
//	user  "Hello"             (ts1)
//	asst  "I will help." + tool_use Read  (ts2, msg_001, two streaming chunks)
//	user  tool_result for Read             (ts3)
//	asst  "Done."                          (ts4, msg_002)
//	user  meta message — excluded          (ts5)
func conversationEntries() []session.Entry {
	mkPtr := func(s string) *string { return &s }

	ts1, _ := time.Parse(time.RFC3339, "2026-01-01T10:00:00Z")
	ts2, _ := time.Parse(time.RFC3339, "2026-01-01T10:00:01Z")
	ts3, _ := time.Parse(time.RFC3339, "2026-01-01T10:00:02Z")
	ts4, _ := time.Parse(time.RFC3339, "2026-01-01T10:00:03Z")
	ts5, _ := time.Parse(time.RFC3339, "2026-01-01T10:00:04Z")

	return []session.Entry{
		{
			Type:      "user",
			UUID:      "u1",
			Timestamp: ts1,
			SessionID: "sid",
			IsMeta:    false,
			Message:   &session.Message{Role: "user", Content: []byte(`"Hello"`)},
		},
		{
			// Streaming chunk 1 — incomplete, partial text.
			Type:      "assistant",
			UUID:      "a1-chunk1",
			Timestamp: ts2,
			SessionID: "sid",
			Message: &session.Message{
				Role:       "assistant",
				Model:      "claude-opus-4-6",
				ID:         "msg_001",
				StopReason: nil,
				Content:    []byte(`[{"type":"text","text":"I will"}]`),
				Usage:      &session.TokenUsage{InputTokens: 100, OutputTokens: 3},
			},
		},
		{
			// Streaming chunk 2 — final, full content + tool_use.
			Type:      "assistant",
			UUID:      "a1-chunk2",
			Timestamp: ts2,
			SessionID: "sid",
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
				Content: []byte(`[{"type":"tool_result","tool_use_id":"toolu_1","content":"file content here","is_error":false}]`),
			},
		},
		{
			Type:      "assistant",
			UUID:      "a2",
			Timestamp: ts4,
			SessionID: "sid",
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
			// Meta message — must be excluded.
			Type:      "user",
			UUID:      "u3",
			Timestamp: ts5,
			SessionID: "sid",
			IsMeta:    true,
			Message:   &session.Message{Role: "user", Content: []byte(`"<local-command-caveat>...</local-command-caveat>"`)},
		},
	}
}

func TestConversationFromEntries_ChronologicalOrder(t *testing.T) {
	entries := conversationEntries()
	opts := session.ConversationOptions{
		IncludeToolResults: true,
	}
	msgs, err := session.ConversationFromEntries(entries, opts)
	require.NoError(t, err)

	// Expect: user "Hello", assistant "I will help.", user tool_result, assistant "Done."
	require.Len(t, msgs, 4)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Hello", msgs[0].Content)

	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "I will help.", msgs[1].Content)

	assert.Equal(t, "user", msgs[2].Role)
	assert.Contains(t, msgs[2].Content, "file content here")

	assert.Equal(t, "assistant", msgs[3].Role)
	assert.Equal(t, "Done.", msgs[3].Content)
}

func TestConversationFromEntries_MetaMessagesExcluded(t *testing.T) {
	entries := conversationEntries()
	opts := session.ConversationOptions{IncludeToolResults: true}
	msgs, err := session.ConversationFromEntries(entries, opts)
	require.NoError(t, err)

	for _, m := range msgs {
		assert.NotContains(t, m.Content, "local-command-caveat", "meta message must not appear")
	}
}

func TestConversationFromEntries_StreamingDeduplication(t *testing.T) {
	entries := conversationEntries()
	opts := session.ConversationOptions{}
	msgs, err := session.ConversationFromEntries(entries, opts)
	require.NoError(t, err)

	// msg_001 has two chunks; only the final one ("I will help.") should appear.
	assistantMsgs := filterRole(msgs, "assistant")
	require.Len(t, assistantMsgs, 2)
	assert.Equal(t, "I will help.", assistantMsgs[0].Content)
	assert.Equal(t, "Done.", assistantMsgs[1].Content)
}

func TestConversationFromEntries_RoleFilterUser(t *testing.T) {
	entries := conversationEntries()
	opts := session.ConversationOptions{Role: "user"}
	msgs, err := session.ConversationFromEntries(entries, opts)
	require.NoError(t, err)

	for _, m := range msgs {
		assert.Equal(t, "user", m.Role)
	}
}

func TestConversationFromEntries_RoleFilterAssistant(t *testing.T) {
	entries := conversationEntries()
	opts := session.ConversationOptions{Role: "assistant"}
	msgs, err := session.ConversationFromEntries(entries, opts)
	require.NoError(t, err)

	for _, m := range msgs {
		assert.Equal(t, "assistant", m.Role)
	}
	assert.Len(t, msgs, 2)
}

func TestConversationFromEntries_ToolResultsIncluded(t *testing.T) {
	entries := conversationEntries()
	opts := session.ConversationOptions{IncludeToolResults: true}
	msgs, err := session.ConversationFromEntries(entries, opts)
	require.NoError(t, err)

	userMsgs := filterRole(msgs, "user")
	// u1 is "Hello", u2 is a tool result.
	require.Len(t, userMsgs, 2)
	assert.Contains(t, userMsgs[1].Content, "file content here")
}

func TestConversationFromEntries_ToolResultsExcluded(t *testing.T) {
	entries := conversationEntries()
	opts := session.ConversationOptions{IncludeToolResults: false}
	msgs, err := session.ConversationFromEntries(entries, opts)
	require.NoError(t, err)

	// u2 is purely a tool result entry; with IncludeToolResults=false it produces
	// no visible content and must be omitted entirely.
	userMsgs := filterRole(msgs, "user")
	require.Len(t, userMsgs, 1)
	assert.Equal(t, "Hello", userMsgs[0].Content)
}

func TestConversationFromEntries_ToolResultTruncation(t *testing.T) {
	longContent := strings.Repeat("x", 600)
	entry := session.Entry{
		Type:      "user",
		UUID:      "u1",
		Timestamp: time.Now(),
		SessionID: "sid",
		Message: &session.Message{
			Role:    "user",
			Content: []byte(`[{"type":"tool_result","tool_use_id":"tid","content":"` + longContent + `","is_error":false}]`),
		},
	}
	opts := session.ConversationOptions{IncludeToolResults: true, Full: false}
	msgs, err := session.ConversationFromEntries([]session.Entry{entry}, opts)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	// Should be truncated to 500 chars + "..."
	assert.True(t, len(msgs[0].Content) < 600, "content should be truncated")
	assert.Contains(t, msgs[0].Content, "...")
}

func TestConversationFromEntries_ToolResultFullDisablesTruncation(t *testing.T) {
	longContent := strings.Repeat("x", 600)
	entry := session.Entry{
		Type:      "user",
		UUID:      "u1",
		Timestamp: time.Now(),
		SessionID: "sid",
		Message: &session.Message{
			Role:    "user",
			Content: []byte(`[{"type":"tool_result","tool_use_id":"tid","content":"` + longContent + `","is_error":false}]`),
		},
	}
	opts := session.ConversationOptions{IncludeToolResults: true, Full: true}
	msgs, err := session.ConversationFromEntries([]session.Entry{entry}, opts)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, longContent)
}

func TestConversationFromEntries_ThinkingExcludedByDefault(t *testing.T) {
	entry := session.Entry{
		Type:      "assistant",
		UUID:      "a1",
		Timestamp: time.Now(),
		SessionID: "sid",
		Message: &session.Message{
			Role: "assistant",
			ID:   "msg_t",
			Content: []byte(`[
				{"type":"thinking","thinking":"secret thoughts","signature":"SIG"},
				{"type":"text","text":"visible text"}
			]`),
		},
	}
	opts := session.ConversationOptions{IncludeThinking: false}
	msgs, err := session.ConversationFromEntries([]session.Entry{entry}, opts)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "visible text", msgs[0].Content)
	assert.NotContains(t, msgs[0].Content, "secret thoughts")
}

func TestConversationFromEntries_ThinkingIncludedWhenRequested(t *testing.T) {
	entry := session.Entry{
		Type:      "assistant",
		UUID:      "a1",
		Timestamp: time.Now(),
		SessionID: "sid",
		Message: &session.Message{
			Role: "assistant",
			ID:   "msg_t",
			Content: []byte(`[
				{"type":"thinking","thinking":"secret thoughts","signature":"SIG"},
				{"type":"text","text":"visible text"}
			]`),
		},
	}
	opts := session.ConversationOptions{IncludeThinking: true}
	msgs, err := session.ConversationFromEntries([]session.Entry{entry}, opts)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "secret thoughts")
	assert.Contains(t, msgs[0].Content, "visible text")
}

func TestConversationFromEntries_ToolUseBlocksNotInOutput(t *testing.T) {
	entry := session.Entry{
		Type:      "assistant",
		UUID:      "a1",
		Timestamp: time.Now(),
		SessionID: "sid",
		Message: &session.Message{
			Role: "assistant",
			ID:   "msg_x",
			Content: []byte(`[
				{"type":"text","text":"Let me run this."},
				{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}
			]`),
		},
	}
	opts := session.ConversationOptions{}
	msgs, err := session.ConversationFromEntries([]session.Entry{entry}, opts)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "Let me run this.", msgs[0].Content)
	assert.NotContains(t, msgs[0].Content, "Bash")
	assert.NotContains(t, msgs[0].Content, "toolu_1")
}

func TestConversationFromEntries_EmptyEntries(t *testing.T) {
	msgs, err := session.ConversationFromEntries(nil, session.ConversationOptions{})
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestConversationFromEntries_AssistantOnlyTextBlockProducesMessage(t *testing.T) {
	// An assistant entry with only tool_use blocks (no text) should produce
	// no conversation message.
	entry := session.Entry{
		Type:      "assistant",
		UUID:      "a1",
		Timestamp: time.Now(),
		SessionID: "sid",
		Message: &session.Message{
			Role: "assistant",
			ID:   "msg_y",
			Content: []byte(`[
				{"type":"tool_use","id":"toolu_2","name":"Write","input":{}}
			]`),
		},
	}
	opts := session.ConversationOptions{}
	msgs, err := session.ConversationFromEntries([]session.Entry{entry}, opts)
	require.NoError(t, err)
	assert.Empty(t, msgs, "entry with only tool_use blocks should produce no message")
}

// filterRole returns only messages with the given role.
func filterRole(msgs []session.ConversationMessage, role string) []session.ConversationMessage {
	var out []session.ConversationMessage
	for _, m := range msgs {
		if m.Role == role {
			out = append(out, m)
		}
	}
	return out
}
