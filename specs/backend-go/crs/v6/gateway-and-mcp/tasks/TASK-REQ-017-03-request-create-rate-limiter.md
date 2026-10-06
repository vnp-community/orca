# TASK-REQ-017-03: Hạn mức tạo Request qua MCP (`MCP_REQUEST_CREATE_PER_HOUR`)

**From Solution:** BE-REQ-SOL-017
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/request_create_limit.go` (mới), `.../request_create_limit_test.go` (mới), `.../config.go`, `.../executor.go`
**Depends on:** TASK-REQ-017-02
**Status:** `[ ] TODO`

---

## Context

- `scm_rate_limit.go`: `scmLimiter`, token bucket theo phút, khoá `tenant + "\x00" + group`, `maxRateBuckets = 10000`, `evict`. Dùng ở `executor.go:289` `e.scmLimit.allow(...)` và trả `&ToolError{"RATE_LIMITED", ...}`.
- `config.go:36-47` `ApplyEnv`: `MCP_SCM_RATE_PER_MIN` (0 thành -1 tắt). Cùng mẫu cho biến mới.
- CR-REQ-017 mục 2.4: lớp 2 theo `(tenant, user, ClientName)`, mặc định 20 mỗi giờ, áp cho `request_create` và `request_spawnChild`.
- `ToolError` ở `pty_tools_config.go:133`.

## Việc cần làm

1. `request_create_limit.go`: `type requestCreateLimiter struct{...}` cửa sổ giờ (token bucket với `perHour`, hồi `perHour/3600` mỗi giây), khoá `tenant\x00user\x00client`, bộ nhớ chặn bằng `maxRateBuckets` và `evict`, đồng hồ tiêm được (`now func() time.Time`). `allow(tenant, user, client string) (bool, time.Duration)`; `perHour <= 0` thì luôn cho phép.
2. `config.go`: thêm `RequestCreatePerHour int` (mặc định 20); `ApplyEnv` đọc `MCP_REQUEST_CREATE_PER_HOUR` (số nguyên không âm, sai thì lỗi `MCP_REQUEST_CREATE_PER_HOUR: "x" is not a non-negative integer`; `0` thành `-1`). Cập nhật mô tả comment của `ApplyEnv`.
3. `executor.go`: tạo limiter trong `NewExecutor` từ `cfg.RequestCreatePerHour`; trong `runGuarded` sau `scmLimit`, nếu `spec.Name` là `request_create` hoặc `request_spawnChild` thì `allow(p.TenantID, p.UserID, client)`; vượt thì `&ToolError{"REQUEST_RATE_LIMITED", fmt.Sprintf("too many requests created; retry in %ds", ...)}`.
4. Thêm `MCP_REQUEST_CREATE_PER_HOUR` vào tài liệu cấu hình MCP (`docs/guides/mcp/admin-guide.md`, mục biến môi trường).

## Kiểm thử

- `request_create_limit_test.go` (đồng hồ giả): 20 lần đầu qua, lần 21 từ chối kèm thời gian chờ; sau 3 phút có 1 token; khoá khác nhau độc lập; `0` tắt; bộ nhớ không vượt `maxRateBuckets` khi có nhiều khoá.
- `config_test.go`: biến sai, `0`, mặc định.
- `executor_test.go`: lần thứ 21 trả `REQUEST_RATE_LIMITED`; `request_get` không bị đếm.
- Lệnh: `go test ./internal/adapter/mcpserver/tools/... -run 'RequestCreate|ApplyEnv'`.

## Tiêu chí hoàn thành

- [ ] Lần tạo thứ 21 trong một giờ bị từ chối với mã `REQUEST_RATE_LIMITED`.
- [ ] `MCP_REQUEST_CREATE_PER_HOUR=0` tắt giới hạn.
- [ ] Bộ nhớ chặn; có test.

## Rủi ro và lưu ý

- Theo từng replica gateway; N replica cho N lần. Chốt chặn thật là `REQUEST_PENDING_LIMIT` ở `request-service` (CR-REQ-004, cần thêm).
- Giới hạn tính cả khi RPC sau đó thất bại (đơn giản hơn; ghi rõ trong mô tả tool).
