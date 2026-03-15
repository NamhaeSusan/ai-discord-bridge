# AI Discord Bridge

## Overview
Discord 봇으로 AI CLI 도구 (Claude Code 등)를 Discord 채널에서 사용할 수 있게 하는 브릿지.

## Features

- Discord 채널 메시지를 Claude/Codex CLI 실행으로 연결
- 생성된 thread에서 동일 세션 이어쓰기
- `/cwd` 명령어로 작업 디렉터리 조회, `/cwd <path>`로 변경 (프롬프트 없이도 사용 가능)
- thread별 작업 디렉터리 세션 저장 및 재사용
- bbolt 기반 세션 영속화 (재시작 후에도 thread-session 매핑 유지)
- 세션 만료 시 thread 안내 메시지 전송, thread 자동 종료, 부모 채널 알림
- provider 응답이 비어 있을 때 메타데이터만 보내지 않고 fallback 메시지 전송

## Architecture

```
ai-discord-bridge/
├── main.go              # 엔트리포인트 (멀티봇 실행, CLI 존재 확인)
├── config.go            # TOML/legacy env 설정 로드, 기본값/정규화
├── bot.go               # Discord 이벤트 핸들러, 세션 맵, 세마포어
├── store.go             # bbolt 기반 세션/스레드 영속화
├── provider.go          # provider 인터페이스와 공통 결과 타입
├── claude.go            # Claude CLI 실행
├── codex.go             # Codex CLI 실행
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
| 영속화 | `bbolt` | 세션/스레드 매핑 저장 |
| AI CLI | `claude`, `codex` (외부) | Claude Code / Codex CLI 실행 |
