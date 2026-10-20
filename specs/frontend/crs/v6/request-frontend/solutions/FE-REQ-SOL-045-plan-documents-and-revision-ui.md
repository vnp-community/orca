# FE-REQ-SOL-045: Tab Plan: xem tài liệu OpenSpec, đề xuất Plan native, chỉnh Plan bằng prompt và danh sách phiên bản

> **📝 Chưa triển khai (0/8 task).** Soạn 2026-10-20 từ đọc mã.

**CR:** [CR-REQ-045](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md)
**Area:** frontend / request / plan
**Backend:** [BE-REQ-SOL-045](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-045-plan-documents-and-revision.md) (`request.planRevisions`, tham số `revision`, `idempotencyKey`, quyền approver, activity `plan.revision_requested`)
**Phụ thuộc solution:** [FE-REQ-SOL-021](./FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) (tab Plan), [FE-REQ-SOL-041](./FE-REQ-SOL-041-analysis-revision-by-prompt.md) (mẫu panel chỉnh, danh sách phiên bản), [FE-REQ-SOL-037](./FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) (tab Activity), **[FE-REQ-SOL-044](./FE-REQ-SOL-044-automatic-plan-generation.md)** (`usePlanRunState`, `derivePlanAutoDisplay`, `PlanRunFailedCard`, `PlanRewriteStrip`, cờ `request.planAuto`; commit Plan phía server), [FE-REQ-SOL-043](./FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md) (cùng sửa `PlanApprovalBar`, `PlanApprovalCard`, `RequestPlanTab`). Backend: BE-REQ-SOL-044 (job `revision`).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-20 (dưới `frontend/src/renderer/src/`, trừ `shared/` là `frontend/src/shared/`):

