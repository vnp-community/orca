# FE-CV-TASK-054-02: `structure-area-model` và `structure-changed-index`

**From Solution:** [FE-CV-SOL-054-structure-lens](../solutions/FE-CV-SOL-054-structure-lens.md) mục 4.3
**Priority:** P1
**Area:** frontend / review-map
**File:** `structure-area-model.ts`, `structure-changed-index.ts` (mới), tests
**Depends on:** FE-CV-TASK-050-01, FE-CV-TASK-053-01
**Status:** [x] DONE

## Context

- `ModuleNode.area?`, `ChangedFile.area`, `status`, `path`; `uncoveredSymbols`, `violations` (theo file); `shared/cross-platform-path.ts` (`normalizeRuntimePathSeparators`).

## Việc cần làm

1. `deriveStructureArea(node|path, changedFiles)`: ưu tiên `node.area`, rồi `ChangedFile.area`, cuối tiền tố đường dẫn (`backend-go/services/<tên>` ⇒ `backend-go/<tên>`).
2. `assignAreaColors(visibleAreas)`: `--review-area-1..5` theo thứ tự chữ cái, còn lại `-6`; màu ngôn ngữ top 5.
3. `buildChangedPathIndex(changedFiles, uncoveredFiles, violationFiles)`: đếm theo tiền tố; trả cờ cho file và đếm cho thư mục; liệt kê file bị xoá riêng.

## Kiểm thử

- Khu vực lạ; thứ tự gán màu; Windows/POSIX; file xoá; đếm thư mục.

## Tiêu chí hoàn thành

- [ ] Thuần, xác định; không hex.

## Rủi ro

- `ChangedFile.area` có thể khác định nghĩa khu vực của treemap: ưu tiên `ModuleNode.area`.
