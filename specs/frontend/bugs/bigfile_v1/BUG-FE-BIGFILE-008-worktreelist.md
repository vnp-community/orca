# BUG-FE-BIGFILE-008 — `WorktreeList.tsx` (6,877 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-008](./solutions/SOLUTION-FE-BIGFILE-008-worktreelist.md)
**Module:** `frontend/src/renderer/src/components/sidebar/WorktreeList.tsx`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

6,877 dòng. `/* eslint-disable max-lines */` không kèm giải thích (giống
`SourceControl.tsx`, xem `BUG-FE-HLD-006`). 5 `useState` + 23 `useEffect` —
mật độ effect cao so với state, gợi ý nhiều logic đồng bộ hoá với external
state (store, DOM measurement, drag-and-drop) hơn là local UI state thuần túy.

8 export top-level là pure helper function, không phải component:

```
324   export function countRecordKeysByReference(...)
334   export function shouldAdjustWorktreeSidebarMeasuredRowScroll(...)
342   export function resolvePendingSidebarReveal(...)
1050  export function renderRowContainsWorktree(...)
1166  export function getRenderRowKey(...)
1191  export function getWorktreeDragGroups(...)
1223  export function canKeepImportedWorktreesHidden(...)
5150  export function installWorktreeVisibleRefreshVisibilityListener(...)
```

Khoảng cách dòng 1223 → 5150 (gần 4,000 dòng) không có export nào khác — rất
có thể là component chính (danh sách worktree, virtualized row rendering, drag
handling) nằm gọn trong khoảng này.

## Hậu quả

- 8 pure helper function đã tách biệt rõ (không JSX, không hook) nhưng vẫn
  nằm trong file `.tsx` 6,877 dòng — không có lý do kỹ thuật để giữ chung với
  phần component.
- Sidebar danh sách worktree là 1 trong những khu vực UI được tương tác nhiều
  nhất (mọi thao tác chuyển worktree đều qua đây) — file lớn làm review mọi
  thay đổi liên quan khó khăn hơn mức cần thiết.

## Bằng chứng

```
wc -l WorktreeList.tsx                                 → 6877
grep -n "^export function" WorktreeList.tsx              → 8 pure helper
grep -c "useEffect(" WorktreeList.tsx                    → 23
head -1 WorktreeList.tsx                                 → "/* eslint-disable max-lines */" (không giải thích)
```

## Đề xuất fix

1. Tách 8 helper function đã liệt kê sang `worktree-list-helpers.ts` (đã có
   tiền lệ tên tương tự trong cùng thư mục:
   `sidebar/worktree-list-groups.ts`, #80 trong bảng tổng, 1,356 dòng) —
   không đổi hành vi, giảm ngay ~800–1,000 dòng khỏi file chính.
2. Với vùng component chính (dòng ~1223–5150), xác định các khối logic độc
   lập (drag-and-drop, virtualized scroll, row rendering) để tách tiếp theo
   sau bước 1 — 23 `useEffect` nên được nhóm lại theo domain trước khi quyết
   định ranh giới tách.
3. Bổ sung `-- Why:` cho disable comment (theo `BUG-FE-HLD-006`).

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- File cùng thư mục đã tách trước: `sidebar/worktree-list-groups.ts` (#80)
- Liên quan: `BUG-FE-HLD-006` (disable không giải thích)
