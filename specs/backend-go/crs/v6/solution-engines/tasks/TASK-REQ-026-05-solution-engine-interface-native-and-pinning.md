# TASK-REQ-026-05: Giao diện `SolutionEngine`, `nativeEngine`, ghim engine và `engine_override`

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/solution_engine.go`, `internal/usecase/engine_native.go`, `internal/usecase/pin_solution_engine.go`, `internal/usecase/generate_solution.go` (sửa, của SOL-007), `internal/usecase/run_solution_generation.go` (sửa), `internal/usecase/generate_plan.go` (sửa, của SOL-012), `internal/usecase/confirm_request_type.go` (sửa, của SOL-005), `proto/orca/request/v1/request.proto` (sửa)
**Depends on:** TASK-REQ-026-01, 026-02, 026-04, TASK-REQ-007-05 (GenerateSolution), TASK-REQ-012-05 (GeneratePlan/CommitPlan), TASK-REQ-005-05 (ConfirmRequestType)
**Status:** [ ] TODO

---

## Context

Đây là task rủi ro hồi quy cao nhất của solution: nó **di chuyển** mã của SOL-007 và SOL-012 sau một giao diện mà không đổi hành vi. Cả hai solution đó phải đã có mã (hoặc đã có PR) khi bắt đầu; nếu chưa, task này chỉ làm giao diện và `EngineRegistry`, rồi mở lại sau.

Mã nguồn cần đọc lại ngay lúc làm (chưa có trong repo): `run_solution_generation.go` (khối "dựng prompt, gọi `AICompleter.Complete`, trích JSON") và `plan_generator_relay.go` (`PlanGenerator.Generate`). Điểm chốt từ solution: `GenerateSolution.Execute` bước 2 dùng `FlowFor(req.Type)`; bước 5 tạo `analysis_runs`; worker `RunSolutionGeneration` làm phần sinh. Chỉ phần **sinh** đi qua `SolutionEngine`; validate, `OpenApproval`, `TransitionRequest`, outbox ở lại use case (SOL-026 mục 2.D).

`GenerateSolutionRequest` ở SOL-007 mục D dùng các số trường 1 đến 4 theo CR-REQ-007; số của `engine_override` phải đọc từ `request.proto` thật, không mặc định 5.

## Việc cần làm

1. `solution_engine.go`: định nghĩa `SolutionEngine` đúng như SOL-026 mục 2.D;
   - `ProjectRef{TenantID, ProjectID, RepoID string}`
   - `AnalysisInput{Request domain.Request; PriorArtifacts []PriorArtifact; Feedback string; RunID string; Attempt int}`
   - `AnalysisOutput{OptionsJSON []byte; Raw string; Provenance domain.Provenance; Model string}`
   - `PlanInput{Request domain.Request; Solution *domain.Solution; Feedback string}`
   - `PlanRaw string`
   - `CompletionInput{Request domain.Request}`
   - `EngineRegistry{For(name) (SolutionEngine, error)}`.
   - `Provenance` do SOL-027 định nghĩa: nếu chưa có, dùng `json.RawMessage` và ghi chú thay sau.
2. `engine_native.go`: `nativeEngine{completer AICompleter; planGen PlanGenerator; prompts ...}`.
   - `Name()` trả `native`
   - `Preflight` trả `PreflightReport{OK:true}` (không kiểm gì mới, vì đường `ai.complete` đã kiểm kết nối bên trong `Complete`)
   - `GenerateAnalysis` chứa **nguyên** đoạn dựng prompt, gọi `Complete`, trích JSON của `RunSolutionGeneration`
   - `GeneratePlan` bọc `PlanGenerator.Generate`
   - `OnRequestCompleted` trả `nil`.
3. Sửa `RunSolutionGeneration` để gọi `engines.For(effective).GenerateAnalysis(...)` thay cho đoạn đã chuyển;
   - giữ lại phần lease, heartbeat, trích kết quả sau engine (`SolutionOptions.Validate`, retry `attempt=2` do **engine** `native` giữ như cũ).
   - Sửa `GeneratePlan.Execute` (PROPOSE) tương tự.
   - Ghi `analysis_runs.engine` = tên engine thật (cột từ task 026-02) khi chèn run.
4. `pin_solution_engine.go`: `PinSolutionEngine.Execute(ctx, requestID string) (domain.EngineName, error)`: trong transaction, `repo.Get`;
   - nếu `solution_engine` đã có thì trả nguyên
   - không thì `domain.EffectiveEngine(nil, settings, FlowFor(type).OpenSpecProfile)`, `repo.UpdateSolutionEngine(ctx, id, name, expectedVersion)` (CAS).
   - Gọi từ `ConfirmRequestType` ngay sau `TransitionRequest(type_confirmed)` cùng transaction
   - nếu SOL-005 chưa thể sửa, gọi lười ở đầu `GenerateSolution` và `GeneratePlan` (idempotent).
5. `engine_override`: thêm `string engine_override` vào `GenerateSolutionRequest` (số trường kiểm thật).
   - Trong `GenerateSolution.Execute`: rỗng thì bỏ qua
   - khác `native` thì `REQUEST_ENGINE_INVALID`
   - người gọi không phải `reporter_id` và không phải admin thì `REQUEST_ENGINE_OVERRIDE_NOT_ALLOWED`
   - hợp lệ thì cập nhật `solution_engine=native` (CAS), ghi dòng audit và sinh bằng `nativeEngine`.
6. Khi `effective==openspec`: `GenerateSolution` gọi `engine.Preflight` **trước khi** chèn run (SOL-007 bước 5), nếu lỗi trả `REQUEST_ENGINE_PREFLIGHT_FAILED` ngay, không tạo run, không tạo Solution `draft`.
7. Giới hạn đồng thời: mở rộng truy vấn đếm run `agent_readonly` của SOL-008 để đếm cả `agent_proposal` (cùng `REQUEST_AGENT_READONLY_MAX_PER_PROJECT`);
   - sửa repository (`CountRunning(ctx, projectID, modes []AnalysisMode)`).
8. Proto: thêm `string engine = N` vào `Solution` và `AnalysisRun` (số theo file thật); chạy `buf generate` và `buf breaking` với nhánh chính.

## Kiểm thử

- **Hồi quy bắt buộc:** toàn bộ test của TASK-REQ-007-05/-06 và TASK-REQ-012-05 chạy lại không sửa kỳ vọng, chỉ đổi cách lắp (fake registry trả `nativeEngine`). Tên test mới: `TestGenerateSolution_ViaNativeEngine_SameBehaviour`, `TestGeneratePlan_ViaNativeEngine_SameBehaviour`.
- `TestPinSolutionEngine_PinsOnce` (gọi hai lần, giá trị không đổi dù `settings` đổi giữa hai lần)
- `TestPinSolutionEngine_NoneProfileForcesNative`
- `TestPinSolutionEngine_CASConflictRetries`.
- `TestEngineOverride_Native_ReporterAllowed`
- `TestEngineOverride_AdminAllowed`
- `TestEngineOverride_OtherUserDenied`
- `TestEngineOverride_NonNativeValueRejected`
- `TestEngineOverride_UpdatesPinAndWritesAudit`.
- `TestGenerateSolution_OpenSpecPreflightFails_NoRunNoDraft`.
- `TestConcurrencyGate_CountsBothReadonlyAndProposal` (fake repo; thêm integration hai dialect cho `CountRunning`, trong 026-08).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'Engine|Pin|GenerateSolution|GeneratePlan'` rồi `go test ./services/request-service/...` (toàn bộ, để bắt hồi quy) và `buf breaking` theo `backend-go/proto/buf.yaml`.

