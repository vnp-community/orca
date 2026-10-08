# FE-CV-TASK-054-03: `structure-tree-model` và `useStructureTree`

**From Solution:** [FE-CV-SOL-054-structure-lens](../solutions/FE-CV-SOL-054-structure-lens.md) mục 4.2
**Priority:** P1
**Area:** frontend / review-map + hooks
**File:** `structure-tree-model.ts`, `hooks/useStructureTree.ts` (mới), tests
**Depends on:** FE-CV-TASK-050-13, FE-CV-TASK-050-01
**Status:** [x] DONE (verified 2026-10-07: structure-tree-model.test + use-structure-tree.test 6/6 pass; folder-at-a-time loader instead of useCodeIntelPagedQuery, no cross-mount cache by headCommit)

## Context

- `structure {path?, depth?, limit?, pageToken?}` ⇒ `Env<ModuleGraph>`; `depth` ngoài 1..3 bị từ chối; `nextPageToken`; `useCodeIntelPagedQuery` (050-13).

## Việc cần làm

1. `flattenStructureRows(tree, expandedPaths)` thuần: hàng thư mục/file/đang tải/lỗi/`{shown}/{total}`+tải thêm.
2. `useStructureTree`: gốc `depth:2`, mở `depth:1`; loại trùng theo `path`; ≤ 2 đồng thời; giới hạn 5 000 nút; huỷ khi gỡ; cache theo `headCommit` index; giá trị thư mục = `symbolCount` hoặc tổng con (đánh dấu "≥").
3. Lọc theo chip (nhánh có file đổi).

## Kiểm thử

- Loại trùng; đồng thời ≤ 2 (đồng hồ giả); 5 000; phân trang; lỗi một thư mục; `path` chứa `..` bị chặn.

## Tiêu chí hoàn thành

- [ ] Test xanh; không gửi `depth` ngoài 1..3.

## Rủi ro

- `symbolCount` thư mục chưa rõ nghĩa (câu hỏi 1 của SOL).
