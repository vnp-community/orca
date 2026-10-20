# CR-REQ-045 — Xem tài liệu OpenSpec của Plan và chỉnh Plan bằng prompt ngay tại cổng duyệt Plan

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-045 |
| **Tên** | Tab Plan có chế độ xem tài liệu mà AI đã viết (proposal, design, specs delta, tasks với engine OpenSpec; đề xuất Plan dạng cấu trúc với engine native), ô "Ask for changes" để người dùng gõ yêu cầu chỉnh và agent sinh phiên bản Plan mới, danh sách phiên bản, người duyệt (approver) được chỉnh; kèm làm sạch bí mật khi lưu bộ tài liệu và chỉ dẫn "sửa tại chỗ" trong prompt |
| **Loại** | Feature (backend và frontend) kèm gia cố (redaction, prompt, sửa các lệch ở đường sinh lại) |
| **Priority** | 🟠 P1 |
| **Effort** | Large (9 task backend, 8 task frontend) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | ✅ Đã triển khai (2026-10-20), chưa merge; e2e stub xanh, chưa chạy thật hạng C |
| **Phụ thuộc** | CR-REQ-012 (sinh Plan, `GeneratePlan`, `CommitPlan`), CR-REQ-026 (engine OpenSpec, `openspec_changes`, `bundle_json`), CR-REQ-010 (Approval), CR-REQ-016 (kênh gateway), CR-REQ-021 (tab Plan), CR-REQ-037 (tab Activity), CR-REQ-041 (mẫu chỉnh bằng prompt: `AuthorizeRevise`, `revision_feedback`, danh sách phiên bản), CR-REQ-039 (khoá nhánh gốc chỉ ảnh hưởng lượt chỉnh), CR-REQ-042 (danh mục prompt và golden), **[CR-REQ-044](./CR-REQ-044-automatic-plan-generation-after-analysis-approval.md) (commit Plan phía server, mục 2.6 và 2.14: `CommitPlan.ExecuteForSystem`, bảng `plan_auto_jobs`, bước `committing`; phụ thuộc cứng, xem mục 6 R1)**, CR-REQ-046 (phối hợp prompt, mục 2.7) |
| **Mở khoá** | Không |
| **Tác động** | `backend-go/services/request-service` (`usecase/generate_plan.go`, `plan_ports.go`, `solution_authorization.go`, `analysis_runner.go`, `run_proposal_generation.go`, `get_plan_proposal.go`, `get_plan_bundle_file.go`, `tasks_md_sync.go`, domain, adapter hai dialect, migration mới), `backend-go/prompts/{plan_openspec,plan_native}`, `backend-go/policy/orca-authz/request.rego`, `backend-go/proto/orca/request/v1/plan.proto`, `backend-go/services/api-gateway` (`wscompat`, `httpgateway`, MCP loại trừ), `backend-go/cmd/orca-cli`, `frontend/src/renderer/src/components/request/plan/`, `activity/`, 5 file locale |

> Nguồn: phản hồi của người dùng ngày 2026-10-20 (lỗi số 4): "Hiện giờ tôi chỉ xem được Plan ở dạng các bước lớn và task con. Tôi muốn xem các tài liệu OpenSpec change mà AI đã sinh (proposal, design, specs delta, tasks) và gửi prompt để agent sửa các tài liệu đó." Hiện trạng dựa trên đọc mã ngày 2026-10-20 (mục 1); chưa dựng giao diện và chưa chạy hệ thống. Điểm nào tôi không kiểm chứng được thì đánh dấu "chưa kiểm chứng".

---

## 1. Bối cảnh và vấn đề

### 1.1 Plan của engine OpenSpec được lưu ở đâu (đã đọc mã)

| Thứ | Nơi lưu | Ghi chú |
|---|---|---|
| Cây Plan (Plan, Phase, task) | task-service; `CommitPlan` ghi cây (`commit_plan.go`), cây lấy từ vùng `orca:begin plan` của `tasks.md` qua `domain.ParsePlanRegion(res.TasksMD, res.Ref)` trong `assemble` (`run_proposal_generation.go`) | Giao diện tab Plan chỉ đọc cây này (`usePlanTree`). |
| Bộ tài liệu OpenSpec change | `request.openspec_changes`, **một dòng mỗi Request** (`UNIQUE (tenant_id, request_id)`, `0007_solution_engines.up.sql`); cột `bundle_json` (`0094_openspec_change_provenance.up.sql`) là map đường dẫn tương đối với change → toàn văn mọi file dưới `openspec/changes/<id>/` (`proposal.md`, `design.md`, `tasks.md`, `specs/**`) | `domain.OpenSpecChange.Bundle`. Dòng bị ghi đè mỗi khi một lượt sinh Plan thành công (`Upsert` trong `finish`, `run_proposal_generation.go`). |
| Lịch sử các lần ghi | Chỉ có git trên nhánh `request/<n>-proposal` (cục bộ trên dev server, không bao giờ push), mỗi lần `finish` là một commit | Orca chỉ chạm git qua git-gateway (`adapter/grpcclient/proposal_workspace.go`); DB không giữ phiên bản cũ. |
| Trần kích thước | tổng 1 MiB (`maxBundleBytes`, `run_proposal_generation.go:275`), 200 file, sâu 6; mỗi file đọc tối đa 256 KiB (`MaxProposalReadBytes`, `ports_workspace.go:5`) | File vượt phần ngân sách còn lại được lưu thành dấu `<omitted size=N sha256=…>` (`omittedMarker`, `run_proposal_generation.go:385`). **File đơn lớn hơn 256 KiB không thành dấu**: `ReadFile` từ chối (`adapter/grpcclient/proposal_workspace.go:54`) nên cả lượt sinh hỏng với `REQUEST_SPEC_AGENT_FAILED`. |
| Mã change | `domain.NewChangeID(number, title)`, cố định từ lần sinh đầu (`Execute`, `run_proposal_generation.go`: `changeID := existing.ChangeID`) | Các lượt chỉnh dùng lại thư mục và nhánh. |
| Kiểm cấu trúc | agent chạy `openspec validate --strict`; Orca kiểm lại bằng `OpenSpecBundleChecker` (`openspec_bundle_checker.go:140`, `openspec validate <id> --strict --json`) | Change không qua kiểm không bao giờ được lưu. |
| Ràng buộc với phê duyệt | `PlanArtifacts.Describe` (`plan_subject_artifacts.go`) nhét `b:<TasksMDDigest>\|<BundleDigest>` vào digest của Approval | Sửa hay sinh lại sau khi Approval mở làm Approval cũ không còn khớp. |

### 1.2 Đường đọc đã có nhưng giao diện chưa dùng

- Kênh gateway `request.planProposal` trả `bundle` (danh sách file: đường dẫn, kích thước, cờ `omitted`; commit; nhánh; trạng thái; trạng thái đồng bộ), `checks` (kiểm cấu trúc và cảnh báo tri thức) và `knowledge` (agent đã đọc gì, khoảng trống, xung đột) (`usecase/get_plan_proposal.go`; `wscompat/channels_request_plan.go`, `addPlanEngineView`).
- Kênh `request.planBundleFile {id, path}` trả `{path, content, omitted}` từ DB, kiểm đường dẫn (`usecase/get_plan_bundle_file.go`); REST `GET /{id}/plan/bundle/file` (`httpgateway/request_routes_flow.go:42`); CLI `orca plan bundle --file` (`cmd/orca-cli/internal/command/plan_cmd.go`).
- Cả hai kênh nằm trong danh sách loại trừ MCP (`mcpserver/tools/excluded_channels.yaml:645-651`).
- Frontend không dùng `request.planBundleFile` ở đâu (`grep` trong `frontend/src` không thấy; `request-rpc-methods.ts` không có hằng tương ứng); `request.planProposal` chỉ được `request-plan-generation.ts` (chờ kết quả lượt sinh) và `usePlanRunInFlight.ts` gọi.

### 1.3 Backend đã có đường "sinh lại kèm phản hồi" cho Plan

