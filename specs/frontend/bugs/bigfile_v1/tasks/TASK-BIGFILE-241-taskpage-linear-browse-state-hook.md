# TASK-BIGFILE-241 — Move: `useTaskPageLinearBrowseState()` custom hook

**Loại:** Move (tách state thành custom hook — xem lưu ý "khác biệt với Move
cơ học" ở TASK-BIGFILE-235, áp dụng y hệt ở đây) · **Effort:** L (lớn nhất
trong 235–241 — làm CUỐI CÙNG, sau khi pattern đã xác nhận ổn định qua 6
task trước)
**Phụ thuộc:** TASK-BIGFILE-235, 239, 240 đã xong · **Status:** 🚧 Blocked (scope) — xem ghi chú kết quả nhóm taskpage-hooks (235-241), chưa thực thi trong phiên này, KHÔNG có thay đổi nào ở TaskPage.tsx cho task này
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-003-taskpage.md` (Giai đoạn 2)
**Sinh ra từ:** TASK-BIGFILE-032 (Investigate)

## Input

- File nguồn: `frontend/src/renderer/src/components/TaskPage.tsx`
- **Xác nhận lại vị trí thật** bằng
  `grep -n "selectedLinearIssueId\|linearMode,\|linearBoardUpdatingIssueIds" TaskPage.tsx`
  trước khi làm — đây là domain LỚN NHẤT (51 state), khả năng lệch dòng cao
  nhất nếu có task khác chạy trước.
- 51 state:
  - Chọn/mở issue (2051–2054, 3): `selectedLinearIssueId`,
    `selectedLinearIssueFallback`, `selectedLinearIssueCanFloat`
  - Browse chính (2225–2303, 48): `linearMode`, `linearIssues`,
    `linearIssueLimit`, `linearIssuePage`, `linearIssueLoadingTargetPage`,
    `linearIssuesHasMore`, `linearLoading`, `linearError`,
    `linearSearchInput`, `appliedLinearSearch`, `linearAttributeFilter`,
    `linearViewMode`, `linearGroupBy`, `linearOrderBy`,
    `linearDisplayProperties`, `linearTeamPropertyTouched`,
    `linearRefreshNonce`, `linearProjectSearchInput`,
    `appliedLinearProjectSearch`, `linearProjectsResult`,
    `linearProjectsLoading`, `linearProjectsError`, `selectedLinearProject`,
    `selectedLinearProjectDetail`, `linearProjectDetailLoading`,
    `linearProjectDetailError`, `linearProjectTab`,
    `linearProjectIssuesResult`, `linearProjectIssueLimit`,
    `linearProjectIssuePage`, `linearProjectIssueLoadingTargetPage`,
    `linearProjectIssuesLoading`, `linearProjectIssuesError`,
    `linearCustomViewsResult`, `linearCustomViewsLoading`,
    `linearCustomViewsError`, `selectedLinearCustomView`,
    `linearProjectParentView`, `linearCustomViewIssuesResult`,
    `linearCustomViewIssueLimit`, `linearCustomViewIssuePage`,
    `linearCustomViewIssueLoadingTargetPage`,
    `linearCustomViewProjectsResult`, `linearCustomViewContentsLoading`,
    `linearCustomViewContentsError`, `linearBoardDraggingIssueId`,
    `linearBoardDragOverKey`, `linearBoardUpdatingIssueIds`
- Effect liên quan (nhiều — dep thật, TASK-BIGFILE-032): 2106–2112,
  3212–3251 (pagination), 5194–5219, **5230–5374** (fetch issues chính,
  ~144 dòng), 5389–5439 (fetch projects), 5452–5527 (project detail +
  project issues), **5536–5646** (custom views fetch, 2 effect lớn),
  5655–5683 (đồng bộ selection). Đây là khối effect DÀY ĐẶC NHẤT trong toàn
  component — đọc kỹ từng effect, không giả định tất cả độc lập nhau (nhiều
  effect trong nhóm này gọi lẫn nhau qua state, vd `linearMode` quyết định
  effect nào fetch).
- **Vì đây là domain to nhất, cân nhắc tách hook thành nhiều file con theo
  sub-view** (issues / projects / custom-views / board) nếu 1 file vượt
  ngân sách oxlint max-lines (300 cho `.ts`) — quyết định khi đọc thực tế,
  giống cách TASK-BIGFILE-030 phải chia GitHub cells thành 2 file.
- Tham số đọc cần nhận: `taskSource`, `taskResumeApplied`, `linearConnected`,
  `selectedLinearWorkspaceId`, `linearTaskSourceContext`,
  `pageData.openLinearIssue`, output của TASK-BIGFILE-240 (teams state) nếu
  đã hoàn tất — dọn lại phụ thuộc tạm thời đã ghi chú ở TASK-BIGFILE-239/240
  tại bước này.

## Output

- File mới: `frontend/src/renderer/src/components/use-task-page-linear-browse-state.ts`
  (hoặc chia nhỏ hơn — xem lưu ý trên)
- `TaskPage.tsx`: thay 51 `useState` + ~15 effect bằng lời gọi hook.

## Các bước

Giống quy trình TASK-BIGFILE-235, nhưng với khối lượng lớn hơn nhiều — cân
nhắc chia làm nhiều commit nhỏ theo sub-view (issues trước, rồi projects,
rồi custom-views, rồi board) thay vì 1 commit khổng lồ, miễn mỗi bước vẫn
giữ `tsc`/test xanh.

## Xác minh xong

- [ ] `pnpm exec tsc --noEmit -p frontend/tsconfig.json --composite false`
- [ ] `pnpm exec oxlint` trên các file đã đổi
- [ ] `pnpm exec vitest run --config config/vitest.config.ts
      src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
- [ ] `gitnexus detect_changes({scope: "all"})` hoặc grep thủ công
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm thêm ~550–650 dòng;
      sau 235–241, `TaskPage.tsx` dự kiến còn ~8,800–9,000 dòng (vẫn Critical,
      cần thiết kế lớp điều phối chéo-domain riêng cho phần lõi còn lại —
      xem "KHÔNG sinh task Move cho" ở solution doc mục Giai đoạn 2)
- [ ] Kiểm tra thủ công đầy đủ luồng Linear tab: issues list + pagination,
      project overview + issues, custom views, board drag&drop.

## Rollback

```
git checkout -- frontend/src/renderer/src/components/TaskPage.tsx
rm frontend/src/renderer/src/components/use-task-page-linear-browse-state.ts
```
