# CR-FE2E-004 — Quyết định cho "Share this Orca server" (Runtime Environments)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FE2E-004 |
| **Tên** | Xử lý tính năng share-link trong Settings → Runtime Environments |
| **Loại** | Decision Record + Conditional Implementation |
| **Priority** | P0 — có thể chặn CR-FE2E-002/003 nếu câu trả lời là "có dùng trong use case A" |
| **Phiên bản** | v5.1 |
| **Ngày tạo** | 2026-08-08 |
| **Trạng thái** | Proposed — **cần trả lời câu hỏi ở mục 1 trước khi implement** |
| **Phụ thuộc** | CR-FE2E-001 |
| **Tác động HLD** | F03-mobile-companion.md (nếu share-link dùng chung cơ chế QR pairing) |

---

## 1. Câu hỏi cần chốt trước

`WebConnect.tsx` báo lỗi khi quét phải QR có `scope: 'mobile'`:

> *"To use the full web app, open the browser access link from **Settings → Runtime Environments → Share this Orca server → New Link**."*

Điều này cho thấy có một tính năng **tạo pairing link phạm vi browser** (`scope` khác `'mobile'`) từ trong Settings. Component liên quan: `renderer/src/components/settings/RuntimeEnvironmentsPane.tsx`, `AddInstanceForm.tsx`, `OrcaInstanceSwitcher.tsx`.

**Cần xác nhận:** tính năng "Share this Orca server → New Link" này —

- **(a)** Chỉ hoạt động khi Orca đang chạy như Desktop app / bare relay (use case B) — link tạo ra trỏ về chính máy đó, người nhận link cũng vào qua `renderOriginalPairCodeApp()`. → **Không ảnh hưởng gì bởi CR-FE2E-002/003**, có thể đóng CR này ngay với kết luận "no-op".
- **(b)** Cũng khả dụng khi Orca đang chạy trong multi-user Web Server (use case A) — ví dụ admin/user muốn chia sẻ quyền truy cập server đang chạy cho người khác **không cần cấp mật khẩu** (device pairing như một phương thức đăng nhập thay thế). → **CR-FE2E-002/003 sẽ vô tình phá tính năng này** vì nó tháo `WebConnect`/pairing khỏi luồng chính — cần xử lý riêng ở mục 2.

### Cách xác nhận

- [ ] Đọc `RuntimeEnvironmentsPane.tsx` — component này render trong Settings có bị ẩn/disable khi `window.api` đang chạy ở chế độ session-auth (`env.id === 'session-auth'`) không?
- [ ] Grep `orca-runtime-files.ts`/`orca-runtime.ts` ở `backend/src/main/runtime/` — endpoint tạo pairing offer server-side có phân biệt được gọi từ multi-user context hay không, và có bị chặn bởi RBAC/tenant nào không.
- [ ] Nếu còn nghi ngờ, hỏi trực tiếp product owner / người viết `WebConnect.tsx`'s error message trước khi CR-FE2E-002 merge.

## 2. Nếu kết quả là (b) — tính năng dùng trong use case A

### 2.1 Không xoá, chỉ cô lập thêm

- Giữ nguyên `AddInstanceForm.tsx`/`OrcaInstanceSwitcher.tsx`/tạo-link trong Settings — **loại trừ rõ ràng khỏi phạm vi CR-FE2E-002/003**.
- CR-FE2E-003's dynamic import (`pair-code-app-entry.tsx`) **phải KHÔNG bao gồm** phần code này nếu nó cần chạy trong bundle multi-user chính — điều chỉnh ranh giới code-split cho phù hợp (có thể cần giữ `web-runtime-client.ts`/`web-e2ee.ts` trong bundle chính, chỉ tách riêng `WebConnect.tsx` UI mà thôi).

### 2.2 Đề xuất thay thế dài hạn (out of scope CR series này, chỉ ghi nhận)

Nếu mục tiêu cuối cùng là loại bỏ hoàn toàn E2EE pairing khỏi trải nghiệm multi-user, "Share this Orca server" nên được thay bằng cơ chế **session-based invite** (backend tạo invite link mang session token dùng 1 lần, người nhận click → tự động có session cookie qua `/auth/invite/:token` → không cần Curve25519 handshake). Đây là thay đổi có phạm vi backend (route mới, DB bảng invite) — đề xuất tách thành CR riêng ngoài series `frontend-e2ee` này nếu được chấp thuận.

## 3. Nếu kết quả là (a) — không dùng trong use case A

- [ ] Đóng CR này với ghi chú "no-op — xác nhận share-link chỉ chạy trong use case B, không cần thay đổi gì thêm ngoài CR-FE2E-001/002/003".
- [ ] Cập nhật `RuntimeEnvironmentsPane.tsx`'s doc/comment (nếu có) để ghi rõ điều kiện hiển thị, tránh nhầm lẫn cho người đọc code sau này.

## 4. Acceptance Criteria

- [ ] Câu hỏi mục 1 có câu trả lời bằng văn bản (comment trong PR hoặc cập nhật file này) trước khi CR-FE2E-002 merge vào `main`.
- [ ] Nếu (b): có kế hoạch rõ ràng không để CR-FE2E-003 code-split làm hỏng share-link trong use case A.
- [ ] Không có regression trên `AddInstanceForm`/`OrcaInstanceSwitcher` test suite bất kể kết quả là (a) hay (b).