- `GeneratePlan.Execute` (`generate_plan.go:84-143`) nhận `Feedback` (tối đa `maxFeedbackRunes` = 2000, lỗi `REQUEST_PLAN_FEEDBACK_TOO_LONG`). `checkPlanState` (`generate_plan.go:199`) cho phép `planning`, hoặc `awaiting_plan_approval` **chỉ khi có `Feedback`**. Trong một transaction: `StartRun` (lượt kiểu `plan`), `CancelPending(..., "revision_requested")` huỷ Approval đang chờ, `TriggerPlanRevision` đưa Request về `planning`; lượt chạy nền bắt đầu (`mode` = `AgentProposal` với engine openspec, `Complete` với native).
- Prompt nhận phản hồi qua partial `request_context/_feedback.tmpl` (khối `<feedback>`, bọc `fence`), dùng trong `plan_openspec/plan.tmpl` và `plan_native/plan.tmpl`.
- Lượt OpenSpec dùng lại mã change; `prepare` (`engine_openspec.go`) gọi `Ensure` rồi `DiscardAll` (dòng 214) nên worktree trở về đầu nhánh, tức các file của phiên bản trước **có mặt** trong thư mục agent làm việc; `finish` commit lại và ghi đè dòng DB.
- Commit Plan thành công thay Plan cũ (`SupersedesPlanID = req.PlanTaskID`, `commit_plan.go:106`) và mở Approval mới.

### 1.4 Các lệch và lỗ hổng phát hiện khi đọc mã

| # | Phát hiện | Bằng chứng | Hệ quả |
|---|---|---|---|
| G1 | Nút "Regenerate plan" và tuỳ chọn "regenerate from this feedback" của hộp thoại từ chối **không gửi phản hồi** và thực tế không chạy được ở cổng duyệt | `usePlanDecision.regeneratePlan()` gọi `generatePlanProposeCommit(request.id)` không `feedback` (`hooks/usePlanDecision.ts`); `checkPlanState` từ chối `awaiting_plan_approval` khi không có phản hồi (`REQUEST_STATE_STALE`). Từ chối Approval Plan đưa Request về `request_backlog` (`TransitionSubjectHandler.OnRejected` → `ReturnRequestToBacklog` → `TriggerPlanRejected`, `approval_artifact_handler.go:69-85`, `return_request_to_backlog.go:108-114`, `request_transition.go:98`), rồi `regeneratePlan()` đụng `checkPlanState` với trạng thái `request_backlog` | Hai đường "sinh lại" cũ của tab Plan đều lỗi trạng thái (suy ra từ mã, chưa chạy thử). Lý do người dùng gõ khi từ chối không bao giờ thành phản hồi. |
| G2 | Quyền ở cổng Plan là reporter hoặc `admin`/`lead`/`owner`; approver không có | `NewPlanActorAuthorizer` → `AuthorizeGenerate` (`plan_ports.go:41-61`); `wire_plan.go:27` dựng `ApproverAwareAuthorizer` nhưng `AuthorizePlan` chỉ gọi nhánh Generate. Cổng Analysis có `AuthorizeRevise` (CR-REQ-041) | Người đọc và quyết định Plan không chỉnh được. |
| G3 | Cổng cửa trước OPA chặn người không phải owner/reporter ở nhóm `plan` | `request.rego`: `group_roles["plan"] = {"owner","reporter"}`; `member_steering_rpcs` (dòng 43-47, 76-81) chỉ có `GenerateSolution`, `ChooseSolutionOption` cho nhóm `analyze` | Approver là member thường bị chặn trước khi tới Go (bài học của TASK-REQ-041-04). |
| G4 | Lượt Plan chạy dưới danh tính người gọi, không dưới reporter | `reporterIdentity` loại `RunKindPlan` (`analysis_runner.go:74-83`: `run.Kind == domain.RunKindPlan` → giữ danh tính người gọi); `GeneratePlan.Execute` gọi `ResolveForProject(ctx, ...)` (`generate_plan.go:93`) với ngữ cảnh người gọi | Dev server gắn với reporter nên approver gặp `REQUEST_PLAN_AI_UNAVAILABLE` hoặc lượt nền thất bại vì không tìm thấy kết nối. Cổng Analysis đã xử lý (`connCtx` trong `generate_solution.go`). |
| G5 | Không có khoá idempotency, không chặn lượt chỉnh thứ hai | `GeneratePlanRequest` chỉ có `request_id`, `feedback` (`plan.proto:42-45`); `run` không đặt `IdempotencyKey`; `StartRun` trả lượt đang chạy khi `Created=false` và `Execute` không kiểm | Bấm đúp sinh một lượt (nhờ unique `analysis_runs_one_running`), nhưng người chỉnh thứ hai nhận run của người thứ nhất như thể phản hồi của họ đã được áp (đúng lỗi 041-04 đã sửa ở cổng Analysis). |
| G6 | Lượt chỉnh thất bại làm mất phản hồi | Hết lượt, Request ở `planning`, Approval cũ đã huỷ, `Regenerate` không chữ chạy lại không phản hồi (cổng Analysis có `inheritedFeedback`, Plan không) | Người dùng phải gõ lại. |
| G7 | Trong lúc chỉnh, tab Plan không thấy lượt chạy | `usePlanRunInFlight` chỉ báo chạy khi `request.status === 'planning' && !request.planTaskId` (`hooks/usePlanRunInFlight.ts`); lúc chỉnh `planTaskId` còn nguyên | Tab hiển thị cây cũ, không Approval, không dấu hiệu đang chạy, không nút nào. |
| G8 | Bộ tài liệu lưu thô, không làm sạch bí mật | `finish`/`commitAndRead` lưu `string(b)` (`run_proposal_generation.go:353`); các đường native và Solution làm sạch (`RedactDocument` trong `EncodePlanProposal`, `plan_document.go:96-103`; `RedactSecrets` trong `analysis_secret_redaction.go:12`) | Bí mật chép từ mô tả Request vào `design.md` nằm nguyên trong DB và trả thẳng cho giao diện/CLI qua `request.planBundleFile`. |
| G9 | Prompt không dặn "sửa tại chỗ" | `plan_openspec/plan.tmpl` luôn nói "Write the change ONLY under openspec/changes/<id>/"; khối `RetryNote` mới nói "The files are still in place; fix them instead of starting over"; khối `_feedback.tmpl` không. `skills/tdd-openspec-author/SKILL.md` (133 dòng) không có chỉ dẫn nào về sửa phiên bản đã có (tìm "existing/revis/in place/feedback" chỉ thấy dòng 33 nói về mã nguồn) | Agent có thể viết lại cả bộ tài liệu thay vì sửa, làm phiên bản mới khác phiên bản cũ nhiều hơn yêu cầu. Với native, prompt không đưa **Plan cũ** vào, chỉ đưa phản hồi: model sinh Plan khác từ đầu. |
| G10 | Danh sách file báo sai kích thước file bị lược | `addChange` (`get_plan_proposal.go:133`) đặt `Size: int64(len(content))`, với file bị lược `content` là dấu nên `Size` là độ dài dấu, không phải kích thước file | Placeholder "file quá lớn" không thể hiện kích thước thật nếu dùng trường này. |
| G11 | Một lượt chỉnh không đổi gì thất bại với thông báo sai | `commitAndRead` trả "the worktree has nothing to commit" (`run_proposal_generation.go:328`), `finish` đổi thành `REQUEST_SPEC_AGENT_FAILED` "the change could not be committed to its branch" | Agent kết luận "tài liệu đã đáp ứng yêu cầu" và không sửa thì người dùng thấy lỗi commit khó hiểu. |
| G12 | Nhánh "analysis.revision_requested" của Activity không lọc theo loại lượt | `adapter/{postgres,mysql}/request_activity.go` (nhánh UNION đọc `analysis_runs` có `feedback`) không có điều kiện `kind` | Ngay khi Plan có phản hồi, dòng Activity sẽ ghi nhầm "Requested changes to the analysis". |
| G13 | Commit Plan phụ thuộc trình duyệt và quyền reporter/admin | `generatePlanProposeCommit` chạy "propose" rồi "commit" từ tab (`hooks/request-plan-generation.ts`); `CommitPlan` cũng gọi `AuthorizePlan` (`commit_plan.go:78`) | Đóng tab giữa chừng thì Request kẹt `planning`; approver chỉnh xong không commit được (sau khi Approval bị huỷ họ không còn là approver). [CR-REQ-044](./CR-REQ-044-automatic-plan-generation-after-analysis-approval.md) mục 2.6 chuyển commit sang server cho lượt tự động nhưng **không** tự commit lượt có `feedback` do người bấm tay (D3 của 044); phần đó thuộc CR này (mục 2.3). |

