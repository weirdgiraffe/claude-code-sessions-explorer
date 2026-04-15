package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSubagentFixture creates a temporary Claude home directory with one
// session and a set of subagents for use in tests. It returns the claudeHome
// path and the full session ID.
func buildSubagentFixture(t *testing.T) (claudeHome, sessionID string) {
	t.Helper()

	claudeHome = t.TempDir()
	sessionID = "aabbccdd-0000-0000-0000-000000000001"
	projectSlug := "-tmp-testproject"

	projectDir := filepath.Join(claudeHome, "projects", projectSlug)
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Write a minimal main session JSONL so Resolve can find the session.
	mainJSONL := filepath.Join(projectDir, sessionID+".jsonl")
	writeEntry(t, mainJSONL, map[string]any{
		"type":      "user",
		"uuid":      "u1",
		"sessionId": sessionID,
		"timestamp": "2026-01-01T00:00:00Z",
		"message":   map[string]any{"role": "user", "content": "hello"},
	})

	// Create the subagents directory.
	subagentsDir := filepath.Join(projectDir, sessionID, "subagents")
	require.NoError(t, os.MkdirAll(subagentsDir, 0o755))

	// Subagent 1: agent-alpha001
	writeAgentJSONL(t, subagentsDir, "alpha001", 2)
	writeAgentMeta(t, subagentsDir, "alpha001", "Explore", "Explore the project")

	// Subagent 2: agent-beta002
	writeAgentJSONL(t, subagentsDir, "beta002", 4)
	writeAgentMeta(t, subagentsDir, "beta002", "Write", "Write some code")

	// Nested subagent inside beta002: agent-gamma003
	nestedDir := filepath.Join(subagentsDir, "agent-beta002", "subagents")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	writeAgentJSONL(t, nestedDir, "gamma003", 1)
	writeAgentMeta(t, nestedDir, "gamma003", "Bash", "Run a command")

	return claudeHome, sessionID
}

func writeEntry(t *testing.T, path string, data map[string]any) {
	t.Helper()
	b, err := json.Marshal(data)
	require.NoError(t, err)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	require.NoError(t, err)
}

func writeAgentJSONL(t *testing.T, dir, agentID string, msgCount int) {
	t.Helper()
	path := filepath.Join(dir, "agent-"+agentID+".jsonl")
	for i := range msgCount {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		writeEntry(t, path, map[string]any{
			"type":      role,
			"uuid":      agentID + "-" + string(rune('a'+i)),
			"timestamp": "2026-01-01T00:01:00Z",
			"message":   map[string]any{"role": role, "content": "msg"},
		})
	}
}

func writeAgentMeta(t *testing.T, dir, agentID, agentType, description string) {
	t.Helper()
	path := filepath.Join(dir, "agent-"+agentID+".meta.json")
	b, err := json.Marshal(map[string]string{
		"agentType":   agentType,
		"description": description,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o644))
}

func TestListSubagents_basic(t *testing.T) {
	claudeHome, sessionID := buildSubagentFixture(t)

	agents, err := ListSubagents(claudeHome, sessionID)
	require.NoError(t, err)

	// Expect three agents: alpha001, beta002, gamma003 (nested).
	assert.Len(t, agents, 3)

	byID := make(map[string]SubagentInfo)
	for _, a := range agents {
		byID[a.ID] = a
	}

	alpha := byID["alpha001"]
	assert.Equal(t, "Explore", alpha.Type)
	assert.Equal(t, "Explore the project", alpha.Description)
	assert.Equal(t, 2, alpha.MessageCount)

	beta := byID["beta002"]
	assert.Equal(t, "Write", beta.Type)
	assert.Equal(t, "Write some code", beta.Description)
	assert.Equal(t, 4, beta.MessageCount)

	gamma := byID["gamma003"]
	assert.Equal(t, "Bash", gamma.Type)
	assert.Equal(t, "Run a command", gamma.Description)
	assert.Equal(t, 1, gamma.MessageCount)
}

func TestListSubagents_prefix(t *testing.T) {
	claudeHome, _ := buildSubagentFixture(t)

	// Resolve by prefix of session ID.
	agents, err := ListSubagents(claudeHome, "aabbccdd")
	require.NoError(t, err)
	assert.Len(t, agents, 3)
}

func TestListSubagents_noSubagents(t *testing.T) {
	claudeHome := t.TempDir()
	sessionID := "cccccccc-0000-0000-0000-000000000001"
	projectSlug := "-tmp-empty"

	projectDir := filepath.Join(claudeHome, "projects", projectSlug)
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	mainJSONL := filepath.Join(projectDir, sessionID+".jsonl")
	writeEntry(t, mainJSONL, map[string]any{
		"type":      "user",
		"uuid":      "u1",
		"sessionId": sessionID,
		"timestamp": "2026-01-01T00:00:00Z",
		"message":   map[string]any{"role": "user", "content": "hello"},
	})

	agents, err := ListSubagents(claudeHome, sessionID)
	require.NoError(t, err)
	assert.NotNil(t, agents)
	assert.Empty(t, agents)
}

func TestResolveSubagent_exact(t *testing.T) {
	claudeHome, sessionID := buildSubagentFixture(t)

	path, err := ResolveSubagent(claudeHome, sessionID, "alpha001")
	require.NoError(t, err)
	assert.Contains(t, path, "agent-alpha001.jsonl")
}

func TestResolveSubagent_prefix(t *testing.T) {
	claudeHome, sessionID := buildSubagentFixture(t)

	path, err := ResolveSubagent(claudeHome, sessionID, "alpha")
	require.NoError(t, err)
	assert.Contains(t, path, "agent-alpha001.jsonl")
}

func TestResolveSubagent_nested(t *testing.T) {
	claudeHome, sessionID := buildSubagentFixture(t)

	path, err := ResolveSubagent(claudeHome, sessionID, "gamma")
	require.NoError(t, err)
	assert.Contains(t, path, "agent-gamma003.jsonl")
}

func TestResolveSubagent_ambiguous(t *testing.T) {
	claudeHome, sessionID := buildSubagentFixture(t)

	// Both "alpha001" and "beta002" share no common prefix but "a" matches only alpha.
	// Use a prefix that matches both beta002 and a hypothetical agent to test ambiguity.
	// Instead, add a second alpha agent.
	projectSlug := "-tmp-testproject"
	subagentsDir := filepath.Join(claudeHome, "projects", projectSlug, sessionID, "subagents")
	writeAgentJSONL(t, subagentsDir, "alpha999", 1)
	writeAgentMeta(t, subagentsDir, "alpha999", "Read", "Read files")

	_, err := ResolveSubagent(claudeHome, sessionID, "alpha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestResolveSubagent_notFound(t *testing.T) {
	claudeHome, sessionID := buildSubagentFixture(t)

	_, err := ResolveSubagent(claudeHome, sessionID, "zzz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no subagent found")
}

func TestResolveSubagent_emptyPrefix(t *testing.T) {
	claudeHome, sessionID := buildSubagentFixture(t)

	_, err := ResolveSubagent(claudeHome, sessionID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}
