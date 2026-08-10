# SOLUTION-FE-BIGFILE-005 — Gỡ trùng lặp & tách `GitHubItemDialog.tsx` + `PullRequestPage.tsx`

**Bug:** `../BUG-FE-BIGFILE-005-githubitemdialog.md` VÀ
`../BUG-FE-BIGFILE-007-pullrequestpage.md` (xử lý cùng nhau — xem lý do trong
`SOLUTION-FE-BIGFILE-001` mục 4)
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #6 — xem `SOLUTION-FE-BIGFILE-001` mục 3

---

## Vì sao xử lý chung

`PullRequestPage.tsx` (7,372 dòng) tự nhận trong comment dòng 1 là
"duplicated from GitHubItemDialog" — cả 2 file có `type ItemDialogTab`,
hàm `invalidateWorkItemDetailsCacheForKey` trùng tên gần như 1:1. Tách riêng
từng file trước khi gỡ trùng lặp sẽ nhân đôi công sức (phải tách logic giống
nhau ở 2 nơi).

## Giai đoạn 1 — Trích phần dùng chung (làm TRƯỚC, bắt buộc)

| File mới | Nội dung | Nguồn |
|---|---|---|
| `github-item-dialog-shared.ts` | `type ItemDialogTab`, `invalidateWorkItemDetailsCacheForKey` | `GitHubItemDialog.tsx:275,1467` (giữ nguyên, xoá bản trùng ở `PullRequestPage.tsx:250,1509`) |

**Bước thực hiện:**
1. `gitnexus impact({target: "invalidateWorkItemDetailsCacheForKey", direction: "upstream"})`
   — chạy RIÊNG cho từng file (`GitHubItemDialog.tsx` và `PullRequestPage.tsx`
   có thể trả về 2 symbol khác nhau dù trùng tên — xác nhận cả 2 caller list
   trước khi gộp).
2. So sánh nội dung 2 hàm/type trùng tên **theo đúng từng dòng** (không chỉ
   tin vào tên trùng) — nếu có sai khác dù nhỏ (edge case xử lý khác nhau
   giữa Issue/PR), ghi chú lại sai khác đó, KHÔNG tự ý gộp nếu chưa xác nhận
   sai khác là vô ý (bug) hay cố ý (khác biệt nghiệp vụ thật giữa 2 luồng).
3. Tạo `github-item-dialog-shared.ts`, copy bản của `GitHubItemDialog.tsx`
   (file "gốc", theo đúng comment của `PullRequestPage.tsx` xác nhận nó là
   bản sao).
4. `GitHubItemDialog.tsx` đổi `export type ItemDialogTab`/
   `export function invalidateWorkItemDetailsCacheForKey` thành
   `export { ... } from './github-item-dialog-shared'`.
5. `PullRequestPage.tsx` XOÁ định nghĩa trùng, import từ
   `github-item-dialog-shared.ts` thay vì tự định nghĩa lại — đổi
   `export type PullRequestPageProjectOrigin` (type riêng, không trùng) giữ
   nguyên tại chỗ.

## Giai đoạn 2 — Tách phần riêng từng file (sau khi giai đoạn 1 xanh)

Sau giai đoạn 1, đánh giá lại kích thước 2 file — nếu vẫn còn 1 component lớn
duy nhất mỗi file (không có sub-component export riêng như
`SourceControl.tsx`/`TaskPage.tsx`), áp dụng chiến lược đọc-trước-khi-tách:

1. Đọc nội bộ `export default function GitHubItemDialog({...})` (dòng
   6,738–cuối, ~1,100 dòng) và `export default function PullRequestPage({...})`
   (dòng 6,433–cuối, ~940 dòng) để xác định các khối JSX lớn (theo đúng
   comment gốc: header, conversation tab, checks tab, files tab).
2. Với MỖI file, tách theo tab: `<file>-header.tsx`, `<file>-conversation-tab.tsx`,
   `<file>-checks-tab.tsx`, `<file>-files-tab.tsx` — component cha giữ lại
   logic điều phối tab + state quản lý chung.
3. Vì 2 file có Primer-style header khác nhau (theo comment gốc của
   `PullRequestPage.tsx`), phần header KHÔNG dùng chung — chỉ 3 tab còn lại
   (conversation/checks/files) là ứng viên trích tiếp sang
   `github-item-dialog-shared.ts` nếu logic thực sự giống nhau sau khi đọc kỹ.

## Xác minh (cả 2 giai đoạn)

- `pnpm run typecheck`, `pnpm run lint`
- Test hiện có cho cả 2 file (nếu có)
- `gitnexus detect_changes({scope: "all"})` — đặc biệt chú ý risk_level, vì
  đây là sửa đồng thời 2 file cùng lúc (khác nguyên tắc chung ở
  `SOLUTION-FE-BIGFILE-001` mục 2.4 "không refactor 2 file cùng lúc" — NGOẠI
  LỆ áp dụng đúng cho cặp file trùng lặp này, vì tách riêng sẽ nhân đôi công
  sức, nhưng vẫn tách thành 2 giai đoạn con để review từng phần)
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

**Trung bình** — rủi ro chính không phải kỹ thuật (copy/paste rõ ràng) mà là
**xác nhận đúng 2 khối code trùng tên thực sự giống hệt nhau** trước khi gộp.
Nếu đã tồn tại sai khác âm thầm (bug tiềm ẩn ở 1 trong 2 nơi mà không ai phát
hiện do trùng lặp), việc gộp có thể vô tình sửa 1 bug thật hoặc tạo ra 1 bug
mới nếu chọn sai bản để giữ lại — bắt buộc so sánh dòng-với-dòng ở bước 2 của
Giai đoạn 1, không bỏ qua.