### 1.5 Vấn đề của người dùng

1. Không xem được OpenSpec change (proposal, design, specs delta, tasks) mà AI sinh, dù dữ liệu đã nằm trong DB và có kênh đọc.
2. Không có cách nói "sửa chỗ này" cho Plan đã đề xuất mà không từ chối nó (từ chối đưa Request về backlog).
3. Không thấy các phiên bản Plan trước và yêu cầu đã dẫn tới chúng.

## 2. Giải pháp đề xuất

### 2.1 Phạm vi

| Loại Request | Engine (mặc định) | Tab Plan có "xem tài liệu" | Chỉnh bằng prompt |
|---|---|---|---|
| `change_request`, `refactor` | openspec | Có: proposal, design, từng spec delta, tasks | Có |
| `bug`, `performance`, `ops_request`, `security` | native | Có: đề xuất Plan dạng cấu trúc, chỉ đọc | Có (`security`: Approval của cổng là `pre_deploy`) |
| `task`, `docs` | native (task list) | Có (đề xuất rất nhỏ) | Có, dùng chung đường code; không xử lý riêng |
| `hotfix` | native, Plan là một task sửa tạo từ chẩn đoán (`PlanSingleTask`, Approval `pre_deploy`) | Không | **Không**: đường tạo Plan khác (`bindHotfixFixTask`); `GeneratePlan` từ chối bằng `REQUEST_PLAN_REVISION_NOT_APPLICABLE` (mục 2.3) |
| `spike`, `question` | không có Plan | Không áp dụng | Không áp dụng |

Engine do Request ghim (`request.engine`), không do loại: `GeneratePlan` quyết định chế độ qua `EnginePinner` (`modeFor`). Giao diện hiển thị "Documents" chỉ khi `request.engine === 'openspec'`, hiển thị "Proposal" khi native.

### 2.2 Xem tài liệu (chỉ đọc)

Tab Plan thêm chế độ xem thứ ba cạnh Tree và Graph:

- **OpenSpec** ("Documents"): thanh tab theo tài liệu, thứ tự cố định: `proposal.md`, `design.md`, mỗi `specs/<capability>/spec.md` (nhãn = tên capability), `tasks.md`, rồi các file khác nếu có (hiếm). Mỗi tài liệu lấy từng file theo yêu cầu (`request.planBundleFile`), không tải cả bộ; kết quả được nhớ trong phiên xem. Văn bản hiển thị bằng `RequestMarkdownContent` (react-markdown, không HTML thô, chỉ liên kết http/https); thêm plugin `remark-gfm` cho tài liệu này (task list `- [ ]` và bảng, có trong `frontend/package.json`) dưới dạng tuỳ chọn để không đổi cách hiển thị Solution; nút "Source" chuyển sang văn bản thô vì HTML comment `<!-- orca:begin plan ... -->` bị renderer bỏ.
- File `omitted`: khối "This file is too large to display (N KB). It is on the proposal branch." với kích thước lấy từ dấu (xem G10, sửa ở backend).
- Dải trên cùng: kiểm tra (`checks`: `STRUCTURE_OK`, cảnh báo tri thức) và tri thức (`knowledge`: có sẵn không, snapshot, số phần tử đã đọc, khoảng trống, xung đột) dưới dạng đoạn gọn mở rộng được, lời của agent được ghi rõ là lời của agent (không phải sự kiện Orca đã kiểm).
- **Native** ("Proposal"): đề xuất Plan của lượt sinh gần nhất thành công (`request.planProposal` với `runId` của phiên bản đang xem): `summary`, `notes`, từng Phase và task với `description`, loại task, nhãn, `depends_on`, ước lượng giờ, cờ `irreversible`; `prompt_template` thu gọn. Chỉ đọc, cùng ô chỉnh.
- Mọi người đọc được Request đều xem được (nhóm quyền `read`).

### 2.3 Chỉnh bằng prompt (cùng mẫu CR-REQ-041)

1. Request ở `awaiting_plan_approval`, người có quyền thấy ô **Ask for changes** (dưới thanh duyệt Plan, hiện ở cả ba chế độ xem).
2. Gõ yêu cầu (tối đa 2000 ký tự, bộ đếm), hoặc bấm chip gợi ý; bấm **Revise plan**.
3. Frontend gọi `request.generatePlan` (`mode: propose`) với `feedback` và `idempotencyKey` mới; không từ chối Approval trước.
4. Backend đi đường sẵn có (mục 1.3): huỷ Approval với lý do `revision_requested`, Request về `planning`, lượt chạy nền. Giao diện hiện "Writing version n+1…" và khoá thanh duyệt.
5. Khi lượt xong, **server** commit Plan (qua hạ tầng của CR-REQ-044, mục "Commit phía server" bên dưới), Request trở về `awaiting_plan_approval` với Plan mới và Approval mới; bản cũ nhãn Superseded.
6. Lặp lại tuỳ ý, không giới hạn số lần (E2). Duyệt đi tiếp sang `executing`. Từ chối vẫn là lối thoát riêng (về backlog), không đổi.

**Commit phía server cho lượt chỉnh.** CR-REQ-044 mục 2.14 chỉ định cách dùng lại: trong cùng transaction với `StartRun`, `GeneratePlan` chèn một job vào `request.plan_auto_jobs` với `origin = 'revision'` (giá trị mới, mở rộng `CHECK` của cột `origin` bằng migration của CR này), `state = 'running'`, `run_id` của lượt vừa tạo, `max_attempts = 1`; worker của 044 lo phần còn lại (đợi lượt xong, `DecodePlanProposal(run.RawOutput)`, `CommitPlan.ExecuteForSystem` với người báo cáo làm `actor`, thử lại commit tối đa 5 lần trên cùng lượt đã thành công, không gọi lại AI). Quy tắc tạo job: Request đã có `PlanTaskID` (đang thay Plan) **và** lượt có `Feedback` khác rỗng, thật hay kế thừa; lần sinh Plan đầu bấm tay (không `PlanTaskID`) giữ luồng cũ, commit từ tab. Luật "đã có `PlanTaskID` thì `superseded`" của worker chỉ áp cho `origin` `event`/`retry` (044 mục 2.8). **Khi 044 bị tắt** (`REQUEST_PLAN_AUTO_GENERATE=false`, không có worker): không có job; chỉnh bởi reporter/admin vẫn chạy như hôm nay (tab propose rồi commit bằng quyền của họ), còn approver không phải reporter/admin bị từ chối với `REQUEST_PLAN_FORBIDDEN`, vì không còn ai commit thay họ. Giao diện biết chế độ nào qua cờ tính năng `request.planAuto` của 044.

Các sửa đi kèm ở đường sinh lại (G4 đến G6, G11, hotfix):

- **Phản hồi của lượt thất bại không mất**: `GeneratePlan` ở `planning` không kèm chữ thì kế thừa `feedback` của lượt Plan gần nhất nếu nó `failed` (mẫu `inheritedFeedback` của cổng Analysis, task 02). Chỉnh mới thay hẳn phản hồi cũ.
- **Người chỉnh thứ hai bị từ chối**: yêu cầu có `feedback` mà `StartRun` trả lượt của người khác (khác `idempotencyKey`) thì trả lỗi mới `REQUEST_PLAN_REVISION_IN_PROGRESS`; cùng khoá thì trả lại lượt đó (bấm đúp).
- **Dev server tra cứu như reporter** khi người gọi khác reporter, lượt chạy vẫn ghi cho người gọi: sửa `GeneratePlan.Execute` (`connCtx`) và `reporterIdentity` (bỏ loại trừ `RunKindPlan`; chỉ dùng danh tính reporter để tra kết nối, không để ghi). Chưa kiểm chứng vì sao `RunKindPlan` bị loại ban đầu (comment ghi "Plan runs keep the caller's identity"); task 02 đọc lịch sử và test mọi lời gọi ngoài việc tra dev server trước khi đổi.
- **Lượt chỉnh không đổi gì** thất bại với mã rõ `REQUEST_PLAN_REVISION_NO_CHANGE` ("The agent did not change any document. Say more precisely what to change.") thay cho lỗi commit (G11).
- **`hotfix`** bị từ chối ở đường chỉnh (`REQUEST_PLAN_REVISION_NOT_APPLICABLE`).

