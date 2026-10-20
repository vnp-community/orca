# FE-REQ-SOL-044: Tab Plan khi Orca tự sinh Plan: đang viết, thẻ lỗi bền vững có Retry, không tự chạy được

> **📝 Chưa triển khai (0/4 task).** Soạn 2026-10-20 từ đọc mã.

**CR:** [CR-REQ-044](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-044-automatic-plan-generation-after-analysis-approval.md)
**Area:** frontend / request
**Backend:** [BE-REQ-SOL-044](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-044-automatic-plan-generation.md) (`request.planProposal` thêm `autoPlan`, kênh `request.retryPlan`, tính năng `request.planAuto`, mã lỗi)
**Phụ thuộc solution:** [FE-REQ-SOL-037](./FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) (chi tiết Request), CR-REQ-021 (tab Plan), [FE-REQ-SOL-040](./FE-REQ-SOL-040-request-base-branch-picker.md) (cơ chế `requestFlowFeatures`)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-20 (đường dẫn dưới `frontend/src/renderer/src/`):

- `hooks/request-plan-generation.ts:113` `generatePlanProposeCommit(id, options)`: gọi `request.generatePlan` mode `propose`; nếu có `runId` thì `waitForProposal` hỏi `request.planProposal {id, runId}` mỗi 2 giây (`PLAN_RUN_POLL_INTERVAL_MS`), tối đa 240 giây (native) hoặc 64 phút (openspec); rồi gọi mode `commit` kèm proposal. Chạy hoàn toàn trong trình duyệt: đóng tab giữa chừng thì không ai commit.
- `components/request/plan/RequestPlanTab.tsx` (249 dòng; giới hạn `max-lines` của vùng này là 300): `usePlanRunInFlight(request, onSettled)` (`:72`) cho `planRunning`; `decision = usePlanDecision(request, onSettled)`; `generateFailure = decision.failure?.scope === 'plan'` (`:122`) là state cục bộ của tab bấm; `PlanStates` nhận `canGenerate`, `generating = decision.isBusy('generatePlan') || planRunning`, `generateError`, `generateAiCode`, `onGenerate` (`:123-135`).
- `hooks/usePlanRunInFlight.ts`: chỉ bật khi `request.status === 'planning' && !request.planTaskId`, hỏi `request.planProposal {id}` mỗi 4 giây (`PLAN_RUN_PRESENCE_POLL_MS`), chỉ coi `status === 'running'` là bận; mọi lỗi (kể cả "run not found") là "không chạy". Test: `usePlanRunInFlight.test.ts`. Chỉ `RequestPlanTab` dùng nó.
- `components/request/plan/plan-view-state.ts:61-63`: Request `planning` mà chưa có `planTaskId` là `generating`; `:70-71` `planning` có `planTaskId` nhưng chưa tải cây cũng `generating`. `canGeneratePlan(status)` (`:80`) đúng cho `planning` và `awaiting_plan_approval`.
- `components/request/plan/PlanStates.tsx`: `generateButton` (`:56-69`, `data-testid="plan-generate"`, nhãn "Generate plan"), nhánh `generating` (`:95-111`, `data-testid="plan-generating"`, "AI is drafting the plan..."), `generateFailure` (`:70-80`: `RequestAiSetupBanner` hoặc `plan-generate-error`).
- `hooks/usePlanDecision.ts:113` `regeneratePlan` = `run('generatePlan', 'plan', () => generatePlanProposeCommit(request.id))`; `reject(..., {regenerate: true})` gọi lại nó (đường "sinh lại sau khi từ chối", giữ thủ công).
- `components/request/request-ai-setup.ts` `firstAiSetupCode`/`aiSetupCodeOf` nhận ra `REQUEST_AI_*` có thể hành động; `RequestAiSetupBanner` hiển thị cách sửa và có `onRetry`.
- Cờ tính năng: `store/slices/request.ts:56,78-79,97` (`requestFlowFeatures`, `supportsBaseRef`, `setRequestFlowFeatures`), nạp bởi `hooks/useRequestFlowSupport.ts`; hằng `REQUEST_BASE_REF_FEATURE` ở `shared/request-base-ref.ts`.
- `shared/request-rpc-methods.ts`: có `GENERATE_PLAN`, `PLAN_PROPOSAL`; chưa có kênh thử lại. `shared/request-types.ts`: `OrcaRequest.planTaskId?`, `updatedAt`.
- Test liên quan: `components/request/plan/RequestPlanTab.test.tsx`, `plan-locale-coverage.test.ts`, `hooks/request-plan-generation.test.ts`.

