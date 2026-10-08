# TASK-REQ-001-02: Proto `orca.request.v1` (RequestService, ApprovalService) và sinh stub

**From Solution:** BE-REQ-SOL-001
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/request/v1/request.proto` (mới), `backend-go/proto/orca/request/v1/approval.proto` (mới), `backend-go/proto/gen/go/orca/request/v1/*.pb.go` (sinh)
**Depends on:** Không (song song được với TASK-REQ-001-01, 001-03)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `cd backend-go/proto && buf lint --path orca/request && buf breaking --against '../../.git#branch=HEAD,subdir=backend-go/proto' --path orca/request && buf generate --path orca/request`)

---

## Context

`proto/buf.yaml` dùng lint `STANDARD`, breaking `FILE`; `buf.gen.yaml` sinh `protoc-gen-go` và `protoc-gen-go-grpc` với `paths=source_relative` vào `gen/go`. Mẫu: `proto/orca/mcp/v1/mcp.proto` (package `orca.mcp.v1`, `go_package` kiểu `github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1;mcpv1`). Makefile `proto-lint` kết thúc bằng `|| true` nên không chặn.

## Việc cần làm

1. Tạo `request.proto`: `package orca.request.v1;`, `option go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1;requestv1";`, `import "google/protobuf/timestamp.proto";`.
2. Định nghĩa `message Request` với số trường cố định (không đổi về sau): `id=1, project_id=2, number=3, title=4, body=5, source_provider=6, source_ref=7, source_url=8, source_site=9, type=10, type_source=11, size=12, urgency=13, confidence=14 (optional double, để phân biệt chưa có đề xuất với 0), classification_reason=15, status=16, returned_from_stage=17, return_reason=18, plan_task_id=19, reporter_id=20, created_at=21 (Timestamp), updated_at=22, version=23 (int64)`. Ghi chú: 24 trở đi dành cho `source_hints` (CR-REQ-004) và `returned_category` (CR-REQ-006). Không có `tenant_id` trên dây.
3. `service RequestService` chỉ có `GetRequest` và `ListRequests` (message ở SOL-001 mục F). `ListRequestsRequest`: `project_id=1`, `repeated string status=2`, `repeated string type=3`, `int32 page_size=4`, `string page_token=5`; ghi chú dành 6 đến 8 cho CR-REQ-004.
4. `approval.proto`: `service ApprovalService {}` rỗng, cùng package và `go_package`. Nếu `buf lint` báo lỗi dịch vụ rỗng thì bỏ file và ghi vào Câu hỏi mở Q3 của SOL-001, đồng thời bỏ `RegisterApprovalServiceServer` ở TASK-REQ-001-05.
5. `make proto-gen`; kiểm tra thư mục `proto/gen/go/orca/request/v1/` có `request.pb.go`, `request_grpc.pb.go`.
6. Không khai báo RPC khác: các RPC còn lại của README v6 mục 3.6 do CR sở hữu thêm cùng file (cộng thêm không phá `buf breaking`).

## Kiểm thử

- `cd backend-go/proto && buf lint && buf breaking --against '../.git#branch=main,subdir=backend-go/proto'` (đường dẫn `--against` có thể cần chỉnh theo cách CI mcp-service gọi; đọc `.github/workflows/backend-go-mcp-service.yml` trước).
- Test biên dịch: `go build ./proto/...` từ `backend-go`.
- Test round-trip proto nhỏ trong `services/request-service/internal/adapter/grpc` (ở TASK-REQ-002-07): `Request` marshal rồi unmarshal giữ nguyên số trường.

## Tiêu chí hoàn thành

- [x] `buf lint --path orca/request` sạch (0 lỗi); `buf breaking` so với HEAD sạch (có `ignore_only` RPC_SAME_*_TYPE cho `approval.proto`, xem RPC-CATALOG.md). Đã mở rộng proto-first toàn bộ miền Request (xem RPC-CATALOG.md).
- [x] (đã kiểm chứng 2026-10-07: `buf generate --path orca/request` cho ra mã trùng, chỉ khác dòng phiên bản protoc-gen-go-grpc) Stub sinh vào `proto/gen/go/orca/request/v1/` và commit cùng PR (kiểm tra repo có commit thư mục `gen` như service khác).
- [x] Số trường `Request` khớp danh sách ở bước 2.
- [x] Không có RPC nào không có message thật.

## Rủi ro và lưu ý

- `repeated` ở `status`, `type` khác CR-REQ-001 (số ít); đây là quyết định của SOL-001 mục F để khỏi phá wire sau này. Báo người duyệt CR khi review.
- Makefile `proto-lint` bỏ qua lỗi: tự chạy `buf` trực tiếp, đừng tin `make proto-lint` xanh.

## Tiến độ

Đã kiểm chứng 2026-10-07: `request.proto` (`GetRequest`, `ListRequests` với `repeated status/type`, `ListBacklog`) khớp danh sách trường; mã sinh trong `proto/gen/go/orca/request/v1` đồng bộ với `.proto`; `go build ./...` xanh. `approval.proto` hiện có đủ 7 RPC (khác task, vốn muốn service rỗng) do đợt trước thêm vào; nó vi phạm lint STANDARD (xem tiêu chí 1). Việc đổi tên response để hết lỗi lint thay đổi API approval nên để đợt Approval (R2) làm; workflow CI chỉ gate lint trên hai file request cho tới lúc đó.

- 2026-10-08: proto-first toàn bộ `request-service`; `approval.proto` tách response dùng chung; sinh lại mã; `go build/vet/test` xanh ở proto, request-service, api-gateway, mcp-service, orchestration-service.
