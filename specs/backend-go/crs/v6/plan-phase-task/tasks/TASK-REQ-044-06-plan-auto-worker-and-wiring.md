# TASK-REQ-044-06: `PlanAutoWorker` (bắt đầu, quan sát, commit, thử lại có giới hạn) và wiring `wire_plan_auto.go`

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/plan_auto_worker.go`, `plan_auto_worker_start.go`, `plan_auto_worker_observe.go`, `plan_auto_worker_commit.go` (mới), các `plan_auto_worker_*_test.go` tương ứng (mới), `backend-go/services/request-service/cmd/server/wire_plan_auto.go` (mới), `cmd/server/main.go` (sửa), `cmd/server/wire_plan_auto_test.go` (mới)
**Depends on:** TASK-REQ-044-03, TASK-REQ-044-05
**Status:** [ ] TODO

---

## Context

- Bảng trạng thái của worker ở solution mục 2.4 là nguồn sự thật cho task này; luật mã tạm thời ở `domain.IsTransientPlanRunCode` (task 01).
- Vòng lặp mẫu: `ClarificationSweeper.RunLoop(ctx, interval, batch)` (`wire_artifact.go:135`) và `RecoverInterruptedAnalysisRuns.RunRecoveryLoop` (`recover_interrupted_analysis_runs.go:58`).
- Danh tính: người báo cáo (`Request.ReporterID`). `GeneratePlan.ExecuteForSystem` và `CommitPlan.ExecuteForSystem` (task 03) tự đặt user, nhưng worker vẫn cần `tenant.WithTenantID` từ `job.TenantID`.
- Phân loại lỗi lúc bắt đầu theo `apperrors.Kind` (`common/apperrors/apperrors.go:20-40`): `KindFailedPrecondition`/`InvalidArgument`/`NotFound`/`PermissionDenied` là vĩnh viễn (`skipped`), trừ `REQUEST_ANALYSIS_BUSY` (hoãn); mọi loại khác là hạ tầng (thử lại trong cửa sổ).
- `wirePlan` chạy trước `wireArtifact` (`main.go:191`, `:202`); `busConsumer` có ở `main.go`; `envBool` ở `wire_artifact.go:54`; `wire_plan.go:76` ghi cảnh báo khi approval tắt và `Commit` nil.

## Việc cần làm

1. `PlanAutoWorker` (struct và `RunOnce(ctx) (int, error)`, `RunLoop(ctx, interval, batch)`): `ClaimDue(owner, policy.LeaseTTL, batch)` rồi xử lý từng job (goroutine riêng, có timeout theo `LeaseTTL`). Mỗi bước: tải Request; áp luật `superseded` (không còn `planning`; hoặc `PlanTaskID != ""` khi `origin` là `event`/`retry`; hoặc `ListRecent` có run Plan mới hơn run của job); rồi chạy bước theo `state`.
2. `plan_auto_worker_start.go` (`pending`): `GeneratePlan.ExecuteForSystem{RequestID, IdempotencyKey: PlanAutoKey(anchor, attempt)}`. Thành công: nếu kết quả `Status == failed` (khoá khớp một run đã thất bại) → xử lý như bước quan sát với run đó; ngược lại `running` + `run_id`. Bận (`REQUEST_ANALYSIS_BUSY`): ở `pending`, `next_check_at += BusyDelay`; quá `StartWindow` kể từ `created_at` → `gave_up(REQUEST_ANALYSIS_BUSY)`. Lỗi vĩnh viễn → `skipped(code)`. Lỗi khác → `pending`, `+= RetryDelay`, cùng cửa sổ rồi `gave_up(code)`.
3. `plan_auto_worker_observe.go` (`running`): `runs.Get`. `running`: `next_check_at = now + PollInterval`. `succeeded`: `committing`. `failed`: nếu `IsTransientPlanRunCode(code)` và `Attempt < MaxAttempts` và Request còn `planning`: `Attempt++`, xoá `run_id`, `pending`, `+= RetryDelay`, ghi `reason_code`; ngược lại `gave_up(code)` (mã rỗng thì `REQUEST_SOLUTION_INTERNAL_ERROR`). Run bị xoá (không tìm thấy) → `superseded`.
4. `plan_auto_worker_commit.go` (`committing`): `DecodePlanProposal(derefStr(run.RawOutput))` (lỗi giải mã → `gave_up(REQUEST_PLAN_COMMIT_FAILED)` vì dữ liệu hỏng, không chạy lại AI); `CommitPlan.ExecuteForSystem`. `AlreadyExists` hoặc thành công → `done`. Lỗi có mã `REQUEST_PLAN_*` → `gave_up(code)`. `REQUEST_STATE_STALE` mà Request đã `awaiting_plan_approval` có `PlanTaskID` → `done`. Lỗi khác → `CommitAttempts++`; đủ `MaxCommitAttempts` → `gave_up(REQUEST_PLAN_COMMIT_FAILED)`, chưa đủ → `+= RetryDelay`.
5. Mọi chuyển trạng thái dùng `Update(job, owner)`; `false` (mất lease) thì dừng im lặng. Mỗi lần job kết thúc (`done`, `gave_up`, `skipped`, `superseded`) ghi một dòng log có cấu trúc (`request_id`, `origin`, `state`, `reason_code`, `attempt`), không nội dung; nếu bộ đếm `EngineObserver` mở rộng gọn thì thêm `plan_auto_jobs{state,reason}`.
6. `wire_plan_auto.go` `wirePlanAuto(ctx, wg, cfg, log, stores, lifecycle, plan, solution, bus)`: đọc `REQUEST_PLAN_AUTO_GENERATE` (`envBool(..., true)`) và `domain.PlanAutoPolicyFromEnv(os.Getenv)` (log cảnh báo); điều kiện chạy: env bật, `cfg.RequestFlowEnabled`, `bus != nil`, `plan.Commit != nil`; thiếu thì log **một** dòng nêu lý do và trả về không dựng gì. Ngược lại dựng `PlanAutoSwitch`, `EnqueuePlanAuto`, consumer (goroutine theo `wg`, log khi dừng), `PlanAutoWorker` (`RunLoop` theo `wg`, `owner` là `solution.owner`); trả một struct để task 07 gắn `RetryPlanGeneration` và `AutoPlan` vào `PlanUseCases`.
7. `main.go`: gọi `wirePlanAuto` sau `wireArtifact` (cần `busConsumer`); lỗi dựng làm service dừng như các wire khác.

## Kiểm thử

- Test từng bước với fake (đồng hồ giả, repository bộ nhớ): (a) bắt đầu ok → `running`; (b) bận → hoãn rồi `gave_up` sau `StartWindow`; (c) mỗi loại lỗi vĩnh viễn (`NO_DEV_SERVER`→`REQUEST_PLAN_AI_UNAVAILABLE`, `NO_REPO_PATH`, `ENGINE_UNAVAILABLE`, `STATE_STALE`, `NOT_APPLICABLE`) → `skipped` với đúng mã; (d) lỗi hạ tầng → thử lại trong cửa sổ rồi `gave_up`; (e) nhận lại run đang chạy khi người bấm tay chen ngang; (f) run `failed` tạm thời còn lượt → lượt 2 với khoá `plan-auto-<anchor>-2`; hết lượt → `gave_up`; (g) từng mã không tạm thời → `gave_up` ngay và **không** có run thứ hai (đếm lần gọi `StartRun`); (h) `succeeded` → commit → `done`; `AlreadyExists`, `STATE_STALE` đã commit → `done`; lỗi `REQUEST_PLAN_*` → `gave_up`; lỗi task-service 5 lần → `gave_up(REQUEST_PLAN_COMMIT_FAILED)` mà không gọi lại AI; (i) Request rời `planning` hoặc có `PlanTaskID` → `superseded`; job có `origin` giả định thứ ba và `PlanTaskID` có sẵn **không** bị `superseded` vì lý do đó (điểm tựa cho CR-REQ-045); (j) mất lease giữa chừng → không ghi.
- Wiring: bảng điều kiện (env tắt, flow tắt, không bus, không `Commit`) cho kết quả "không dựng", và một ca dựng đủ.
- `cd backend-go/services/request-service && go test ./internal/usecase/... ./cmd/server/... -run 'PlanAuto' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Lỗi không tạm thời không bao giờ sinh run thứ hai (test đếm lần `StartRun`).
- [ ] Lỗi tạm thời sinh đúng tối đa `MaxExtraAttempts` lượt thêm, mỗi lượt khoá mới.
- [ ] Bận không phải lỗi cứng; mọi lỗi vĩnh viễn là `skipped` với mã, không làm consumer lặp.
- [ ] Người bấm tay chen ngang vô hại; job không làm gì thêm khi Plan đã có.
- [ ] Env tắt hoặc thiếu `Commit`: không consumer, không worker, có đúng một dòng log lý do.

## Rủi ro và lưu ý

- Run mồ côi (tạo xong mà ghi `running` thất bại) không có goroutine gia hạn lease nên sẽ thành `REQUEST_SOLUTION_RUN_INTERRUPTED` rồi được thử lại; chưa tốn tiền vì chưa chạy gì. Ghi vào nhận xét mã.
- Vòng thăm dò mỗi instance mỗi 5 giây; chưa đo tải.
- Chạy `gitnexus_impact` cho `PlanUseCases` và `wirePlan` (đổi chữ ký trả về ở task 07).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
