# FE-REQ-TASK-045-05: `PlanRevisionPanel` ở cổng duyệt Plan, nối với `usePlanRunState` của 044, thẻ lỗi "Try again" và dọn hai đường sinh lại cũ

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `renderer/src/components/request/solution/{RevisionPromptBox,AnalysisRevisionPanel}.tsx` (+ test; mới/sửa), `renderer/src/components/request/plan/revision/{usePlanRevision,PlanRevisionPanel,PlanRevisionSection,plan-revision-errors,plan-revision-suggestions}.ts(x)` (+ test, mới; `PlanRevisionSection` gom thẻ lỗi + panel + dải để `RequestPlanTab` chỉ thêm một dòng), `renderer/src/components/request/plan/{RequestPlanTab,PlanApprovalBar,PlanDecisionButtons,PlanApprovalCard,PlanRunFailedCard}.tsx` (sửa; hai file cuối của 043/044), `renderer/src/hooks/usePlanDecision.ts` (sửa), test hiện có của các file đó
**Depends on:** FE-REQ-TASK-045-01, 045-02; **FE-REQ-TASK-044-02 và 044-03** (`usePlanRunState`, `derivePlanAutoDisplay`, `PlanRunFailedCard`, `PlanRewriteStrip`, cờ `request.planAuto`); FE-REQ-TASK-043-03 (cùng sửa `PlanApprovalBar`/`PlanApprovalCard`; hợp nhất theo hàm); backend BE-REQ-TASK-045-01, 045-02
**Status:** [ ] PARTIAL (2026-10-20): mọi tiêu chí không cần stack đã xong và có test; tiêu chí "Approval cũ bị huỷ và Approval mới xuất hiện sau khi lượt xong" và trải nghiệm thật cần backend CR-045 và stack: not run on the stack (spec `plan-revision`).

---

## Context

- Nút `plan-regenerate` ("Regenerate plan") ở `PlanApprovalBar.tsx` hiện cho Approval `pending`/`rejected` và gọi `regeneratePlan()` không `feedback` (`usePlanDecision.ts`). Tuỳ chọn "regenerate from this feedback" của `RejectReasonDialog` (`offerRegenerate` truyền từ `PlanApprovalBar` qua `PlanDecisionButtons`) chạy `reject` rồi `regeneratePlan()`. Theo mã backend cả hai lỗi trạng thái: `GeneratePlan` không phản hồi ở `awaiting_plan_approval` bị `REQUEST_STATE_STALE`; từ chối đưa Request về `request_backlog` (`TriggerPlanRejected`). Chưa chạy thật: task đầu viết test hành vi hiện có và, nếu có stack, kiểm bằng giao diện trước khi xoá.
- `AnalysisRevisionPanel.tsx` (192 dòng) nhận `revision` (từ `useAnalysisRevision`) và `kind`; textarea, bộ đếm theo ký tự Unicode, chip, Cmd/Ctrl+Enter theo `getShortcutPlatform()`, `aria-live`.
- `usePlanRunInFlight` chỉ báo chạy khi `!planTaskId`; FE-REQ-SOL-044 thay nó bằng `usePlanRunState` (bật cả khi đã có Plan) và thêm `derivePlanAutoDisplay` (`writing`/`failed`/`skipped`, cờ `replan`), `PlanRunFailedCard` (nhóm mã, `RequestAiSetupBanner`, nút Retry), `PlanRewriteStrip`.
- Khi lượt chỉnh chạy: `request.status = 'planning'`, `planTaskId` còn nguyên, cây cũ vẫn trả về từ `usePlanTree`, Approval cũ `cancelled`.
- Lượt chỉnh được server commit bằng job `origin = 'revision'` (CR-REQ-045 mục 2.3, CR-REQ-044 mục 2.6); cờ `request.planAuto` cho biết server có làm việc đó không. Khi cờ vắng (gateway cũ hoặc 044 tắt) thì commit từ tab như hôm nay và chỉ reporter/admin chỉnh được.

## Việc cần làm

