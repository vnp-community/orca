# frontend Solutions — MCP Resources & Prompts (v5)

**CRs:** [docs/crs/v5/mcp-resources-prompts](../../../../../../docs/crs/v5/mcp-resources-prompts/README.md)
**Hợp đồng:** [CONTRACT-mcp-ui-api.md](../../../../../backend-go/crs/v5/CONTRACT-mcp-ui-api.md) · **Quy ước FE v5:** [crs/v5/README.md](../../README.md) · **Phía backend:** [specs/backend-go/crs/v5/mcp-resources-prompts/solutions](../../../../../backend-go/crs/v5/mcp-resources-prompts/solutions/README.md)

## Solutions

| Solution | CR | Area | Effort | Status |
|---|---|---|---|---|
| — | CR-MCP-010 (resources) | **Không có solution FE — xem "Resources không có UI" bên dưới** | — | Không áp dụng |
| [FE-MCP-SOL-007](./FE-MCP-SOL-007-custom-prompts-admin.md) | CR-MCP-011 (UI) | Tab "Prompts" trong `McpSettingsPane` (admin): danh sách built-in chỉ đọc + CRUD prompt tuỳ chỉnh, editor tham số, editor template với gợi ý `{{name}}`, hiển thị version, lỗi validation | Medium (≈3 ngày) | 🔲 Designed — chưa implement |

## Resources (CR-MCP-010) không có UI — lý do

- `resources/list|templates/list|read|subscribe` và `notifications/resources/*` là **primitive của giao thức MCP do client (host của agent) tiêu thụ** (Claude Desktop/Code, Inspector…), qua `/mcp` — không phải kênh WS RPC của UI Orca. CONTRACT-mcp-ui-api.md **không có** kênh `mcp.resource.*` nào, và BE-MCP-SOL-010 không đưa ra kênh UI.
- Không có thao tác nào của người dùng Orca cần màn hình: quyền đọc dùng chung policy/scope của tool tương ứng (cấu hình ở FE-MCP-SOL-008), subscription gắn vòng đời phiên (hiển thị `activeStreams` đã có ở FE-MCP-SOL-002), audit hiển thị ở FE-MCP-SOL-010 (`McpAuditEntry.tool` = tên tool ánh xạ).
- Tab "Tools" (FE-MCP-SOL-005) không liệt kê resource; nếu sau này cần "xem tập resource template mà tenant mở", phải thêm kênh vào CONTRACT trước (đề xuất tương lai, không chặn GA).
- Hệ quả cho FE: **không** thêm code, kiểu, slice hay i18n cho resources.

## Re-verify: hiện trạng frontend so với giả định

| # | Giả định | Thực tế (đã đọc mã) | Drift |
|---|---|---|---|
| 1 | Có UI mẫu CRUD admin để theo | `admin-org-console-policies-tab.tsx` (form + danh sách, `window.api.admin.*`, nhãn hard-code tiếng Anh) — theo cấu trúc nhưng dùng `translate` | Có (cải tiến i18n) |
| 2 | Primitive `alert-dialog`/`switch` | Không có; `dialog`, `sheet`, `textarea`, `checkbox`, `toggle-group` có; xác nhận bằng `useConfirmationDialog()` | Có |
| 3 | Dialog đã có `DialogDescription` | Từng thiếu (commit `da67f0b33` sửa dialog AI Provider) — FE-007 bắt buộc có | Lưu ý |
| 4 | CONTRACT có mã lỗi cho prompt | Không — đề xuất `MCP_PROMPT_*` (BE-011); FE có đường lùi hiển thị thông điệp server | Có — chờ CONTRACT |
| 5 | `McpPrompt` có locale | Không; locale chỉ phía server cho built-in | Hạn chế đã ghi |
| 6 | Cú pháp biến `{{name}}` | Do BE-011 chốt cho prompt tuỳ chỉnh; built-in dùng Go template (chỉ đọc) ⇒ "Duplicate" built-in cần chặn cú pháp nâng cao | Quyết định thiết kế |

## Tính nhất quán BE ↔ FE (BE-MCP-SOL-011)

| Chủ đề | Giá trị dùng ở cả hai phía |
|---|---|
| Kênh | `mcp.admin.prompt.list/upsert/delete` (CONTRACT §2.2) |
| Quy tắc | tên `^[a-z][a-z0-9_]{2,47}$`, không trùng built-in; tham số `^[a-z][a-z0-9_]{0,31}$`; ≤ 10 tham số, ≤ 50 prompt/tenant, template ≤ 8192, mô tả ≤ 500 |
| Lỗi | `MCP_NOT_ADMIN`, `MCP_DISABLED`, `MCP_NOT_FOUND` (có sẵn); `MCP_PROMPT_INVALID: <field>: <lý do>`, `MCP_PROMPT_NAME_CONFLICT`, `MCP_PROMPT_VERSION_CONFLICT`, `MCP_PROMPT_BUILTIN_READONLY` (đề xuất) |
| `id` built-in | `builtin:<name>` (FE không gửi upsert/delete cho id này) |

## Thứ tự & phụ thuộc
```
FE-MCP-SOL-001 (nền) ─► FE-MCP-SOL-007 ◄─ BE-MCP-SOL-011 (kênh prompt) ◄─ BE-MCP-SOL-010 (resource nhúng, chỉ cho prompts/get)
```
FE-007 không cần BE-010 để chạy (UI chỉ gọi kênh admin).

## Sửa TDD kèm theo (tổng hợp)
`specs/frontend/tdd/v5` (05-admin): tab Prompts ở `McpSettingsPane`.

## Thay đổi CONTRACT đề nghị
Thêm vào §2.3: `MCP_PROMPT_INVALID`, `MCP_PROMPT_NAME_CONFLICT`, `MCP_PROMPT_VERSION_CONFLICT`, `MCP_PROMPT_BUILTIN_READONLY`.
