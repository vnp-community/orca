# TASK-REQ-029-08: Nối `ReadinessGate`, packet, `VerifyExecution` vào `AdvanceExecution` và `ReportTaskOutcome`; cờ, sự kiện, hợp đồng và e2e

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.D, 2.F, 2.G
**Priority:** P0
**Service/Area:** `request-service` (mới) / usecase (sửa của SOL-013), config, composition root; `task-service` chỉ test hợp đồng
**File:** `internal/usecase/advance_execution.go` (sửa, tạo ở TASK-REQ-013-05), `internal/usecase/report_task_outcome.go` (sửa, tạo ở TASK-REQ-013-06), `internal/usecase/render_execution_packet.go` (mới), `internal/usecase/readiness_actions.go` (mới), `internal/adapter/secretscan/secretscan_adapter.go` (mới), `internal/config/config.go` (sửa), `cmd/server/main.go` (sửa), `internal/domain/outbox_subjects.go` (sửa), `backend-go/services/request-service/testdata/contract/{task_spec_v2.schema.json,execution_result.schema.json}` (mới), `backend-go/services/task-service/testdata/contract/` (bản sao), `internal/e2e/execution_contract_e2e_test.go` (mới), và các `_test.go`
**Depends on:** TASK-REQ-029-03, 029-04, 029-05, 029-06, 029-07; TASK-REQ-013-04 (client `TaskService`), 013-05 (`AdvanceExecution`), 013-06 (`ReportTaskOutcome`); CR-REQ-028 (`RequestClarification`); TASK-REQ-006 (`ReturnToBacklog`); TASK-REQ-025-07 (agent giả)
**Status:** [ ] TODO

---

## Context

- SOL-013 mục 2.4: `AdvanceExecution` (idempotent): (1) liệt kê task lá, (2) chọn `open` trừ số `in_progress` khỏi `REQUEST_MAX_PARALLEL_TASKS`, (3) `TypePolicy.PreExecutionGate(task)`, (4) worktree dùng chung, (5) `task-service.Execute(task_id, request_id="req:<request_id>:<task_id>:<attempt>")`. CR-REQ-029 mục 9 chèn `ReadinessGate` **giữa bước 3 và bước 5**, và `Execute` truyền `prompt = packet.Text` cùng `result_nonce`.
- SOL-013 mục 2.5 bảng phân loại `ReportTaskOutcome`: dòng `execution_completed`/`review` → CR-REQ-029 chèn `VerifyExecution` **trước** `REQUEST_AUTO_COMPLETE_TASKS` đặt `done`; dòng `execution_failed` phân nhánh theo `failure_class` (có sẵn trong payload `statuschanged` sau TASK-REQ-029-03; nếu rỗng thì `request-service` tự phân loại bằng `ClassifyFailure`).
- Ánh xạ hành động theo `outcome` của cổng (CR mục 2.4, 9): `ready` thì render và `Execute`; `needs_info` thì `RequestClarification(source=task_blocked, source_ref=task_id, questions[])` (CR-REQ-028 mục 2.6; chỉ khi Request `executing`, Request sang `awaiting_information`); `spec_defect` thì `ReturnToBacklog(stage=plan, category=other)`; `env_defect` thì giữ task `open`, Request `executing`, đối soát thử lại (vòng `ReconcileExecutingRequests` của TASK-REQ-013-07), quá `REQUEST_DISPATCH_RETRY_WINDOW` thì `ReturnToBacklog(stage=task, category=blocked_dependency)`. **Không tính lần thử** cho ba kết quả đó.
- `task_run_outcomes` (013-03) giữ lần thử; `failure_class`, `verdict`, `execution_record_id` thêm ở task 04. `request_checks.source=orca_verified` ở task 04. Cờ `REQUEST_EXECUTION_CONTRACT_ENABLED` (mặc định `false`): tắt thì **toàn bộ** hành vi của SOL-013 gốc (không cổng, không packet, không verify).
- `common/secretscan` (CR-REQ-035) chưa có trong repo; adapter ở đây bọc nó, và nếu CR-035 chưa merge thì adapter trả scanner rỗng kèm cảnh báo khởi động (cờ tắt cũng được).
- Idempotency: `AdvanceExecution` chạy lại nhiều lần (sự kiện, đối soát). Mỗi `(task_id, attempt)` chỉ có một `execution_packets` (UNIQUE ở task 04) và `Execute` chỉ gọi khi insert packet thành công hoặc packet đã có mà chưa có `task_run_outcomes` `started` cho lần thử đó.

