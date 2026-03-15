# /cwd standalone command support

## Summary
`/cwd` 명령어를 독립 명령어로도 사용할 수 있도록 개선.

## Changes

### bot.go
- `parseWorkdirDirective`: `/cwd` 단독 → `("", "", nil)`, `/cwd <path>` 단독 → `(absDir, "", nil)` 리턴 (기존: 에러)
- `handleThreadMessage`: prompt 빈 경우 분기 추가 — 현재 세션 workdir 조회/변경, AI 호출 없이 종료
- `handleChannelMessage`: prompt 빈 경우 분기 추가 — thread 생성 후 workdir 표시/설정
- `updateSessionWorkingDir`: 기존 세션의 workingDir만 업데이트하는 헬퍼 메서드 추가

### provider_test.go
- `TestParseWorkdirDirectiveRequiresPrompt` 제거
- `TestParseWorkdirDirectiveShowCwd` 추가: `/cwd` → `("", "", nil)`
- `TestParseWorkdirDirectiveChangeOnly` 추가: `/cwd .` → `(absDir, "", nil)`

## Behavior Matrix

| Context | Input | Action |
|---------|-------|--------|
| thread | `/cwd` | Show current session working directory |
| thread | `/cwd <path>` | Change session working directory (no AI call) |
| thread | `/cwd <path>\n<prompt>` | Change dir + AI call (unchanged) |
| channel | `/cwd` | Create thread, show config default working directory |
| channel | `/cwd <path>` | Create thread, set working directory |
