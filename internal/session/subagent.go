package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SubagentInfo holds aggregated metadata for a single subagent.
type SubagentInfo struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	MessageCount int    `json:"message_count"`
}

// ListSubagents discovers all subagents (including nested) for the session
// identified by sessionPrefix. It returns them in lexical order by ID.
func ListSubagents(claudeHome, sessionPrefix string) ([]SubagentInfo, error) {
	s, err := Resolve(claudeHome, sessionPrefix)
	if err != nil {
		return nil, err
	}
	subagentsDir := filepath.Join(claudeHome, "projects", s.ProjectSlug, s.ID, "subagents")
	agents, err := scanSubagents(subagentsDir)
	if err != nil {
		return nil, err
	}
	if agents == nil {
		agents = []SubagentInfo{}
	}
	return agents, nil
}

// ResolveSubagent finds the unique subagent whose ID begins with agentPrefix
// inside the session identified by sessionPrefix. It returns the absolute path
// to the subagent JSONL file.
func ResolveSubagent(claudeHome, sessionPrefix, agentPrefix string) (string, error) {
	if agentPrefix == "" {
		return "", fmt.Errorf("agent ID prefix must not be empty")
	}
	s, err := Resolve(claudeHome, sessionPrefix)
	if err != nil {
		return "", err
	}
	subagentsDir := filepath.Join(claudeHome, "projects", s.ProjectSlug, s.ID, "subagents")
	matches, err := findAgentPaths(subagentsDir, agentPrefix)
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no subagent found with ID prefix %q in session %s", agentPrefix, s.ID)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, p := range matches {
			ids[i] = AgentIDFromPath(p)
		}
		return "", fmt.Errorf("ambiguous subagent ID prefix %q matches: %s",
			agentPrefix, strings.Join(ids, ", "))
	}
}

// scanSubagents recursively scans dir for agent-*.jsonl files and returns
// SubagentInfo for each one found. dir need not exist — a missing directory
// returns an empty slice without error.
func scanSubagents(dir string) ([]SubagentInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read subagents directory %s: %w", dir, err)
	}

	var agents []SubagentInfo
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		agentID := strings.TrimSuffix(strings.TrimPrefix(name, "agent-"), ".jsonl")
		jsonlPath := filepath.Join(dir, name)

		info, err := loadSubagentInfo(agentID, jsonlPath)
		if err != nil {
			// Skip unreadable files rather than aborting.
			continue
		}
		agents = append(agents, info)

		// Recurse into nested subagents directory.
		nestedDir := filepath.Join(dir, "agent-"+agentID, "subagents")
		nested, err := scanSubagents(nestedDir)
		if err != nil {
			return nil, err
		}
		agents = append(agents, nested...)
	}
	return agents, nil
}

// findAgentPaths recursively searches dir for agent JSONL paths whose ID
// begins with prefix.
func findAgentPaths(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read subagents directory %s: %w", dir, err)
	}

	var paths []string
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		agentID := strings.TrimSuffix(strings.TrimPrefix(name, "agent-"), ".jsonl")
		if strings.HasPrefix(agentID, prefix) {
			paths = append(paths, filepath.Join(dir, name))
		}

		// Always recurse into nested subagents.
		nestedDir := filepath.Join(dir, "agent-"+agentID, "subagents")
		nested, err := findAgentPaths(nestedDir, prefix)
		if err != nil {
			return nil, err
		}
		paths = append(paths, nested...)
	}
	return paths, nil
}

// loadSubagentInfo reads the JSONL and optional .meta.json for a subagent and
// returns the aggregated SubagentInfo.
func loadSubagentInfo(agentID, jsonlPath string) (SubagentInfo, error) {
	agentType, description := readAgentMeta(jsonlPath)
	messageCount, err := countMessages(jsonlPath)
	if err != nil {
		return SubagentInfo{}, fmt.Errorf("failed to count messages in %s: %w", jsonlPath, err)
	}
	return SubagentInfo{
		ID:           agentID,
		Type:         agentType,
		Description:  description,
		MessageCount: messageCount,
	}, nil
}

// readAgentMeta reads the .meta.json file adjacent to a subagent JSONL file.
// Returns empty strings if the file is absent or malformed.
func readAgentMeta(jsonlPath string) (agentType, description string) {
	metaPath := strings.TrimSuffix(jsonlPath, ".jsonl") + ".meta.json"
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return "", ""
	}
	var meta struct {
		AgentType   string `json:"agentType"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", ""
	}
	return meta.AgentType, meta.Description
}

// countMessages opens a JSONL file and counts user+assistant entries.
func countMessages(jsonlPath string) (int, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", jsonlPath, err)
	}
	defer f.Close()

	entries, err := ParseEntries(f)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s: %w", jsonlPath, err)
	}

	count := 0
	for _, e := range entries {
		if e.Type == "user" || e.Type == "assistant" {
			count++
		}
	}
	return count, nil
}

// AgentIDFromPath extracts the agent ID from an agent-*.jsonl file path.
func AgentIDFromPath(jsonlPath string) string {
	name := filepath.Base(jsonlPath)
	name = strings.TrimSuffix(name, ".jsonl")
	return strings.TrimPrefix(name, "agent-")
}