- `components/request/plan/RequestPlanTab.tsx` (249 dòng): `usePlanTree`, `usePlanRunInFlight`, `usePlanDecision`; `planView: 'tree' | 'graph'` (state cục bộ); khi `state !== 'ready'` hiện `PlanStates`; khi sẵn sàng: `PlanSummaryHeader` (chứa `ToggleGroup` Tree/Graph), `PlanDriftSection`, `PlanApprovalCard` (chế độ `continuous` và Approval `pending`) hoặc `PlanApprovalBar`, rồi `PlanTree`/`PlanOutline`/`GraphPanel`, `PlanGateChips`.
- `PlanApprovalBar.tsx`: nút Approve/Reject (`PlanDecisionButtons`, `offerRegenerate`) và nút `plan-regenerate` ("Regenerate plan") hiện khi Approval `pending` hoặc `rejected`; gọi `decision.regeneratePlan()`.
- `hooks/usePlanDecision.ts`: `regeneratePlan = () => run('generatePlan','plan', () => generatePlanProposeCommit(request.id))` **không truyền `feedback`**; `reject(approval, comment, {regenerate})` gọi `APPROVAL_REJECT` rồi `regeneratePlan()`; không có đường nhận phản hồi.
- `hooks/request-plan-generation.ts`: `generatePlanProposeCommit(id, options)` với `options.feedback`, `onEngine`; "propose" → thăm dò `PLAN_PROPOSAL` (2 giây; ngân sách 240 giây native, 64 phút openspec) → "commit". `ProposeValue` không có kiểu cho `bundle`, `checks`, `knowledge`.
- `hooks/usePlanRunInFlight.ts`: `waiting = request.status === 'planning' && !request.planTaskId`; lúc chỉnh `planTaskId` còn nguyên nên hook trả `false` (G7). `shared/request-rpc-methods.ts` có `GENERATE_PLAN`, `PLAN_PROPOSAL`; **không có** hằng cho `request.planBundleFile` hay `request.planRevisions` (`grep planBundleFile|PLAN_BUNDLE_FILE` trong `frontend/src` rỗng).
- `components/request/plan/plan-view-state.ts`: `derivePlanViewState` (`ready` khi có cây Plan; `generating` chỉ khi chưa có `planTaskId`), `canGeneratePlan(status)`.
- `components/request/solution/`: `RequestMarkdownContent.tsx` (89 dòng; react-markdown, không HTML thô, chỉ `http(s)`, không dùng `remark-gfm`; thu gọn sau `MARKDOWN_COLLAPSE_LENGTH` = 4000 ký tự bằng `Sheet`), `AnalysisRevisionPanel.tsx` (192 dòng, nhận `revision` từ `useAnalysisRevision` và `kind`), `analysis-revision-errors.ts`, `revision-suggestions.ts`, `OlderVersionBanner.tsx`, `SolutionVersionSwitcher.tsx`, `useAnalysisRevision.ts` (214 dòng).
- `shared/request-types.ts`: `OrcaRequest.engine?: RequestEngine` (dòng 142) và `executionMode`; `store/slices/request.ts` giữ `requestFlowFeatures` (cờ tính năng của `request.flowStatus`, mẫu `REQUEST_BASE_REF_FEATURE` ở `shared/request-base-ref.ts:16`).
- Activity: `components/request/activity/{activity-entry-presentation,activity-entry-text}.ts` (CR-REQ-041 đã thêm `analysis.revision_requested`; kind lạ phải chịu được).
- Mã lỗi Plan đã có: `REQUEST_PLAN_FORBIDDEN`, `REQUEST_PLAN_FEEDBACK_TOO_LONG`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_STATE_STALE` (trạng thái sai); `components/request/plan/plan-decision-error-message.ts` ánh xạ một phần.
- i18n: `i18n/locales/{en,es,ja,ko,zh}.json`; khoá dạng `auto.components.request.<khu>.<tên>`.

### Correction relative to CR-REQ-045 / brief

1. Brief: "`regeneratePlan()` takes no feedback although `generatePlanProposeCommit` supports options.feedback" đúng; thêm: cả hai đường cũ (nút Regenerate và tuỳ chọn trong hộp thoại từ chối) đều lỗi trạng thái theo mã backend (CR G1), nên task 05 **bỏ** chúng ở cổng Plan thay vì sửa.
2. `usePlanRunInFlight` bị FE-REQ-SOL-044 thay bằng `usePlanRunState` (bật cả khi đã có `planTaskId`, trả cả view lượt chạy và `autoPlan`); CR này **dùng** hook đó cho lượt chỉnh và không viết hook thăm dò thứ hai. Nếu 044 chưa vào, task 05 dừng (phụ thuộc cứng).
3. Phản hồi của đề xuất native lấy từ `request.planProposal {runId}` của **lượt tương ứng bản**, không dùng "lượt mới nhất" (sẽ rỗng khi lượt chỉnh đang chạy).
4. `RequestMarkdownContent` không có GFM: task list `- [ ]` của `tasks.md` hiện thành gạch đầu dòng kèm ký tự "[ ]"; cần tuỳ chọn `gfm`.

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `frontend/src/`)

```
shared/request-plan-revision.ts                                       (mới) kiểu PlanRevision, PlanBundleFileEntry, hằng REQUEST_PLAN_REVISIONS_FEATURE
shared/request-plan-revision-parsers.ts (+ test)                      (mới) parser dây không ném lỗi
shared/request-rpc-methods.ts                                         (sửa) PLAN_BUNDLE_FILE, PLAN_REVISIONS
renderer/src/hooks/request-plan-generation.ts                         (sửa) generatePlanPropose (chỉ propose) + feedback, idempotencyKey
renderer/src/hooks/usePlanRevisions.ts, usePlanBundleFile.ts          (mới) danh sách phiên bản, tải từng file có nhớ
renderer/src/hooks/usePlanProposalOfRun.ts                            (mới) đề xuất của một lượt (native)
renderer/src/components/request/plan/documents/
  PlanDocumentsView.tsx, PlanDocumentTabs.tsx, PlanDocumentBody.tsx   (mới)
  PlanChecksPanel.tsx, PlanProposalView.tsx                           (mới)
  plan-document-model.ts (+ test)                                     (mới) thứ tự tab, nhãn, nhận dạng bị lược
renderer/src/components/request/plan/revision/
  usePlanRevision.ts                                                  (mới)
  PlanRevisionPanel.tsx                                               (mới)
  plan-revision-errors.ts, plan-revision-suggestions.ts               (mới)
  PlanVersionSwitcher.tsx, plan-version-history.ts                    (mới)
