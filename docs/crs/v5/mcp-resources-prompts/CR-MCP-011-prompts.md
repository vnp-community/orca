# CR-MCP-011 — Prompts: mẫu quy trình dùng lại cho agent

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-011 |
| **Tên** | `prompts/list` / `prompts/get` với các mẫu quy trình Orca (review PR, triage issue, bàn giao task, tóm tắt worktree) |
| **Loại** | Feature |
| **Priority** | 🟡 P2 |
| **Effort** | Small (2–3 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát nhu cầu từ các flow hiện có (hostedReview, task, workflow) |
| **Phụ thuộc** | [CR-MCP-010](./CR-MCP-010-resources-templates-subscriptions.md) |

---

## Bối cảnh

Prompt trong MCP do **người dùng** chủ động chọn (ví dụ slash command trong client) — khác tool do LLM tự quyết. Chúng đóng gói "cách làm đúng" trên Orca để agent không phải tự mò chuỗi tool: nhờ vậy giảm sai sót và giảm số bước.

## Giải pháp đề xuất

### A. Bộ prompt khởi đầu (có tham số, có `completion` gợi ý)

| Prompt | Tham số | Kết quả |
|--------|---------|---------|
| `review_pull_request` | `provider`, `repo`, `number` | Tin nhắn hướng dẫn: đọc resource PR + diff, kiểm tra checks, đề xuất nhận xét — **không** tự đăng nhận xét nếu chưa được xác nhận |
| `triage_issue` | `issue_ref` | Đọc ticket (Jira/Linear/GitHub), đề xuất nhãn/ưu tiên/task con |
| `plan_task` | `task_id` | Đọc task + đồ thị phụ thuộc, đề xuất kế hoạch từng bước |
| `summarize_worktree` | `worktree_id` | Tóm tắt thay đổi chưa commit, nhánh, trạng thái CI |
| `handoff_to_agent` | `task_id`, `agent` | Soạn mô tả bàn giao cho agent khác (dùng orchestration) |

Mỗi prompt trả `messages` kèm **embedded resource** (CR-010) thay vì nhét dữ liệu thô vào văn bản, và nêu rõ ranh giới: những bước nào cần xác nhận của người dùng.

### B. Quản lý nội dung

- Prompt viết bằng file template trong repo (`mcpserver/prompts/*.tmpl`) có version; có test golden.
- Cho phép tenant bổ sung prompt riêng (lưu ở `mcp-service`, kèm kiểm duyệt: chỉ admin tạo; chống chèn chỉ dẫn vượt quyền).
- Văn bản theo ngôn ngữ người dùng (vi/en) theo `Accept-Language`/hồ sơ, mặc định en.

### C. An toàn

Prompt **không** được chứa dữ liệu tenant thật ngoài tham số được truyền; không nới quyền (quyền thật vẫn do token + policy). Tham số được validate (độ dài, ký tự) trước khi nội suy.

## Acceptance Criteria

- [ ] `prompts/list` trả 5 prompt; `prompts/get` với tham số hợp lệ trả `messages` đúng golden.
- [ ] Tham số thiếu/sai ⇒ lỗi tham số rõ ràng.
- [ ] Prompt tenant tuỳ chỉnh chỉ do admin tạo và không xuất hiện ở tenant khác.
- [ ] Thay đổi prompt ⇒ `notifications/prompts/list_changed`.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** giá trị phụ thuộc client có hiển thị prompt hay không ⇒ ưu tiên P2.
- **Ngoài phạm vi:** UI soạn prompt (frontend).
