# TASK-REQ-016-08: Kênh bổ sung `request.links`, `request.flow`, `request.checks` (tuỳ chọn)

**From Solution:** BE-REQ-SOL-016
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_extras.go` (mới), `.../channels_request_extras_test.go` (mới), `.../excluded_channels.yaml`
**Depends on:** TASK-REQ-016-03; CONTRACT mục 9 Q1 đã chốt; RPC `ListRequestLinks` (CR-REQ-006), `GetRequestFlow` (CR-REQ-003), `ListRequestChecks` (CR-REQ-014)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./... -count=1)

---

## Context

- README v6 mục 8 dòng 12 liệt kê RPC này nhưng CR-REQ-016 không có kênh. Frontend CR-REQ-019 hỏi `request.get` có `links` không (Q1 của CR đó) và CR-REQ-003 Q2 đề nghị `GetRequestFlow`.
- Hình dạng `FlowDefinition` do CR-REQ-003 chốt; `ListRequestChecksResponse` do CR-REQ-014.
- Task này **chỉ làm khi** CONTRACT mục 2.5 được duyệt; nếu không, đóng task.

## Việc cần làm

1. `request.links` `{id}` gọi `ListRequestLinks`, trả `{parents, children}` (`RequestLinkView`). 8s.
2. `request.flow` `{type, size?}` gọi `GetRequestFlow`, trả cấu trúc đã chốt; khoá camelCase.
3. `request.checks` `{id}` gọi `ListRequestChecks`; trả `{checks: [...]}`.
4. `excluded_channels.yaml`: ba dòng (BE-REQ-SOL-017 quyết có tool cho `request.checks` đọc hay không).
5. Cập nhật CONTRACT mục 2.5 thành mục chính thức (chuyển từ "đề xuất" sang bảng 2.1) và báo frontend.

## Kiểm thử

- Fake client: ánh xạ, `[]` khi rỗng, lỗi qua `requestChannelError`. `go test ./internal/adapter/wscompat/... -run RequestExtras`.

## Tiêu chí hoàn thành

- [x] Ba kênh có test; parity xanh; CONTRACT cập nhật.

## Rủi ro và lưu ý

- Đừng thêm `RecordRequestCheck` thành kênh WS ở task này (CR-REQ-014 Q1).

## Ghi chú triển khai (2026-10-08)

Đã chuyển CONTRACT mục 2.5 thành kênh thật.
