# BE-CV-TASK-040-16: Kênh `codeIntel.reindex` và `codeIntel.reindexStatus`

**From Solution:** BE-CV-SOL-040-codeintel-write-and-stream-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_state_reindex.go` (mới), `channels_codeintel_state_reindex_test.go` (mới), `channels_codeintel_register.go`
**Depends on:** TASK-040-07; stub `RequestReindex`, `GetReindexJob` (BE-CV-SOL-021-agent-collector)
**Status:** [ ] TODO

---

## Context

UI-API 3.1: `reindex` (sel, `mode?`) => `{jobId, status, mode, trigger}`, quyền `reindex`, 8 s, trả ngay; tiến trình qua push, dự phòng `reindexStatus` (sel, `jobId`) => `ReindexJob` (4.1: `status` queued|running|succeeded|failed|cancelled; `percent` số hoặc `null`). PQ-16: UI chỉ gửi `mode`; `trigger='manual'` do service đặt. PQ-24: `GetReindexJob` chạy cả khi cờ tắt. Lỗi: `CODEINTEL_REINDEX_IN_PROGRESS | {"jobId","stage"}`, `CODEINTEL_REINDEX_COOLDOWN | {"retryAfterSeconds"}` (đi qua nguyên).

## Việc cần làm

1. `reindexArgs{sel, Mode string}`: `mode` rỗng hoặc incremental|full; map sang enum/chuỗi proto; `Core.RequestReindex`; kết quả `{jobId,status,mode,trigger}` từ `resp.GetJob()` (encoder `encodeCodeIntelWire`, không envelope).
2. `reindexStatusArgs{sel, JobID}`: `jobId` bắt buộc (`checkOpaqueID`); `Core.GetReindexJob`; trả `ReindexJob`; `percent` vắng => `null`.
3. Đăng ký với `Timeout 8 s`; không retry; không chặn khi cờ tắt (service quyết).
4. Không nhận `trigger`, `tools`, `ifStale`, `expectHead`, `tiers` từ client (khoá lạ bị từ chối).

## Kiểm thử

- Validate: `mode` `weekly`; `jobId` rỗng/129 ký tự; `trigger` gửi lên => `INVALID_PARAMS`.
- Fake: `RequestReindex` nhận selector đúng, `mode` rỗng => không điền; `AlreadyExists`/`REINDEX_IN_PROGRESS` đi qua; cooldown `data.retryAfterSeconds`.
- Deadline 8 s; `Unimplemented` => `UNAVAILABLE`.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelState(Reindex)'`.

## Tiêu chí hoàn thành

- [ ] Hai kênh đúng shape; không trường ngoài hợp đồng.
- [ ] Mã `REINDEX_*` giữ nguyên cho client.

## Rủi ro và lưu ý

- Gateway không có `CancelReindex` (O-17): không thêm kênh.
- Job id của tenant khác phải ra `NOT_FOUND`/`NOT_AUTHORIZED` ở service (test thuộc service).
