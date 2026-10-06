# TASK-REQ-026-01: Domain engine, hồ sơ OpenSpec theo loại, `change_id` và mã lỗi

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/engine_name.go`, `internal/domain/openspec_profile.go`, `internal/domain/engine_settings.go`, `internal/domain/openspec_change.go`, `internal/domain/engine_errors.go`, `internal/domain/request_flow_registry.go` (sửa, của SOL-003) và các `*_test.go` (mới)
**Depends on:** TASK-REQ-003-01 (flow registry), TASK-REQ-002-02 (domain `RequestType`, `apperrors` constructor)
**Status:** [ ] TODO

---

## Context

`backend-go/services/request-service/` chưa có mã lúc soạn (đã kiểm `ls`). Task này chỉ đặt kiểu thuần Go, không I/O, không import ngoài stdlib và `common/apperrors`, theo `arch/03` (domain không biết DB hay gRPC). `FlowDefinition` do TASK-REQ-003-01 tạo là cấu trúc phẳng (`AnalysisKind`, `PlanKind`, `PhaseRule`, `StartGate`, `ExecutionGates`, `CompletesAfterAnalysis`); CR-REQ-026 mục 9 yêu cầu thêm `OpenSpecProfile` vào đó. SOL-007 lại viết `FlowFor(type).Analysis.MinOptions` (lồng): mâu thuẫn có sẵn, task này **không** đụng `MinOptions`, chỉ thêm một trường phẳng.

Quy ước dự án: tên file theo khái niệm (`openspec_profile.go`, không `helpers`), không thêm `max-lines` disable, mã lỗi tiền tố `REQUEST_` và tạo qua `apperrors.New(kind, code, msg, cause)` như `task-service/internal/usecase/update_task.go` (`apperrors.KindNotFound`, `KindInvalidArgument`...).

## Việc cần làm

1. `engine_name.go`: `type EngineName string`;
   - hằng `EngineNative = "native"`, `EngineOpenSpec = "openspec"`
   - `ParseEngineName(s string) (EngineName, error)` (rỗng và giá trị lạ trả `REQUEST_ENGINE_INVALID`, `KindInvalidArgument`)
   - `AllEngineNames() []EngineName`.
2. `openspec_profile.go`: `type OpenSpecProfile string` (`full|light|none`);
   - `func OpenSpecProfileFor(t RequestType) OpenSpecProfile` theo bảng CR mục 2.2 (`change_request`,`refactor` là `full`; `bug`,`security`,`performance` là `light`; `task`,`docs`,`ops_request`,`hotfix`,`spike`,`question` là `none`)
   - loại không biết trả `none`.
   - Một test bảng chạy qua `AllRequestTypes()` để loại mới không thể bị bỏ sót (fail nếu thiếu dòng).
3. `request_flow_registry.go` (sửa): thêm trường `OpenSpecProfile OpenSpecProfile` vào `FlowDefinition`, điền từ `OpenSpecProfileFor(t)` trong hàm khởi tạo bảng;
   - không đổi chữ ký `FlowFor`.
   - Cập nhật test registry (TASK-REQ-003-01) kiểm trường mới.
4. `engine_settings.go`: `ProjectEngineSettings{TenantID, ProjectID string; Engine EngineName; MinVersion string; UpdatedBy string; UpdatedAt time.Time; Version int64}`;
   - `NewProjectEngineSettings(tenantID, projectID, engine, minVersion, actor string, now time.Time) (ProjectEngineSettings, error)` (tenant, project, actor bắt buộc; `MinVersion` rỗng hoặc khớp `^\d+\.\d+\.\d+$`).
5. `EffectiveEngine(pinned *EngineName, settings *ProjectEngineSettings, profile OpenSpecProfile) EngineName`: `profile==none` thì `native` (kể cả khi `pinned==openspec`, vì người dùng đổi loại sang loại `none` sau khi ghim);
   - ngược lại `pinned` nếu khác nil
   - ngược lại `settings.Engine` nếu có
   - ngược lại `native`.
   - Hàm thuần.
6. `openspec_change.go`: `ChangeStatus` (`preparing|ready|archived|abandoned`), `SyncState` (`in_sync|pending|failed`), struct `OpenSpecChange` đúng cột ở SOL-026 mục 2.B;
   - phương thức `MarkReady()`, `MarkArchived(commit string)`, `Abandon()` có bảng chuyển hợp lệ (`preparing→ready→archived`; `preparing|ready→abandoned`; còn lại trả `REQUEST_OPENSPEC_CHANGE_STATE_INVALID`).
7. `NewChangeID(number int64, title string) string`: chuẩn hoá NFD (`golang.org/x/text/unicode/norm`, đã có trong `task-service/go.mod`; kiểm đã nằm trong `go.mod` của `request-service` khi TASK-REQ-001-01 xong, nếu chưa thì thêm), bỏ dấu kết hợp (`unicode.Mn`), đổi `đ`→`d`, hạ chữ thường, thay mọi ký tự ngoài `[a-z0-9]` bằng `-`, gộp `-` liên tiếp, bỏ `-` đầu cuối, cắt tối đa 40 ký tự (không để kết thúc bằng `-`), rỗng thì `request`;
   - trả `req-<number>-<slug>`.
   - `ValidChangeID(s string) bool` dùng `^req-[0-9]+-[a-z0-9-]{1,40}$`.
8. `engine_errors.go`: constructor cho `REQUEST_ENGINE_INVALID`, `REQUEST_ENGINE_PREFLIGHT_FAILED`, `REQUEST_ENGINE_NO_CONNECTION`, `REQUEST_ENGINE_OPENSPEC_MISSING`, `REQUEST_ENGINE_OPENSPEC_VERSION`, `REQUEST_ENGINE_OPENSPEC_NOT_INITIALIZED`, `REQUEST_ENGINE_CLAUDE_MISSING`, `REQUEST_ENGINE_CLAUDE_NOT_AUTHENTICATED` (đều `KindFailedPrecondition`), `REQUEST_ENGINE_OVERRIDE_NOT_ALLOWED` (`KindPermissionDenied`), `REQUEST_ENGINE_REPO_NOT_RESOLVED`, `REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT` (`KindAborted` hoặc `KindFailedPrecondition`, theo kiểu `apperrors` đang có), `REQUEST_OPENSPEC_INVALID_OUTPUT`, `REQUEST_OPENSPEC_AGENT_FAILED`, `REQUEST_OPENSPEC_TIMEOUT`, `REQUEST_OPENSPEC_OUT_OF_SCOPE_CHANGE`, `REQUEST_OPENSPEC_CHANGE_STATE_INVALID`.
   - Ba mã `REQUEST_OPENSPEC_*` đầu ghi vào `analysis_runs.error_code`, không ném ra RPC (như SOL-007).

## Kiểm thử

- `TestParseEngineName_ValidAndInvalid`
- `TestOpenSpecProfileFor_AllElevenTypes` (đủ 11 loại, fail nếu `AllRequestTypes()` có loại mới mà bảng thiếu)
- `TestFlowFor_CarriesOpenSpecProfile`.
- `TestEffectiveEngine_Table`: tổ hợp `pinned` (nil, native, openspec) x `settings` (nil, native, openspec) x `profile` (full, light, none) = 27 dòng; khẳng định `none` luôn `native`.
- `TestNewChangeID_Cases`: tiêu đề tiếng Việt có dấu ("Thêm đăng nhập bằng Google" cho `req-142-them-dang-nhap-bang-google`), có `Đ`, emoji, toàn ký tự đặc biệt (`req-7-request`), rỗng, 200 ký tự (slug ≤ 40 và không kết thúc `-`), kết quả luôn qua `ValidChangeID`.
- `TestOpenSpecChange_StateMachine` (mọi cặp hợp lệ và không hợp lệ).
- Fuzz `FuzzNewChangeID`: không panic, kết quả luôn khớp mẫu.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'Engine|OpenSpec|ChangeID|FlowFor'` và `go test ./services/request-service/internal/domain/ -fuzz FuzzNewChangeID -fuzztime 20s`.