### Correction relative to CR-REQ-044

1. CR nói "ẩn hoặc đổi nhãn nút Generate khi run tự động đang chạy": thực tế nút nằm trong `PlanStates` (hai nơi dùng `generateButton`: nhánh `generating` và nhánh `empty`/`not_found`), nên ẩn ở `generateButton` chung bằng một cờ, không sửa từng nhánh.
2. Run tự động của lượt replan (Request `planning` **có** `planTaskId`) hiện rơi vào trạng thái `ready` của `RequestPlanTab` (cây cũ vẫn hiện, thanh duyệt bị huỷ) và không có chỉ báo đang chạy; cần một dải thông báo cho trường hợp này (task 03).
3. Có một khoảng ngắn giữa lúc duyệt Analysis và lúc job được tạo (consumer, tối đa vài giây) mà `request.planProposal` chưa có job lẫn run; trong khoảng đó giao diện không được hiện nút Generate như thể không có gì sắp xảy ra (mục 2.3, "starting").

## 2. Giải pháp

### 2.1 Cây file

```
shared/request-plan-auto.ts, request-plan-auto.test.ts                          (mới) REQUEST_PLAN_AUTO_FEATURE, kiểu, parsePlanRunView
shared/request-rpc-methods.ts                                                   (sửa) RETRY_PLAN
renderer/src/store/slices/request.ts                                            (sửa) supportsPlanAuto
renderer/src/hooks/usePlanRunState.ts, usePlanRunState.test.ts                  (mới) thay usePlanRunInFlight
renderer/src/hooks/usePlanRunInFlight.ts, usePlanRunInFlight.test.ts           (xoá sau khi chuyển người gọi duy nhất)
renderer/src/hooks/usePlanRetry.ts, usePlanRetry.test.ts                        (mới)
renderer/src/components/request/plan/plan-auto-display.ts (+ test)              (mới) derivePlanAutoDisplay
renderer/src/components/request/plan/PlanAutoWriting.tsx                        (mới)
renderer/src/components/request/plan/PlanRunFailedCard.tsx                      (mới)
renderer/src/components/request/plan/PlanAutoSkippedNote.tsx                    (mới)
renderer/src/components/request/plan/PlanRewriteStrip.tsx                       (mới)
renderer/src/components/request/plan/PlanStates.tsx, RequestPlanTab.tsx         (sửa)
renderer/src/i18n/locales/{en,es,ja,ko,zh}.json                                 (sửa)
tests/stages/e2e/orca-flows/ui-live/auto-plan-generation.web.e2e.ts, live-auto-plan-steps.ts   (mới)
```

### 2.2 Dữ liệu và hỗ trợ

`shared/request-plan-auto.ts`:

- `REQUEST_PLAN_AUTO_FEATURE = 'request.planAuto'`; `store/slices/request.ts` thêm `supportsPlanAuto(state)` đọc `requestFlowFeatures`.
- Kiểu `PlanAutoState` (`pending|running|committing|done|gave_up|skipped|superseded|''`), `PlanAutoView {enabled, state, origin, attempt, maxAttempts, reasonCode, nextAttemptAt?}`, `PlanRunView {runId, status, errorCode, errorMessage, engine, autoPlan: PlanAutoView | null}`.
- `parsePlanRunView(raw)` **không bao giờ ném lỗi**: thiếu khoá → giá trị rỗng; `autoPlan` thiếu → `null` (phân biệt "gateway cũ" với `enabled:false`); `state` lạ → `''`.
- `REQUEST_RPC_METHODS.RETRY_PLAN = 'request.retryPlan'`.

### 2.3 Trạng thái hiển thị suy ra từ run mới nhất

`derivePlanAutoDisplay({request, run, supportsPlanAuto, graceElapsed})` là hàm thuần:

