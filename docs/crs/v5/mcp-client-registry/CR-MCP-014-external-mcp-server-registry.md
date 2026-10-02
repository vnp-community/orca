# CR-MCP-014 — Registry MCP server ngoài và cấp MCP cho agent do Orca chạy

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-014 |
| **Tên** | `mcp-service` quản lý registry server ngoài (stdio/HTTP), secret qua Vault, kiểm tra an toàn, và sinh cấu hình MCP cho tiến trình agent |
| **Loại** | Feature |
| **Priority** | 🟡 P2 |
| **Effort** | Large (8–10 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `tenant-service/internal/domain/profile_resolution.go:231-282`, `agent/src/shared/mcp-config.ts` |
| **Phụ thuộc** | [CR-MCP-001](../mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md); khuyến nghị sau [CR-MCP-013](../mcp-governance-safety/CR-MCP-013-approvals-audit-killswitch.md) |

---

## Bối cảnh & Vấn đề

- `mergeMCPServers` (tenant-service) chỉ **gộp và loại trùng** `mcp.servers` theo `name` qua các lớp profile (user thắng) rồi trả về — không xác thực nội dung, không quản lý secret, không biết server nào "đang chạy".
- `agent/src/shared/mcp-config.ts` đọc `.mcp.json` / Cursor / Claude config và tóm tắt (`stdio`/`http`, `enabled|disabled|invalid`) phía máy dev — chỉ để hiển thị, không phải nguồn sự thật có kiểm soát.
- Cấu hình MCP server thường chứa **secret** (`env`, header `Authorization`) và **lệnh thực thi** (`command` của stdio) — cả hai là rủi ro nếu chép tuỳ tiện giữa các lớp profile.
- Khi Orca khởi chạy agent CLI cho user, agent nên nhận: (1) MCP của chính Orca (với token hạn chế, `mcp_depth+1`), (2) các server ngoài đã được duyệt.

## Giải pháp đề xuất

### A. Registry ở `mcp-service` (`external_servers`)

Trường: `id, tenant_id, scope (tenant|team|user), name, transport (http|stdio), url | command+args, env_refs, header_refs, status (pending_review|approved|disabled), created_by, reviewed_by`. **`env_refs/header_refs` là tham chiếu secret** (Vault/`credential-broker-service`), không chứa giá trị. Quan hệ với profile: `profile.mcp.servers[].name` ánh xạ tới bản ghi registry; entry chưa có/chưa duyệt ⇒ bị bỏ qua và cảnh báo (giữ nguyên quy tắc gộp "user thắng" nhưng **chỉ trong tập đã duyệt**).

### B. Kiểm tra an toàn khi đăng ký/sử dụng

- **HTTP:** chặn SSRF (loopback, link-local, dải riêng, metadata cloud `169.254.169.254`, DNS rebinding — resolve rồi pin IP), chỉ `https` (trừ allow-list dev), timeout/size limit.
- **stdio:** chạy `command` là thực thi mã tuỳ ý ⇒ chỉ cho phép khi admin tenant duyệt; tuỳ chọn allow-list lệnh (`npx`/`uvx` + gói đã duyệt, ghim phiên bản/hash); chạy trong sandbox (không kế thừa env của tiến trình cha).
- Thông tin server ngoài (tên tool, mô tả) là **dữ liệu không tin cậy** (tool poisoning): hiển thị cho admin khi duyệt, ghim/hash mô tả, cảnh báo khi mô tả đổi ("rug pull").
- Không bao giờ chuyển token của người dùng Orca sang server ngoài (cấm token passthrough — CR-005).

### C. Cấp MCP cho agent do Orca khởi chạy

`mcp-service.ResolveAgentMcpConfig(tenant, user, project, agent)` ⇒ trả cấu hình (định dạng của từng agent CLI) gồm: `orca` (URL `/mcp` + token MCP ngắn hạn, scope hẹp, `mcp_depth+1`) và các server ngoài đã duyệt, secret được **giải tại thời điểm spawn** và chỉ truyền qua env của tiến trình con. Hiệp ước với phần `agent/`/relay (spawn agent): đây là nơi duy nhất cần thay đổi bên `agent/` (ngoài phạm vi CR này, ghi nhận phụ thuộc). Hoạt động cả trường hợp SSH (cấu hình sinh ở backend, đẩy xuống host đích).

### D. Quan sát & vòng đời

Health check định kỳ server HTTP đã duyệt (trạng thái hiển thị cho admin); audit mọi thay đổi registry; thu hồi (disable) có hiệu lực cho lần spawn kế (và thông báo tiến trình đang chạy nếu cơ chế cho phép).

## Acceptance Criteria

- [ ] CRUD + duyệt registry; entry `pending_review` không được cấp cho agent.
- [ ] URL trỏ `169.254.169.254`/`127.0.0.1`/DNS đổi IP sau khi duyệt ⇒ bị chặn (test SSRF).
- [ ] Secret không xuất hiện trong DB, log, trace, API response; chỉ có trong env tiến trình con. **Đã chốt (D1):** UI gửi plaintext một lần qua WS/TLS (`mcp.externalServer.setSecret{serverId,kind,name,value}`), không dùng phong bì client; `mcp-service` ghi vào `credential-broker-service` (category `mcp_external_secret`, Vault Transit); `value` bị che ở log WS và trace store, response không echo; có test che `value`.
- [ ] Migration broker sửa CHECK `category` (đã xác minh chỉ có 5 giá trị ở `postgres/` và `mysql/`) để nhận `mcp_external_secret`.
- [ ] Mô tả tool của server ngoài đổi so với bản đã duyệt ⇒ cảnh báo + yêu cầu duyệt lại.
- [ ] Agent spawn nhận đúng cấu hình; `mcp_depth` tăng; vượt ngưỡng ⇒ không cấp MCP của Orca.
- [ ] `mergeMCPServers` vẫn đúng (test hiện có `profile_resolution_test.go` xanh).

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** stdio server = thực thi mã ⇒ mặc định tắt, chỉ admin bật.
- **Rủi ro:** thay đổi `tenant-service` ⇒ chạy `gitnexus_impact` trên `mergeMCPServers` trước khi sửa; ưu tiên **không đổi** hàm đó mà thêm bước lọc ở nơi gọi.
- **Ngoài phạm vi:** sửa `agent/` để nhận cấu hình (CR riêng phía agent), UI quản lý registry (frontend).
