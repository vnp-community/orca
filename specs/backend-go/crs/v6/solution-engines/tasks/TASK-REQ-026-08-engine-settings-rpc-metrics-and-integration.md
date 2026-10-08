# TASK-REQ-026-08: RPC `Get/SetProjectEngineSettings`, metric, audit và kiểm thử tích hợp hai dialect

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `proto/orca/request/v1/request_engine.proto`, `internal/usecase/manage_project_engine_settings.go`, `internal/adapter/grpc/engine_settings_server.go`, `internal/adapter/metrics/engine_metrics.go`, `services/request-service/README.md` (sửa phần cấu hình), `internal/usecase/engine_flow_integration_test.go` (mới)
**Depends on:** TASK-REQ-026-02, 026-05, 026-06, 026-07, TASK-REQ-001-05 (gRPC server, health), TASK-REQ-024-01 (`AppendDetailed`, nếu chưa có thì dùng `auditclient.Append`)
**Status:** [ ] TODO

---

## Context

Cờ cấp project `solution_engine` cần RPC để admin đặt và đọc (CR-REQ-026 mục 2.2). README v6 mục 8 điểm 13: gateway không kiểm quyền OPA trước định tuyến, nên **mọi RPC của `request-service` tự kiểm quyền**; mã quyền theo `common/tenant` (`tenant.Role(ctx)=="admin"`, `tenant.UserID(ctx)`). Kênh WS `request.engineGet`, `request.engineSet` do CR-REQ-016 (`BE-REQ-SOL-016`) đặt tên; task này chỉ làm gRPC và ghi lại payload cho bên gateway.

`common/auditclient.Append` hiện không mang `actor_type`, `target_type` (README mục 8 điểm 14); TASK-REQ-024-01 thêm `AppendDetailed`. `issue-status-sync` chưa có `/metrics`; `request-service` tự đăng ký metric Prometheus theo mẫu của TASK-REQ-024-07 (đọc tên package metric ở đó, không tạo cách thứ hai).

## Việc cần làm

1. `request_engine.proto` (package `orca.request.v1`, thêm vào `RequestService`): `rpc GetProjectEngineSettings(GetProjectEngineSettingsRequest) returns (ProjectEngineSettings)`;
   - `rpc SetProjectEngineSettings(SetProjectEngineSettingsRequest) returns (ProjectEngineSettings)`
   - message như SOL-026 mục 2.H
   - thêm `string solution_engine_effective` vào `Request` nếu CR-REQ-016 cần hiển thị (kiểm CONTRACT-request-ui-api trong `gateway-and-mcp`).
   - Chạy `buf generate`, `buf lint`, `buf breaking`.
2. `manage_project_engine_settings.go`: `GetProjectEngineSettings.Execute(ctx, projectID)`: `RequireTenantID`;
   - không có dòng thì trả `{engine:"native", version:0}` (không lỗi, không ghi).
   - `SetProjectEngineSettings.Execute(ctx, in)`: chỉ `admin` (`REQUEST_FORBIDDEN` hoặc mã sẵn có của SOL-009 `REQUEST_APPROVAL_FORBIDDEN`: dùng mã chung của service, đọc `request_errors.go`), `ParseEngineName`, `MinVersion` đúng `\d+\.\d+\.\d+`, `Upsert` CAS theo `expected_version` (0 nghĩa là tạo mới), ghi audit (`action="request.engine_settings.set"`, `target_type="project"`, `target_id=projectID`, `before/after` chỉ gồm `solution_engine`, `openspec_min_version`).
3. Khi `engine=openspec` được đặt: **không** chạy Preflight ngay (tốn kết nối);
   - trả cảnh báo `preflight_not_run` trong response (trường `warnings`), để UI gợi ý nút "Kiểm tra điều kiện" (RPC `CheckEngineReadiness` chưa có trong CR: ghi Q, không làm).
4. `engine_settings_server.go`: ánh xạ lỗi sang gRPC qua `apperrors.ToGRPCStatus`;
   - từ chối khi thiếu metadata tenant
   - log có `tenant_id`, không log `openspec_min_version` thô nếu rỗng.
