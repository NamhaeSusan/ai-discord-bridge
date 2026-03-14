# AI Discord Bridge

Discord bot that bridges AI CLI tools (Claude Code, etc.) to Discord channels.

## Features

- Send Discord messages → Claude CLI (`--print`) → reply with results
- Reply to bot messages to resume the same Claude session (`--resume`)
- TOML config file + environment variable overrides (12-factor compatible)
- Channel/user whitelist filtering
- Auto-chunking for messages over 2000 characters (with code block split handling)
- Typing indicator while Claude is running (8s interval)
- Concurrent execution limit (semaphore, default 5)
- Session map with TTL-based auto-cleanup (default 60 min)

## Requirements

- Go 1.26+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) installed and in PATH

## Setup

1. Create a Discord bot at [Discord Developer Portal](https://discord.com/developers/applications)
2. Enable **Message Content Intent** under Bot settings
3. Copy config example:

```bash
cp config/config.example.toml config/config.toml
```

4. Fill in `bot_token` and optional settings

## Configuration

| Field | Env Var | Description |
|-------|---------|-------------|
| `bot_token` | `DISCORD_BOT_TOKEN` | Discord bot token (required) |
| `allowed_channels` | `DISCORD_ALLOWED_CHANNELS` | Comma-separated channel IDs |
| `allowed_users` | `DISCORD_ALLOWED_USERS` | Comma-separated user IDs |
| `claude_working_dir` | `CLAUDE_WORKING_DIR` | Working directory for Claude CLI |
| `claude_model` | `CLAUDE_MODEL` | Claude model to use |
| `claude_max_budget_usd` | `CLAUDE_MAX_BUDGET_USD` | Max budget per invocation |
| `claude_allowed_tools` | `CLAUDE_ALLOWED_TOOLS` | Comma-separated allowed tools |
| `claude_timeout_seconds` | `CLAUDE_TIMEOUT_SECONDS` | Timeout per invocation (default: 300) |
| `max_concurrent` | — | Max parallel Claude invocations (default: 5) |
| `session_ttl_minutes` | — | Session cleanup interval (default: 60) |

Environment variables override TOML values.

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
