# TASK-FE-ORCH-001-A: Mở rộng OrcaApi Interface — thêm `agent` namespace

**Domain:** agent-orchestration  
**Solution Ref:** SOL-FE-ORCH-001 Bước 1  
**Priority:** 🔴 P0 — prerequisite cho mọi task tiếp theo  
**Estimated:** 30 phút  
**Status:** ✅ DONE — Types added in preload/index.ts agentOrchestration namespace

---

## Mục tiêu

Thêm `agent` namespace vào `OrcaApi` interface để Renderer có thể gọi `window.api.agent.start/stop/resume`.

---

## Files cần sửa

1. `src/renderer/src/web/web-preload-api.ts`
2. `src/shared/types/api-types.ts` (nếu types dùng chung)

---

## Các bước thực thi

### Bước 1: Thêm types vào shared/api-types

Tìm hoặc tạo file types, thêm:

```typescript
export interface AgentStartOptions {
  worktreeId: string
  agentType: 'claude' | 'codex' | 'custom'
  trustPreset: 'standard' | 'permissive' | 'strict'
}

export interface AgentStartResult {
  sessionId: string
  status: 'started' | 'already-running'
}

export interface AgentStopOptions  { sessionId: string }
export interface AgentResumeOptions { sessionId: string }
export interface AgentResumeResult  { resumed: boolean }

export interface AgentStatusEvent {
  worktreeId: string
  sessionId?: string
  status: 'starting' | 'running' | 'stopped' | 'error'
  errorMessage?: string
}
```

### Bước 2: Thêm `agent` vào OrcaApi interface

Trong `web-preload-api.ts`, tìm interface `OrcaApi` (hoặc `Window['api']`) và thêm:

```typescript
agent: {
  start:  (opts: AgentStartOptions)  => Promise<AgentStartResult>
  stop:   (opts: AgentStopOptions)   => Promise<void>
  resume: (opts: AgentResumeOptions) => Promise<AgentResumeResult>
  onStatusChanged:  (cb: (event: AgentStatusEvent) => void) => void
  offStatusChanged: (cb: (event: AgentStatusEvent) => void) => void
}
```

---

## Verify

```bash
grep -n "agent:" src/renderer/src/web/web-preload-api.ts
```

## Depends on
Không có

## Blocking
TASK-FE-ORCH-001-B, TASK-FE-ORCH-001-C, TASK-FE-ORCH-001-E
