# BE-CV-TASK-022-06: `not_modified`, giới hạn payload, hạn mức binding/tenant

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/usecase/cached_view_reader.go` (sửa), `.../usecase/snapshot_quota.go` (mới) và test
**Depends on:** TASK-022-05
**Status:** [x] DONE

---

## Context

PQ-12/14: không `data` khi khớp ETag; payload > cấu hình không lưu; 64 MiB/512 MiB.

## Việc cần làm

1. Khớp `if_none_match` → `ResultMeta.not_modified`, không `data`.
2. `Put` bỏ qua payload vượt `CODEINTEL_SNAPSHOT_MAX_BYTES` (vẫn trả).
3. `snapshot_quota.go`: sau `Put` kiểm `TotalBytes`/`TenantBytes`, gọi `EvictOldest`.

## Kiểm thử

- `go test ./internal/usecase/ -run 'NotModified|Quota' -race`; ca: 3,5 MiB không lưu; vượt 64 MiB đưa về ≤ 64 MiB và giữ commit mới nhất mỗi view.

## Tiêu chí hoàn thành

- [x] Các ca xanh.

## Rủi ro và lưu ý

- Chưa đo kích thước thực.
