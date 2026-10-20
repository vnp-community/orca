# CR-REQ-043 — "Approve and run all steps": chọn chạy liên tục ngay lúc duyệt Plan, công tắc theo dự án và hiển thị chế độ của Request

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-043 |
| **Tên** | `approval.approve` nhận tham số tuỳ chọn `executionMode` (chỉ `continuous` hoặc rỗng) khi duyệt Plan, task list hoặc `pre_deploy` ở `awaiting_plan_approval`; request-service ghim chế độ trong cùng transaction với quyết định, ghi vào `approval.decided` để audit. Giao diện: nút phụ "Approve and run all steps" kèm hộp xác nhận ở `PlanApprovalBar` (và cam kết rõ ở `PlanApprovalCard`), công tắc chế độ thực thi trong Project Settings, nhãn chế độ trên tab Execution |
| **Loại** | Feature (sửa một điểm yếu trải nghiệm: phải duyệt và bấm từng Phase) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | ✅ Đã triển khai (2026-10-20), chưa merge; e2e stub xanh, chưa chạy thật hạng C |
| **Phụ thuộc** | CR-REQ-038 (chế độ `continuous`, ghim `execution_mode`, `ExecutionStartPrecheck`), CR-REQ-009 và 010 (Approval và quyền duyệt), CR-REQ-013 (`StartPhase`, `EvaluateExecution`), CR-REQ-016 (kênh gateway, `request.flowStatus.features`), CR-REQ-021 và 037 (thanh duyệt Plan, tab Execution), CR-REQ-035 (audit) |
| **Mở khoá** | Không. Ghi nhận hai CR con có thể tách sau: đổi chế độ cho Request đang chạy (mục 2.10) và Pause/Resume (mục 2.11) |
| **Tác động** | `backend-go/proto/orca/request/v1/approval.proto`, `backend-go/services/request-service` (`usecase/decide_approval.go`, `approval_execution_precheck.go`, `audit_event_tap.go`, `domain/execution_mode.go`, `domain/approval_events.go`, `adapter/grpc/approval_server.go`, `cmd/server/wire_approval.go`), `backend-go/services/api-gateway` (`wscompat/channels_approval.go`, `channels_request_base_ref.go` hoặc file tương đương cho `requestFeatures()`, `httpgateway/request_routes.go`), `backend-go/cmd/orca-cli` (cờ của `orca approval approve`), `frontend/src/renderer/src/components/request/{plan,execution}`, `components/project`, `hooks/usePlanDecision.ts`, `shared/request-*.ts`, 5 file locale, `tests/stages/e2e/orca-flows` |

> Nguồn: phản hồi của người dùng ngày 2026-10-20: "Phần Execution phải cho bấm 'Approve all' để chạy hết các task, không duyệt từng bước; trải nghiệm hiện tại rất kém". Phần hiện trạng dưới đây được đọc lại từ mã ngày 2026-10-20 (số dòng là của HEAD `26c5dabf8`); chưa chạy lại hệ thống để quan sát, ngoài dữ liệu Request #1771 do bước khảo sát trước ghi lại (xem 1.1, không tự kiểm lại được).

---

## 1. Bối cảnh và vấn đề

### 1.1 Hiện trạng

Một chế độ "duyệt một lần, chạy hết" **đã có**: `continuous` (CR-REQ-038). Điều thiếu là người dùng không có đường nào đi tới nó từ giao diện.

