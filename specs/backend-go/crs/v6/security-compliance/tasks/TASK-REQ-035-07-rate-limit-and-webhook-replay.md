# TASK-REQ-035-07: `RateLimitInterceptor`, trần đồng thời và chống phát lại webhook (thời gian + nonce)

**From Solution:** BE-REQ-SOL-035 (mục F rate limit, webhook)
**Priority:** P1
**Service:** `request-service`, `api-gateway`
**File:** `backend-go/services/request-service/internal/domain/rate_limit_policy.go` (mới), `.../internal/usecase/rate_limit_policy.go` (mới), `.../internal/adapter/grpc/rate_limit.go` (mới), `.../internal/adapter/{postgres,mysql}/{concurrency_counts.go,webhook_nonces.go}` (mới), `.../internal/adapter/grpc/server_webhook_nonce.go` (mới: RPC nội bộ `RecordWebhookNonce`), `backend-go/proto/orca/request/v1/request.proto` (sửa), `backend-go/services/api-gateway/internal/adapter/httpgateway/request_webhook_routes.go` (sửa; do TASK-REQ-004-08 tạo), và `_test.go` tương ứng
**Depends on:** TASK-REQ-035-04 (`Catalog.RateClass`), TASK-REQ-035-06 (bảng `request_webhook_nonces`), TASK-REQ-004-08 (route webhook), BE-REQ-SOL-007 (`analysis_runs`), BE-REQ-SOL-008 (trần 2 run mỗi dự án)
**Status:** `[ ] TODO`

---

## Context

- Gateway có bộ giới hạn theo tenant trong bộ nhớ, mỗi bản sao một bộ (`api-gateway/internal/usecase/rate_limit.go`), không biết lớp lời gọi (đọc, ghi, AI). Giới hạn trong bộ nhớ của `request-service` cũng mỗi bản sao một bộ; giới hạn thật cho AI nằm ở ngân sách (BE-REQ-SOL-034).
- `golang.org/x/time/rate` đã được gateway dùng (CR 2.7): kiểm `go.mod` của `common`/gateway trước khi thêm vào `request-service` (cùng phiên bản).
- Mặc định đề xuất, chưa đo (CR 2.7): `read` 20 req/s burst 40; `write` 5 req/s burst 10; `ai` 2 req/s burst 4; `webhook` 10 req/s burst 20 (MCP theo token). Trần đồng thời đếm bằng DB: `analysis_runs running` 10 mỗi tenant và 2 mỗi dự án (CR-REQ-008), Request chưa kết thúc 2000 mỗi tenant. Giới hạn kích thước: `title` 500, `body` 100 000 ký tự, `comment` Approval 4000, thân webhook 256 KiB, JSON `options` 64 KB.
- Webhook (TASK-REQ-004-08): `X-Orca-Signature: sha256=<hex>` = HMAC-SHA256 theo `(tenant, source)` trên thân; route đặt ngoài nhóm JWT; thân tối đa 256 KiB; mọi lỗi xác thực trả 401 cùng thông báo. CR 035 thêm `X-Orca-Timestamp` và nonce.
- Chữ ký tự nó là nonce tự nhiên: cùng thân và cùng timestamp ⇒ cùng chữ ký; dùng `sha256(signature_hex)` làm `nonce_hash` (không cần header nonce thứ hai).

## Việc cần làm

