package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/weirdgiraffe/claude-code-sessions-explorer/internal/output"
	"github.com/weirdgiraffe/claude-code-sessions-explorer/internal/session"
)

func main() {
	app := &cli.Command{
		Name:  "cc-se",
		Usage: "Read-only CLI for browsing Claude Code session data",
		Commands: []*cli.Command{
			listCommand(),
			overviewCommand(),
			conversationCommand(),
			toolsCommand(),
			subagentsCommand(),
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func listCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List sessions from ~/.claude/projects/",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "project",
				Usage: "filter by project slug",
			},
			&cli.StringFlag{
				Name:  "since",
				Usage: "only show sessions started on or after this date (ISO 8601, e.g. 2026-01-01)",
			},
			&cli.StringFlag{
				Name:  "until",
				Usage: "only show sessions started before this date (ISO 8601, e.g. 2026-02-01)",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "maximum number of sessions to return",
				Value: 20,
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: json or text",
				Value: "json",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			claudeHome, err := session.ClaudeHome()
			if err != nil {
				return err
			}

			opts := &session.ListOptions{
				ProjectSlug: cmd.String("project"),
				Limit:       cmd.Int("limit"),
			}

			if sinceStr := cmd.String("since"); sinceStr != "" {
				t, err := parseDateArg(sinceStr)
				if err != nil {
					return err
				}
				opts.Since = t
			}

			if untilStr := cmd.String("until"); untilStr != "" {
				t, err := parseDateArg(untilStr)
				if err != nil {
					return err
				}
				opts.Until = t
			}

			sessions, err := session.List(claudeHome, opts)
			if err != nil {
				return err
			}

			format := cmd.String("format")
			switch format {
			case "json":
				return output.WriteJSON(os.Stdout, sessions)
			case "text":
				return output.WriteText(os.Stdout, sessions)
			default:
				return fmt.Errorf("unknown format %q: must be json or text", format)
			}
		},
	}
}

func overviewCommand() *cli.Command {
	return &cli.Command{
		Name:      "overview",
		Usage:     "Summarise a single session",
		ArgsUsage: "<session-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "subagent",
				Usage: "show overview for a specific subagent (ID or prefix)",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: json or text",
				Value: "json",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return fmt.Errorf("session-id argument is required")
			}
			prefix := cmd.Args().First()

			claudeHome, err := session.ClaudeHome()
			if err != nil {
				return err
			}

			var ov session.Overview
			if agentPrefix := cmd.String("subagent"); agentPrefix != "" {
				jsonlPath, err := session.ResolveSubagent(claudeHome, prefix, agentPrefix)
				if err != nil {
					return err
				}
				entries, err := loadEntriesFromPath(jsonlPath)
				if err != nil {
					return err
				}
				agentID := session.AgentIDFromPath(jsonlPath)
				ov, err = session.OverviewFromEntries(agentID, entries)
				if err != nil {
					return err
				}
			} else {
				ov, err = session.LoadOverview(claudeHome, prefix)
				if err != nil {
					return err
				}
			}

			format := cmd.String("format")
			switch format {
			case "json":
				return output.WriteOverviewJSON(os.Stdout, ov)
			case "text":
				return output.WriteOverviewText(os.Stdout, ov)
			default:
				return fmt.Errorf("unknown format %q: must be json or text", format)
			}
		},
	}
}

func conversationCommand() *cli.Command {
	return &cli.Command{
		Name:      "conversation",
		Usage:     "Show human/assistant messages from a session",
		ArgsUsage: "<session-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "subagent",
				Usage: "show conversation for a specific subagent (ID or prefix)",
			},
			&cli.StringFlag{
				Name:  "role",
				Usage: "filter messages by role: human or assistant",
			},
			&cli.BoolFlag{
				Name:  "no-thinking",
				Usage: "exclude thinking blocks (default: thinking already excluded)",
			},
			&cli.BoolFlag{
				Name:  "no-tool-results",
				Usage: "omit tool result summaries entirely",
			},
			&cli.BoolFlag{
				Name:  "full",
				Usage: "disable 500-character truncation of tool result content",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: json or text",
				Value: "json",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return fmt.Errorf("session-id argument is required")
			}
			prefix := cmd.Args().First()

			claudeHome, err := session.ClaudeHome()
			if err != nil {
				return err
			}

			roleFlag := cmd.String("role")
			// Normalise "human" -> "user" to match the JSONL role values.
			if roleFlag == "human" {
				roleFlag = "user"
			}
			if roleFlag != "" && roleFlag != "user" && roleFlag != "assistant" {
				return fmt.Errorf("invalid --role %q: must be human or assistant", cmd.String("role"))
			}

			// Thinking is excluded by default; --no-thinking is a no-op that exists
			// for forward compatibility. Content is redacted in persisted sessions anyway.
			opts := session.ConversationOptions{
				Role:               roleFlag,
				IncludeThinking:    false,
				IncludeToolResults: !cmd.Bool("no-tool-results"),
				Full:               cmd.Bool("full"),
			}

			var msgs []session.ConversationMessage
			if agentPrefix := cmd.String("subagent"); agentPrefix != "" {
				jsonlPath, err := session.ResolveSubagent(claudeHome, prefix, agentPrefix)
				if err != nil {
					return err
				}
				entries, err := loadEntriesFromPath(jsonlPath)
				if err != nil {
					return err
				}
				msgs, err = session.ConversationFromEntries(entries, opts)
				if err != nil {
					return err
				}
			} else {
				msgs, err = session.LoadConversation(claudeHome, prefix, opts)
				if err != nil {
					return err
				}
			}

			format := cmd.String("format")
			switch format {
			case "json":
				return output.WriteConversationJSON(os.Stdout, msgs)
			case "text":
				return output.WriteConversationText(os.Stdout, msgs)
			default:
				return fmt.Errorf("unknown format %q: must be json or text", format)
			}
		},
	}
}

