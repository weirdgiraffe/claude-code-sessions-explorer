# Claude Code Session JSONL Schema

Reference for the JSONL format used by Claude Code to store session transcripts.
Derived from real session files found in `~/.claude/projects/`.

## File Layout

```
~/.claude/projects/<project-slug>/
  <session-id>.jsonl                                    — main session log
  <session-id>/
    subagents/
      agent-<agent-id>.jsonl                            — subagent transcript
      agent-<agent-id>.meta.json                        — subagent metadata
      agent-<agent-id>/
        subagents/                                      — nested subagents (recursive)
    tool-results/
      <tool-use-id>.json                                — large tool output (see below)
```

Project slugs are derived from the working directory path with `/` replaced by
`-` and a leading `-`. Example: `/Users/username/code/myproject` becomes
`-Users-username-code-myproject`.

## Entry Types

Each line in a JSONL file is a self-contained JSON object. The `type` field
discriminates the entry kind.

| Type | Purpose |
|------|---------|
| `permission-mode` | Sets session permission level |
| `user` | User messages and tool results |
| `assistant` | AI responses with token usage and model info |
| `attachment` | Metadata (tools, instructions, skills, permissions) |
| `progress` | Subagent execution progress |
| `system` | System events (turn duration, local command output) |
| `file-history-snapshot` | File change tracking for undo |
| `custom-title` | Title assigned to the session by the model |
| `agent-name` | Name assigned to the session |
| `last-prompt` | Stores last user prompt for session history |

## Common Fields

Most entries share these fields:

| Field | Type | Description |
|-------|------|-------------|
| `type` | `string` | Entry type discriminator |
| `uuid` | `string` | Unique entry identifier |
| `parentUuid` | `string \| null` | UUID of the preceding entry |
| `timestamp` | `string` | ISO 8601 timestamp |
| `sessionId` | `string` | Session UUID |
| `isSidechain` | `bool` | `true` for subagent messages |
| `userType` | `string` | `"external"` for CLI sessions |
| `entrypoint` | `string` | How Claude Code was invoked (e.g., `"cli"`) |
| `cwd` | `string` | Working directory |
| `version` | `string` | Claude Code version |
| `gitBranch` | `string` | Git branch at time of message |
| `slug` | `string` | Human-readable session slug |

## Content Blocks

Both `user` and `assistant` entries have a `message` field containing a
`content` field. For `user` entries, `content` can be a plain string or an array
of typed blocks. For `assistant` entries, `content` is always an array.

### Text

```json
{ "type": "text", "text": "string content" }
```

### Tool Use (assistant only)

```json
{
  "type": "tool_use",
  "id": "toolu_01ABC...",
  "name": "Bash|Edit|Read|Write|Glob|Grep|Agent|...",
  "input": { "command": "...", "file_path": "...", "..." : "..." },
  "caller": { "type": "direct" }
}
```

### Tool Result (user only)

```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01ABC...",
  "content": "string output or structured object",
  "is_error": false
}
```

Tool result `content` can be:
- A plain string (most tools)
- An object with `stdout`, `stderr`, `interrupted` fields (Bash tool)
- An object with structured data (Agent tool returns)

### Thinking (assistant only)

```json
{
  "type": "thinking",
  "thinking": "",
  "signature": "BASE64_SIGNATURE"
}
```

Note: thinking content is empty in persisted sessions (redacted). Only the
signature is stored.

## Entry Type Details

### `permission-mode`

Records the active permission mode. Typically the first entry in a session.

```json
{
  "type": "permission-mode",
  "permissionMode": "default",
  "sessionId": "049e42da-..."
}
```

### `user`

User input or tool result. Three variants based on `message.content` and `isMeta`.

