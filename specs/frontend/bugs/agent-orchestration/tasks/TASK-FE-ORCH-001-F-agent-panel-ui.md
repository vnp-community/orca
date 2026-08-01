# TASK-FE-ORCH-001-F: AgentPanel UI Component

**Domain:** agent-orchestration  
**Solution Ref:** SOL-FE-ORCH-001 Bước 8  
**Priority:** 🟡 P2  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `AgentPanel.tsx` — giao diện Start/Stop/Resume agent có status badge, agent type selector, optimistic UI.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/workspace/AgentPanel.tsx`

---

## Các bước thực thi

Tạo file với nội dung đầy đủ từ SOL-FE-ORCH-001 §Bước 8. Component bao gồm:

1. **AgentPanel** — main component
   - Đọc `session` từ `useAppStore(s => s.remoteAgentSessions[worktreeId])`
   - `startAgent()` → gọi `window.api.agent.start()` → `updateAgentStatus()`
   - `stopAgent()` → gọi `window.api.agent.stop()`
   - `resumeAgent()` → gọi `window.api.agent.resume()`
   - Optimistic update: set `status: 'starting'` trước khi call API

2. **AgentStatusBadge** — inline helper component
   - variants: `starting` (yellow), `running` (green), `stopped` (muted), `error` (red)

3. **Agent Type Selector** — `<Select>` cho claude/codex/custom

**Import cần dùng:**
```typescript
import { useAppStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useWorkspace } from '@/context/WorkspaceContext'
import { Play, Square, RotateCcw, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
```

---

## Verify

```bash
# File tồn tại
ls src/renderer/src/components/workspace/AgentPanel.tsx

# Component render không lỗi
grep -n "export function AgentPanel" \
  src/renderer/src/components/workspace/AgentPanel.tsx
```

## Test

```typescript
// src/renderer/src/components/workspace/__tests__/AgentPanel.test.tsx
// - renders "Start" button when no session
// - shows "Starting..." status when status=starting  
// - shows "Stop" button when status=running
// - shows "Resume" button when status=stopped and sessionId set
// - calls window.api.agent.start() with correct worktreeId + agentType
```

## Depends on
TASK-FE-ORCH-001-E (slice), TASK-FE-ORCH-001-A (types)
