# 2026-03-26: `/cwd` picker, aliases, and recent paths

## Summary
- Added structured `/cwd` command parsing with support for aliases, recent-path commands, and Discord component pickers.
- Persisted shared aliases and recent working directories in bbolt.
- Restricted all `/cwd` targets to the current user's home directory and updated docs.

## Details
- `cwd.go`
  - Added `/cwd` command parser for `recent` and `alias` subcommands.
  - Added working-directory resolution helpers and picker component builders.
- `cwd_runner.go`
  - Added message-component interaction handling for alias/recent selection and refresh.
  - Added thread/channel `/cwd` execution flows and Discord admin checks for alias management.
- `store.go`
  - Added alias persistence and deduplicated recent-directory persistence.
- `README.md`, `CLAUDE.md`
  - Documented picker UI, alias/recent commands, and home-directory restriction.

## Verification
- `GOCACHE=/tmp/go-build-ai-discord-bridge go test ./...`