## Việc cần làm

1. `config.go`: `ExecutionContractEnabled bool` (`REQUEST_EXECUTION_CONTRACT_ENABLED`, mặc định false), `VerifyBudget time.Duration` (`REQUEST_VERIFY_BUDGET`, 8m), `ReadinessBaselineTTL` (`REQUEST_READINESS_BASELINE_TTL`, 30m), `SpecVagueWords []string` (`REQUEST_SPEC_VAGUE_WORDS`, danh sách phân tách bằng dấu phẩy, mặc định "tốt,hợp lý,nếu cần"). Giá trị sai thì khởi động lỗi rõ ràng (không mặc định im lặng); `VerifyBudget` ngoài `[1m, 15m]` bị từ chối.
2. `render_execution_packet.go`: use case `PreparePacket.Execute(ctx, in) (ExecutionPacket, nonce string, err error)`: gom `PacketInput` từ Request (`GetRequest`), Solution đã chọn, Plan/Phase (`GetTask`), `evidence_ids`, câu trả lời Clarification đã `answered` của Request (CR-REQ-028), đầu vào `inputs` từ `ListExecutionRecords(latest_only)` của các task nguồn (cắt 4 KB, **chỉ** output có tên khai ở `outputs[]`), `PriorVerdictSummary` từ `task_run_outcomes.verdict` lần trước (≤ 1 KB, do mã dựng: `"Lần 1: SCOPE_VIOLATION tại <path>; CHECK_FAILED c1 (exit 1)"`):
   - sinh `nonce` bằng `crypto/rand` (`domain.NewResultNonce`, 32 ký tự chữ-số, thống nhất với TASK-REQ-033-05)
   - gọi `RenderExecutionPacket` với `redact = secretscan.Redact`
   - ghi `execution_packets` (chỉ **băm** nonce: `nonce_hash = sha256(nonce)`)
   - trả packet và nonce **chỉ trong bộ nhớ**.
3. `advance_execution.go` (sửa): sau bước 3, nếu `Cfg.ExecutionContractEnabled` và task thuộc Request:
   a. `rep, err := gate.Check(ctx, GateInput{RequestID, TaskID, Attempt})`; `errors.Is(err, ErrDependencyNotDone)` thì bỏ task này khỏi vòng (không lỗi);
   b. `rep.Outcome` khác `ready` thì `readiness_actions.Apply(ctx, rep)` theo bảng Context và dừng task này (không `Execute`, không tăng `attempt`); phát `orca.request.readiness.reported {request_id, task_id, attempt, outcome, tier}` bằng outbox cùng giao dịch với việc ghi báo cáo (không phải sau);
   c. `ready`: `PreparePacket`, rồi `TaskClient.Execute(ctx, ExecuteInput{TaskID, RequestID: "req:...", Prompt: packet.Text, ResultNonce: nonce})` (trường `ResultNonce` của client gRPC sinh từ proto của TASK-REQ-029-02);
   d. `TASK_EXECUTE_ALREADY_IN_PROGRESS` coi là thành công; lỗi hạ tầng (`NO_CONNECTION`, `WORKTREE_FAILED`) không tính lần thử, xử lý như SOL-013.
   Cờ tắt: bỏ qua cả khối, `Execute` với `Prompt` rỗng như SOL-013 gốc.
4. `readiness_actions.go`: `Apply(ctx, rep TaskReadinessReport) error`: `needs_info` thì `ClarificationService.RequestClarification` với `questions` dựng từ finding `INPUT_NOT_AVAILABLE`/`stop_conditions` (câu hỏi tiếng Việt cố định theo mã, mỗi câu `key=<finding code>:<path>`):
   - gọi `RequestClarification.Execute(ctx, RequestClarificationInput{RequestID, Source: giá trị `task_blocked` của `ClarificationSource` (TASK-REQ-028-02), SourceRef: taskID, Questions, ActorID: "system", ActorKind: system})` (TASK-REQ-028-04)
   - giao lặp với cùng `source`, `source_ref` và cùng `question_key` đã được use case đó xử lý idempotent (trả Clarification `open` sẵn có, không tạo thứ hai) nên không cần khoá riêng ở đây
   - `spec_defect` thì `ReturnToBacklog`
   - `env_defect` thì ghi cảnh báo (`orca.request.readiness.reported` đã đủ) và để đối soát.
