# TASK-001-E — Cập nhật SshSettingsPanel với Import Fleet Button

**Task ID:** TASK-001-E  
**CR:** CR-001 — Fleet Inventory Config  
**Solution Ref:** SOL-CR-001, Section 4.3  
**Dependencies:** TASK-001-A, TASK-001-B, TASK-001-C  
**Estimated:** 1 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Cập nhật `SshSettingsPanel` (hoặc component tương đương) để thêm button "Import Fleet Config" và tích hợp `FleetImportDialog`.

---

## Bước thực thi

### Bước 1: Tìm SSH settings panel hiện tại

```bash
find src/renderer/src/components -name "*ssh*" -o -name "*Ssh*" | grep -i "setting\|panel\|host"
find src/renderer/src/components/settings -type f | head -30
```

### Bước 2: Đọc component hiện tại

Đọc file SSH settings panel để hiểu cấu trúc hiện tại, tìm:
- Vị trí toolbar/header với các buttons
- Component list SSH targets
- Import statements cần thêm

### Bước 3: Thêm state và dialog

```typescript
// Trong component function, thêm state:
const [fleetImportOpen, setFleetImportOpen] = useState(false)
```

### Bước 4: Thêm button "Import Fleet Config"

Tìm vị trí các action buttons (thường gần "Add SSH Host" button), thêm:

```typescript
<Button
  variant="outline"
  size="sm"
  onClick={() => setFleetImportOpen(true)}
>
  {translate('fleet.import.button', 'Import Fleet Config')}
</Button>
```

### Bước 5: Thêm FleetImportDialog ở cuối JSX

```typescript
{/* Fleet Import Dialog */}
<FleetImportDialog
  open={fleetImportOpen}
  onClose={() => setFleetImportOpen(false)}
/>
```

### Bước 6: Thêm imports cần thiết

```typescript
import { FleetImportDialog } from './FleetImportDialog'
```

### Bước 7: Verify

```bash
npx tsc --noEmit 2>&1 | grep "SshSettings\|FleetImport" | head -10
```

---

## Acceptance Criteria

- [x] Button "Import Fleet Config" xuất hiện trong SSH settings toolbar
- [x] Click button → `FleetImportDialog` mở
- [x] Dialog close → button có thể click lại
- [x] TypeScript compile không lỗi
- [x] Không phá vỡ layout/functionality hiện tại

---

## Notes cho AI

- Nếu SSH settings component có nhiều tên khác nhau (SshHostsPage, RemoteServersPanel, v.v.), tìm đúng file bằng grep
- Không xóa hoặc thay thế button "Add Host" hiện tại — chỉ THÊM button mới
- Đặt button "Import Fleet Config" TRƯỚC button "Add Host" (theo logic: import trước → add)
- Lazy load `FleetImportDialog` nếu component lớn: `const FleetImportDialog = lazyWithRetry(() => import('./FleetImportDialog'))`

---

## Implementation Notes

> **Completed:** 2026-07-23 | `SshPane.tsx`: 'Import Fleet' button in toolbar, FleetImportDialog integrated, close handler resets dialog state. TypeScript: ✅ 0 errors.