## Tiêu chí hoàn thành

- [ ] Không có dòng `project_engine_settings` nào: hành vi `GenerateSolution` và `GeneratePlan` giống hệt trước task (toàn bộ test cũ xanh, kỳ vọng không đổi).
- [ ] Mọi lần sinh ghi `analysis_runs.engine`; Request có `solution_engine` sau lần sinh đầu.
- [ ] Đổi cờ project sau khi đã ghim không đổi engine của Request đó (test).
- [ ] `engine_override` từ người không có quyền bị từ chối; hợp lệ có audit.
- [ ] Preflight `openspec` thất bại không để lại run hay Solution `draft`.
- [ ] `buf breaking` không báo thay đổi phá vỡ.

## Rủi ro và lưu ý

- Di chuyển mã hai solution khác dễ gây xung đột merge; làm trên một nhánh ngắn ngày và rebase thường xuyên, hoặc thoả thuận với người giữ SOL-007 và SOL-012 thêm điểm nối `SolutionEngine` ngay từ đầu (khuyến nghị: thêm vào SOL-007 và 012 trước khi cài).
- Ghim ở `ConfirmRequestType` thêm một cột ghi vào transaction vốn đã nhiều bước; nếu CAS thua do `ConfirmRequestType` đang cập nhật `type`, dùng phương án lười (bước 4 cuối).
- Request `type_change` sang loại `none` sau khi ghim `openspec`: `EffectiveEngine` ép `native`, nhưng `solution_engine` vẫn là `openspec`; chấp nhận, ghi ở README của task.
- `Provenance` (`model` rỗng với `openspec`) phụ thuộc SOL-027; thiếu thì lưu `null` và bổ sung sau.
