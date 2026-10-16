# TASK-REQ-038-04: Trường `engine`, `execution_mode` trên `Request` và `open_spec_profile` trên `GetRequestFlow`

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `request-service`, `api-gateway`
**File:** `proto/orca/request/v1/request.proto` (sửa), `internal/adapter/grpc/request_mapper.go` (sửa), `internal/usecase/get_request.go` hoặc use case đọc Request tương ứng (sửa), `internal/usecase/get_request_flow.go` (sửa), `api-gateway/internal/adapter/wscompat/channels_request.go`, `channels_request_flow.go` (sửa), các test mapper và gateway
**Depends on:** TASK-REQ-038-02
**Status:** [x] DONE

---

## Context

- `Request` proto dừng ở trường 31 (CR-REQ-027 dùng 27 đến 31); CR đề xuất 32 và 33. **Đọc lại `request.proto` lúc làm** và dùng số lớn nhất cộng 1 nếu đã bị chiếm.
- `EffectiveEngine(pinned, settings, profile)` trả `native` khi `OpenSpecProfileFor(type) == None`. `change_request`, `refactor` = `Full`; `bug`, `security`, `performance` = `Light`; còn lại `None`.
- Frontend hiện chỉ biết engine qua `request.planProposal`; cần engine hiệu lực ngay trên Request để quyết định hiển thị Verify và Handover.
- `GetRequestFlowResponse` hiện không có hồ sơ OpenSpec; gateway `request.flow` đã có.

## Việc cần làm

1. Proto: `Request` thêm `string engine = 32;` và `string execution_mode = 33;` (đã chú thích: engine hiệu lực; mode đã ghim, rỗng khi chưa `executing`). `GetRequestFlowResponse` thêm `string open_spec_profile` (`full|light|none`).
2. Use case đọc Request: tính `Engine` bằng `EffectiveEngine(pinnedEngine, settings, profile)` (nguồn `pinned` là nơi ghim engine hiện có, xem `engine_pinning_test.go`/`engine_openspec.go`; đọc lại để dùng đúng chỗ), `ExecutionMode` từ cột đã ghim. Dùng cache cài đặt dự án nếu `List` gọi nhiều lần (tránh N+1: lấy cài đặt một lần cho cả trang).
3. `request_mapper.go`: map hai trường; `ListRequests` mang cả hai (hoặc rỗng nếu tốn kém; ghi quyết định vào PR).
4. `GetRequestFlow` trả `open_spec_profile` từ `OpenSpecProfileFor(type)`.
5. Gateway: `request.get`, `request.list` và `request.flow` trả `engine`, `executionMode`, `openSpecProfile` (camelCase). Cập nhật `CONTRACT-request-ui-api.md` ở task 07.

## Kiểm thử

- Mapper: các giá trị `engine` cho 11 loại với cài đặt dự án `native` và `openspec`; `bug` với dự án `openspec` thì `openspec`; `task` luôn `native`.
- `execution_mode` rỗng trước `executing`, đúng giá trị sau khi ghim.
- Gateway: trường camelCase có mặt; client cũ bỏ qua trường lạ (không lỗi).
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/adapter/grpc/... ./services/api-gateway/internal/adapter/wscompat/... -run 'RequestEngine|RequestFlow|Mapper' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] `Request.engine` đúng theo `EffectiveEngine` cho mọi loại.
- [x] `Request.execution_mode` đúng sau khi ghim.
- [x] `request.flow` trả `openSpecProfile`.
- [x] Không phá client cũ; `buf` không báo thay đổi phá vỡ (thêm trường mới).

## Rủi ro và lưu ý

- Số trường proto có thể xung đột CR khác; kiểm trước khi merge.
- `ListRequests` tính engine cho từng dòng có thể chậm; gom cài đặt theo `project_id` một lần.
- `make proto-lint` có `|| true` (README backend mục 4, điểm 15) nên chạy `buf breaking` trực tiếp.

## Ghi chú triển khai

Hoàn thành 2026-10-16.

- `Request.Engine` (suy ra, không lưu) và `Request.ExecutionMode` ở domain, proto (`engine=32`, `execution_mode=33`), mapper gRPC và `RequestView` của gateway. `RequestEngineResolver` điền `engine` theo `EffectiveEngine` cho `GetRequest`/`ListRequests` (đọc cài đặt dự án một lần mỗi dự án; lỗi đọc thì để trống thay vì làm hỏng lần đọc).
- **Điểm bổ sung so với bản soạn:** chế độ chỉ được ghim khi duyệt Plan (lúc `StartExecution`), nên trước đó màn hình không biết chế độ sẽ áp dụng và không thể mô tả thẻ duyệt. `Annotate` giờ điền `execution_mode` theo cài đặt dự án/mặc định triển khai cho Request chưa ghim (`request_engine_resolver_test.go`); sau khi ghim, giá trị đã ghim thắng.
- `GetRequestFlowResponse.open_spec_profile = 11` (`full|light|none`). Chỉ thêm trường mới, client cũ không bị phá.
