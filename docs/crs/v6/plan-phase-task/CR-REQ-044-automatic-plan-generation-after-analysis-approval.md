# CR-REQ-044 — Tự động sinh Plan ngay sau khi Analysis được duyệt, tự commit phía server, thử lại có giới hạn, và thẻ lỗi bền vững có nút Retry ở tab Plan

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-044 |
| **Tên** | Khi Request vào `planning` (sau khi Analysis được duyệt, hoặc sau khi xác nhận loại với các loại không có Analysis), request-service tự bắt đầu run sinh Plan, tự commit proposal khi run thành công, tự thử lại có giới hạn khi gặp lỗi tạm thời, và ghi lại kết quả để giao diện hiện đúng trạng thái "đang viết", "thất bại (mã lỗi) kèm nút Retry" hoặc "không tự chạy được" ngay cả sau khi tải lại trang |
| **Loại** | Feature (kèm đảo ngược một quyết định thiết kế cũ, mục 1.4) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | ✅ Đã triển khai (2026-10-20), chưa merge; e2e stub xanh, chưa chạy thật hạng C |
| **Phụ thuộc** | CR-REQ-012 (`GeneratePlan`, `CommitPlan`), CR-REQ-013 (Request sau khi Plan được duyệt), CR-REQ-026 (engine OpenSpec, run `agent_proposal`), CR-REQ-028 (consumer `request-service-clarification-resume` là mẫu), CR-REQ-038 (`ProjectEngineSettings`, `execution_mode`), CR-REQ-009 và 010 (Approval mở sau commit) |
| **Mở khoá** | [CR-REQ-045](./CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md) (xem tài liệu của Plan và chỉnh lại Plan) phụ thuộc vào **commit phía server** thiết kế ở đây (mục 2.6): không có nó thì Plan do tác tử viết chỉ commit được khi trình duyệt còn mở |
| **Tác động** | `backend-go/services/request-service` (domain `plan_auto`, usecase `EnqueuePlanAuto`, `PlanAutoWorker`, `RetryPlanGeneration`, sửa `GeneratePlan`, `CommitPlan`, `GetPlanProposal`, `ManageProjectEngineSettings`, consumer `eventbus`, wiring `wire_plan_auto.go`, migration mới hai dialect), `backend-go/proto/orca/request/v1/{plan,solution,request}.proto`, `backend-go/services/api-gateway` (kênh `request.retryPlan`, `request.planProposal`, `request.engineSettings(Set)`, `request.flowStatus.features`, MCP), `backend-go/policy/orca-authz/request.rego`, `frontend/src/renderer/src/{hooks,components/request/plan,shared}`, 5 file locale, `tests/stages/e2e/orca-flows` |

> Nguồn: phản hồi người dùng (bug 3, 2026-10-20), kèm khảo sát chỉ-đọc bằng đọc mã cùng ngày. Số dòng và tên file dưới đây đã đọc lại khi soạn; **chưa chạy hệ thống** để quan sát, và mọi hành vi mô tả ở "hiện trạng" là suy ra từ mã.

---

## 1. Bối cảnh và vấn đề

### 1.1 Phản hồi của người dùng

> "After I approve the Analysis the Request moves to the Plan screen and I have to click Generate plan myself. Expected: plan generation is triggered automatically when the analysis is approved, and can be retried if it fails."

### 1.2 Hiện trạng: không có gì bắt đầu run sinh Plan ngoài nút bấm

| Bước | Ai làm | Nơi trong mã |
|---|---|---|
| Duyệt Analysis | Người duyệt | `usecase/solution_approval_handler.go` `analysisApprovalHandler.OnApproved` (`:127`): duyệt Solution, ghi outbox `orca.request.solution.approved` (`:166`), rồi chạy transition `TriggerAnalysisApproved` (`:177`) |
| Chuyển trạng thái | `TransitionRequest` | `domain/request_transition.go:83`: `analysis_approved` → `planning` (hoặc `completed` nếu `CompletesAfterAnalysis`). Sự kiện `orca.request.request.status_changed` mang `to=planning`, `trigger=analysis_approved`, `project_id`, `reporter_id`, `type` (`usecase/transition_request_events.go:15-37`) |
| Bắt đầu run | **Không ai** | `GeneratePlan.Execute` (`usecase/generate_plan.go:70`) chỉ được gọi từ gRPC `GeneratePlan` (`adapter/grpc/server_plan.go:30`), tức nút "Generate plan" của giao diện. Worker `RunPlanGeneration` (native, `ai.complete`) và `RunProposalGeneration` (OpenSpec, `agent_proposal`) chỉ được sinh ra qua `AnalysisRunner.Spawn` từ `GeneratePlan` |
| Commit Plan | Trình duyệt | `frontend/src/renderer/src/hooks/request-plan-generation.ts:113` `generatePlanProposeCommit`: gọi `request.generatePlan` mode `propose`, hỏi `request.planProposal` mỗi 2 giây, rồi gọi mode `commit` kèm proposal. `CommitPlan.Execute` (`usecase/commit_plan.go:66`) cần danh tính người dùng và proposal do người gọi đưa vào |

Trigger tự động duy nhất hiện có là `ResumeAfterClarification` (`usecase/resume_after_clarification.go:84`), consumer bền `request-service-clarification-resume` (`adapter/eventbus/clarification_resume_consumer.go:15`). Nó chỉ lọc `trigger=information_provided`, sinh lại **Solution** (khoá idempotency `"clr-"+id`, dấu `processed_events` cùng transaction, run được sinh sau commit bằng `after()`), và với `planning` thì bỏ qua có chủ ý: `return nil // planning is never started on the person's behalf` (`resume_after_clarification.go:116`).

### 1.3 Ba vấn đề cần giải quyết cùng nhau

1. **Không có trigger.** Người dùng duyệt xong phải biết bấm tiếp. Đây là phản hồi gốc.
2. **Hai bước chạy trong trình duyệt.** Chỉ khởi động run thì không đủ: run thành công chỉ lưu proposal vào `analysis_runs.raw_output` (`run_plan_generation.go` `finishPlanRun`), chưa có Plan nào trong task-service và Request vẫn ở `planning`. Muốn "Plan tự xuất hiện" thì **server phải tự commit** (đọc proposal bằng `DecodePlanProposal` từ `run.RawOutput`, gọi logic của `CommitPlan`). `GetPlanProposal` đã giải mã đúng cách đó cho cả run native lẫn `agent_proposal` (`get_plan_proposal.go:99-104`) và cả hai worker kết thúc bằng cùng `finishPlanRun` (`run_plan_generation.go`, `run_proposal_generation.go` `finish`), nên một đường commit dùng được cho cả hai engine.
3. **Lỗi chỉ hiện ở tab đã bấm, và không có đường thử lại.** `decision.failure` là state cục bộ của `RequestPlanTab` (`components/request/plan/RequestPlanTab.tsx:122`). Sau khi tải lại, run `failed` trông như "AI is drafting the plan..." kèm nút Generate (`plan/plan-view-state.ts:61-63` trả `generating` khi `planning` và chưa có `planTaskId`; `PlanStates.tsx:95-111`); `usePlanRunInFlight` chỉ coi `running` là bận. Không có endpoint thử lại: thử lại nghĩa là gọi `GeneratePlan` lần nữa, và `GeneratePlanInput` (`generate_plan.go:14`) không có khoá idempotency dù cột `analysis_runs.idempotency_key` và chỉ mục duy nhất `analysis_runs_idem` đã có (`migrations/*/0006_analysis_runs`).

