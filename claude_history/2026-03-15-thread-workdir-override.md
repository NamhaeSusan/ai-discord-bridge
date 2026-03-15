# 2026-03-15 Thread Workdir Override

## Summary
- Added per-thread working directory override with `/cwd <path>` on the first message line.
- Stored resolved working directory in the thread session and reused it for follow-up replies.
- Updated Claude/Codex providers to accept per-invocation working directory overrides.
- Documented the new usage in `README.md` and `CLAUDE.md`.

## Changed Files
- `bot.go`
- `provider.go`
- `claude.go`
- `codex.go`
- `provider_test.go`
- `README.md`
- `CLAUDE.md`

## Verification
- `GOCACHE=/tmp/go-build-ai-discord-bridge go test ./...`
