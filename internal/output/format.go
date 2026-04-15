// Package output provides formatters for CLI output.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/weirdgiraffe/claude-code-sessions-explorer/internal/session"
)

// WriteJSON encodes sessions as a JSON array to w.
func WriteJSON(w io.Writer, sessions []session.Session) error {
	if sessions == nil {
		sessions = []session.Session{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sessions); err != nil {
		return fmt.Errorf("failed to encode sessions as JSON: %w", err)
	}
	return nil
}

// WriteText writes sessions as a human-readable tab-separated table to w.
func WriteText(w io.Writer, sessions []session.Session) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tPROJECT\tSLUG\tMODEL\tSTARTED\tMSGS\tDURATION")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			s.ID,
			s.ProjectSlug,
			s.Slug,
			s.Model,
			s.StartedAt.Format(time.RFC3339),
			s.MessageCount,
			formatDuration(s.DurationSecs),
		)
	}
	return tw.Flush()
}

// WriteOverviewJSON encodes a single session overview as a JSON object to w.
func WriteOverviewJSON(w io.Writer, ov session.Overview) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ov); err != nil {
		return fmt.Errorf("failed to encode overview as JSON: %w", err)
	}
	return nil
}

// WriteOverviewText writes a session overview as human-readable key-value pairs to w.
func WriteOverviewText(w io.Writer, ov session.Overview) error {
	kv := func(key, val string) {
		fmt.Fprintf(w, "%-28s %s\n", key+":", val)
	}

	kv("session_id", ov.SessionID)
	kv("model", ov.Model)
	kv("duration", formatDuration(ov.DurationSecs))
	kv("messages_user", fmt.Sprintf("%d", ov.MessageCount.User))
	kv("messages_assistant", fmt.Sprintf("%d", ov.MessageCount.Assistant))
	kv("messages_total", fmt.Sprintf("%d", ov.MessageCount.Total))
	kv("tokens_input", fmt.Sprintf("%d", ov.TokenUsage.InputTokens))
	kv("tokens_output", fmt.Sprintf("%d", ov.TokenUsage.OutputTokens))
	kv("tokens_cache_created", fmt.Sprintf("%d", ov.TokenUsage.CacheCreationInputTokens))
	kv("tokens_cache_read", fmt.Sprintf("%d", ov.TokenUsage.CacheReadInputTokens))
	kv("subagent_count", fmt.Sprintf("%d", ov.SubagentCount))

	if len(ov.Tools) == 0 {
		kv("tools", "(none)")
	} else {
		kv("tools", strings.Join(ov.Tools, ", "))
	}

	return nil
}

// WriteConversationJSON encodes conversation messages as a JSON array to w.
func WriteConversationJSON(w io.Writer, msgs []session.ConversationMessage) error {
	if msgs == nil {
		msgs = []session.ConversationMessage{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(msgs); err != nil {
		return fmt.Errorf("failed to encode conversation as JSON: %w", err)
	}
	return nil
}

// WriteConversationText writes conversation messages as role-prefixed lines to w.
func WriteConversationText(w io.Writer, msgs []session.ConversationMessage) error {
	for i, m := range msgs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		label := strings.ToUpper(m.Role)
		fmt.Fprintf(w, "[%s] %s\n", label, m.Content)
	}
	return nil
}

// WriteToolsJSON encodes tool calls as a JSON array to w.
func WriteToolsJSON(w io.Writer, calls []session.ToolCall) error {
	if calls == nil {
		calls = []session.ToolCall{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(calls); err != nil {
		return fmt.Errorf("failed to encode tool calls as JSON: %w", err)
	}
	return nil
}

// WriteToolsText writes tool calls as labeled blocks to w.
func WriteToolsText(w io.Writer, calls []session.ToolCall) error {
	for i, c := range calls {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "[TOOL] %s\n", c.Name)
		if c.Input != "" {
			fmt.Fprintf(w, "input:  %s\n", c.Input)
		}
		if c.IsError {
			fmt.Fprintf(w, "error:  %s\n", c.Result)
		} else if c.Result != "" {
			fmt.Fprintf(w, "result: %s\n", c.Result)
		}
	}
	return nil
}

// WriteToolSequenceJSON encodes a sequence of tool names as a JSON array to w.
func WriteToolSequenceJSON(w io.Writer, names []string) error {
	if names == nil {
		names = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(names); err != nil {
		return fmt.Errorf("failed to encode tool sequence as JSON: %w", err)
	}
	return nil
}

// WriteToolSequenceText writes tool names one per line to w.
func WriteToolSequenceText(w io.Writer, names []string) error {
	for _, name := range names {
		fmt.Fprintln(w, name)
	}
	return nil
}

// WriteSubagentsJSON encodes a slice of subagent info as a JSON array to w.
// An empty slice is encoded as [] rather than null.
func WriteSubagentsJSON(w io.Writer, agents []session.SubagentInfo) error {
	if agents == nil {
		agents = []session.SubagentInfo{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(agents); err != nil {
		return fmt.Errorf("failed to encode subagents as JSON: %w", err)
	}
	return nil
}

// WriteSubagentsText writes subagent info as a human-readable tab-separated
// table to w.
func WriteSubagentsText(w io.Writer, agents []session.SubagentInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTYPE\tMSGS\tDESCRIPTION")
	for _, a := range agents {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n",
			a.ID,
			a.Type,
			a.MessageCount,
			a.Description,
		)
	}
	return tw.Flush()
}

func formatDuration(secs float64) string {
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	if secs < 3600 {
		m := int(secs / 60)
		s := int(secs) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(secs / 3600)
	m := int(secs/60) % 60
	s := int(secs) % 60
	return fmt.Sprintf("%dh%dm%ds", h, m, s)
}
