# FE-REQ-SOL-041: Chỉnh phân tích bằng prompt tại cổng duyệt Analysis, danh sách phiên bản

> **📝 Chưa triển khai (0/4 task).** Soạn 2026-10-19 từ đọc mã.

**CR:** [CR-REQ-041](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md)
**Area:** frontend / request / solution
**Backend:** [BE-REQ-SOL-041](../../../../../backend-go/crs/v6/solution-analysis/solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md) (`AuthorizeRevise`, `revisionFeedback`, activity `analysis.revision_requested`)
**Phụ thuộc solution:** [FE-REQ-SOL-037](./FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) (tab Activity), CR-REQ-020-02 (Solution panel, đã làm)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-19 (dưới `frontend/src/renderer/src/components/request/`):

- `solution/RequestAnalysisTab.tsx` hiển thị `SolutionPanel`. `SolutionPanel.tsx` dựng: `useSolutionDecision` (dòng ~118), `SolutionDecisionBar`, `SolutionVersionSwitcher` (dòng ~364, `solutions={ordered} currentId={solution.id} onSelect={setViewId}`), trạng thái sinh, lỗi chạy, câu hỏi mở.
- `SolutionVersionSwitcher.tsx` (CR-REQ-020-02): liệt kê phiên bản mới nhất trước, nhãn "Older version" cho `superseded`; ẩn khi < 2 phiên bản.
- `SolutionDecisionBar.tsx`: nút Approve, Reject, Regenerate (`actions.includes('regenerate')`, gọi `decision.regenerate()` **không** phản hồi); `RejectReasonDialog` có tuỳ chọn "regenerate from this feedback" (hai lời gọi liên tiếp).
- `useSolutionDecision.ts`: `actions.generate({feedback?, idempotencyKey?})`, `clampFeedback` (2000, `solution-view-model.ts`), `reject(comment, regenerate)`, `regenerate()`.
- `shared/request-types.ts` `Solution` có `id, requestId, kind, status, content, options, ..., rejectionReason, generatedAt, reviewedAt, version?`; chưa có phản hồi chỉnh hay `seq`.
- Hook `hooks/useSolutions.ts` tải `solution.list`; kênh `SOLUTION_GENERATE: 'solution.generate'`.
- `activity/activity-entry-presentation.ts` ánh xạ `kind` → biểu tượng/nhãn (có `analysis.generated`); kind lạ phải chịu được.
- Mã lỗi đã có: `REQUEST_SOLUTION_FEEDBACK_TOO_LONG`, `REQUEST_SOLUTION_FORBIDDEN`, `REQUEST_SOLUTION_WRONG_STATE`, `REQUEST_ANALYSIS_NO_CONNECTION`, `REQUEST_SOLUTION_NO_CONNECTION`.

### Correction relative to CR-REQ-041

1. CR đề xuất file mới `AnalysisVersionList.tsx`; **đã có** `SolutionVersionSwitcher`: mở rộng nó (trạng thái, ghi chú phản hồi, số phiên bản), không tạo cái thứ hai.
2. Nút Regenerate không phản hồi và hộp thoại từ chối vẫn giữ nguyên; panel chỉnh là đường thứ ba, ưu tiên cho "chỉnh nhẹ".
3. Vị trí chèn là `SolutionPanel` (cùng nơi `SolutionDecisionBar`), không phải `RequestAnalysisTab`.

## 2. Giải pháp

### 2.1 Cây file

```
shared/request-types.ts, request-wire-parsers.ts, (solution parser)             (sửa) Solution.revisionFeedback, seq
renderer/src/components/request/solution/useAnalysisRevision.ts                 (mới)
renderer/src/components/request/solution/AnalysisRevisionPanel.tsx              (mới)
renderer/src/components/request/solution/revision-suggestions.ts                (mới) chip theo loại tài liệu
renderer/src/components/request/solution/SolutionPanel.tsx                      (sửa) chèn panel, chế độ chỉ đọc bản cũ
renderer/src/components/request/solution/SolutionVersionSwitcher.tsx            (sửa) trạng thái, ghi chú, số bản
renderer/src/components/request/activity/activity-entry-presentation.ts         (sửa) kind mới
renderer/src/i18n/locales/{en,es,ja,ko,zh}.json                                 (sửa)
tests/stages/e2e/orca-flows/ui-live/analysis-revision.web.e2e.ts                (mới)
```

### 2.2 Hook `useAnalysisRevision`

