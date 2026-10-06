# Feature: code-intel-gateway — Kênh WS `codeIntel.*` và (tuỳ chọn) tool MCP

> **Trạng thái:** 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code ngày 2026-10-05, chưa chạy hệ thống, chưa viết code.
> **Hợp đồng chung:** [`../README.md`](../README.md) (mục 3.3 mã lỗi, 3.6 gRPC, 3.7 kênh). Khi CR và README khác nhau, theo CR (README mục 8).

## 1. Mục tiêu

Đưa `code-intel-service` (CR-CV-010) ra frontend qua đường có sẵn của `api-gateway`: kênh WebSocket `wscompat`. Gateway chỉ dịch tham số, gắn `Identity`, đặt timeout, giới hạn kích thước, ánh xạ lỗi và chuyển tiếp push; **không** giữ DB, **không** gọi agent, **không** tự quyết quyền nghiệp vụ. Tuỳ chọn (P2): mở một tập con **chỉ-đọc** cho agent ngoài qua MCP, đi qua chính sách của `mcp-service`.

## 2. Danh sách CR

| CR | Tên | Priority | Effort | Phụ thuộc | Mở khoá |
|---|---|---|---|---|---|
| [CR-CV-040](./CR-CV-040-api-gateway-codeintel-channels.md) | Kênh `codeIntel.*` ở `api-gateway`, push, giới hạn kích thước, parity MCP | 🔴 P0 | Large | CR-CV-010 (proto, service chạy), 011, 012, 013, 021; một số kênh theo sau CR-CV-031, 033, 034, 036 | CR-CV-041, 050 trở đi (frontend), 071, 072, 073 |
| [CR-CV-041](./CR-CV-041-mcp-codeintel-tools.md) | (Tuỳ chọn) Tool MCP `codeIntel_*` chỉ-đọc qua chính sách `mcp-service` | ⚪ P2 | Medium | CR-CV-040, v5 CR-MCP-007, 008, 012, 013; quyết định O2 | không |

## 3. Thứ tự thực thi

```
CR-CV-010 (proto + service) ─▶ CR-CV-040 ─▶ CR-CV-041 (chỉ khi O2 được duyệt)
```

CR-CV-040 có thể làm theo từng lát: đăng ký **tất cả** tên kênh ngay từ đầu (trả `CODEINTEL_UNAVAILABLE` khi RPC đích chưa có), rồi nối RPC thật dần theo các CR của service. Lý do ở quyết định G3.

## 4. Quyết định chung của feature

| # | Quyết định | Lý do |
|---|---|---|
| G1 | `tenantId`, `userId`, `devServerId`, `workspaceRoot`, tên repo GitNexus, `args`, tên lệnh CLI, chuỗi Cypher **không bao giờ** là tham số của kênh; gateway giải mã **chặt** (từ chối khoá lạ) | D5 và O4 của README; client không được chọn máy hay repo. Mẫu `Identity` ở `wscompat/registry.go` |
| G2 | Mọi kênh view chỉ nhận định danh Orca (`worktreeId`, `projectId`) và để `code-intel-service` tự tra máy dev, repo, quyền | Gateway không có OPA trước định tuyến (README api-gateway, mục "No OPA authorization check", đã kiểm lại 2026-10-05): mọi RPC của service tự kiểm quyền |
| G3 | Đăng ký toàn bộ kênh vô điều kiện trong `RegisterProductionChannels` (client nil vẫn hợp lệ) | `parity_test.go` dựng inventory bằng `ChannelDeps{TaskActivityEnabled: true}` và đỏ nếu một dòng loại trừ không khớp kênh nào ("dead exclusion") hoặc kênh không có ToolSpec lẫn dòng loại trừ |
| G4 | Ở MVP (O2) loại **toàn bộ** `codeIntel.*` khỏi MCP bằng một dòng `codeIntel.*` trong `excluded_channels.yaml`; CR-CV-041 thay dòng đó bằng danh sách loại trừ tường minh khi mở tool | Giữ parity xanh ở từng PR |
| G5 | Cờ `code_intel_enabled` do `code-intel-service` thi hành (CR-CV-073); gateway chỉ chuyển lỗi `CODEINTEL_DISABLED` | Một điểm thi hành cho WS, MCP, push |
| G6 | Lỗi ra WS luôn dạng `CODE: message` một dòng; message không chứa mã nguồn, đường dẫn tuyệt đối trên dev server hay nội dung tham số | Envelope lỗi của WS chỉ có trường `message` (`envelope.go`), dialect session-client cố định `code:"internal"` (`session_dialect.go`), nên mã nằm ở tiền tố `message` |

## 5. Phát hiện chung (cần người duyệt xem)

- **Thiếu kênh đăng ký push.** README 3.7 liệt kê hai khung push `codeIntel.changed`, `codeIntel.reindexProgress` nhưng không có kênh `subscribe` nào để mở chúng. Mẫu có sẵn (`notifications.subscribe` đẩy `notifications.event`, `workspacePorts.subscribe` đẩy `workspacePorts.opened/closed`) đều có một kênh subscribe. CR-CV-040 thêm `codeIntel.subscribe`.
- **Thiếu kênh cờ.** README 3.5/3.6/3.7 chưa có bảng, RPC hay kênh để frontend đọc `code_intel_enabled`. CR-CV-040 thêm `codeIntel.settings.get`, `codeIntel.settings.set`; RPC và bảng do CR-CV-073.
- **Mã lỗi gateway thiếu trong 3.3**: `CODEINTEL_UNAVAILABLE`, `CODEINTEL_DISABLED`, `CODEINTEL_NOT_FOUND`, `CODEINTEL_RESPONSE_TOO_LARGE`, `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_RATE_LIMITED`, `CODEINTEL_INTERNAL` (xem CR-CV-040 mục 2.8).
- **Tên tool MCP:** README 3.x và mục 4 viết `codeintel_*`, nhưng quy ước D7 của v5 và `ChannelToToolName` giữ nguyên chữ hoa, nên tên đúng là `codeIntel_status`, `codeIntel_impact`... (CR-CV-041).
- **Giới hạn thật của hạ tầng (đã đọc code):** `gatewaygrpc.Dial` không đặt `MaxCallRecvMsgSize` (mặc định gRPC 4 MiB nhận); `invokeTimeout` của WS là 25 s; repo không gọi `SetReadLimit` nên giới hạn đọc mặc định của thư viện WebSocket áp cho tham số vào (chưa kiểm chứng bằng đọc mã thư viện). Hệ quả cho kích thước ở CR-CV-040.

## 6. Ngoài phạm vi

- Logic dựng view, cache, quyền nghiệp vụ, audit (CR-CV-013, 020 đến 038).
- Route HTTP (`httpgateway`) cho code-intel: không có ở v7; mọi thứ đi WS.
- Hỗ trợ kênh qua mobile (`mobile_envelope.go`): CR-CV-062 xét riêng.
- Tool MCP ghi/reindex: không bao giờ mở (CR-CV-041, quyết định D2).

## 7. Tài liệu liên quan

- `backend-go/services/api-gateway/README.md`
- `docs/crs/v6/gateway-and-mcp/README.md`, `CR-REQ-016`, `CR-REQ-017` (mẫu định dạng và cách xử lý parity)
- `docs/crs/v5/README.md` (D7), `docs/crs/v5/mcp-tool-catalog/CR-MCP-007`, `CR-MCP-008`, `docs/crs/v5/mcp-governance-safety/CR-MCP-012`, `CR-MCP-013`
