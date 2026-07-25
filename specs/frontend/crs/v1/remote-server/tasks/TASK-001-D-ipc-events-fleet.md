# TASK-001-D — Thêm Fleet IPC Event Handlers vào useIpcEvents

**Task ID:** TASK-001-D  
**CR:** CR-001 — Fleet Inventory Config  
**Solution Ref:** SOL-CR-001, Section 3.2  
**Dependencies:** TASK-001-A, TASK-001-B  
**Estimated:** 1 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Thêm handler cho `onFleetImportProgress` event vào `useIpcEvents` hook để store được cập nhật real-time khi backend streaming tiến trình import.

---

## File cần sửa

`src/renderer/src/hooks/useIpcEvents.ts`

---

## Bước thực thi

### Bước 1: Đọc useIpcEvents hiện tại

```bash
wc -l src/renderer/src/hooks/useIpcEvents.ts
grep -n "ssh\|SSH\|onConnection" src/renderer/src/hooks/useIpcEvents.ts | head -20
```

### Bước 2: Tìm vị trí thêm handler

Tìm block xử lý SSH events (gần `window.api.ssh.onConnectionStateChanged`):

```bash
grep -n "ssh.onConnection\|ssh\.on" src/renderer/src/hooks/useIpcEvents.ts
```

### Bước 3: Thêm fleet import progress handler

Trong `useEffect` của `useIpcEvents`, sau SSH connection handler, thêm:

```typescript
// [NEW CR-001] Fleet import progress events
const unsubFleetImport = window.api.ssh.onFleetImportProgress?.((event) => {
  store.setFleetImportStatus({
    phase: event.phase,
    totalServers: event.total,
    importedServers: event.imported,
    skippedServers: event.skipped,
    failedServers: event.failed,
    errors: event.errors ?? [],
    configFilePath: event.configFilePath ?? '',
  })

  if (event.phase === 'done') {
    const msg = `${event.imported} servers imported`
      + (event.skipped > 0 ? `, ${event.skipped} skipped` : '')
    toast.success(msg)
    scheduleRuntimeGraphSync()
  }

  if (event.phase === 'error') {
    scheduleRuntimeGraphSync()
  }
})
```

### Bước 4: Thêm cleanup trong return

```typescript
// Trong cleanup function của useEffect:
return () => {
  // ...existing cleanups...
  unsubFleetImport?.()   // [NEW]
}
```

### Bước 5: Verify

```bash
npx tsc --noEmit 2>&1 | grep "useIpcEvents\|fleetImport" | head -10
```

---

## Acceptance Criteria

- [x] Handler `onFleetImportProgress` được đăng ký trong useIpcEvents
- [x] Khi nhận event → `store.setFleetImportStatus()` được gọi đúng
- [x] Khi phase = 'done' → toast success + `scheduleRuntimeGraphSync()`
- [x] Cleanup function có `unsubFleetImport?.()`
- [x] Optional chaining `?.` bảo vệ khi `onFleetImportProgress` chưa có

---

## Notes cho AI

- Dùng optional chaining `window.api.ssh.onFleetImportProgress?.()` vì API có thể chưa tồn tại trong mọi build
- `store` ở đây là `useAppStore()` hoặc tương đương đã khai báo ở top của hook
- `scheduleRuntimeGraphSync` import từ `@/runtime/sync-runtime-graph`
- Giữ nguyên tất cả handlers hiện có, chỉ THÊM handler mới

---

## Implementation Notes

> **Completed:** 2026-07-23 | `useIpcEvents.ts`: onFleetImportProgress handler with optional chaining, all phase cases, toast on done, scheduleRuntimeGraphSync(), cleanup unsub. TypeScript: ✅ 0 errors.
