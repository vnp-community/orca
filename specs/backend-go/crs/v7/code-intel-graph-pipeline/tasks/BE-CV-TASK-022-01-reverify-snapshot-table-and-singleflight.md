# BE-CV-TASK-022-01: Re-verify bảng `graph_snapshots`, `x/sync` và tên cột

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P0
**Service:** `code-intel-service`
**File:** (không sửa mã) `backend-go/services/code-intel-service/migrations/{postgres,mysql}/`, `go.mod`
**Depends on:** SOL-011 (migration 0002)
**Status:** [x] DONE

---

## Context

Hợp đồng đánh dấu migration do CR-011 sở hữu và số có thể dịch. Task chốt tên cột và phụ thuộc trước khi code.

## Việc cần làm

1. Đọc migration `0002_*` thật: có `head_commit`, `etag`, `total_count`, `schema_version`, UNIQUE đúng PQ-15 và chỉ mục `(expires_at)`, `(tenant_id, repo_binding_id, created_at)`; ghi chênh lệch.
2. Xác nhận `golang.org/x/sync` trong `go.mod` của service (`go list -m golang.org/x/sync`); nếu thiếu, thêm trực tiếp (đã dùng ở `api-gateway`, `git-gateway-service`).
3. Xác nhận `dbcapability` (MySQL không RETURNING/JSONB/RLS).
4. Xác nhận SOL-021 đã có `ViewReader`; nếu chưa, dùng interface giả.

## Kiểm thử

- `go list -m golang.org/x/sync`; `grep -n head_commit migrations/*/0002*.sql`; dán kết quả vào PR.

## Tiêu chí hoàn thành

- [x] Bảng chênh lệch cột (nếu có) ghi vào PR. - [x] `x/sync` có trong `go.mod`.

## Rủi ro và lưu ý

- Nếu migration dùng `commit` thì dừng và báo chủ SOL-011 (PQ-15).
