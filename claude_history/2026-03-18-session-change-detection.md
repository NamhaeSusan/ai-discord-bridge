# 2026-03-18: Session change detection and logging

## Problem
Thread sessions silently switching to new sessions (context lost) without any indication to the user.
Possible causes: context compaction (CLI returns different session_id), resume failure fallback, empty session ID stored.

## Changes

### bot.go — runThreadMessage
- Return `sessionChanged bool` to caller
- Detect session ID mismatch after resume (context compaction)
- Detect resume failure fallback to new session
- Add detailed logging for all session transition paths:
  - No existing session
  - Empty session ID stored
  - Resume failure
  - Session ID changed (compaction)

### bot.go — handleThreadMessage
- Send warning message to thread when session changes
