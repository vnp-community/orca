# TASK-REQ-002-03: Cổng repository, `ListFilter`, phân trang keyset

**From Solution:** BE-REQ-SOL-002
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/ports.go` (bổ sung), `internal/usecase/page_token.go`, `internal/usecase/page_token_test.go` (mới)
**Depends on:** TASK-REQ-002-02, TASK-REQ-001-04 (`TxRunner`, `OutboxWriter`)
**Status:** [ ] TODO

---

## Context

`ports.go` hiện có `TxRunner`, `OutboxWriter` (TASK-REQ-001-04). `task-service/internal/usecase/ports.go` là mẫu về cách khai báo cổng (dòng 510 `OutboxWriter`) và `task_source_ports.go` cho cổng nguồn. Cổng đặt trong `usecase`, adapter cài đặt; domain không biết repository.

## Việc cần làm

1. Khai báo trong `ports.go` các interface của SOL-002 mục C: `RequestRepository` (`Create`, `Get`, `GetByNumber`, `List`, `Update(ctx, r, expectedVersion)`, `NextNumber`), `RequestTypeHistoryRepository`, `SolutionRepository`, `RequestLinkRepository`, `RequestIdempotencyRepository` (`Claim`, `Find`).
2. `ListFilter{ProjectID, ReporterID, SourceProvider, SourceSite, SourceRef string; Statuses []domain.RequestStatus; Types []domain.RequestType; PageSize int; PageToken string}` và `ListResult{Requests []domain.Request; NextPageToken string}`; hàm `(f ListFilter) Normalize() (ListFilter, error)`: `PageSize` 0 thành 50, trên 200 thành 200, âm là `InvalidArgument`.
3. `page_token.go`: `EncodePageToken(createdAt time.Time, id string) string` (base64 URL không padding của JSON `{"c": <unix micro>, "i": "<id>"}`), `DecodePageToken(string) (time.Time, string, error)`; token rỗng nghĩa là trang đầu; token sai trả `REQUEST_INVALID_PAGE_TOKEN` (`KindInvalidArgument`, constructor thêm vào `domain/request_errors.go`).
4. Ghi chú trên cổng: mọi phương thức lấy tenant từ ctx qua `tenant.RequireTenantID`, không nhận tenant làm tham số (tránh gọi nhầm tenant); `Update` trả bản ghi sau khi tăng `Version` và `UpdatedAt` đọc lại từ DB.
5. `Claim` trả `(existingRequestID string, claimed bool, err error)`: `claimed=true` thì `existingRequestID` rỗng; `claimed=false` thì là id của bên thắng. `Find` không tạo gì.
6. Tạo `internal/usecase/fakes_test.go`? **Không**: không tạo file `fakes`; fake repo cho unit test của use case sau này đặt cạnh test cần nó với tên cụ thể (`request_repository_fake_test.go`).

## Kiểm thử

- `TestPageToken_RoundTrip`, `TestPageToken_InvalidBase64`, `TestPageToken_InvalidJSON`, `TestPageToken_EmptyMeansFirstPage`.
- `TestListFilter_Normalize` (mặc định 50, trần 200, âm lỗi).
- Biên dịch: `go vet ./services/request-service/...`; hai adapter ở TASK-REQ-002-04, 002-05 phải có `var _ usecase.RequestRepository = (*Repository)(nil)` (kiểm khi các task đó xong).
- Lệnh: `go test ./services/request-service/internal/usecase/...`.

## Tiêu chí hoàn thành

- [ ] Cổng khớp SOL-002 mục C, chữ ký đúng tên.
- [ ] `ListFilter.Normalize` và token có test.
- [ ] Không có `fakes.go` hay `mocks.go` chung chung.
- [ ] `go vet` xanh.

## Rủi ro và lưu ý

- `ListFilter` đã có bốn trường nguồn để CR-REQ-004 không đổi chữ ký; adapter phải lọc theo chúng ngay (TASK-REQ-002-04, 002-05).
- Token mang `created_at` ở micro giây: MySQL `TIMESTAMP(6)` giữ đủ, Postgres `TIMESTAMPTZ` cũng; đừng cắt xuống giây.
