# FE-REQ-SOL-020: Xem, so sánh, chọn và duyệt Solution (Chẩn đoán, Findings, Answer)

> 🚧 **In Progress.** 2026-10-07: 020-01..04 DONE (vitest 90 pass); 020-05 PARTIAL (i18n xong; e2e chưa chạy, docs trang chưa có).

**CR:** [CR-REQ-020](../../../../../../docs/crs/v6/request-frontend/CR-REQ-020-solution-review-ui.md)
**Area:** frontend (`components/request/solution/`)
**Hợp đồng backend:** CR-REQ-016 mục 2.4 (`solution.list`, `solution.generate`, `solution.choose`, `approval.*`), 2.8 (mã lỗi). CONTRACT backend chưa có: chỗ tạm ghi "(tạm)".
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md), [v5/13-ai-provider-ui](../../../../tdd/v5/13-ai-provider-ui.md)

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `components/task/TaskAIDecompose.tsx` (mẫu đề xuất rồi áp dụng; không có so sánh phương án hay cổng duyệt), `lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`, `components/ui/{table,sheet,dialog,textarea,skeleton,progress,tabs}.tsx` (không có `alert`), `frontend/package.json` (có `react-markdown ^10.1.0`; `components/editor/MarkdownPreview.tsx` là trình xem của editor, rất nặng, không dùng lại). Không có UI cho `DecisionGate` (README v6 mục 1, O4: không trộn).

**Correction relative to CR-REQ-020:**

| # | CR-020 | Hợp đồng CR-016 / thực tế | Quyết định |
|---|---|---|---|
| 1 | `solution.chooseOption {solutionId, optionId, version}` | `solution.choose {requestId, solutionId, optionId, comment?}` (không có `version`) | Dùng CR-016; chống ghi chồng bằng `approval` (`expectedVersion`) |
| 2 | `approval.approve {approvalId, comment?, version}` | `approval.approve {id, expectedVersion, expectedDigest, comment?}` | Dùng CR-016; `expectedDigest` lấy từ `Approval.subjectDigest` |
| 3 | `solution.generate {requestId, feedback}` "chưa có trong README" | CR-016 2.4: `feedback?` (<=2000) và `idempotencyKey?` đã có | Dùng; cắt `feedback` ở 2000 ký tự phía client |
| 4 | Ánh xạ cổng `Approval.subject_type` | README 3.4: `solution` cho `solution|diagnosis`, `findings` cho spike, `answer` cho question; `hotfix`: không cổng | Giữ; hàm `solutionApprovalSubject(kind, requestType)` ở `solution-view-model.ts` |
| 5 | `content_ref` tải theo yêu cầu | CR-016 không có kênh đọc nội dung dài; `solutionView` chưa định nghĩa | Khi chưa có kênh: hiện tóm tắt trong `options`/`content`; nút "Xem đầy đủ" chỉ hiện khi có nội dung nội tuyến (tạm; câu hỏi mở 2) |
| 6 | `Solution.chosen_option` chỉ số | README v6 mục 8 số 5: số nguyên; CR-016 `optionId` | Gửi `SolutionOption.id` đã parse (`String(index)` khi thiếu); xác nhận khi có CONTRACT |

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/components/request/solution/
  RequestAnalysisTab.tsx            (mới) điểm cắm vào RequestDetailPane của SOL-019
  SolutionPanel.tsx                 SolutionVersionSwitcher.tsx   SolutionStatusBanner.tsx
  SolutionBody.tsx                  (rẽ theo kind)
  SolutionOptionCompare.tsx         SolutionOptionCard.tsx        SolutionComparisonTable.tsx
  DiagnosisView.tsx                 FindingsView.tsx              AnswerView.tsx
  SolutionGenerationState.tsx       SolutionDecisionBar.tsx
  RejectReasonDialog.tsx            (dùng chung cho Plan/Phase/hộp duyệt, SOL-021/022)
  RequestMarkdownContent.tsx        (mới) react-markdown an toàn, không HTML thô
  solution-view-model.ts            (mới) hàm thuần: trạng thái → banner/hành động, bảng so sánh, validate lý do