5. `report_task_outcome.go` (sửa): với task có spec và cờ bật:
   a. `execution_completed`/`review`: `verdict := verify.Execute(...)`; ghi `task_run_outcomes` (`verdict`, `execution_record_id`, `failure_class` nếu `failed`) và phát `orca.request.execution.verified {request_id, task_id, attempt, status, failure_class, finding_codes[]}` (không có đoạn diff, không có giá trị); `passed` thì ghi `request_checks` (`source=orca_verified`) cùng giao dịch, rồi mới đến `REQUEST_AUTO_COMPLETE_TASKS`; `failed` thì `ClassifyFailure` và hành động: `agent_defect` còn lần thì chạy lại (`attempt+1`, `PriorVerdictSummary`), hết lần thì `ReturnToBacklog(stage=task, category=other)`; `GoStraightToBacklog` (`SECRET_FOUND`) đi thẳng backlog; `spec_defect` thì `ReturnToBacklog(stage=plan, category=other)`; `env_defect` thì giữ `open`, đối soát.
   b. `execution_failed`: dùng `failure_class` trong payload (rỗng thì `ClassifyFailure` từ `error_message`/`cause`); `retryable` tính lần thử; `needs_info` thì `RequestClarification` với `questions` đọc từ `result.questions` của bản ghi (`ListExecutionRecords`); `agent_defect` có `RetryWithFormatReminder` thì lần chạy lại thêm mục nhắc định dạng ở `PriorVerdictSummary` ("Lần trước không có khối ORCA_RESULT; cần in đúng một khối cuối cùng").
   c. Nguyên tắc ngân sách: tổng thời gian `VerifyExecution` ≤ `REQUEST_VERIFY_BUDGET`; vượt thì verdict `failed` mã `REQUEST_VERIFY_BUDGET_EXCEEDED` lớp `retryable` không tính lần thử (chỉ một lần, lần hai tính).
6. `adapter/secretscan/secretscan_adapter.go`: cài `SecretScanner` và `redact` bằng `common/secretscan` (khi có); nếu thư mục chưa tồn tại thì build tag/constructor trả bản rỗng và `main.go` log `secretscan unavailable`; **không** tạo bản sao riêng của bộ mẫu ở đây (CR-REQ-035 mục 2.6 muốn một nguồn).
7. Hợp đồng JSON: `testdata/contract/task_spec_v2.schema.json`, `execution_result.schema.json` (JSON Schema draft 2020-12) ở `request-service`:
   - bản sao giống hệt ở `task-service/testdata/contract/`
   - thêm script `scripts/check-execution-contract-schemas.sh` so `sha256sum` hai bản schema
   - mẫu vàng canonical do TASK-REQ-027-02/027-03 sở hữu (script chỉ gọi lại bước so sánh của chúng nếu đã có), thoát khác 0 khi lệch
   - thêm target Makefile riêng (không dùng `proto-lint`). Hai bên đều có test `TestSchemaAcceptsSamples` / `_RejectsBadSamples` với cùng mẫu.
8. Sự kiện: hằng `SubjectReadinessReported = "orca.request.readiness.reported"`, `SubjectExecutionVerified = "orca.request.execution.verified"` ở `outbox_subjects.go`; payload struct có `json` tag `snake_case`.
9. e2e `execution_contract_e2e_test.go` (khung của TASK-REQ-025-03, agent giả của 025-07): các kịch bản ở mục Kiểm thử.

## Kiểm thử

