# CR-DS-008 — First-Login Department Gate & Dev Server Access Request

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-008 |
| **Tên** | First-Login Department Selection Gate + Access Request Flow |
| **Loại** | Feature — Multi-tenant Access Control (Phase 3) |
| **Priority** | P2 — Medium (depends on CR-DS-006/007 shipping first) |
| **Phiên bản** | v6.1 |
| **Ngày tạo** | 2026-08-28 |
| **Trạng thái** | ✅ Hoàn tất (backend + frontend) — access-request flow (BE-SOL-004), Department Gate + skip-onboarding role branch + access-request form (FE-SOL-002/003), deployed b15.openledger.vn |
| **Phụ thuộc** | CR-DS-006, CR-DS-007, tenant-service Department model |
| **Tác động HLD** | onboarding flow (docs/crs/v1/onboarding) |

---

## 1. Bối cảnh

Hai yêu cầu cụ thể:

1. User đăng nhập lần đầu (chưa có `department_id` trong `tenant.user_profiles`) → phải chọn phòng ban **trước khi** thấy onboarding hoặc dùng bất kỳ tính năng nào của Orca. Hiện tại `department_id=""` được tenant-service coi là trạng thái hợp lệ ("company-only inheritance"), không phải trạng thái "chưa chọn, phải chặn" — CR này thêm ý nghĩa gate cho giá trị rỗng đó ở lớp frontend/gateway, KHÔNG đổi ý nghĩa của nó ở tenant-service (tránh phá vỡ semantics settings-inheritance hiện có).
2. Nút "Skip onboarding":
   - **Admin**: cho phép skip, chuyển hướng thẳng tới Settings.
   - **User thường**: nếu chưa có quyền vào dev server nào (theo CR-DS-007's `ListDevServersForUser` rỗng) → chặn Skip, hiển thị form "Yêu cầu quyền truy cập dev server" thay vì đóng onboarding.

## 2. Giải pháp

### 2.1 First-login gate
- Endpoint mới `GET /v1/me/department-status` (hoặc RPC tương đương) — trả `{ hasDepartment: boolean }` dựa trên `GetResolvedProfile`.
- Frontend: một layer gate mới bọc ngoài `OnboardingFlow`/toàn bộ app shell — nếu `hasDepartment === false`, render `DepartmentSelectionScreen` (chọn 1 trong danh sách department của tenant, gọi `SetUserDepartment`) thay vì `OnboardingFlow`. Sau khi chọn xong mới cho vào onboarding bình thường.
- **Quyết định cần xác nhận**: admin có bị gate không, hay admin luôn bỏ qua bước chọn phòng ban? (Đề xuất: admin cũng nên chọn, để nhất quán dữ liệu — nhưng cần xác nhận vì có thể admin quản lý nhiều phòng ban cùng lúc, không thuộc riêng 1 phòng ban nào.)

### 2.2 Skip onboarding — rẽ nhánh theo role
Sửa `OnboardingFlow.tsx`'s `confirmSkipOnboarding`:
```
if (currentUser.role === 'admin') {
  await dismissOnboarding()
  navigateTo('settings')
} else {
  const accessibleServers = await listDevServersForUser()
  if (accessibleServers.length > 0) {
    await dismissOnboarding()   // user đã có server, skip bình thường
  } else {
    openAccessRequestForm()     // chặn skip, bắt gửi yêu cầu
  }
}
```

### 2.3 Access request flow (backend-go)
- Bảng mới `infra.dev_server_access_requests(id, tenant_id, user_id, dev_server_group_id, status, message, created_at, resolved_at, resolved_by)`.
- Usecase: `CreateAccessRequest`, `ListPendingAccessRequests` (admin), `ResolveAccessRequest` (approve → tạo `dev_server_group_grants` row cho user's department/team; reject → ghi nhận, không tạo grant).
- UI: form đơn giản (chọn group muốn xin quyền + lời nhắn) cho user; danh sách pending request + nút approve/reject cho admin (có thể gộp vào cùng màn hình admin approve agent ở CR-DS-006 Phase 2 cho đỡ phân mảnh UI).

## 3. Điểm cần quyết định trước khi triển khai — ĐÃ CHỐT (defaults đã áp dụng, có thể đảo ngược)

1. **Admin có bị gate chọn phòng ban không**: KHÔNG — admin bypass hoàn
   toàn (cùng hướng với admin bypass skip-onboarding ở mục 2.2). Admin quản
   lý cả tổ chức từ Admin Console (duyệt agent, gán nhóm, cấp quyền theo
   phòng ban) — bắt admin chọn 1 phòng ban trước sẽ chặn chính console dùng
   để gán phòng ban cho người khác.
2. **Access request nhắm vào 1 group cụ thể**: đã chọn phương án này —
   `AccessRequestDialog` cho user chọn 1 `DevServerGroup` cụ thể từ
   `devServerGroup.list()` (xác nhận: RPC này KHÔNG thực sự admin-gate ở
   usecase layer — `ListDevServerGroups.Execute` không gọi `requireAdmin`,
   chỉ tenant-scoped — nên user thường gọi được, không cần nới quyền gì
   thêm).
3. **Email/notification cho admin**: KHÔNG làm ở bản này — chỉ hiện trong
   tab "Access requests" của Admin Console (FE-SOL-001), admin phải tự vào
   xem. Notification là follow-up riêng, chưa có CR.

## 4. Acceptance Criteria

- [x] User chưa có department không thấy được onboarding/tính năng nào khác ngoài màn hình chọn phòng ban (`DepartmentGate`, admin bypass).
- [x] Admin bấm Skip → về Settings ngay, không bị chặn.
- [x] User thường bấm Skip, có sẵn ≥1 dev server được cấp quyền → skip bình thường.
- [x] User thường bấm Skip, không có dev server nào → bắt buộc thấy form gửi yêu cầu (`AccessRequestDialog`), không có lối "skip im lặng" — chỉ có Cancel (quay lại onboarding) hoặc Send request.
- [x] Admin thấy danh sách access request đang chờ và approve/reject được (Admin Console's "Access requests" tab).
- [x] 3 điểm cần quyết định ở mục 3 đã được chốt (xem trên) — defaults tài liệu hoá, có thể đảo ngược nếu cần.
