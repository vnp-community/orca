# BE-CV-TASK-040-22: Bộ kiểm hợp đồng cho 10 kênh ghi/trạng thái và stream (tenant, dialect, rò goroutine, `send`)

**From Solution:** BE-CV-SOL-040-codeintel-write-and-stream-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_state_conformance_test.go` (mới), `channels_codeintel_stream_e2e_test.go` (mới)
**Depends on:** TASK-040-16..21
**Status:** [ ] TODO

---

## Context

Chốt chất lượng nhóm ghi + stream theo CR-040 mục 4: khoá cấm, tenant, `DeviceID`, hai dialect, không rò goroutine, stream không rò tenant.

## Việc cần làm

1. Bảng 10 kênh ghi (liệt kê cứng) với fake client: khoá cấm (`tenantId, userId, role, deviceId, devServerId, workspaceRoot, repo, args, command, cypher`) => `INVALID_PARAMS` nêu tên trường, fake không được gọi; deadline 8 s; `Aborted` => `VERSION_CONFLICT`; `DeviceID != ""` => `NOT_AUTHORIZED` ngoại trừ `settings.get`.
2. `TestCodeIntelWriteChannels_NeverRetry`: fake đếm lời gọi; lỗi `Unavailable` => đúng 1 lời gọi.
3. E2E mạng thật (httptest + `coder/websocket` client, fake `CodeIntelServiceClient` có `StreamCodeIntelEvents` bufconn): native và session-client: gọi `settings.get` (ack/kết quả), `subscribe` (ack `null`), nhận khung `changed` và `reindexProgress` đúng tên kênh (native) / có `event` (session-client); đóng socket => stream phía fake bị huỷ; số goroutine về mức cũ.
4. Tenant: fake stream của tenant A nhận metadata `x-orca-tenant-id=A`; kiểm khẳng định gateway không lọc nội dung (trách nhiệm service; ghi chú test: "service phải có test lọc tenant, CR-072").
5. Subscribe lần hai trên cùng socket thay thế lần đầu (fake thấy huỷ), không rò.
6. `send` tới kênh ghi: gọi qua `handleSend` => lỗi chỉ log; test ghi nhận hành vi (tài liệu hoá rủi ro, không sửa).

## Kiểm thử

`go test ./internal/adapter/wscompat/ -run 'CodeIntel(State|Stream)(Conformance|E2E)' -race -count=1`.

## Tiêu chí hoàn thành

- [ ] 10 kênh + stream qua đủ sáu nhóm kiểm.
- [ ] Không rò goroutine sau đóng/thay thế.
- [ ] Hai dialect cùng hoạt động.

## Rủi ro và lưu ý

- Test mạng chậm/flaky nếu dùng sleep; dùng kênh đồng bộ và `eventually` có hạn.
- Cô lập tenant thật nằm ở service; test gateway chỉ chứng minh metadata đúng.
