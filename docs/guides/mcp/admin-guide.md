# Hướng dẫn quản trị MCP (admin)

Chỉ tài khoản có vai trò **admin** thấy các tab quản trị. Mọi kênh `mcp.admin.*` kiểm quyền ở phía
server (`MCP_NOT_ADMIN`); ẩn tab chỉ là tiện ích giao diện.

## 1. Bật/tắt MCP và mặc định cho tổ chức

Có hai tầng công tắc:

| Công tắc | Ở đâu | Ý nghĩa |
|---|---|---|
| `MCP_ENABLED` | biến môi trường của **api-gateway** (mặc định `false`) | Công tắc tổng cấp tiến trình. `false` ⇒ `/mcp` và `/.well-known/oauth-*` trả 404, mọi kênh `mcp.*` trừ `mcp.server.info` trả `MCP_DISABLED`; giao diện ẩn mục MCP, không báo lỗi. Bật **cuối cùng** khi triển khai. |
| `MCP_TENANT_DEFAULT_ENABLED` | biến môi trường của **mcp-service** (mặc định `true`) | Giá trị `enabled` của hàng cấu hình tenant **được tạo mới từ giờ**. **Không** đổi hàng đã có, **không** đổi mặc định rủi ro/hard-deny/scope/kill switch. |
| Công tắc tenant | Settings → MCP (admin) | Admin bật/tắt MCP cho tổ chức, đặt `maxTokenDays`, `approvalTtlSeconds`, DCR. |

Khi tenant bị tắt, user thường không thấy gì; admin thấy thẻ **"MCP is turned off for your
organization"** kèm công tắc bật lại (tránh tự khóa mình ngoài hệ thống).

## 2. Policy công cụ

Tab **Tools** liệt kê danh mục công cụ (namespace, rủi ro, scope cần, quyết định hiệu lực và
nguồn: mặc định / policy tenant / hard-deny / kill switch). Tab **Policies** cho phép đặt ghi đè:

- Khớp theo `tool`, `namespace`, `risk`, `clientId`, `roles`; quyết định `allow`, `require_approval`, `deny`.
- Mặc định an toàn: `exec`/`destructive` ⇒ yêu cầu phê duyệt, `admin` ⇒ từ chối. **Hard-deny**
  không thể nới bằng policy (`MCP_POLICY_HARD_DENY`).
- Sửa đồng thời được bảo vệ bằng `version` (`MCP_POLICY_VERSION_CONFLICT` ⇒ tải lại rồi sửa).
- **Explain** cho biết vì sao một lời gọi được/không được phép (decision + reasons).

## 3. Kill switch

Dừng khẩn cấp theo phạm vi: `tenant`, `client`, `grant`, `session`. Bật/tắt cần nhập **lý do**
(hiện cho người dùng trong banner). Hiệu lực: ngay qua event bus, tối đa ~30 giây khi NATS lỗi
(`MCP_KILLSWITCH_POLL`). Người dùng thấy banner ở mọi tab **không cần tải lại**; tạo token mới và
phê duyệt bị chặn, nhưng vẫn thu hồi được. Quy trình sự cố nhanh nhất:
kill switch tenant → `MCP_ENABLED=false` trên gateway (cần restart) → dừng mcp-service.
Đừng chạy migration `down` khi sự cố.

## 4. OAuth clients, grants, phiên

- **OAuth clients**: xem client đã đăng ký (qua DCR hay admin), đặt `allowed`/`blocked`. Client tự
  đăng ký hiện cảnh báo "unverified application" ở trang consent.
- **All grants**: xem/thu hồi quyền đã cấp của mọi người dùng.
- Phiên đang hoạt động của mọi người dùng (admin) để đóng phiên đáng ngờ.

## 5. Nhật ký kiểm toán (Audit log)

Tab **Audit log** truy vấn `mcp.admin.audit.query` theo thời gian, người dùng, công cụ, quyết định
(`allow`/`deny`/`approved`/`denied`/`expired`); xuất CSV. Mỗi dòng có tóm tắt tham số (không phải
giá trị bí mật), kết quả, thời lượng, người phê duyệt, `traceId`. Dùng để trả lời "agent nào đã
làm gì, ai đã duyệt".

## 6. Prompt tuỳ chỉnh

Tab **Prompts**: quản lý prompt MCP của tổ chức (prompt dựng sẵn là chỉ đọc —
`MCP_PROMPT_BUILTIN_READONLY`; trùng tên ⇒ `MCP_PROMPT_NAME_CONFLICT`).

## 7. Giai đoạn rollout hiện hành

Backend: `MCP_ENABLED=false` mặc định tới khi qua cổng chất lượng CR-MCP-015 (bật mà thiếu
`MCP_PUBLIC_BASE_URL`/`PUBLIC_BASE_URL` thì gateway từ chối khởi động). Trên UI, biến build
`VITE_MCP_UI_STAGE=beta` hiện nhãn **Beta** ở mục MCP; để trống thì coi là GA và không có nhãn
(`frontend/src/renderer/src/lib/mcp-labels.ts`).
