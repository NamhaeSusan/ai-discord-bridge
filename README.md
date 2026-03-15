# AI Discord Bridge

Discord bot that bridges AI CLI tools like Claude Code and Codex to Discord channels.

## Features

- Run multiple Discord bots from one process
- Send Discord messages to Claude or Codex and continue in the created thread
- Reply in a thread to resume the same Claude or Codex session
- Override the working directory per thread with a `/cwd <path>` first line
- TOML config file for multi-bot setup, plus legacy single-bot env overrides
- Channel/user whitelist filtering
- Auto-chunking for messages over 2000 characters (with code block split handling)
- Typing indicator while the provider is running (8s interval)
- Concurrent execution limit (semaphore, default 5)
- Session map with TTL-based auto-cleanup (default 60 min)

## Requirements

- Go 1.26+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) installed and in PATH
- `codex` CLI installed and in PATH if any bot uses `provider = "codex"`

## Setup

1. Create a Discord bot at [Discord Developer Portal](https://discord.com/developers/applications)
2. Enable **Message Content Intent** under Bot settings
3. Copy config example:

```bash
cp config/config.example.toml config/config.toml
```

4. Fill in one or more `[[bots]]` entries

## Configuration

Each bot uses one `[[bots]]` entry:

| Field | Description |
|-------|-------------|
| `name` | Bot name for logs |
| `provider` | `claude` or `codex` |
| `bot_token` | Discord bot token |
| `allowed_channels` | Channel ID allowlist |
| `allowed_users` | User ID allowlist |
| `working_dir` | CLI working directory |
| `model` | Model name passed to the CLI |
| `timeout_seconds` | Timeout per invocation (default: 300) |
| `max_concurrent` | Max parallel invocations (default: 5) |
| `session_ttl_minutes` | Session cleanup TTL (default: 60) |
| `max_budget_usd` | Claude-only budget limit |
| `allowed_tools` | Claude-only allowed tools |
| `sandbox` | Codex-only sandbox mode (default: `danger-full-access`) |

Legacy top-level Claude config and env overrides still work for a single-bot setup. The new multi-bot format is TOML-first.

### Per-thread working directory override

You can start a thread in a different repository by putting `/cwd <path>` on the first line:

```text
/cwd /Users/kimtaeyun/trelab-workspace/trelab-drb/trelab-drb-server

PostgreSQL support work 이어서
```

The parsed directory is stored for that Discord thread and reused on follow-up replies.

Example:

```toml
[[bots]]
name = "claude"
provider = "claude"
bot_token = "discord-token-for-claude"
working_dir = "/path/to/project"
model = "sonnet"
allowed_tools = ["Read", "Grep", "Glob"]

[[bots]]
name = "codex"
provider = "codex"
bot_token = "discord-token-for-codex"
working_dir = "/path/to/project"
model = "gpt-5-codex"
sandbox = "danger-full-access"
```

## Run

```bash
# With config file
go run . -config config/config.toml

# With environment variables
DISCORD_BOT_TOKEN=xxx go run .

# Build and run
go build -o ai-discord-bridge .
./ai-discord-bridge -config config/config.toml
```

## License

Private
