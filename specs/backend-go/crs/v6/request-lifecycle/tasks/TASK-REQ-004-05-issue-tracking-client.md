# TASK-REQ-004-05: Client `issue-tracking-service` (`IssueFetcher`) chuyển tenant và user

**From Solution:** BE-REQ-SOL-004
**Priority:** P1
**Service:** `request-service`
**File:** `internal/adapter/grpcclient/issue_tracking_client.go`, `internal/adapter/grpcclient/identity_forwarding.go`, `internal/adapter/grpcclient/issue_tracking_client_test.go` (mới); `internal/config/config.go`, `cmd/server/main.go` (sửa)
**Depends on:** TASK-REQ-004-04
**Status:** [ ] TODO

---

## Context

Tiền lệ: `git-gateway-service/internal/adapter/grpcclient/issuetracking_client.go` gọi `GetIssue` với `Provider`, `IssueId` và `withTenantMetadata` (chỉ tenant). Jira credential lưu theo `(tenant, user)` (CR-TG-008), nên client này phải chuyển cả `x-orca-user-id` (`grpcmw.MetadataUserID`); mẫu chuyển cả hai nằm ở `task-service/internal/adapter/grpcclient/project_execution_resolver.go` (`metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)`). `GetIssueRequest` có `workspace_id=3`. `Issue` trả `title`, `description_markdown`, `url`, `key`, `labels`, `issue_type`, `priority` (proto `issuetracking.proto` dòng 134 đến 156). Địa chỉ service: cần thêm `ISSUE_TRACKING_SERVICE_ADDR` vào config (TASK-REQ-001-01 chưa có).

## Việc cần làm

1. `config.go`: thêm `IssueTrackingServiceAddr` (`ISSUE_TRACKING_SERVICE_ADDR`).
2. `identity_forwarding.go`: `withIdentityMetadata(ctx) (context.Context, error)`: `tenant.RequireTenantID`, `tenant.UserID` (thiếu user thì lỗi `REQUEST_REPORTER_REQUIRED`), thêm hai khoá metadata. Không import chéo service.
3. `issue_tracking_client.go`: `IssueTrackingClient` cài `usecase.IssueFetcher`: ánh xạ provider `jira`/`linear` sang `IssueProvider_ISSUE_PROVIDER_JIRA`/`_LINEAR` (provider khác thì lỗi lập trình `unsupported provider`); gọi `GetIssue(ctx, &GetIssueRequest{Provider, IssueId: ref, WorkspaceId: site})`; `codes.NotFound` ánh xạ `usecase.ErrIssueNotFound`; lỗi khác bọc nguyên; kết quả sang `IssueSnapshot` (`Hints.IssueType` từ `issue_type.name` nếu có, `Priority` từ `priority.name`, `Labels`). Đặt timeout 10 giây trên ctx.
4. `main.go`: dial khi `IssueTrackingServiceAddr != ""`; rỗng thì dùng `IssueFetcher` trả `ErrIssueNotFound` (log cảnh báo) để service vẫn chạy được.

## Kiểm thử

- `TestIssueTrackingClient_ForwardsTenantAndUser` (dùng gRPC server giả `bufconn`, kiểm metadata nhận được).
- `TestIssueTrackingClient_NotFoundMapsToSentinel`, `TestIssueTrackingClient_MapsHints`, `TestIssueTrackingClient_UnsupportedProvider`.
- `TestIdentityForwarding_MissingUserFails`.
- Chưa kiểm chứng với Jira thật (README v6 mục 7); ghi vào PR.
- Lệnh: `go test ./services/request-service/internal/adapter/grpcclient/...`.

## Tiêu chí hoàn thành

- [ ] Metadata tenant và user tới được server giả.
- [ ] `NotFound` thành `REQUEST_SOURCE_NOT_FOUND` qua use case.
- [ ] Không có provider ngoài `jira`, `linear` được gọi.
- [ ] Service khởi động được khi không cấu hình địa chỉ.

## Rủi ro và lưu ý

- `workspace_id = site` chưa kiểm chứng; nếu `issue-tracking-service` chọn workspace theo `ConnectionStatus.active_workspace_id` khi trống thì truyền `site` rỗng khi `site` là URL không khớp workspace id (kiểm khi có Jira thật).
- Nếu cần gọi lại `GetIssue` cho nhiều Request trong một webhook bùng nổ, cân nhắc giới hạn tốc độ: ngoài phạm vi.