```

### 2.2 Dữ liệu và trạng thái

- `useSolutions(requestId)` (`solution.list {requestId, kind?, status?, pageSize, pageToken}`), `generate`, `choose` (SOL-018). `useApprovals({requestId, subjectType, status:'pending'})` (`approval.list`) cho cổng; `approve/reject` gắn `expectedVersion`, `expectedDigest`.
- Giả định (chưa kiểm chứng): cổng `pending` chỉ có cho Solution `proposed` mới nhất; các bản `superseded` không có Approval `pending`.
- Làm mới khi `request-event-bus` phát `solution.proposed|approved`, `approval.requested|decided`, `request.status_changed` với `requestId`.
- Ánh xạ `Solution.status` → UI (`solution-view-model.ts#getSolutionPresentation`):

| status | Banner | Hành động |
|---|---|---|
| `draft` | "AI đang soạn" + `Loader2` | không |
| `proposed` | "Chờ duyệt" (+ `dueAt` nếu có) | chọn phương án (kind `solution`), Duyệt, Từ chối |
| `approved` | "Đã duyệt" (`status-success`), phương án đã chọn đánh dấu | "Sinh Plan" nếu loại có Plan và chưa có (`request.generatePlan`, SOL-021) |
| `rejected` | "Bị từ chối" + lý do, người, thời điểm | "Sinh lại" |
| `superseded` | thu gọn "Bản cũ" | không |

### 2.3 Chọn, duyệt, từ chối

- `kind==='solution'`: radio trong `SolutionOptionCard`; "Duyệt phương án này" chạy chuỗi: (1) nếu `chosenOption` khác phương án đang chọn thì `solution.choose {requestId, solutionId, optionId}`; (2) `approval.approve {id, expectedVersion, expectedDigest, comment?}`. Bước 2 lỗi thì giữ lựa chọn, báo lỗi, cho thử lại và KHÔNG gọi lại bước 1 nếu `chosenOption` đã khớp. Chưa rõ backend có yêu cầu `choose` rồi mới `approve` hay `approve` đã gồm lựa chọn (câu hỏi mở 1).
- Dưới 2 phương án với `change_request`: cảnh báo "Cần ít nhất 2 phương án" (README 3.4), khoá Duyệt, cho "Sinh lại".
- `diagnosis|findings|answer`: không chọn phương án; nút "Duyệt" (nhãn "Chấp nhận" với `answer`). `hotfix`: chỉ xem, không `SolutionDecisionBar`.
- Từ chối: `RejectReasonDialog` (`Textarea`, tối thiểu 10 ký tự sau trim, `aria-invalid`, nút `variant="destructive"` khoá khi chưa đủ) rồi `approval.reject {id, expectedVersion, expectedDigest, comment}`; ô "Sinh lại dựa trên phản hồi" mặc định bật và gọi `solution.generate {requestId, feedback: comment, idempotencyKey}`.
- Phím: `Mod+Enter` (`isScreenSubmitShortcut`: `metaKey` Mac, `ctrlKey` nơi khác) gửi trong hộp từ chối; nhãn bằng `ShortcutKeyCombo`. KHÔNG gán phím tắt cho Duyệt.
- `conflict` (`APPROVAL_VERSION_CONFLICT`/`SOLUTION_VERSION_CONFLICT`): tải lại, banner "Solution vừa được cập nhật", giữ lựa chọn nếu phương án còn. `APPROVAL_ALREADY_DECIDED`: tải lại, UI thành chỉ đọc. `APPROVAL_EXPIRED`: nhãn "Quá hạn", chỉ cho "Sinh lại". Nút ghi khoá khi đang gửi.

### 2.4 Quyền

Không có `viewerCan`. Hiện nút và xử lý `APPROVAL_NOT_APPROVER`/`forbidden`: toast, đổi thanh quyết định thành "Bạn không có quyền duyệt" cho Approval đó (nhớ theo `approval.id`). Chế độ chỉ đọc khi `requestStatus` đã qua `awaiting_analysis_approval` hoặc Request `cancelled`. Luật tự duyệt thuộc CR-REQ-010; UI chỉ hiện lỗi backend.

### 2.5 Trạng thái rỗng, tải, lỗi

