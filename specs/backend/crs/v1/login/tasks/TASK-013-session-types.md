# TASK-013: Tạo `src/main/session/session-types.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 2 — User Sandbox
**Solution:** [SOL-LG-002](../solutions/SOL-LG-002-user-sandbox.md) §4.1
**Depends on:** (không có)
**Blocks:** TASK-014, TASK-016

---

## Mục tiêu

Tạo types cho session manager subsystem — UserProcess và config.

---

## File cần tạo

**Path:** `src/main/session/session-types.ts`

---

## Nội dung

```typescript
// src/main/session/session-types.ts
import type { ChildProcess } from 'node:child_process'

export type UserProcess = {
  userId:       string
  pid:          number
  socketPath:   string
  startedAt:    number
  lastSeenAt:   number
  process:      ChildProcess
  respawnCount: number
}

export type SessionManagerConfig = {
  baseDataPath:        string   // e.g. /data/orca — base dir cho all users
  userProcessEntry:    string   // absolute path to user-process-entry.js (built)
  idleTimeoutMs?:      number   // default: 4h
  maxRespawnAttempts?: number   // default: 3
}
```

---

## Acceptance Criteria

- [x] File tồn tại
- [x] Export `UserProcess` với đúng fields (pid, socketPath, startedAt, lastSeenAt, process, respawnCount)
- [x] Export `SessionManagerConfig` với đúng fields (baseDataPath, userProcessEntry, optional idleTimeoutMs, maxRespawnAttempts)
- [x] TypeScript compile sạch
