package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeToolUseEntry returns an assistant entry with the given tool_use blocks.
func makeToolUseEntry(msgID string, tools []ToolUseBlock) Entry {
	type rawBlock struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	blocks := make([]rawBlock, len(tools))
	for i, tu := range tools {
		blocks[i] = rawBlock{
			Type:  "tool_use",
			ID:    tu.ID,
			Name:  tu.Name,
			Input: tu.Input,
		}
	}
	content, _ := json.Marshal(blocks)
	return Entry{
		Type: "assistant",
		UUID: msgID,
		Message: &Message{
			Role:    "assistant",
			ID:      msgID,
			Content: json.RawMessage(content),
		},
		Timestamp: time.Now(),
	}
}

// makeToolResultEntry returns a user entry with the given tool_result blocks.
func makeToolResultEntry(results []ToolResultBlock) Entry {
	type rawBlock struct {
		Type      string          `json:"type"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
		IsError   bool            `json:"is_error,omitempty"`
	}
	blocks := make([]rawBlock, len(results))
	for i, tr := range results {
		blocks[i] = rawBlock{
			Type:      "tool_result",
			ToolUseID: tr.ToolUseID,
			Content:   tr.Content,
			IsError:   tr.IsError,
		}
	}
	content, _ := json.Marshal(blocks)
	return Entry{
		Type: "user",
		Message: &Message{
			Role:    "user",
			Content: json.RawMessage(content),
		},
		Timestamp: time.Now(),
	}
}

func TestToolsFromEntries_BasicPairing(t *testing.T) {
	input := json.RawMessage(`{"cmd":"ls"}`)
	result := json.RawMessage(`"file1.txt\nfile2.txt"`)

	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Bash", Input: input},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: result},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Equal(t, "Bash", calls[0].Name)
	assert.Equal(t, `{"cmd":"ls"}`, calls[0].Input)
	assert.Equal(t, "file1.txt\nfile2.txt", calls[0].Result)
	assert.False(t, calls[0].IsError)
}

func TestToolsFromEntries_MultipleToolsPerTurn(t *testing.T) {
	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Read", Input: json.RawMessage(`{"path":"/a"}`)},
			{ID: "tu2", Name: "Bash", Input: json.RawMessage(`{"cmd":"pwd"}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(`"content of a"`)},
			{ToolUseID: "tu2", Content: json.RawMessage(`"/home/user"`)},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)
	require.Len(t, calls, 2)
	assert.Equal(t, "Read", calls[0].Name)
	assert.Equal(t, "content of a", calls[0].Result)
	assert.Equal(t, "Bash", calls[1].Name)
	assert.Equal(t, "/home/user", calls[1].Result)
}

func TestToolsFromEntries_MultipleTurns(t *testing.T) {
	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Glob", Input: json.RawMessage(`{"pattern":"*.go"}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(`"main.go"`)},
		}),
		makeToolUseEntry("msg2", []ToolUseBlock{
			{ID: "tu2", Name: "Read", Input: json.RawMessage(`{"path":"main.go"}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu2", Content: json.RawMessage(`"package main"`)},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)
	require.Len(t, calls, 2)
	assert.Equal(t, "Glob", calls[0].Name)
	assert.Equal(t, "Read", calls[1].Name)
}

func TestToolsFromEntries_FilterByName(t *testing.T) {
	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Bash", Input: json.RawMessage(`{"cmd":"ls"}`)},
			{ID: "tu2", Name: "Read", Input: json.RawMessage(`{"path":"/x"}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(`"out"`)},
			{ToolUseID: "tu2", Content: json.RawMessage(`"data"`)},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{Name: "Bash"})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Equal(t, "Bash", calls[0].Name)
}

func TestToolsFromEntries_SequenceOnly(t *testing.T) {
	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Glob", Input: json.RawMessage(`{}`)},
			{ID: "tu2", Name: "Bash", Input: json.RawMessage(`{}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(`"x"`)},
			{ToolUseID: "tu2", Content: json.RawMessage(`"y"`)},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)

	seq := ToolSequence(calls)
	assert.Equal(t, []string{"Glob", "Bash"}, seq)
}

func TestToolsFromEntries_Truncation(t *testing.T) {
	longVal := make([]byte, 600)
	for i := range longVal {
		longVal[i] = 'a'
	}
	longJSON, _ := json.Marshal(string(longVal))
	longResult, _ := json.Marshal(string(longVal))

	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Bash", Input: json.RawMessage(`{"cmd":"` + string(longVal) + `"}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(longResult)},
		}),
	}
	_ = longJSON

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	// With truncation: input and result must not exceed 500 + 3 chars ("...")
	assert.LessOrEqual(t, len([]rune(calls[0].Input)), defaultTruncateLen+3)
	assert.LessOrEqual(t, len([]rune(calls[0].Result)), defaultTruncateLen+3)
	assert.True(t, len([]rune(calls[0].Input)) > 0)
}

