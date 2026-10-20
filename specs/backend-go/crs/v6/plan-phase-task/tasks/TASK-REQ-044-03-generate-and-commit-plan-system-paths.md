# TASK-REQ-044-03: `GeneratePlan.ExecuteForSystem` với khoá idempotency, `CommitPlan.ExecuteForSystem` và commit idempotent khi có cuộc đua

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/generate_plan.go` (sửa), `internal/usecase/commit_plan.go` (sửa), `internal/usecase/generate_plan_system_test.go`, `internal/usecase/commit_plan_system_test.go`, `internal/usecase/commit_plan_race_test.go` (mới)
**Depends on:** TASK-REQ-044-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- `GeneratePlan.Execute` (`generate_plan.go:70`) đòi user trong `ctx`, gọi `AuthorizePlan` (`:86`), dựng `AnalysisRun` **không có `IdempotencyKey`** (`:108-112`), và `uc.spawner.Spawn(res.Run)` sau transaction (`:140`).
- `GenerateSolution.PrepareForSystem`/`ExecuteForSystem` (`generate_solution.go:150-175`) là tiền lệ: bỏ phân quyền, `actor` là người báo cáo, đặt user vào `ctx` để tìm dev server (project-service từ chối lời gọi chỉ có tenant), `skipWithoutConnection` (`solution_regenerator.go`) đổi lỗi vĩnh viễn thành "bỏ qua".
- `findBlocking` trả run đã có khi khoá khớp (mọi trạng thái) hoặc có run Plan đang `running` (`adapter/postgres/analysis_run_repository.go:56`, MySQL `:68`).
- `CommitPlan.Execute` (`commit_plan.go:66`): `replay` khi `awaiting_plan_approval` và có `PlanTaskID` (`:82`); `record` (`:128`) trả `REQUEST_STATE_STALE` khi `cur.Status != planning` (`:134`), kể cả khi Plan vừa được bên kia commit. `AuthorizePlan` ở `:78`.
- Hai worker lưu `EncodePlanProposal` vào `analysis_runs.raw_output` qua `finishPlanRun` (`run_plan_generation.go`), nên `DecodePlanProposal` đọc được cả run `agent_proposal` (`get_plan_proposal.go:99-104`).

## Việc cần làm

1. `GeneratePlanInput.IdempotencyKey string` (RPC công khai không điền). Tách thân `Execute` thành `run(ctx, in, system bool)`; `Execute` gọi `run(..., false)` y như cũ; thêm `ExecuteForSystem(ctx, in)` gọi `run(..., true)`.
2. Nhánh `system`: sau khi đọc Request, nếu `ctx` không có user thì `actor = req.ReporterID` và `ctx = tenant.WithUserID(ctx, actor)` (lỗi `ErrRequestReporterRequired` nếu Request không có người báo cáo); bỏ `AuthorizePlan`; giữ `checkPlannable`, `conns.ResolveForProject`, `modeFor`, `StartRun`. `run.IdempotencyKey = &key` khi khác rỗng; `run.ActorID = actor`.
3. `GeneratePlanResult` thêm `Status domain.RunStatus` và `ErrorCode string` của run trả về (để worker biết khi khoá khớp một run đã `failed`).
4. `CommitPlan`: tách `Execute` thành `run(ctx, in, system bool)`; thêm `ExecuteForSystem` (bỏ `AuthorizePlan`, `actor = req.ReporterID`, đặt user vào `ctx`). `record`: nếu `cur.Status == awaiting_plan_approval && cur.PlanTaskID == tree.PlanTaskID` thì trả `nil` (đã commit bởi bên kia) thay vì `REQUEST_STATE_STALE`; các trạng thái khác giữ `ErrPlanWrongState`.
5. Không đổi chữ ký hay hành vi của gRPC `GeneratePlan`/`CommitPlan` (`adapter/grpc/server_plan.go`), ngoài việc `CommitPlan` giờ chịu được cuộc đua.

## Kiểm thử

- `GeneratePlan.ExecuteForSystem`: (a) không có user trong `ctx` vẫn chạy, run có `ActorID` là người báo cáo và `IdempotencyKey` đã đặt; (b) gọi lần hai cùng khoá trả cùng `RunID`, `Created=false`, không spawn thêm; (c) khoá khớp một run `failed` trả `Status=failed` và `ErrorCode`; (d) khoá mới sau lỗi tạo run mới; (e) không dev server → `REQUEST_PLAN_AI_UNAVAILABLE` và không ghi dòng nào; (f) `Execute` thường vẫn đòi user và vẫn gọi `AuthorizePlan` (test hiện có xanh).
- `CommitPlan.ExecuteForSystem`: proposal hợp lệ commit được không cần quyền người gọi; proposal giải mã từ `RawOutput` của run `agent_proposal` (dùng helper dựng proposal của `openspec_engine_fakes_test.go`) qua `ValidatePlanProposal`; `AlreadyExists` từ task-service vẫn thành công.
- Cuộc đua (`commit_plan_race_test.go`): hai lời gọi (một thường, một hệ thống) cùng đọc `planning`, bên thua ở `record` thấy Request đã `awaiting_plan_approval` với đúng `PlanTaskID` → trả kết quả thành công, không `REQUEST_STATE_STALE`; một Plan, một Approval (kiểm với fake writer và fake approvals); `PlanTaskID` khác thì vẫn lỗi.
- `cd backend-go/services/request-service && go test ./internal/usecase/... -run 'GeneratePlan|CommitPlan' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Đường hệ thống không đi qua `AuthorizePlan` và không có RPC công khai nào truy cập được nó.
- [x] Gọi lại cùng khoá không tạo run thứ hai; khoá mới tạo run mới.
- [x] Commit chịu được cuộc đua: một Plan, một Approval, không lỗi hiển thị cho bên thua.
- [x] `RawOutput` của cả hai engine commit được.
- [x] Test hiện có của `GeneratePlan`, `CommitPlan` xanh.

