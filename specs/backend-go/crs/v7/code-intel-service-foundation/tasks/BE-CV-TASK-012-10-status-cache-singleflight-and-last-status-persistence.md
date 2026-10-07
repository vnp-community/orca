# BE-CV-TASK-012-10: Cache trạng thái 15 s, `singleflight`, lưu `last_status`, prober `RelayByDevServer`

**From Solution:** BE-CV-SOL-012-index-status-aggregation
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/index_status_cache.go`, `internal/adapter/grpcclient/agent_status_prober.go` (+ `_test.go`) (mới)
**Depends on:** BE-CV-TASK-012-04, 012-08, 011-07, 011-09
**Status:** [x] DONE

---

## Context

SOL-012-index-status mục 2.D. `RelayByDevServerRequest{dev_server_id, method, params_json}` (`infrafleet.proto:866`); lỗi `INFRA_DEV_SERVER_NOT_CONNECTED` là `FailedPrecondition`. Timeout Go 30 s; service đặt 10 s (`CODEINTEL_STATUS_TIMEOUT`). `golang.org/x/sync/singleflight` (kiểm `go.mod`; nếu chưa có, thêm; chưa kiểm chứng đã là phụ thuộc gián tiếp).

## Việc cần làm

1. `agent_status_prober.go`: `Probe(ctx, devServerID, workspaceRoot, baseRef string) (map[string]any, error)`: gọi `RelayByDevServer` method `codeintel.status` với `params_json` chỉ `{"workspaceRoot", "baseRef"?}` (không khoá lạ); `context.WithTimeout(StatusTimeout)`; phân loại lỗi: `FailedPrecondition`+`INFRA_DEV_SERVER_NOT_CONNECTED` → `ErrDevServerOffline`; còn lại → lỗi thô kèm mã (trước SOL-023: `INFRA_AGENT_EXEC_FAILED`).
2. `index_status_cache.go`: cache TTL 15 s khoá `(tenant, bindingID)` (đồng hồ tiêm), `singleflight` theo khoá; lời gọi dẫn đầu chạy bằng `context.WithoutCancel(ctx)` + timeout riêng; người đợi huỷ ctx không huỷ thăm dò; `Invalidate(tenant, bindingID)`; giới hạn kích thước (mặc định 2048 mục).
3. Lưu: sau thăm dò thành công `SaveStatusCache(bindingID, statusJSON, now)`; `statusJSON` ≤ 64 KiB (bỏ `languages`, `pendingChanges` rồi cắt thô; không lỗi).
4. Metric/log: chỉ `binding_id`, `duration`, kết quả; không đường dẫn ở INFO.
5. Cổng `AgentCallGate` (013) tiêm, nil → no-op.

## Kiểm thử

- Unit (đồng hồ giả + prober giả): 20 goroutine → 1 lần probe; `refresh` bỏ qua; hết TTL gọi lại; huỷ ctx người đợi không huỷ probe; lỗi không bị cache (hoặc cache âm 2 s — chọn và ghi); `Invalidate`.
- Integration (hai dialect): `SaveStatusCache` ≤ 64 KiB và cắt khi lớn.
- `go test ./services/code-intel-service/internal/... -run 'StatusCache|Probe'`

## Tiêu chí hoàn thành

- [x] Đúng một `RelayByDevServer` cho 20 lời gọi đồng thời.
- [x] `last_status` lưu và đọc lại được ở hai dialect.

## Rủi ro và lưu ý

- Cache theo bản sao; nhiều replica gọi nhiều hơn (Q3 SOL).