renderer/src/components/request/solution/RevisionPromptBox.tsx        (mới) tách từ AnalysisRevisionPanel, không đổi hành vi
renderer/src/components/request/solution/{AnalysisRevisionPanel,RequestMarkdownContent,OlderVersionBanner}.tsx (sửa)
renderer/src/components/request/plan/{RequestPlanTab,PlanSummaryHeader,PlanApprovalBar,PlanDecisionButtons}.tsx (sửa)
renderer/src/hooks/usePlanDecision.ts                                 (sửa) bỏ regenerate-sau-từ-chối
renderer/src/components/request/activity/{activity-entry-presentation,activity-entry-text}.ts (sửa)
renderer/src/i18n/locales/{en,es,ja,ko,zh}.json                       (sửa)
tests/stages/e2e/orca-flows/ui-live/plan-documents.web.e2e.ts, plan-revision.web.e2e.ts (+ steps) (mới)
```

### 2.2 Dữ liệu và hook

- `usePlanRevisions(request)` gọi `request.planRevisions` (trang đầu 20, nút "Show older" tải trang tiếp), làm mới khi `request.version`/`status` đổi hoặc nhận sự kiện `plan.*`/`approval.*`/`request.status_changed` (cùng mẫu `PROGRESS_EVENT_RE` của `useAnalysisRevision`). Trả `{revisions, current, loadMore, isLoading, error, supported}`; `supported=false` khi gateway cũ (`Unimplemented`/không có cờ `request.planRevisions`) và mọi tính năng xem phiên bản ẩn đi, vẫn hiển thị Documents của bản hiện hành bằng `request.planProposal`.
- `usePlanBundleFile(requestId, path, revision)` gọi `request.planBundleFile`, nhớ kết quả theo `(requestId, revision, path)` trong phiên (Map trong `useRef`, xoá khi `requestId` đổi), huỷ khi rời tab, trạng thái `idle|loading|ready|error|omitted|pruned`.
- `usePlanProposalOfRun(requestId, runId)` gọi `request.planProposal {id, runId}` cho đề xuất native và cho `bundle`/`checks`/`knowledge` của bản hiện hành.
- Parser mới không ném lỗi: trường thiếu → mặc định an toàn (`files: []`, `textKept: true`, `change: 'unchanged'`).

### 2.3 Xem tài liệu (OpenSpec)

`PlanDocumentsView` nhận `request`, phiên bản đang xem (`revision`), và hiển thị:

1. **Dải kiểm tra** (`PlanChecksPanel`, thu gọn, chỉ bản hiện hành): các `checks` theo mức (info/warning), khối tri thức "Reported by the authoring agent" (có sẵn không, snapshot, số phần tử đã đọc, danh sách `gaps`, `conflicts`), cảnh báo `SECRETS_REDACTED` nếu có. Bản cũ: dòng "Checks are shown for the current version only".
2. **Thanh tab tài liệu** (`PlanDocumentTabs`): thứ tự do `plan-document-model.ts` quyết định: `proposal.md` ("Proposal"), `design.md` ("Design"), mỗi `specs/<capability>/spec.md` (nhãn = `<capability>`, chung nhóm "Specs"), `tasks.md` ("Tasks"), file khác ("Other"). Mỗi tab mang huy hiệu `New`/`Changed`/`Removed` (so với bản liền trước; chỉ khi có bản trước) và biểu tượng "too large" khi `omitted`. Dùng `ToggleGroup`/`Tabs` shadcn, điều khiển bằng bàn phím, `aria`.
3. **Thân tài liệu** (`PlanDocumentBody`): chỉ tải file của tab đang chọn; trạng thái tải (skeleton), lỗi (nút "Try again"), `omitted` ("This file is too large to display (N KB). It is on the proposal branch `request/<n>-proposal`."), `pruned` ("The text of this version is no longer kept"), rỗng. Hai chế độ "Rendered" / "Source" (nút chuyển); Rendered dùng `RequestMarkdownContent` với `gfm`; Source là `<pre>` cuộn (không lồng cuộn với trang, giới hạn chiều cao theo khung). Văn bản dài hơn `MARKDOWN_COLLAPSE_LENGTH` theo cơ chế sẵn có.
4. Chỉ hiện khi `request.engine === 'openspec'`; tab "Documents" nằm cạnh Tree/Graph trong `PlanSummaryHeader`.

### 2.4 Đề xuất Plan native (chỉ đọc)

`PlanProposalView` (tab "Proposal", chỉ khi `request.engine !== 'openspec'`): `title`, `summary`, `notes` (Markdown), từng Phase (tiêu đề, mô tả, phụ thuộc Phase) và task (tiêu đề, mô tả Markdown, loại, nhãn, `depends on #n` theo chỉ số, ước lượng giờ, huy hiệu "Irreversible"); `prompt_template` thu gọn trong `<details>`. Không cho sửa. Chọn bản qua `PlanVersionSwitcher`; bản cũ lấy `runId` từ `PlanRevision.runId`.

