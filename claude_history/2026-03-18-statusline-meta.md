# 2026-03-18: Replace provider meta line with statusline-style info

## Problem
Response footer showed `Provider: claude | Cost: $0.0000 | Duration: 3.2s` which was
mostly useless (provider already known per bot, cost often 0).

## Changes

### provider.go
- Add `WorkingDir` field to `ProviderResult`

### bot.go
- Set `result.WorkingDir` in both `handleChannelMessage` and `handleThreadMessage`

### formatter.go
- Replace `formatResultMeta` with statusline-style format:
  `> 📂 ~/project (main ✎3) | 💰 $0.0312 | ⏱ 3.2s`
- Add `shortenHome(dir)`: replaces $HOME with ~
- Add `gitInfo(dir)`: gets branch name + changed file count via git CLI
- Omit dir/git/cost sections when not available
