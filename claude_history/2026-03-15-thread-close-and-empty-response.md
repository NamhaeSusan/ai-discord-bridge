# 2026-03-15 Thread Close And Empty Response

## Summary
- Closed expired Discord threads by archiving and locking them after posting the expiry notice.
- Notified the parent channel when a thread session expired.
- Replaced metadata-only Discord posts with a fallback message when the provider returned empty text.
- Updated tests to match current CLI argument construction and to cover the empty-response fallback.

## Verification
- `gofmt -w bot.go formatter.go provider_test.go`
- `go test ./...`