func toolsCommand() *cli.Command {
	return &cli.Command{
		Name:      "tools",
		Usage:     "Extract tool_use/tool_result pairs from a session",
		ArgsUsage: "<session-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "subagent",
				Usage: "show tools for a specific subagent (ID or prefix)",
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "filter to a specific tool name",
			},
			&cli.BoolFlag{
				Name:  "sequence-only",
				Usage: "output just the tool names in call order",
			},
			&cli.BoolFlag{
				Name:  "full",
				Usage: "disable 500-character truncation of input and result content",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: json or text",
				Value: "json",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return fmt.Errorf("session-id argument is required")
			}
			prefix := cmd.Args().First()

			claudeHome, err := session.ClaudeHome()
			if err != nil {
				return err
			}

			opts := session.ToolsOptions{
				Name:         cmd.String("name"),
				SequenceOnly: cmd.Bool("sequence-only"),
				Full:         cmd.Bool("full"),
			}

			var calls []session.ToolCall
			if agentPrefix := cmd.String("subagent"); agentPrefix != "" {
				jsonlPath, err := session.ResolveSubagent(claudeHome, prefix, agentPrefix)
				if err != nil {
					return err
				}
				entries, err := loadEntriesFromPath(jsonlPath)
				if err != nil {
					return err
				}
				calls, err = session.ToolsFromEntries(entries, opts)
				if err != nil {
					return err
				}
			} else {
				var err error
				calls, err = session.LoadTools(claudeHome, prefix, opts)
				if err != nil {
					return err
				}
			}

			format := cmd.String("format")
			if opts.SequenceOnly {
				names := session.ToolSequence(calls)
				switch format {
				case "json":
					return output.WriteToolSequenceJSON(os.Stdout, names)
				case "text":
					return output.WriteToolSequenceText(os.Stdout, names)
				default:
					return fmt.Errorf("unknown format %q: must be json or text", format)
				}
			}

			switch format {
			case "json":
				return output.WriteToolsJSON(os.Stdout, calls)
			case "text":
				return output.WriteToolsText(os.Stdout, calls)
			default:
				return fmt.Errorf("unknown format %q: must be json or text", format)
			}
		},
	}
}

func subagentsCommand() *cli.Command {
	return &cli.Command{
		Name:      "subagents",
		Usage:     "List subagents in a session",
		ArgsUsage: "<session-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: json or text",
				Value: "json",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return fmt.Errorf("session-id argument is required")
			}
			prefix := cmd.Args().First()

			claudeHome, err := session.ClaudeHome()
			if err != nil {
				return err
			}

			agents, err := session.ListSubagents(claudeHome, prefix)
			if err != nil {
				return err
			}

			format := cmd.String("format")
			switch format {
			case "json":
				return output.WriteSubagentsJSON(os.Stdout, agents)
			case "text":
				return output.WriteSubagentsText(os.Stdout, agents)
			default:
				return fmt.Errorf("unknown format %q: must be json or text", format)
			}
		},
	}
}

// loadEntriesFromPath opens a JSONL file and returns its parsed entries.
func loadEntriesFromPath(jsonlPath string) ([]session.Entry, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", jsonlPath, err)
	}
	defer f.Close()

	entries, err := session.ParseEntries(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", jsonlPath, err)
	}
	return entries, nil
}

// parseDateArg parses an ISO 8601 date or datetime string into a time.Time.
func parseDateArg(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q: expected ISO 8601 format, e.g. 2026-01-01", s)
}