**Text input** (user's actual prompt):

```json
{
  "parentUuid": "c06decdb-...",
  "isSidechain": false,
  "promptId": "69863765-...",
  "type": "user",
  "message": {
    "role": "user",
    "content": "I would like to update the /poa-healthcheck skill..."
  },
  "isMeta": false,
  "uuid": "7ecb98c1-...",
  "timestamp": "2026-03-23T13:57:53.475Z",
  "permissionMode": "plan",
  "userType": "external",
  "entrypoint": "cli",
  "cwd": "/Users/username/project",
  "sessionId": "110bd295-...",
  "version": "2.1.81",
  "gitBranch": "main",
  "slug": "iridescent-scribbling-noodle"
}
```

**Tool result** (response to a tool call):

```json
{
  "parentUuid": "8fb7a993-...",
  "isSidechain": false,
  "promptId": "69863765-...",
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {
        "tool_use_id": "toolu_011k5DBMQSePnP8nENCds4jy",
        "type": "tool_result",
        "content": "tool output text or structured content...",
        "is_error": false
      }
    ]
  },
  "uuid": "a5cd882d-...",
  "timestamp": "2026-03-23T13:58:01.431Z",
  "sourceToolAssistantUUID": "cc02a97d-...",
  "userType": "external",
  "entrypoint": "cli",
  "cwd": "/Users/username/project",
  "sessionId": "110bd295-...",
  "version": "2.1.81",
  "gitBranch": "main"
}
```

**Meta message** (synthetic, not real user input):

```json
{
  "parentUuid": null,
  "isSidechain": false,
  "type": "user",
  "message": {
    "role": "user",
    "content": "<local-command-caveat>Caveat: The messages below were generated by the user while running local commands...</local-command-caveat>"
  },
  "isMeta": true,
  "uuid": "f39e6e62-...",
  "timestamp": "2026-03-23T13:57:52.037Z"
}
```

#### User-Specific Fields

| Field | Type | Description |
|-------|------|-------------|
| `promptId` | `string` | Groups entries from the same user prompt |
| `isMeta` | `bool` | `true` for synthetic messages (local command caveats) |
| `message.content` | `string \| array` | String for text input; array of `tool_result` objects for tool responses |
| `sourceToolAssistantUUID` | `string` | For tool results: UUID of the assistant entry that made the tool call |
| `permissionMode` | `string` | Permission mode active when message was sent (e.g., `"plan"`, `"default"`) |
| `agentId` | `string` | Present in subagent messages; links to subagent JSONL file |

### `assistant`

Model response. Contains one or more content blocks.

```json
{
  "parentUuid": "7ecb98c1-...",
  "isSidechain": false,
  "message": {
    "model": "claude-opus-4-6",
    "id": "msg_012FyJPYAHoHKy8RZo2DYjJD",
    "type": "message",
    "role": "assistant",
    "content": [ "...content blocks..." ],
    "stop_reason": "tool_use",
    "stop_sequence": null,
    "usage": { "...see Token Usage section..." }
  },
  "requestId": "req_011CZL8UCymSoy9MVqhXnGDZ",
  "type": "assistant",
  "uuid": "8fb7a993-...",
  "timestamp": "2026-03-23T13:58:00.116Z",
  "userType": "external",
  "entrypoint": "cli",
  "cwd": "/Users/username/project",
  "sessionId": "110bd295-...",
  "version": "2.1.81",
  "gitBranch": "main",
  "slug": "iridescent-scribbling-noodle"
}
```

A single assistant message may be split across multiple JSONL lines sharing the
same `requestId` and `message.id`. This happens during streaming -- early lines
may have `stop_reason: null` and partial content, while the final line has the
complete content and `stop_reason` set.

#### Assistant-Specific Fields

| Field | Type | Description |
|-------|------|-------------|
| `message.model` | `string` | Model used (e.g., `"claude-opus-4-6"`, `"claude-sonnet-4-6-20260414"`) |
| `message.id` | `string` | API message ID; shared across streaming chunks of the same response |
| `message.content` | `array` | Array of content blocks (`text`, `tool_use`, `thinking`) |
| `message.stop_reason` | `string \| null` | `"end_turn"`, `"tool_use"`, or `null` (streaming in progress) |
| `message.usage` | `object` | Token usage for this API call (see Token Usage section) |
| `requestId` | `string` | API request ID; shared across streaming chunks |

### `progress`

Subagent execution progress. Emitted while an Agent tool call is running.

```json
{
  "parentUuid": "8fb7a993-...",
  "isSidechain": false,
  "type": "progress",
  "data": {
    "message": {
      "type": "user",
      "message": {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "Explore the poa-healthcheck skill directory..."
          }
        ]
      },
      "uuid": "4ae4f192-...",
      "timestamp": "2026-03-23T13:58:00.117Z"
    },
    "type": "agent_progress",
    "prompt": "Explore the poa-healthcheck skill directory...",
    "agentId": "a22b0b3973014b318"
  },
  "toolUseID": "agent_msg_012FyJPYAHoHKy8RZo2DYjJD",
  "parentToolUseID": "toolu_011k5DBMQSePnP8nENCds4jy",
  "uuid": "c4a6829e-...",
  "timestamp": "2026-03-23T13:58:00.118Z"
}
```

#### Progress-Specific Fields

| Field | Type | Description |
|-------|------|-------------|
| `data.type` | `string` | Progress type (e.g., `"agent_progress"`) |
| `data.agentId` | `string` | Subagent identifier; maps to `agent-<agentId>.jsonl` file |
| `data.prompt` | `string` | The prompt sent to the subagent |
| `parentToolUseID` | `string` | The `tool_use.id` from the Agent call in the parent assistant message |

### `system`

System events and metadata. Discriminated by `subtype`.

**Turn duration** (`subtype: "turn_duration"`):

```json
{
  "parentUuid": "17e5c362-...",
  "isSidechain": false,
  "type": "system",
  "subtype": "turn_duration",
  "durationMs": 273714,
  "timestamp": "2026-03-23T14:23:57.834Z",
  "uuid": "3951c8cd-...",
  "isMeta": false,
  "sessionId": "110bd295-..."
}
```

**Local command output** (`subtype: "local_command"`):

```json
{
  "parentUuid": "0b36d750-...",
  "isSidechain": false,
  "type": "system",
  "subtype": "local_command",
  "content": "<local-command-stdout>...</local-command-stdout>",
  "level": "info",
  "timestamp": "2026-03-23T13:57:52.037Z"
}
```

### `file-history-snapshot`

File state checkpoint for undo/rewind.

```json
{
  "type": "file-history-snapshot",
  "messageId": "0b36d750-...",
  "snapshot": {
    "messageId": "0b36d750-...",
    "trackedFileBackups": {},
    "timestamp": "2026-03-23T13:57:52.037Z"
  },
  "isSnapshotUpdate": false
}
```

### `custom-title`

Title assigned to the session by the model.

```json
{
  "type": "custom-title",
  "customTitle": "migrate-contract-discovery-to-chronicles",
  "sessionId": "110bd295-..."
}
```

### `agent-name`

Name assigned to the session (often matches `custom-title`).

```json
{
  "type": "agent-name",
  "agentName": "migrate-contract-discovery-to-chronicles",
  "sessionId": "110bd295-..."
}
```

### `last-prompt`

The last user prompt in the session. Used for session resume display.

```json
{
  "type": "last-prompt",
  "lastPrompt": "yes. looks correct",
  "sessionId": "110bd295-..."
}
```

## Token Usage

Attached to every `assistant` entry in `message.usage`. This is the only source
of token data -- `user` and `system` entries do not carry usage.

```json
{
  "input_tokens": 15000,
  "output_tokens": 2000,
  "cache_creation_input_tokens": 5000,
  "cache_read_input_tokens": 10000,
  "cache_creation": {
    "ephemeral_5m_input_tokens": 3000,
    "ephemeral_1h_input_tokens": 2000
  },
  "server_tool_use": {
    "web_search_requests": 0,
    "web_fetch_requests": 0
  },
  "iterations": [
    { "input_tokens": 15000, "output_tokens": 2000 }
  ],
  "service_tier": "standard",
  "inference_geo": ""
}
```

| Field | Type | Description |
|-------|------|-------------|
| `input_tokens` | `int` | Tokens in the input (prompt) sent to the model |
| `output_tokens` | `int` | Tokens generated by the model |
| `cache_creation_input_tokens` | `int` | Total tokens written to cache (sum of 5m and 1h) |
| `cache_read_input_tokens` | `int` | Tokens served from cache |
| `cache_creation.ephemeral_5m_input_tokens` | `int` | Tokens written to 5-minute cache |
| `cache_creation.ephemeral_1h_input_tokens` | `int` | Tokens written to 1-hour cache |
| `server_tool_use.web_search_requests` | `int` | Number of web searches performed |
| `server_tool_use.web_fetch_requests` | `int` | Number of web fetches performed |
| `iterations` | `array` | Per-iteration token breakdown |
| `service_tier` | `string` | API service tier (e.g., `"standard"`) |

When `stop_reason` is `null` (streaming in progress), the `usage` fields may
contain partial or placeholder values. Use the final entry in a streaming
sequence (where `stop_reason` is set) for accurate token counts.

## Subagent Files

### Meta File (`agent-<id>.meta.json`)

```json
{
  "agentType": "Explore",
  "description": "Explore poa-healthcheck skill"
}
```

### Transcript File (`agent-<id>.jsonl`)

Same JSONL format as the main session, with two differences:

1. **`isSidechain: true`** on all messages
2. **`agentId` field** present on messages, matching the `<id>` in the filename

Subagents may use a different model than the parent session (e.g., haiku for
Explore agents, opus for the main session).

## Session Identification

There is no centralized session index. Sessions are identified by:

- **Session ID**: UUID in the JSONL filename
- **Project slug**: parent directory name
- **Slug/name**: human-readable slug in entry fields (e.g.,
  `"groovy-sauteeing-falcon"`)
- **Timestamp**: from first entry or file modification time
- **CWD**: working directory recorded in entries

## Large Tool Results

When tool output exceeds a size threshold, it is persisted to a separate file:

```
~/.claude/projects/<project-slug>/<session-id>/tool-results/<tool-use-id>.json
```

The JSONL entry contains a reference to this path rather than the full content.