| Điều kiện (chỉ khi `request.status === 'planning'`) | Kết quả |
|---|---|
| `run.status === 'running'`, hoặc `autoPlan.state` ∈ `pending`, `running` hoặc `committing` | `writing` (kèm `attempt`, `maxAttempts`, `waiting` khi `pending` sau lượt thất bại, `replan` khi có `planTaskId`) |
| `autoPlan.enabled` và chưa có job, chưa có run, chưa quá thời gian chờ ban đầu (20 giây từ lần đầu hook thấy trạng thái này) | `writing` với `starting: true` |
| `autoPlan.state === 'gave_up'`, hoặc `run.status === 'failed'` (không có job hoạt động) | `failed` (mã = `autoPlan.reasonCode` hoặc `run.errorCode`; `viaServer = autoPlan?.enabled === true`) |
| `autoPlan.state === 'skipped'` | `skipped` (kèm `reasonCode`) |
| còn lại | `none` (như hiện nay) |

Khi gateway cũ (`supportsPlanAuto` sai hoặc `autoPlan === null`) hàm vẫn cho ra `failed` từ `run.status === 'failed'`: thẻ lỗi bền vững là cải thiện độc lập với tính năng tự sinh.

### 2.4 Hook `usePlanRunState`

Thay `usePlanRunInFlight`. Bật khi `request.status === 'planning'` (**bất kể** `planTaskId`, để phủ replan). Hỏi `request.planProposal {id}` mỗi 4 giây (hằng `PLAN_RUN_STATE_POLL_MS`), lỗi "run not found" là "chưa có run" (như trước). Trả `{view: PlanRunView | null, busy, refresh}`, `busy = writing`. Gọi `onFinished` khi `busy` chuyển từ true sang false. Hủy timer khi Request rời `planning` hoặc component tháo. Giữ đúng hợp đồng cũ để test cũ chuyển sang dễ.

### 2.5 Hành động Retry

`usePlanRetry(request, run, regenerateManually)`: nếu `supportsPlanAuto && run.autoPlan?.enabled` thì gọi `request.retryPlan {id}` (`{idempotent: true}`: server trả job đang hoạt động khi bấm lặp), rồi `refresh()` của hook trạng thái; lỗi `REQUEST_PLAN_AUTO_DISABLED` hoặc `autoPlan.enabled` sai thì rơi về `regenerateManually()` (luồng thủ công cũ `generatePlanProposeCommit`). Trả `{retry, retrying, error}`; `error` qua `requestErrorMessageForError`. Nút bị khoá khi `useRequestCooldown('ai')` đang làm nguội, như nút Generate.

### 2.6 Giao diện

- `PlanAutoWriting`: khung đang viết (`data-testid="plan-auto-writing"`): skeleton + "Orca is writing the plan..." (hoặc "Orca is starting the plan..." khi `starting`), dòng phụ "Attempt {{attempt}} of {{max}}" khi lượt > 1, "Trying again shortly" khi `waiting`. **Không có nút Generate.**
- `PlanRunFailedCard`: thẻ bền vững (`role="alert"`, `data-testid="plan-run-failed"`): tiêu đề "Plan generation failed ({{code}})", một dòng gợi ý theo nhóm mã (timeout, bị gián đoạn, đầu ra không hợp lệ, bận, lỗi commit, khác); với mã `REQUEST_AI_*` có thể hành động thì dùng `RequestAiSetupBanner`; nút **Retry** (`plan-run-retry`) và nút phụ "Generate manually" (`plan-generate`, ghost) nằm dưới thẻ.
- `PlanAutoSkippedNote`: ghi chú "Orca could not start the plan on its own ({{reason}}). Generate it when you are ready." kèm nút Generate như hiện nay.
- `PlanRewriteStrip`: dải mỏng trên cây Plan khi `writing` + `replan` ("Orca is rewriting the plan...") để người dùng thấy lượt sinh lại đang chạy sau khi tải lại trang.
- `PlanStates`: nhận `autoDisplay`; nhánh `generating` hiển thị `PlanAutoWriting`, `PlanRunFailedCard` hoặc `PlanAutoSkippedNote` thay cho "AI is drafting the plan..." tuỳ `kind`; `generateButton` bị ẩn khi `writing`. Lỗi inline `generateError` chỉ hiện khi `kind !== 'failed'` (tránh hai thông báo cho một lỗi); lỗi bắt đầu mà không có run (ví dụ không dev server) vẫn hiện inline như cũ.
- `RequestPlanTab`: dùng `usePlanRunState` thay `usePlanRunInFlight`; truyền `autoDisplay` và `retry` xuống; hiển thị `PlanRewriteStrip` ở trạng thái `ready` khi `writing` + `replan`. Không để file vượt 300 dòng: phần suy ra nằm ở `plan-auto-display.ts`.

### 2.7 i18n

