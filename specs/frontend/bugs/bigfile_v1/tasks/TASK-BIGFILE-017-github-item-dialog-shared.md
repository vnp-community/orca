# TASK-BIGFILE-017 — Move+Dedupe: `github-item-dialog-shared.ts`

**Loại:** Move (2 file nguồn cùng lúc — ngoại lệ nguyên tắc "1 file/task")
**Effort:** M · **Phụ thuộc:** — · **Status:** 🟡 Done một phần — xem "Kết quả thực tế" bên dưới
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md`

## Kết quả thực tế (2026-08-12)

Nội dung 2 symbol xác nhận **giống hệt nhau từng dòng** ở cả 2 file (nhánh 1
của mục "⚠️" bên dưới) — nhưng chỉ **`type ItemDialogTab` được move**;
`invalidateWorkItemDetailsCacheForKey` bị **chặn lại** (không move), vì task
doc gốc không phát hiện: hàm này đọc/ghi state module-private
(`workItemDetailsCache` Map, `workItemDetailsCacheGeneration` counter, gọi
`notifyWorkItemDetailsCache()`) — 1 phần của cả 1 subsystem cache SWR
~180 dòng (`WORK_ITEM_DETAILS_CACHE_MAX`, `WorkItemDetailsCacheEntry`,
`touchWorkItemDetailsCache`, `getWorkItemDetailsCacheKey`,
`invalidateWorkItemDetailsCacheByMatch`, ...) mà **mỗi file định nghĩa và
dùng ĐỘC LẬP** (~15 điểm gọi mỗi file, xem `GitHubItemDialog.tsx:1405-1580`
+ `:6886,6977,7001,7014,7015,7044,7045,7161` và tương ứng ở
`PullRequestPage.tsx`). Move riêng mỗi hàm sang file dùng chung sẽ:
- HOẶC âm thầm gộp 2 cache độc lập thành 1 (đổi hành vi runtime — 2 dialog
  hiện KHÔNG chia sẻ cache, gộp có thể gây cache 1 dialog bị evict bởi hoạt
  động ở dialog kia), HOẶC
- HOẶC cần đổi chữ ký hàm (nhận Map + setter qua tham số) — không còn là
  "copy nguyên văn" như task yêu cầu.

Đây là pattern giống hệt bài học ipc/pty.ts (TASK-BIGFILE-001..007) — state
module-private dùng chéo ngoài phạm vi khai báo của task. Đã KHÔNG ép move
phần này. `invalidateWorkItemDetailsCacheForKey` giữ nguyên định nghĩa độc
lập ở cả 2 file (không đổi).

Xác nhận không có caller nào khác của 2 symbol này ngoài chính 2 file nguồn
(grep toàn `frontend/src`):
- `invalidateWorkItemDetailsCacheForKey`: 0 external caller (chỉ tự gọi nội
  bộ mỗi file, dòng `GitHubItemDialog.tsx:7202` / `PullRequestPage.tsx:6920`).
- `ItemDialogTab`: import từ `GitHubItemDialog.tsx` bởi
  `frontend/src/renderer/src/components/TaskPage.tsx` (dòng 144, 3847, 3920,
  3941) — đây là lý do `GitHubItemDialog.tsx` cần `export type { ItemDialogTab }
  from './github-item-dialog-shared'` (re-export, không chỉ import) để không
  đổi import path của `TaskPage.tsx`. Không có import nào của
  `ItemDialogTab` từ `PullRequestPage.tsx` — xác nhận đúng giả định trong
  Output gốc ("không cần re-export").

`GitHubItemDialogProjectOrigin`/`PullRequestPageProjectOrigin` (nhánh 3):
xác nhận nội dung giống hệt nhau nhưng tên KHÁC nhau thật — giữ nguyên tại
chỗ ở mỗi file, không gộp, đúng như task doc gốc dự đoán.

`gitnexus impact`/`detect_changes` không khả dụng trong phiên này (MCP
server trả "Connection closed" ở mọi lần gọi) — thay bằng
`grep -rn` toàn `frontend/src` để xác nhận caller list (kết quả ở trên).

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
