# BUG-BE-AT-001: `AutomationService` không implement event-based triggers — không có `EventBus`, `AutomationEventHandler`, GitHub Webhook

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-AT-002  
**Note:** AutomationEventBridge.ts created  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AT-03) mô tả event-based automation triggers:
```
[EventBus] sự kiện hệ thống:
    worktree:idle (agent đã xong task)
    pr:merged (GitHub webhook)
    git:push (file watch)

[Main Process — AutomationEventHandler]
    SELECT automations WHERE triggerType='event' AND triggerConfig.event=<eventType>
    → AutomationEngine.run(automationId, { eventContext })

GitHub Webhook:
    GitHub → POST /webhook/github → Main
    → Verify HMAC-SHA256 signature
    → AutomationEventHandler.handle(event)
```

Nhưng `AutomationService` thực tế **chỉ support schedule-based** triggers:
```typescript
// service.ts:177
const now = Date.now()
for (const automation of this.store.listAutomations()) {
  if (!automation.enabled || automation.nextRunAt > now) {
    continue  // ← chỉ check nextRunAt (schedule)
  }
  await this.evaluateAutomation(automation, now)
}
```

Grep toàn bộ `src/main/automations/`:
```
EventBus              → No results
AutomationEventHandler → No results
worktree:idle          → No results
POST /webhook/github   → No results
HMAC-SHA256            → No results
triggerType.*event     → No results
```

## File liên quan

- [`src/main/automations/service.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/automations/service.ts) — Lines 169-185: `evaluateDueRuns()` chỉ check schedule

## Ảnh hưởng

1. **BL-AT-03**: Event-based triggers (worktree:idle, pr:merged, git:push) không hoạt động.
2. GitHub webhook integration không có → automations không tự kích hoạt khi PR merged.
3. Automation chỉ chạy theo schedule — không reactive với system events.
4. `precheck` command có implement nhưng event-based automation không bao giờ trigger `precheck`.

## Điểm đánh giá tích cực

`AutomationService` **đã implement** schedule-based triggers (BL-AT-01, BL-AT-02) với:
- `rrule` support thông qua `getLatestAutomationOccurrence`
- `missedRunGraceMinutes` handling
- Headless dispatch mode (`headlessDispatcher`)
- `precheck` runner

## Liên quan đến luồng

- **BL-AT-03**: Event-based automation trigger — chưa implement.
