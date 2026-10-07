# TASK-REQ-031-04: Adapter nguồn dữ liệu Orca (`request_origin`, `history`, `ownership`, `dev_server_profile`)

**From Solution:** BE-REQ-SOL-031 (mục A, CR mục 2.3 dòng `request_origin`, `history`, `ownership`, `dev_server_profile`)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/adapter/sources/{request_origin.go,history.go,ownership.go,dev_server_profile.go}` (mới) và `_test.go`; `.../internal/adapter/grpcclient/{issue_context_client.go,ownership_client.go,dev_server_profile_client.go}` (mới); `.../internal/usecase/ports.go` (thêm cổng đọc); `.../internal/adapter/{postgres,mysql}/request_history_queries.go` (mới)
**Depends on:** TASK-REQ-031-02; BE-REQ-SOL-002 (bảng `requests`, `solutions`); BE-REQ-SOL-013 (`task_run_outcomes`, nếu chưa có thì `history` bỏ phần kết quả thực thi)
**Status:** `[x] DONE`

---

## Context

- RPC thật đã `grep` trong `backend-go/proto`: `issuetracking.v1.GetIssue`, `ListIssueComments` (`proto/orca/issuetracking/v1/*.proto`, dòng 27 và 30); `scmintegration.v1.ListIssueCommentsBySlug`, `GetLinkedPullRequestsForIssue` (dòng 14, 15). Tên message request/response đọc lại từ file `.proto` trước khi dựng client (task này không sao chép chữ ký từ trí nhớ).
- `GetDevServerCapabilities` **chưa có** trong `proto/orca/infrafleet/v1` (đã grep: không kết quả; CR-REQ-033 đề xuất). Adapter đặt sau port `DevServerProfileReader`; khi RPC chưa có thì bản cài `NotAvailableProfileReader` trả `ErrNotConnected` và Builder ghi `missing{not_connected}`.
- `request_origin`: nội dung do bên ngoài kiểm soát (Jira, GitHub), nên `Trust=low` (bọc `<untrusted>`). Request đã có `source_provider`, `source_ref`, `source_site` (SOL-002); không dùng `body` đã lưu vì đã có trong prompt, chỉ lấy **bình luận và liên kết** mới.
- `history`: so khớp bằng cùng `type` và từ khoá tiêu đề (CR nói không dùng embedding). Dùng DB của `request-service`, đa tenant bắt buộc `tenant_id` ở mọi `WHERE`.
- `ownership`: `CODEOWNERS` **chưa tồn tại** ở gốc repo hay `.github/` (CR đã xác nhận); người dùng, team qua `auth-service` và `tenant-service` (đọc `proto/orca/auth/v1`, `proto/orca/tenant/v1` để chọn RPC có thật, ví dụ `ListTeamsForUser` của `tenant-service` xuất hiện trong spec v4).

## Việc cần làm

1. `request_origin.go`: `Key() = "request_origin"`. `Search` chỉ khả dụng khi `Request.source_provider ∈ {jira, github, gitlab, linear}` (manual, mcp, webhook: trả rỗng, `Freshness=unknown`, không lỗi). Gọi qua cổng `IssueContextReader`:

```go
type IssueContextReader interface {
    Comments(ctx context.Context, provider, site, ref string, limit int) ([]IssueComment, error)
    LinkedPullRequests(ctx context.Context, provider, site, ref string) ([]LinkedPR, error)
}
```

   Mỗi comment một `SourceItem` (`Ref = "<provider>:<ref>#comment-<id>"`, `Title` = tác giả đã bỏ email), giới hạn 20 comment mới nhất và 4 KB mỗi comment, `Trust=low`. Cần che email tác giả (qua `Redactor` profile `pii`) vì pack không được chứa danh tính ngoài nhu cầu.
2. `issue_context_client.go`: bản cài trên `issuetrackingv1.IssueTrackingServiceClient` và `scmintegrationv1.ScmIntegrationServiceClient` (tên client đúng như sinh ra; đọc `proto/gen/go`). Truyền metadata tenant theo mẫu `tenant_forwarding.go` của `task-service` (sao chép, không import chéo service).
3. `history.go`: `Key() = "history"`. Port `RequestHistoryReader`:

```go
type RequestHistoryReader interface {
    SimilarRequests(ctx context.Context, t domain.RequestType, terms []string, excludeID string, limit int) ([]HistoryRow, error)
    OutcomesForRequests(ctx context.Context, requestIDs []string) (map[string][]OutcomeRow, error) // task_run_outcomes; rỗng nếu bảng chưa có
}
```

   Mỗi `HistoryRow` thành một `SourceItem`: `Ref="request:<number>"`, nội dung là loại, tiêu đề, trạng thái cuối, Solution đã chọn (tên phương án, **không** toàn văn `options`), kết quả thực thi tóm tắt (`failure_class` nếu có). `Trust=high`. Không đưa `body` của Request cũ (nội dung khách).
4. Truy vấn `request_history_queries.go` hai dialect: `SELECT ... FROM requests WHERE tenant_id=? AND type=? AND id<>? AND status IN ('completed','cancelled','request_backlog') AND (LOWER(title) LIKE ? OR ...) ORDER BY updated_at DESC LIMIT ?` (Postgres dùng `ILIKE` và tham số `$n`; mọi `LIKE` thoát `%`, `_`, `\` trong từ khoá, tránh người dùng chèn ký tự đại diện); tối đa 5 từ khoá, 10 dòng. Chỉ mục `(tenant_id, type, status, updated_at DESC)`: kiểm có sẵn từ SOL-002 (`(tenant_id, status, updated_at DESC)`); thiếu `type` thì thêm vào migration của task 01 hoặc ghi vào rủi ro (không tạo migration riêng ở task này).
5. `ownership.go`: `Key() = "ownership"`. Hai phần: (a) `CODEOWNERS` (đọc `CODEOWNERS`, `.github/CODEOWNERS`, `docs/CODEOWNERS` qua `RepoReader`; không có thì bỏ qua, không lỗi); (b) danh sách người dùng và team của dự án qua `OwnershipReader` (`auth-service`/`tenant-service`/`project-service`: chỉ id, tên hiển thị, vai trò; không email). `Trust=high`.
6. `dev_server_profile.go`: `Key() = "dev_server_profile"`. Port:

```go
type DevServerProfileReader interface {
    Profile(ctx context.Context, projectID string) (DevServerProfile, error)
}
type DevServerProfile struct {
    Tools map[string]string   // tên công cụ -> phiên bản
    EnvPresent []string       // CHỈ tên biến (ví dụ ANTHROPIC_API_KEY), không giá trị
    ClaudeLoggedIn bool
    Degraded bool
    CollectedAt time.Time
}
```

   Một `SourceItem` duy nhất (`Ref="dev_server:<id>"`) với nội dung là bảng công cụ và tên biến; `Degraded=true` thì `Freshness=stale`. Tuyệt đối không đưa giá trị biến môi trường (test quét).
7. Đăng ký bốn adapter vào `sources.NewRegistry` của task 03.
8. Mọi adapter nhận `tenantID` từ `tenant.RequireTenantID(ctx)`; lỗi downstream (gRPC `Unavailable`, `DeadlineExceeded`) được ánh xạ sang lỗi miền `ErrSourceTimeout`, `ErrNotConnected` cho Builder; không trả lỗi gRPC thô.

## Kiểm thử

- `request_origin_test.go`: provider `manual` trả rỗng không lỗi; 25 comment thì chỉ 20 mới nhất; email tác giả bị che; nội dung `Trust=low`.
- `history_test.go` (adapter) với `FakeRequestHistoryReader`: `Request` đang xét bị loại; tiêu đề chứa `%` không làm hỏng truy vấn (test ở repository bên dưới).
- `request_history_queries_integration_test.go` (hai dialect): `TestSimilarRequests_TenantIsolation` (tenant B không thấy Request tenant A), `TestSimilarRequests_EscapesWildcards` (từ khoá `100%` chỉ khớp tiêu đề có chữ `100%`), `TestSimilarRequests_OnlyTerminalStatuses`.
- `ownership_test.go`: không có CODEOWNERS: chỉ phần người dùng; tên hiển thị không chứa `@` nhờ che.
- `dev_server_profile_test.go`: `Degraded=true` cho `Freshness=stale`; `TestNoEnvValuesLeak`: profile có `EnvPresent=["ANTHROPIC_API_KEY"]` và fake trả thêm giá trị `sk-ant-xxxx` trong `Tools` (lỗi cố ý của fake) thì pipeline che (task 02 + 035) làm biến mất.
- `NotAvailableProfileReader` trả `ErrNotConnected`.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/adapter/sources/... && go test -tags=integration ./services/request-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [x] `request_origin` không lấy thân Request, chỉ comment, liên kết; `Trust=low`.
- [x] `history` không đưa `body` của Request cũ vào pack.
- [x] `dev_server_profile` không bao giờ chứa giá trị biến môi trường (test quét).
- [x] Truy vấn lịch sử cách ly tenant và thoát ký tự đại diện, cả hai dialect.
- [x] Khi RPC `GetDevServerCapabilities` chưa có, Builder vẫn dựng pack với `missing{dev_server_profile, not_connected}`.

## Rủi ro và lưu ý

- `LIKE` theo từ khoá cho gợi ý tiền lệ nghèo nàn (không embedding); chấp nhận ở v1.
- Chưa kiểm chứng phản hồi thật của `issue-tracking-service` khi Jira chưa bật (server dev có 0 dòng `task_sources` theo README v6 mục 7).
- Bình luận Jira có thể chứa bí mật dán vào: bắt buộc qua `Redactor` trước khi vào pack (do Builder thực hiện, task 05), test ở đó.
- `CODEOWNERS` chưa tồn tại: việc dữ liệu (tạo tệp) có chủ sở hữu ngoài mã của task này.
- Chỉ mục thiếu `type` có thể làm `SimilarRequests` quét nhiều với tenant lớn; theo dõi sau.