### 2.4 Quyền chỉnh (quyết định)

Người có quyền duyệt cổng Plan đang chờ được chỉnh, cùng với người báo cáo và `admin`/`lead`/`owner`.

- Thêm `SolutionActorAuthorizer.AuthorizeRevisePlan(ctx, req)` (cùng mẫu `AuthorizeRevise` của CR-REQ-041): `ReporterOrAdminAuthorizer` = `AuthorizeGenerate`; `ApproverAwareAuthorizer` qua nếu nhánh Base qua, nếu không duyệt Approval **đang chờ** của Request thuộc cổng Plan (`plan`, `task_list`, `pre_deploy`) và qua khi `Decider.CanDecide` thành công cho bất kỳ Approval nào (user, team, role tính như lúc bấm Approve). Lỗi hạ tầng trả nguyên, không nuốt thành "không có quyền".
- `PlanActorAuthorizer` thêm `AuthorizePlanRevise`; `GeneratePlan.Execute` chọn `AuthorizePlanRevise` khi `req.Status == awaiting_plan_approval && Feedback != ""`, còn lại `AuthorizePlan` (người duyệt không tự khởi động lần sinh Plan đầu). Kiểm quyền chạy **trước** transaction huỷ Approval (sau khi huỷ, người đó không còn là approver của Approval nào); test khẳng định thứ tự. `AuthorizePlan` cho đường Request đã có Plan ở `planning` được thay bằng `AuthorizePlanRevise` (cùng nhánh Base cho reporter/admin).
- `request.rego`: thêm nhóm `member_plan_steering_rpcs := {"GeneratePlan"}` cho hành động `plan` với vai trò dự án `member` (cùng cách `member_steering_rpcs`); Go quyết định phần còn lại. Người không phải thành viên dự án vẫn bị chặn ở cổng cửa trước.
- **Thử lại lượt chỉnh thất bại của chính mình.** Lượt thất bại để Request ở `planning` và Approval cũ đã huỷ, nên approver không còn là approver của Approval nào. `AuthorizePlanRevise` vì vậy cũng qua khi Request đang `planning` có Plan, người gọi là `ActorID` của lượt Plan gần nhất, lượt đó `failed` và có `Feedback` (thật). Reporter/admin luôn qua nhánh Base. Người khác (kể cả approver khác) không thử lại hộ được.
- `CommitPlan` giữ `AuthorizePlan` cho đường công khai; lượt chỉnh được commit bởi worker của 044 bằng `CommitPlan.ExecuteForSystem` nên người chỉnh không cần quyền commit (R1).
- Mọi người đọc được Request đọc được `revision_feedback` của mọi phiên bản (không thêm tầng quyền, giống E8 của CR-REQ-041).

### 2.5 Lịch sử phiên bản (quyết định: lưu lịch sử, có giữ chữ giới hạn)

DB hiện chỉ giữ một bản (mục 1.1). Để giao diện hiển thị được "các phiên bản" và xem lại bản cũ ở chế độ chỉ đọc, CR này thêm bảng `request.openspec_change_revisions` (migration mới, hai dialect):

| Cột | Ý nghĩa |
|---|---|
| `id`, `tenant_id`, `request_id` | khoá; `UNIQUE (tenant_id, request_id, revision)` |
| `revision` | 1, 2, 3, … theo thứ tự các lượt sinh Plan **thành công** của Request (bằng thứ tự lượt Plan `succeeded`) |
| `run_id` | lượt sinh sinh ra bản này; phản hồi và người chỉnh lấy từ `analysis_runs` (không sao chép) |
| `commit_sha`, `tasks_md_digest`, `bundle_digest` | định danh bản; digest tính trên bản đã làm sạch (mục 2.6) |
| `files_json` | chỉ mục file: đường dẫn, kích thước thật, `omitted`, `sha256` |
| `bundle_json` | toàn văn (đã làm sạch); **NULL sau khi bị lược** |
| `created_at` | |

- Ghi **trong cùng transaction** với `Upsert` của `finish` (một bản mỗi lượt thành công).
- Giữ toàn văn cho `PlanRevisionTextKept = 10` bản mới nhất; bản cũ hơn bị đặt `bundle_json = NULL` ngay trong transaction ghi bản mới, chỉ mục và digest giữ mãi. Chặn trần lưu trữ: tối đa 10 MiB mỗi Request trong trường hợp xấu nhất, thường dưới 1 MiB. Giao diện báo "The text of this version is no longer kept" cho bản đã lược (quyết định E5, hằng số, chỉnh được bằng một dòng mã).
- Với native không cần bảng: phiên bản là các lượt Plan `succeeded`, đề xuất nằm ở `analysis_runs.raw_output` (đã làm sạch bởi `EncodePlanProposal`).
- "Đã đổi gì": mỗi file của một bản có nhãn `added`/`changed`/`unchanged` (và `removed` cho file có ở bản trước, mất ở bản này) so với bản liền trước, tính từ `sha256`. **So sánh từng dòng nằm ngoài CR này** (quyết định E6); người dùng mở hai bản cạnh nhau bằng bộ chuyển phiên bản. Kiểm tra (`checks`) và `knowledge` chỉ có cho bản hiện hành (cột tri thức nằm trên dòng hiện hành, không sao chép theo bản).
- Số phiên bản = `revision`. Lượt thất bại không tạo phiên bản.

### 2.6 Làm sạch bí mật (quyết định: làm sạch khi lưu, cộng thêm khi đọc)

- **Khi lưu**: `commitAndRead` chạy `RedactSecrets` (`common/secretscan`, cửa sổ quét 1 MiB lớn hơn mỗi file tối đa 256 KiB) trên từng file trước khi vào `Bundle`; số bí mật đã thay lưu ở cột mới `redacted_secrets` của `openspec_changes`; `planChecksFor` thêm cảnh báo `SECRETS_REDACTED` ("N secrets were masked in the stored copy; the file on the proposal branch is unchanged"). `BundleDigest`, `Describe` và bảng phiên bản dùng bản đã làm sạch nên nhất quán.
- **Khi đọc**: `GetPlanBundleFile` chạy `RedactSecrets` lần nữa, rẻ, để che luôn các dòng cũ (đã lưu thô trước CR này), không cần backfill.
- **Điểm vướng đã xử lý**: `TasksMdSyncer.restoreDrift` (`tasks_md_sync.go`, quanh dòng 300) khôi phục vùng Plan của `tasks.md` thật từ `Bundle["tasks.md"]` khi phát hiện bị sửa tay. Nếu bản lưu đã bị che, khôi phục sẽ ghi `[REDACTED]` vào file thật. Task 03 chặn việc đó: khi `redacted_secrets > 0` thì không khôi phục từ bản lưu và lấy `TasksMDDigest` (digest bản thô) làm mốc.
- **Giới hạn nói rõ**: nhánh git `request/<n>-proposal` trên dev server vẫn chứa văn bản agent đã viết; làm sạch chỉ bảo vệ DB, giao diện và CLI.

### 2.7 Prompt (quyết định: dặn "sửa tại chỗ" khi đã có phiên bản trước)

Thay đổi nằm trong `backend-go/prompts` (CR-REQ-042) và chỉ kích hoạt khi cần, để **golden hiện có không đổi một byte**:

