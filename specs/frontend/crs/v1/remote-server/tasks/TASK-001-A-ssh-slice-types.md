# TASK-001-A — Mở rộng SshSlice Types

**Task ID:** TASK-001-A  
**CR:** CR-001 — Fleet Inventory Config  
**Solution Ref:** SOL-CR-001, Section 2  
**Dependencies:** Không  
**Estimated:** 1–2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Thêm `FleetImportStatus` type và mở rộng `SshSlice` trong Zustand store để lưu trạng thái fleet import.

---

## Context

**Codebase:** `src/renderer/src/store/`  
**Pattern:** Zustand slice (StateCreator\<AppState\>)  
**TDD Ref:** TDD-FE-02 (State Management)

---

## Các bước thực thi

### Bước 1: Tìm file ssh slice hiện tại

```bash
find src/renderer/src/store/slices -name "ssh.ts" -o -name "ssh*.ts"
```

Đọc file để hiểu type hiện tại trước khi modify.

### Bước 2: Thêm types vào `src/renderer/src/store/types.ts`

Tìm vị trí khai báo types trong file types.ts, thêm:

```typescript
// Fleet import types
export type FleetImportPhase = 'parsing' | 'importing' | 'done' | 'error'

export type FleetImportStatus = {
  phase: FleetImportPhase
  totalServers: number
  importedServers: number
  skippedServers: number
  failedServers: number
  errors: string[]
  configFilePath: string
}

export type FleetImportResult = {
  imported: SshTarget[]
  skipped: string[]
  failed: Array<{ id: string; error: string }>
  totalParsed: number
}
```

### Bước 3: Mở rộng `SshSlice` interface

Trong file slice ssh.ts, tìm type/interface `SshSlice` và thêm các fields mới:

```typescript
// Thêm vào SshSlice type:
fleetImportStatus: FleetImportStatus | null
setFleetImportStatus: (status: FleetImportStatus | null) => void
clearFleetImportStatus: () => void
```

### Bước 4: Implement actions trong `createSshSlice`

Trong hàm `createSshSlice`, thêm initial values và actions:

```typescript
fleetImportStatus: null,

setFleetImportStatus: (status) =>
  set(s => { s.fleetImportStatus = status }),

clearFleetImportStatus: () =>
  set(s => { s.fleetImportStatus = null }),
```

### Bước 5: Verify TypeScript

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | head -50
```

---

## Acceptance Criteria

- [x] `FleetImportStatus` type được export từ `store/types.ts`
- [x] `SshSlice` có `fleetImportStatus`, `setFleetImportStatus`, `clearFleetImportStatus`
- [x] `createSshSlice` implement đúng 3 actions mới
- [x] `AppState` type (intersection) vẫn compile không lỗi
- [x] Không có type error trong tsc --noEmit

---

## Notes cho AI

- File chính cần sửa: `src/renderer/src/store/slices/ssh.ts` và `src/renderer/src/store/types.ts`
- Nếu types.ts không tồn tại riêng, types có thể nằm trong `store/index.ts` — tìm và thêm vào đúng chỗ
- Giữ nguyên tất cả code hiện tại, chỉ THÊM không XÓA
- Nếu `SshSlice` dùng `immer` middleware, đảm bảo mutate pattern đúng

---

## Implementation Notes

> **Completed:** 2026-07-23 | `store/slices/ssh.ts`: FleetImportStatus, FleetImportPhase, fleetImportStatus state + actions. `store/types.ts`: Re-exported all fleet+auth types. TypeScript: ✅ 0 errors.
