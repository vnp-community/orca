# SOLUTION-FE-BIGFILE-003 — Tách `TaskPage.tsx` (12,833 dòng)

**Bug:** `../BUG-FE-BIGFILE-003-taskpage.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #8 — xem `SOLUTION-FE-BIGFILE-001` mục 3
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

## Cấu trúc file mới

13 sub-component top-level đã có sẵn, chia theo domain (Linear / Jira /
GitHub / chung):

| File mới | Nội dung | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `task-page-types.ts` | `type LinearProjectTab`, `type LinearGroupSection`, `type LinearIssueListRow` | 600–650 | ~50 |
| `task-page-linear-cells.tsx` | `LinearStateCell` | 651–883 | ~230 |
| `task-page-jira-banner.tsx` | `TaskPageJiraErrorBanner` | 884–1,068 | ~185 |
| `task-page-github-cells.tsx` | `GHStatusCell`, `ReviewChipAvatar`, `GitHubAssigneeAvatar`, `GitHubIssueLabelSelector`, `GitHubIssueAssigneeSelector`, `GHAssigneesCell`, `PRReviewCell`, `PRChecksCell`, `PRMergeCell` | 1,069–2,950 | ~1,880 |
| `task-page-pagination.tsx` | `PaginationBar` | 2,951–3,055 | ~105 |
| `TaskPage.tsx` (giữ nguyên tên) | `export default function TaskPage(...)` (component chính, 83 `useState`/58 `useEffect`) + `export { ... }` cho các file trên | 3,056–cuối | ~9,777 (giảm từ 12,833, nhưng VẪN còn Critical) |

## Các bước thực hiện

1. `gitnexus impact` cho 13 sub-component + 3 type — xác nhận không có nơi
   nào khác import trực tiếp (khả năng thấp vì đây là component nội bộ trang,
   nhưng vẫn cần xác nhận đúng quy trình).
2. Tách `task-page-types.ts` trước (không JSX).
3. Tách theo nhóm domain (Linear → Jira → GitHub → pagination) — mỗi nhóm 1
   commit, xác nhận xanh trước khi sang nhóm tiếp theo. Nhóm GitHub lớn nhất
   (~1,880 dòng, 9 component) — có thể tách tiếp thành 2 file nếu cần
   (`task-page-github-review-cells.tsx` cho `ReviewChipAvatar`/`PRReviewCell`/
   `PRChecksCell`/`PRMergeCell`, `task-page-github-assignee-cells.tsx` cho
   phần còn lại).
4. `TaskPage.tsx` thêm `export { ... }` cho 5 file trên.

Sau bước 1–4: `TaskPage.tsx` giảm từ 12,833 → ~9,777 dòng — **vẫn là file lớn
nhất repo sau `orca-runtime.ts`** (Giai đoạn 1 của `SOLUTION-FE-BIGFILE-002`
đã đưa `orca-runtime.ts` xuống dưới mức này). Cần Giai đoạn 2.

## Giai đoạn 2 — Tách component `TaskPage` chính (~9,777 dòng, 83 state)

Đây là bước rủi ro cao nhất trong file này — 83 `useState` là mật độ state
cực cao, cần xác định rõ nhóm state trước khi tách để tránh vỡ closure.

1. **Trước khi tách bất kỳ dòng nào**, liệt kê toàn bộ 83 `useState` và phân
   nhóm theo domain (Linear-specific / GitHub-specific / Jira-specific /
   filter-chung / pagination-chung / UI-chung như modal mở/đóng). Đây là bước
   phân tích bắt buộc, không đoán trước trong solution doc — thực hiện bằng
   cách đọc trực tiếp file sau khi Giai đoạn 1 hoàn tất (lúc đó file đã ngắn
   hơn, dễ đọc hơn).
2. Ứng viên tách theo custom hook (không đổi vị trí file, chỉ đổi cách tổ
   chức code — bước AN TOÀN hơn tách hẳn ra file/component riêng):
   - `useTaskPageLinearState()` — state + effect liên quan Linear
   - `useTaskPageGitHubState()` — state + effect liên quan GitHub PR/Issue
   - `useTaskPageJiraState()` — state + effect liên quan Jira
   - `useTaskPageFilters()` — state filter/search chung
3. Sau khi tách thành custom hook (vẫn trong `TaskPage.tsx` hoặc tách hook
   sang file riêng `use-task-page-linear-state.ts`, ...), component chính chỉ
   còn JSX layout + gọi 4 hook trên — lúc này mới đánh giá có cần tách JSX
   layout ra file riêng hay không (thường không cần nếu component chính đã
   xuống dưới ~500–800 dòng).

## Xác minh

- `pnpm run typecheck`, `pnpm run lint`
- Test hiện có (`TaskPage.test.tsx` nếu có)
- `gitnexus detect_changes({scope: "all"})`
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

- Giai đoạn 1: **Thấp** — 13 sub-component đã tách biệt rõ, chỉ cần di
  chuyển.
- Giai đoạn 2: **Cao** — 83 `useState` trong 1 component là dấu hiệu rõ ràng
  của state quản lý phức tạp, có khả năng cao có state phụ thuộc chéo giữa
  các nhóm domain tưởng như độc lập (vd: filter chung ảnh hưởng cả Linear lẫn
  GitHub). Bước 1 (phân nhóm state) phải làm cẩn thận, có thể cần viết test
  bổ sung cho các luồng chính trước khi tách hook.
