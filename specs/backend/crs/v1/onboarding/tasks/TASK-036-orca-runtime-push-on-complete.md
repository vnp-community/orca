# TASK-036: Sửa `src/main/runtime/orca-runtime.ts` — Push Notification khi Agent Task Complete

**Phase:** 3 — Web Push Notifications  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §B.7  
**Depends on:** TASK-035  
**Blocks:** (không)

---

## Mục tiêu

Trigger web push notification khi một agent task complete trong `OrcaRuntime`.

---

## File cần sửa

**Path:** `src/main/runtime/orca-runtime.ts` (hoặc tên file tương đương)

---

## Context cần tra cứu

1. Tìm hàm/event xử lý "agent task complete" trong runtime: grep `taskComplete\|task:complete\|onTaskDone` trong `src/main/runtime/`
2. Xác định tên parameter của summary/worktreeId trong handler đó

---

## Thay đổi cần thực hiện

Inject `WebPushManager` vào `OrcaRuntime` và gọi `sendToAll()` khi task complete:

```typescript
import type { WebPushManager } from '../notifications/web-push-manager'

// Trong constructor hoặc setter:
private pushManager: WebPushManager | null = null

setPushManager(manager: WebPushManager): void {
  this.pushManager = manager
}

// Trong agent task complete handler:
private async onAgentTaskComplete(worktreeId: string, summary: string): Promise<void> {
  // ...existing logic giữ nguyên...

  // NEW: push notification
  if (this.pushManager) {
    await this.pushManager.sendToAll({
      title: 'Task complete',
      body: summary,
      tag: `worktree-${worktreeId}`,
      url: `/worktree/${worktreeId}`
    }).catch(err => {
      // Log error nhưng không throw — push notification là best-effort
      console.error('[WebPush] sendToAll failed:', err)
    })
  }
}
```

Trong `server-bootstrap.ts` (sau TASK-035), gọi `setPushManager`:

```typescript
orcaRuntime.setPushManager(pushManager)
```

---

## Acceptance Criteria

- [x] `OrcaRuntime` có `setPushManager()` setter hoặc constructor injection
- [x] Khi agent task complete, `pushManager.sendToAll()` được gọi
- [x] Push notification không block task completion (lỗi push không ảnh hưởng task logic)
- [x] `title` = 'Task complete', `body` = summary, `tag` = `worktree-{id}`
- [x] `pushManager` có thể là `null` (push là optional feature)
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Tìm đúng tên method/event "agent task complete" trong `orca-runtime.ts`
2. Tên `OrcaRuntime` có thể khác — tìm class quản lý agent worktrees
3. Đảm bảo push là fire-and-forget (không await hoặc catch error)
