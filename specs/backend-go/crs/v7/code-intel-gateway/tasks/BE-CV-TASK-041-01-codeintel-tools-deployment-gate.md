# BE-CV-TASK-041-01: Cổng triển khai `MCP_CODEINTEL_TOOLS_ENABLED` và builder `sized`

**From Solution:** BE-CV-SOL-041-mcp-codeintel-tools
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/config.go`, `catalog.go`, `spec_builders.go`, `catalog_test.go`, `backend-go/services/api-gateway/cmd/server/mcp_tools_wiring.go`, `cmd/server/mcp_tools_wiring_test.go`
**Depends on:** quyết định O2 = "Có"; BE-CV-SOL-040-codeintel-channel-foundation xong
**Status:** [ ] TODO

---

## Context

`DefaultConfig` bật pack 1 (`config.go`), `NewCatalog` giữ mọi spec của pack bật; thêm 9 spec `Risk=read` sẽ lập tức hiện trong `tools/list`. O2/O-11: mặc định tắt. `ApplyEnv` đọc `MCP_SCM_RATE_PER_MIN`, `MCP_PII_MASK`, `MCP_SENSITIVE_PATH_EXTRA`; wiring ở `mcp_tools_wiring.go:29-44` (đọc `MCP_TOOL_PACKS_ENABLED`, `MCP_TOOL_TIMEOUT`). Chạy `gitnexus_impact` trên `NewCatalog` và `Config.ApplyEnv`.

## Việc cần làm

1. `Config.CodeIntelTools bool`; `ApplyEnv`: `MCP_CODEINTEL_TOOLS_ENABLED` (`true|false|1|0`, sai giá trị => lỗi nêu tên biến; mặc định false).
2. `NewCatalog`: sau `s.prepare()`, `if s.Namespace == "codeIntel" && !cfg.CodeIntelTools { continue }`. Giữ `prepare()` chạy để spec hỏng vẫn fail sớm.
3. `spec_builders.go`: `func (s *ToolSpec) sized(n int) *ToolSpec { s.MaxResultBytes = n; return s }`.
4. `mcp_tools_wiring.go`: không đổi logic (cấu hình đi qua `ApplyEnv`); thêm vào chú thích hàm tên biến mới.

## Kiểm thử

- `catalog_test.go`: tắt (mặc định) => `Lookup("codeIntel_status")` không có, `Specs()` không chứa namespace `codeIntel`; bật => có; spec `prepare()` hỏng vẫn lỗi khi tắt.
- `mcp_tools_wiring_test.go`: `MCP_CODEINTEL_TOOLS_ENABLED=maybe` => lỗi; `=true` => `CodeIntelTools`.
- Lệnh: `go test ./internal/adapter/mcpserver/tools/... ./cmd/server/... -run 'Catalog|Wiring'`.

## Tiêu chí hoàn thành

- [x] Mặc định không lộ tool codeIntel; bật bằng env.
- [x] Parity/golden không phụ thuộc cổng (duyệt `AllSpecs()`).

## Rủi ro và lưu ý

- Admin Tools tab (`admin_views.go`) có thể liệt kê `Specs()`: khi cổng tắt tool không hiện (đúng ý).
- Đây là bổ sung của solution, chưa có trong hợp đồng (SOL-041 M6, Q1).
