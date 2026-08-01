# BUG-BE-AT-002: `WorktreeCleanupService` chưa được implement trong `AutomationService` — BL-AT-04 thiếu

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-BE-AT-002  
**Note:** automations/WorktreeCleanupService.ts: BL-AT-04 with uncommitted-changes safety check  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AT-04) mô tả:
```
[Daemon — WorktreeCleanupService.run()]
    SELECT worktrees WHERE createdAt < (now - policy.maxAge)
        AND status IN ('idle', 'stopped')
    FOR each worktree:
        git status (uncommitted changes?)  ← safety check
        IF safe: BL-WT-03 (delete worktree)
        IF unsafe: log + skip + alert admin
    UPDATE orca_automation_runs { cleanedCount, skippedCount }
    emit: cleanup:completed
```

Grep `src/main/automations/`:
```
WorktreeCleanupService → No results
cleanup:completed      → No results
type: 'cleanup'        → No results (action type in service)
```

`AutomationService.requestDispatch()` khi execute một automation có `type: 'cleanup'` sẽ chỉ dispatch sang renderer (`webContents.send('automations:dispatchRequested', ...)`) — **không có handler phía backend** cho cleanup action.

## File liên quan

- [`src/main/automations/service.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/automations/service.ts) — Lines 210-250: `requestDispatch()` không handle cleanup type
- HLD BL-AT-04 mô tả backend-side cleanup service

## Ảnh hưởng

1. **BL-AT-04**: Cleanup automations không thực sự cleanup worktrees.
2. `type: 'cleanup'` automation dispatch sang renderer — renderer không biết cách cleanup worktrees.
3. Disk sẽ không tự được free khi worktrees expire theo policy.

## Điểm đánh giá tích cực

`AutomationService` đã handle:
- `type: 'fan-out'` → dispatch sang renderer (renderer tạo N worktrees)
- `type: 'single'` → dispatch sang renderer (renderer tạo 1 worktree)

Nhưng `type: 'cleanup'` → dispatcher cần backend service để thực hiện git safety check + worktree delete.

## Liên quan đến luồng

- **BL-AT-04**: Cleanup policy — WorktreeCleanupService missing.
