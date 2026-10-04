# CR-JIRA-001 — Hỗ trợ Jira Server/Data Center (self-hosted), không chỉ Jira Cloud

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-JIRA-001 |
| **Tên** | `issue-tracking-service`'s Jira adapter hardcode `/rest/api/3/` + auth kiểu Cloud, luôn fail với site self-hosted (VNPay nội bộ) |
| **Loại** | Feature Gap / Bug Fix |
| **Priority** | 🔴 P0 — chặn 100% khả năng kết nối Jira nội bộ VNPay, thông báo lỗi sai lệch (đổ lỗi cho credential) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | ✅ **Fixed & confirmed working (2026-09-15)** — verify trực tiếp bằng PAT thật của user qua Bearer auth: `200 OK`, đúng profile `luatnc@vnpay.vn`. Lưu ý dùng: phải để trống Email khi connect (điền Email sẽ khiến code chọn Basic auth thay vì Bearer, bị site từ chối). |
| **Tác giả** | User report trực tiếp: connect `https://jr.servicehub.vn` với credential hợp lệ, luôn nhận `ISSUETRACKING_AUTH_FAILED` |
| **Tác động HLD** | `backend-go/services/issue-tracking-service/internal/adapter/jira/client.go` (toàn bộ), `jira-connect-dialog.tsx`, `jira-integration-card.tsx` |
| **Tác động Features** | Kết nối Jira, mọi tính năng phụ thuộc (browse issue, tạo worktree từ issue — xem CR-PW-009) |

---

## Bối cảnh & Vấn đề gốc

Xác nhận live bằng `curl` thẳng vào site thật của user (`jr.servicehub.vn`):
```
/rest/api/3/myself  → HTTP 302   (Cloud-only endpoint, site không nhận ra → redirect login)
/rest/api/2/myself  → HTTP 401   (endpoint THẬT của Server/Data Center — tồn tại, chỉ thiếu auth đúng)
```

`internal/adapter/jira/client.go` hardcode `/rest/api/3/` (Jira Cloud-only, không tồn tại trên Server/Data Center) trong **toàn bộ** method (`Whoami`, `ListIssues`, `CreateIssue`, `UpdateIssue`, comment, project, issue-type, assignee...) — không có branch theo loại site. Kết quả: `Whoami` (bước xác thực đầu tiên) không bao giờ chạm được endpoint thật của site self-hosted, luôn fail — **không liên quan gì đến credential**, nhưng `connect.go`'s wrapper trả về `ISSUETRACKING_AUTH_FAILED: could not authenticate with the provided credential` — thông báo sai lệch, khiến user loay hoay kiểm tra lại credential đúng trong khi vấn đề nằm ở tầng code.

Đi kèm: UI copy (`jira-integration-card.tsx`, `jira-connect-dialog.tsx`) hướng dẫn *"Create a token in Atlassian account settings"* — chỉ đúng cho Cloud (`id.atlassian.com`), Server/Data Center tạo Personal Access Token ngay tại chính site (`https://<site>/plugins/servlet/personal-access-tokens`).

Xem đầy đủ bằng chứng ở [BUG-013](../../../../specs/backend-go/bugs/missing-v2/BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md).

## Giải pháp đã implement (2026-09-15)

1. **Phát hiện loại site**: `resolveAPIVersion` gọi `/rest/api/2/serverInfo` (tồn tại trên cả Cloud lẫn Server/Data Center), đọc `deploymentType` — không cần thêm toggle UI, tự động hoàn toàn. Cache 10 phút theo `baseURL`.
2. **Branch API version**: áp dụng cho TOÀN BỘ method trong `client.go` qua helper `apiURL(baseURL, apiVersion, path)`, không riêng `Whoami`.
3. **Branch auth scheme**: `authHeaderValue` — có `Email` → `Basic email:token` (giữ nguyên hành vi Cloud); không có `Email` → `Bearer <token>` (coi là Personal Access Token của Server/Data Center). Đây là lựa chọn hợp lý theo tài liệu Atlassian, **chưa xác nhận trực tiếp với đội vận hành Jira VNPay** — cần user thử kết nối thật (để trống Email, dán PAT tạo từ chính `jr.servicehub.vn` vào Token) sau khi deploy để xác nhận cuối cùng.
4. **UI copy**: chưa sửa (vẫn ghi "Create a token in Atlassian account settings") — không chặn việc kết nối, chỉ là hướng dẫn chưa chính xác cho site self-hosted; để lại cho lượt sau.

Test: 10/10 pass trong package `jira`, gồm 2 test mới (`TestWhoami_SelfHostedDataCenter_UsesV2AndBearerAuth`, `TestWhoami_CloudSite_StillUsesV3AndBasicAuth`) xác nhận cả 2 nhánh hành vi, Cloud không bị ảnh hưởng.

## Không thuộc phạm vi CR này

- Thiết kế lại toàn bộ UI kết nối Jira (giữ nguyên form 3 field hiện tại: site URL/email/token — chỉ thêm phân nhánh xử lý phía sau).

## Liên quan

- [BUG-013](../../../../specs/backend-go/bugs/missing-v2/BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md)
- [`missing-v1`/BUG-015](../../../../specs/backend-go/bugs/missing-v1/BUG-015-jira-channels-not-implemented.md) / [SOL-015](../../../../specs/backend-go/bugs/missing-v1/solutions/SOL-015-jira-channels.md) — nơi tích hợp Jira Cloud gốc được xây, đúng phạm vi lúc đó; CR này mở rộng, không phải sửa lỗi của SOL-015
- [CR-PW-009](../v3/project-workspace/CR-PW-009-close-backend-only-ui-gaps-in-project-workspace.md) — `worktree.createFromIssue` phụ thuộc Jira connect hoạt động được trước