func TestToolsFromEntries_FullDisablesTruncation(t *testing.T) {
	longVal := make([]byte, 600)
	for i := range longVal {
		longVal[i] = 'b'
	}
	longResult, _ := json.Marshal(string(longVal))

	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Bash", Input: json.RawMessage(`{"cmd":"short"}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(longResult)},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{Full: true})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	// Result must be the full 600-char string, not truncated.
	assert.Equal(t, string(longVal), calls[0].Result)
}

func TestToolsFromEntries_IsError(t *testing.T) {
	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Bash", Input: json.RawMessage(`{}`)},
		}),
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(`"command not found"`), IsError: true},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.True(t, calls[0].IsError)
	assert.Equal(t, "command not found", calls[0].Result)
}

func TestToolsFromEntries_StreamingDeduplication(t *testing.T) {
	// Two entries with the same message.id (streaming chunks): the second
	// (more complete) one should be used for tool_use extraction.
	input := json.RawMessage(`{"cmd":"echo"}`)
	partialContent, _ := json.Marshal([]map[string]any{
		{"type": "tool_use", "id": "tu1", "name": "Bash", "input": json.RawMessage(`{}`)},
	})
	fullContent, _ := json.Marshal([]map[string]any{
		{"type": "tool_use", "id": "tu1", "name": "Bash", "input": input},
	})

	entries := []Entry{
		{
			Type: "assistant",
			UUID: "e1",
			Message: &Message{
				Role:    "assistant",
				ID:      "shared-msg-id",
				Content: json.RawMessage(partialContent),
			},
			Timestamp: time.Now(),
		},
		{
			Type: "assistant",
			UUID: "e2",
			Message: &Message{
				Role:    "assistant",
				ID:      "shared-msg-id",
				Content: json.RawMessage(fullContent),
			},
			Timestamp: time.Now(),
		},
		makeToolResultEntry([]ToolResultBlock{
			{ToolUseID: "tu1", Content: json.RawMessage(`"hello"`)},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)
	// Only one tool call despite two streaming entries.
	require.Len(t, calls, 1)
	assert.Equal(t, "Bash", calls[0].Name)
	assert.Equal(t, `{"cmd":"echo"}`, calls[0].Input)
}

func TestToolsFromEntries_NoResult(t *testing.T) {
	// Tool use without a matching result (e.g. session ended mid-turn).
	entries := []Entry{
		makeToolUseEntry("msg1", []ToolUseBlock{
			{ID: "tu1", Name: "Bash", Input: json.RawMessage(`{}`)},
		}),
	}

	calls, err := ToolsFromEntries(entries, ToolsOptions{})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Equal(t, "Bash", calls[0].Name)
	assert.Equal(t, "", calls[0].Result)
	assert.False(t, calls[0].IsError)
}

func TestToolsFromEntries_Empty(t *testing.T) {
	calls, err := ToolsFromEntries([]Entry{}, ToolsOptions{})
	require.NoError(t, err)
	assert.Empty(t, calls)
}