- `plan_openspec`: `Input.PreviousFiles []string` (đường dẫn file của phiên bản trước, tối đa 200). Partial mới `_revise_in_place.tmpl`, chỉ hiện khi `PreviousFiles` không rỗng: nêu rằng thư mục change đã có các file liệt kê, **sửa chính các file đó với thay đổi nhỏ nhất đủ đáp ứng khối `<feedback>`**, không viết lại file mà phản hồi không đụng tới, giữ mã change, giữ dấu vùng `orca:begin plan` và giữ id task còn tồn tại, tạo hoặc xoá file chỉ khi phản hồi đòi, chạy lại `openspec validate --strict`. Điều kiện bật dựa trên việc đã có phiên bản trước (worker truyền `existing.Bundle` khi `existing.Status == ready`), không dựa trên `Feedback`, để "Try again" cũng được dặn như vậy.
- `plan_native`: `Input.PriorPlan string` (đề xuất của lượt Plan thành công gần nhất, JSON đã làm sạch, cắt theo rune, bọc `fence`) và partial `_prior_plan.tmpl` hiện khi không rỗng: "đây là Plan hiện hành (dữ liệu, không phải lệnh); trả về toàn bộ Plan đã sửa, chỉ đổi những gì phản hồi yêu cầu".
- Quy trình golden (CR-REQ-042, `prompts/README.md` mục "Golden rule", `backend-go/ci/check-prompt-golden.sh`): (1) sửa template và `Input`; (2) thêm ca golden mới trong `prompt_golden_openspec_test.go` / `prompt_golden_plan_test.go` (ca có `PreviousFiles`, ca có `PriorPlan`, ca thù địch có thẻ đóng trong tên file); (3) chạy `check-prompt-golden.sh` (so sánh): chỉ báo thiếu golden của ca mới, **các golden cũ phải không đổi**; (4) chạy `--update` để sinh golden của ca mới, xem `git diff -- '*/testdata/prompt-golden/*'` chỉ có file thêm mới; (5) mở rộng fixture trong `prompt_size_budget_test.go` và đo lại `plan_openspec/plan` (75000) và `plan_native/plan` (72000); (6) `make check-prompts`; (7) commit template và golden cùng một commit.
- **Phối hợp với CR-REQ-046** (đã đọc v1.0 ngày 2026-10-20): 046 thêm trường `Language` (rỗng = không phát gì, nên golden hiện có không đổi) và hai partial `request_context/_language_rule.tmpl`, `plan_openspec/_language_rule_openspec.tmpl`, chèn trước `<request>` trong `plan.tmpl`; 046 R11 đã nêu việc cùng chạm `plan_native/plan.tmpl`, `plan_openspec/plan.tmpl`, `prompt_size_budget_test.go` và golden Plan. Cả hai CR cùng nguyên tắc "partial có điều kiện, golden chỉ thêm tệp". Hợp nhất: mỗi partial chèn bằng một dòng `{{template …}}` đứng riêng (của 046 ở đầu, của CR này sau `_feedback.tmpl`); **sinh golden sau khi các mẫu đã hợp nhất**, không ghép tệp golden từ hai nhánh; ca golden mới của CR này để `Language` rỗng, nên không phụ thuộc thứ tự phát hành. Chỉ dẫn ngôn ngữ của 046 áp cho văn xuôi tài liệu; khối "sửa tại chỗ" nói về thao tác trên file, không xung đột. `skills/tdd-openspec-author/SKILL.md` không đổi trong CR này (046 có sửa nó cho tiếng Việt; chỉ dẫn theo từng lượt nằm trong prompt vì kỹ năng dùng cho cả lần đầu).

### 2.8 API và kênh

| Kênh / RPC | Thay đổi |
|---|---|
| `request.generatePlan` (`GeneratePlan`) | thêm `idempotencyKey` (proto trường 3; CR-REQ-044 thêm trường tương ứng `GeneratePlanInput.IdempotencyKey` ở tầng usecase cho đường hệ thống, CR này nối nó với RPC công khai); lỗi mới `REQUEST_PLAN_REVISION_IN_PROGRESS`, `REQUEST_PLAN_REVISION_NOT_APPLICABLE`, `REQUEST_PLAN_REVISION_NO_CHANGE` (lỗi của lượt chạy) |
| `request.planProposal` (`GetPlanProposal`) | thêm `revision` (int, 0 = lượt gần nhất như hiện nay); `bundle.files[]` thêm `sha256`, `change`; `Size` của file bị lược là kích thước thật |
| `request.planBundleFile` (`GetPlanBundleFile`) | thêm `revision` (0 = hiện hành); bản đã lược chữ trả `REQUEST_PLAN_REVISION_TEXT_PRUNED` |
| `request.planRevisions` (`ListPlanRevisions`, **mới**, nhóm `read`) | `{id, pageSize?, pageToken?}` → `{revisions: [{revision, runId, status current/superseded, engine, revisionFeedback, actorId, generatedAt, commit, filesChanged, textKept, stats?}], nextPageToken}`, mới nhất trước, mặc định 20, tối đa 100 |
| Activity | kind mới `plan.revision_requested` (mục 2.9) |
| REST / CLI | `GET /{id}/plan/bundle/file?revision=`, `GET /{id}/plan/revisions`; CLI `orca plan bundle --revision`, `orca plan revisions` |
| MCP | **không thêm công cụ**: giữ loại trừ, ghi lý do (mục 2.10) |

### 2.9 Giao diện

Thư mục `frontend/src/renderer/src/components/request/plan/` (thêm `documents/` và `revision/`); chi tiết ở FE-REQ-SOL-045:

