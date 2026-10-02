# CR-MCP-012 — Policy theo tool (OPA), annotations và deny-list cứng

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-012 |
| **Tên** | Cổng chính sách trước mỗi `tools/call`/`resources/read`: scope × quyền user × policy tenant × mức rủi ro tool |
| **Loại** | Feature (bảo mật) |
| **Priority** | 🔴 P0 |
| **Effort** | Medium (5–7 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `backend-go/policy/orca-authz`, `auth-service` (`opaclient`, `AccessPolicy`) |
| **Phụ thuộc** | [CR-MCP-006](../mcp-authorization/CR-MCP-006-mcp-tokens-scopes-and-role.md), [CR-MCP-007](../mcp-tool-catalog/CR-MCP-007-registry-introspection-and-descriptors.md) |

---

## Bối cảnh & Vấn đề

- Handler của UI giả định người gọi là **con người** đang nhìn màn hình: nhiều thao tác nguy hiểm (xoá, push, chạy lệnh) chỉ được "bảo vệ" bằng hộp thoại xác nhận ở frontend — vốn **không tồn tại** với agent.
- api-gateway chưa có bước authz trước routing (README). `auth-service` có `AccessPolicy` (CRUD + versioning + publisher) và `opaclient`; `policy/orca-authz` là bundle OPA. Chưa có gì ánh xạ "tool MCP" vào policy đó.
- Annotation của MCP (`readOnlyHint`, `destructiveHint`…) chỉ là **gợi ý** cho client; spec nói rõ không được tin khi server không đáng tin — và ngược lại, ta không được dùng chúng làm cơ chế enforcement.

## Giải pháp đề xuất

### A. Mô hình quyết định

```
allow(tool call) =
    token.scopes ⊇ tool.requiredScope
  ∧ user được phép theo RBAC hiện có (Identity.Role, quyền project — CR-006)
  ∧ tenantPolicy(tool, user, client, project) = allow | require_approval | deny
  ∧ ¬ hardDeny(tool)
  ∧ trong hạn mức (CR-013)
```

Kết quả ba trạng thái: `allow`, `require_approval` (→ CR-013), `deny` (kèm lý do an toàn để hiển thị cho LLM, không lộ chi tiết policy).

### B. Tích hợp OPA

- Mở rộng bundle `policy/orca-authz` với package `orca.mcp` (input: `tenant`, `user.role`, `client_id`, `tool{name,risk,scope,namespace}`, `args_summary` đã che, `time`). Kiểm `ci/check-opa-bundle-in-images.sh` vẫn xanh.
- api-gateway gọi OPA qua cùng cơ chế `auth-service` đang dùng (không tạo client OPA thứ hai nếu tái dùng được; cần ADR nếu gọi trực tiếp). Quyết định **fail-closed** khi OPA không phản hồi (trừ tool `readOnly` có thể cấu hình fail-open có chủ đích — mặc định đóng).
- Policy tenant lưu ở `mcp-service.tool_policies`, có version + audit, publish qua cơ chế hiện có của `policypublisher`.

### C. Mức rủi ro của tool & mặc định

| Risk | Ví dụ | Mặc định |
|------|-------|----------|
| `read` | `task_list`, `git_diff` | allow |
| `write_reversible` | `task_create`, `git_commit` | allow (có thể hạ về approval) |
| `exec` | `terminal_send`, `agent_start`, `workflow_run` | **require_approval** |
| `destructive` | xoá worktree, force-push, merge PR | **require_approval** + chỉ khi tenant bật |
| `admin` | quản trị user/policy | **deny** trừ khi tenant bật + role admin |

### D. Deny-list cứng (không policy nào mở được)

Đọc/ghi giá trị secret (`credentials.*`), phát hành/thu hồi token & phiên (`auth.*`, `force-revoke`), tạo/sửa user, sửa chính policy MCP, gỡ audit, đổi tenant. Agent **không** tự cấp thêm quyền cho mình.

### E. Cho admin tenant

API quản trị (qua api-gateway REST, không qua MCP): xem catalog + trạng thái từng tool, đặt policy (allow/approval/deny) theo tool/namespace/client/nhóm user, chặn client OAuth cụ thể, bật/tắt MCP toàn tenant. UI là frontend CR riêng.

## Acceptance Criteria

- [ ] Bảng chân trị (scope × role × policy × risk) có test table-driven cho ≥ 40 tổ hợp, gồm cả OPA lỗi ⇒ deny.
- [ ] Tool `deny` không xuất hiện trong `tools/list` của (tenant,user) đó và gọi trực tiếp vẫn bị chặn (không chỉ ẩn).
- [ ] Deny-list cứng không thể mở bằng policy tenant (test).
- [ ] **Đã chốt (D6):** tenant mới có MCP **bật mặc định** (`MCP_TENANT_DEFAULT_ENABLED`, mặc định `true`, `mcp-service` đọc khi tạo lười hàng settings; admin tắt được; `MCP_ENABLED` vẫn là công tắc tổng, mặc định `false` tới gate CR-015). Cờ này **chỉ** đặt `enabled`: mặc định rủi ro ở bảng trên, hard-deny, scope và kill switch không bị nó thay đổi — có test `MCP_TENANT_DEFAULT_ENABLED=true` không đổi mặc định rủi ro.
- [ ] Đổi policy ⇒ session đang mở nhận `tools/list_changed` ≤ 5s và hiệu lực ngay ở lần gọi kế.
- [ ] Mọi quyết định (kể cả allow) có bản ghi để CR-013 audit.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** policy phức tạp khó debug ⇒ công cụ "giải thích quyết định" (`why denied`) cho admin.
- **Rủi ro:** độ trễ OPA mỗi lần gọi ⇒ cache quyết định ngắn theo (tenant,user,tool) và vô hiệu khi policy đổi.
- **Ngoài phạm vi:** phê duyệt tương tác (CR-013).
