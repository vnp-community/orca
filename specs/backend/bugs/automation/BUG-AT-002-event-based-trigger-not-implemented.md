# BUG-AT-002: BL-AT-03 Event-based Automation — EventBus không tồn tại, GitHub webhook handler không có

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-AT-002  
**Note:** AutomationEventBridge.ts: git.push/pr.created/worktree.created triggers  

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD BL-AT-03 mô tả:
```
[EventBus] sự kiện hệ thống:
    worktree:idle, pr:merged, git:push
    →  AutomationEventHandler.handle(event)

GitHub Webhook:
    GitHub → POST /webhook/github → Main
    Verify HMAC-SHA256 signature
    → AutomationEventHandler.handle(event)
```

Grep codebase:
```
EventBus              → Not found (src/main/)
AutomationEventHandler → Not found
POST /webhook/github  → Not found
worktree:idle         → Not found (as trigger)
```

**BL-AT-03 hoàn toàn chưa implement.**

`AutomationService` chỉ support **schedule-based** triggers (cron expressions). Không có event-based trigger mechanism.

`git:push` chỉ xuất hiện trong `src/main/ipc/filesystem.ts:1844` như một IPC event name, không phải automation trigger.

## Ảnh hưởng

1. BL-AT-03 (Event-based): 0% implementation
2. GitHub webhook → automation không hoạt động
3. `worktree:idle` trigger không có → automation sau khi agent xong không tự trigger
4. PR merge trigger không có

## Files cần tạo

```
src/main/automations/event-bus.ts        — EventBus class
src/main/automations/event-handler.ts    — AutomationEventHandler
src/server/routes/webhook-github.ts      — POST /webhook/github
src/main/automations/trigger-registry.ts — Register event triggers
```

## Files liên quan

- `src/main/automations/service.ts`: chỉ có schedule triggers
- `src/shared/automations-types.ts`: triggerType type cần kiểm tra