## Rủi ro và lưu ý

- Blast radius: `GeneratePlan.Execute` và `CommitPlan.Execute` được gRPC và nhiều test gọi; chạy `gitnexus_impact` cho cả hai và `record`, báo mức rủi ro.
- Không mở rộng `ActorKind` ở đây (solution S9): transition và Approval ghi người báo cáo làm `ActorID`.
- Hotfix (`PlanSingleTask`) có đường commit riêng (`bindHotfixFixTask`); không đụng.

## Ghi chú triển khai

**Hoàn thành 2026-10-20.** Đường dẫn dưới `backend-go/services/request-service/`.

- Sửa `internal/usecase/generate_plan.go`: `GeneratePlanInput.IdempotencyKey`, `GeneratePlanResult.Status/ErrorCode`, thân `Execute` tách thành `run(ctx, in, system)`, thêm `ExecuteForSystem`. Nhánh system đặt `actor = req.ReporterID` và `tenant.WithUserID` (project-service từ chối lời gọi chỉ có tenant), bỏ `AuthorizePlan`, giữ `checkPlannable`/`modeFor`/`StartRun`. Sửa `internal/usecase/commit_plan.go`: `run(ctx, in, system)`, `ExecuteForSystem`, và `record` coi Request đã `awaiting_plan_approval` với đúng `PlanTaskID` là thành công (bên thua cuộc đua không thấy `REQUEST_STATE_STALE`; `PlanTaskID` khác hoặc Request đã rời `planning` vẫn lỗi). gRPC `GeneratePlan`/`CommitPlan` không đổi chữ ký; không RPC công khai nào gọi `ExecuteForSystem`.
- Test mới: `generate_plan_system_test.go` (7 ca: không caller, dev server tra bằng người báo cáo, cùng khoá trả cùng run và không spawn thêm, khoá khớp run `failed` trả `Status=failed`+mã, khoá mới tạo run mới, không dev server không ghi gì, thiếu người báo cáo, `Execute` thường vẫn đòi user và `AuthorizePlan`), `commit_plan_system_test.go` (commit không cần quyền, validate như đường công khai, `AlreadyExists`, `RawOutput` của run `agent_proposal` giải mã rồi commit được), `commit_plan_race_test.go` (bên thua `record` thành công; một Plan, một Approval, một `plan.generated`; `PlanTaskID` khác hoặc Request rời `planning` vẫn `REQUEST_STATE_STALE`).
- Lệnh: `go test ./internal/usecase/ -run 'GeneratePlan|CommitPlan' -count=1` xanh; toàn bộ `go test ./...` của request-service xanh.
- Lệch spec: không có (tên file test đúng danh sách). Blast radius (grep): `GeneratePlan.Execute` gọi bởi `adapter/grpc/server_plan.go` và test; `CommitPlan.Execute` bởi `server_plan.go` và test; `record` chỉ trong `commit_plan.go`. Hotfix (`bindHotfixFixTask`) không đụng.
- Chưa kiểm: không có gì cần stack. Test dùng `refusingPlanAuth` (nhúng interface `PlanActorAuthorizer`) để chứng minh đường system không hỏi quyền; nếu agent CR-045 đổi interface thì test vẫn biên dịch vì chỉ ghi đè `AuthorizePlan`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
