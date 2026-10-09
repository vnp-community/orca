# TASK-E2E-020: `/readyz` phản ánh cấu hình thật; rà guard nội bộ ở các service

**From Solution:** SOL-E2E-004 · **Resolves:** BUG-E2E-002
**Priority:** P1 · **Service/Area:** request-service và các service dùng internalcaller
**Files:**
- backend-go/services/request-service/cmd/server/*.go (readyz)
- deploy/dev/docker-compose.yml (healthcheck)
- backend-go/common/internalcaller/internalcaller.go (rà ngữ nghĩa token rỗng)
**Depends on:** TASK-E2E-018
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit; healthcheck compose chưa chạy trên stack thật)

---

## Context

Token rỗng hiện chỉ ghi log ERROR và service vẫn "chạy"; lỗi hiện thành `REQUEST_FORBIDDEN` mơ hồ.

## Việc cần làm

- [x] `/readyz` không sẵn sàng khi: bundle OPA không nạp được (`opa_bundle`: `RequestPolicy.Ready` = `Warm`, bắt được bundle bị xoá/hỏng sau khi khởi động trong vòng chu kỳ kiểm 2 giây của evaluator), hai token rỗng (có từ trước), DB không tới được (`postgres`/`mysql` có từ trước, nay ping có hạn 3 giây để DB treo không làm treo `/readyz`).
- [x] Compose: `healthcheck` gọi `/readyz`. Image distroless không có wget/curl nên thêm lệnh con `orca healthcheck` (`cmd/server/healthcheck_command.go`, dùng `common/health.Probe`); `interval 10s`, `timeout 5s`, `retries 3`, `start_period 20s` trong `deploy/dev/docker-compose.yml`.
- [x] Rà `internalcaller.Guard` và mọi nơi dùng; token rỗng luôn từ chối. Hai nơi coi rỗng là "tắt guard" đã sửa và có test (bảng ở Kết quả thực thi).

## Tiêu chí hoàn thành

- [~] Tắt token -> container không healthy trong 60 giây. Tính toán: `start_period 20s` + 3 lần lỗi cách 10s = khoảng 50 giây; **chưa thử trên stack thật** (không được chạy compose trong đợt này).
- [x] Test chứng minh guard rỗng từ chối (request-service có sẵn; thêm cho project, auth, mcp).

## Kiểm thử

Unit test readyz; test guard rỗng.

## Kết quả thực thi

Xong về mã và test; **chưa chạy compose thật**.

**Thay đổi** (`backend-go`): `common/health/health.go` (`Probe`) + `probe_test.go`; `services/request-service/cmd/server/{readiness_dependencies,healthcheck_command}.go`, `main.go`, `store_wiring.go`, `wire_security.go`, `internal/adapter/opaclient/request_policy.go` (`Ready`); `deploy/dev/docker-compose.yml`.
**Cảnh báo triển khai**: container phải dùng binary mới; với binary cũ, `orca healthcheck` sẽ khởi động một service thứ hai. `redeploy-local.sh` build trước khi `up -d` nên an toàn.

**Rà guard nội bộ (`internalcaller`)**

| Nơi | Trước | Sau |
|---|---|---|
| request-service `security_chain.go` (gateway + service token, unary + stream) | token rỗng từ chối (`TestChain_EmptyConfiguredTokensDenyEverything`) | giữ nguyên; thêm `/readyz` |
| project-service `ResolveProjectAccess` | rỗng từ chối nhưng không có test ở tầng wiring | tách `internalCallerGuard`, thêm test (rỗng, sai, đúng, RPC công khai không cần token) |
| auth-service RPC OAuth nội bộ + `SetMcpPatSuspension` | rỗng từ chối | giữ nguyên, thêm test |
| auth-service `ResolveMcpPrincipal` | **rỗng = tắt guard** (RPC mở) | luôn lắp guard, rỗng từ chối; log ERROR khi OAuth bật (`cmd/server/internal_caller_guards.go` + test) |
| mcp-service RPC governance, registry nội bộ (gồm `CallExternalTool`) và `StreamEvents` | **rỗng = tắt guard** | luôn lắp guard unary + stream, rỗng từ chối, log ERROR (`cmd/server/internal_caller_guards.go` + test bufconn cho từng RPC và stream) |
| api-gateway (`ClientInterceptor`, `mcp_principal_resolver`) | chỉ phía gọi, không có guard | không đổi; token rỗng thì không gửi và phía nhận từ chối |
| code-intel-service | gateway có đặt `CODEINTEL_INTERNAL_CALLER_TOKEN` nhưng **service không lắp guard nào** | không sửa (ngoài phạm vi); ghi nhận để theo dõi |

**Tác động**: sau khi triển khai lại auth-service/mcp-service, `ResolveMcpPrincipal` và các RPC governance sẽ từ chối cho tới khi `AUTH_MCP_PRINCIPAL_CALLER_TOKEN` và `MCP_INTERNAL_CALLER_TOKEN` được đặt đồng nhất hai phía (`deploy/dev/scripts/ensure-mcp-env.sh` sinh chúng; `.env` local hiện để trống, MCP đang tắt). Đã cập nhật README auth-service, mcp-service và chú thích cấu hình.

**Test đã chạy**: `go build/vet/test ./...` xanh ở request-service, auth-service, mcp-service, project-service; `common/health` xanh; `common/tracing` còn lỗi có sẵn (`TestTraceEventSpanProcessor_OnEnd_ErrorStatusPublishesFailLevel`). Lỗi nhỏ phát hiện khi chạy e2e: gắn con trỏ nil `*ProjectAccessLookup` vào interface làm panic ở handler; đã sửa bằng `projectAccess.existence()` + test (`wire_project_access_test.go`).
**Chưa làm/kiểm**: healthcheck trên stack thật; cảnh báo cho các service khác chưa rà guard (ngoài danh sách `grep internalcaller`).
