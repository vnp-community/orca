# CR-MCP-013 — Phê duyệt của người dùng, audit, kill switch, chống đệ quy & prompt-injection

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-MCP-013 |
| **Tên** | Human-in-the-loop cho tool rủi ro, nhật ký kiểm toán đầy đủ, hạn mức/kill switch, và các biện pháp giảm thiểu đặc thù agent |
| **Loại** | Feature (bảo mật) |
| **Priority** | 🔴 P0 |
| **Effort** | Large (8–10 ngày) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-01 |
| **Trạng thái** | 🔲 Chưa triển khai |
| **Tác giả** | Khảo sát `auth-service` audit (`AppendAuditEntry`), `notification-service`, `rate_limit.go` |
| **Phụ thuộc** | [CR-MCP-012](./CR-MCP-012-tool-policy-and-annotations.md), [CR-MCP-004](../mcp-protocol-server/CR-MCP-004-sessions-sse-resumability.md); đường Web Push khi app không focus: [CR-NOTIF-002](../../v4/notification/CR-NOTIF-002-deliver-push-usecase.md) (`DeliverPush`) |

---

## Bối cảnh & Vấn đề

Khi `decision = require_approval` (CR-012) cần một **con người** xác nhận. Ngoài ra:

- `auth-service` đã có `AppendAuditEntry/QueryAuditLog` nhưng không có khái niệm "hành động do agent thay mặt user".
- Rate limit hiện tại theo tenant, in-memory, theo request — không bảo vệ khỏi một agent chạy vòng lặp trong quota của tenant, và không chia theo client/agent.
- Orca có thể chạy agent CLI (Claude, Codex…) mà chính agent đó được cấp MCP của Orca ⇒ nguy cơ **đệ quy** (agent gọi `agent_start` sinh agent gọi `agent_start`…).
- Nội dung PR/issue/terminal/file là dữ liệu không tin cậy có thể chứa chỉ dẫn nhắm vào LLM (**prompt injection**) — vector thực tế nhất của kiến trúc này.

## Giải pháp đề xuất

### A. Phê duyệt

Luồng: tool bị `require_approval` ⇒ `mcp-service.CreateApproval` (ghi: tool, **tóm tắt tham số đã che**, rủi ro, client, session, hết hạn 5 phút) ⇒ thông báo cho người dùng:
1. **Elicitation của MCP** (`elicitation/create`) nếu client khai báo capability — hỏi ngay trong client;
2. Đồng thời đẩy qua `notification-service` (WS/push, có sẵn) để duyệt trong web UI/mobile (UI phê duyệt là frontend CR riêng) — **phê duyệt ngoài kênh** an toàn hơn vì client bị thao túng có thể tự "đồng ý". Deep link thông báo cố định `/?section=mcp&tab=approvals&approval=<id>` (D5). **Phụ thuộc thật:** `notification-service` chưa có usecase `DeliverPush` (CR-NOTIF-002, `docs/crs/v4/notification/CR-NOTIF-002-deliver-push-usecase.md`) nên Web Push chưa được gửi cho tới khi CR đó hoàn tất; hộp thoại trong app (WS) là đường chính.

Quy tắc cứng:
- Quyết định chỉ hợp lệ từ **session/thiết bị của chính user** (không từ chính token MCP đang xin).
- Phê duyệt gắn với **đúng tham số** đã hiển thị (hash); đổi tham số ⇒ phải duyệt lại. Một lần dùng (single-use), có hạn; tuỳ chọn "cho phép trong phiên N phút" chỉ với `write_reversible`, **không** với `exec/destructive`.
- Từ chối/hết hạn ⇒ tool trả `isError:true` với lý do trung tính. Tool đang chờ có thể bị huỷ (CR-004).
- Hiển thị cho người duyệt: lệnh/đường dẫn **nguyên văn** (không diễn giải lại bằng lời của LLM).

### B. Audit