Khoá dạng `auto.components.request.plan.<Component>.<key>` cho 5 locale; `plan-locale-coverage.test.ts` đã kiểm vùng plan.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| F1 | Trạng thái suy ra từ run mới nhất và `autoPlan`, không từ state cục bộ | Sống sót sau tải lại; đúng với run do tab khác bấm |
| F2 | `autoPlan === null` ≠ `enabled:false` | Phân biệt gateway cũ với tính năng tắt |
| F3 | Retry đi qua server khi `enabled`, rơi về luồng thủ công khi không | Đóng tab giữa chừng không để run kẹt; quay lui (`REQUEST_PLAN_AUTO_GENERATE=false`) vẫn có Retry hoạt động |
| F4 | Thời gian chờ "starting" đo ở client (20 giây từ lần đầu thấy) | Không phụ thuộc đồng hồ máy chủ; khoảng trống giữa duyệt và tạo job rất ngắn |
| F5 | Ẩn nút Generate khi `writing`, không khoá | Hai nút cho một việc làm người dùng bấm nhầm |
| F6 | Thay `usePlanRunInFlight` thay vì sửa tại chỗ | Đổi hợp đồng (bật cả khi có `planTaskId`, trả cả view); chỉ một người gọi |
| F7 | Thẻ lỗi bền vững áp cho cả run thủ công | Phản hồi gốc nói lỗi chỉ hiện ở tab đã bấm |

## 4. Phụ thuộc và thứ tự

Cần backend TASK-REQ-044-07 và 044-08 (`autoPlan`, `request.retryPlan`, tính năng `request.planAuto`). Thứ tự: 01 → 02 → 03 → 04.

## 5. Kiểm thử

- Parser: thiếu khoá, kiểu sai, `autoPlan` thiếu/`null`/`state` lạ. Store: `supportsPlanAuto`.
- `derivePlanAutoDisplay`: bảng mọi tổ hợp (status Request × `run.status` × `autoPlan.state` × `enabled` × `planTaskId` × `starting`), gồm gateway cũ.
- `usePlanRunState`: bật khi có `planTaskId`; `busy` cho `running` và cho `pending|running|committing`; `onFinished`; dừng khi rời `planning`; "run not found" không lỗi.
- `usePlanRetry`: đi qua server khi `enabled`; rơi về thủ công khi `REQUEST_PLAN_AUTO_DISABLED` hoặc `enabled=false`; bấm hai lần chỉ một lời gọi đang bay; khoá khi làm nguội.
- Component: `PlanStates` (writing ẩn Generate; failed có Retry và Generate phụ; skipped; lỗi inline không trùng thẻ), `RequestPlanTab` (replan có dải), `PlanRunFailedCard` (nhóm mã, banner cài đặt AI).
- Hồi quy: `RequestPlanTab.test.tsx`, `plan-locale-coverage.test.ts`, `request-plan-generation.test.ts` vẫn xanh.
- E2E trực tiếp (`ui-live`, `run_ui_flow.py --ai-stub`) trên repo LedgerLite: xem task 04.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chạy `gitnexus_impact` cho `usePlanRunInFlight`, `PlanStates`, `RequestPlanTab` trước khi sửa (`PlanStates` còn dùng ở nơi khác?; chưa kiểm).
- Run đã `succeeded` nhưng chưa commit do đóng tab giữa chừng (luồng thủ công) vẫn hiện như hiện nay; không thêm trạng thái "đã viết nhưng chưa lưu" vì sẽ nhấp nháy trong lúc luồng thủ công đang commit; phần này thuộc CR-REQ-045.
- Dải replan chưa dựng thử với cây thật lớn; kiểm bằng ảnh chụp ở task 04.
- Chưa dựng giao diện thử; mọi mô tả hành vi hiện tại dựa trên đọc mã.

## 7. Câu hỏi mở

Như CR-REQ-044 mục 5. Riêng frontend: công tắc theo dự án chưa có giao diện (câu hỏi 5.1 của CR); mặc định tạm không làm trong giải pháp này.

## 8. Tham chiếu

- Các file ở mục 1; [CR-REQ-044](../../../../../../docs/crs/v6/plan-phase-task/CR-REQ-044-automatic-plan-generation-after-analysis-approval.md); [BE-REQ-SOL-044](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
- `tests/stages/e2e/orca-flows/ui-live/live-analysis-approval-steps.ts`, `live-ledgerlite-project-flow.ts` (mẫu spec)
