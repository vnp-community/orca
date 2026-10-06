# TASK-REQ-028-08: Proto `clarification.proto`, `decision.proto`, gRPC server, `ListPendingClarificationsForUser` và tích hợp đầu cuối

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service` · `proto`
**File:** `backend-go/proto/orca/request/v1/clarification.proto`, `backend-go/proto/orca/request/v1/decision.proto`, `internal/adapter/grpc/clarification_server.go`, `internal/adapter/grpc/decision_server.go`, `internal/adapter/grpc/clarification_mapper.go`, `internal/usecase/list_pending_clarifications.go`, `internal/usecase/clarification_flow_integration_test.go`, `services/request-service/README.md` (sửa) và test
**Depends on:** TASK-REQ-028-04, 028-05, 028-06, 028-07, TASK-REQ-001-02 (proto khung), TASK-REQ-001-05 (gRPC server), TASK-REQ-010-03 (`ApprovalAuthorization`, tập principal)
**Status:** [ ] TODO

---

## Context

CR-REQ-028 mục 2.9. README v6 mục 8 điểm 13: mọi RPC của `request-service` tự kiểm quyền (gateway không kiểm OPA trước định tuyến). Kênh WS `clarification.list|get|answer|cancel`, `request.readiness`, `decision.list|confirm` và tool MCP `clarification_list`, `clarification_get`, `clarification_answer` do CR-REQ-016/017 đặt tên (đọc `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` để khớp, không sửa file đó); **không** đăng ký `decision_confirm` ở MCP.

Proto theo CR 2.9: `ClarificationStatus` (`CLARIFICATION_STATUS_UNSPECIFIED=0`, `OPEN=1`, `ANSWERED=2`, `EXPIRED=3`, `CANCELLED=4`), `QuestionKind` (`...=0`, `TEXT=1`, `SINGLE_CHOICE=2`, `MULTI_CHOICE=3`, `FILE=4`, `BOOLEAN=5`), `ClarificationQuestion{id, seq, question_key, kind, prompt, reason, options_json, suggested_default_json, required, target_path, answer_json}`, `Clarification{id, display_id, request_id, source, source_ref, status, resume_status, round, questions[], due_at, version}`, `AnswerItem{question_id, value_json, accept_default}`, `AnswerClarificationRequest{clarification_id, answers[], complete, expected_version}`, `AnswerClarificationResponse{clarification, request_status, request_revision, still_missing}`. Thư mục `proto/orca/request/v1` chưa tồn tại lúc soạn (đã kiểm `ls`): số trường và vị trí `go_package` theo `buf.gen.yaml` (đọc lúc làm); proto enum Postgres/MySQL không liên quan, nhưng giá trị chuỗi `status` ở DB và enum proto phải ánh xạ 1-1 trong `clarification_mapper.go`.

## Việc cần làm

1. `clarification.proto` và `decision.proto` (package `orca.request.v1`): thêm các message trên và RPC vào `RequestService` (hoặc service riêng `ClarificationService` nếu TASK-REQ-001-02 đã tách; theo cách đã làm với `ApprovalService`): `GetRequestReadiness`, `RequestClarification` (RPC người dùng, nguồn `solution_open_question|plan_assumption|manual`; nguồn `readiness` và `task_blocked` bị `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED` qua RPC công khai vì chỉ do hệ thống gọi), `ListClarifications`, `GetClarification`, `AnswerClarification`, `CancelClarification`, `ListPendingClarificationsForUser`, `WaiveReadiness`, `ListDecisions`, `GetDecision`, `ConfirmDecision`.
   - `ChooseSolutionOptionRequest` thêm `rationale`
   - `ChooseSolutionOptionResponse` thêm `decision_status`, `requires_confirmation` (cộng thêm).
2. `buf lint`, `buf generate`, `buf breaking --against` nhánh chính; không dùng lại số trường.
3. `clarification_mapper.go`: domain ↔ proto cả hai chiều;
   - `options_json`, `suggested_default_json`, `answer_json` là chuỗi JSON chuẩn tắc
   - câu trả lời chỉ trả về cho người thuộc `assignees`, người hỏi, hoặc admin (người chỉ đọc Request thấy câu hỏi nhưng `answer_json` rỗng) để tránh lộ dữ liệu nhạy cảm.
4. `list_pending_clarifications.go`: `ListPendingClarifications.Execute(ctx, in{PageSize, PageToken})` lấy `UserID`, `Role`, danh sách team (`tenant-service.ListTeamsForUser` qua `TeamResolver` dùng chung với SOL-010) và gọi `ListPendingForUser` (task 028-03);
   - phân trang `(created_at, id)`
   - `PageSize` mặc định 50, tối đa 200.
5. `clarification_server.go`, `decision_server.go`: mỗi handler `tenant.RequireTenantID`;
   - kiểm quyền theo use case
   - lỗi qua `apperrors.ToGRPCStatus` (bảng mã: `REQUEST_CLARIFICATION_NOT_FOUND` NotFound, `_NOT_ASSIGNEE` PermissionDenied, `_INVALID_ANSWER` InvalidArgument, `_VERSION_CONFLICT` Aborted, `_EXPIRED`/`_NOT_OPEN`/`_ALREADY_ANSWERED`/`_INCOMPLETE`/`_STATE_NOT_ALLOWED` FailedPrecondition)
   - id lạ hoặc khác tenant đều `NOT_FOUND`.
6. Test hợp đồng trạng thái: một test đối chiếu `AllRequestStatuses()` (12 giá trị) với danh sách trong README v6 mục 3.3 (chuẩn bị cho khi README được cập nhật) và với enum `RequestStatus` của proto nếu có;
   - test fail kèm chỉ dẫn "cập nhật README v6".
7. `clarification_flow_integration_test.go` (tag `integration`, Postgres và MySQL, fake dev server và fake `GenerateSolution`): (a) Request `bug` thiếu dữ liệu: `ConfirmRequestType` → `awaiting_information` + một Clarification `open`;
   - (b) `AnswerClarification` nháp rồi hoàn tất → revision tăng, Request vào `analyzing`, consumer tạo đúng một run (giao lặp sự kiện hai lần)
   - (c) trả lời vẫn thiếu → `round=2`, sau 3 vòng về `request_backlog`
   - (d) hết hạn (fake clock) → `request_backlog` `missing_info`
   - (e) `type_change` từ `awaiting_information` hủy Clarification
   - (f) chọn phương án `breaking_change` → `chosen` → `Approve` bị chặn → `ConfirmDecision` → `Approve` qua
   - (g) hai `RequestClarification` đồng thời: một thắng.
8. README của service: mục "Clarification và Decision" (trạng thái `awaiting_information`, trigger, hạn mặc định đề xuất, biến môi trường `REQUEST_CLARIFICATION_MAX_ROUNDS`, `REQUEST_READINESS_AI_DRAFT`, `REQUEST_DECISION_HIGH_RISK_SERVICES`, `REQUEST_CLARIFICATION_WORKERS_ENABLED`, `REQUEST_CLARIFICATION_AUTO_REGENERATE`) và danh sách **chưa kiểm chứng**.
9. Báo người giữ CR-REQ-016/017/018: kênh WS mới, `RequestStatus` thêm `awaiting_information` (frontend), bảng loại trừ `parity_test.go` của MCP cho `decision_confirm` (không có tool).

## Kiểm thử

- `TestClarificationServer_Auth_NoTenant`
- `TestClarificationServer_CrossTenantNotFound` (từng RPC)
- `TestClarificationServer_RequestClarification_RejectsSystemOnlySources`
- `TestClarificationServer_Answer_MapsErrors` (bảng mã)
- `TestClarificationServer_ListPending_PaginatesAndFiltersByAssignee`
- `TestClarificationServer_AnswerJSONHiddenFromNonAssignee`.
- `TestDecisionServer_ConfirmDecision_MapsMismatchToFailedPrecondition`
- `TestDecisionServer_ListDecisions_OnlyReadableRequests`.
- `TestClarificationMapper_RoundTrip_AllKinds`
- `TestStatusList_MatchesREADME` (đọc `docs/crs/v6/README.md` mục 3.3 và so; hiện README liệt kê 11 nên test này **đỏ cho tới khi README được cập nhật**: đánh dấu `t.Skip` có điều kiện bằng biến `REQUEST_README_STATUS_CHECK=1` trong CI để không chặn các PR khác; ghi ở mâu thuẫn).
- Integration (a) đến (g) ở bước 7, hai dialect.
- Hợp đồng: `buf lint`, `buf breaking`.
- Lệnh: `cd /opt/repos/orca/backend-go && (cd proto && buf lint && buf breaking --against '../.git#branch=main,subdir=backend-go/proto') ; go test ./services/request-service/internal/adapter/grpc/... -run 'Clarification|Decision' && go test -tags=integration ./services/request-service/internal/usecase/... -run ClarificationFlow`. (Cú pháp `buf breaking` kiểm lại theo `proto/buf.yaml`.)

## Tiêu chí hoàn thành

- [ ] 11 RPC mới hoạt động, mỗi RPC có test chéo tenant trả `NOT_FOUND`.
- [ ] Bảy kịch bản tích hợp (a) đến (g) xanh trên Postgres và MySQL.
- [ ] `RequestClarification` qua RPC công khai không tạo được nguồn `readiness`/`task_blocked`.
- [ ] `answer_json` không lộ cho người không thuộc `assignees`/người hỏi/admin.
- [ ] `buf lint`, `buf breaking` sạch.
- [ ] README ghi biến môi trường và phần chưa kiểm chứng; mâu thuẫn "README v6 liệt kê 11 trạng thái" được ghi lại.

## Rủi ro và lưu ý

- README v6 mục 3.3 hiện 11 trạng thái; test đối chiếu sẽ đỏ tới khi người duyệt cập nhật README (CR mục 9). Dùng cờ điều kiện, không bỏ test.
- Proto thêm `RequestStatus`/trạng thái làm client cũ thấy giá trị lạ: đảm bảo frontend (CR-REQ-018) xử lý giá trị không biết.
- Kiểu `file` chỉ nhận văn bản ≤ 64 KB vì backend chưa có kho tải lên (chưa kiểm chứng rằng không có proto upload nào khác: `grep` toàn `backend-go/proto` trước khi chốt).
- Người trả lời qua MCP: chỉ khi người dùng cấp quyền theo chính sách `mcp-service`; chưa có cơ chế nhận biết nguồn máy.