### 2.5 Chỉnh bằng prompt

- `usePlanRevision({request, planApproval, onSettled})` trả `{canShow, busy, error, draft, setDraft, submit, nextVersion}`: `canShow` khi `request.status === 'awaiting_plan_approval'`, `planKind` của luồng là `plan` hoặc `task_list` (từ `request.flow`, dự phòng `REQUEST_FLOW_REGISTRY[type].plan`; `single_task` của hotfix và `none` bị loại, giao diện không rẽ nhánh theo `type === 'hotfix'` theo quy ước của `request-flow-registry.ts`), và đang xem bản hiện hành. `submit(text)` kiểm rỗng/độ dài (`FEEDBACK_MAX_LENGTH` = 2000, đếm theo ký tự Unicode), gọi `generatePlanPropose(id, {feedback, idempotencyKey: createBrowserUuid()})`; **không** gọi `reject`, **không** commit từ tab (CR-REQ-044). `busy = calling || revising` với `revising` suy từ Request: `planning` kể từ khi gửi, hoặc về `awaiting_plan_approval` mà `version`/`updatedAt`/`planTaskId`/Approval chưa đổi so với lúc gửi.
- Theo dõi lượt chỉnh dùng **`usePlanRunState`** của FE-REQ-SOL-044 (`view`, `busy`, `refresh`) và `derivePlanAutoDisplay`: `kind: 'writing'` (có `replan`, và `origin === 'revision'` trong `autoPlan`) là "đang viết bản mới"; `kind: 'failed'` là lỗi lượt chỉnh. Ngân sách hiển thị ≤ 64 phút (openspec) / 240 giây (native); quá thì báo "Taking longer than expected" và vẫn cho Try again.
- `PlanRevisionPanel`: dựng từ `RevisionPromptBox` (tách khỏi `AnalysisRevisionPanel`: textarea, bộ đếm `n / 2000` theo ký tự Unicode, chip, nút gửi, `aria-live`, phím tắt Cmd/Ctrl+Enter theo nền tảng bằng `getShortcutPlatform`). Tiêu đề "Request changes to this plan", nút "Revise plan", `busy`: "Writing version {n}…". Chip theo engine (nhãn tiếng Anh, 5 locale): OpenSpec "Add a task for tests", "Split the biggest task", "Tighten the design", "Add a scenario for the edge case"; native "Split the largest task", "Add a rollback task", "Reorder to ship the risky part first", "Merge tiny tasks".
- Khoá khi `busy`: thanh duyệt (`PlanApprovalBar`/`PlanApprovalCard`) ẩn nút Approve/Reject (Approval cũ đã huỷ nên không còn gì để bấm; tránh nháy), chế độ xem phiên bản khác vẫn dùng được.
- **Thẻ lỗi lượt chỉnh**: dùng `PlanRunFailedCard` của 044 (nhóm mã timeout, gián đoạn, đầu ra không hợp lệ, bận, lỗi commit; thêm nhóm của CR này: `REQUEST_PLAN_REVISION_NO_CHANGE`, `REQUEST_BASE_REF_NOT_FOUND`) hiển thị cả ở trạng thái `ready` khi đang thay Plan (044 chỉ hiển thị nó trong `PlanStates`, task 05 mở rộng chỗ đặt). Nút **Try again** ở ngữ cảnh thay Plan (`display.replan`) gọi `request.generatePlan` không phản hồi (backend kế thừa phản hồi, tạo job `revision` mới và dùng quyền "thử lại của chính mình"), **không** gọi `request.retryPlan` (thuộc luồng sinh Plan đầu, quyền khác); thêm gợi ý "or write a new request below". `PlanRewriteStrip` của 044 không hiện khi `PlanRevisionPanel` đã hiện dòng "Writing version n…" (tránh hai dải).
- **Đường commit**: khi cờ `request.planAuto` có (server commit bằng job `revision`), `usePlanRevision.submit` chỉ propose (`generatePlanPropose`). Khi không có cờ (gateway cũ hoặc 044 tắt) dùng `generatePlanProposeCommit(id, {feedback, idempotencyKey})` như hôm nay; lúc đó chỉ reporter/admin chỉnh được (backend từ chối approver bằng `REQUEST_PLAN_FORBIDDEN`) và thông điệp `forbidden` nói rõ.
- Dọn đường cũ: `PlanApprovalBar` bỏ nút `plan-regenerate` cho Approval `pending`/`rejected`; `PlanDecisionButtons` bỏ `offerRegenerate` ở cổng Plan; `usePlanDecision.reject` bỏ nhánh `regenerate`; `regeneratePlan` còn dùng cho "Generate plan" của `PlanStates` (lần đầu, `planning` chưa có Plan) và nay chỉ propose khi CR-REQ-044 có; test cũ cập nhật.
- Lỗi (`plan-revision-errors.ts`, mẫu `analysis-revision-errors.ts`): rỗng ("Describe what should change before you send it."), `tooLong` (`REQUEST_PLAN_FEEDBACK_TOO_LONG`), `forbidden` (`REQUEST_PLAN_FORBIDDEN`: "Only the approvers of this plan can request changes. Your text is kept."), `wrongState` (`REQUEST_STATE_STALE`, `REQUEST_PLAN_REVISION_IN_PROGRESS`: "Someone else is already revising this plan or has decided. It was refreshed; your text is kept."), `noConnection` (`REQUEST_PLAN_AI_UNAVAILABLE`: "No dev server is connected… the current plan and its approval are unchanged."), `notApplicable` (`REQUEST_PLAN_REVISION_NOT_APPLICABLE`), `rateLimited`, `network`, `other`. Không bao giờ xoá nội dung đã gõ khi lỗi.

