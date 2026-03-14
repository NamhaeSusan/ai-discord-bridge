# AI Discord Bridge

## Overview
Discord 봇으로 AI CLI 도구 (Claude Code 등)를 Discord 채널에서 사용할 수 있게 하는 브릿지.

## Architecture

```
ai-discord-bridge/
├── main.go              # 엔트리포인트 (TOML config, env var, signal)
├── bot.go               # Discord 이벤트 핸들러, 세션 맵, 세마포어
├── claude.go            # Claude CLI 실행 (--print, --resume)
├── formatter.go         # Discord 2000자 청킹, 코드 블록 분할
├── go.mod               # Go 모듈
├── config/
│   └── config.example.toml  # 설정 예시
└── README.md
```

## Tech Stack

| 영역 | 라이브러리 | 용도 |
|------|-----------|------|
| Discord | `discordgo` | Discord Bot API |
| 설정 | `BurntSushi/toml` | TOML 파싱 |
| AI CLI | `claude` (외부) | Claude Code CLI 실행 |
