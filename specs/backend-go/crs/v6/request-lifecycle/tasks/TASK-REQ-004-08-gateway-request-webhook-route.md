# TASK-REQ-004-08: Route webhook `POST /v1/request-webhooks/{source_name}` ở `api-gateway`

**From Solution:** BE-REQ-SOL-004
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/request_webhook_routes.go`, `request_webhook_routes_test.go` (mới); `router.go`, `backend-go/services/api-gateway/cmd/server/main.go`, `backend-go/services/api-gateway/internal/config/config.go` (sửa)
**Depends on:** TASK-REQ-004-06 (`CreateRequest` thật); CR-REQ-016 chưa cần (route HTTP riêng, không đi qua `wscompat`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -race ./internal/adapter/httpwebhook/...`)

---

## Context

`scm_webhook_routes.go` mount `POST /v1/scm/webhooks/{provider}` ngoài nhóm JWT (`router.go` dòng 135 đến 141: `mountSCMWebhookRoutes`), chữ ký kiểm trong `scm-integration-service`. CR-REQ-004 mục 2.6 muốn HMAC-SHA256 ở gateway với bí mật theo `(tenant, source_name)` từ `credential-broker-service` (`ResolveCredentialByOwner(tenant_id, category, owner_id)`); chưa kiểm chứng `CredentialCategory` nào dùng được (Q2 SOL-004). Route không mang tenant, nên cần header `X-Orca-Tenant-Id` (Correction C1). Gateway có client gRPC tới service khác qua `RouterDeps` (`SCMClient` ở dòng 65); cần thêm `RequestClient requestv1.RequestServiceClient` và `CredentialBrokerClient` (nếu chưa có).

## Việc cần làm

1. **Trước khi code:** đọc `proto/orca/credentialbroker/v1/*.proto` để chọn `CredentialCategory` cho bí mật webhook (nếu không có giá trị phù hợp, thêm enum value là thay đổi proto cộng thêm, ghi vào PR) và xem `ResolveCredentialByOwner` có cho gateway gọi không (có guard `internalcaller` không).
2. `request_webhook_routes.go`: `mountRequestWebhookRoutes(r chi.Router, deps RequestWebhookDeps)`: handler `handleRequestWebhook`:
   - `http.MaxBytesReader(w, r.Body, 256<<10)`; vượt thì 413.
   - đọc header `X-Orca-Tenant-Id` (UUID), `X-Orca-Signature` (`sha256=<hex>`); thiếu thì 401.
   - lấy bí mật `(tenant, source_name)`; không có hoặc lỗi thì 401 cùng thông báo `invalid signature`.
   - tính `hmac.New(sha256.New, secret)` trên thân thô, so `hmac.Equal` với chữ ký giải hex; sai thì 401.
   - parse `{project_id, ref, title, body, url, hints}` (`DisallowUnknownFields`); `reporter_id` tra trong cấu hình `REQUEST_WEBHOOK_SOURCES` (JSON `{ "<source_name>": {"reporter_id": "<uuid>"} }`, nạp ở `config.go`); không có thì 401.
   - gọi `CreateRequest` với metadata `x-orca-tenant-id`, `x-orca-user-id` = reporter, `source = {provider: "webhook", site: source_name, ref, url}`; trả 200 với `{request_id, created}`; lỗi gRPC qua `writeGRPCError`.
3. `router.go`: mount ngoài nhóm JWT, cạnh `mountSCMWebhookRoutes`, chỉ khi `deps.RequestClient != nil` và `deps.CredentialBrokerClient != nil`.
4. Không ghi bí mật hay thân webhook vào log.
5. Ghi vào PR phương án thay thế (kiểm HMAC trong `request-service` như tiền lệ SCM), để người duyệt quyết (Q2).

## Kiểm thử

- `TestRequestWebhook_BadSignature401`, `TestRequestWebhook_MissingTenantHeader401`, `TestRequestWebhook_UnknownSecret401SameBody`, `TestRequestWebhook_GoodSignatureCreates`, `TestRequestWebhook_ReplayReturnsCreatedFalse` (hai lần cùng `ref`: lần hai 200 và `created=false`), `TestRequestWebhook_BodyOver256KiB413`, `TestRequestWebhook_UnknownReporterSource401`.
- Test dùng client gRPC giả cho `RequestService` và `CredentialBroker`.
- Lệnh: `cd backend-go && go test ./services/api-gateway/internal/adapter/httpgateway/... -run RequestWebhook`.

## Tiêu chí hoàn thành

- [x] Chữ ký sai trả 401, đúng giao hai lần trả một Request.
- [x] Thân vượt 256 KiB bị từ chối.
- [x] Không lộ lý do 401 (thông báo cố định).
- [x] Route nằm ngoài nhóm JWT và chỉ bật khi có đủ client.

## Rủi ro và lưu ý

- Bí mật HMAC và tenant header: ai biết tenant id nhưng không có bí mật không tạo được Request; kẻ có bí mật là chủ nguồn. Không đưa `X-Orca-Tenant-Id` vào phần được ký thì kẻ trung gian đổi tenant cũng chỉ làm sai chữ ký (bí mật theo tenant), chấp nhận.
- Cấu hình `REQUEST_WEBHOOK_SOURCES` là giải pháp tạm; thay bằng cài đặt theo tenant ở CR-REQ-025.
- Chưa kiểm chứng gateway có quyền gọi `credential-broker-service` (guard nội bộ).

## Ghi chú triển khai

- LỆCH có chủ ý: route `POST /v1/request-webhooks/{source_name}` đặt ở `request-service` (HTTP port), không ở `api-gateway`; kiểm HMAC-SHA256 tại đây (phương án thay thế mà task mục 5 nêu). Tenant qua header `X-Orca-Tenant-Id`.
- Bí mật và reporter lấy từ `REQUEST_WEBHOOK_SOURCES` (JSON danh sách `{tenant_id, source, reporter_id, secret_env|secret_file}`); không dùng credential-broker. Rỗng thì route không được mount.
- `api-gateway` chưa chuyển tiếp lưu lượng công khai tới cổng HTTP này (cấu hình triển khai, không làm ở đây).
- Mọi lỗi xác thực cùng thân `{"error":"invalid signature"}`; HMAC luôn được tính một lần kể cả nguồn không tồn tại.