| # | Sự thật | Chứng cứ (đã đọc ngày 2026-10-20) |
|---|---|---|
| 1 | Mặc định là `step_by_step` | `domain/execution_mode.go:47-55` (`EffectiveExecutionModeOf`: ghim, rồi cài đặt dự án, rồi mặc định triển khai); `deploy/dev/docker-compose.yml:473` (`REQUEST_DEFAULT_EXECUTION_MODE: ${...:-step_by_step}`); `deploy/dev/.env.example:228` |
| 2 | Chỉ có hai cách đặt chế độ: biến môi trường và kênh WebSocket dành cho admin | `api-gateway/internal/adapter/wscompat/channels_request_engine.go:83` (`request.engineSettingsSet`, chặn `id.Role != roleAdmin`); `deploy/dev/RELEASE-2026-10-DEPLOY.md` mục 4.3 (dòng 103-109) và dòng 207: "Chưa có màn hình cài đặt engine và chế độ thực thi". `frontend/src` không có chỗ nào gọi `request.engineSettings*` (grep `engineSettings`: chỉ có helper test `tests/.../live-continuous-execution-steps.ts` `configureProject`) |
| 3 | Chế độ được ghim trên Request khi nó vào `executing`, không đổi nữa | `usecase/start_execution.go:38-44` gọi `PinExecutionMode`; `adapter/postgres/request_repository.go:171-193` và `adapter/mysql/request_repository.go:164` (`PinExecutionModeIfUnset`: chỉ ghi khi cột còn `NULL`). Không có mã nào xoá hay sửa cột sau đó (grep `execution_mode` trong `usecase`, `adapter`) |
| 4 | Ở `step_by_step` mỗi Phase của `change_request` cần hai lần bấm | `usecase/evaluate_execution.go:249-290` (`completePhases` mở một Approval `phase` cho Phase sẵn sàng kế tiếp); `usecase/start_phase.go:92-93` (`StartPhase` trả `ErrPhaseNotApproved` khi chưa duyệt). Giao diện: `plan/PhaseApprovalBar.tsx:61` ("Approve phase") rồi `:82` ("Start phase"); thẻ "Approve step: X" ở `execution/ExecutionNeedsYou.tsx:72` |
| 5 | Phase xuất hiện ở những luồng nào | `domain/request_flow_registry.go:64` (`change_request`: `PhaseAlways`, `ExecutionGates: [GatePhase]`), `:65` và `:70` (`bug`, `refactor`: `PhaseWhenSizeL`, không có cổng Phase nên người bấm "Start phase"), `:74` (`ops_request`: cổng `pre_deploy` trong `executing`); `:66` và `:71` (`hotfix`, `security`: `StartGate = pre_deploy`). Các luồng còn lại không có Phase: duyệt Plan xong là tự chạy (`start_execution.go:45-50`) |
| 6 | `continuous` bỏ cổng Phase và tự bắt đầu Phase kế tiếp | `domain/execution_mode.go:59-69` (`EffectiveExecutionGates` bỏ `GatePhase`); `usecase/continuous_phases.go:16-52` (`startNextPhase`, người thực thi là `ActorFor(ctx, req, "")`); `usecase/execution_actors.go:35-73` (`pick`: Approval đã duyệt gần nhất không phải `phase`, tức người duyệt Plan) |
| 7 | `continuous` vẫn dừng ở bốn chỗ | Clarification (`awaiting_information`), task hết lượt thử (Request về `request_backlog`), duyệt lệch kế hoạch (`continuous_phases.go:27-33`: Approval `phase` đang chờ giữ run; `get_execution_summary.go:190`: `Stage == "drift_review"`), và `pre_deploy`. Hết quyền thực thi giữa chừng: `continuous_phases.go:56-69` đưa Request về backlog |
| 8 | Kiểm quyền thực thi ngay lúc duyệt Plan | `usecase/approval_execution_precheck.go:21-53` (`ExecutionStartPrecheck.Check`: chỉ chạy cho `plan`, `task_list`, `pre_deploy` ở `awaiting_plan_approval`, và chỉ khi chế độ phân giải là `continuous`); gọi từ `usecase/decide_approval.go:75-79`; lỗi `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` ở `domain/execution_errors.go:33` |
| 9 | Quyết định duyệt cần phiên bản và digest | `usecase/decide_approval.go:45-170`: khoá Request rồi khoá Approval (`:84-88`), `CanDecide` (`:111`), `ExpectedVersion` (`:114`), `ExpectedDigest` (`:117`), trạng thái Request phải bằng `a.Stage` (`:120`); gọi lại cùng quyết định thì trả kết quả cũ, không sinh sự kiện thứ hai (`:93-96`, `sameDecision` `:177`). Ai được duyệt: `domain/approval_authorization.go:9-70` (agent bị từ chối; admin hoặc người được chỉ định; tự duyệt bị từ chối nếu không bật `SelfApprovalAllowed`) |
| 10 | Giao diện không có thao tác hàng loạt | `plan/RequestPlanTab.tsx:148` quyết định `continuous` từ `request.executionMode`, `:181-196` chọn `PlanApprovalCard` (một nút "Approve and start execution", `PlanApprovalCard.tsx:94`) hay `PlanApprovalBar`; `hooks/usePlanDecision.ts:85-111` duyệt từng Approval; grep `approveAll`, `ApproveMany`, `BulkApprove`, `DecideMany` trong `frontend/src` và `backend-go`: không có |
| 11 | `Request.execution_mode` trả về là chế độ *sẽ dùng* khi chưa ghim | `usecase/request_engine_resolver.go:20-50` (`Annotate` điền `ExecutionMode` cho Request chưa ghim bằng mặc định hiệu lực); proto `request.proto:56` |
| 12 | Quan sát trên hệ thống chạy | Request #1771 của dự án `vnp-qa-platform` đang `executing`, `execution_mode=step_by_step`, có một Approval `phase` đang chờ và chưa task nào chạy. **Không tự kiểm lại được trong lần soạn này** (chưa mở DB hay UI); ghi lại theo khảo sát trước |

### 1.2 Vấn đề

1. Người dùng chỉ muốn "duyệt rồi để máy chạy hết", nhưng muốn vậy phải nhờ admin đặt cả dự án sang `continuous` bằng một lệnh WebSocket, không có màn hình, và quyết định đó áp cho **mọi** Request mới của dự án, không phải riêng cái họ đang duyệt.
2. Với mặc định `step_by_step`, một `change_request` 7 Phase bắt người dùng bấm 14 lần ("Approve phase", "Start phase") và quay lại màn hình sau mỗi Phase.
3. Người dùng không thấy Request của mình đang chạy theo chế độ nào, nên không biết vì sao cứ phải duyệt tiếp.

### 1.3 Điều CR này **không** sửa

