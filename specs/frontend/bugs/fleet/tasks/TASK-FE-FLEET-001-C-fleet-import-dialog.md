# TASK-FE-FLEET-001-C: `FleetImportDialog` component — import orca-fleet.yaml (CR-001)

**Domain:** fleet  
**Solution Ref:** SOL-FE-FLEET-001B Phần 3  
**Bug:** BUG-FE-FLEET-001  
**Priority:** 🟠 P1  
**Estimated:** 50 phút  
**Status:** ✅ DONE — Implemented in admin/fleet/fleet-import-dialog.tsx

---

## Mục tiêu

Tạo `FleetImportDialog` — dialog chọn file YAML + progress import với added/skipped/failed counts.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx`

---

## Các bước thực thi

Tạo file với nội dung đầy đủ từ SOL-FE-FLEET-001B §Phần 3:

1. **File picker:** drag-and-drop zone `.yaml/.yml` → `fileInputRef.current.click()`
2. **Parse:** `file.text()` → store YAML content
3. **Import flow:**
   ```typescript
   setFleetImportStatus({ phase: 'parsing', totalServers: 0, ... })
   const result = await rpc.call('fleet.import', { yamlContent })
   setFleetImportStatus({ phase: 'done', ...result })
   ```
4. **Progress display:**
   - Phase indicator với spinner / CheckCircle / XCircle icon
   - Progress bar (importedServers / totalServers * 100)
   - Grid 3 cols: Added (green) / Skipped (yellow) / Failed (red)
   - Error list (scrollable, max-h-20)
5. **Actions:** Cancel (outline) + Import button; Done/Close setelah finish

---

## Verify

```bash
grep -n "FleetImportDialog\|fleet.import" \
  src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx
```

## Depends on
TASK-FE-FLEET-001-A (fleetImportStatus actions)

## Blocking
TASK-FE-FLEET-001-D (FleetDashboard)
