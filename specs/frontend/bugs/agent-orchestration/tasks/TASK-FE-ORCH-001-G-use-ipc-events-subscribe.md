# TASK-FE-ORCH-001-G: useIpcEvents — Subscribe `agent:statusChanged`

**Domain:** agent-orchestration  
**Solution Ref:** SOL-FE-ORCH-001 Bước 6  
**Priority:** 🟡 P2  
**Estimated:** 20 phút  
**Status:** ✅ DONE — Implemented in hooks/use-agent-orchestration-events.ts

---

## Mục tiêu

Subscribe event `agent:statusChanged` từ backend trong `useIpcEvents` để store tự động cập nhật khi agent status thay đổi.

---

## Files cần sửa

- `src/renderer/src/hooks/useIpcEvents.ts`

---

## Các bước thực thi

Tìm `useEffect` trong `useIpcEvents.ts` nơi các IPC events được đăng ký. Thêm subscription cho agent events:

```typescript
// Trong useEffect() của useIpcEvents:

// Agent status events (BUG-FE-ORCH-001 fix)
const handleAgentStatusChanged = (event: AgentStatusEvent) => {
  store.updateAgentStatus(event)
}

window.api.agent?.onStatusChanged?.(handleAgentStatusChanged)

// Cleanup
return () => {
  window.api.agent?.offStatusChanged?.(handleAgentStatusChanged)
  // ... other cleanups ...
}
```

**Lưu ý:** Dùng optional chaining `?.` để không break khi chạy mode không có agent API.

---

## Verify

```bash
grep -n "agent.*onStatusChanged\|offStatusChanged" \
  src/renderer/src/hooks/useIpcEvents.ts
```

## Depends on
TASK-FE-ORCH-001-A (types), TASK-FE-ORCH-001-E (slice actions)

## Blocking
Không có (task cuối trong chain)