### 1.4 Đảo ngược quyết định cũ (nói rõ)

`specs/backend-go/crs/v6/request-artifact-model/IMPLEMENTATION-NOTES.md:98` ghi quyết định "tự sinh Solution sau khi trả lời (`REQUEST_CLARIFICATION_AUTO_REGENERATE=true`), **không tự sinh Plan**", và mã thể hiện nó ở `resume_after_clarification.go:116`. Lý do cũ: Plan (nhất là Plan OpenSpec, 7 đến 13 USD mỗi lần) tốn tiền và quyết định bắt đầu nên do người.

**CR này đảo ngược quyết định đó cho hai trigger cụ thể (mục 2.2).** Lý do: người dùng yêu cầu rõ ràng (bug 3) và **chấp nhận chi phí**, kể cả chi phí OpenSpec. Phạm vi đảo ngược được giữ hẹp để phần còn lại của quyết định cũ vẫn đúng:

- Chỉ **lần sinh Plan đầu tiên** sau một cổng người đã đi qua (duyệt Analysis, hoặc xác nhận loại với loại không có Analysis) mới tự chạy.
- **Replan, vòng chỉnh sửa, và sinh lại sau khi từ chối duyệt Plan không bao giờ tự chạy** (mục 2.2); `ResumeAfterClarification` vẫn **không** khởi động `planning` (không đổi dòng `:116`).
- Có hai công tắc tắt được (mục 2.3).

### 1.5 Điểm lệch so với mô tả ban đầu (đã đọc lại mã)

1. **Mã lỗi của run Plan.** `REQUEST_SOLUTION_AI_FAILED` và `REQUEST_SOLUTION_AI_TIMEOUT` (`domain/solution_errors.go:72-73`) là mã của run **Solution** (`run_solution_generation.go:101-104`), không phải của run Plan. Worker Plan native ghi `REQUEST_AI_COMPLETE_TIMEOUT` (timeout), `REQUEST_PLAN_AI_UNAVAILABLE` (lỗi AI chung, thông điệp "the AI call failed"; cũng là mã khi không có dev server), `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_PLAN_REQUEST_MOVED`, `REQUEST_PLAN_NOT_APPLICABLE` và các mã `REQUEST_AI_*` có thể hành động (`ai_unavailable.go:10-18`: `NOT_CONFIGURED`, `AGENT_UNAVAILABLE`, `SESSION_EXPIRED`, `QUOTA_EXHAUSTED`, `BUSY`) (`run_plan_generation.go:94-168`). Worker OpenSpec ghi `REQUEST_OPENSPEC_TIMEOUT`, `REQUEST_OPENSPEC_AGENT_FAILED`, `REQUEST_OPENSPEC_INVALID_OUTPUT`, `REQUEST_ENGINE_UNAVAILABLE`, `REQUEST_BASE_REF_NOT_FOUND`, `REQUEST_SPEC_RUN_OUT_OF_SCOPE`, `REQUEST_ANALYSIS_NO_CONNECTION`, `REQUEST_ANALYSIS_NO_REPO_PATH` (`engine_openspec.go:132-330`, `run_proposal_generation.go:98-159`). Sweeper ghi `REQUEST_SOLUTION_RUN_INTERRUPTED` (`recover_interrupted_analysis_runs.go:38`). Bảng phân loại ở mục 2.7 dùng các mã **thật** này.
2. **Commit chưa idempotent hoàn toàn khi hai bên cùng commit.** `CommitPlan.Execute` chỉ coi là replay khi lúc đọc Request đã `awaiting_plan_approval` và có `PlanTaskID` (`commit_plan.go:82`). Nếu server và trình duyệt cùng đọc `planning`, cả hai gọi `CreatePlanTree` (task-service idempotent theo Request, trả `AlreadyExists`), nhưng bên thua ở `record` (`commit_plan.go:128-134`) gặp `cur.Status != planning` và trả `REQUEST_STATE_STALE`. Mô tả "commit idempotent AlreadyExists" đúng cho task-service, **chưa đúng cho cả `CommitPlan`**; CR sửa điểm này (mục 2.9).
3. **Đường commit của trình duyệt làm rơi `satisfies`.** Proto `TaskProposal` (`plan.proto:12-21`) không có trường `satisfies`, `planProposalFromProto` (`adapter/grpc/server_plan.go`) không ánh xạ nó, nhưng `task_plan_writer.go:92` dùng `t.Satisfies` để thêm các tiêu chí chấp nhận mà task phục vụ vào mô tả task. Với Plan OpenSpec (tasks.md có `ac=`), commit qua trình duyệt làm mất các dòng đó; commit phía server đọc thẳng `RawOutput` (`planDocument` có `satisfies`, `plan_document.go:37`) nên giữ lại. **Chưa kiểm bằng chạy thật**; là hệ quả tốt, không phải mục tiêu.
4. **Công tắc theo dự án chưa có giao diện.** Không có trang cài đặt engine trong `frontend/src` (tìm `engineSettings`, `solutionAgent`, `executionMode` chỉ ra spec e2e); `ProjectEngineSettings` đổi được qua kênh admin `request.engineSettingsSet` và `orca-cli`. Xem câu hỏi mở 5.1.
5. **Số dòng.** Một vài số dòng trong mô tả ban đầu lệch 1 (`OnApproved` gọi transition ở `:177`, comment bỏ qua planning ở `:116`, nhánh "approval is off" ở `wire_plan.go:76`, `generatePlanProposeCommit` ở `request-plan-generation.ts:113`); không ảnh hưởng thiết kế.

## 2. Giải pháp đề xuất

### 2.1 Tổng quan luồng