- Wiring: `TestAdvance_FlagOff_NoGateNoPacket` (hành vi SOL-013 gốc, `Prompt` rỗng, không `ResultNonce`), `_FlagOn_ReadyExecutesWithPacketAndNonce`, `_FlagOn_SpecDefect_NoExecute_NoAttemptIncrement_ReturnsToBacklogPlan`, `_NeedsInfo_CreatesOneClarification_Idempotent`, `_EnvDefect_KeepsOpen_RetriedByReconcile`, `_EnvDefectPastWindow_ReturnsToBacklogBlockedDependency`, `_RepeatedAdvanceNoDoubleExecute`, `_PacketInsertedOncePerAttempt`, `_NonceNeverLogged` (bắt log), `_NonceHashStoredNotNonce`.
- `TestReport_ExecutionCompleted_VerifyPassed_WritesRequestChecksOrcaVerified_ThenAutoComplete`, `_VerifyFailed_ScopeViolation_RetriesWithPriorSummary`, `_SecretFound_GoesStraightToBacklog`, `_FailureClassFromPayload`, `_MissingBlockFirstTimeNotCounted`, `_NeedsInfoFromResultQuestions`, `_BudgetExceededClassRetryable`, `_FlagOff_OriginalFlow`.
- Hợp đồng: `TestSchemasMatchTaskService` (script so hash trong CI), `TestSchemaAcceptsSamples`, `TestSchemaRejectsBadSamples`, `buf lint` + `buf breaking` cho proto thêm.
- e2e (T1 giả lập): kịch bản (1) thiếu Check, ra `spec_defect` và về `planning`/backlog:
     - (2) thiếu `go` trong hồ sơ, `env_defect`, lần thử không tăng
     - (3) agent giả không trả khối, `agent_defect`, thử lại một lần có nhắc
     - (4) khối nonce sai bị bỏ qua
     - (5) agent giả khai Check xanh, Orca đo đỏ
     - (6) agent sửa file ngoài scope kể cả tự commit
     - (7) diff chứa khoá PEM
     - (8) hai nhánh cờ bật/tắt (README v6 / CR-REQ-025).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/... ./services/task-service/...` và `go test -tags=integration ./services/request-service/...`.

## Tiêu chí hoàn thành

- [ ] Cờ tắt: luồng giống hệt SOL-013 gốc (test hồi quy so sánh chuỗi lệnh gọi `TaskClient`).
- [ ] `needs_info`, `spec_defect`, `env_defect` không tăng số lần thử; `needs_info` sang `awaiting_information` qua `RequestClarification`, không tạo hai Clarification khi lặp.
- [ ] Task có spec không sang `done` khi `VerifyExecution` `failed`; `request_checks` của task có spec mang `source=orca_verified`.
- [ ] Không log hay payload sự kiện nào chứa nonce, giá trị biến môi trường, diff, hoặc giá trị bí mật (test quét).
- [ ] Script so schema và vector digest xanh; `buf lint`/`buf breaking` xanh.
- [ ] `gitnexus_impact` đã chạy cho `AdvanceExecution` và `ReportTaskOutcome` (do SOL-013 sở hữu) trước khi sửa, và `detect_changes` trước khi commit.

## Rủi ro và lưu ý

- Hai task (013-05, 013-06) do agent khác soạn; khi chúng đổi tên hàm hay chữ ký, task này phải cập nhật. Đọc lại hai file đó lúc làm.
- Vòng `AdvanceExecution` gọi cổng mỗi lần, mỗi lần vài lệnh `agent.exec`; số task song song nhỏ (`REQUEST_MAX_PARALLEL_TASKS=1`) giới hạn tải, nhưng đối soát 60 giây có thể chạy cổng lặp cho task `env_defect`; cache Check nền theo `(worktree_id, head_sha)` giảm nhưng chưa đo.
- Nonce sống trong bộ nhớ từ `PreparePacket` tới `Execute`; nếu tiến trình chết giữa chừng, lần sau sinh nonce mới và packet mới (`attempt` giữ nguyên, bản ghi cũ bị thay): cần quy tắc `UPSERT` thay vì `Insert` (hoặc bản ghi `attempt` mới). Chốt khi cài: đề xuất `attempt` chỉ tăng khi có `task_run_outcomes` `started`.
- `Q2` của CR-REQ-029: `spec_defect` về `plan_revision` hay backlog; hiện backlog, đổi khi CR-REQ-003 xác nhận.
- Agent giả (TASK-REQ-025-07) chưa có khi viết; nếu chưa merge, viết bản giả tối thiểu trong test (relay giả trả `stdout` có khối), và đánh dấu e2e ở chế độ `-short` bỏ qua.
