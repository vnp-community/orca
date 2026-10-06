# TASK-REQ-004-07: Test tích hợp tiếp nhận (tranh chấp, rollback, khoá theo site) hai dialect

**From Solution:** BE-REQ-SOL-004
**Priority:** P0
**Service:** `request-service`
**File:** `internal/adapter/contracttest/create_request_contract.go` (mới), `internal/adapter/postgres/create_request_integration_test.go`, `internal/adapter/mysql/create_request_integration_test.go` (mới)
**Depends on:** TASK-REQ-004-06, TASK-REQ-002-06
**Status:** [ ] TODO

---

## Context

Các tiêu chí mục 4 của CR-REQ-004 chỉ chứng minh được trên DB thật: 12 lệnh đồng thời cùng khoá, không để lại khoá mồ côi, không đốt số Request. Dùng `contracttest` và role không superuser (Postgres) như các task trước. `IssueFetcher` là bản giả có đếm số lần gọi.

## Việc cần làm

1. `RunCreateRequestContract(t, newEnv)` với các kịch bản:
   - `Concurrent12SameKey`: đúng một Request, `number` duy nhất, đúng một dòng `orca.request.request.created` trong outbox, 11 lời gọi còn lại nhận `Created=false` cùng id.
   - `JiraKeyCaseInsensitive`: `eng-1` và `ENG-1` cho một Request.
   - `TwoJiraSitesTwoRequests`: cùng `ENG-1`, hai `site` khác nhau cho hai Request.
   - `EnrichFromIssue`: `title` rỗng thì lấy từ `IssueFetcher` giả; `IssueFetcher` lỗi và `title` có thì vẫn tạo.
   - `GitHubMissingTitleNoExternalCall`.
   - `StatusClassifyingAndTwoEventsInOrder` (so `seq`).
   - `FailureMidwayLeavesNothing`: ép lỗi ở `Create` hoặc ở outbox, không còn dòng `request_idempotency` và `request_counters.next_number` không tăng (kiểm giá trị trước và sau).
   - `CancelledRequestReturnedUnchanged`.
   - `TenantIsolationOfKeys`: tenant B với cùng khoá nguồn tạo Request riêng.
2. Hai file `_test.go` chỉ khởi môi trường và gọi hàm contract.
3. Chạy mỗi kịch bản đồng thời với `-race`.

## Kiểm thử

- `go test -tags=integration -race ./services/request-service/internal/adapter/postgres/... -run CreateRequest -v`
- `go test -tags=integration -race ./services/request-service/internal/adapter/mysql/... -run CreateRequest -v`
- Lặp `-count=3` cho `Concurrent12SameKey`.

## Tiêu chí hoàn thành

- [ ] Mọi kịch bản xanh với cả hai dialect.
- [ ] Không có khoá mồ côi, không lỗ hổng số sau lỗi giữa chừng.
- [ ] Thứ tự hai sự kiện đúng nhờ `seq`.
- [ ] Tenant A và B không đụng khoá.

## Rủi ro và lưu ý

- MySQL có thể báo deadlock khi 12 giao dịch cùng ghi `request_counters`; test cho phép thử lại tối đa 3 lần với lỗi 1213 ở phía gọi (use case không tự thử lại giao dịch ngoài cùng ở feature này). Nếu deadlock xảy ra thường xuyên, báo lại để thêm retry vào `CreateRequest`.