1. `domain/rate_limit_policy.go`: `type RateClass string` (`read|write|ai|webhook|none`); `type Limit struct{ PerSecond float64; Burst int }`; `DefaultLimits map[RateClass]Limit` theo bảng trên; `type ConcurrencyCaps struct{ RunningPerTenant, RunningPerProject, OpenRequestsPerTenant int }` mặc định `10, 2, 2000`; `ErrRateLimited{Class RateClass; RetryAfter time.Duration}` ⇒ `REQUEST_RATE_LIMITED`.
2. `usecase/rate_limit_policy.go`: `RateLimiter` giữ `map[key]*rate.Limiter` với khoá `tenant + "\x00" + class` (mẫu `scm_rate_limit.go` của gateway: `maxRateBuckets = 10000`, `evict` theo thời gian không dùng; đọc file đó trước, không sao chép máy móc); `Allow(tenant string, c RateClass) (ok bool, retryAfter time.Duration)` dùng `limiter.Reserve()` để tính `retryAfter`; ghi đè hạn mức theo tenant từ `tenant_settings` là việc sau (ghi vào rủi ro). `CheckConcurrency(ctx, kind, tenantID, projectID)` gọi `ConcurrencyCounter` (cổng) rồi so trần.
3. `adapter/{postgres,mysql}/concurrency_counts.go`: `CountRunning(ctx, projectID *string) (int, error)` (`SELECT COUNT(*) FROM analysis_runs WHERE tenant_id=? AND status='running' [AND project_id=?]`: `analysis_runs` không có `project_id` nếu SOL-007 không thêm; kiểm schema lúc làm, nếu thiếu thì `JOIN requests`), `CountOpenRequests(ctx) (int, error)` (`status NOT IN ('completed','cancelled')`). Chỉ mục có sẵn (`(tenant_id, status, updated_at)` ở SOL-002) đủ; kiểm kế hoạch truy vấn trên dữ liệu thử (chưa đo).
4. `adapter/grpc/rate_limit.go`: `RateLimitInterceptor(l *RateLimiter, c Catalog, caps ConcurrencyCaps, counter ConcurrencyCounter) grpc.UnaryServerInterceptor`: `RateClass` từ `Catalog[method].RateClass`; `Allow` sai ⇒ `status.New(codes.ResourceExhausted, "REQUEST_RATE_LIMITED")` với `errdetails.RetryInfo{RetryDelay}`; với `CreateRequest` kiểm `CheckConcurrency(open requests)`, với `GenerateSolution`/`GeneratePlan`/`ClassifyRequest`/`StartPhase` kiểm trần `running` theo tenant và (cho phân tích chỉ đọc) theo dự án; vượt ⇒ `REQUEST_RATE_LIMITED` (cùng mã, `detail="concurrency"`). Lớp `webhook` áp cho `CreateRequest` khi `source_provider ∈ {webhook, mcp}` (đọc từ thân); MCP theo token: khoá thêm `actor_type=agent` và `user_id`.
5. Kích thước: `protovalidate`/kiểm thủ công ở handler `CreateRequest`: `title` > 500 hoặc `body` > 100 000 ký tự (đếm rune) ⇒ `InvalidArgument` `REQUEST_PAYLOAD_TOO_LARGE`; thân webhook > 256 KiB ⇒ 413 ở gateway (đã có ở 004-08); `comment` > 4000 và `options` > 64 KB kiểm ở các use case tương ứng (ghi vào PR của CR 009, 007 nếu chưa có). Thêm ràng buộc `protovalidate` cho độ dài (CR-REQ-001 cần nêu; ghi vào PR).
6. Nonce webhook: proto nội bộ `rpc RecordWebhookNonce(RecordWebhookNonceRequest{tenant_id, source, nonce_hash}) returns (RecordWebhookNonceResponse{bool accepted})` (nhóm `internal`, token service). Hiện thực `INSERT INTO request_webhook_nonces (tenant_id, source, nonce_hash, expires_at) VALUES (..., now()+interval '10 minutes') ON CONFLICT DO NOTHING` và `accepted = RowsAffected()==1` (MySQL `INSERT IGNORE`); dọn hàng hết hạn bằng `DELETE ... WHERE expires_at < now() LIMIT 500` chạy cùng bộ quét hết hạn (SOL-010) hoặc vòng riêng 1 phút.
7. Gateway `request_webhook_routes.go`: (a) đọc `X-Orca-Timestamp` (Unix giây hoặc RFC 3339; một định dạng, chốt Unix giây): thiếu hoặc không phân tích được ⇒ 401 cùng thông báo; lệch `|now - ts| > 5 phút` ⇒ 401; (b) chuỗi ký là `<timestamp> + "." + <thân thô>` (đổi so với 004-08 chỉ ký thân: ghi rõ trong PR và trong mô tả cấu hình nguồn webhook, đây là thay đổi **phá vỡ** với người gửi hiện có; cho phép `REQUEST_WEBHOOK_REQUIRE_TIMESTAMP=false` trong giai đoạn chuyển tiếp, mặc định `true`); (c) sau khi xác thực HMAC đúng gọi `RecordWebhookNonce(tenant, source, sha256hex(signature))`; `accepted=false` ⇒ trả **200** `{created:false, duplicate:true}` (nhất quán với "giao lặp trả 200" của 004-08) và không tạo Request; lỗi RPC ⇒ 503 (không bỏ qua kiểm tra).
8. Metric `request_rate_limited_total{class}` (không nhãn tenant), log chỉ lớp.

