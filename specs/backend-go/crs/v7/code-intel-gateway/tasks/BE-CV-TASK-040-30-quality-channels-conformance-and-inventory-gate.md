# BE-CV-TASK-040-30: Kiểm hợp đồng 20 kênh `quality.*` và cổng cuối 46 kênh (không còn placeholder)

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_conformance_test.go` (mới), `channels_codeintel_all_wired_test.go` (mới)
**Depends on:** TASK-040-23..29, TASK-040-15, TASK-040-22
**Status:** [ ] TODO

---

## Context

Cổng hoàn tất CR-CV-040: 46 kênh (UI-API §3) đều có handler thật; gateway cô lập tenant; lỗi đúng tiền tố; `TestChannelInventory`/`TestToolParity` xanh với dòng `codeIntel.*`.

## Việc cần làm

1. Bảng 20 kênh quality (liệt kê cứng): khoá cấm (gồm `command`, `env`, `cwd`, `timeout`, `args`) => `INVALID_PARAMS` nêu tên, fake không được gọi; deadline đúng bảng (8/20/24 s); `DeviceID` => `NOT_AUTHORIZED`; cờ: `QUALITY_GATE_DISABLED`, `AI_REVIEW_DISABLED`, `DISABLED` đi qua nguyên; không retry; không khoá snake_case; mảng rỗng `[]`.
2. Tenant: metadata gRPC `x-orca-tenant-id/user-id/role` từ `Identity` cho cả 20 kênh.
3. Cỡ: `profile.save` 96 KiB và +1; `trace.confirm|link` 8 KiB và +1; `turn.record` 16 KiB và +1.
4. `TestCodeIntelAllChannelsWired`: dựng `registerCodeIntelChannels` với fake đầy đủ cả hai client; với mỗi kênh catalog (trừ danh sách `pendingRPC` chú thích CR chặn, mục tiêu rỗng) gọi bằng args tối thiểu hợp lệ và khẳng định **không** trả thông điệp placeholder `channel not wired`. Danh sách `pendingRPC` rỗng là điều kiện đóng CR-040.
5. Chạy: `go test ./internal/adapter/mcpserver/tools/...` (parity xanh), in dòng `MCP_CHANNEL_INVENTORY` và ghi `total` vào PR (kỳ vọng 46 kênh `codeIntel.*` nằm trong `excluded`).

## Kiểm thử

`go test ./internal/adapter/wscompat/... ./internal/adapter/mcpserver/tools/... -race`.

## Tiêu chí hoàn thành

- [x] 46/46 kênh có handler thật (hoặc `pendingRPC` có CR chặn được ghi).
- [x] Parity xanh; `TestCodeIntelChannelInventory` xanh.
- [x] Không `max-lines` disable; file mới < 400 dòng.

## Rủi ro và lưu ý

- Cổng này chỉ đóng hoàn toàn khi toàn bộ RPC backend có mặt (đợt 9); trước đó nó phản ánh tiến độ qua `pendingRPC`.
