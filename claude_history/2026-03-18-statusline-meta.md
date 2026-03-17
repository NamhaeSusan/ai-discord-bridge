# Replace provider meta line with statusline-style info

## Changes
- `provider.go`: Added `WorkingDir` field to `ProviderResult`
- `bot.go`: Set `WorkingDir` on result in both `handleChannelMessage` and `handleThreadMessage`
- `formatter.go`: Replaced `formatResultMeta` with new statusline format showing directory (with ~ shortening), git branch + changed file count, cost (when available), and duration. Added `shortenDir` and `gitBranchInfo` helpers.
- `formatter_test.go`: New test file covering `formatResultMeta` variants, `shortenDir`, `splitIntoChunks`, and `countOpenCodeBlocks`

## Format
```
> 📂 ~/project (main ✎3) | 💰 $0.0312 | ⏱ 3.2s
```