```
OnApproved ──(cùng transaction)──▶ status_changed {to=planning, trigger=analysis_approved}
                                          │  consumer bền "request-service-plan-auto-generate"
                                          ▼
                       EnqueuePlanAuto.Handle   (lọc trigger, hai công tắc, luật bỏ qua; dấu processed_events)
                                          │  INSERT request.plan_auto_jobs (state=pending, khoá duy nhất 1 job đang hoạt động / Request)
                                          ▼
      PlanAutoWorker (vòng lặp thăm dò ~5 giây, mỗi job có lease)
        pending ──▶ GeneratePlan.ExecuteForSystem (khoá "plan-auto-<anchor>[-n]", danh tính = người báo cáo)
                      ├─ bận (REQUEST_ANALYSIS_BUSY)          ▶ pending, hoãn
                      ├─ lỗi vĩnh viễn (không dev server...)  ▶ skipped (lý do)
                      └─ đã tạo / nhận lại run                ▶ running
        running ──▶ đọc run:  succeeded ▶ committing
                              failed + tạm thời + còn lượt ▶ pending (khoá mới, lượt n+1)
                              failed khác                  ▶ gave_up (mã lỗi)
        committing ▶ CommitPlan.ExecuteForSystem(DecodePlanProposal(run.RawOutput)) ▶ done  (Request → awaiting_plan_approval, mở Approval)
```

Giao diện không cần biết các bước: nó hỏi `request.planProposal` (đã có, thêm khối `autoPlan`) và hiện một trong bốn trạng thái ở mục 2.11.

### 2.2 Khi nào tự chạy (và khi nào không)

Chỉ khi nhận `orca.request.request.status_changed` thỏa **tất cả**:

| Điều kiện | Giá trị | Lý do |
|---|---|---|
| `to` | `planning` | Chỉ lúc Request vừa vào chỗ chờ Plan |
| `trigger` | `analysis_approved`, hoặc `type_confirmed` cho loại có `AnalysisKind == none` (`task`, `docs`, `ops_request`) | Hai đường duy nhất đưa Request tới `planning` mà **người vừa đi qua một cổng**. Các trigger khác tới `planning` (`plan_revision`; `analysis_ready` hiện không loại nào dùng) là replan hoặc vòng chỉnh sửa: **không tự chạy** |
| Flow | `PlanKind` không phải `none` hay `single_task`, và `PlanShapeFor(flow, size) != ShapeNone` | `spike`, `question` đi `analysis_approved → completed`, không bao giờ vào `planning`; `hotfix` có `PlanSingleTask` nên `analysis_ready` đưa thẳng tới `awaiting_plan_approval` (`request_transition.go:74-82`), cũng không vào `planning` |
| Request | `PlanTaskID == ""` | Request đã có Plan (ví dụ đổi loại rồi xác nhận lại) không bao giờ tự sinh lại: đó là replan |
| Công tắc | cả hai bật (mục 2.3) | Quay lui được |

**Về `type_confirmed` (quyết định D5, bảng mục 3):** nhận. Cùng một lý do với phản hồi gốc: loại không có Analysis xác nhận xong là rơi vào `planning` và người dùng vẫn phải bấm Generate; việc xác nhận loại là hành động người tường minh tương đương duyệt. Chi phí thấp: `OpenSpecProfileFor` trả `None` cho `task`, `docs`, `ops_request` (`domain/openspec_profile.go:17`), nên các loại này luôn chạy engine native (`ai.complete`), không phải OpenSpec.

Bỏ qua còn lại (kiểm lúc chạy job, không lúc nhận sự kiện): không có dev server, repo không có đường dẫn, engine không có worker, Request đã rời `planning` (`REQUEST_STATE_STALE`). Mỗi trường hợp là `skipped` có lý do, **không** là lỗi cứng và **không** làm consumer lặp vô hạn (mục 2.5).

### 2.3 Hai công tắc, cả hai phải bật

1. **Triển khai:** biến môi trường `REQUEST_PLAN_AUTO_GENERATE`, **mặc định `true`**. Giá trị quay lui: `false` (hành vi cũ hoàn toàn: không có consumer, không có worker). Cùng điều kiện với consumer resume: `cfg.RequestFlowEnabled`, có event bus, và `CommitPlan` đã được nối (tức approval bật, `wire_plan.go:76`; thiếu thì ghi cảnh báo lúc khởi động và tính năng tắt, vì Plan được commit mà không ai duyệt được là vô nghĩa).
2. **Theo dự án:** trường mới `plan_auto_generate` trên `ProjectEngineSettings`, **mặc định `true`**, đặt qua `request.engineSettingsSet` (chỉ admin). Dự án chưa có dòng cài đặt coi như `true`. Client cũ không gửi trường này **không được** làm dự án bật lại hay tắt đi: proto dùng `optional bool` và rỗng nghĩa "giữ giá trị đã lưu" (cùng cách `execution_mode` xử lý ở `manage_project_engine_settings.go`).

Mặc định bật cho mọi người là quyết định của người dùng (D1).

### 2.4 Bảng job `request.plan_auto_jobs`

Một bảng nhỏ giữ trạng thái bền của "việc tự sinh Plan" cho mỗi Request, để (a) sống sót qua khởi động lại, (b) hoãn khi bận, (c) thử lại có đếm, (d) giao diện đọc được kết quả sau khi tải lại. Lý do không dùng riêng sự kiện: consumer của bus chỉ có redeliver tức thì khi trả lỗi (`common/eventbus/eventbus.go:123-155`, không có độ trễ), nên "hoãn" và "thử lại sau 30 giây" không biểu diễn được bằng sự kiện.

Cột chính: `tenant_id`, `request_id`, `anchor`, `origin` (`event` | `retry`), `project_id`, `state`, `attempt`, `max_attempts`, `run_id`, `reason_code`, `commit_attempts`, `next_check_at`, `lease_owner`, `lease_expires_at`, `requested_by`, `created_at`, `updated_at`. Khoá chính `(tenant_id, request_id, anchor)`; chỉ mục duy nhất một job **đang hoạt động** (`pending`, `running`, `committing`) trên mỗi Request (mẫu `analysis_runs_one_running` của Postgres và cột sinh `active_key` của MySQL, `migrations/*/0006_analysis_runs`).

Trạng thái: `pending` (chờ bắt đầu hoặc chờ lượt thử lại), `running` (đã có run), `committing` (run xong, đang ghi Plan), và ba trạng thái cuối `done`, `gave_up`, `skipped`, cộng `superseded` (người đã tự làm trước: Request rời `planning`, đã có `PlanTaskID`, hoặc có run mới hơn không phải của job).

**Khoá idempotency của run:** lượt 1 là `plan-auto-<anchor>`, lượt n≥2 là `plan-auto-<anchor>-<n>`. `anchor` là **id Solution đã duyệt** với loại có Analysis (đúng `plan-auto-<solutionId>` như người dùng chốt), và `ty-<event id>` với `type_confirmed`; job `retry` dùng `retry-<uuid>`. Cột khoá là `VARCHAR(128)` ở MySQL (`0006_analysis_runs`), chuỗi dài nhất ở đây khoảng 60 ký tự.