## Kiểm thử

- `rate_limit_policy_test.go` (đồng hồ giả): `TestAllow_BurstThenRefill`; `TestRetryAfterPositiveWhenLimited`; `TestPerTenantIsolation` (tenant A cạn không ảnh hưởng B); `TestEvictOldBuckets`.
- `rate_limit_test.go` (interceptor): vượt lớp `ai` ⇒ `ResourceExhausted` có `RetryInfo`; `GetRequest` (lớp `read`) không bị lớp `ai` chi phối; trần `OpenRequestsPerTenant=2000` bằng bộ đếm giả; `RPC` thiếu trong `Catalog` ⇒ coi `write` (và log cảnh báo; thực ra `ValidateCatalog` đã chặn ở khởi động).
- `concurrency_counts_integration_test.go` (hai dialect): đếm đúng `running`, đếm theo dự án, cách ly tenant.
- `request_webhook_routes_test.go`: timestamp lệch 6 phút ⇒ 401; lệch 4 phút ⇒ qua; nonce lặp ⇒ 200 `duplicate`, `CreateRequest` gọi đúng một lần; `RecordWebhookNonce` lỗi ⇒ 503; chữ ký đúng nhưng timestamp bị sửa (ký không còn khớp) ⇒ 401; chế độ chuyển tiếp `REQUIRE_TIMESTAMP=false` ký thân cũ ⇒ qua.
- `server_webhook_nonce_test.go`: lần hai cùng `nonce_hash` ⇒ `accepted=false`; sau 10 phút (đồng hồ DB giả hoặc cập nhật `expires_at`) dọn rồi chấp nhận lại.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpc/... ./services/api-gateway/internal/adapter/httpgateway/... && go test -tags=integration ./services/request-service/internal/adapter/...`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Vượt giới hạn lớp `ai` trả `REQUEST_RATE_LIMITED` kèm `retry-after`.
- [ ] Thân webhook lệch thời gian 6 phút hoặc nonce lặp bị từ chối/không tạo trùng.
- [ ] Trần đồng thời theo tenant và dự án được thi hành bằng DB (đúng cả khi nhiều bản sao).
- [ ] Kích thước `title`, `body` vượt trần bị `InvalidArgument`.
- [ ] Không log thân webhook hay chữ ký.

## Ví dụ tham khảo

Tính chữ ký webhook phía người gửi (giai đoạn `REQUEST_WEBHOOK_REQUIRE_TIMESTAMP=true`):

```go
ts := strconv.FormatInt(time.Now().Unix(), 10)
mac := hmac.New(sha256.New, secret)
mac.Write([]byte(ts + "." + string(body)))
sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
// headers: X-Orca-Timestamp: ts, X-Orca-Signature: sig, X-Orca-Tenant-Id: <tenant>
nonceHash := sha256hex(strings.TrimPrefix(sig, "sha256=")) // khoá chống phát lại ở request_webhook_nonces
```

## Thứ tự làm gợi ý

1. `rate_limit_policy` (domain, usecase) với đồng hồ giả, chưa gắn interceptor.
2. Interceptor và trần đồng thời (cần `Catalog` của task 04).
3. RPC `RecordWebhookNonce` và bảng nonce.
4. Cuối cùng sửa route webhook ở gateway (thay đổi phá vỡ chuỗi ký, bật cờ chuyển tiếp trước).

## Rủi ro và lưu ý

- Bộ xô token trong bộ nhớ mỗi bản sao một bộ: giới hạn thật bằng N lần với N bản sao; trần đồng thời bằng DB mới chính xác.
- Đổi chuỗi ký webhook thành `timestamp.body` phá vỡ người gửi đang dùng chữ ký chỉ trên thân: cần giai đoạn chuyển tiếp và thông báo (cờ `REQUEST_WEBHOOK_REQUIRE_TIMESTAMP`).
- Số hạn mức là đề xuất chưa đo; thiết lập sai có thể chặn người dùng thật (`write` 5 req/s có thể quá chặt với tích hợp hàng loạt).
- `CountRunning` theo dự án phụ thuộc cột dự án của `analysis_runs`; kiểm schema SOL-007 lúc làm.
