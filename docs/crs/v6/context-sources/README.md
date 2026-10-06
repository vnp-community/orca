# Context Sources: Source Registry và Context Pack (v6)

> Chất lượng đầu ra của AI bị giới hạn bởi ngữ cảnh đầu vào. Feature này định nghĩa registry nguồn dữ liệu (nội bộ và MCP ngoài), hợp đồng adapter, bộ lắp Context Pack ở `request-service` kèm `Evidence`, và các khoảng trống của registry MCP ngoài hiện có. Hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-031](./CR-REQ-031-source-registry-and-context-pack.md) | `ai.complete` chỉ nhận một chuỗi prompt; nguồn có sẵn trong repo (quy ước, ADR, hợp đồng, CI, OPA, CodeGraph, GitNexus) không vào được; registry MCP ngoài chỉ phục vụ agent, backend không tự gọi `tools/call` hay `resources/read`; chưa có `CODEOWNERS`, danh mục service, ví dụ vàng, từ điển thuật ngữ | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-002, 005, 007, 008, 012 (các bước AI) ─┐
CR-REQ-033 (hồ sơ năng lực), 035 (secretscan) ─┼─▶ CR-REQ-031 đợt 1 (nguồn nội bộ) ─▶ đợt 2 (MCP ngoài) ─▶ đợt 3
CR-REQ-017 (tool MCP, `orca://`) ──────────────┘
```

| Đợt | Nội dung | Điều kiện |
|---|---|---|
| 1 | Adapter nội bộ: quy ước, ADR, CR, hợp đồng, schema, CI, OPA, CodeGraph/GitNexus, lịch sử Git, lịch sử Orca, hồ sơ dev server; Builder, `Evidence`, `context_packs` | Không phụ thuộc bên ngoài |
| 2 | RPC `CallExternalTool` và `ReadExternalResource` ở `mcp-service`; nguồn MCP thiết yếu (CI, quan sát, quét lỗ hổng, tài liệu thư viện, wiki) | Mỗi nguồn có chủ sở hữu, TTL, che dữ liệu, hạn mức, cách tắt nhanh, kiểm toán |
| 3 | Chat, email, lịch phát hành, CVE, tiêu chuẩn | Chính sách quyền riêng tư của tổ chức |

Song song, việc dữ liệu (không phải mã): tạo `CODEOWNERS`, danh mục service sinh tự động, bộ ví dụ vàng, từ điển thuật ngữ, chỉ mục ADR/CR.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| S1 | Registry `context_sources` ở `request-service` (hai dialect); máy chủ MCP ở `mcp-service` (Postgres) tham chiếu bằng `server_ref` | `mcp-service` chỉ Postgres; không nhân đôi bảng |
| S2 | Context Pack lắp ở `request-service` trước khi gọi `ai.complete` hoặc `agent.execPrompt`; agent không tự gọi nguồn ngoài giữa chừng cho bước sinh nội dung | Một chỗ kiểm tra, che, ghi bằng chứng |
| S3 | Nguồn nội bộ đọc repo qua `Relay` (`fs.*`, `git.*`, `agent.exec`) | Repo nằm trên dev server; hỗ trợ SSH và remote |
| S4 | Xếp hạng và cắt bằng quy tắc xác định (`cp/1`), không tóm tắt bằng AI, không embedding | Tái lập; GitNexus báo `embeddings: 0` |
| S5 | Mọi mảnh có `source_id`, `ref`, `retrieved_at`, `freshness`, `trust`, `digest`, `size` và một `Evidence` (`EVD-<n>`); đầu ra AI phải trích `evidence_refs` hợp lệ | Người duyệt kiểm được nguồn gốc |
| S6 | Mảnh `trust=low` và mọi nguồn `mcp` nằm trong khối `<untrusted>`; v1 chỉ đọc, không tool ghi từ nguồn ngoài | Chống chèn chỉ dẫn; `ToolInfo` không phân loại đọc và ghi |
| S7 | `CallExternalTool` chỉ nhận tool thuộc `ApprovedTools` ∩ `scopes`, máy chủ `Usable()`, `transport=http` | Dùng lại SSRF, bí mật, duyệt và rug-pull check của `mcp-service` |
| S8 | `orca://request|solution|plan|evidence|impact|context/...` là resource chỉ đọc, đánh `untrusted` | Theo mẫu `resources/` có sẵn |
| S9 | Nguồn thiếu luôn hiện trong `missing[]`; không im lặng bỏ | Người duyệt biết giới hạn của bằng chứng |

## Phạm vi ngoài feature này

Tool ghi tới nguồn ngoài (ví dụ comment Jira, CR-REQ-024), nguồn Request từ MCP (CR-REQ-017), quản trị ngân sách và mô hình AI (CR-REQ-034), `secretscan` (CR-REQ-035), hồ sơ năng lực dev server (CR-REQ-033), giao diện hiển thị Evidence (CR-REQ-020, 021, 032).

## Điểm lệch và điểm cần xác nhận khi viết feature này

- Registry MCP ngoài chưa được thử với máy chủ thật; mức hoàn thiện `ResolveAgentMcpConfig` cho `claude --print` chưa kiểm chứng (renderer `Unverified`).
- `ToolInfo` không có chú thích chỉ-đọc, nên danh sách tool cho phép phải khai tường minh (Q4 của CR).
- CR-REQ-017 mục 7 (Q4) ghi `orca://request/{id}` "ngoài phạm vi"; CR này nhận.
- Định dạng đầu ra CLI CodeGraph và GitNexus để phân tích bằng chương trình chưa kiểm chứng; có thể phải đi qua MCP của chúng.
- `CODEOWNERS` không có ở gốc hay `.github/`; nguồn `ownership` yếu cho tới khi tạo.
