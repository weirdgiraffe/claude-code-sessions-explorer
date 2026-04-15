# claude-code-sessions-explorer (cc-se)

Read-only CLI for browsing Claude Code session data. Provides structured,
filtered views of session JSONL files — optimized for both human and LLM
consumption.

## Install

```bash
go install github.com/weirdgiraffe/claude-code-sessions-explorer/cmd/cc-se@latest
```

Or build from source:

```bash
go build -o cc-se ./cmd/cc-se
```

## Usage

All commands output JSON by default. Use `--format text` for human-readable
output. Session IDs support prefix matching (6-8 characters is usually enough).

### List sessions

```bash
cc-se list [--project <slug>] [--since <date>] [--limit N]
```

Scans `~/.claude/projects/` for session files. Returns session ID, project slug,
timestamp, message count, duration, and model.

### Session overview

```bash
cc-se overview <session-id>
```

Single-session summary: message counts, tool call sequence, token usage, subagent
count, duration, model.

### Read conversation

```bash
cc-se conversation <session-id> [--role human|assistant] [--no-tool-results] [--full]
```

Human/assistant messages in order, with tool results shown as summaries. Use
`--full` to disable 500-character truncation.

### Extract tool calls

```bash
cc-se tools <session-id> [--name <tool>] [--sequence-only] [--full]
```

Paired tool_use/tool_result blocks. `--sequence-only` returns just tool names in
call order. `--name` filters to a specific tool.

### Explore subagents

```bash
cc-se subagents <session-id>
cc-se overview <session-id> --subagent <agent-id>
cc-se conversation <session-id> --subagent <agent-id>
cc-se tools <session-id> --subagent <agent-id>
```

Lists subagents with type and description. The `--subagent` flag scopes any
command to a specific agent's transcript.

## How it works

Claude Code stores session transcripts as JSONL files under
`~/.claude/projects/`. This CLI discovers those files and provides filtered views
without requiring manual JSONL parsing. See
[docs/JSONL-SCHEMA.md](docs/JSONL-SCHEMA.md) for the session file format
reference.

## Design constraints

- **Read-only** — never modifies session files
- **Single binary** — Go, no runtime dependencies
- **JSON-first** — structured output for LLM consumption; `--format text` for
  humans
- **Truncation by default** — tool I/O capped at 500 chars; `--full` to disable
- **Respects `CLAUDE_HOME`** — env var override, defaults to `~/.claude/`

## License

MIT
