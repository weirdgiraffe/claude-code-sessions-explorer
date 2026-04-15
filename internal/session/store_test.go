package session_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weirdgiraffe/claude-code-sessions-explorer/internal/session"
)

// writeJSONL writes a slice of Entry values as JSONL to the given file path.
func writeJSONL(t *testing.T, path string, entries []session.Entry) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		require.NoError(t, enc.Encode(e))
	}
}

// newEntry builds a minimal Entry with the given type, uuid, and timestamp.
func newEntry(typ, uuid string, ts time.Time) session.Entry {
	return session.Entry{
		Type:      typ,
		UUID:      uuid,
		SessionID: "test-session",
		Timestamp: ts,
	}
}

func TestList_Empty(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "projects"), 0755))

	sessions, err := session.List(dir, nil)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestList_SingleSession(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "-Users-test-myproject")
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	sessionID := "abc12345-0000-0000-0000-000000000000"
	start := time.Date(2026, 3, 23, 13, 57, 52, 0, time.UTC)
	end := start.Add(5 * time.Minute)

	userEntry := newEntry("user", "u1", start)
	userEntry.Slug = "groovy-sauteeing-falcon"
	assistantEntry := newEntry("assistant", "u2", end)
	assistantMsg := &session.Message{
		Role:  "assistant",
		Model: "claude-opus-4-6",
	}
	assistantMsg.Content = json.RawMessage(`[]`)
	assistantEntry.Message = assistantMsg

	writeJSONL(t, filepath.Join(projectDir, sessionID+".jsonl"), []session.Entry{
		userEntry,
		assistantEntry,
	})

	sessions, err := session.List(dir, nil)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	s := sessions[0]
	assert.Equal(t, sessionID, s.ID)
	assert.Equal(t, "-Users-test-myproject", s.ProjectSlug)
	assert.Equal(t, "groovy-sauteeing-falcon", s.Slug)
	assert.Equal(t, "claude-opus-4-6", s.Model)
	assert.Equal(t, 2, s.MessageCount)
	assert.True(t, s.StartedAt.Equal(start))
	assert.True(t, s.EndedAt.Equal(end))
	assert.InDelta(t, 300.0, s.DurationSecs, 0.01)
}

func TestList_FilterByProjectSlug(t *testing.T) {
	dir := t.TempDir()
	for _, slug := range []string{"-project-alpha", "-project-beta"} {
		projectDir := filepath.Join(dir, "projects", slug)
		require.NoError(t, os.MkdirAll(projectDir, 0755))
		e := newEntry("user", "u1", time.Now())
		writeJSONL(t, filepath.Join(projectDir, "sess-0000-0000-0000-0000-000000000000.jsonl"), []session.Entry{e})
	}

	sessions, err := session.List(dir, &session.ListOptions{ProjectSlug: "-project-alpha"})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "-project-alpha", sessions[0].ProjectSlug)
}

func TestList_FilterBySince(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "-project-test")
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Old session — before cutoff
	old := newEntry("user", "u1", cutoff.Add(-24*time.Hour))
	writeJSONL(t, filepath.Join(projectDir, "old-sess-0000-0000-0000-00000000.jsonl"), []session.Entry{old})

	// New session — after cutoff
	fresh := newEntry("user", "u2", cutoff.Add(time.Hour))
	writeJSONL(t, filepath.Join(projectDir, "new-sess-0000-0000-0000-00000000.jsonl"), []session.Entry{fresh})

	sessions, err := session.List(dir, &session.ListOptions{Since: cutoff})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.True(t, sessions[0].StartedAt.After(cutoff) || sessions[0].StartedAt.Equal(cutoff))
}

func TestList_Limit(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "-project-test")
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		e := newEntry("user", "u1", base.Add(time.Duration(i)*time.Hour))
		name := filepath.Join(projectDir, filepath.Base(e.UUID)+"-"+string(rune('0'+i))+".jsonl")
		writeJSONL(t, name, []session.Entry{e})
	}

	sessions, err := session.List(dir, &session.ListOptions{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, sessions, 3)
}

func TestList_SortedMostRecentFirst(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "-project-test")
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	times := []time.Time{
		base.Add(2 * time.Hour),
		base,
		base.Add(time.Hour),
	}
	for i, ts := range times {
		e := newEntry("user", "u1", ts)
		name := filepath.Join(projectDir, "sess-000"+string(rune('0'+i))+"-0000-0000-0000-000000000000.jsonl")
		writeJSONL(t, name, []session.Entry{e})
	}

	sessions, err := session.List(dir, nil)
	require.NoError(t, err)
	require.Len(t, sessions, 3)
	// Most recent first
	assert.True(t, sessions[0].StartedAt.After(sessions[1].StartedAt))
	assert.True(t, sessions[1].StartedAt.After(sessions[2].StartedAt))
}

func TestResolve_ExactPrefix(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "-project-test")
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	sessionID := "abcdef12-0000-0000-0000-000000000000"
	e := newEntry("user", "u1", time.Now())
	writeJSONL(t, filepath.Join(projectDir, sessionID+".jsonl"), []session.Entry{e})

	s, err := session.Resolve(dir, "abcdef12")
	require.NoError(t, err)
	assert.Equal(t, sessionID, s.ID)
}

func TestResolve_NotFound(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "projects"), 0755))

	_, err := session.Resolve(dir, "deadbeef")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session found")
}

func TestResolve_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "projects", "-project-test")
	require.NoError(t, os.MkdirAll(projectDir, 0755))

	for _, id := range []string{
		"abcdef12-0000-0000-0000-000000000001",
		"abcdef12-0000-0000-0000-000000000002",
	} {
		e := newEntry("user", "u1", time.Now())
		writeJSONL(t, filepath.Join(projectDir, id+".jsonl"), []session.Entry{e})
	}

	_, err := session.Resolve(dir, "abcdef12")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestResolve_EmptyPrefix(t *testing.T) {
	dir := t.TempDir()
	_, err := session.Resolve(dir, "")
	assert.Error(t, err)
}

func TestClaudeHome_EnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_HOME", dir)
	h, err := session.ClaudeHome()
	require.NoError(t, err)
	assert.Equal(t, dir, h)
}
