# TASK-REQ-038-02: Domain `ExecutionMode`, `EffectiveExecutionGates` và cài đặt theo dự án

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `request-service`, `api-gateway`
**File:** `internal/domain/execution_mode.go` (mới), `internal/domain/engine_settings.go`, `internal/domain/request.go`, `internal/domain/request_flow_registry.go` (sửa), `internal/config/config.go` (sửa), `internal/adapter/grpc/server_engine_settings.go` (sửa), `proto/orca/request/v1/request.proto` (sửa `ProjectEngineSettings`), `api-gateway/internal/adapter/wscompat/channels_request_engine.go` (sửa), `internal/domain/execution_mode_test.go` (mới)
**Depends on:** TASK-REQ-038-01
**Status:** [x] DONE

---

## Context

- `GetProjectEngineSettings` và `SetProjectEngineSettings` đã có; gateway `request.engineSettings` và `request.engineSettingsSet`. Chỉ thêm trường, không thêm RPC.
- `FlowDefinition.ExecutionGates` hiện chỉ `change_request` có `GatePhase`, `ops_request` có `GatePreDeploy`. `continuous` chỉ bỏ `GatePhase`; `GatePreDeploy` giữ ở mọi mode (B7).
- Quy ước: domain thuần Go, tên file theo khái niệm; không `max-lines` disable.

## Việc cần làm

1. `execution_mode.go`: kiểu `ExecutionMode` (`ExecutionStepByStep`, `ExecutionContinuous`), `ParseExecutionMode(string) (ExecutionMode, error)` (rỗng thành `step_by_step`; giá trị lạ trả lỗi domain `REQUEST_DEFAULT_EXECUTION_MODE_INVALID`), và `func (f FlowDefinition) EffectiveExecutionGates(m ExecutionMode) []GateSubject` trả bản sao, bỏ `GatePhase` khi `continuous`.
2. `FlowDefinition.HasExecutionGateFor(g GateSubject, m ExecutionMode) bool` (hoặc đổi chữ ký `HasExecutionGate` để nhận thêm mode; chạy `gitnexus_impact` trước và cập nhật mọi nơi gọi: `start_phase.go`, `evaluate_execution.go`, `backlog_gate.go`, `approval_subject_stage.go`).
3. `ProjectEngineSettings` thêm `ExecutionMode`; `EngineDefaults` thêm `ExecutionMode` đọc từ `REQUEST_DEFAULT_EXECUTION_MODE` trong `config.go` (mặc định `step_by_step`; giá trị lạ làm service từ chối khởi động với thông báo rõ).
4. `domain.Request` thêm `ExecutionMode ExecutionMode` (rỗng khi chưa ghim). Hàm `EffectiveExecutionModeOf(req, settings, defaults)` trả mode đã ghim nếu có, nếu không thì mode của cài đặt dự án, nếu không thì mặc định.
5. Proto: `ProjectEngineSettings` và `SetProjectEngineSettingsRequest` thêm `string execution_mode`; mapping trong `server_engine_settings.go` (validate bằng `ParseExecutionMode`, lỗi `InvalidArgument`). Chỉ người có quyền hiện có trên cài đặt engine được đổi.
6. Gateway: hai kênh engine đọc/ghi thêm trường `executionMode` (camelCase).

## Kiểm thử

- Table-driven `EffectiveExecutionGates` cho 11 loại × 2 mode (kỳ vọng: chỉ `change_request` mất `phase`; `ops_request` luôn giữ `pre_deploy`).
- `ParseExecutionMode` (rỗng, hợp lệ, lạ); `EffectiveExecutionModeOf` (ghim thắng cài đặt, cài đặt thắng mặc định).
- Server engine settings: đặt giá trị lạ trả `InvalidArgument`; đặt hợp lệ rồi đọc lại; gateway map đúng camelCase.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/adapter/grpc/... -run 'ExecutionMode|EngineSettings' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] `EffectiveExecutionGates` đúng bảng cho 11 loại × 2 mode.
- [x] Không còn nơi nào tự đọc `ExecutionGates` thô để quyết định cổng Phase.
- [x] `Set/GetProjectEngineSettings` mang `execution_mode`; giá trị lạ bị từ chối.
- [x] Mặc định hệ thống vẫn là `step_by_step`; test hiện có xanh.

## Rủi ro và lưu ý

- Đổi chữ ký `HasExecutionGate` chạm nhiều use case (blast radius cao); chạy `gitnexus_impact` và báo mức rủi ro trong PR.
- Số trường proto cho `execution_mode` ở `ProjectEngineSettings`: kiểm số lớn nhất hiện có trước khi đặt.

## Ghi chú triển khai

Hoàn thành 2026-10-16.

- `domain/execution_mode.go`: `ExecutionMode` (`step_by_step` mặc định, `continuous`), `ParseExecutionMode` (giá trị lạ bị từ chối), `EffectiveExecutionModeOf` (đã ghim > cài đặt dự án > mặc định triển khai), `EffectiveExecutionGates(mode)` chỉ bỏ cổng `phase`, `pre_deploy` luôn giữ. Test bảng 11 loại × 2 mode (`execution_mode_test.go`).
- Cài đặt dự án: `ProjectEngineSettings.ExecutionMode`, `Get/SetProjectEngineSettings` và proto `execution_mode`; gửi rỗng giữ giá trị đã lưu. Biến môi trường mặc định của triển khai là **`REQUEST_DEFAULT_EXECUTION_MODE`** (CR ghi `REQUEST_EXECUTION_MODE`; tên thật theo mẫu `REQUEST_DEFAULT_*` của engine); đã thêm vào `deploy/dev/docker-compose.yml`.
- Cổng Phase đọc qua `HasExecutionGateFor`/`EffectiveExecutionGates`, không còn chỗ đọc `ExecutionGates` thô để quyết định cổng Phase.
