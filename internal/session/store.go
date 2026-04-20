package session

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ClaudeHome returns the root Claude Code data directory, using CLAUDE_HOME
// if set, otherwise defaulting to ~/.claude/.
func ClaudeHome() (string, error) {
	if h := os.Getenv("CLAUDE_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// ListOptions controls which sessions are returned by List.
type ListOptions struct {
	ProjectSlug string    // filter to a specific project slug (empty = all)
	Since       time.Time // exclude sessions that started before this time (zero = no filter)
	Until       time.Time // exclude sessions that started on or after this time (zero = no filter)
	Limit       int       // maximum number of sessions to return (0 = all)
}

// List scans ~/.claude/projects/ (or CLAUDE_HOME) for sessions and returns
// aggregated metadata, sorted by most-recent-first. Options may be nil.
func List(claudeHome string, opts *ListOptions) ([]Session, error) {
	projectsDir := filepath.Join(claudeHome, "projects")
	projectEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var sessions []Session
	for _, pe := range projectEntries {
		if !pe.IsDir() {
			continue
		}
		projectSlug := pe.Name()
		if opts != nil && opts.ProjectSlug != "" && projectSlug != opts.ProjectSlug {
			continue
		}
		projectDir := filepath.Join(projectsDir, projectSlug)
		jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil {
			return nil, fmt.Errorf("failed to glob %s: %w", projectDir, err)
		}
		for _, jsonlPath := range jsonlFiles {
			s, err := loadSessionMeta(projectSlug, jsonlPath)
			if err != nil {
				// Skip unreadable or malformed files rather than aborting.
				continue
			}
			if opts != nil && !opts.Since.IsZero() && s.StartedAt.Before(opts.Since) {
				continue
			}
			if opts != nil && !opts.Until.IsZero() && !s.StartedAt.Before(opts.Until) {
				continue
			}
			sessions = append(sessions, s)
		}
	}

	// Sort most-recent-first.
	slices.SortFunc(sessions, func(a, b Session) int {
		if a.StartedAt.After(b.StartedAt) {
			return -1
		}
		if a.StartedAt.Before(b.StartedAt) {
			return 1
		}
		return 0
	})

	if opts != nil && opts.Limit > 0 && len(sessions) > opts.Limit {
		sessions = sessions[:opts.Limit]
	}
	return sessions, nil
}

// Resolve finds the unique session whose ID begins with prefix. Returns an
// error if the prefix is ambiguous or matches no session.
func Resolve(claudeHome, prefix string) (Session, error) {
	if prefix == "" {
		return Session{}, fmt.Errorf("session ID prefix must not be empty")
	}
	sessions, err := List(claudeHome, nil)
	if err != nil {
		return Session{}, fmt.Errorf("failed to list sessions: %w", err)
	}
	var matches []Session
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, prefix) {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return Session{}, fmt.Errorf("no session found with ID prefix %q", prefix)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		return Session{}, fmt.Errorf("ambiguous session ID prefix %q matches: %s",
			prefix, strings.Join(ids, ", "))
	}
}

// loadSessionMeta reads only as many lines as needed from a JSONL file to
// extract session metadata without loading full content blocks.
func loadSessionMeta(projectSlug, jsonlPath string) (Session, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return Session{}, fmt.Errorf("failed to open %s: %w", jsonlPath, err)
	}
	defer f.Close()

	entries, err := ParseEntries(f)
	if err != nil {
		return Session{}, fmt.Errorf("failed to parse %s: %w", jsonlPath, err)
	}
	if len(entries) == 0 {
		return Session{}, fmt.Errorf("empty session file: %s", jsonlPath)
	}

	sessionID := sessionIDFromPath(jsonlPath)

	var (
		slug         string
		model        string
		startedAt    time.Time
		endedAt      time.Time
		messageCount int
	)

	for _, e := range entries {
		if e.Timestamp.IsZero() {
			continue
		}
		if startedAt.IsZero() || e.Timestamp.Before(startedAt) {
			startedAt = e.Timestamp
		}
		if e.Timestamp.After(endedAt) {
			endedAt = e.Timestamp
		}
		if e.Slug != "" && slug == "" {
			slug = e.Slug
		}
		if e.Type == "assistant" && e.Message != nil && e.Message.Model != "" && model == "" {
			model = e.Message.Model
		}
		if e.Type == "user" || e.Type == "assistant" {
			messageCount++
		}
	}

	var durationSecs float64
	if !startedAt.IsZero() && !endedAt.IsZero() && endedAt.After(startedAt) {
		durationSecs = endedAt.Sub(startedAt).Seconds()
	}

	return Session{
		ID:           sessionID,
		ProjectSlug:  projectSlug,
		Slug:         slug,
		Model:        model,
		StartedAt:    startedAt,
		EndedAt:      endedAt,
		DurationSecs: durationSecs,
		MessageCount: messageCount,
	}, nil
}

// LoadOverview reads the full JSONL file for the session identified by prefix
// and returns an aggregated Overview. prefix may be a full ID or a unique prefix.
func LoadOverview(claudeHome, prefix string) (Overview, error) {
	s, err := Resolve(claudeHome, prefix)
	if err != nil {
		return Overview{}, err
	}

	jsonlPath := filepath.Join(claudeHome, "projects", s.ProjectSlug, s.ID+".jsonl")
	f, err := os.Open(jsonlPath)
	if err != nil {
		return Overview{}, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	entries, err := ParseEntries(f)
	if err != nil {
		return Overview{}, fmt.Errorf("failed to parse session: %w", err)
	}

	return OverviewFromEntries(s.ID, entries)
}

// sessionIDFromPath extracts the session UUID from a .jsonl file path.
func sessionIDFromPath(jsonlPath string) string {
	base := filepath.Base(jsonlPath)
	return strings.TrimSuffix(base, ".jsonl")
}
