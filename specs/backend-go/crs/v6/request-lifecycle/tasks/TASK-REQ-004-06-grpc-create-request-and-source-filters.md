# TASK-REQ-004-06: Handler gRPC `CreateRequest` và bộ lọc nguồn trong repository

**From Solution:** BE-REQ-SOL-004
**Priority:** P0
**Service:** `request-service`
**File:** `internal/adapter/grpc/server.go`, `internal/adapter/grpc/request_mapper.go` (sửa); `internal/adapter/postgres/request_repository.go`, `internal/adapter/mysql/request_repository.go` (sửa `List`); `cmd/server/main.go` (sửa)
**Depends on:** TASK-REQ-004-03, TASK-REQ-004-04, TASK-REQ-004-05
**Status:** [ ] TODO

---

## Context

`CreateRequest` use case (TASK-REQ-004-04), proto (TASK-REQ-004-03). `ListFilter` có sẵn `SourceProvider/SourceSite/SourceRef` từ TASK-REQ-002-03 nhưng adapter chưa lọc theo chúng (task đó chỉ nêu trong ghi chú). Metadata danh tính do `grpcmw.TenantExtractionInterceptor` đưa vào ctx. Quyền ghi theo project do gateway kiểm; `request-service` tự kiểm tenant (README v6 mục 8 điểm 13, mục 6).

## Việc cần làm

1. `server.go`: `CreateRequest(ctx, req)`: ánh xạ `RequestSource` sang `domain.SourceRef`, `SourceHints`, gọi `usecase.CreateRequest.Execute`, trả `CreateRequestResponse{Request, Created}`; lỗi qua `apperrors.ToGRPCStatus`.
2. `request_mapper.go`: thêm `sourceFromProto`, `hintsFromProto`.
3. Hai repository: `List` thêm điều kiện `source_provider = ?`, `source_site = ?`, `source_ref = ?` khi đặt (đã chuẩn hoá trước khi truyền: use case `ListRequests` gọi `NormalizeSourceRef` khi cả ba có).
4. `main.go`: lắp `CreateRequest` (cần `IssueFetcher`, `TransitionRequest`, repo) và truyền vào `NewServer`.
5. README service: `CreateRequest` thật.

## Kiểm thử

- `TestServer_CreateRequest_Manual`, `TestServer_CreateRequest_IdempotentByClientRequestID`, `TestServer_CreateRequest_MissingProject` (`InvalidArgument`, `REQUEST_PROJECT_REQUIRED`), `TestServer_CreateRequest_NoUserMetadata` (`REQUEST_REPORTER_REQUIRED`).
- `TestListRequests_FilterBySource` hai dialect (hỏi "issue này đã có Request chưa": `source_provider=jira, source_site=..., source_ref=eng-1` tìm được bản `ENG-1`).
- Lệnh: `go test ./services/request-service/internal/adapter/grpc/...` và bản `-tags=integration` cho repo.

## Tiêu chí hoàn thành

- [ ] `CreateRequest` thật, trả `created` đúng.
- [ ] Lọc nguồn đúng, đã chuẩn hoá.
- [ ] `go vet` xanh; README cập nhật.

## Rủi ro và lưu ý

- Mọi RPC của `request-service` tự kiểm quyền (README v6 mục 8 điểm 13), nhưng mô hình quyền Request chưa chốt (CR-REQ-010). Tạm thời kiểm tenant, user có mặt.