### 2.5 Bắt đầu run thay cho người dùng

- **Danh tính.** Dev server gắn với người báo cáo, và `AnalysisRunner.reporterIdentity` (`analysis_runner.go:74`) cố ý giữ danh tính người gọi cho run Plan; vì vậy run phải được tạo với `ActorID` là người báo cáo. `GeneratePlan` thêm `ExecuteForSystem`: bỏ `AuthorizePlan`, đặt `actor = req.ReporterID`, đặt user vào `ctx` để `conns.ResolveForProject` tìm đúng dev server; giống `GenerateSolution.PrepareForSystem` (`generate_solution.go:160`) và `skipWithoutConnection` (`solution_regenerator.go`). Không thêm đường bỏ qua quyền cho RPC công khai.
- **Khoá idempotency.** `GeneratePlanInput.IdempotencyKey` (mới) được ghi vào `run.IdempotencyKey`. `findBlocking` (`adapter/postgres/analysis_run_repository.go:56`, MySQL `:68`) trả run đã có nếu khoá khớp (mọi trạng thái) hoặc đang có một run Plan `running`; vì vậy gọi lại cùng khoá không tạo run mới, và **lượt thử lại phải dùng khoá mới**, nếu không sẽ nhận lại chính run đã `failed`.
- **Phân loại lỗi lúc bắt đầu** (theo `apperrors.Kind` và mã, không liệt kê cứng từng mã):

| Kết quả | Xử lý |
|---|---|
| `REQUEST_ANALYSIS_BUSY` (`KindFailedPrecondition`, chỉ chế độ agent, trần `MaxAgentRunsPerProject` mặc định 2, `analysis_settings.go:40`) | Hoãn: giữ `pending`, `next_check_at = now + 60 giây`; sau 30 phút kể từ `created_at` thì `gave_up` với mã đó. **Không** là lỗi cứng |
| `KindFailedPrecondition`, `KindInvalidArgument`, `KindNotFound`, `KindPermissionDenied` khác (gồm `REQUEST_PLAN_AI_UNAVAILABLE` lúc không có dev server, `REQUEST_ANALYSIS_NO_REPO_PATH`, `REQUEST_ENGINE_UNAVAILABLE`, `REQUEST_STATE_STALE`, `REQUEST_PLAN_NOT_APPLICABLE`, `REQUEST_PLAN_SOLUTION_NOT_APPROVED`) | `skipped` kèm mã. Người dùng vẫn có nút Generate thủ công như cũ |
| Lỗi hạ tầng (cơ sở dữ liệu, mạng) | Giữ `pending`, thử lại sau 30 giây trong cùng cửa sổ 30 phút, rồi `gave_up`. Consumer không bao giờ trả lỗi chỉ vì lỗi này |
| Đã tạo hoặc nhận lại run | `running`; nếu `Created` thì `after()` sinh worker sau khi transaction đóng |

Nhận lại run (`Created=false`) xảy ra khi người dùng bấm tay cùng lúc, hoặc job chết giữa lúc tạo run và lúc ghi trạng thái: job nhận run đó làm của mình. Trường hợp thứ hai không có goroutine gia hạn lease nên run sẽ bị sweeper đánh dấu `REQUEST_SOLUTION_RUN_INTERRUPTED` rồi được thử lại như lỗi tạm thời; chưa tốn tiền vì chưa chạy gì.

### 2.6 Commit phía server

`CommitPlan.ExecuteForSystem(ctx, {RequestID, Proposal})`: bỏ `AuthorizePlan`, dùng người báo cáo làm `actor`/`CreatorID`, giữ nguyên mọi bước còn lại (`ValidatePlanProposal`, `PlanPreconditions`, `CreatePlanTree`, `record`: Plan id, sự kiện `plan.generated`, transition `plan_ready`, mở Approval). Proposal lấy bằng `DecodePlanProposal(run.RawOutput)`. Điều kiện cần: `CommitPlan` được nối, tức approval bật (`wire_plan.go:76`).

Lỗi của bước commit:

| Lỗi | Xử lý |
|---|---|
| `AlreadyExists`, hoặc Request đã `awaiting_plan_approval` với `PlanTaskID` | `done` (người đã commit trước) |
| Mã `REQUEST_PLAN_*` (proposal không hợp lệ so với luật hiện tại) | `gave_up` với mã đó; **không** chạy lại AI |
| Lỗi khác (task-service không với tới) | Thử lại commit tối đa 5 lần, cách 30 giây, **trên cùng run đã thành công**, rồi `gave_up` với `REQUEST_PLAN_COMMIT_FAILED`. Không gọi lại AI |

### 2.7 Thử lại tự động có giới hạn

Chỉ cho lỗi **tạm thời**, tối đa `REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS` lượt thêm (mặc định **1**, cho phép 0 đến 2), mỗi lượt cách `REQUEST_PLAN_AUTO_RETRY_DELAY` (mặc định 30 giây) và dùng khoá mới `plan-auto-<anchor>-<n>`:

| Loại | Mã của run | Thử lại? |
|---|---|---|
| Tạm thời | `REQUEST_AI_COMPLETE_TIMEOUT`, `REQUEST_PLAN_AI_UNAVAILABLE` (lỗi AI chung), `REQUEST_AI_BUSY`, `REQUEST_OPENSPEC_TIMEOUT`, `REQUEST_OPENSPEC_AGENT_FAILED`, `REQUEST_SOLUTION_RUN_INTERRUPTED` | Có, nếu còn lượt |
| Cần người xử lý | `REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_AI_SESSION_EXPIRED`, `REQUEST_AI_QUOTA_EXHAUSTED`, `REQUEST_ANALYSIS_NO_CONNECTION`, `REQUEST_ANALYSIS_NO_REPO_PATH`, `REQUEST_ANALYSIS_AGENT_TOO_OLD`, `REQUEST_ENGINE_UNAVAILABLE`, `REQUEST_BASE_REF_NOT_FOUND` | Không: thẻ lỗi chỉ cách sửa (giao diện đã có `RequestAiSetupBanner`) |
| Vòng chi phí | `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_OPENSPEC_INVALID_OUTPUT`, `REQUEST_SPEC_RUN_OUT_OF_SCOPE`, `REQUEST_PLAN_*` khác, `REQUEST_SOLUTION_INTERNAL_ERROR`, `REQUEST_PLAN_REQUEST_MOVED` | **Không bao giờ** tự thử lại (mỗi lượt tốn tiền mà đầu ra có thể sai y hệt) |

Danh sách này là hằng trong `domain/plan_auto.go` có test bảng. Job `retry` (người bấm Retry) có `max_attempts = 1`: người đang ngồi đó, không tự thử thêm.

### 2.8 Retry thủ công, phục hồi được sau khi tải lại

