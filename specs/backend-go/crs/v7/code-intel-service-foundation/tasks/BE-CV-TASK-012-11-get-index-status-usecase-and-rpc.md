# BE-CV-TASK-012-11: Use case và handler `GetIndexStatus` (offline, refresh, job đang chạy, ghi lại binding)

**From Solution:** BE-CV-SOL-012-index-status-aggregation
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_index_status.go`, `internal/adapter/grpc/index_status_server.go` (+ `_test.go`) (mới)
**Depends on:** BE-CV-TASK-012-05, 012-09, 012-10, 012-02
**Status:** [x] DONE

---

## Context

Hợp đồng §3.1 (`GetIndexStatus`: `refresh`, quyền `read`, kênh `codeIntel.status`). Kết quả `IndexStatus` đơn (PQ-08 (2)), không phong bì. `OFFLINE` không phải lỗi.

## Việc cần làm

1. `GetIndexStatus.Execute(ctx, sel, refresh)`: `ResolveTarget` → kiểm `IsDevServerConnected` rẻ (hoặc dựa lỗi probe) → nếu offline: trả `IndexStatus{overall: OFFLINE}` từ `last_status`/`last_status_at` (rỗng nếu chưa có); ngược lại cache/probe → `DecodeAgentStatus` → `ComputeOverall` với `ActiveJob` từ `ReindexJobRepository.ListByBinding(limit=1)` (job `queued/running`).
2. Cập nhật binding: `gitnexus_repo`, `codegraph_path`, `index_scope` (DB) khi khác bản đã lưu (`Upsert` với CAS version); khi `index_scope` đổi thì `SnapshotRepository.DeleteByBinding`.
3. Lỗi probe → `IndexStatus{overall: UNKNOWN, error_code: <mã thô>}` (không lỗi RPC). Chỉ lỗi phân giải/quyền mới là lỗi RPC.
4. Handler: ánh xạ domain → proto `IndexStatus` (bỏ đường dẫn tuyệt đối; `binding` chỉ `id, project_id, repo_id, worktree_id, index_scope, version`); `tools` theo thứ tự cố định `gitnexus`, `codegraph`.
5. RPC khác (không phải `GetIndexStatus`) khi dev server offline dùng hàm chung `RequireOnline(target)` trả `CODEINTEL_DEV_SERVER_OFFLINE` (`KindUnavailable`).

## Kiểm thử

- Unit với fake: 9 trạng thái end-to-end qua handler; offline trả `last_status`; probe lỗi → `UNKNOWN`; job đang chạy → `BUILDING`; `refresh` bỏ qua cache; `index_scope` đổi xoá snapshot; `RequireOnline` trả `Unavailable`.
- Integration (hai dialect): luồng `GetIndexStatus` với DB thật và prober giả.
- `go test ./services/code-intel-service/... -run GetIndexStatus`

## Tiêu chí hoàn thành

- [x] Tiêu chí SOL-012-index-status mục 4 đạt.
- [x] Không đường dẫn tuyệt đối/`dev_server_id` trong phản hồi.

## Rủi ro và lưu ý

- Gateway/FE phải đổi từ `IndexStatus[]` sang object (PQ-08); ghi trong README service.