Request đã vào `executing` ở `step_by_step` (như #1771) vẫn chạy từng bước sau CR này. Lý do và đề xuất ở mục 2.10.

## 2. Giải pháp đề xuất

Theo phương án (b) mà người yêu cầu đã chọn: thêm **lựa chọn chạy hết ngay lúc duyệt Plan**, bên cạnh công tắc theo dự án đã có. Không tạo thao tác hàng loạt và không thêm bước duyệt hộ.

### 2.1 Tổng quan luồng

```
Plan chờ duyệt (awaiting_plan_approval)
  ├─ [Approve plan]                       → như hôm nay: chế độ = cài đặt dự án / mặc định triển khai
  └─ [Approve and run all steps] + xác nhận
        approval.approve {id, expectedVersion, expectedDigest, executionMode:"continuous"}
          trong MỘT transaction của DecideApproval:
            khoá Request, khoá Approval, CanDecide, version, digest, stage        (không đổi)
            + chế độ chỉ hợp lệ cho plan | task_list | pre_deploy ở awaiting_plan_approval
            + PinExecutionModeIfUnset(continuous); đã ghim chế độ khác thì từ chối cả quyết định
            subject handler (đưa Request sang executing)                          (không đổi)
            outbox approval.decided {..., execution_mode:"continuous"}
        trước transaction: ExecutionStartPrecheck với chế độ yêu cầu, người duyệt thiếu quyền execute bị từ chối ngay
  → StartExecution thấy Request đã ghim continuous, tự bắt đầu Phase đầu bằng quyền của người duyệt (continuous_phases.go)
```

### 2.2 Tham số `executionMode` của `approval.approve`

- Proto `ApproveRequest.execution_mode = 5` (chuỗi). Rỗng = hành vi hiện tại. Giá trị duy nhất khác rỗng được nhận: `continuous`. `step_by_step` tường minh **không** được nhận (người yêu cầu chỉ chọn "continuous hoặc rỗng"; xem câu hỏi 4).
- Chỉ cho `approve`, không cho `reject`: kèm chế độ khi từ chối là `REQUEST_EXECUTION_MODE_INVALID`.
- Chỉ hợp lệ khi Approval có `subject_type` ∈ {`plan`, `task_list`, `pre_deploy`} **và** `stage = awaiting_plan_approval` (cùng tập điều kiện của `ExecutionStartPrecheck`, `approval_execution_precheck.go:22-29`). Ngoài tập đó: `REQUEST_EXECUTION_MODE_NOT_APPLICABLE`. Không kiểm "luồng có Phase hay không" ở backend (xem D7): với luồng không có Phase, chế độ được ghim và không có tác dụng nào khác ngoài kiểm quyền thực thi; giao diện chỉ mời lựa chọn khi luồng có Phase.
- `ApproveResponse.execution_mode = 3` trả chế độ hiện ghim trên Request sau quyết định (rỗng nếu chưa ghim). Gọi lại một quyết định đã xong (nhấp đúp) trả chế độ đang ghim chứ không đọc lại tham số, nên giao diện luôn thấy sự thật.

### 2.3 Ghim trong cùng transaction

Trong `DecideApproval.Execute`, sau khi mọi kiểm tra (quyền, phiên bản, digest, stage) đã qua và **trước** `Approval.Approve` / handler:

1. `got := Pinner.PinExecutionModeIfUnset(ctx, req.ID, continuous)` (cùng repository đã ghim cho `StartExecution`; `scoped()` dùng transaction trong `ctx`, đã đọc `postgres/tx.go:82` và `mysql/tx.go:75`, nên ghim cuộn lại nếu handler lỗi).
2. `got != continuous` (Request đã ghim `step_by_step` ở một lần chạy trước rồi được mở lại): trả `REQUEST_EXECUTION_MODE_ALREADY_PINNED` (`FailedPrecondition`), **không ghi quyết định**. Không im lặng bỏ lựa chọn, vì người dùng đã xác nhận một hộp thoại nói rằng Orca sẽ chạy hết.
3. Handler Plan đưa Request sang `executing` như cũ; sự kiện `status_changed` nằm trong cùng transaction, nên consumer `StartExecution` (`eventbus/request_status_consumer.go:46-58`) luôn thấy Request đã ghim.

Hệ quả cho Request có `executionMode` rỗng: mặc định hiệu lực (cài đặt dự án, rồi mặc định triển khai) vẫn được ghim lúc `StartExecution` như hiện nay khi người dùng bấm nút duyệt thường.

### 2.4 Kiểm quyền thực thi dựa trên chế độ được yêu cầu

`ExecutionStartPrecheck.Check(ctx, approval, approverID, requested)` nhận thêm chế độ yêu cầu: `requested == continuous` thì chạy `Access.CheckReady` **kể cả khi dự án đang `step_by_step`**; ngược lại dùng chế độ phân giải như cũ. Người duyệt không có quyền execute nhận `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` (đã có, giao diện đã dịch ở `plan-decision-error-message.ts:23`) và quyết định chưa được ghi. Người không có quyền duyệt vẫn nhận lỗi duyệt thường, không phải gợi ý về quyền task (`approval_execution_precheck.go:41-46`). Không thêm đường phân quyền mới: dùng `Authorizer.CanDecide` và `ExecutionAccess` có sẵn.

### 2.5 Sự kiện và audit

- `ApprovalDecidedPayload` thêm `execution_mode` (`omitempty`): chế độ mà người duyệt **đã chọn** trong lời gọi (`continuous`), rỗng khi không chọn. Không điền chế độ mặc định phân giải, vì mục đích là ghi lại lựa chọn của con người.
- `AuditTap` (`usecase/audit_event_tap.go:25-42,111-129`) đọc thêm khoá `execution_mode` và đặt vào `Metadata["execution_mode"]` của bản ghi `approval.approve`. Không ghi nội dung Request.
- Cộng thêm: payload là phần bổ sung, người tiêu thụ hiện có (notification-service, metrics, gateway stream) không đọc khoá lạ; chưa kiểm notification-service có giải mã nghiêm ngặt không (R9).
- **Giới hạn đã kiểm**: khung `request.event` của gateway chỉ chuyển danh sách khoá cho phép (`channels_request_stream.go:25-52`) và `admin.queryAuditLog` chỉ trả `Target`, không trả metadata (`channels_admin_audit.go:20-37`). Vì vậy **acceptance qua WebSocket không đọc được `execution_mode` trong payload**; điều đó được kiểm ở contract test (đọc hàng outbox `orca.request.approval.decided`, mẫu ở `contracttest/approval_flow_contract.go:217`). Lịch sử (History) dựng từ bảng `approvals`, không từ outbox (`adapter/postgres/request_activity.go:52`), nên không hiện lựa chọn; History đã hiện từng "Phase started automatically" (`phase_starts.start_trigger=auto`).

### 2.6 Gateway, REST, CLI và cờ tính năng

| Bề mặt | Thay đổi |
|---|---|
| WS `approval.approve` | nhận `executionMode?`; chuyển nguyên xuống `ApproveRequest.execution_mode` (không kiểm giá trị ở gateway, tránh hai nơi giữ danh sách). `approval.reject` từ chối khoá này ngay ở gateway bằng `REQUEST_EXECUTION_MODE_INVALID: executionMode applies to approve only` (proto `RejectRequest` không có trường nên không thể để nó lặng lẽ bị bỏ) |
| Kết quả `approval.approve` | thêm `executionMode` (chuỗi, bỏ khi rỗng) |
| REST `POST /v1/approvals/{id}/approve` | body thêm `executionMode` (`request_routes.go:130-146`); route `reject` chuyển trường này xuống kênh nên bị từ chối như trên |
| `request.flowStatus` | `requestFeatures()` thêm `approval.runAll` (mẫu `request.baseRef`, `channels_request_base_ref.go:16-19`). Cần vì gateway cũ **bỏ qua tham số lạ**: duyệt "chạy hết" trên gateway cũ sẽ lặng lẽ thành duyệt thường |
| MCP | không đổi: `approval.approve` đã nằm trong `excluded_channels.yaml:601` ("agent không được duyệt"); không thêm ToolSpec. `parity_test` phải vẫn xanh |
| CLI | `orca approval approve <id> --execution-mode continuous` (`cmd/orca-cli/internal/command/approval_cmd.go`, `apiclient.DecideApproval` truyền thêm trường trong body) |

### 2.7 Giao diện duyệt Plan

- `PlanApprovalBar` (`plan/PlanApprovalBar.tsx`) hiện nút phụ **"Approve and run all steps"** cạnh "Approve plan", khi **tất cả** đúng: Approval đang `pending` và là `plan` hoặc `task_list`; luồng của Request có Phase ở cỡ hiện tại (`RequestFlowView.hasPhases`, cùng nguồn với `RequestPlanTab.tsx:186`); `request.executionMode !== 'continuous'` (nếu đã `continuous`, nút chính vốn đã chạy hết); gateway công bố `approval.runAll`; người dùng chưa bị `hideApprove` do quy tắc rủi ro. Với `change_request` và `bug`/`refactor` ở cỡ L: đúng tập luồng có Phase hôm nay.
- Bấm nút phụ mở hộp thoại xác nhận (`dialog.tsx`; không có `alert-dialog` trong `components/ui`). Văn bản đề xuất, bản gốc của người yêu cầu thêm một vế vì có thật trong `PlanApprovalCard`/CR-038 (Clarification cũng dừng run): *"Run all {{n}} steps without asking again? Orca starts each step as soon as the previous one finishes, using your access. It stops only when the agent needs information from you, a task keeps failing (the request then goes back to the backlog), a change drifts from the plan and needs a review{{preDeploy}}. You can stop the run at any time."* với `{{preDeploy}}` = ", or it reaches the pre-deploy approval" khi luồng có cổng đó. Nút xác nhận "Run all steps"; nút huỷ "Not now".
- Xác nhận gọi lại đúng đường `decision.approve` (cùng khoá bận `approve:<id>`, nên nhấp đúp không gửi hai lần; bản thân backend cũng idempotent) kèm `executionMode: 'continuous'` và các trường rủi ro hiện có (`approveExtras`). Sau thành công đối chiếu `executionMode` trả về; khác `continuous` thì báo "This server didn't run the plan on its own." thay vì im lặng (phòng thủ chiều sâu, ngoài cờ tính năng).
- `PlanApprovalCard` (dự án/Request đã `continuous`): nút chính "Approve and start execution" **gửi `executionMode: 'continuous'` tường minh**. Lý do: thẻ cam kết "Approve once and the agent runs all"; nếu admin đổi cài đặt dự án giữa lúc render và lúc bấm, cam kết vẫn đúng. Không thêm nút phụ vì nút chính đã là hành động đó. Đây là điều chỉnh nhỏ so với câu chữ "thêm nút phụ ở cả hai nơi" trong yêu cầu; xem D6.
- Mã lỗi mới trong `plan-decision-error-message.ts`: `REQUEST_EXECUTION_MODE_ALREADY_PINNED` → "This request already runs step by step because it was started that way before. Approve it normally."; hiện thông điệp rồi ẩn nút phụ cho lần render này. `REQUEST_EXECUTION_MODE_NOT_APPLICABLE` và `REQUEST_EXECUTION_MODE_INVALID` là lỗi lập trình ở giao diện: thông điệp chung, ghi log.
- Hộp thoại Approval Inbox (`useApprovalInbox.ts:168`) **không đổi** (xem D9).

### 2.8 Hiển thị chế độ trên tab Execution

`ExecutionSummaryStrip` thêm nhãn nhỏ cạnh trạng thái chạy, đọc `summary.executionMode` (`activity.proto:109`, rỗng trước khi vào `executing`): "Runs all steps on its own" khi `continuous`; "Asks before each step" khi `step_by_step` **và** luồng có Phase; không hiện gì khi rỗng, hoặc `step_by_step` mà luồng không có Phase. Tooltip nói ai là người thực thi ("Steps run with the access of {{approver}}") khi biết; nếu `summary` chưa có tên người duyệt thì chỉ nói "the person who approved the plan" (không thêm trường chỉ để lấy tên). Không thêm nút nào ở đây (xem 2.10).

### 2.9 Công tắc chế độ theo dự án

- Mục mới "Request execution" trong tab General của `ProjectSettings` (cạnh `ProjectJiraMappingSection`): một công tắc/ô chọn hai giá trị "Ask before each step" (`step_by_step`) và "Run all steps after one approval" (`continuous`), kèm một dòng nói hiệu lực: áp cho Request **mới vào `executing`** sau đó, không đổi Request đã ghim (đúng ghi chú `RELEASE-2026-10-DEPLOY.md:117`).
- Gọi `request.engineSettings` để đọc rồi `request.engineSettingsSet` với **toàn bộ** giá trị hiện có (engine, `openspecMinVersion`, `solutionAgent`) và `executionMode` mới, kèm `expectedVersion`. Lý do phải gửi lại cả bộ: `SetProjectEngineSettings` coi `engine` rỗng là `native` (`manage_project_engine_settings.go:93-95`) và `solution_agent` là một bool trần; chỉ riêng `executionMode` rỗng mới "giữ nguyên" (`:110-118`). Không thêm RPC hẹp trong CR này (D8). Xung đột phiên bản: tải lại và hỏi.
- Chỉ admin toàn hệ thống đổi được (backend chặn `id.Role != roleAdmin`, `channels_request_engine.go:83-86`, và `callerIsAdmin` trong usecase). Người khác thấy giá trị hiện tại ở dạng chỉ đọc kèm "Only an administrator can change this." Việc cho chủ dự án đổi là câu hỏi mở 6.
- Ghi chú hệ quả: lưu cài đặt ghi cả engine hiện hành vào dòng cài đặt dự án, nên mặc định triển khai (`REQUEST_DEFAULT_ENGINE`) không còn tự áp cho dự án đó (R5).

### 2.10 Request đang `executing` ở `step_by_step`: ngoài phạm vi, tách thành CR riêng (đề xuất)

Người yêu cầu để mở: hoặc thêm "Switch this request to run all remaining steps" (RPC mới có khoá Request và audit), hoặc ghi rõ ngoài phạm vi. **Đề xuất: ngoài phạm vi CR này, làm ở CR riêng**, vì:

1. **Người thực thi bị lệch.** `continuous` chạy bằng quyền của người duyệt Plan gần nhất (`execution_actors.go:35-73`). Người bấm "chuyển" có thể không phải người đó; muốn "dùng quyền của bạn" thì phải thêm khái niệm người thực thi mới hoặc ép người bấm phải là người duyệt Plan. Đây là thiết kế riêng, không phải một cờ.
2. **Approval `phase` đang chờ.** `startNextPhase` dừng nếu còn bất kỳ Approval `phase` nào `pending` (`continuous_phases.go:27-33`). Muốn chuyển phải huỷ Approval cổng nhưng **giữ** Approval duyệt lệch (`Stage == "drift_review"`), và chạy `OnClosedWithoutDecision` của handler Phase; chưa đọc đủ để chắc không có tác dụng phụ.
3. **Cổng Phase là kiểm soát được cấu hình** cho `change_request`. Tháo nó sau khi Plan đã duyệt cần quyền ngang người có thể duyệt Phase, kiểm bằng `CanDecide` trên chính Approval đó; cần quyết định về phân quyền và audit riêng.
4. **Task đang chạy dở** khi chuyển và các trường hợp đua với `EvaluateExecution`/reconcile cần test riêng.
5. CR này đã đủ lớn; lựa chọn lúc duyệt (2.2) giải quyết các Request **mới** và an toàn hơn vì ghim nằm cùng transaction với quyết định.

Hệ quả phải nói thẳng: **Request #1771 và mọi Request đã ghim `step_by_step` vẫn phải duyệt từng Phase**. Mở lại Request (backlog rồi duyệt lại) không giúp: cột ghim không bị xoá, nên duyệt với "chạy hết" trả `REQUEST_EXECUTION_MODE_ALREADY_PINNED`. Nếu người yêu cầu coi #1771 là bắt buộc, CR riêng cần lên trước; đó là câu hỏi mở 1.

### 2.11 Pause: ngoài phạm vi, giữ nguyên "Stop run"

Hôm nay không có điều khiển tạm dừng, chỉ "Stop run" (huỷ Request): `ExecutionSummaryStrip.tsx` và CR-REQ-037 F12, CR-REQ-038 B9 (cờ `execution_paused` được để dành cho "CR sau"). Chạy liên tục làm khoảng trống này rõ hơn, vì người dùng giao việc rồi đi. **Đề xuất: không đưa Pause vào CR này; mở CR riêng** (cờ `execution_paused`, `AdvanceExecution` không dispatch task mới khi bật, RPC `Pause`/`Resume`, nút ở dải tóm tắt). Biện pháp giảm thiểu trong CR này: hộp thoại xác nhận nói rõ "You can stop the run at any time" và dừng bằng "Stop run", và các điều kiện dừng tự động ở mục 1.1 dòng 7.

### 2.12 Phase thêm sau khi duyệt

Lựa chọn "chạy hết" áp cho Plan **đúng như đã duyệt** (digest trong Approval). Nếu Plan đổi sau khi duyệt (thêm Phase, sửa task), điều bảo vệ duy nhất là duyệt lệch kế hoạch (`impact.drift_detected`, CR-REQ-030/036): Phase bị đánh dấu lệch dừng chờ người xem (`get_execution_summary.go:190`). Không có cơ chế riêng nào khác chặn Phase mới được tự bắt đầu; CR này không thêm. Chưa kiểm bằng chạy thật xem một Phase thêm tay vào Plan đã duyệt có luôn sinh duyệt lệch hay không (R3).

### 2.13 Phương án đã cân nhắc và loại

| Phương án | Vì sao loại |
|---|---|
| **"Approve all N approvals" trong một lời gọi** | Approval của các Phase **chưa tồn tại** lúc duyệt Plan: `completePhases` chỉ mở Approval của Phase kế tiếp khi Phase trước xong (`evaluate_execution.go:249-290`). Duyệt trước đòi tạo trước N Approval với digest chưa biết. Lỗi từng phần (một Approval hết hạn hay bị từ chối giữa chừng) để lại trạng thái nửa duyệt. Và phải tái tạo `continuous` bằng cách khác |
| **Duyệt tự động theo chính sách bằng tài khoản hệ thống** | Phá phân tách nhiệm vụ: máy và agent bị từ chối quyết định (`approval_authorization.go:16-18`, `ErrAgentForbidden`), và người tự duyệt bị chặn khi không bật `SelfApprovalAllowed` (`:55-66`). Một "người duyệt hệ thống" sẽ khoét lỗ đúng chỗ kiểm soát này và làm mờ truy vết theo người (CR-REQ-038 B11) |
| **Chỉ làm công tắc theo dự án** | Áp cho mọi Request mới của dự án, chỉ admin đổi được, và không cho chọn riêng từng Request. Được giữ làm bổ sung (2.9), không đủ một mình |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Phương án (b): lựa chọn tuỳ chọn lúc duyệt Plan, kèm công tắc dự án (người yêu cầu chốt 2026-10-20) | Tái dùng `continuous` đã kiểm thử; không thao tác hàng loạt |
| D2 | Tham số `executionMode` chỉ nhận `continuous` hoặc rỗng, chỉ khi duyệt (không phải từ chối) | Đúng quyết định của người yêu cầu; một đường hẹp dễ kiểm |
| D3 | Chỉ cho `plan`/`task_list`/`pre_deploy` ở `awaiting_plan_approval` | Cùng tập với `ExecutionStartPrecheck`; Approval `phase` và các cổng phân tích không có nghĩa "chạy hết" |
| D4 | Ghim bằng `PinExecutionModeIfUnset` trong transaction của `DecideApproval`, trước handler | Lựa chọn và quyết định cùng thành công hoặc cùng cuộn lại; `StartExecution` không còn đoán |
| D5 | Ghim khác `continuous` thì từ chối cả quyết định (`REQUEST_EXECUTION_MODE_ALREADY_PINNED`) | Không bỏ lựa chọn của người đã xác nhận hộp thoại; không đổi pin sẵn có (giữ B2 của CR-038) |
| D6 | `PlanApprovalCard` gửi `continuous` tường minh thay vì thêm nút phụ | Nút chính đã là hành động đó; gửi tường minh khoá cam kết trước đổi cài đặt giữa chừng. Khác chữ yêu cầu ở "thêm nút phụ cả hai nơi" |
| D7 | Backend không kiểm "luồng có Phase"; giao diện ẩn lựa chọn khi không có Phase | Giữ nguyên quy tắc đã chốt; ghim `continuous` ở luồng không Phase vô hại và vẫn cho kiểm quyền thực thi |
| D8 | Công tắc dự án dùng `request.engineSettings`/`engineSettingsSet` hiện có, đọc rồi ghi lại cả bộ | Không thêm RPC; chấp nhận R5 |
| D9 | Approval Inbox không có lựa chọn "chạy hết" | Hàng trong hộp thư không có số bước, rủi ro, luồng để hiển thị hộp thoại đúng; duyệt ở đó giữ hành vi theo cài đặt dự án |
| D10 | Payload `approval.decided` mang lựa chọn của người duyệt, không mang mặc định phân giải | Mục đích là truy vết hành động của con người |
| D11 | `ApproveResponse.execution_mode` và cờ `approval.runAll` | Gateway cũ bỏ qua tham số lạ; giao diện không được hứa điều backend không làm |
| D12 | Request đang chạy `step_by_step`: ngoài phạm vi, CR riêng (đề xuất) | Lý do ở 2.10; cần người yêu cầu xác nhận (câu hỏi 1) |
| D13 | Pause: ngoài phạm vi, CR riêng (đề xuất) | Giữ nguyên B9 của CR-038 và F12 của CR-037; xem 2.11 |
| D14 | Sửa B10 của CR-REQ-038 ("người duyệt không chọn chế độ trên thẻ duyệt Plan trong v6") | Người yêu cầu đã đổi quyết định ngày 2026-10-20; CR-038 phải ghi chú tham chiếu CR này |

## 4. Tiêu chí chấp nhận

- [ ] Duyệt Plan **không** kèm `executionMode`: hành vi y hệt hôm nay ở dự án `step_by_step` (Approval `phase` được mở cho Phase đầu, cần "Approve phase" rồi "Start phase") và ở dự án `continuous`.
- [ ] Duyệt `change_request` có 3 Phase với `executionMode=continuous` ở dự án `step_by_step`: Request chuyển `executing`, `request.get.executionMode == continuous` (đã ghim), **không** mở Approval `phase`, Phase đầu bắt đầu tự động với `start_trigger=auto` bằng quyền của người duyệt, các Phase sau tự chạy tới khi hết hoặc gặp một điều kiện dừng.
- [ ] Hàng outbox `orca.request.approval.decided` của quyết định đó có `execution_mode="continuous"`; bản ghi audit `approval.approve` có metadata `execution_mode`; quyết định không kèm chế độ thì không có khoá.
- [ ] Người duyệt không có quyền execute: `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`, Approval vẫn `pending`, Request vẫn `awaiting_plan_approval`, chế độ chưa ghim.
- [ ] `executionMode="step_by_step"`, `"bogus"`, hoặc kèm khi từ chối: `REQUEST_EXECUTION_MODE_INVALID`, không ghi gì.
- [ ] Gửi `continuous` cho Approval `solution`, `phase` hoặc Approval ở stage khác: `REQUEST_EXECUTION_MODE_NOT_APPLICABLE`.
- [ ] Request đã ghim `step_by_step` rồi được duyệt lại với `continuous`: `REQUEST_EXECUTION_MODE_ALREADY_PINNED`, Approval vẫn `pending`.
- [ ] Nhấp đúp (hai lời gọi liên tiếp cùng người, cùng phiên bản, cùng digest): một sự kiện `approval.decided`, một lần ghim, lời gọi thứ hai trả cùng kết quả và `executionMode` đang ghim.
- [ ] Hai người duyệt đồng thời một Approval, một kèm `continuous` một không: đúng một người thắng; người kia nhận `REQUEST_APPROVAL_ALREADY_DECIDED`; chế độ ghim khớp với người thắng.
- [ ] Gateway không công bố `approval.runAll`: nút phụ ẩn; vẫn duyệt thường được. Gateway công bố nhưng trả `executionMode` khác `continuous`: giao diện báo lỗi rõ.
- [ ] Giao diện: nút "Approve and run all steps" hiện đúng ở `change_request` và `bug`/`refactor` cỡ L, ẩn ở loại không có Phase và ở Request đã `continuous`; hộp thoại có đúng số bước; Esc/Huỷ không gửi gì.
- [ ] Tab Execution hiện nhãn chế độ đúng cho `continuous`, `step_by_step` có Phase, và không hiện khi chưa ghim.
- [ ] Project Settings: admin đổi được chế độ, giữ nguyên engine/`openspecMinVersion`/`solutionAgent`; người khác thấy chỉ đọc; xung đột phiên bản được xử lý; Request đã ghim không đổi.
- [ ] `parity_test` MCP xanh; CONTRACT được cập nhật bởi người điều phối (mục 8); Migration: không có.
- [ ] Đủ khoá i18n 5 locale; không file nào vượt `max-lines`; không thêm `max-lines` disable.

## 5. Kiểm thử

- **Unit (Go)**: `domain.ParseApprovalExecutionMode` (rỗng, `continuous`, hoa thường và khoảng trắng, `step_by_step`, rác); `DecideApproval` có fake: chế độ hợp lệ ghim và đưa vào payload, `reject`+chế độ, subject/stage sai, ghim sẵn `step_by_step`, ghim sẵn `continuous` (idempotent), handler lỗi cuộn lại ghim, nhấp đúp, `sameDecision` bỏ qua tham số; `ExecutionStartPrecheck` với chế độ yêu cầu (dự án `step_by_step` vẫn kiểm; người không được duyệt nhận lỗi duyệt thường); `AuditTap` thêm metadata. Mở rộng `usecase/approval_execution_precheck_test.go`, `approval_usecase_test.go`, `continuous_execution_test.go` (thêm ca "ghim lúc duyệt rồi `StartExecution` tự bắt đầu Phase đầu").
- **Contract trên Postgres 16 và MySQL 8 (testcontainers)**: không đổi lưu trữ nên không có migration; thêm ca chạy DecideApproval thật với repository thật: ghim và quyết định cùng commit; handler lỗi thì cột `execution_mode` vẫn `NULL` (kiểm cuộn lại); đọc hàng outbox `approval.decided` có `execution_mode`. Mở rộng `contracttest/approval_flow_contract.go` và `execution_flow_contract.go` (kịch bản ba Phase chạy hết sau một lần duyệt với fake task service).
- **Gateway**: `wscompat` test cho `approval.approve` (chuyển `executionMode`, kết quả có `executionMode`, `approval.reject` kèm `executionMode` bị từ chối trước khi gọi service), `request.flowStatus.features` có `approval.runAll`, `request_rpc_channel_map_test.go`; `httpgateway` test body; `parity_test` MCP; CLI test cờ.
- **Acceptance (`tests/stages/e2e/orca-flows`, claude giả `--ai-stub`, chi phí loại A)**: suite mới `suite_approve_run_all.py`, id `ARA-xx`:
  - `ARA-01` luồng `step_by_step` không đổi: duyệt Plan thường, có Approval `phase` chờ, chưa Phase nào bắt đầu; duyệt Phase rồi `request.startPhase` thủ công vẫn chạy.
  - `ARA-02` duyệt kèm `continuous`: phản hồi `executionMode=continuous`, `request.get.executionMode=continuous`, không có Approval `phase`, `request.activity` có Phase đầu bắt đầu `auto` (khi `ORCA_E2E_EXECUTE_AGENT=1`).
  - `ARA-03` chạy hết các Phase không thêm lần bấm nào (cần task thực thi chạy được: mở rộng `claude` giả với loại `execute`, task 06; nếu không chấp nhận được thì chỉ chạy ở loại B `--real continuous`, xem R6).
  - `ARA-04` giá trị không hợp lệ, kèm khi từ chối, subject sai: đúng mã lỗi, Approval vẫn `pending`.
  - `ARA-05` nhấp đúp (hai lời gọi liên tiếp): một kết quả, không lỗi.
  - `ARA-06` ghim sẵn `step_by_step` rồi duyệt lại kèm `continuous`: `ALREADY_PINNED`.
  - `ARA-07` người duyệt thiếu quyền execute bị từ chối trước: **cần một tài khoản thứ hai không có quyền execute**, harness hiện chỉ có phiên admin (`orca_stack_config.py`); ca ở trạng thái `BLOCKED` có lý do khi thiếu biến `ORCA_E2E_MEMBER_EMAIL` và `ORCA_E2E_MEMBER_PASSWORD` (đề xuất tên), còn lại được phủ ở unit và contract.
  - Không có ca nào đọc payload `approval.decided` qua WS (xem 2.5); phần đó ở contract test.
- **UI sống (`ui-live`, `run_ui_flow.py --ai-stub -- approve-run-all`, loại A)** với fixture LedgerLite: Plan `change_request` có Phase cho thấy hai nút; hộp thoại đúng số bước và nội dung; Huỷ không đổi gì; xác nhận dẫn tới `executionMode=continuous` và nhãn trên tab Execution; công tắc Project Settings (admin) đổi và lưu, tải lại vẫn giữ; Request đã `continuous` chỉ có nút chính; gateway giả lập không công bố cờ thì nút phụ ẩn (kỹ thuật proxy WS của `live-base-branch-gateway-features.ts`). Dưới stub không có task chạy thật, nên phần "chạy hết tới xong" nằm ở `--real continuous` (loại B).
- **Loại B (Claude thật, native, repo tạm)**: cập nhật `continuous-execute-real.web.e2e.ts` để chọn "Approve and run all steps" thay vì `configureProject(continuous)`, chạy một lần ở cuối đợt. Loại C (OpenSpec, 7 đến 13 USD mỗi Plan): **không** chạy trừ khi người yêu cầu đồng ý rõ ràng.
- Lưu ý khi chạy e2e thật: không sửa checkout trong lúc `--real` chạy (guard quiet-checkout); sau khi triển khai lại stack local, khởi động lại gateway rồi mới chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- **R1: gateway cũ bỏ qua `executionMode`.** Giảm thiểu bằng cờ `approval.runAll` và đối chiếu `executionMode` trả về (D11).
- **R2: người duyệt là người thực thi cho cả run dài.** Quyền bị gỡ giữa chừng thì task kế tiếp lỗi quyền và Request về backlog (B11 của CR-038, `continuous_phases.go:56-69`). Hộp thoại phải nói "using your access".
- **R3: bớt điểm dừng của con người.** Chấp nhận rủi ro theo từng Phase không còn; dựa vào quy tắc B8 của CR-038 (thu một lần ở Approval Plan). **Chưa kiểm chứng** backend có đọc các trường rủi ro (`acceptedFindingIds`, `viewedImpactDigest`) hay không: grep trong `backend-go` không thấy mã nào dùng chúng, chỉ frontend gửi (`usePlanDecision.ts:103-106`). Phase thêm sau khi duyệt chỉ được duyệt lệch bảo vệ (2.12).
- **R4: ghim cố định.** Request đã ghim `step_by_step` không bao giờ chạy hết được bằng lựa chọn này (2.10). Giao diện không biết ghim hay "sẽ dùng" vì `Annotate` ghi đè (`request_engine_resolver.go:47`), nên có thể mời nút rồi nhận `ALREADY_PINNED`; chấp nhận vì hiếm (chỉ Request quay lại `awaiting_plan_approval` sau khi đã chạy), thông điệp rõ.
- **R5: lưu công tắc dự án ghi cả engine và `solution_agent` hiện hành vào dòng cài đặt.** Mặc định triển khai (`REQUEST_DEFAULT_ENGINE`, `REQUEST_DEFAULT_SOLUTION_AGENT`) không còn tự áp cho dự án đó. Hai admin sửa cùng lúc được khoá lạc quan chặn.
- **R6: `claude` giả không nhận prompt thực thi task.** `fake_claude_cli/claude` chỉ nhận bốn dấu mở đầu (`CLASSIFY_MARK`, `SOLUTION_MARK`, `PLAN_MARK`, `READONLY_MARK`, dòng 124-127, `prompt_kind` dòng 484-490); prompt thực thi (`prompts/task_execution/execute.tmpl`) rơi vào `unknown` và "fail loudly". Spec UI `qa-bug-task-native-flow` cũng dừng ở duyệt Plan vì lý do này. Chạy hết các Phase dưới stub cần mở rộng stub (task 06); **chưa kiểm chứng** task-service có chấp nhận khối kết quả `done` không có thay đổi tệp không. Nếu không, ca chạy hết chỉ ở loại B.
- **R7:** số trường proto (`ApproveRequest` 5, `ApproveResponse` 3) và tên mã lỗi có thể bị CR khác giữ; kiểm lại khi triển khai.
- **R8: công tắc chỉ cho admin toàn hệ thống.** Chủ dự án mở được Project Settings nhưng không đổi được; có thể gây khó hiểu (câu hỏi 6).
- **R9:** chưa đọc notification-service để chắc nó giải mã `approval.decided` không nghiêm ngặt với khoá lạ (mới thấy `DisallowUnknownFields` ở webhook, `clarification`, `classification`, `provenance`, không ở consumer này).
- **R10: chỉ có "Stop run" để phanh.** Chạy liên tục làm khoảng trống Pause rõ hơn (2.11).
- Chưa chạy hệ thống; mô tả hiện trạng dựa trên đọc mã ngày 2026-10-20.

## 7. Câu hỏi còn mở

> **Đã chốt 2026-10-20:** câu 1 và 2 theo đề xuất (Request đã ghim `step_by_step` tiếp tục từng bước, CR đổi chế độ cho Request đang chạy làm sau; công tắc dự án vẫn chỉ admin hệ thống đổi). Câu 3 đến 5 chưa được hỏi riêng: lấy theo mặc định ghi ở từng câu.

1. Có chấp nhận để Request #1771 (và mọi Request đã ghim `step_by_step`) tiếp tục từng bước sau CR này không, hay CR "đổi chế độ cho Request đang chạy" (2.10) phải làm trước? Đề xuất: chấp nhận, vì lý do 2.10.
2. Approval Inbox có cần lựa chọn "chạy hết" không? Mặc định: không (D9).
3. Có cần ghi nhận lựa chọn "chạy hết" vào History (Activity), ngoài audit? Muốn vậy phải lưu vào bảng `approvals` (migration) hoặc đọc từ outbox; hôm nay History dựng từ bảng. Mặc định: không trong CR này.
4. Dự án đang để `continuous` mà một Request cụ thể muốn từng bước thì làm sao? Tham số hiện chỉ nhận `continuous` hoặc rỗng nên chỉ có cách đổi cài đặt dự án. Có cần nhận `step_by_step` tường minh ở một CR sau không?
5. Có cho chủ dự án (không phải admin) đổi công tắc không? Hôm nay backend chặn bằng `callerIsAdmin`; cho phép phải sửa phân quyền của `SetProjectEngineSettings`, ngoài CR này.

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-038 | B10 ("người duyệt không chọn chế độ trên thẻ duyệt Plan") nay có ngoại lệ: người duyệt chọn `continuous` lúc duyệt. Ghi chú tham chiếu CR này; B9 (Pause) vẫn giữ |
| CR-REQ-009, 010 | `approval.approve` có tham số `executionMode` và mã lỗi mới; payload `approval.decided` thêm `execution_mode` |
| CR-REQ-016 và CONTRACT-request-ui-api | Dòng 198 (`approval.approve`: thêm `executionMode?` và `executionMode` trong kết quả), dòng 408 (body REST thêm `executionMode`), bảng lỗi (dòng ~431): ba mã mới; `request.flowStatus.features` thêm `approval.runAll`. **Người điều phối cập nhật**, CR này không sửa CONTRACT |
| CR-REQ-021, 037 | Thanh duyệt Plan có nút phụ; dải tóm tắt tab Execution có nhãn chế độ |
| CR-REQ-035 | Bản ghi audit `approval.approve` có metadata `execution_mode` |
| RELEASE-2026-10-DEPLOY.md | Mục 4.3 và dòng 207 ("Chưa có màn hình cài đặt…") lỗi thời sau CR này |

## 9. Tham chiếu

- `backend-go/services/request-service/internal/usecase/{decide_approval,approval_execution_precheck,execution_mode_pin,start_execution,continuous_phases,evaluate_execution,start_phase,execution_actors,audit_event_tap,manage_project_engine_settings,request_engine_resolver}.go`, `domain/{execution_mode,execution_errors,approval_events,approval_authorization,request_flow_registry}.go`, `adapter/{postgres,mysql}/request_repository.go` (`PinExecutionModeIfUnset`), `adapter/grpc/approval_server.go`, `cmd/server/{wire_approval,wire_execution,store_wiring}.go`
- `backend-go/proto/orca/request/v1/{approval,request,activity}.proto`
- `backend-go/services/api-gateway/internal/adapter/wscompat/{channels_approval,channels_request_engine,channels_request_base_ref,channels_request_stream,channels_admin_audit}.go`, `httpgateway/request_routes.go`, `mcpserver/tools/excluded_channels.yaml`
- `backend-go/cmd/orca-cli/internal/command/approval_cmd.go`, `internal/apiclient/request.go`
- `frontend/src/renderer/src/components/request/plan/{PlanApprovalBar,PlanApprovalCard,PhaseApprovalBar,RequestPlanTab,plan-decision-error-message}.tsx`, `execution/{ExecutionSummaryStrip,ExecutionNeedsYou}.tsx`, `components/project/ProjectSettings.tsx`, `hooks/usePlanDecision.ts`
- `deploy/dev/{docker-compose.yml,.env.example,RELEASE-2026-10-DEPLOY.md}`
- `tests/stages/e2e/orca-flows/{fake_claude_cli/claude,orca_stack_config.py,suite_request_base_ref.py,ui-live/live-continuous-execution-steps.ts,ui-live/continuous-execute-real.web.e2e.ts}`
- [CR-REQ-038](./CR-REQ-038-continuous-execution-activity-feed-and-execution-summary.md), [CR-REQ-039](./CR-REQ-039-request-worktree-base-branch.md) (mẫu cờ tính năng), [CR-REQ-009](../approval/CR-REQ-009-generic-approval-domain-and-api.md), [CR-REQ-037](../request-frontend/CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md)
- Mới: `domain/approval_execution_mode.go`, `plan/RunAllStepsConfirmDialog.tsx`, `components/project/ProjectRequestExecutionSection.tsx`, `tests/stages/e2e/orca-flows/suite_approve_run_all.py`
