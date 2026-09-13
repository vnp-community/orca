# Jira — kết nối làm Task Source trong Orca

**Cập nhật:** 2026-09-13 (trả lời câu hỏi hỗ trợ trên `b15.openledger.vn` — xác nhận lại bằng
code `frontend/src/renderer/src/components/jira-connect-dialog.tsx` +
`backend-go/services/issue-tracking-service/internal/usecase/connect.go`)

Tài liệu này mô tả cách người dùng tự kết nối Jira Cloud vào Orca để browse/tạo/start-work từ
Jira issue ngay trong trang Tasks, và cách phạm vi kết nối được scope theo từng tài khoản đăng
nhập.

## 1. Jira là một "Task Source", không phải cấu hình cấp hệ thống

Orca coi Jira như một trong các nguồn work item trên trang **Tasks** (`TaskPage.tsx`), cùng
nhóm với GitHub/GitLab/Linear — xem khái niệm "Task (Source)" ở
[task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 1. Không có bước "admin bật Jira cho cả tenant" — mỗi người dùng tự kết nối tài khoản Jira
Cloud của chính họ.

## 2. Cách kết nối (từ UI)

1. Vào trang **Tasks**.
2. Ở thanh chọn nguồn task, chọn **Jira** (nếu đang ẩn, cần "Show" nguồn Jira trước).
3. Vì chưa kết nối, màn hình hiện nút **"Connect Jira"** ngay tại đó — cùng dialog với
   **Settings → Task Sources → thẻ Jira → "Connect Jira"**
   (`frontend/src/renderer/src/components/settings/jira-integration-card.tsx`).
4. Điền 3 trường trong `JiraConnectDialog`
   (`frontend/src/renderer/src/components/jira-connect-dialog.tsx`):

   | Trường | Giá trị |
   |---|---|
   | Site URL | Địa chỉ Jira Cloud, vd `https://yourcompany.atlassian.net` |
   | Email | Email tài khoản Atlassian |
   | API Token | Tạo tại `id.atlassian.com/manage-profile/security/api-tokens` — Jira Cloud dùng API token, không dùng mật khẩu trực tiếp |

5. Bấm **Connect**. Backend xác thực thật với Jira (`provider.Whoami`) **trước khi lưu** —
   token/site sai sẽ báo lỗi ngay, không tạo ra một kết nối "connected" hỏng
   (`issue-tracking-service/internal/usecase/connect.go`, comment SOL-015).

Có thể kết nối **nhiều site Jira cùng lúc** (nút "Add Jira site" khi đã có ít nhất 1 site).

## 3. Sau khi kết nối

- Jira issue hiện trực tiếp trong tab **Jira** của trang Tasks — browse theo từng Jira Project,
  sort/filter (`use-task-page-jira-browse-state.ts`, `jira-issue-sorter.ts`).
- Có thể **"start work" trực tiếp từ 1 Jira issue** → Orca tạo worktree/task gắn với issue đó
  (chuyển Jira issue thành một task chạy thật trong Orca).
- Nút **Test** trên từng site (Settings → thẻ Jira) xác thực lại kết nối theo yêu cầu, không chỉ
  dựa vào lần connect ban đầu.

## 4. Phạm vi theo tài khoản đăng nhập — mặc định, không cần cấu hình thêm

Kết nối Jira được lưu gắn với **`(tenant_id, user_id)`** của người đang đăng nhập, không phải
cấp tenant hay cấp server:

```go
// issue-tracking-service/internal/usecase/connect.go — Connect.Execute
credID, err := uc.credentials.Write(ctx, tenantID, userID, in.Provider, cred)
...
status, err := uc.connections.Upsert(ctx, tenantID, userID, in.Provider, workspace, viewer, credID)
```

→ **Mỗi tài khoản đăng nhập Orca tự kết nối Jira của riêng mình**, và chỉ nhìn thấy issue từ
site/account Jira mà chính họ đã connect. Hai người dùng đăng nhập khác nhau trên cùng
`b15.openledger.vn` có hai kết nối Jira độc lập, không lẫn vào nhau — đây là hành vi mặc định
đã có sẵn, không phải tính năng cần bật thêm.

## 5. Nơi lưu token — tuỳ theo runtime đang active

Dòng chữ nhỏ trong `JiraConnectDialog` nói rõ nơi lưu token theo runtime hiện tại
(`hasRemoteProviderRuntime(settings)`,
`frontend/src/renderer/src/components/settings/provider-account-scope.ts`):

- **Có Remote Orca Server đang active** (trường hợp web/`session-auth` như `b15.openledger.vn`):
  token được gửi tới runtime đó và lưu ở đó, với mã hoá runtime hỗ trợ.
- **Không có remote runtime** (desktop, chạy local): token lưu cục bộ trên máy, mã hoá nếu local
  runtime storage hỗ trợ.

Xem thêm mô tả "Account scope" ngay trong card Jira ở Settings — luôn hiển thị runtime nào đang
sở hữu credential hiện tại.
