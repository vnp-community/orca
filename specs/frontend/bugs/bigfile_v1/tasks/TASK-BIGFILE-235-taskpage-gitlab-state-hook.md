# TASK-BIGFILE-235 — Move: `useTaskPageGitLabState()` custom hook

**Loại:** Move (tách state thành custom hook — KHÔNG phải move cơ học thuần
như 027–031, xem lưu ý bên dưới) · **Effort:** M · **Phụ thuộc:** TASK-BIGFILE-027..031
đã xong · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)
**Sinh ra từ:** TASK-BIGFILE-032 (Investigate)

## Vì sao làm task này TRƯỚC (235, không phải cuối dãy 235–241)

GitLab là domain NHỎ NHẤT và CÔ LẬP NHẤT trong 7 domain được xác nhận ở
TASK-BIGFILE-032 (9 state, chỉ 2 effect có dependency ngoài domain — cả 2
chỉ phụ thuộc `taskSource`). Làm domain này trước để xác nhận pattern
"custom hook nhận state bootstrap làm tham số" hoạt động đúng, trước khi áp
dụng cho domain lớn hơn nhiều (Linear 80 state, task 239–241) — giống
khuyến nghị "làm domain nhỏ nhất trước" ở TASK-BIGFILE-035.

## ⚠️ Khác biệt với Move cơ học (027–031)

Đây KHÔNG phải di chuyển 1 hàm/type độc lập — là tách 9 `useState` + 2
`useEffect` (đang nằm giữa thân component `TaskPage`) ra 1 custom hook
riêng. Điểm khác biệt quan trọng:

- Các biến state hiện được dùng trực tiếp trong hàng chục nơi (JSX render,
  handler `onClick`, v.v.) rải khắp `TaskPage`. Sau khi tách, MỌI điểm dùng
  phải đổi từ biến cục bộ (`gitlabItems`) sang truy cập qua object trả về
  của hook (`gitlab.items` hoặc destructure `const { gitlabItems, ... } =
  useTaskPageGitLabState(...)` — chọn 1 trong 2 style, khuyến nghị
  destructure để giảm diff tại call site).
- Hook cần nhận `taskSource` làm tham số đọc (không sở hữu) — xem bảng dep
  bên dưới.

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- **Đọc lại đúng vị trí thật trước khi làm** — số dòng dưới đây xác nhận tại
  thời điểm viết task (sau TASK-BIGFILE-027..031, file 10,664 dòng); nếu có
  task khác chạy trước làm lệch dòng, xác nhận lại bằng
  `grep -n "gitlabFilter\|gitlabItems\|gitlabLoading" TaskPage.tsx` thay vì
  tin số dòng literal.
- 9 state (dòng 1554–1570):
  `gitlabFilter`, `gitlabItems`, `gitlabLoading`, `gitlabError`,
  `gitlabRefreshNonce`, `gitlabDialogItem`, `gitlabView`, `gitlabTodos`,
  `gitlabTodosLoading`
- 2 effect liên quan:
  - Dòng 1767–1769: mở GitLab work item từ deep-link
    (dep: `pageData.openGitLabWorkItem`)
  - Dòng 2749–2900: fetch chính (MR/issue list dòng 2749–2860, dep
    `taskSource, gitlabView, activeGitlabFilter, gitlabRefreshNonce,
    selectedReposKey`) + fetch Todos (dòng 2865–2900, dep `taskSource,
    gitlabView, gitlabRefreshNonce, primaryRepo`)
- Tham số đọc cần nhận từ ngoài (KHÔNG sở hữu trong hook):
  `taskSource`, `selectedRepos` (hoặc `primaryRepo`/`selectedReposKey` dẫn
  xuất từ đó), `pageData.openGitLabWorkItem`

## Output

- File mới: `frontend/src/renderer/src/components/use-task-page-gitlab-state.ts`
  (hoặc `.tsx` nếu cần JSX — kiểm tra thực tế, khả năng cao chỉ cần `.ts`)
- Hook export state + setter cần dùng ở nơi khác trong `TaskPage`, cộng thêm
  bất kỳ giá trị dẫn xuất nào hiện đang được tính ngay sau khối state (vd
  `gitlabEmptyState` ở dòng ~1571 — xác nhận đọc thực tế xem có phụ thuộc
  state khác domain hay không trước khi gộp vào hook).
- `TaskPage.tsx`: thay 9 dòng `useState` + 2 effect bằng 1 lời gọi
  `const { ... } = useTaskPageGitLabState({ taskSource, selectedRepos,
  openGitLabWorkItem: pageData.openGitLabWorkItem })`, cập nhật mọi điểm
  tham chiếu `gitlabX`/`setGitlabX` còn lại trong file sang tên trả về từ
  hook (giữ nguyên tên biến nếu destructure để giảm diff).

## Các bước

1. `gitnexus impact({target: "gitlabItems", direction: "upstream"})` (hoặc
   target đại diện khác) — dừng nếu risk HIGH/CRITICAL. Nếu gitnexus
   segfault (đã gặp khi làm 027–031 trong môi trường nhiều repo được index),
   dùng `grep -rn "gitlabItems\|gitlabFilter\|gitlabDialogItem" frontend/src`
   để xác nhận thủ công không có nơi nào ngoài `TaskPage.tsx` dùng các biến
   này.
2. Đọc lại đúng vùng dòng đã ước tính ở trên để XÁC NHẬN ranh giới thật.
3. Grep TOÀN FILE `TaskPage.tsx` cho từng tên trong 9 state + setter tương
   ứng — liệt kê đầy đủ điểm dùng ngoài khối khai báo/effect (JSX, handler)
   trước khi tách, để biết chính xác cần export gì từ hook.
4. Tạo `use-task-page-gitlab-state.ts`, copy nguyên văn logic effect + state,
   nhận tham số đọc như mô tả ở Input.
5. Sửa `TaskPage.tsx`: xoá state/effect cũ, gọi hook mới, sửa mọi điểm tham
   chiếu đã liệt kê ở bước 3.
6. `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false` (cờ
   `--composite false` để tránh nhiễu tsbuildinfo dùng chung nếu có nhóm
   khác chạy song song).

## Xác minh xong

- [ ] `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false`
- [ ] `pnpm exec oxlint frontend/src/renderer/src/components/TaskPage.tsx
      frontend/src/renderer/src/components/use-task-page-gitlab-state.ts`
- [ ] `pnpm exec vitest run --config config/vitest.config.ts
      src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
      (chạy từ `frontend/`)
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low (hoặc grep thủ
      công nếu gitnexus không khả dụng)
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `TaskPage.tsx` giảm thêm
      ~130–160 dòng (9 state + 2 effect + comment liên quan)
- [ ] Kiểm tra thủ công (hoặc test e2e nếu có) luồng GitLab tab: mở tab
      GitLab, đổi filter, mở MR/issue trong dialog, chuyển qua Todos —
      không phải chỉ dựa vào tsc xanh vì đây là thay đổi hành vi runtime
      (đổi cách state được sở hữu/truyền), không phải move cơ học thuần.

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/use-task-page-gitlab-state.ts
```