## Tiêu chí hoàn thành

- [ ] `go vet ./services/request-service/internal/domain/...` sạch; domain không import gói ngoài stdlib, `golang.org/x/text`, `common/apperrors`.
- [ ] `OpenSpecProfileFor` phủ đủ 11 loại và có test chống thiếu.
- [ ] `EffectiveEngine` có test 27 tổ hợp; `none` luôn trả `native`.
- [ ] `NewChangeID` luôn thoả `ValidChangeID` (fuzz 20 giây không lỗi).
- [ ] Mọi mã lỗi liệt kê ở bước 8 có constructor và kind đúng; không trùng mã với `request_flow_errors.go`.
- [ ] Không có `max-lines` disable; tên file không chứa `helpers`, `utils`, `common`.

## Rủi ro và lưu ý

- `FlowDefinition` còn đang được TASK-REQ-003-01 viết: nếu task đó chưa merge, làm bước 3 trên nhánh của nó hoặc đợi; không tạo bản sao struct.
- `golang.org/x/text` có trong `task-service/go.mod` nhưng chưa chắc là phụ thuộc trực tiếp của `request-service`; chạy `go mod tidy` trong module mới (TASK-REQ-001-01) và không dùng `replace`.
- `change_id` được dùng làm tên thư mục và tham số lệnh; một bug ở `NewChangeID` là lỗ hổng đường dẫn, nên mẫu `ValidChangeID` phải được kiểm lại ở mọi điểm dùng (task 026-06), không chỉ lúc sinh.
- Bảng `OpenSpecProfileFor` là mã Go tĩnh theo CR; đổi cho một loại là thay đổi hợp đồng, cần sửa CR trước.
