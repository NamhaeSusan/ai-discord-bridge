# bbolt 기반 세션 영속화

## 작업 내용
- bbolt(go.etcd.io/bbolt)를 사용하여 세션/스레드 매핑을 디스크에 영속화
- 봇 재시작 후에도 thread-session 매핑 및 작업 디렉터리 유지

## 변경 파일
- `store.go` (신규): bbolt 래퍼 — PutSession, AllSessions, PurgeExpiredSessions, PutThread, AllThreads
- `config.go`: Config에 DBPath 필드 추가, 기본값 `./sessions.db`
- `bot.go`: Runner/threadRegistry에 store 주입, write-through 캐시 패턴 적용
- `main.go`: OpenStore 호출 및 defer close
- `provider_test.go`: newThreadRegistry(nil) 호환
- `config/config.example.toml`: db_path 설정 예시 추가
- `CLAUDE.md`: store.go, bbolt 의존성 문서화

## 설계 결정
- sync.Map을 제거하지 않고 write-through 캐시로 유지 (조회 성능, 변경 범위 최소화)
- 버킷 구조: `sessions:{botName}` (봇별 분리), `threads` (공유)
- 시작 시 DB에서 sync.Map으로 bulk load하여 복원
