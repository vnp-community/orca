# SOL-QA-04: Kết nối Jira Server/DC bằng PAT, cấu hình từ `.env`

**Bug:** [BUG-QA-04](../BUG-QA-04-jira-servicehub-task-source.md)

## Bước 1: chọn cách xác thực theo site, không theo việc có email hay không

`issue-tracking-service/internal/adapter/jira/client.go`:
- Thay `authHeaderValue(cred)` bằng `resolveAuth(ctx, cred)` trả `{scheme, apiVersion}`:
  1. Thử `GET /rest/api/2/serverInfo` với **Bearer** `cred.Token`. `200` và `deploymentType ∈ {Server, Data Center}` → `scheme=bearer`, `apiVersion=2`.
  2. Nếu `401/403` và có email: thử Basic `email:token` trên `/rest/api/3/myself` (Cloud) → `scheme=basic`, `apiVersion=3`.
  3. Còn lại → lỗi rõ ràng (`ISSUETRACKING_AUTH_FAILED` kèm lý do: "PAT rejected by Server/Data Center" hoặc "API token rejected by Cloud"); **không** rơi về "3".
- Lưu `scheme` cùng credential (`usecase.Credential.AuthScheme`) để các lần sau không dò lại; đệm theo `(baseURL, hash(token))`, không theo `baseURL` một mình, và đệm thất bại tối đa 30 giây.
- `Whoami`: Server/DC trả `name`/`key` thay vì `accountId` ở `/myself`; lấy `accountId`, nếu trống thì `key`, rồi `name` làm `Viewer.ID`.
- Với Server/DC, `email` chỉ để hiển thị/lọc; JQL "giao cho tôi" dùng `assignee = currentUser()`.
- Thông điệp lỗi của `jira.connect` nói rõ lý do ("401: the site rejected this PAT", "404: Cloud-only API"), thay cho "could not authenticate with the provided credential" hiện nay (QA-JIRA-03).
- Test Go: máy chủ `httptest` mô phỏng Server/DC (chỉ nhận Bearer, `/rest/api/3` trả 404) và Cloud (chỉ nhận Basic); bảng (email có/không) × (site loại nào) → thành công/thất bại đúng.

## Bước 2: hộp thoại kết nối nói đúng sự thật

`jira-connect-dialog.tsx`: nhãn email "Email (optional for Jira Server/Data Center — your PAT is used on its own)"; sau khi kết nối hiện loại site đã nhận ra ("Jira Data Center · Bearer PAT"). Nếu `ok:false` hiện lý do cụ thể từ bước 1.

## Bước 3: cấu hình từ `.env` (không gõ tay)

- `deploy/dev/.env.example`: thêm khối
  ```
  # Jira source for the Tasks tab (optional). Leave empty to connect from the UI.
  JIRA_SITE_URL=
  JIRA_EMAIL=
  JIRA_PAT_TOKEN=
  ```
  (không điền giá trị thật vào file mẫu).
- Script `deploy/dev/scripts/connect-jira-from-env.sh` (bash, macOS/Linux; bản `.ps1` cho Windows nếu có người dùng Windows): đọc `JIRA_*` từ `deploy/dev/.env`, **chấp nhận thêm** tên cũ `JR_URL`, `JR_EMAIL`, `JR_PAT_TOKEN` từ `.env` ở gốc; đăng nhập quản trị như `redeploy-local.sh`, gọi `jira.connect` qua gateway, in "connected as <displayName>" (không in token, không dùng `set -x`, token qua stdin/JSON body, không qua argv).
- Tuỳ chọn khởi động: issue-tracking-service đọc `ISSUETRACKING_BOOTSTRAP_JIRA_*` và tự kết nối cho tenant quản trị khi chưa có kết nối; chỉ bật khi biến có mặt. Vì token nằm trong môi trường container, khuyến nghị đi qua Vault (`deploy/dev/orca-policy.hcl`) thay vì biến môi trường trần; ghi vào README triển khai.
- `jira.status` trả thêm `source: "env"|"ui"` để giao diện hiển thị nguồn kết nối.

## Bước 4: tab Tasks lấy task từ site

- Sau khi kết nối, tab Tasks → Jira liệt kê issue qua `jira.listIssues`/`jira.searchIssues` (đã có) trên `/rest/api/2/search` cho Server/DC; kiểm phân trang `startAt/maxResults`, lọc "assigned to me", mở một issue, xem bình luận.
- Chọn dự án Jira (`jira.listProjects`) và gán `jiraProjectKey` cho dự án Orca.

## Kiểm thử cùng bản sửa

| Tầng | Test |
|---|---|
| Go adapter | bảng Server/DC × Cloud × email (bước 1) |
| Acceptance (giả lập) | QA-JIRA-01: `jira.connect` với email + token tới Jira Server giả (chỉ Bearer) → `ok:true`; QA-JIRA-02: `jira.listIssues` trả issue từ `/rest/api/2/search`; QA-JIRA-03: token sai → lỗi rõ, không đệm sai |
| Acceptance (thật, cờ) | QA-JIRA-LIVE-01: đọc `.env`, `jira.connect` tới `JR_URL`, `jira.listIssues` trả mảng (có thể rỗng), không lộ token trong bất kỳ phản hồi/log nào |
| UI live | QA-UI-JIRA-01: tab Tasks → Jira (sau khi script bước 3 chạy) hiện danh sách; không hiện "Connect your Jira site" |

## Rủi ro
- PAT hết hạn/thu hồi: lỗi phải chỉ ra "PAT rejected", không giữ trạng thái "connected".
- Site tự ký chứng chỉ: không tắt kiểm TLS; cho phép thêm CA qua cấu hình triển khai nếu cần.
- `jira.status` đang đọc theo người dùng; kết nối từ script là của người dùng quản trị, các người dùng khác phải tự kết nối (ghi vào README).