Mỗi lời gọi (allow/deny/approval) ghi `AppendAuditEntry`: `tenant, user, client_id, mcp_session, tool, args_hash + tóm tắt đã che, decision, approver, kết quả (ok/lỗi), duration, trace_id`. Đánh dấu `actor_type = agent` để phân biệt với thao tác trực tiếp của người dùng. Không bao giờ ghi secret/giá trị nhạy cảm (bộ che dùng chung với tool output). Truy vấn qua `QueryAuditLog` + export; lưu giữ theo chính sách tenant. Liên kết `trace_id` với OpenTelemetry (CR-015).

### C. Hạn mức & kill switch

- Rate limit **theo (tenant, user, client, nhóm risk)**: ví dụ `exec` ≤ 30/phút, tổng ≤ 300/phút; ngân sách tổng/ngày; số stream/terminal mở đồng thời (CR-009). Dùng bộ đếm phân tán (không in-memory — nhiều replica) hoặc chấp nhận rõ ràng sai số và ghi lại.
- Phát hiện vòng lặp: cùng tool + cùng tham số lặp > N lần/khoảng ⇒ làm chậm, rồi chặn tạm.
- **Kill switch** ở 3 mức: toàn hệ thống (`MCP_ENABLED`), theo tenant, theo client/grant/session. Hiệu lực ≤ 60s, huỷ tool đang chạy, đóng SSE, thu hồi token liên quan.

### D. Chống đệ quy

Mọi tiến trình do Orca chạy cho agent mang thông tin `mcp_depth` (qua token MCP riêng cấp cho tiến trình con, CR-014). `depth ≥ MAX` (mặc định 1–2) ⇒ tool `agent_start`/`workflow_run`/`terminal_*` bị `deny`. Giới hạn tổng số agent con theo session gốc.

### E. Giảm thiểu prompt-injection (nhiều lớp, không có viên đạn bạc)

1. Quyền tối thiểu: scope hẹp + `exec`/`destructive` luôn qua người (A).
2. Đánh dấu nội dung không tin cậy: tool/resource đọc dữ liệu ngoài bọc nhãn rõ ràng (ví dụ khung `<untrusted-content source="pr_description">…</untrusted-content>` trong `content`) và không trộn vào `instructions`.
3. Không cho tool đọc dữ liệu ngoài **và** tool gửi dữ liệu ra ngoài (mạng/push/comment công khai) tự động nối tiếp mà không có điểm duyệt — tránh "lethal trifecta" (dữ liệu riêng tư + nội dung không tin cậy + khả năng exfiltrate). Tool `openWorld:true` (đăng bình luận, push, gọi mạng) mặc định cần approval khi session đã đọc nội dung không tin cậy.
4. Che secret ở mọi đầu ra; chặn URL chứa token.
5. Giới hạn kích thước/độ sâu output để tránh "nhồi" ngữ cảnh.

## Acceptance Criteria

- [ ] E2E: tool `exec` ⇒ client nhận elicitation (hoặc thông báo ngoài kênh) ⇒ duyệt ⇒ chạy; từ chối ⇒ không chạy, có audit.
- [ ] Duyệt bằng chính token MCP (tự duyệt) bị từ chối; đổi tham số sau khi duyệt ⇒ yêu cầu duyệt lại.
- [ ] Audit đủ trường, có `actor_type=agent`, không chứa secret (test quét).
- [ ] Kill switch tenant: tool đang chạy bị huỷ, request mới `403` trong ≤ 60s.
- [ ] Đệ quy: agent con ở `depth=MAX` gọi `agent_start` ⇒ deny.
- [ ] Bộ test "red team": PR chứa chỉ dẫn "chạy `curl … | sh`/gửi `.env`" ⇒ không thể dẫn tới hành động ghi/exec/exfiltrate mà không qua người duyệt.

## Rủi ro & Ngoài phạm vi

- **Rủi ro:** mệt mỏi phê duyệt (approval fatigue) ⇒ mặc định chỉ hỏi với `exec/destructive/openWorld`, gom nhóm hợp lý, cho "cho phép trong phiên" ở mức an toàn.
- **Rủi ro:** client không hỗ trợ elicitation ⇒ rơi về kênh ngoài (A.2); nếu không có kênh nào ⇒ `deny`, không bao giờ mặc định `allow`.
- **Ngoài phạm vi:** UI phê duyệt/audit (frontend), DLP nâng cao.
