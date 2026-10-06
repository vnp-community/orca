# TASK-REQ-003-05: RPC `GetRequestFlow` (registry và đường chuẩn cho frontend)

**From Solution:** BE-REQ-SOL-003
**Priority:** P1
**Service:** `request-service`, `proto`
**File:** `proto/orca/request/v1/request.proto` (sửa), `proto/gen/go/orca/request/v1/*` (sinh lại), `internal/usecase/get_request_flow.go`, `internal/usecase/get_request_flow_test.go`, `internal/adapter/grpc/server.go` (sửa)
**Depends on:** TASK-REQ-003-02, TASK-REQ-002-07
**Status:** [ ] TODO

---

## Context

README v6 mục 8 điểm 12 liệt kê `GetRequestFlow` là RPC thiếu, giao cho CR-REQ-003. Frontend (CR-REQ-019, 021) cần biết bước của loại đã chọn; CR-REQ-003 Q2 đề nghị RPC này để khỏi sao chép registry. Kênh WS do CR-REQ-016 chốt. Ghi chú proto: cộng thêm RPC và message không phá `buf breaking`.

## Việc cần làm

1. `request.proto`: thêm `rpc GetRequestFlow(GetRequestFlowRequest) returns (GetRequestFlowResponse);` và hai message ở SOL-003 mục G (`type=1`, `size=2`; phản hồi gồm `type`, `human_confirm_required`, `analysis_kind`, `analysis_gate`, `plan_kind`, `has_phases`, `start_gate`, `execution_gates`, `completes_after_analysis`, `status_path`). Sinh lại stub (`make proto-gen`).
2. `usecase/get_request_flow.go`: `GetRequestFlow.Execute(ctx, typ, size string) (FlowView, error)`: `tenant.RequireTenantID`; `ParseRequestType`; `size` rỗng chấp nhận, khác thì `ParseSize`; `FlowFor`; `PhasesFor`; `HappyPath`. Không đọc DB.
3. `adapter/grpc/server.go`: handler `GetRequestFlow`, ánh xạ sang proto, lỗi qua `apperrors.ToGRPCStatus`.
4. Cập nhật bảng real vs stub trong `README.md` của service.

## Kiểm thử

- `TestGetRequestFlow_AllTypes`: với từng loại trả đúng `analysis_kind`, `start_gate`, `execution_gates` theo bảng README mục 3.4; `status_path` đầu `new` cuối `completed`.
- `TestGetRequestFlow_PhasesBySize`: `bug` + `L` có `has_phases=true`, `S` thì `false`; `change_request` luôn `true`; `hotfix` luôn `false`.
- `TestGetRequestFlow_InvalidType` (`InvalidArgument`, `REQUEST_INVALID_TYPE`), `TestGetRequestFlow_InvalidSize`.
- Hợp đồng: `buf breaking` xanh.
- Lệnh: `go test ./services/request-service/internal/usecase/... -run RequestFlow`.

## Tiêu chí hoàn thành

- [ ] RPC trả đúng 11 loại, kể cả `status_path`.
- [ ] `buf lint` và `buf breaking` xanh.
- [ ] README service cập nhật.
- [ ] Không đọc DB (test không cần container).

## Rủi ro và lưu ý

- Gateway chỉ định tuyến được RPC này khi CR-REQ-016 thêm kênh; ở đây chỉ làm phía service.
- `status_path` có thể bị frontend hiểu là mọi đường; ghi chú trong proto: chỉ là đường chuẩn không gồm nhánh trả backlog, đổi loại, huỷ.
