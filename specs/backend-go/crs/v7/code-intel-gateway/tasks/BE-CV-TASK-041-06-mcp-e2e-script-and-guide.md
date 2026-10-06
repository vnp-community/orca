# BE-CV-TASK-041-06: Script e2e MCP và hướng dẫn "Code intelligence" cho người dùng MCP

**From Solution:** BE-CV-SOL-041-mcp-codeintel-tools
**Priority:** P2
**Service:** `api-gateway` (tài liệu, script)
**File:** `tests/mcp/check_mcp_codeintel_tools.py` (mới), `docs/guides/mcp/code-intelligence-tools.md` (mới), `docs/guides/mcp/README.md`, `tests/mcp/run_all.py`
**Depends on:** TASK-041-01..05; stack dev có `code-intel-service` (BE-CV-SOL-010)
**Status:** [ ] TODO

---

## Context

`tests/mcp/` có khung `mcp_check_framework.py` (`Context`, `WsRpc`, `run_single`) và các suite (`check_mcp_tools.py`, `check_mcp_ws_channels.py`...); `run_all.py` chạy gộp. `docs/guides/mcp/README.md` có bảng "Ai cần đọc gì". Tên file tài liệu theo nội dung, không `utils/helpers`.

## Việc cần làm

1. `check_mcp_codeintel_tools.py` theo khung có sẵn: (a) `tools/list` có/không 9 tool tuỳ `MCP_CODEINTEL_TOOLS_ENABLED`; (b) cờ tenant bật: `codeIntel_status` -> `codeIntel_changeOverlay` -> `codeIntel_symbol` trên worktree mẫu (lấy `project_id`, `worktree_id` từ `project_list`/`worktree_list`); khẳng định khối text có `<untrusted-content`; (c) cờ tắt: lỗi mã `CODEINTEL_DISABLED`; (d) worktree của người khác: từ chối; (e) `tools/call codeIntel_reindex` không tồn tại. Thiếu môi trường => SKIP có lý do, không FAIL.
2. Đăng ký suite vào `run_all.py` theo cách các suite khác.
3. `code-intelligence-tools.md` (tiếng Việt, như các guide khác): 9 tool và tham số (`project_id`, `worktree_id`), điều kiện (cổng triển khai, cờ tenant `code_intel_enabled`), cảnh báo nội dung không tin cậy, giới hạn kích thước/`truncated`, quyền (`orca:read`), tenant có thể hạ `codeIntel_symbol` về `deny`/`require_approval`, không có reindex/ghi. Thêm dòng vào bảng của `README.md` guide.

## Kiểm thử

`python tests/mcp/check_mcp_codeintel_tools.py` (cần stack; chưa chạy); kiểm lint tài liệu theo quy ước repo (link tương đối hợp lệ).

## Tiêu chí hoàn thành

- [ ] Suite chạy hoặc SKIP rõ lý do; tài liệu mô tả đủ 9 tool và cảnh báo.

## Rủi ro và lưu ý

- Không đặt token/secret thật trong script (đọc từ `.env` như `get_mcp_token.py`).
- Số lượng tool tăng 9 ảnh hưởng ngữ cảnh LLM; ghi nhận trong guide, đo ở CR-071.