RPC mới `RetryPlanGeneration(request_id)` (kênh `request.retryPlan`), quyền như `GeneratePlan` (nhóm `plan`, `rpc_catalog.go:107`: người báo cáo hoặc owner, không cho agent). Điều kiện: Request `planning`; nếu đã có job đang hoạt động thì trả chính job đó (bấm hai lần không sinh thêm). Hành vi:

- Nếu run Plan mới nhất **đã thành công mà chưa commit** (lỗi ở bước commit), job `retry` khởi tạo thẳng ở `committing` trên run đó: **không trả tiền AI lần nữa**.
- Ngược lại, job `retry` mới ở `pending`, anchor `retry-<uuid>`.
- Khi công tắc tắt (mục 2.3) trả `REQUEST_PLAN_AUTO_DISABLED`; giao diện dùng lại luồng thủ công cũ (mục 2.11) nên thẻ lỗi vẫn có nút Retry hoạt động.

Nút "Generate plan" thủ công cũ **giữ nguyên**, kể cả khi `feedback` (replan từ `awaiting_plan_approval`).

### 2.9 Va chạm với người bấm tay cùng lúc phải vô hại

Bảo vệ nhiều lớp, mỗi lớp có test:

1. Chỉ mục `analysis_runs_one_running` + `findBlocking`: tối đa một run Plan `running` mỗi Request; bên đến sau nhận run đang chạy (trình duyệt nhận `runId` đó và chờ như bình thường).
2. Job luôn tải lại Request trước mỗi bước: nếu không còn `planning`, hoặc đã có `PlanTaskID`, hoặc run mới nhất không phải của job → `superseded`/`done`, không làm gì thêm.
3. Commit idempotent: sửa `CommitPlan.record` để Request đã `awaiting_plan_approval` với đúng `PlanTaskID` vừa nhận được coi là thành công (không trả `REQUEST_STATE_STALE`), nhờ đó bên thua cuộc đua (trình duyệt hoặc server) không thấy lỗi. `CreatePlanTree` đã idempotent theo Request ở task-service.
4. Giao lại cùng sự kiện: dấu `processed_events` (khoá chính `(tenant_id, event_id)`, `0001_init`) cùng transaction với việc chèn job, cộng khoá chính của job và chỉ mục "một job hoạt động": giao hai lần chỉ tạo một job, một run.

Ràng buộc cần nhớ: `processed_events` khoá theo `event_id` chứ không theo consumer, nên bộ lọc của consumer mới phải **rời** với hai consumer cùng subject (phân loại: `to=classifying`; resume: `trigger=information_provided`). Bộ lọc ở mục 2.2 thỏa điều đó.

### 2.10 API, kênh, hợp đồng

| Thay đổi | Chi tiết |
|---|---|
| `plan.proto` | `GetPlanProposalResponse.auto_plan` (trường 13, message `PlanAutoState`: `enabled`, `state`, `origin`, `attempt`, `max_attempts`, `reason_code`, `next_attempt_at`); RPC `RetryPlanGeneration` (+ request/response) |
| `solution.proto` | `ProjectEngineSettings.plan_auto_generate` (trường 7); `SetProjectEngineSettingsRequest.plan_auto_generate` (`optional bool`, trường 7) |
| `GetPlanProposal` | Khi không có run Plan nào nhưng có job, trả view rỗng với `auto_plan` thay vì `REQUEST_PLAN_RUN_NOT_FOUND`; `enabled` luôn được điền (cả khi không có job) |
| Gateway | `request.planProposal` thêm `autoPlan`; kênh mới `request.retryPlan` (`GroupWrite`; thêm vào `request_rpc_channel_map_test.go`); `request.engineSettings(Set)` thêm `planAutoGenerate`; `request.flowStatus.features` thêm `request.planAuto` (cơ chế đã có, `channels_request_base_ref.go:16-19`) |
| MCP | `request.retryPlan` thêm một dòng vào `excluded_channels.yaml` (lý do: việc khôi phục của giao diện, agent đi qua `request_generatePlan`); `parity_test` xanh |
| Authz | `rpc_catalog.go`: `RetryPlanGeneration` là `onRequest(GroupPlan, "request_id", RateAI, false)`; `flow_gate.go`: `classAdvance`; không thêm vào `agent_rpcs` của `request.rego` |
| CONTRACT | Mục mới trong `CONTRACT-request-ui-api.md`: `autoPlan`, `request.retryPlan`, `planAutoGenerate`, mã lỗi mới |
| Mã lỗi mới | `REQUEST_PLAN_AUTO_DISABLED`, `REQUEST_PLAN_COMMIT_FAILED` (chỉ là `reason_code`, không là lỗi RPC) |

### 2.11 Frontend

Trạng thái suy ra từ **run Plan mới nhất** và `autoPlan`, nên bền qua tải lại và đúng với cả run do tab khác bấm:

| Điều kiện (Request `planning`) | Hiện |
|---|---|
| `autoPlan.state` là `pending`/`running`/`committing`, hoặc run mới nhất `running` | "Orca is writing the plan" + tiến độ ("Attempt 2 of 2" khi đang thử lại); **ẩn** nút Generate; hiện cả khi đã có `planTaskId` (replan) dưới dạng dải trên cây Plan |
| Run mới nhất `failed`, hoặc `autoPlan.state == gave_up` | Thẻ "Plan generation failed (`<mã>`)" bền vững, nút **Retry** (gọi `request.retryPlan` nếu `autoPlan.enabled`, ngược lại luồng thủ công cũ), và nút Generate thủ công còn lại ở dạng phụ |
| `autoPlan.state == skipped` | Ghi chú "Orca could not start the plan automatically (`<lý do>`)" + nút Generate như cũ |
| Không có gì | Như hiện nay |

`usePlanRunInFlight` (chỉ coi `running` là bận, chỉ bật khi chưa có `planTaskId`, `hooks/usePlanRunInFlight.ts`) được thay bằng hook `usePlanRunState` coi run `running` là bận cả khi `planTaskId` đã có (replan). Chi tiết ở `FE-REQ-SOL-044`.

### 2.12 Chi phí (nói thẳng)

- Engine **native** (`ai.complete`): một đến hai lệnh gọi AI mỗi Plan; nhỏ.
- Engine **OpenSpec** (`change_request`, `refactor`; và `bug`/`security`/`performance` khi dự án chọn engine này): 7 đến 13 USD mỗi Plan (đo ở các đợt trước, xem kế hoạch `specs/IMPLEMENTATION-PLAN-CR-REQ-039-042.md` mục 4.5). **Người dùng chấp nhận chi phí này** (D2). Từ nay mỗi lần duyệt Analysis của loại chạy OpenSpec tự tốn khoản đó, kể cả khi người duyệt định tạm dừng; công tắc dự án (D1) là cách tránh.
- Giới hạn chi phí: tối đa 1 lượt thêm mặc định cho lỗi tạm thời; không bao giờ thử lại tự động lỗi đầu ra; replan, vòng chỉnh sửa, sinh lại sau khi từ chối không tự chạy (D3). Trong tình huống xấu nhất một Request OpenSpec tốn hai Plan (lượt đầu timeout, lượt thêm thành công).

