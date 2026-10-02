# CR-MCP-010 — Resources, resource templates và subscriptions

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-010 |
| **Tên** | Phơi dữ liệu Orca dạng resource URI (`orca://…`), hỗ trợ template, `resources/subscribe` và `list_changed` |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium (4–6 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `wscompat` (task/files/git), `notification-service` |
| **Phụ thuộc** | [CR-MCP-007](../mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md), [CR-MCP-004](../mcp-protocol-server/CR-MCP-004-sessions-sse-resumability.md) |

---

## Bối cảnh & Vấn đề

Với tool-only, agent muốn biết "task này đang ở đâu" phải gọi nhiều tool và tự ghép. Resource cho phép **client** (người dùng hoặc host) đính kèm ngữ cảnh ổn định, có URI, có thể cache và có thể theo dõi thay đổi. Orca đã có sẵn nguồn dữ liệu (task, annotation, worktree, git diff, PR, file) và kênh sự kiện (NATS, notification-service) — chỉ thiếu lớp phơi theo chuẩn MCP.

## Giải pháp đề xuất

### A. Không gian URI

| URI / template | Nội dung | Nguồn |
|----------------|----------|-------|
| `orca://projects` | Danh sách project | `project.*` |
| `orca://project/{projectId}` | Mô tả project + worktree | `project`, `worktree` |
| `orca://task/{taskId}` | Task + phụ thuộc + hoạt động | `task.*` |
| `orca://worktree/{id}/status` | Trạng thái git (branch, thay đổi) | `git.*` |
| `orca://worktree/{id}/diff{?base}` | Diff đang chờ | `git.*` |
| `orca://worktree/{id}/file/{+path}` | Nội dung file (giới hạn kích thước, chặn path traversal) | `files.*` |
| `orca://review/{provider}/{repo}/{number}` | PR/MR kèm checks (`provider` ∈ github/gitlab/…) | `hostedReview` |
| `orca://terminal/{id}/scrollback` | Scrollback gần nhất | `terminal` scrollback |

Dùng `resources/templates/list` cho các URI có tham số; `resources/list` chỉ liệt kê các resource gốc/ít (phân trang). Trả `mimeType` đúng (`application/json`, `text/x-diff`, `text/plain`…); resource nhị phân dạng `blob` base64 chỉ khi cần và có giới hạn.

### B. Kiểm soát truy cập

Mỗi `resources/read` đi qua **cùng** kiểm tra quyền và policy như tool đọc tương ứng (scope `orca:read` + quyền tenant/project). URI chứa id mà user không có quyền ⇒ trả lỗi "không tìm thấy" (không phân biệt "không tồn tại" và "không có quyền" để khỏi lộ sự tồn tại). Đường dẫn file: chuẩn hoá và khoá trong gốc worktree, từ chối `..`, symlink thoát gốc, file nằm ngoài danh sách cho phép (`.env`, khoá riêng, `.git/config` chứa credential) — dùng cùng danh sách che dữ liệu nhạy cảm với tool `files_read`.

### C. Subscriptions & thông báo

- Khai báo `resources{subscribe:true, listChanged:true}` (CR-003).
- `resources/subscribe` trên task/worktree/PR/terminal: gateway đăng ký subject NATS tương ứng (đã có sự kiện task activity: `channels_task_activity.go`; notification stream) và gửi `notifications/resources/updated` (chỉ URI, client tự `read` lại — tránh đẩy dữ liệu lớn).
- Giới hạn số subscription/session; huỷ khi session đóng.

### D. Dữ liệu không tin cậy

Nội dung đến từ bên ngoài (mô tả PR, issue Jira/Linear, output terminal, file trong repo) có thể chứa chỉ dẫn nhắm vào LLM. Resource gắn `annotations` phù hợp (`audience`, `priority`) và, quan trọng hơn, **không** trộn dữ liệu không tin cậy vào `instructions`/mô tả tool. Xem CR-013 §prompt-injection.

## Acceptance Criteria

- [ ] `resources/list` (phân trang) + `resources/templates/list` đúng; đọc từng URI ở bảng A thành công với dữ liệu mẫu.
- [ ] Đọc file ngoài gốc worktree / qua symlink thoát / `.env` ⇒ bị từ chối (test bảo mật).
- [ ] Resource của tenant khác ⇒ trả "không tìm thấy", không lộ tồn tại.
- [ ] Đổi trạng thái task ⇒ client đã subscribe nhận `resources/updated` ≤ 2s; huỷ khi đóng session.
- [ ] Resource > ngưỡng kích thước bị cắt kèm chỉ dẫn phân trang.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** URI lộ id nội bộ ⇒ id đã là UUID không đoán được, và vẫn luôn kiểm quyền.
- **Ngoài phạm vi:** prompts (CR-011); resource do bên thứ ba cung cấp (CR-014).
