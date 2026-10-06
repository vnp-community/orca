# BE-CV-TASK-086-07: Handler gRPC, wiring `main.go`, ma trận capability

**From Solution:** BE-CV-SOL-086-scm-commit-checks
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/grpc/server_commit_checks.go`, `cmd/server/main.go` (sửa), `README.md` của service (mục Known gaps)
**Depends on:** BE-CV-TASK-086-01, 086-03, 086-04, 086-06
**Status:** [ ] TODO

## Context
`grpc.New` có ~47 tham số vị trí: dùng `WithCommitChecks`.

## Việc cần làm
1. Handler ánh xạ proto↔domain; lỗi qua `apperrors.ToGRPCStatus`.
2. `main.go`: tạo use case, `scmgrpc.New(...).WithCommitChecks(uc)`.
3. Test ma trận: github/gitlab thật (httptest), bitbucket/azure/gitea ⇒ unsupported.

## Kiểm thử
- `go build ./... && go vet ./... && go test ./...`; workflow `backend-go-scm-integration-service.yml` không cần đổi.

## Tiêu chí hoàn thành
- [ ] RPC gọi được bằng `grpcurl` (reflection bật sẵn); không file frontend bị sửa.

## Rủi ro
Thứ tự đăng ký trong `main.go` dễ nhầm do danh sách dài.