### 2.6 Danh sách phiên bản

`PlanVersionSwitcher` (mẫu `SolutionVersionSwitcher`, không dùng chung kiểu `Solution`): hiện khi ≥ 2 bản; mục "Version n", ghi chú rút gọn 80 ký tự từ `revisionFeedback` ("First proposal" cho bản đầu), thời điểm, huy hiệu (Current, Superseded), số file đổi (OpenSpec) và chi phí nếu `stats`; phân trang theo trang của `usePlanRevisions`. Chọn bản cũ: các chế độ Documents/Proposal chuyển sang **chỉ đọc bản đó** với `OlderVersionBanner` (thêm prop `testId`; "You're viewing an older version. Back to current"), cây Tree/Graph và thanh duyệt luôn là của bản hiện hành; panel chỉnh ẩn. Khối thu gọn "What was asked for this version" hiện toàn bộ phản hồi. Bản bị lược chữ: "The text of this version is no longer kept" kèm chỉ mục file.

### 2.7 Activity và i18n

`plan.revision_requested`: biểu tượng, nhóm lọc `changes`, mở tab Plan, nhãn "Requested changes to the plan", **không bao giờ** có dòng chi tiết nội dung (kể cả khi backend gửi thêm `feedback`). Khoá i18n mới cho 5 locale; `verify:localization-catalog` và `verify:localization-coverage`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| U1 | Documents là chế độ xem thứ ba của tab Plan, không phải tab mới của Request | Cùng Approval và cùng phiên bản với cây; giữ điều hướng gọn |
| U2 | Tải từng file theo tab, nhớ trong phiên | Bộ tài liệu tới 1 MiB; ít người mở hết |
| U3 | `gfm` là tuỳ chọn của `RequestMarkdownContent` | Không đổi cách Solution hiển thị |
| U4 | Tách `RevisionPromptBox`, không sao chép panel Analysis | Một nơi giữ phím tắt, bộ đếm, truy cập |
| U5 | Bỏ hai đường sinh lại cũ của cổng Plan | Chúng lỗi trạng thái (G1); panel chỉnh thay thế |
| U6 | Giao diện không commit Plan | CR-REQ-044; đóng tab không kẹt |
| U7 | Hook tiến trình mới thay vì sửa `usePlanRunInFlight` | Hook cũ phục vụ lần sinh đầu |
| U8 | Kiểm tra và tri thức chỉ cho bản hiện hành | Backend không lưu chúng theo bản |
| U9 | Cây Tree/Graph luôn của bản hiện hành | Task-service chỉ giữ Plan hiện hành làm cây thao tác |
| U10 | Bản đã lược chữ vẫn liệt kê, có chỉ mục file | Không mất lịch sử |

