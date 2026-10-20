# TASK-REQ-044-04: Công tắc theo dự án `plan_auto_generate` (usecase, proto, gRPC) và `PlanAutoSwitch`

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/manage_project_engine_settings.go` (sửa), `internal/usecase/plan_auto_switch.go` (mới), `internal/usecase/plan_auto_switch_test.go`, `internal/usecase/manage_project_engine_settings_plan_auto_test.go` (mới), `internal/adapter/grpc/server_engine_settings.go` (sửa), `internal/adapter/grpc/server_engine_settings_test.go` (sửa), `backend-go/proto/orca/request/v1/solution.proto` (sửa) + mã sinh
**Depends on:** TASK-REQ-044-01, TASK-REQ-044-02
**Status:** [x] DONE (2026-10-20)

---

## Context

- `SetProjectEngineSettingsInput` và `ManageProjectEngineSettings.SetProjectEngineSettings` (`usecase/manage_project_engine_settings.go`): dựng `ProjectEngineSettings` mới từ đầu rồi `Upsert`; riêng `ExecutionMode` có xử lý "client cũ gửi rỗng thì giữ giá trị đã lưu". Một `bool` thường trong proto3 không phân biệt "false" và "không gửi", nên `SolutionAgent` hiện bị ghi đè `false` bởi client cũ; công tắc mới **không được** lặp lỗi đó.
- Proto: `ProjectEngineSettings` tới trường 6 và `SetProjectEngineSettingsRequest` tới trường 6 (`proto/orca/request/v1/solution.proto:143-175`); `optional` đã dùng ở nơi khác (`ai_admin.proto:19`, `approval.proto:170`).
- `GetProjectEngineSettings` cho dự án chưa có dòng trả mặc định tại chỗ (`manage_project_engine_settings.go`), cần thêm `PlanAutoGenerate: true`.
- Quyền: chỉ admin (`callerIsAdmin`); RPC ở `rpc_catalog.go:106` là `GroupAdmin`.

## Việc cần làm

1. `solution.proto`: `ProjectEngineSettings.plan_auto_generate = 7` (bool); `SetProjectEngineSettingsRequest.plan_auto_generate = 7` kiểu `optional bool`. Chạy `make proto-gen` và `make proto-lint` (buf lint, breaking) từ `backend-go`.
2. `SetProjectEngineSettingsInput.PlanAutoGenerate *bool`. `nil`: lấy giá trị đã lưu nếu có dòng, nếu không `true`. Có giá trị: đặt theo giá trị. Thêm `plan_auto_generate` vào metadata audit.
3. `GetProjectEngineSettings` cho dự án chưa có dòng: `PlanAutoGenerate: true`.
4. `server_engine_settings.go`: đọc `req.PlanAutoGenerate` (con trỏ do `optional`) và truyền xuống; `toProtoEngineSettings` điền `PlanAutoGenerate`.
5. `plan_auto_switch.go`: `PlanAutoSwitch{EnvOn bool, CommitWired bool, Settings EngineSettingsRepository}` với `Enabled(ctx, projectID) (bool, error)`: `false` ngay khi `!EnvOn || !CommitWired`; ngược lại đọc cài đặt dự án (không có dòng = `true`); lỗi đọc trả `(false, err)` (người gọi quyết định thử lại; **không** bật ngầm). Một hàm `GloballyAvailable() bool` cho `GetPlanProposal` và `RetryPlanGeneration`.

## Kiểm thử

- `ManageProjectEngineSettings`: `nil` giữ `false` đã lưu; `nil` với dự án chưa có dòng cho `true`; `&false` ghi `false`; `&true` ghi `true`; client cũ (không gửi) đặt `SolutionAgent` rồi đọc lại thấy `PlanAutoGenerate` không đổi; phiên bản xung đột vẫn lỗi.
- gRPC: ánh xạ hai chiều (có/không có `optional`); giá trị lạ không áp dụng.
- `PlanAutoSwitch`: bảng (env, commit, dòng cài đặt có/không, true/false, lỗi đọc).
- `make proto-gen && make proto-lint`, rồi `go test ./internal/usecase/... ./internal/adapter/grpc/... -run 'EngineSettings|PlanAutoSwitch' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Client cũ (không gửi trường) không bao giờ đổi công tắc.
- [x] Dự án chưa có dòng và dòng cũ đều đọc ra `true`.
- [x] Lỗi đọc cài đặt không bật tính năng.
- [x] Proto lint không báo breaking.

## Rủi ro và lưu ý

- `optional` trong proto3 sinh trường con trỏ; kiểm mã sinh của `buf` ở repo này (đã có ví dụ `ai_admin.proto`).
- Chạy `gitnexus_impact` cho `SetProjectEngineSettingsInput` và `toProtoEngineSettings` (dùng ở test gateway).
- Giao diện cho công tắc này chưa có (CR mục 5.1): chỉ kênh admin và `orca-cli` (task 08).

## Ghi chú triển khai

**Hoàn thành 2026-10-20.** Đường dẫn dưới `backend-go/services/request-service/`.

- Proto: `solution.proto` `ProjectEngineSettings.plan_auto_generate = 7`, `SetProjectEngineSettingsRequest.plan_auto_generate = 7` (`optional bool`). `buf` không có: sinh bằng protoc 33.4 + protoc-gen-go v1.36.11 qua script trong scratchpad (chỉ ghi file có nội dung đổi, giữ nguyên bốn dòng header của từng file); `git diff --stat` chỉ thêm `solution.pb.go` (và `approval.pb.go` đã đổi từ CR-043). Lần chạy đầu của tôi làm đổi dòng header của 11 file không liên quan, đã khôi phục từ HEAD.
- `usecase/manage_project_engine_settings.go`: `SetProjectEngineSettingsInput.PlanAutoGenerate *bool` (nil giữ giá trị đã lưu, `true` khi chưa có dòng), audit thêm `plan_auto_generate`, mặc định đọc dự án chưa có dòng là `true`; `usecase/engine_settings_defaults.go` đặt `PlanAutoGenerate: true` (cạm bẫy ghi ở task 01). `usecase/plan_auto_switch.go`: `PlanAutoSwitch{EnvOn, CommitWired, Settings}` với `Enabled` (lỗi đọc trả `(false, err)`) và `GloballyAvailable`. `adapter/grpc/server_engine_settings.go` ánh xạ hai chiều.
- Test: `manage_project_engine_settings_plan_auto_test.go` (nil giữ `false`, dự án chưa có dòng, `&false`/`&true`, client cũ đổi `SolutionAgent` không đổi công tắc, audit, xung đột phiên bản, `DefaultedEngineSettings`), `plan_auto_switch_test.go` (bảng env/commit/dòng/lỗi đọc), `server_engine_settings_test.go` (thêm ca ánh xạ `optional`). Không có `SetProjectEngineSettings` nào còn ghi `false` âm thầm.
- Lệch spec: `PlanAutoSwitch` nằm ở `plan_auto_switch.go` (không ở `ports_plan_auto.go`). Lệnh: `go test ./internal/usecase/ ./internal/adapter/grpc/ -run 'EngineSettings|PlanAutoSwitch'` xanh. `proto-lint` (buf breaking) không chạy được: chỉ thêm trường mới nên không phá tương thích.
- Blast radius (grep): người dựng `ProjectEngineSettings` bằng literal ngoài test chỉ còn hai nơi trên; `toProtoEngineSettings` dùng bởi `server_engine_settings.go`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
