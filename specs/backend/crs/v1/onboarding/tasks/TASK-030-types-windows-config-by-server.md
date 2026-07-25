# TASK-030: Sửa `src/shared/types.ts` — Thêm `GlobalSettings.terminalWindowsConfigByServer`

**Phase:** 3 — Windows Terminal  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §A.3  
**Depends on:** TASK-002  
**Blocks:** (không)

---

## Mục tiêu

Thêm field `terminalWindowsConfigByServer` vào `GlobalSettings` để lưu per-server Windows terminal configuration.

---

## File cần sửa

**Path:** `src/shared/types.ts`

---

## Thay đổi cần thực hiện

```typescript
type GlobalSettings = {
  // ... existing terminal settings giữ nguyên cho backward compat ...
  terminalWindowsConfigByServer?: Record<string, {  // NEW
    shell: string              // 'powershell.exe' | 'cmd.exe' | 'wsl.exe' | git-bash-path
    wslDistro: string | null
    rightClickToPaste: boolean
  }>
}
```

---

## Acceptance Criteria

- [x] `GlobalSettings` có field `terminalWindowsConfigByServer?: Record<string, {...}>`
- [x] Field là optional (backward compatible)
- [x] Per-server config có `shell`, `wslDistro`, `rightClickToPaste`
- [x] Các settings Windows cũ (flat) được giữ nguyên (không thay đổi)
- [x] TypeScript compile thành công
