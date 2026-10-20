# TASK-REQ-044-01: Miền `plan_auto`: trạng thái job, khoá idempotency, mã lỗi tạm thời, chính sách từ env và luật nhận job

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/plan_auto.go` (mới), `internal/domain/plan_auto_test.go` (mới), `internal/domain/engine_settings.go` (sửa)
**Depends on:** (không)
**Status:** [x] DONE (2026-10-20)

---

## Context

- Phần miền thuần Go, không I/O. Mọi task sau dùng lại: repository (02), công tắc (04), consumer (05), worker (06).
- Luật nhận job lấy từ CR-REQ-044 mục 2.2: `to=planning`, trigger `analysis_approved` hoặc (`type_confirmed` cho loại `AnalysisKind == none`), `PlanKind` không phải `none`/`single_task`, `PlanShapeFor != ShapeNone`, `PlanTaskID == ""`. Các hằng loại và flow ở `internal/domain/request_flow_registry.go`; transition ở `internal/domain/request_transition.go:60-125`.
- Mã lỗi của run Plan là mã **thật** (xem solution mục 1, phần Correction): `REQUEST_AI_COMPLETE_TIMEOUT`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_AI_*` (`usecase/ai_unavailable.go:10-18`), `REQUEST_OPENSPEC_*`, `REQUEST_SOLUTION_RUN_INTERRUPTED` (`domain/solution_errors.go:70-93`). Không dùng `REQUEST_SOLUTION_AI_FAILED`/`_AI_TIMEOUT` (mã của run Solution).
- Mẫu đọc env có cảnh báo: `EngineDefaultsFromEnv` trong `internal/domain/engine_settings.go` và `PlanLimitsFromEnv`.

## Việc cần làm

1. `plan_auto.go`: `PlanAutoState` (`pending`, `running`, `committing`, `done`, `gave_up`, `skipped`, `superseded`), `IsActive()`, `IsFinal()`, `CanMove(to)` theo bảng của solution mục 2.2 (ba trạng thái cuối không đi tiếp; `pending→pending` và `committing→committing` hợp lệ cho lịch lại).
2. `PlanAutoOrigin` (`event`, `retry`) và `PlanAutoJob` với các trường ở solution 2.2.
3. `PlanAutoKey(anchor, attempt)`, `SolutionAnchor(id)`, `TypeConfirmAnchor(eventID)`, `RetryAnchor(uuid)`. Khoá tối đa 128 ký tự (cột `VARCHAR(128)` của MySQL, `migrations/mysql/0006_analysis_runs.up.sql`); trả lỗi miền nếu vượt.
4. `IsTransientPlanRunCode(code)`: một bảng duy nhất: `REQUEST_AI_COMPLETE_TIMEOUT`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_AI_BUSY`, `REQUEST_OPENSPEC_TIMEOUT`, `REQUEST_OPENSPEC_AGENT_FAILED`, `REQUEST_SOLUTION_RUN_INTERRUPTED` là tạm thời; mọi mã khác (kể cả mã lạ và rỗng) là không. Thêm hằng cho `REQUEST_PLAN_COMMIT_FAILED` và `REQUEST_PLAN_AUTO_DISABLED` cùng hàm lỗi `ErrPlanAutoDisabled()` (`KindFailedPrecondition`, đặt cạnh `plan_errors.go` theo mẫu `planInvalid`).
5. `PlanAutoPolicy` + `PlanAutoPolicyFromEnv(getenv)`: `MaxExtraAttempts` (0..2, mặc định 1; ngoài khoảng thì kẹp và cảnh báo), `RetryDelay` 30 giây, `BusyDelay` 60 giây, `StartWindow` 30 phút, `MaxCommitAttempts` 5, `PollInterval` 5 giây, `LeaseTTL` 60 giây. Env: `REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS`, `REQUEST_PLAN_AUTO_RETRY_DELAY`, `REQUEST_PLAN_AUTO_BUSY_DELAY`, `REQUEST_PLAN_AUTO_START_WINDOW`, `REQUEST_PLAN_AUTO_POLL_INTERVAL` (chuỗi `time.ParseDuration`). Giá trị lạ giữ mặc định và trả cảnh báo; không làm service từ chối khởi động.
6. `WantsAutoPlan(req Request, flow FlowDefinition, trigger Trigger, to RequestStatus) bool` theo luật ở Context.
7. `engine_settings.go`: `ProjectEngineSettings.PlanAutoGenerate bool`; `NewProjectEngineSettings` đặt `true`. Chưa đụng repository (task 02) hay usecase (task 04).

## Kiểm thử

- Bảng `IsTransientPlanRunCode`: từng mã ở mục 4 (đúng), từng mã không tạm thời đã biết (`REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_OPENSPEC_INVALID_OUTPUT`, `REQUEST_SPEC_RUN_OUT_OF_SCOPE`, `REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_QUOTA_EXHAUSTED`, `REQUEST_BASE_REF_NOT_FOUND`, `REQUEST_PLAN_REQUEST_MOVED`, rỗng, mã lạ) sai.
- `CanMove`: mọi cặp trạng thái (bảng kỳ vọng đầy đủ 7×7).
- `PlanAutoKey`: lượt 1, lượt 2, lượt 3; anchor dài làm vượt 128 thì lỗi.
- `PlanAutoPolicyFromEnv`: rỗng, hợp lệ, `MaxExtraAttempts=5` bị kẹp về 2 kèm cảnh báo, duration sai.
- `WantsAutoPlan`: 11 loại × trigger {`analysis_approved`, `type_confirmed`, `plan_revision`, `analysis_ready`, `information_provided`} × `PlanTaskID` rỗng/có × `to` {`planning`, `analyzing`}. Kỳ vọng đúng: `change_request`, `bug`, `refactor`, `security`, `performance` với `analysis_approved`; `task`, `docs`, `ops_request` với `type_confirmed`; tất cả còn lại sai (đặc biệt `hotfix`, `spike`, `question`, mọi `plan_revision`, và mọi ca đã có `PlanTaskID`).
- `cd backend-go/services/request-service && go test ./internal/domain/... -run 'PlanAuto|EngineSettings' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Một bảng mã tạm thời, có test theo từng mã. (`TestIsTransientPlanRunCode`: từng mã đúng và 19 mã sai)
- [x] `WantsAutoPlan` đúng bảng 11 loại × trigger; `hotfix`, `spike`, `question` và replan không bao giờ được nhận. (`TestWantsAutoPlan`: ma trận 11 loại x trigger x `PlanTaskID` x `to`; `hotfix`, `spike`, `question`, `plan_revision` và mọi ca đã có `PlanTaskID` đều sai)
- [x] Máy trạng thái job có test đầy đủ; trạng thái cuối không đi tiếp. (`TestPlanAutoStateCanMove`: bảng 7x7 đầy đủ, ba trạng thái cuối không đi tiếp)
- [x] Giá trị env lạ không làm service dừng, có cảnh báo. (`TestPlanAutoPolicyFromEnv`: giá trị lạ giữ mặc định hoặc kẹp, trả cảnh báo, không có đường dừng service)
- [x] `go build ./...` và test miền xanh. (`go build ./...`, `go vet ./internal/domain/`, `go test ./internal/domain/... ./internal/usecase/... ./internal/adapter/grpc/...` xanh ngày 2026-10-20)

