# TASK-REQ-017-01: `ToolSpec` cho `request_*`, `solution_*`, `approval_*`, `backlog_*` và cập nhật loại trừ MCP

**From Solution:** BE-REQ-SOL-017
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/pack_request_flow.go` (mới), `.../all_specs.go`, `.../excluded_channels.yaml`, `.../pack_request_flow_test.go` (mới)
**Depends on:** BE-REQ-SOL-016 (các kênh đã đăng ký: TASK-REQ-016-03, 04, 05)
**Status:** `[x] DONE`

---

## Context

- `spec_builders.go:46-72`: `read` (pack 1, `RiskRead`), `write` (pack 2, `RiskWriteReversible`), `execTool` (pack 3), `.asList()`, `.untrusted()`, `.openWorld()`, `.idempotent()`, `.declared()`. `fields.go`: `Str`, `Int`, `Bool`, `Strs`; `Req`, `OneOf`, `Len`, `Range`.
- `all_specs.go:5-15` `AllSpecs()` gộp 13 nhóm; thêm nhóm mới ở đây.
- `config.go:66` `Packs` mặc định `{1: true}`: tool pack 2 và 3 ẩn trừ khi `MCP_TOOL_PACKS_ENABLED=1,2` (`ParsePacks` dòng 69).
- Tên tool = `ChannelToToolName(channel)` (`spec.go:119`), giữ chữ hoa (`request_changeType`). `NameReason` chỉ cần khi đổi tên.
- `excluded_channels.yaml`: mỗi kênh có `ToolSpec` hoặc dòng `pattern`; không cả hai (`TestChannelInventory` báo `both`).
- Bảng tool: SOL-017 mục 2.1 và CR-REQ-017 mục 2.1.

## Việc cần làm

1. `pack_request_flow.go`: `func packRequestFlow() []*ToolSpec` trả:
   - đọc (pack 1): `request.list` (`.asList().untrusted()`), `request.get` (`.untrusted()`), `request.typeHistory` (`.asList().untrusted()`), `solution.list` (`.asList().untrusted()`), `approval.get`, `approval.list` (`.asList()`), `approval.listPending` (`.asList()`), `backlog.requests` (`.asList().untrusted()`), `backlog.tasks` (`.asList()`), `backlog.execute` (`.asList()`), `request.flowStatus`;
   - ghi (pack 2): `request.create`, `request.changeType`, `request.returnToBacklog`, `request.reopen`, `request.spawnChild`, `solution.generate` (`.openWorld()`);
   - `.declared()`: `request.classify`, `request.generatePlan` (pack 2), `request.startPhase` (`execTool`, pack 3).
2. Fields theo CR-REQ-017 mục 2.1 (`snake_case` tool, wire `camelCase` khớp CONTRACT mục 2). Enum bằng `OneOf`: loại (11), `stage` (5), `link reason` (4), `kind` (4), `status`. `Len`: `title` 500, `body` 100000, `client_request_id` 128, `reason` 2000, `feedback` 2000. Không field `tenant_id`, `user_id`, `reporter_id`, `source_provider`.
3. Mô tả `request.create` nhắc agent đặt `client_request_id` ổn định; mô tả `request.changeType` nói rõ người xác nhận lại.
4. `all_specs.go`: thêm `packRequestFlow()` vào danh sách.
5. `excluded_channels.yaml`: (a) gỡ dòng tạm "Chờ BE-REQ-SOL-017" cho mọi kênh có `ToolSpec` (kể cả `Declared`); (b) thêm dòng vĩnh viễn (`category: human_gate`) cho `request.confirmType`, `solution.choose`, `approval.approve`, `approval.reject`, `approval.cancel`, `request.cancel`, `request.flowSet`, `request.subscribe`, mỗi dòng có `reason` rõ.
6. Chạy lại `tools/testdata/tools_list.golden.json` bằng cách đã dùng trong repo (xem lệnh cập nhật golden ở đầu `catalog_test.go`; chưa kiểm chứng cờ `-update`), kiểm diff chỉ thêm 17 tool.

## Kiểm thử

- `pack_request_flow_test.go`: 17 tool liệt kê khi bật pack 1,2; 3 tool `Declared` có trong `Specs()` nhưng không có trong `Lookup` hoặc danh sách; risk, scope đúng bảng; tool đọc có `readOnlyHint`.
- `parity_test.go` chạy lại: xanh. `go test ./internal/adapter/mcpserver/tools/...`.
- Test `tools/call` tới `approval_approve`, `solution_choose`, `request_confirmType`: "tool không tồn tại".

## Tiêu chí hoàn thành

- [x] 17 tool liệt kê, 3 `Declared`; golden cập nhật có chủ đích trong cùng PR.
- [x] Không kênh Request nào vừa có spec vừa có dòng loại trừ.
- [x] Input schema không có field định danh (test ở TASK-REQ-017-02).

## Rủi ro và lưu ý

- Số tool tăng làm `tools/list` dài; đo kích thước golden, báo nếu vượt ngưỡng đã bàn ở CR-MCP-008.
- Đừng đổi tên kênh để "gọn" tên tool; tên theo `ChannelToToolName` là hợp đồng.
