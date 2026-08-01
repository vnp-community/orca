# TASK-FE-CR-001-C: Tạo `ChangedFilesTree` component — files list với change counts (BL-CR-03)

**Domain:** code-review  
**Solution Ref:** SOL-FE-CR-001 Component 5 (Bổ sung)  
**Priority:** 🟠 P1  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `ChangedFilesTree` component — hiển thị danh sách files thay đổi nhóm theo thư mục, với số lượng dòng thêm/xóa, icon theo loại thay đổi.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/code-review/changed-files-tree.tsx`

---

## Các bước thực thi

Tạo file với nội dung đầy đủ từ SOL-FE-CR-001 §Component 5. Bao gồm:

1. **Types:**
   ```typescript
   type ChangeType = 'added' | 'deleted' | 'modified' | 'renamed'
   interface ChangedFile { path, changeType, additions, deletions, oldPath? }
   ```

2. **`groupByDirectory(files)`** — helper function group files by parent dir path

3. **`ChangeTypeIcon`** — `FilePlus` (green), `FileMinus` (red), `FileEdit` (blue), `FileCode` (yellow)

4. **`ChangeStats`** — `+N -N` hiển thị additions/deletions per file and per dir

5. **Main component `ChangedFilesTree`:**
   - Summary header: "N files changed +X -Y"
   - Collapsible directories với `ChevronRight/Down`
   - File rows: indent + icon + filename + stats
   - Renamed files: hiện `← oldName`
   - Highlighted selected file

---

## Verify

```bash
grep -n "ChangedFilesTree\|groupByDirectory" \
  src/renderer/src/components/code-review/changed-files-tree.tsx
```

## Test

```typescript
// changed-files-tree.test.tsx
// - groups files by directory correctly
// - shows +/- counts per file and dir
// - collapse/expand directory toggle works
// - renamed file shows oldPath arrow
// - selected file has highlight class
```

## Depends on
Không có

## Blocking
TASK-FE-CR-001-D (useCodeReview), TASK-FE-CR-001-F (CodeReviewPanel)