### 2.13 Migration

Số tiếp theo của `request-service` hiện là `0101` (mới nhất `0100_analysis_runs_revision_index`; **kiểm lại khi triển khai** vì các CR khác có thể lấy số này): `0101_plan_auto_generation` hai dialect, tạo bảng `plan_auto_jobs` (có `.down.sql`) và thêm `project_engine_settings.plan_auto_generate BOOLEAN NOT NULL DEFAULT true`. Không backfill: dòng cài đặt cũ lấy `true` theo mặc định cột (đúng D1).

### 2.14 Điểm tựa cho CR-REQ-045

[CR-REQ-045](./CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md) cần server commit Plan sau một lượt chỉnh do approver bấm (R1 của nó). CR này cung cấp ba thứ để dùng lại, không làm thay phần của 045:

1. `CommitPlan.ExecuteForSystem` (mục 2.6): commit không cần quyền commit của người gọi, đã xử lý replan (`SupersedesPlanID`) vì dùng chung thân hàm với `Execute`.
2. Bước `committing` của worker (đọc `RawOutput`, thử lại commit, trả lỗi `REQUEST_PLAN_*` thành `gave_up`): 045 chèn một job có `run_id` của lượt chỉnh và `state=running`, `max_attempts=1`, rồi worker làm phần còn lại. Khi đó 045 mở rộng `CHECK` của `origin` bằng giá trị mới trong migration của nó; CR này **chỉ** cho `event` và `retry`.
3. Luật `PlanTaskID == ""` ở mục 2.2 và ở bước `superseded` chỉ áp cho job `event`/`retry`; job của 045 được phép có `PlanTaskID` (đang replan).

CR này **không** tự commit lượt có `feedback` do người bấm tay ở `GeneratePlan` (D3); việc đó thuộc 045.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Tự sinh Plan **bật mặc định cho mọi người**; hai công tắc: env `REQUEST_PLAN_AUTO_GENERATE` (mặc định `true`, quay lui `false`) và trường dự án `plan_auto_generate` (mặc định `true`); phải bật cả hai (**người dùng chốt**) | Đúng yêu cầu; vẫn quay lui được ở hai tầng |
| D2 | Chấp nhận chi phí OpenSpec 7 đến 13 USD mỗi Plan; CR nêu rõ và chặn các đường tự chạy khác (**người dùng chốt**) | Quyết định kinh doanh của người dùng |
| D3 | Chỉ trigger ở `status_changed` với `to=planning` và `trigger=analysis_approved` (và `type_confirmed` cho loại không có Analysis, D5); replan, vòng chỉnh sửa, sinh lại sau từ chối duyệt Plan, `ResumeAfterClarification` không bao giờ tự chạy; Request đã có `PlanTaskID` không tự chạy | Giữ chi phí đoán được |
| D4 | Lỗi: thử lại tự động có giới hạn (1 lượt thêm mặc định, tối đa 2, khoá `plan-auto-<solutionId>-<n>`) **chỉ** cho lỗi tạm thời; `INVALID_OUTPUT`/`INVALID_JSON` không bao giờ; `ErrAnalysisBusy` là hoãn, không là lỗi; sau đó giữ `planning` với run `failed` và thẻ "Plan generation failed (mã)" có Retry cùng nút thủ công (**người dùng chốt**) | Ngăn vòng chi phí; người luôn có đường ra |
| D5 | Nhận thêm `type_confirmed` cho `task`, `docs`, `ops_request` | Cùng lý do phản hồi gốc; luôn là engine native nên rẻ; xác nhận loại là hành động người |
| D6 | Commit phía server là một phần của CR; dùng `DecodePlanProposal(run.RawOutput)` cho cả hai engine (**người dùng chốt**) | Run thành công mà không commit thì không có Plan; CR-REQ-045 cần |
| D7 | Bảng job `plan_auto_jobs` thay vì chỉ sự kiện hoặc chỉ suy từ `analysis_runs` | Cần hoãn khi bận, đếm lượt, nhớ "đã thử bao nhiêu lần", và cho giao diện một nguồn trạng thái; bus không có độ trễ redeliver |
| D8 | Một đường thực thi duy nhất: consumer chỉ chèn job, worker làm mọi việc | Dễ kiểm thử; không hai đường lệch nhau |
| D9 | Job `retry` không tự thử thêm; Retry sau lỗi commit tiếp tục từ bước commit | Người đang theo dõi; không trả tiền AI hai lần cho lỗi không phải của AI |
| D10 | Danh tính chạy run và commit là người báo cáo; không thêm đường bỏ qua quyền cho RPC công khai | Dev server gắn người báo cáo; giống `PrepareForSystem` |

## 4. Rủi ro và điểm chưa kiểm chứng

- **R1 Chi phí OpenSpec phát sinh ngoài ý muốn.** Mỗi lần duyệt Analysis của loại chạy OpenSpec tốn 7 đến 13 USD ngay lập tức, kể cả khi người duyệt nhấn nhầm. Giảm thiểu: công tắc dự án; giới hạn thử lại; ghi chú phát hành nêu rõ. Con số 7 đến 13 USD lấy từ kế hoạch 039 đến 042, chưa đo lại cho CR này.
- **R2 Thử lại lỗi `REQUEST_OPENSPEC_AGENT_FAILED`.** Mã này gộp nhiều nguyên nhân (agent thoát lỗi, không commit được nhánh, readiness check không chạy: `engine_openspec.go:198-318`, `run_proposal_generation.go:293`); thử lại 1 lần có thể tốn thêm một Plan cho lỗi không thật sự tạm thời. Đã chọn theo yêu cầu người dùng (D4); có thể đặt `REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS=0`.
- **R3 Bốn đường vào `planning` chưa đo hết.** Mới đọc `request_transition.go:60-125` và các nơi gọi `TriggerAnalysisApproved`/`TriggerTypeConfirmed`; chưa chạy tất cả 11 loại. Test bảng 11 loại × trigger trong TASK-REQ-044-01 bắt lỗi này.
- **R4 Job mồ côi.** Job `running` mà run đã bị xoá (retention) hoặc Request bị xoá: worker phải coi là `superseded`. Test riêng.
- **R5 Vòng thăm dò thêm tải.** Một truy vấn có chỉ mục mỗi 5 giây cho mọi instance, dùng `FOR UPDATE SKIP LOCKED` như `ClaimExpired`; chưa đo. Nhịp có thể đổi bằng `REQUEST_PLAN_AUTO_POLL_INTERVAL`.
- **R6 Hai gateway/runtime khác phiên bản.** Frontend mới với gateway cũ: không có `request.planAuto` trong `features` → giao diện giữ hành vi cũ (nút Generate), nhưng vẫn hiện thẻ lỗi từ run mới nhất vì dữ liệu đó có sẵn ở `request.planProposal`.
- **R7 Số migration `0101` và số trường proto** có thể bị CR khác giữ; kiểm lại lúc triển khai.
- **Chưa kiểm chứng:** `REQUEST_PLAN_AI_UNAVAILABLE` có thực sự là mã khi lệnh `claude` của stub thoát lỗi hay là một mã `REQUEST_AI_*` (phụ thuộc `mapAIRelayError`, `adapter/grpcclient/ai_relay_errors.go`); ca APG-03 kiểm điều này khi dựng.

