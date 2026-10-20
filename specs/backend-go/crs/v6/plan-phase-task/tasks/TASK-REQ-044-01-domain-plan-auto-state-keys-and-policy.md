# TASK-REQ-044-01: Miền `plan_auto`: trạng thái job, khoá idempotency, mã lỗi tạm thời, chính sách từ env và luật nhận job

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/plan_auto.go` (mới), `internal/domain/plan_auto_test.go` (mới), `internal/domain/engine_settings.go` (sửa)
**Depends on:** (không)
**Status:** [ ] TODO

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

- [ ] Một bảng mã tạm thời, có test theo từng mã.
- [ ] `WantsAutoPlan` đúng bảng 11 loại × trigger; `hotfix`, `spike`, `question` và replan không bao giờ được nhận.
- [ ] Máy trạng thái job có test đầy đủ; trạng thái cuối không đi tiếp.
- [ ] Giá trị env lạ không làm service dừng, có cảnh báo.
- [ ] `go build ./...` và test miền xanh.

## Rủi ro và lưu ý

- Thêm trường vào `ProjectEngineSettings` đổi struct dùng ở nhiều test; chạy `gitnexus_impact` cho `ProjectEngineSettings` và `NewProjectEngineSettings` trước khi sửa.
- Nếu một mã tạm thời mới xuất hiện ở worker khác, bảng này là nơi duy nhất cần thêm; ghi chú điều đó ở đầu bảng.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