- `PlanSummaryHeader` thêm mục "Documents" (OpenSpec) hoặc "Proposal" (native) vào bộ chuyển Tree/Graph.
- `documents/PlanDocumentsView`, `PlanDocumentTabs`, `PlanDocumentBody`, `PlanChecksPanel`, `PlanProposalView`, `usePlanBundleFile`, `plan-document-model.ts`.
- `revision/PlanRevisionPanel` (dùng chung phần textarea/đếm/chip/phím tắt với `AnalysisRevisionPanel` sau khi tách `RevisionPromptBox`), `usePlanRevision`, `PlanVersionSwitcher`, `plan-revision-errors.ts`, chip gợi ý theo engine: OpenSpec "Add a task for tests", "Split the biggest task", "Tighten the design", "Add a scenario for X"; native "Split the largest task", "Add a rollback task", "Reorder to ship the risky part first".
- Bỏ nút "Regenerate plan" ở trạng thái chờ duyệt và tuỳ chọn "regenerate from this feedback" của hộp thoại từ chối Plan (G1): thay bằng panel chỉnh; "Generate plan" của `PlanStates` (Request ở `planning` chưa có Plan) giữ nguyên.
- Trong lúc chỉnh: dùng lại hạ tầng của CR-REQ-044 (FE-REQ-SOL-044): `usePlanRunState` (bật cả khi `planTaskId` đã có, thay `usePlanRunInFlight`; giải quyết G7), `derivePlanAutoDisplay`, `PlanRunFailedCard`, `PlanRewriteStrip`; không viết hook thăm dò thứ hai. Thẻ lỗi lượt thất bại hiện ở trạng thái `ready` khi đang thay Plan, nút "Try again" gọi `request.generatePlan` (phản hồi được server kế thừa), không phải `request.retryPlan`, vì quyền người chỉnh (mục 2.4).
- Activity: dòng "Requested changes to the plan" (`plan.revision_requested`), **không chứa nội dung phản hồi** (C9); bộ lọc nhóm "changes" nhận kind mới; nhánh `analysis.revision_requested` của SQL được lọc `kind <> 'plan'` (G12).
- Mỗi file dưới 300 dòng mã; không thêm `max-lines` disable; token của `main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); 5 locale.

### 2.10 Ngoài phạm vi, kèm lý do

**Người dùng tự sửa tay tài liệu: không làm (quyết định của người yêu cầu).** Lý do để ghi lại:

1. Không có đường ghi: bộ tài liệu trong DB chỉ là ảnh chụp của một commit trên nhánh proposal ở dev server; nguồn thật của agent và của `openspec archive` là nhánh git. Ghi vào DB làm DB và nhánh lệch nhau, ghi vào nhánh đòi đường ghi file mới qua git-gateway cùng giải quyết xung đột với lượt agent đang chạy.
2. Digest của Approval gắn với `b:<TasksMDDigest>|<BundleDigest>`: mọi chỉnh tay đổi nó, nên phải huỷ và mở lại Approval, tức một "sinh lại" không có người viết.
3. `tasks.md` là nguồn của cây Plan (`ParsePlanRegion`) và được `TasksMdSyncer` đồng bộ hai chiều với phát hiện sửa tay (vùng bị sửa tay được khôi phục, `restoreDrift`); sửa tay tasks đòi phân tích lại, kiểm giới hạn Plan và commit lại cây ở task-service, tức một đường commit thứ hai.
4. Tách nhiệm: người duyệt tự sửa nội dung rồi tự duyệt làm yếu kiểm soát mà Approval tồn tại để bảo đảm. Chỉnh bằng lời qua agent giữ toàn bộ rào đã có (kiểm `openspec validate`, kiểm phạm vi thư mục, kiểm cấu trúc, kiểm Plan, lịch sử).

**Công cụ MCP đọc hai kênh: không làm, giữ loại trừ.** Văn bản trong tài liệu do agent viết từ nội dung Request, nên đưa nó vào ngữ cảnh của một agent khác qua MCP mở một đường chèn chỉ dẫn; agent không được gọi `GeneratePlan` (`AgentAllowed=false` trong `rpc_catalog.go`) nên công cụ đọc không có luồng công việc đi kèm; con người đã có CLI (`orca plan bundle`). Xem lại khi có vai trò agent "reviewer" với ngữ cảnh được bọc như dữ liệu không tin cậy.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| E1 | Chỉnh là hành động riêng, gọi `generatePlan` có `feedback`, không đi qua "từ chối" | Từ chối đưa Request về backlog (G1); chỉnh là hội thoại |
| E2 | Không giới hạn số lần chỉnh | Quyết định của người yêu cầu; rào có sẵn (một lượt chạy mỗi Request, `MaxAgentRunsPerProject`, khoá idempotency) giữ nguyên |
| E3 | Áp dụng cho Plan native và OpenSpec; `hotfix` ngoại lệ | Cùng đường `GeneratePlan`; hotfix tạo Plan bằng đường khác |
| E4 | Approver của cổng Plan được chỉnh (`AuthorizeRevisePlan`), thêm cửa OPA cho `GeneratePlan` | Quyết định của người yêu cầu; cùng mẫu CR-REQ-041; Go quyết định, OPA chỉ mở cửa |
| E5 | Bảng lịch sử, giữ toàn văn 10 bản mới nhất, chỉ mục và digest giữ mãi | Cần "phiên bản" cho UI; không giới hạn số lần chỉnh nên phải chặn trần lưu trữ |
| E6 | "Đã đổi gì" ở mức file bằng `sha256`, không diff từng dòng | Không cần thêm thư viện hay RPC git; diff dòng để CR sau nếu cần |
| E7 | Làm sạch bí mật khi lưu và khi đọc; không backfill | Khi lưu cho dữ liệu mới và digest nhất quán; khi đọc che luôn dữ liệu cũ |
| E8 | Điều kiện bật chỉ dẫn "sửa tại chỗ" là có phiên bản trước, không phải có `Feedback` | "Try again" sau lượt thất bại cũng cần; golden cũ không đổi |
| E9 | Native đưa Plan hiện hành vào prompt khi chỉnh | Không có nó, model sinh lại từ đầu và phản hồi như "tách task 3" vô nghĩa |
| E10 | Lượt chỉnh không đổi gì là lỗi `REQUEST_PLAN_REVISION_NO_CHANGE`, không phải phiên bản trùng | Tránh phiên bản giống hệt trong danh sách và Approval vô ích; xem câu hỏi mở 3 |
| E11 | Không có đường ghi tay; không thêm MCP | Mục 2.10 |
| E12 | Lượt chỉnh được server commit bằng job `origin = 'revision'` của CR-REQ-044; khi 044 tắt, chỉ reporter/admin chỉnh được | Đóng tab giữa chừng không kẹt; approver không cần quyền commit; không có đường commit thứ hai |
| E13 | Người yêu cầu chỉnh được thử lại lượt thất bại của chính họ | Sau khi Approval bị huỷ họ mất tư cách approver; nếu không họ không dùng lại được yêu cầu của mình |

## 4. Tiêu chí chấp nhận

- [ ] Request `change_request` (OpenSpec) đã có Plan: tab Plan có "Documents" với đủ tab `proposal.md`, `design.md`, mỗi spec delta, `tasks.md`; mỗi tab lấy file theo yêu cầu và hiển thị Markdown; "Source" hiện văn bản thô gồm dấu `orca:begin plan`.
- [ ] File bị lược hiện placeholder "too large" với kích thước thật; danh sách file trả `size` đúng cho file bị lược.
- [ ] Dải kiểm tra hiện `STRUCTURE_OK` và cảnh báo tri thức; tri thức do agent báo được ghi rõ là lời của agent.
- [ ] Request native (`bug`) có "Proposal" với `summary`, `notes`, Phase và task chỉ đọc.
- [ ] `hotfix`, `spike`, `question` không có ô chỉnh; `hotfix` gọi `generatePlan` có phản hồi bị `REQUEST_PLAN_REVISION_NOT_APPLICABLE`.
- [ ] Chỉnh hai lần liên tiếp bằng giao diện: mỗi lần Approval cũ có lý do `revision_requested`, Request đi `planning` rồi về `awaiting_plan_approval` với Approval **mới**; danh sách có 3 phiên bản (1 hiện hành, 2 superseded); `revisionFeedback` đúng chữ đã gõ.
- [ ] Lặp lại ít nhất 5 lần liên tiếp không bị chặn; số phiên bản tăng đúng; bản cũ hơn 10 bản mất chữ nhưng giữ chỉ mục và digest.
- [ ] Người gõ đúng 2000 ký tự được; 2001 bị `REQUEST_PLAN_FEEDBACK_TOO_LONG` từ API và giao diện chặn trước.
- [ ] Approver (theo user, team, role) là member dự án, không phải reporter/admin/lead/owner, chỉnh được; người lạ (không phải member) bị chặn; member không phải approver bị `REQUEST_PLAN_FORBIDDEN` và Approval cùng bản hiện tại còn nguyên.
- [ ] Approver không khởi động được lần sinh Plan đầu (Request ở `planning` chưa có Plan).
- [ ] `AuthorizePlanRevise` chạy trước khi Approval cũ bị huỷ (test thứ tự).
- [ ] Bấm đúp Revise chỉ sinh một lượt; chỉnh thứ hai khi lượt đang chạy bị `REQUEST_PLAN_REVISION_IN_PROGRESS`, giao diện làm mới.
- [ ] Dev server offline: `REQUEST_PLAN_AI_UNAVAILABLE`, Approval và bản hiện tại còn nguyên (kiểm trước khi ghi).
- [ ] Lượt chỉnh thất bại (`[stub:fail-once]`, hết hạn, JSON sai): Request ở `planning`, thẻ lỗi có "Try again", bấm "Try again" chạy lại **với cùng phản hồi** và thành công, Approval mới mở.
- [ ] Lượt chỉnh không đổi gì: `REQUEST_PLAN_REVISION_NO_CHANGE`, Request ở `planning`, chỉnh lại được.
- [ ] Approver chỉnh xong, Plan được commit bởi server (job `revision`) và Approval mới mở dù tab đã đóng và approver không có quyền commit.
- [ ] Approver thử lại được lượt chỉnh thất bại của chính họ; approver khác không; reporter/admin luôn thử lại được.
- [ ] Khi `REQUEST_PLAN_AUTO_GENERATE=false`: reporter/admin chỉnh được như hôm nay (commit từ tab), approver bị `REQUEST_PLAN_FORBIDDEN`.
- [ ] Bộ tài liệu có bí mật cắm sẵn (ví dụ khoá giả dạng AWS) được thay `[REDACTED]` khi lưu; dòng cũ lưu thô được che khi đọc; `redacted_secrets` > 0 và cảnh báo `SECRETS_REDACTED` hiện; `TasksMdSyncer` không ghi `[REDACTED]` vào `tasks.md` thật.
- [ ] Prompt lượt chỉnh OpenSpec có khối "sửa tại chỗ" liệt kê file của phiên bản trước; prompt lượt chỉnh native có Plan hiện hành; mọi golden hiện có không đổi; ngân sách kích thước prompt còn xanh.
- [ ] Phản hồi không xuất hiện trong log, span hay khung sự kiện; Activity có `plan.revision_requested` không chứa nội dung và không còn dòng `analysis.revision_requested` cho lượt Plan.
- [ ] Migration xuôi và ngược trên Postgres 16 và MySQL 8; Request cũ (một dòng `openspec_changes`, không có dòng bản) vẫn xem được và coi như bản 1.
- [ ] Đủ khoá i18n 5 locale; `pnpm lint` xanh; không file nào vượt `max-lines`; `make check-prompts` và `opa-test` xanh.

## 5. Kiểm thử

- **Unit backend**: `checkPlanState`/`GeneratePlan` với và không `feedback`; tạo job `revision` cùng transaction với `StartRun` (và không tạo ở lần sinh đầu); worker của 044 không bỏ job `revision` vì có `PlanTaskID`; `AuthorizePlanRevise` (bảng reporter, admin, lead, owner, approver theo user/team/role, member thường, chỉ đọc, người lạ × ba kiểu Approval `plan`, `task_list`, `pre_deploy`); thứ tự kiểm quyền trước `CancelPending`; `refuse second revision`, `inherited feedback`, khoá idempotency; hotfix; `NO_CHANGE`; `reporterIdentity` cho lượt Plan (kết nối tra như reporter, lượt ghi cho người gọi); redaction (một bí mật mỗi loại `secretscan`, nhiều file, đúng 256 KiB, dấu `omitted` không lộ); `restoreDrift` với `redacted_secrets`; phân tích dấu `omitted`; đánh số phiên bản; lược chữ ở bản thứ 11.
- **Rego** (`make opa-test`, `request_test.rego`): member qua cửa `GeneratePlan`, không qua `CommitPlan`/`StartPhase`; không phải member bị chặn.
- **Hợp đồng hai dialect** (testcontainers Postgres 16 và MySQL 8): bảng `openspec_change_revisions` (ghi cùng transaction với `Upsert`, ràng buộc duy nhất, lược chữ, Request cũ không có dòng), cột `redacted_secrets`, `ListPlanRevisions` phân trang ổn định, nhánh Activity mới và nhánh cũ đã lọc `kind`, EXPLAIN chỉ mục trên Request có ≥ 50 lượt.
- **Prompt**: golden mới (ca `PreviousFiles`, ca `PriorPlan`, ca thù địch), golden cũ không đổi, `prompt_size_budget_test.go`, test gói `prompts` (`go test ./... -race`).
- **Gateway**: `wscompat` test cho `request.generatePlan` có `idempotencyKey`, `request.planRevisions`, `request.planProposal` và `request.planBundleFile` có `revision`; `channel_inventory.json`; `parity_test` MCP; REST và CLI.
- **Component (vitest)**: viewer (tải từng file, lỗi, rỗng, bị lược, Source), checks panel, proposal native, panel chỉnh (rỗng, đếm, chip, đang chạy, lỗi, giữ chữ), bộ chuyển phiên bản (bản cũ chỉ đọc, bản bị lược chữ), ẩn theo loại và theo trạng thái.
- **Acceptance (suite `PRV-xx`, claude giả `--ai-stub`, repo LedgerLite, loại A)**: xem đề xuất native; chỉnh hai lần → Approval mới và Approval cũ bị huỷ; approver member chỉnh được; người lạ bị chặn; phản hồi quá dài bị từ chối; lượt thất bại để Request thao tác được và "Try again" giữ phản hồi; bấm đúp và hai người chỉnh; hotfix bị từ chối; bộ tài liệu cắm bí mật được che (dòng seed SQL như `ARV-07` cho kiểm che khi đọc); placeholder file bị lược (dòng seed). Ca OpenSpec chạy thật cần `fake claude` hỗ trợ tác giả OpenSpec hoặc dữ liệu seed (task 09 quyết định theo khả thi); ca **loại C** (Claude thật, OpenSpec, pilot hoặc fixture tài chính, **7 đến 13 USD mỗi lượt sinh, mỗi vòng chỉnh thêm một lượt cùng bậc**, chưa đo riêng) chỉ chạy khi người dùng đồng ý, đặt `ORCA_E2E_OPENSPEC=1`.
- **UI live** (`run_ui_flow.py --ai-stub -- plan-documents` và `-- plan-revision`, LedgerLite, claude giả): xem Documents/Proposal, chỉnh hai lần bằng giao diện, bản cũ chỉ đọc, "Try again", Activity. `--real` (loại B: native, repo tạm, khoảng 3 phút, chi phí nhỏ) chỉ cho một vòng chỉnh `bug` native; không sửa checkout trong lúc `--real` chạy (quy tắc "quiet-checkout" của `run_ui_flow.py`).

## 6. Rủi ro và điểm chưa kiểm chứng

- **R1: phụ thuộc CR-REQ-044 (đã đọc v1.0 ngày 2026-10-20, chưa triển khai).** Điểm nối: `plan_auto_jobs`, `PlanAutoJobRepository`, `GeneratePlan.ExecuteForSystem`, `CommitPlan.ExecuteForSystem`, bước `committing` (044 mục 2.6, 2.14), `usePlanRunState` và `PlanRunFailedCard` ở phía giao diện. Nếu 044 đổi tên hay chữ ký thì task 02 (job `revision`) và FE task 05 khớp lại. Không phát hành CR này trước 044. Khi 044 bị tắt bằng công tắc env, CR này vẫn đúng nhưng hẹp hơn (E12).
- **R2: bảng lịch sử và dung lượng.** Không giới hạn số lần chỉnh nên bảng lớn lên mãi; giới hạn bằng lược chữ nhưng chỉ mục và digest giữ mãi (vài trăm byte mỗi file). Cần kiểm kích thước thực với bộ tài liệu thật của pilot (chưa đo).
- **R3: danh tính lượt Plan.** `reporterIdentity` loại `RunKindPlan` có chủ ý (comment trong mã) nhưng tôi không tìm được lý do; đổi nó cần kiểm mọi lời gọi của worker Plan (project-service, git-gateway, task-service) với danh tính reporter. Chưa kiểm chứng; task 02 bắt đầu bằng việc đọc `git log -S` và test.
- **R4: `SupersedesPlanID` và cây task cũ.** Commit lần sau thay Plan cũ trong task-service; hành vi khi task cũ đã bắt đầu chạy (`ErrPlanHasRunningTasks`) không liên quan vì chỉnh chỉ ở `awaiting_plan_approval`, nhưng tôi chưa đọc phía task-service. Chưa kiểm chứng.
- **R5: fake claude không tác giả OpenSpec.** `fake_claude_cli/claude` chỉ nhận ra prompt phân loại, Solution, Plan native và phân tích chỉ đọc; prompt OpenSpec bị "fail loudly". Engine OpenSpec còn cần `openspec` trên dev server, kỹ năng tri thức và `.tdd-agent/config.toml` (`KNOWLEDGE_HUB_UNSUPPORTED` với `--ai-stub`, ghi chú e2e). Vì vậy kiểm xem tài liệu/che bí mật/placeholder bằng dữ liệu seed và test hợp đồng; kiểm lượt chỉnh OpenSpec thật chỉ ở loại C.
- **R6: prompt "sửa tại chỗ" là chỉ dẫn, không phải bảo đảm.** Agent vẫn có thể viết lại nhiều; kiểm chứng chất lượng cần Claude thật (loại C, opt-in). Digest và kiểm phạm vi vẫn bảo vệ tính đúng.
- **R7: nhánh git giữ văn bản thô** (mục 2.6); người có quyền trên dev server vẫn đọc được.
- **R8: chi phí.** Mỗi vòng chỉnh OpenSpec là một lượt agent; không giới hạn số vòng và chưa có ngân sách AI theo tenant (CR-REQ-034, `AiBudgetAdminService` trả `Unimplemented`). Giao diện hiện chi phí của lượt nếu agent báo (`stats`).
- **R9: số migration và trường proto** (`0101` đã được CR-REQ-044 giữ; CR này dùng `0102_plan_auto_jobs_revision_origin`, `0103_openspec_change_redaction`, `0104_openspec_change_revisions`; `GeneratePlanRequest.idempotency_key = 3`, `GetPlanProposalRequest.revision = 3`, `GetPlanBundleFileRequest.revision = 3`) có thể bị CR khác giữ khi triển khai; kiểm lại. Migration mới nhất tôi thấy: `0100_analysis_runs_revision_index`.
- **R10: đã suy ra từ mã, chưa chạy:** G1 (nút Regenerate và tuỳ chọn regenerate-khi-từ-chối lỗi trạng thái), G4, G5, G11, G12. Task đầu của mỗi nhóm viết test đỏ trước khi sửa.
- Chưa chạy hệ thống; mọi mô tả dựa trên đọc mã ngày 2026-10-20.

## 7. Quyết định đã chốt và câu hỏi còn mở

**Đã chốt (người yêu cầu, 2026-10-20):**

1. Phạm vi: (1) xem chỉ đọc tài liệu OpenSpec trong tab Plan (tab theo tài liệu, tải từng file, file bị lược có placeholder kèm kích thước, hiển thị kiểm tra và khoảng trống tri thức, chỉ engine OpenSpec, Markdown qua `RequestMarkdownContent`); (2) chỉnh bằng prompt tại cổng Plan, cùng UX CR-REQ-041 (ô nhập, không giới hạn số lần, phiên bản, Approval mới sau mỗi vòng, approver được chỉnh với `AuthorizeRevise` + `ApproverAwareAuthorizer`, dev server tra như reporter), áp dụng cho native và OpenSpec (native: xem JSON đề xuất dạng cấu trúc chỉ đọc, cùng ô chỉnh); (3) gia cố: làm sạch bí mật khi lưu, dặn prompt sửa tại chỗ khi có phản hồi (kèm quy trình golden và phối hợp CR-REQ-046), không thêm công cụ MCP.
2. Người dùng tự sửa tay tài liệu: ngoài phạm vi, lý do ở mục 2.10.

**Quyết định của tác giả CR (trong khung đã chốt, có thể đảo):** bảng lịch sử với giữ chữ 10 bản (E5), so sánh mức file (E6), điều kiện prompt theo "có phiên bản trước" (E8), `NO_CHANGE` là lỗi (E10), `hotfix` loại khỏi đường chỉnh.

**Còn mở:**

1. Sau khi lượt chỉnh thất bại, Approval cũ đã huỷ và Plan cũ vẫn còn trong task-service: có cần hành động "Keep the previous version" (mở lại Approval cho Plan hiện có, không gọi AI) không? Mặc định tạm: không; người dùng thử lại hoặc chỉnh lại. Làm thì thêm một RPC và đường chuyển trạng thái `planning` → `awaiting_plan_approval` không qua `CommitPlan`.
2. Lượt chỉnh không đổi gì: chấp nhận làm lỗi (E10, mặc định) hay tạo phiên bản trùng kèm ghi chú "No document changed"?

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-012 | `GeneratePlan` nhận `idempotency_key`, kế thừa phản hồi, từ chối chỉnh lần hai và `hotfix`; quyền chỉnh của approver |
| CR-REQ-026 | `openspec_changes` thêm `redacted_secrets`; bảng `openspec_change_revisions`; `finish` ghi bản trong cùng transaction; `restoreDrift` không khôi phục từ bản đã che |
| CR-REQ-016 | `request.planRevisions` (mới); `request.generatePlan` nhận `idempotencyKey`; `request.planProposal` và `request.planBundleFile` nhận `revision` |
| CR-REQ-017 | `excluded_channels.yaml` thêm `request.planRevisions`; cập nhật lý do hai dòng hiện có |
| CR-REQ-021 | Tab Plan thêm chế độ xem tài liệu, panel chỉnh, danh sách phiên bản; bỏ "Regenerate plan" ở trạng thái chờ duyệt |
| CR-REQ-037 | Activity có `plan.revision_requested`; nhánh `analysis.revision_requested` lọc theo loại |
| CR-REQ-041 | Dùng lại mẫu; tách `RevisionPromptBox` khỏi `AnalysisRevisionPanel` (không đổi hành vi) |
| CR-REQ-042 | Hai partial prompt mới, golden mới, ngân sách kích thước |
| CR-REQ-044 | Phụ thuộc: dùng lại `plan_auto_jobs`/worker/`CommitPlan.ExecuteForSystem` cho lượt chỉnh; thêm `origin = 'revision'`; `usePlanRunState` và `PlanRunFailedCard` dùng cho cả thay Plan; 044 mục 2.14 đã ghi nhận điểm nối này |
| CR-REQ-046 | Phối hợp: cả hai thêm khối có điều kiện vào cùng `plan.tmpl` |
| CR-REQ-010 | `AuthorizePlanRevise` dùng `CanDecide`; ghi vào mô tả quyền |
| CR-REQ-035 | Phản hồi tuân C9; quyền đọc phản hồi bằng quyền đọc Request; OPA nhóm `plan` thêm `member_plan_steering_rpcs` |

## 9. Tham chiếu

- `backend-go/services/request-service/internal/usecase/generate_plan.go` (`Execute`, `checkPlanState`), `plan_ports.go` (`PlanActorAuthorizer`), `solution_authorization.go` (`ApproverAwareAuthorizer`, `orPendingApprover`), `analysis_runner.go` (`reporterIdentity`), `run_proposal_generation.go` (`finish`, `commitAndRead`, `omittedMarker`), `engine_openspec.go` (`prepare`, `runAgent`), `get_plan_proposal.go`, `get_plan_bundle_file.go`, `plan_subject_artifacts.go`, `tasks_md_sync.go` (`restoreDrift`), `analysis_secret_redaction.go`, `plan_document.go`, `commit_plan.go`, `solution_revision_recovery.go` (mẫu `inheritedFeedback`, `refuseSecondRevision`)
- `backend-go/services/request-service/internal/domain/openspec_change.go`, `plan_errors.go`, `request_activity.go`, `request_transition.go`, `rpc_catalog.go`, `proposal_cleanup.go`; `internal/adapter/{postgres,mysql}/openspec_change_repository.go`, `request_activity.go`; `migrations/{postgres,mysql}/0007`, `0094`, `0100`
- `backend-go/proto/orca/request/v1/plan.proto`; `backend-go/policy/orca-authz/request.rego`; `backend-go/prompts/{plan_openspec,plan_native,request_context}`, `prompts/README.md`, `backend-go/ci/check-prompt-golden.sh`
- `backend-go/services/api-gateway/internal/adapter/{wscompat/channels_request_plan.go,httpgateway/request_routes_flow.go,mcpserver/tools/excluded_channels.yaml}`; `backend-go/cmd/orca-cli/internal/command/plan_cmd.go`
- `frontend/src/renderer/src/components/request/plan/{RequestPlanTab,PlanApprovalBar,PlanDecisionButtons,PlanSummaryHeader,PlanStates,plan-view-state}`, `hooks/{usePlanDecision,request-plan-generation,usePlanRunInFlight}.ts`, `components/request/solution/{RequestMarkdownContent,AnalysisRevisionPanel,SolutionVersionSwitcher,OlderVersionBanner}.tsx`
- `tests/stages/e2e/orca-flows/{fake_claude_cli/claude,suite_analysis_revision.py,run_acceptance.py,run_ui_flow.py}`, fixture `ledgerlite-finance`
- [CR-REQ-012](./CR-REQ-012-plan-phase-task-generation-from-solution.md), [CR-REQ-039](./CR-REQ-039-request-worktree-base-branch.md), [CR-REQ-041](../solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md), [CR-REQ-042](../ai-governance/CR-REQ-042-prompt-catalog-shared-agent-relay-and-stdin.md), [CR-REQ-021](../request-frontend/CR-REQ-021-plan-phase-tree-and-approval-ui.md), [CR-REQ-037](../request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md)
- Mới: `internal/domain/openspec_change_revision.go`, `internal/usecase/list_plan_revisions.go`, `internal/usecase/plan_revision_authorization.go`, migrations `0102_plan_auto_jobs_revision_origin`, `0103_openspec_change_redaction`, `0104_openspec_change_revisions`, `components/request/plan/documents/`, `components/request/plan/revision/`
