# claude-code-sessions-explorer

Go CLI. Single binary, no runtime deps. Binary name: `cc-se`.

## Structure

| File | Purpose |
|------|---------|
| `cmd/cc-se/main.go` | CLI entry, command registration, flag handling |
| `internal/session/types.go` | JSONL entry + content block Go types |
| `internal/session/parser.go` | Line-by-line JSONL parser, content block parsing |
| `internal/session/store.go` | Session discovery, dir scanning, ID prefix resolution |
| `internal/session/conversation.go` | Conversation extraction, role filtering, tool result summaries |
| `internal/session/tools.go` | Tool call pairing (tool_use + tool_result), sequence extraction |
| `internal/session/subagent.go` | Subagent discovery, meta.json parsing, nested scanning |
| `internal/output/format.go` | JSON and text formatters for all commands |

## Build & Test

```
go build -o cc-se ./cmd/cc-se
go test ./...
```

## Design

- JSON output default, `--format text` for humans
- 500-char truncation on tool I/O; `--full` disables
- Session ID prefix matching on all commands
- `CLAUDE_HOME` env var overrides default `~/.claude/`
- JSONL format: [`docs/JSONL-SCHEMA.md`](docs/JSONL-SCHEMA.md)
