# BE-CV-TASK-041-04: Thay dòng loại trừ `codeIntel.*` bằng danh sách tường minh; parity và golden

**From Solution:** BE-CV-SOL-041-mcp-codeintel-tools
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`, `testdata/tools_list.golden.json`
**Depends on:** TASK-041-02
**Status:** [ ] TODO

---

## Context

Một kênh vừa có `ToolSpec` vừa bị loại trừ làm `TestChannelInventory` đỏ ("both exposed as tools and excluded", `parity_test.go:56-58`); kênh không có cả hai cũng đỏ. Dòng `codeIntel.*` của TASK-040-09 phủ cả 9 kênh có tool nên **phải** thay trong cùng PR với `pack1CodeIntel`. `Match` hỗ trợ `.*` trải mọi cấp (`excluded.go:51`). Golden duyệt mọi spec (`catalog_test.go:161`).

## Việc cần làm

1. Xoá dòng `pattern: "codeIntel.*"`.
2. Thêm 17 dòng kênh: `codeIntel.reindex`, `codeIntel.reindexStatus`, `codeIntel.structure`, `codeIntel.architecture`, `codeIntel.dataFlow`, `codeIntel.storage`, `codeIntel.subgraph`, `codeIntel.contractDiff`, `codeIntel.dismissFinding`, `codeIntel.reviewState.get`, `codeIntel.reviewState.save`, `codeIntel.c4.get`, `codeIntel.c4.save`, `codeIntel.bindRepo`, `codeIntel.settings.get`, `codeIntel.settings.set`, `codeIntel.subscribe`; mỗi dòng `category` (`code-intel-v1`, riêng `subscribe`: `stream`) và `reason` >= 20 ký tự, có lý do cụ thể (ghi/đổi trạng thái, đồ thị lớn, cấu hình, push).
3. Thêm **một** dòng `codeIntel.quality.*` (`category: code-intel-quality-v1`): hợp đồng §9 cấm tool cho `quality.*`.
4. `go test -run TestToolsListGolden -update` rồi review diff: chỉ thêm 9 mục `codeIntel_*` (pack 1, risk read, scope `orca:read`).

## Kiểm thử

- `go test ./internal/adapter/mcpserver/tools/... -run 'Parity|ChannelInventory|Golden'` xanh; `MCP_CHANNEL_INVENTORY` có `covered` tăng 9 và `excluded` tăng 37 so với trước.
- Thử xoá một dòng loại trừ => `TestChannelInventory` đỏ nêu kênh; thêm lại.
- `tools/call` `codeIntel_reindex` và `codeIntel_bindRepo` => "unknown tool" (test trong executor).

## Tiêu chí hoàn thành

- [ ] 9 + 37 = 46 kênh phủ, không trùng.
- [ ] Golden đổi đúng 9 mục.

## Rủi ro và lưu ý

- Thêm kênh `codeIntel.*` mới sau này mà quên dòng loại trừ sẽ đỏ ngay (mong muốn).
- Các dòng `reason` bằng tiếng Anh theo mẫu file hiện có.