## 5. Câu hỏi còn mở

> **Đã chốt 2026-10-20 (sau triển khai):** trả lời Clarification sẵn sàng (trigger `information_provided`) KHÔNG tự sinh Plan, giữ nguyên thiết kế D3; người dùng bấm Generate. Hệ quả đã biết: Request `task`, `docs`, `ops_request` phải trả lời Clarification thì chỉ tự sinh Plan khi đã có tiêu chí chấp nhận lúc xác nhận loại (APG-09 và APG-09b kiểm cả hai trường hợp).

> **Đã chốt 2026-10-20:** câu 1 và 2 theo đề xuất (không làm giao diện cài đặt công tắc theo dự án trong CR này, ghi rõ trong tài liệu vận hành; `REQUEST_PLAN_AI_UNAVAILABLE` ở cấp run là lỗi tạm). Câu 3 và 4 chưa được hỏi riêng: lấy theo mặc định ghi ở từng câu.

1. **Giao diện cho công tắc dự án.** Hiện không có trang cài đặt engine trong frontend; admin đổi qua `request.engineSettingsSet`/`orca-cli`. Mặc định đề xuất: không làm giao diện trong CR này (người dùng đã chốt công tắc "theo dự án" mà không nói nơi đặt), ghi rõ trong tài liệu vận hành. Nếu muốn có nút trong giao diện thì cần một CR frontend riêng về trang cài đặt engine.
2. **`REQUEST_PLAN_AI_UNAVAILABLE` ở cấp run là tạm thời?** Đề xuất: có (đó là lỗi AI chung "the AI call failed"). Nếu thực tế mã này hay đi kèm cấu hình sai thì nên chuyển vào nhóm "cần người xử lý".
3. **Có ghi dòng "Plan auto-generation started/failed" vào nhật ký hoạt động (`request.activity`) không?** Đề xuất: không trong CR này; thẻ ở tab Plan đủ cho phản hồi gốc, và nhật ký lấy từ các bảng nguồn có sẵn (`domain/request_activity.go`).
4. Cửa sổ hoãn khi bận 30 phút và nhịp 60 giây có hợp lý cho dự án nhiều Request OpenSpec cùng lúc? Giá trị tạm chưa dựa trên số đo.

## 6. Tiêu chí chấp nhận

- [ ] Duyệt Analysis của một Request `bug` (engine native): không cần bấm gì, Request tới `awaiting_plan_approval` có `planTaskId` và một Approval `plan` đang mở; `request.planProposal` trả `autoPlan.state=done`.
- [ ] Trong lúc job chưa xong, tab Plan hiện "Orca is writing the plan" và **không** có nút Generate; tải lại trang vẫn hiện đúng trạng thái.
- [ ] Run thất bại do lỗi không tạm thời (ví dụ `REQUEST_PLAN_AI_INVALID_JSON`): đúng một run, Request giữ `planning`, `autoPlan.state=gave_up`; thẻ "Plan generation failed (REQUEST_PLAN_AI_INVALID_JSON)" có Retry, còn sau khi tải lại; bấm Retry sinh Plan thành công.
- [ ] Lỗi tạm thời: lượt 1 `failed`, lượt 2 với khoá `plan-auto-<solutionId>-2` thành công; tổng hai run.
- [ ] Không dev server: không có run nào, `autoPlan.state=skipped` kèm lý do; nút Generate thủ công hoạt động như cũ.
- [ ] `spike`, `question`, `hotfix`: không có job, không có run. Replan (`feedback` từ `awaiting_plan_approval`), sinh lại sau từ chối duyệt Plan, Request đã có `PlanTaskID` xác nhận lại loại: không có job.
- [ ] `REQUEST_PLAN_AUTO_GENERATE=false` hoặc `plan_auto_generate=false` của dự án: Request ở `planning` như hành vi cũ, không job, không run; `autoPlan.enabled=false`.
- [ ] Giao lại cùng sự kiện hai lần: một job, một run. Người bấm Generate tay trong lúc job chạy: một run, một Plan, không lỗi hiển thị cho bên nào.
- [ ] Run OpenSpec thành công được commit phía server với `satisfies` được giữ (so với commit qua trình duyệt); chạy thật chỉ khi người dùng đồng ý chi phí.
- [ ] Migration xuôi và ngược trên Postgres 16 và MySQL 8; cài đặt dự án cũ đọc ra `plan_auto_generate=true`; client cũ gọi `SetProjectEngineSettings` không đổi công tắc.
- [ ] Gateway cũ (không có `request.planAuto`): giao diện hoạt động như cũ, vẫn hiện thẻ lỗi bền vững của run mới nhất.
- [ ] `parity_test` MCP, `request_rpc_channel_map_test`, kiểm tra `rpc_catalog` xanh; 5 locale đủ khoá.

## 7. Kế hoạch kiểm thử