## 4. Phụ thuộc và thứ tự

01 (kiểu, parser, RPC) → 02 (hook dữ liệu) → 03 (viewer OpenSpec) và 04 (native) song song → 05 (panel chỉnh và dọn; cần FE-REQ-TASK-044-02 và 044-03) → 06 (phiên bản) → 07 (activity, i18n) → 08 (e2e). Backend: 01, 02 cần BE-045-05 và 08 để chạy thật; có thể làm trước với mock. Phát hành sau CR-REQ-044.

## 5. Kiểm thử

- Parser: thiếu trường, sai kiểu, `change` lạ, `files` rỗng. Hook: tải/lỗi/nhớ, huỷ khi đổi Request, gateway cũ (`supported=false`).
- Component: viewer (tab theo thứ tự, huy hiệu, bị lược, bị lược chữ, Source/Rendered, lỗi, tải), `PlanChecksPanel`, `PlanProposalView`, panel chỉnh (rỗng, đếm, chip, đang chạy, lỗi, giữ chữ, phím tắt theo nền tảng), thẻ lỗi lượt chỉnh dùng `PlanRunFailedCard` (Try again gọi `request.generatePlan`), `PlanVersionSwitcher` (bản cũ chỉ đọc, ẩn panel), ẩn theo loại/trạng thái/quyền, `PlanApprovalBar` không còn nút Regenerate; test `AnalysisRevisionPanel` hiện có xanh sau khi tách.
- E2E (`ui-live`, `--ai-stub`, LedgerLite): `plan-documents` (ca native "Proposal"; ca OpenSpec bằng dữ liệu seed nếu fake claude chưa tác giả OpenSpec) và `plan-revision` (chỉnh hai lần bằng giao diện, 3 phiên bản, Approval mới sau mỗi vòng, bản cũ chỉ đọc, "Try again", Activity). `--real` loại B: một vòng chỉnh `bug` native.

## 6. Rủi ro và điểm chưa kiểm chứng

- **FE-REQ-SOL-044 chưa triển khai**: tên `usePlanRunState`, `derivePlanAutoDisplay`, `PlanRunFailedCard`, cờ `request.planAuto` lấy từ tài liệu v1.0; ba CR (043, 044, 045) cùng sửa `RequestPlanTab.tsx` (249 dòng, trần 300), `PlanApprovalBar.tsx`, `PlanApprovalCard.tsx`: mọi phần mới của CR này nằm trong file con, `RequestPlanTab` chỉ thêm vài dòng gắn kết, hợp nhất theo hàm.
- `RequestPlanTab` đã 249 dòng: thêm chế độ xem và panel phải tách (không thêm `max-lines` disable).
- Văn bản tài liệu do agent viết: render qua `react-markdown` không HTML thô và chỉ `http(s)`; "Source" dùng `<pre>` văn bản thuần (không `dangerouslySetInnerHTML`).
- Bản đã lược chữ và Request cũ (chỉ bản hiện hành có tài liệu): thông điệp phải đúng, không hứa nội dung không có.
- Vùng `orca:begin plan` bị ẩn trong Rendered (HTML comment): có "Source" để xem.
- Chưa dựng giao diện hay chạy thử.

## 7. Câu hỏi mở

Hai câu hỏi của CR (mục 7 của CR-REQ-045). Riêng giao diện: nếu quyết định "Keep the previous version" (câu 1) là có, thẻ lỗi lượt chỉnh thêm nút thứ hai.

## 8. Tham chiếu

- Các file ở mục 1; [CR-REQ-045](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md); [BE-REQ-SOL-045](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-045-plan-documents-and-revision.md); [FE-REQ-SOL-041](./FE-REQ-SOL-041-analysis-revision-by-prompt.md); `guides/STYLEGUIDE.md`
