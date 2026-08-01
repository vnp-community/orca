# TASK-FE-ORCH-001-E: Zustand AgentStatus Slice Extension

**Domain:** agent-orchestration  
**Solution Ref:** SOL-FE-ORCH-001 Bước 7  
**Priority:** 🔴 P1  
**Estimated:** 30 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Mở rộng `agent-status` slice để track remote agent sessions (IPC-based). Thêm `remoteAgentSessions`, `setRemoteAgentSession`, `updateAgentStatus`.

---

## Files cần sửa

- `src/renderer/src/store/slices/agent-status.ts`
- `src/renderer/src/store/index.ts` (nếu cần register type extensions)

---

## Các bước thực thi

### Bước 1: Thêm types

```typescript
export type RemoteAgentSession = {
  sessionId: string
  worktreeId: string
  agentType: 'claude' | 'codex' | 'custom'
  status: 'starting' | 'running' | 'stopped' | 'error'
  startedAt: number
  stoppedAt?: number
  errorMessage?: string
}
```

### Bước 2: Thêm state + actions vào slice

Trong `createAgentStatusSlice`, thêm:

```typescript
remoteAgentSessions: {} as Record<string, RemoteAgentSession>,

setRemoteAgentSession: (worktreeId, session) =>
  set(s => ({
    remoteAgentSessions: { ...s.remoteAgentSessions, [worktreeId]: session }
  })),

clearRemoteAgentSession: (worktreeId) =>
  set(s => {
    const { [worktreeId]: _, ...rest } = s.remoteAgentSessions
    return { remoteAgentSessions: rest }
  }),

updateAgentStatus: (event: AgentStatusEvent) =>
  set(s => {
    const existing = s.remoteAgentSessions[event.worktreeId]
    if (!existing && !event.sessionId) return s
    const updated: RemoteAgentSession = {
      ...(existing ?? { worktreeId: event.worktreeId, agentType: 'claude', startedAt: Date.now() }),
      sessionId: event.sessionId ?? existing?.sessionId ?? '',
      status: event.status,
      ...(event.errorMessage ? { errorMessage: event.errorMessage } : {}),
      ...(event.status === 'stopped' ? { stoppedAt: Date.now() } : {}),
    }
    return { remoteAgentSessions: { ...s.remoteAgentSessions, [event.worktreeId]: updated } }
  }),
```

---

## Verify

```bash
grep -n "remoteAgentSessions\|updateAgentStatus" \
  src/renderer/src/store/slices/agent-status.ts
```

## Depends on
TASK-FE-ORCH-001-A

## Blocking
TASK-FE-ORCH-001-F (AgentPanel UI), TASK-FE-ORCH-001-G (useIpcEvents)