1. **Tách `RevisionPromptBox`** khỏi `AnalysisRevisionPanel` không đổi hành vi: props `{draft, setDraft, onSubmit, busy, busyText, error, suggestions, labels{title,description,submit,placeholder}, testIdPrefix}`; `AnalysisRevisionPanel` dùng lại; test `AnalysisRevisionPanel.test.tsx` và `SolutionPanel.revision.test.tsx` giữ xanh không sửa (nếu phải sửa test là dấu hiệu đổi hành vi).
2. `plan-revision-errors.ts`: `classifyPlanRevisionError(error)` → `empty|tooLong|forbidden|wrongState|noConnection|notApplicable|noChange|baseRefGone|rateLimited|network|other` theo mã backend (`REQUEST_PLAN_FEEDBACK_TOO_LONG`, `REQUEST_PLAN_FORBIDDEN`, `REQUEST_STATE_STALE`, `REQUEST_PLAN_REVISION_IN_PROGRESS`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_PLAN_REVISION_NOT_APPLICABLE`, `REQUEST_PLAN_REVISION_NO_CHANGE`, `REQUEST_BASE_REF_NOT_FOUND`); thông điệp tiếng Anh như solution 2.5; mẫu `analysis-revision-errors.ts`.
3. `plan-revision-suggestions.ts`: chip theo engine (OpenSpec và native, nhãn ở solution 2.5); chip chỉ điền chữ, không gửi.
4. `usePlanRevision({request, engine, flowPlanKind, serverCommit, runState, onSettled})`: mẫu `useAnalysisRevision`: `canShow` = `status === 'awaiting_plan_approval'` và `flowPlanKind` là `plan` hoặc `task_list` và đang xem bản hiện hành; `submit(text)` kiểm rỗng/độ dài, rồi gọi `generatePlanPropose(id, {feedback, idempotencyKey: createBrowserUuid()})` (task 01) khi `serverCommit` (= `supportsPlanAuto(state)`), ngược lại `generatePlanProposeCommit(id, {feedback, idempotencyKey})`; chống bấm đúp bằng `useRef`; **không** gọi `reject`; `busy = calling || revising` với `revising` suy từ Request (`planning` kể từ khi gửi, hoặc về `awaiting_plan_approval` mà `version`/`updatedAt`/`planTaskId` chưa đổi so với lúc gửi); `nextVersion` = số bản hiện hành + 1; làm mới theo `useRequestSubscription` và `REQUEST_POLL_EVENT_TYPE`; không bao giờ xoá nội dung đã gõ khi lỗi; `draft` theo `requestId`.
5. **Không** viết hook thăm dò mới: `RequestPlanTab` đã gọi `usePlanRunState` (044); `PlanRevisionSection` nhận `view`/`busy` và `derivePlanAutoDisplay`. `usePlanRevision.busy = calling || revising || (runState.busy && display.replan)`; báo "Taking longer than expected" sau 240 giây (native) / 64 phút (openspec) kể từ lúc gửi nhưng không tự coi là hỏng.
6. `PlanRevisionPanel` (dùng `RevisionPromptBox`): tiêu đề "Request changes to this plan", mô tả ngắn "The AI revises the documents. You approve the new version afterwards.", nút "Revise plan", trạng thái bận "Writing version {{n}}…" / "Writing the next version…"; hiện khi `canShow` hoặc `busy`/đang ở `planning` có Plan.
7. Thẻ lỗi lượt chỉnh: `PlanRunFailedCard` của 044 hiển thị ở trạng thái `ready` khi `display.kind === 'failed' && display.replan` (mở rộng nơi đặt, thêm prop `onRetry` và `retryLabel` nếu 044 chưa có); thêm nhóm mã của CR này vào bảng gợi ý (`REQUEST_PLAN_REVISION_NO_CHANGE`: "The agent did not change any document. Say more precisely what to change."; `REQUEST_BASE_REF_NOT_FOUND`); nút **Try again** gọi `generatePlanPropose(id, {})` (hoặc `generatePlanProposeCommit` khi `!serverCommit`), **không** `request.retryPlan`; dòng "Or write a new request below"; không hiện khi `busy`. `PlanRewriteStrip` ẩn khi `PlanRevisionPanel` đang hiện "Writing version n…".
8. `RequestPlanTab`: chèn `PlanRevisionSection` (thẻ lỗi + panel) ngay dưới thanh duyệt (`PlanApprovalBar`/`PlanApprovalCard`), hiển thị ở mọi chế độ xem (Tree/Graph/Documents/Proposal); khi `revising` ẩn nút Approve/Reject (không có Approval `pending`); tách thành file con để không vượt `max-lines`.
9. **Dọn**: `PlanApprovalBar` bỏ nút `plan-regenerate` và biến `regenerating` liên quan cho Approval `pending`/`rejected` (cổng Plan); `PlanDecisionButtons` bỏ prop `offerRegenerate` (và `RejectReasonDialog` vẫn nhận nó cho cổng Analysis); `usePlanDecision.reject` bỏ tham số `options.regenerate` và nhánh gọi `regeneratePlan`; `regeneratePlan` giữ cho "Generate plan" của `PlanStates`. Cập nhật mọi test tham chiếu (`plan-approval-bars.test.tsx`, `RequestPlanTab.test.tsx`, `PlanApprovalCard.test.tsx`).
10. `PlanStates`/`plan-view-state.ts`: không đổi; `canGeneratePlan` giữ.

## Kiểm thử

- Hook `usePlanRevision`: thành công (gọi đúng tham số, không `reject`), từng lỗi (giữ chữ), chống bấm đúp, `busy` kéo dài theo trạng thái Request, ẩn theo loại/trạng thái, không hiện ở bản cũ.
- Tích hợp với `usePlanRunState`/`derivePlanAutoDisplay` (mock): `writing` + `replan` → busy; `failed` + `replan` → thẻ lỗi + Try again; `none` → không gì; cờ `request.planAuto` vắng → đường `generatePlanProposeCommit`.
- Component: panel (rỗng, đếm, chip, đang chạy, lỗi, phím tắt theo nền tảng), thẻ lỗi lượt chỉnh (Try again gọi `request.generatePlan`, không gọi `request.retryPlan`), `PlanApprovalBar` không còn `plan-regenerate`, `RejectReasonDialog` ở cổng Plan không còn tuỳ chọn regenerate.
- Test `AnalysisRevisionPanel` hiện có xanh sau khi tách.
- `pnpm vitest run`, kiểu, lint.

## Tiêu chí hoàn thành

- [ ] Chỉnh Plan bằng một thao tác, không qua từ chối; Approval cũ bị huỷ và Approval mới xuất hiện sau khi lượt xong (qua server commit). (còn lại: chưa chạy trên stack (not run on the stack))
- [x] Trong lúc chỉnh tab hiển thị rõ đang viết bản mới dù cây cũ còn đó; thất bại có "Try again" giữ phản hồi. Bằng chứng: `RequestPlanTab.revision.test.tsx`, `RequestPlanTab.auto.test.tsx` (đã cập nhật), `usePlanRevision.test.ts`.
- [x] Hai đường sinh lại cũ của cổng Plan không còn; "Generate plan" lần đầu vẫn hoạt động. Bằng chứng: `plan-approval-bars.test.tsx` (không `plan-regenerate`, hộp thoại từ chối không có tuỳ chọn regenerate, từ chối chỉ gọi `approval.reject`); `PlanStates.auto.test.tsx` và `RequestPlanTab.auto.test.tsx` (Generate plan giữ nguyên).
- [x] Panel Analysis hoạt động như cũ sau khi tách. Bằng chứng: `AnalysisRevisionPanel.test.tsx` và `SolutionPanel.revision.test.tsx` xanh, không sửa.
- [x] Đúng truy cập bàn phím và đọc màn hình; không file nào vượt `max-lines`. Bằng chứng: `PlanRevisionPanel.test.tsx` (phím tắt theo nền tảng, `aria-live`, `aria-invalid`), `oxlint` sạch.

## Rủi ro và lưu ý

- Chưa kiểm chứng bằng chạy thật rằng hai đường cũ lỗi; nếu thực tế chúng chạy được (ví dụ gateway chuyển tiếp khác), vẫn bỏ vì phản hồi khi từ chối không bao giờ thành feedback và từ chối đưa về backlog.
- Chạy `gitnexus_impact` cho `usePlanDecision`, `PlanDecisionButtons`, `RejectReasonDialog`, `AnalysisRevisionPanel`.
- FE-REQ-SOL-044 chưa triển khai: tên hook, thẻ và cờ lấy từ tài liệu v1.0, đối chiếu mã thật trước khi viết. Ba CR cùng sửa `RequestPlanTab.tsx` (249 dòng, trần 300), `PlanApprovalBar`, `PlanApprovalCard`: giữ phần của CR này trong file con.

## Ghi chú triển khai (2026-10-20)

- **File mới**: `solution/RevisionPromptBox.tsx` (tách từ `AnalysisRevisionPanel`, không đổi hành vi: `testIdPrefix`, nhãn qua props, khoá i18n cũ giữ nguyên), `plan/revision/{usePlanRevision.ts,PlanRevisionPanel.tsx,PlanRevisionSection.tsx,plan-revision-errors.ts,plan-revision-suggestions.ts}` + test. **Sửa**: `solution/AnalysisRevisionPanel.tsx` (dùng `RevisionPromptBox`), `plan/RequestPlanTab.tsx`, `plan/PlanApprovalBar.tsx` (bỏ `plan-regenerate`), `plan/PlanDecisionButtons.tsx` (bỏ `offerRegenerate`), `hooks/usePlanDecision.ts` (`reject` không còn `options.regenerate`; `regeneratePlan` giữ cho "Generate plan"), `plan/PlanRunFailedCard.tsx` (biến thể `revise`: tiêu đề "The plan could not be revised", nút "Try again", dòng "Or write a new request below.", gợi ý cho `REQUEST_PLAN_REVISION_NO_CHANGE` và `REQUEST_BASE_REF_NOT_FOUND`), `plan/PlanRunNotice.tsx` (`hideStrip`, `hideFailure`, thẻ lỗi luôn là biến thể `revise` vì chỉ nằm trên Plan đã có), `hooks/usePlanAutoGeneration.ts` (trả thêm `run` và `refresh`). Test cũ đã cập nhật: `plan-approval-bars.test.tsx` (hai test), `RequestPlanTab.auto.test.tsx` (ca "rewrite": panel nói "Writing", dải `plan-rewrite-strip` bị ẩn như task yêu cầu).
- **Lệnh**: `vitest run` trên `plan/revision`, `RequestPlanTab*`, `plan-approval-bars*`, `AnalysisRevisionPanel.test.tsx`, `SolutionPanel.revision.test.tsx`, `PlanRunFailedCard.test.tsx`: 17 file, 216 test xanh. Toàn vùng `plan solution activity hooks/usePlan shared/request store/slices/request`: 81 file, 1269 test xanh. `tsc` mốc 113; `oxlint` sạch; `RequestPlanTab.tsx` 263 dòng mã (cap 300).
- **Thiết kế `usePlanRevision`**: `busy = calling || retrying || revising || (server đang viết lại)`; `revising` suy từ Request (`planning` kể từ khi gửi, hoặc quay về `awaiting_plan_approval` mà `version`/`updatedAt`/`planTaskId` chưa đổi) và kết thúc khi lượt thất bại **mới** (so `runId` đã thấy lúc gửi với `run.runId` hiện tại, để thẻ lỗi của lượt trước không làm tab hết bận); không bao giờ xoá chữ khi lỗi; không gọi `reject`; `serverCommit = supportsPlanAuto` thì `generatePlanPropose`, ngược lại `generatePlanProposeCommit` (chỉ reporter/admin chỉnh được; thông điệp `forbidden` nói rõ). "Try again" = `generatePlanPropose(id, {})` (hoặc `…Commit`), không `request.retryPlan`. Sau 240 giây (native) / 64 phút (openspec) hiện "Taking longer than expected" nhưng vẫn bận. Panel hiện khi `viewingCurrent` và (đang ở cổng, hoặc `planning` còn Plan và lượt không đang chạy, hoặc bận); cổng Approval/Card ẩn khi `busy`. Chỉ kind `plan`/`task_list` (hotfix `single_task` không).
- **Sai lệch**: (1) `PlanRunFailedCard` nhận `variant` thay vì `onRetry`/`retryLabel` (đã có `onRetry`). (2) `usePlanAutoGeneration` sửa nhỏ (trả `run`, `refresh`) vì không thể phân biệt lỗi của lượt mới với lỗi lượt cũ nếu thiếu `runId`; hook thuộc 044. (3) `RequestPlanTab.auto.test.tsx` có một ca đổi kỳ vọng (xem trên).
- **Blast radius** (grep): `PlanDecisionButtons` dùng ở `PlanApprovalBar`, `PhaseApprovalBar`, `PlanGateChips`, `PlanApprovalCard`, `PlanRunAllButton` (chỉ prop `offerRegenerate` bỏ; không ai khác truyền); `usePlanDecision.reject` chỉ `PlanDecisionButtons` gọi; `RejectReasonDialog` giữ `offerRegenerate` cho cổng Analysis (`SolutionDecisionBar`); `usePlanAutoGeneration` chỉ `RequestPlanTab`; `AnalysisRevisionPanel` chỉ `SolutionPanel`; `PlanRunNotice` chỉ `RequestPlanTab`.
- **Chưa kiểm**: chuỗi thật "chỉnh → Approval cũ huỷ → job `revision` commit → Approval mới" cần backend CR-045 (BE-045-01, 02, 05, 08) và stack; hai đường cũ lỗi trạng thái trên stack thật chưa được xác nhận (đã bỏ vì không thành feedback và từ chối đưa về backlog).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
