# BE-CV-TASK-013-04: `ProjectAuthorizer` (`GetProject` → `ListMembers` → OPA, cache 10 s, fail closed)

**From Solution:** BE-CV-SOL-013-authorization-flags-and-audit
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/project_authorizer.go`, `internal/adapter/policyengine/opa.go`, `internal/usecase/project_authorizer_test.go` (mới); `cmd/server/main.go` (nạp OPA)
**Depends on:** BE-CV-TASK-013-01, 013-02, BE-CV-TASK-012-04
**Status:** [ ] TODO

---

## Context

SOL-013-authorization mục 2.B/2.E. Mẫu nạp OPA: `task-service/cmd/server/main.go:293-298` (`policy.NewEvaluator(cfg.OPABundlePath)`, `Warm`, thoát khi lỗi). Danh tính: `tenant.UserID`, `tenant.Role` (rỗng = không phải admin).

## Việc cần làm

1. `policyengine/opa.go`: `Evaluator.Allowed(ctx, input map[string]any) (bool, error)` gọi `policy.Evaluator.Decision(ctx, "data.orca.authz.codeintel.allow", input)`.
2. `main.go`: `NewEvaluator(cfg.OPABundlePath)` + `Warm(ctx, "data.orca.authz.codeintel.allow")`; lỗi → `return fmt.Errorf(...)` (không khởi động).
3. `ProjectAuthorizer.Require(ctx, projectID, action)`: `GetProject` (lỗi `NotFound`/`PermissionDenied` → `CODEINTEL_NOT_AUTHORIZED` đồng nhất); `ListMembers` → role của user (không có dòng → `""`); input OPA `{caller_project_role, caller_global_role, action}`; deny → audit `codeintel.access` (denied, `project:<id>`) rồi `CODEINTEL_NOT_AUTHORIZED`.
4. Cache `(tenant, user, project, action)` TTL `CODEINTEL_AUTHZ_CACHE_TTL` (mặc định 10 s, thêm vào config nếu chưa có), đồng hồ tiêm; cache cả allow và deny; **không** cache lỗi hạ tầng.
5. Lỗi `GetProject`/`ListMembers`/OPA do hạ tầng (`Unavailable`, `DeadlineExceeded`, lỗi eval) → `CODEINTEL_AUTHZ_UNAVAILABLE` (`KindUnavailable`); không bao giờ allow.
6. Không gọi `GetRepo`/`GetWorktree`; cổng `ProjectDirectory` (012-04) không có hai phương thức đó.
7. Điều chỉnh theo kết luận 013-01 nếu `ListMembers` bị hạn chế.

## Kiểm thử

- Unit với fake + bó Rego thật (đường dẫn `backend-go/policy/orca-authz`): bảng `owner|member|admin|none × 8 action`; project tenant khác; project không tồn tại; lỗi hạ tầng; cache TTL (đồng hồ giả); gỡ thành viên sau TTL bị từ chối; admin toàn cục tenant khác bị `GetProject` từ chối; không có `RelayByDevServer` nào được gọi (fake đếm).
- `go test ./services/code-intel-service/internal/usecase/... -run ProjectAuthorizer`

## Tiêu chí hoàn thành

- [ ] Các tiêu chí quyền ở SOL mục 4 đạt.
- [ ] Bó thiếu → service không khởi động.

## Rủi ro và lưu ý

- Mỗi RPC chưa cache = 2 lời gọi nội bộ; cache 10 s giảm tải; chưa đo.
- `x-orca-role` có thể vắng ở đường không cookie/JWT mới: admin không qua (đúng fail closed).
