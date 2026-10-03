# CR-MCP-007 — Introspection cho `wscompat.Registry` + descriptor/schema cho tool

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-007 |
| **Tên** | Thêm lớp mô tả tool (tên, mô tả, JSON Schema vào/ra, scope, annotations) và cơ chế `ToolExecutor` bọc `Registry.Dispatch` |
| **Loại** | Feature (nền tảng) |
| **Priority** | 🔴 P0 |
| **Effort** | Large (8–10 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | ✅ Đã triển khai (unit/integration test) — xem solution và README service để biết khoảng trống |
| **Tác giả** | Khảo sát `wscompat/registry.go` |
| **Phụ thuộc** | [CR-MCP-003](../mcp-protocol-server/CR-MCP-003-streamable-http-and-lifecycle.md) |

---

## Bối cảnh & Vấn đề

`registry.go` cho thấy:

- `ChannelHandler = func(ctx, Identity, []json.RawMessage) (any, error)` — **tham số theo vị trí, không có schema**; mỗi handler tự giải mã `args`.
- `Registry` giữ 4 map riêng (`handlers`, `streamHandlers`, `streamChannelHandlers`, `binaryStreamHandlers`), tất cả **private**, và trong phần mã đã đọc **không có** API liệt kê. Một channel chỉ thuộc đúng một trong bốn loại.
- Kết quả trả về là `any` (thường là proto message hoặc struct), không có mô tả.
- Channel không đăng ký ⇒ `notImplementedHandler` trả lỗi văn bản — client MCP sẽ thấy tool "tồn tại" nhưng lỗi nếu ta liệt kê mù.

`tools/list` đòi mỗi tool có `name`, `description` (LLM đọc!) và `inputSchema` (JSON Schema). Không thể suy ra tự động từ chữ ký hàm hiện tại một cách đáng tin.

## Giải pháp đề xuất

### A. Mở API introspection tối thiểu cho Registry

Thêm `Registry.Channels() []ChannelInfo{Name, Kind}` (Kind ∈ unary/stream/streamChannel/binary) — chỉ đọc, không đổi hành vi. Chạy `gitnexus_impact` trên `Registry` trước khi sửa (nhiều caller) và nêu blast radius trong PR; thay đổi chỉ **thêm** method nên rủi ro thấp.

### B. Descriptor khai báo tường minh (không suy diễn)

Mỗi tool được khai báo trong code Go cạnh domain của nó (ví dụ `mcpserver/tools/task.go`), dạng:

```go
ToolSpec{
  Channel:     "task.create",              // khoá nối tới Registry
  Name:        "task_create",              // D7
  Title:       "Tạo task",
  Description: "…viết cho LLM: khi nào dùng, tham số nào bắt buộc, tác dụng phụ…",
  Input:       schemaOf[TaskCreateInput](), // sinh JSON Schema từ struct Go (jsonschema tag)
  Output:      schemaOf[TaskView](),        // outputSchema + structuredContent
  Scope:       "orca:write",
  Annotations: {ReadOnly:false, Destructive:false, Idempotent:false, OpenWorld:false},
  Args:        func(in TaskCreateInput) []json.RawMessage { … }, // đổi object → args vị trí
}
```

Lý do khai báo tay: handler nhận args vị trí; sinh schema bằng reflect trên handler sẽ ra schema sai/nghèo. `Args()` là chỗ **duy nhất** biết thứ tự tham số ⇒ có test cho từng tool.

Quy tắc viết mô tả tool: ngắn, nêu tác dụng phụ và điều kiện tiên quyết; **không** nhúng dữ liệu tenant; không hướng dẫn lách kiểm soát. Output trả cả `structuredContent` (khớp `outputSchema`) lẫn `content` văn bản để client cũ vẫn đọc được; nội dung dài ⇒ cắt kèm gợi ý phân trang, tránh nuốt ngữ cảnh LLM.

### C. `ToolExecutor`

`tools/call` ⇒ validate input theo schema ⇒ kiểm scope/policy (CR-006/012) ⇒ `Registry.Dispatch(ctx, Identity, channel, Args(in))` ⇒ chuẩn hoá kết quả (proto → JSON bằng `protojson`, cắt kích thước, che trường nhạy cảm) ⇒ audit (CR-013). Lỗi: map về `isError:true` có thông điệp ngắn.

### D. Parity test — chống lệch giữa UI và MCP

Một test CI duy nhất: liệt kê `Registry.Channels()` và so với danh sách `ToolSpec` + **danh sách loại trừ có lý do** (`excluded_channels.yaml`: ví dụ `credentials.*`, `mobile.*`, `terminal.multiplex` binary…). Channel mới thêm vào Registry mà không có `ToolSpec` cũng không có lý do loại trừ ⇒ **CI đỏ**. Test cũng kiểm: tên tool duy nhất sau D7, schema hợp lệ, mô tả không rỗng và ≤ độ dài tối đa, mọi tool có scope + annotations.

### E. Catalog theo tenant & `list_changed`

`tools/list` trả theo **policy hiệu lực** của (tenant, user, scope) — tool bị cấm thì ẩn. Khi policy đổi ⇒ phát `notifications/tools/list_changed` (CR-004).

## Acceptance Criteria

- [ ] `Registry.Channels()` có test; không đổi hành vi hiện có (toàn bộ test `wscompat` vẫn xanh).
- [ ] Parity test chạy trong `pnpm`/`make test` và đỏ khi thêm channel không khai báo.
- [ ] Ít nhất 1 tool mỗi loại (đọc, ghi, phá huỷ) end-to-end qua MCP Inspector với schema đúng.
- [ ] Input sai schema ⇒ lỗi tham số rõ ràng, không chạm tới handler.
- [ ] Số liệu chính xác "bao nhiêu channel / bao nhiêu đã có tool / bao nhiêu loại trừ" được in ra ở CI để theo dõi tiến độ.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** nợ kỹ thuật khi ~417 mô tả phải viết tay ⇒ chia pack (CR-008), ưu tiên theo giá trị; mô tả tool cũng là bề mặt prompt-injection nên cần review.
- **Rủi ro:** phụ thuộc vào `wscompat` (lớp tương thích "legacy") ⇒ nếu sau này thay bằng gRPC-gateway, `ToolSpec.Channel` là điểm duy nhất cần đổi.
- **Ngoài phạm vi:** nội dung từng pack (CR-008), stream (CR-009).