`useAnalysisRevision({request, solution, actions, onSettled})` trả `{canShow, busy, error, submit(text)}`. `submit` kiểm rỗng và độ dài (`clampFeedback`), gọi `actions.generate({feedback, idempotencyKey: crypto.randomUUID()})`; **không** gọi `reject`. Trạng thái `busy` kéo dài đến khi `request.status` rời `analyzing` (qua `useRequestFlow`/đăng ký sự kiện), không chỉ đến khi lời gọi trả về. Ánh xạ lỗi: `FEEDBACK_TOO_LONG`, `FORBIDDEN` (thông báo quyền, giữ nội dung đã gõ), `WRONG_STATE` ("Someone else is already revising or has decided", làm mới), `*_NO_CONNECTION` (giữ nguyên bản hiện tại), còn lại thông điệp chung. Không bao giờ xoá nội dung đã gõ khi lỗi.

### 2.3 Panel

`AnalysisRevisionPanel`: textarea (tối đa 2000, bộ đếm), chip gợi ý theo loại tài liệu (`revision-suggestions.ts`; Solution, chẩn đoán, Findings, Answer khác nhau), nút "Revise analysis". Hiện khi `request.status === 'awaiting_analysis_approval'`, đang xem **phiên bản hiện hành** và có thanh quyết định (UI không biết trước quyền; server chốt). Khi `busy` hiện "Analyzing version n+1…" và khoá Approve, Revise, Choose, Reject. Không hiện với `hotfix`, `task`, `docs`, `ops_request` (theo `request.flow`: không có cổng Analysis).

### 2.4 Danh sách phiên bản

`SolutionVersionSwitcher`: mỗi mục "Version n" (từ `seq`, nếu thiếu thì thứ tự theo `generatedAt`), ghi chú rút gọn 80 ký tự từ `revisionFeedback` ("First proposal" cho bản đầu), thời điểm, huy hiệu trạng thái (Current, Superseded, Rejected, Approved). Hiện khi ≥ 2 phiên bản (như cũ). Xem bản cũ: `SolutionPanel` chuyển sang chế độ **chỉ đọc**: ẩn Approve, Choose, Reject, Revise; hiện dải "You're viewing an older version. Back to current". Toàn bộ phản hồi hiện đầy đủ trong một khối thu gọn "What was asked for this version" (mọi người đọc được Request đều xem được).

### 2.5 Activity và i18n

`activity-entry-presentation.ts`: kind `analysis.revision_requested` (biểu tượng, nhãn "Requested changes to the analysis", nhóm Analysis), kind lạ vẫn có mặc định. Khoá i18n mới cho 5 locale.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| E1 | Panel gọi `generate` trực tiếp, không qua reject | Chỉnh nhẹ không phải là từ chối |
| E2 | Mở rộng `SolutionVersionSwitcher` | Đã có, tránh hai danh sách |
| E3 | `busy` theo trạng thái Request, không chỉ theo lời gọi | Lượt agent chạy nền |
| E4 | Không xoá nội dung gõ khi lỗi | Không mất công người dùng |
| E5 | Không giới hạn số lần chỉnh | Quyết định 2026-10-19 (hiển thị chi phí nếu có) |
| E6 | Hiện panel cùng chỗ thanh quyết định | UI không biết trước quyền approver |

## 4. Phụ thuộc và thứ tự

Cần backend TASK-REQ-041-01 và 041-02 (quyền, `revisionFeedback`). Thứ tự: 01 → 02 và 03 (song song) → 04.

## 5. Kiểm thử

- Parser: `revisionFeedback`, `seq` thiếu/sai kiểu. Hook: thành công, từng lỗi, `busy` đến khi trạng thái đổi, không xoá nội dung.
- Component: panel (rỗng, đếm, chip, đang chạy, lỗi), switcher (huy hiệu, ghi chú, chế độ chỉ đọc), `SolutionPanel` ẩn nút ở bản cũ, ẩn theo loại.
- E2E (`ui-live`, `--ai-stub`) trên repo fixture tài chính: chỉnh hai lần bằng giao diện, 3 phiên bản, Approval mới sau mỗi vòng; bốn kiểu tài liệu; approver không phải reporter; người lạ bị từ chối; Activity có dòng mới.

## 6. Rủi ro và điểm chưa kiểm chứng

- Số vòng không giới hạn: danh sách phiên bản dài; hiển thị gọn, cuộn nội bộ có giới hạn chiều cao (không lồng cuộn), và phân trang khi > 20.
- `seq` có thể thiếu với bản thất bại trước khi lập chỉ mục; dự phòng theo `generatedAt`.
- Chi phí mỗi vòng chỉ hiện khi agent báo số liệu (`AnalysisRun.stats`).

## 7. Câu hỏi mở

Còn một: hoàn tác về bản cũ (CR-REQ-041 mục 7); mặc định tạm: không làm.

## 8. Tham chiếu

- Các file ở mục 1; [CR-REQ-041](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md); [BE-REQ-SOL-041](../../../../../backend-go/crs/v6/solution-analysis/solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md); kế hoạch tổng: [`IMPLEMENTATION-PLAN-CR-REQ-039-042`](../../../../../IMPLEMENTATION-PLAN-CR-REQ-039-042.md)
