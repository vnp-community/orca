# TASK-BIGFILE-017 — Move+Dedupe: `github-item-dialog-shared.ts`

**Loại:** Move (2 file nguồn cùng lúc — ngoại lệ nguyên tắc "1 file/task")
**Effort:** M · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`

## Input

- File nguồn 1: `frontend/src/renderer/src/components/GitHubItemDialog.tsx`
  — đọc đúng dòng 275 (`type ItemDialogTab`), dòng 1,467
  (`invalidateWorkItemDetailsCacheForKey`)
- File nguồn 2: `frontend/src/renderer/src/components/PullRequestPage.tsx` —
  đọc đúng dòng 250 (`type ItemDialogTab` — TRÙNG TÊN), dòng 1,509
  (`invalidateWorkItemDetailsCacheForKey` — TRÙNG TÊN)

## ⚠️ Bắt buộc: so sánh nội dung TRƯỚC khi gộp

Không tin vào tên trùng — đọc cả 2 định nghĩa `invalidateWorkItemDetailsCacheForKey`
(và context xung quanh `type ItemDialogTab`) ở CẢ 2 file, so sánh **từng
dòng**. Có 3 khả năng:

1. **Giống hệt nhau** → gộp bình thường theo các bước dưới.
2. **Khác nhau ở chi tiết nhỏ** (edge case xử lý khác) → KHÔNG tự ý chọn 1
   bản để giữ — dừng lại, ghi rõ sai khác vào cả 2 bug doc
   (`../BUG-FE-BIGFILE-005-githubitemdialog.md` và
   `../BUG-FE-BIGFILE-007-pullrequestpage.md`), hỏi lại người yêu cầu task
   trước khi quyết định.
3. **`PullRequestPageProjectOrigin`** (dòng 296 của `PullRequestPage.tsx`) —
   type này KHÔNG trùng tên với `GitHubItemDialogProjectOrigin` (dòng 309 của
   `GitHubItemDialog.tsx`) — đây là 2 type RIÊNG, không gộp, giữ nguyên tại
   chỗ ở mỗi file.

## Output (nếu xác nhận giống hệt — nhánh 1)

- File mới: `frontend/src/renderer/src/components/github-item-dialog-shared.ts`
  chứa `type ItemDialogTab`, `invalidateWorkItemDetailsCacheForKey`
- `GitHubItemDialog.tsx`: thay 2 định nghĩa gốc bằng
  `export { type ItemDialogTab, invalidateWorkItemDetailsCacheForKey } from './github-item-dialog-shared'`
- `PullRequestPage.tsx`: XOÁ 2 định nghĩa trùng, import trực tiếp từ
  `./github-item-dialog-shared` (không cần re-export nếu
  `PullRequestPage.tsx` không phải nơi module khác import các symbol này từ)

## Các bước

1. `gitnexus impact` cho `invalidateWorkItemDetailsCacheForKey` — chạy RIÊNG
   cho từng file (chúng là 2 symbol khác nhau dù trùng tên) — lấy đủ 2 danh
   sách caller trước khi gộp.
2. So sánh nội dung theo mục "⚠️" ở trên.
3. Nếu giống hệt: tạo file mới, copy bản của `GitHubItemDialog.tsx` (file
   "gốc" theo đúng comment của `PullRequestPage.tsx` xác nhận nó là bản sao).
4. Sửa cả 2 file nguồn theo đúng "Output" ở trên.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — xác nhận risk, vì đây là
      sửa 2 file cùng lúc
- [ ] `node scripts/find-frontend-bigfiles.mjs` — cả 2 file giảm nhẹ (không
      nhiều, phần dùng chung nhỏ so với tổng thể — mục tiêu chính là loại bỏ
      trùng lặp, không phải giảm dòng đáng kể ở bước này)
- [ ] Test liên quan (cả 2 file) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/GitHubItemDialog.tsx \
                 frontend/src/renderer/src/components/PullRequestPage.tsx
rm frontend/src/renderer/src/components/github-item-dialog-shared.ts
```