- **Unit (domain).** `PlanAutoKey` (lượt 1, n≥2, độ dài tối đa); bảng `IsTransientPlanRunCode` cho **mọi** mã ở mục 2.7 (kỳ vọng từng mã); máy trạng thái job (mọi chuyển hợp lệ và bị từ chối); luật nhận job (bảng 11 loại × trigger × `PlanTaskID` × hai công tắc).
- **Unit (usecase, fake).** `EnqueuePlanAuto`, `PlanAutoWorker` từng bước (bắt đầu ok, bận hoãn rồi bỏ cuộc sau 30 phút, lỗi vĩnh viễn → `skipped`, lỗi hạ tầng, nhận lại run, run `running`/`succeeded`/`failed` tạm thời/không tạm thời, hết lượt, `superseded` khi người đã commit, commit `AlreadyExists`, lỗi `REQUEST_PLAN_*`, lỗi commit lặp 5 lần); `GeneratePlan.ExecuteForSystem` (không quyền, danh tính người báo cáo, khoá); `CommitPlan.ExecuteForSystem` và sửa `record` cho cuộc đua; `RetryPlanGeneration` (job có sẵn, run đã thành công chưa commit, công tắc tắt); `ManageProjectEngineSettings` giữ công tắc khi client cũ không gửi.
- **Hợp đồng hai dialect (Postgres 16, MySQL 8, testcontainers).** Bảng `plan_auto_jobs` (khoá chính, chỉ mục một job hoạt động, claim `SKIP LOCKED` không trùng giữa hai worker, đổi trạng thái có điều kiện), cột `plan_auto_generate` (mặc định `true`, đọc/ghi), `findBlocking` với khoá mới/cũ; kịch bản tích hợp "giao lại hai lần cùng sự kiện" với `processed_events` thật.
- **Gateway.** Test `wscompat` cho `request.retryPlan`, `autoPlan` trong `request.planProposal`, `planAutoGenerate` trong engine settings; `parity_test`; `rpc_catalog` và `request.rego` test.
- **Acceptance (`tests/stages/e2e/orca-flows/suite_request_auto_plan.py`, `--ai-stub`, loại A, CI):** `APG-01` duyệt Analysis → Plan tự xuất hiện (không gọi `generatePlan`); `APG-02` lỗi không tạm thời → `gave_up` → `request.retryPlan` → thành công; `APG-03` lỗi tạm thời → tự thử lại một lần (hai run, khoá `-2`); `APG-04` không dev server → `skipped`, nút thủ công chạy; `APG-05` `spike`/`question`/`hotfix` không kích hoạt; `APG-06` công tắc env/dự án tắt → hành vi cũ; `APG-07` bấm tay đồng thời → một run, một Plan; `APG-08` replan/từ chối/đã có Plan không tự chạy; `APG-09` `task` xác nhận loại → tự sinh. Stub cần mở rộng: đánh dấu trạng thái `fail_plan_next` trong `$FAKE_CLAUDE_STATE_DIR` (số lần và kiểu `error`/`invalid`) vì run tự động không có khối `<feedback>` để mang mã `[stub:fail-once]` (`fake_claude_cli/claude`). Ca "giao lại sự kiện" và "hoãn khi bận" chỉ kiểm được ở tầng tích hợp, ghi rõ ở task.
- **UI live (`ui-live/auto-plan-generation.web.e2e.ts`, `--ai-stub`, repo LedgerLite kịch bản 1 `bug`):** (a) duyệt Analysis trên UI → tab Plan "Orca is writing the plan" → cây Plan và thanh duyệt, không có nút Generate; (b) lỗi không tạm thời → thẻ có mã → tải lại vẫn còn → Retry → thành công; (c) lỗi tạm thời → dải "Attempt 2 of 2" → thành công; (d) gateway giả lập không có `request.planAuto` → hành vi cũ.
- **Chạy thật.** Loại B (`--real`, native, repo tạm): một ca bug đến Plan tự xuất hiện, chạy cuối đợt. Loại C (OpenSpec, `ORCA_E2E_OPENSPEC=1`, fixture LedgerLite kịch bản 3): `APG-10` kiểm run `agent_proposal` được commit phía server (và `satisfies` còn); **tốn 7 đến 13 USD, chỉ chạy khi người dùng đồng ý rõ ràng**, mặc định `SKIP`.

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-012 | `GeneratePlan` thêm `IdempotencyKey` và `ExecuteForSystem`; `CommitPlan` thêm `ExecuteForSystem` và sửa `record` cho cuộc đua; ghi rằng commit không còn chỉ là việc của trình duyệt |
| CR-REQ-028 | Ghi chú: quyết định "không tự sinh Plan" bị đảo cho hai trigger của CR-REQ-044 (mục 1.4); `ResumeAfterClarification` giữ nguyên |
| CR-REQ-026 | Run `agent_proposal` cũng được commit phía server |
| CR-REQ-038 | `ProjectEngineSettings` thêm `plan_auto_generate` |
| CR-REQ-016 | Kênh `request.retryPlan`; `request.planProposal` thêm `autoPlan`; `flowStatus.features` thêm `request.planAuto` |
| CR-REQ-017 | Dòng loại trừ MCP cho `request.retryPlan` |
| CR-REQ-021 | Tab Plan: trạng thái "đang viết", thẻ lỗi bền vững, nút Retry |
| CR-REQ-045 | Dựa trên commit phía server của CR này |

## 9. Tham chiếu

- `backend-go/services/request-service/internal/usecase/solution_approval_handler.go` (`OnApproved`), `internal/domain/request_transition.go`, `internal/usecase/transition_request_events.go`
- `internal/usecase/{generate_plan,commit_plan,get_plan_proposal,run_plan_generation,run_proposal_generation,analysis_runner,recover_interrupted_analysis_runs,resume_after_clarification,solution_regenerator,generate_solution}.go`, `internal/adapter/eventbus/clarification_resume_consumer.go`, `internal/adapter/postgres|mysql/analysis_run_repository.go`, `cmd/server/{wire_plan,wire_artifact}.go`
- `internal/domain/{plan_errors,solution_errors,engine_settings,request_flow_registry,openspec_profile,rpc_catalog}.go`, `internal/usecase/{ai_unavailable,manage_project_engine_settings}.go`
- `backend-go/proto/orca/request/v1/{plan,solution}.proto`, `backend-go/services/api-gateway/internal/adapter/wscompat/{channels_request_plan,channels_request_engine,channels_request_base_ref}.go`, `mcpserver/tools/excluded_channels.yaml`, `backend-go/policy/orca-authz/request.rego`
- `frontend/src/renderer/src/hooks/{request-plan-generation,usePlanRunInFlight,usePlanDecision}.ts`, `components/request/plan/{RequestPlanTab,PlanStates,plan-view-state}.tsx|ts`
- `specs/backend-go/crs/v6/request-artifact-model/IMPLEMENTATION-NOTES.md:98` (quyết định cũ), `specs/IMPLEMENTATION-PLAN-CR-REQ-039-042.md` mục 4.5 (loại chi phí e2e)
- [CR-REQ-012](./CR-REQ-012-plan-phase-task-generation-from-solution.md), [CR-REQ-013](./CR-REQ-013-phase-execution-and-feedback-loop.md), [CR-REQ-038](./CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md), [CR-REQ-045](./CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md)
- Solution: [BE-REQ-SOL-044](../../../../specs/backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-044-automatic-plan-generation.md), [FE-REQ-SOL-044](../../../../specs/frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-044-automatic-plan-generation.md)
- Mới: `internal/domain/plan_auto.go`, `internal/usecase/{enqueue_plan_auto,plan_auto_worker,retry_plan_generation}.go`, `internal/adapter/eventbus/plan_auto_consumer.go`, migration `0101_plan_auto_generation`
