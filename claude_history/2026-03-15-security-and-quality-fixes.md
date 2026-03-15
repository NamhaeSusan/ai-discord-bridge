# Security & Quality Fixes

## 작업 내용
코드 리뷰 후 CRITICAL/HIGH 이슈 수정

## 변경 사항

### Security
- `bot.go`: `validateWorkingDir()` 추가 — `/cwd` 경로가 `working_dir` 하위인지 검증 (경로 순회 방어)

### Bug Fixes
- `config.go`: `parseFloatOrExit`/`parseIntOrExit` 삭제, `applyLegacyEnvOverrides`가 error 반환 (os.Exit 제거)
- `main.go`: defer bot.Close() 루프 내 사용 수정 — 실패 시 이미 열린 봇 정리
- `bot.go`: 세션 TTL을 `createdAt` → `lastAccessAt` 기준으로 변경
- `store.go`: storedSession 필드명 동기화 (`CreatedAt` → `LastAccessAt`)

### Improvements
- `bot.go`: `cleanupSessions` 고루틴에 `done` 채널 추가, `Close()`에서 종료
- `bot.go`: `truncate` 함수 `[]rune` 기반으로 변경 (UTF-8 안전)