5. `engine_metrics.go`: `request_engine_runs_total{engine,kind,result}`, `request_engine_preflight_failures_total{code}`, `request_openspec_sync_state{state}` (gauge, cập nhật bởi vòng quét 026-07), `request_engine_drift_total`;
   - nhãn chỉ nhận tập giá trị đóng (kiểm bằng hằng, không nhãn từ đầu vào người dùng)
   - gọi từ `RunSolutionGeneration`, gate Preflight, `TasksMdSyncer`.
6. Tích hợp (file `engine_flow_integration_test.go`, tag `integration`): kịch bản đầu cuối với fake dev server, fake git-gateway, Postgres và MySQL thật: (a) Request `change_request` + project `openspec` → `GenerateSolution` tạo run `engine=openspec`, `mode=agent_proposal`, Solution `proposed`, Approval `pending`, Request `awaiting_analysis_approval` (cùng một transaction phần DB);
   - (b) loại `hotfix` ở project `openspec` chạy `native`
   - (c) preflight lỗi: không run, không worktree mới
   - (d) đổi cờ sau khi ghim không đổi engine
   - (e) `request.completed` giao lặp hai lần: một lần archive.
7. README của service: bảng biến môi trường mới (`REQUEST_OPENSPEC_ENABLED`, `REQUEST_OPENSPEC_MIN_VERSION`, `REQUEST_OPENSPEC_TIMEOUT_MS`, `REQUEST_OPENSPEC_SYNC_INTERVAL`, `REQUEST_ENGINE_SKIP_CLAUDE_AUTH_PROBE`), mặc định, và mục "Chưa kiểm chứng trên OpenSpec thật".
8. Ghi vào `docs` của PR danh sách thay đổi CR khác cần sửa (CR mục 9): README v6, CR-REQ-002/003/007/008/012/013/016/017/024/025;
   - **không** sửa các CR đó trong task này.

## Kiểm thử

- `TestSetProjectEngineSettings_AdminOnly`
- `TestSetProjectEngineSettings_InvalidEngine`
- `TestSetProjectEngineSettings_InvalidMinVersion`
- `TestSetProjectEngineSettings_CASConflict`
- `TestSetProjectEngineSettings_CreatesWhenVersionZero`
- `TestSetProjectEngineSettings_WritesAuditWithBeforeAfter`; `TestGetProjectEngineSettings_DefaultsToNative`.
- `TestEngineMetrics_LabelsAreClosedSet` (đăng ký rồi gather, so tên và nhãn; không có nhãn tự do).
- Integration hai dialect: `CountRunning` đếm cả `agent_readonly` và `agent_proposal`; kịch bản (a) đến (e) ở bước 6; Postgres RLS: tenant B không đọc cờ tenant A.
- Hợp đồng: `buf breaking`; test proto→domain ánh xạ `ProjectEngineSettings`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... ./services/request-service/internal/adapter/grpc/... -run 'EngineSettings|EngineMetrics' && go test -tags=integration ./services/request-service/... -run 'EngineFlow|CountRunning'`.
- Chưa kiểm chứng (không giả vờ): chạy trên dev server và OpenSpec thật; lệnh thủ công ghi trong README của service.

## Tiêu chí hoàn thành

- [ ] `SetProjectEngineSettings` từ người không phải admin bị từ chối; admin thành công có dòng audit.
- [ ] Không có dòng cờ: `GetProjectEngineSettings` trả `native`, và không ghi DB.
- [ ] Bốn metric có mặt, nhãn là tập đóng.
- [ ] Năm kịch bản tích hợp xanh trên cả Postgres và MySQL.
- [ ] README service liệt kê đủ biến môi trường và chỉ rõ phần chưa kiểm chứng.
- [ ] `buf lint` và `buf breaking` sạch.

## Rủi ro và lưu ý

- `CheckEngineReadiness` (nút kiểm tra) không có trong CR; thiếu nó thì admin chỉ biết lỗi khi sinh Solution đầu tiên. Đề xuất bổ sung CR hoặc làm ở CR-REQ-016.
- Audit `before/after` không được chứa bí mật; hiện chỉ có hai trường vô hại.
- Metric `request_openspec_sync_state` là gauge theo nhãn trạng thái; nếu tính từ DB mỗi lần quét sẽ nặng khi nhiều Request, nên chỉ đếm theo `GROUP BY tasks_sync_state` có chỉ mục.
- Kịch bản tích hợp dùng fake: không thay thế được kiểm thử thật; CR-REQ-025 mới là nơi kiểm thật.