| Tình huống | UI |
|---|---|
| Request `analyzing`, chưa có Solution | Skeleton + "AI đang phân tích" |
| Loại không có bước phân tích | Không render tab (do SOL-019 ẩn) |
| Sinh lỗi | `SolutionGenerationState`: lỗi + "Sinh lại" |
| `solution.list` lỗi `network` | Banner "Thử lại" |
| `forbidden` | "Bạn không có quyền xem phân tích" |
| Phương án thiếu trường | Ô "Không có dữ liệu", không ném (parser) |
| `REQUEST_RATE_LIMITED` khi sinh lại | Toast + khoá nút ngắn |

Nội dung dài render bằng `RequestMarkdownContent` (react-markdown, không plugin HTML thô, link chỉ `http/https` và mở bằng mở-liên-kết ngoài của app như `MarkdownPreview` nếu có), không `dangerouslySetInnerHTML`.

### 2.6 i18n

Tiền tố `auto.components.request.solution.`, 5 locale: như CR-020 2.6 cộng `SolutionVersionSwitcher.*`, `SolutionComparisonTable.*`, `RejectReasonDialog.{title,placeholder,required,regenerate,submit}`, `SolutionDecisionBar.{approve,accept,reject,choose,noPermission,expired}`.

## 3. Quyết định thiết kế

- Một `SolutionPanel` cho 4 `kind`, rẽ ở `SolutionBody`.
- Thẻ và bảng so sánh cùng dữ liệu; bảng cho quyết định khó.
- Từ chối bắt buộc lý do, kiểm ở client, backend kiểm lại (`APPROVAL_COMMENT_REQUIRED`).
- Không gộp với `DecisionGate` (O4).
- `RejectReasonDialog` ở thư mục `solution/` nhưng không phụ thuộc Solution: SOL-021 và SOL-022 import.
- Không phím tắt cho Duyệt, chống duyệt nhầm.

## 4. Phụ thuộc và thứ tự

Phụ thuộc SOL-018, SOL-019 (`RequestDetailPane`). Backend CR-REQ-007/008/009 qua CR-016. Mở khoá SOL-021 (`RejectReasonDialog`), SOL-022. Thứ tự task: 020-01 → 020-02, 020-03 → 020-04 → 020-05.

## 5. Kiểm thử

Vitest: `solution-view-model` (bảng status, comparison, validate lý do), `SolutionOptionCompare`, `RejectReasonDialog`, `SolutionDecisionBar` theo quyền, mỗi `kind`, chuỗi choose→approve với lỗi bước hai, `conflict`. E2E `tests/e2e/request-solution-review.spec.ts` (mới; cần backend 007/008/009; chưa chạy): sinh, chọn, duyệt, từ chối rồi sinh lại. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/request/solution`.

## 6. Rủi ro và điểm chưa kiểm chứng

- Schema `SolutionOption` chưa chốt: bảng so sánh phụ thuộc trường chung.
- Chuỗi hai lời gọi có thể nửa chừng khi mất mạng.
- Nội dung AI có thể chứa Markdown độc hại hoặc rất dài.
- Cách lấy nội dung dài (`content_ref`) chưa có kênh.
- Giả định Approval pending chỉ cho bản mới nhất.

## 7. Câu hỏi mở

1. `choose` rồi `approve` hay một RPC? `approve` có gồm `optionId`?
2. Nội dung đầy đủ (`content_ref`) lấy bằng kênh nào?
3. Chẩn đoán của `bug|security|performance` dùng `subject_type=solution`: xác nhận.
4. Hiển thị đếm ngược `dueAt` và xử lý `expired` (CR-REQ-010)?
5. Người báo cáo có tự duyệt được không?

## 8. Tham chiếu

`/opt/repos/orca/docs/crs/v6/README.md` (3.4, 3.5, O4, mục 8), `/opt/repos/orca/docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md`, `/opt/repos/orca/frontend/src/renderer/src/components/task/TaskAIDecompose.tsx`, `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/ShortcutKeyCombo.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/ui/`, `/opt/repos/orca/frontend/package.json`, `/opt/repos/orca/guides/STYLEGUIDE.md`.