## Rủi ro và lưu ý

- Thêm trường vào `ProjectEngineSettings` đổi struct dùng ở nhiều test; chạy `gitnexus_impact` cho `ProjectEngineSettings` và `NewProjectEngineSettings` trước khi sửa.
- Nếu một mã tạm thời mới xuất hiện ở worker khác, bảng này là nơi duy nhất cần thêm; ghi chú điều đó ở đầu bảng.

## Ghi chú triển khai

**Hoàn thành 2026-10-20.** Đường dẫn dưới `backend-go/services/request-service/`.

- File mới: `internal/domain/plan_auto.go` (trạng thái job, `CanMove`, `IsActive`/`IsFinal`, `PlanAutoOrigin`, `PlanAutoJob`, `PlanAutoKey` và ba hàm anchor, bảng mã tạm thời `IsTransientPlanRunCode`, `WantsAutoPlan`), `internal/domain/plan_auto_policy.go` (`PlanAutoPolicy`, `DefaultPlanAutoPolicy`, `PlanAutoPolicyFromEnv`), `internal/domain/plan_auto_errors.go` (`ErrPlanAutoDisabled`), `internal/domain/plan_auto_test.go`. Sửa `internal/domain/engine_settings.go` (`PlanAutoGenerate`, `NewProjectEngineSettings` đặt `true`) và `engine_settings_test.go` (thêm một test).
- Lệch so với spec: (1) `PlanAutoKey` trả `(string, error)` thay vì chỉ chuỗi, vì spec đòi lỗi miền khi vượt 128 ký tự; lỗi là `KindInvalidArgument` mã `REQUEST_PLAN_AUTO_KEY_INVALID`. (2) Tách policy ra `plan_auto_policy.go` và lỗi ra `plan_auto_errors.go` (không ghi thêm vào `plan_errors.go` để không đụng file dùng chung). (3) `WantsAutoPlan` với `analysis_approved` đòi thêm `flow.AnalysisGate != none` (loại không có Analysis như `task` không bao giờ đi qua trigger này), nên ma trận 11 loại cho kết quả đúng như bảng kỳ vọng của spec.
- Mã `REQUEST_PLAN_AI_UNAVAILABLE` ở cấp run được tính là tạm thời (quyết định đã chốt). `REQUEST_PLAN_AUTO_DISABLED` và `REQUEST_PLAN_COMMIT_FAILED` có hằng `PlanAutoCode*`.
- Lệnh đã chạy: `go test ./internal/domain/... -count=1` xanh; `go vet ./internal/domain/` xanh; `go build ./...` xanh. Các test: `TestIsTransientPlanRunCode` (6 mã đúng, 19 mã sai gồm rỗng và mã lạ), `TestPlanAutoStateCanMove` (bảng 7x7 đầy đủ), `TestPlanAutoKey`, `TestPlanAutoPolicyFromEnv` (mặc định, hợp lệ, kẹp 5 về 2 và -3 về 0 kèm cảnh báo, 5 giá trị sai giữ mặc định), `TestWantsAutoPlan` (11 loại x 2 cỡ x `PlanTaskID` x 2 trạng thái đích x 5 trigger), `TestErrPlanAutoDisabled`, `TestNewProjectEngineSettingsPlansAutomaticallyByDefault`.
- Blast radius (grep, GitNexus không có): `ProjectEngineSettings` thêm trường; nơi dựng struct bằng literal (không qua `NewProjectEngineSettings`) sẽ có `PlanAutoGenerate=false`: `internal/usecase/engine_settings_defaults.go:29` (mặc định theo deployment) và `internal/usecase/manage_project_engine_settings.go:72` (dự án chưa có dòng). **Task 04 phải đặt `true` ở cả hai** (và cho `PlanAutoGenerate *bool` giữ giá trị đã lưu), nếu không dự án mới lưu cài đặt engine sẽ tắt tính năng ngầm. `NewProjectEngineSettings` chỉ được gọi ở `manage_project_engine_settings.go:105` và contract test. Không có người gọi nào bị đổi hành vi hiện tại vì chưa có consumer đọc trường mới.
- Chưa kiểm: không có gì cần stack.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
