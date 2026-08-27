# BUG-FE-BIGFILE-003 — `TaskPage.tsx` (12,833 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-003](./solutions/SOLUTION-FE-BIGFILE-003-taskpage.md)
**Module:** `frontend/src/renderer/src/components/TaskPage.tsx`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

12,833 dòng — file lớn thứ 2 toàn `frontend/src`, ~32× ngưỡng oxlint mặc định
cho `.tsx` (400 dòng). Component chính `export default function TaskPage()`
chỉ bắt đầu ở dòng 3,056 — nghĩa là **hơn 3,000 dòng phía trước** (types +
~13 sub-component riêng) đều là phần hỗ trợ, không phải bản thân trang.

Các sub-component top-level đã tồn tại sẵn trong CÙNG file (dấu hiệu rõ nhất
cho thấy việc tách file là khả thi, không cần thiết kế lại logic):

```
651   function LinearStateCell(...)
884   function TaskPageJiraErrorBanner(...)
1069  function GHStatusCell(...)
1512  function ReviewChipAvatar(...)
1535  function GitHubAssigneeAvatar(...)
1558  function GitHubIssueLabelSelector(...)
1652  function GitHubIssueAssigneeSelector(...)
1761  function GHAssigneesCell(...)
2092  function PRReviewCell(...)
2642  function PRChecksCell(...)
2722  function PRMergeCell(...)
2951  function PaginationBar(...)
3056  export default function TaskPage(...)   ← component chính
```

Component chính tự nó dùng **83 `useState`** và **58 `useEffect`** trong toàn
file — mật độ state rất cao cho 1 trang.

## Hậu quả

- 13 sub-component độc lập (mỗi cái chỉ liên quan 1 cột/1 ô trong bảng
  Linear/GitHub/Jira) đang sống chung 1 file với component cha có 83 state —
  bất kỳ thay đổi nhỏ nào ở 1 cell component cũng buộc diff xuất hiện trong
  file 12,833 dòng, khó review.
- Không có ranh giới rõ giữa "logic hiển thị 1 cell" và "logic điều phối toàn
  trang" — tăng rủi ro side-effect ngoài ý muốn khi sửa 1 cell.

## Bằng chứng

```
wc -l TaskPage.tsx                                    → 12833
grep -n "^function \|^export default function" ...    → 13 sub-component + 1 default
grep -c "useState(" TaskPage.tsx                       → 83
grep -c "useEffect(" TaskPage.tsx                      → 58
```

## Đề xuất fix

1. **Tách trước, rủi ro thấp nhất**: 13 sub-component top-level (dòng
   651–2,951) → mỗi nhóm liên quan chuyển sang file riêng theo domain:
   - `task-page-linear-cells.tsx` (`LinearStateCell`)
   - `task-page-jira-cells.tsx` (`TaskPageJiraErrorBanner`)
   - `task-page-github-cells.tsx` (`GHStatusCell`, `ReviewChipAvatar`,
     `GitHubAssigneeAvatar`, `GitHubIssueLabelSelector`,
     `GitHubIssueAssigneeSelector`, `GHAssigneesCell`, `PRReviewCell`,
     `PRChecksCell`, `PRMergeCell`)
   - `task-page-pagination.tsx` (`PaginationBar`)
2. Sau khi tách xong nhóm cell, đo lại kích thước phần types (dòng 600–650) —
   nếu type dùng chung giữa nhiều cell, cân nhắc `task-page-types.ts`.
3. Component `TaskPage` chính (83 state) là ứng viên cho bước tách tiếp theo —
   xem xét custom hook hoá theo domain (Linear state riêng, GitHub state
   riêng, Jira state riêng) sau khi các cell component đã tách xong và kích
   thước file dễ quan sát hơn.

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- Chính sách: `AGENTS.md` → "Lint Rules: Do Not Disable Max Lines"
