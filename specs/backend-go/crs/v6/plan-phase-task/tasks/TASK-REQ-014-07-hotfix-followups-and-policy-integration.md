# TASK-REQ-014-07: Request theo dõi sau hotfix (`OnCompleted`) và kiểm thử tích hợp chính sách

**From Solution:** BE-REQ-SOL-014
**Priority:** P2
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/hotfix_followups.go` (mới), `internal/domain/type_policy_hotfix.go` (sửa `OnCompleted`), `internal/usecase/type_policy_integration_test.go` (mới), `internal/usecase/hotfix_followups_test.go` (mới)
**Depends on:** TASK-REQ-014-03 đến 06, CR-REQ-006 (`SpawnChildRequest`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./... && go test -tags integration ./internal/adapter/{postgres,mysql,eventbus}` trong `request-service`)

---

## Context

- `OnCompleted` của `hotfix` trả hai `FollowUp`: Request `bug` ("Nguyên nhân gốc và test hồi quy cho <title>") và `task` ("Review sau hotfix: <title>"). `SpawnChildRequest` (CR-REQ-006) nhận `link_reason=followup_hotfix` với cha `hotfix` ở `executing` hoặc `completed`, `type_hint` `bug` hoặc `task`.
- Idempotency: `client_request_id = hotfix-followup:<request_id>:<bug|task>`; CR-REQ-006 dựng khoá idempotency từ `actor` (`user:<actor>`), gọi từ hệ thống chưa có người nên cần quy ước actor (`system:request-service`), chờ CR-REQ-006 xác nhận (SOL-014 mục 1, điều 3).
- Hai Request mới vào `classifying` và đi đủ phân loại, người xác nhận loại (CR-REQ-005).
- `OnCompleted` chạy sau `request.completed` (SOL-013 2.5 bước 4), có thể bị gọi lặp (at-least-once).

## Việc cần làm

1. `type_policy_hotfix.go`: `OnCompleted(req)` trả hai `FollowUp{TypeHint, Title, Body, LinkReason:"followup_hotfix", ClientRequestID}`; `Body` ghi liên kết về Request hotfix và tóm tắt Chẩn đoán nếu có.
2. `hotfix_followups.go`: `ExecuteFollowUps(ctx, req Request, fus []FollowUp) error` gọi `SpawnChildRequest` cho từng `FollowUp` với `actor_kind=system`; `created=false` (đã tồn tại) coi là thành công; lỗi tạm thời trả lỗi để consumer retry; lỗi vĩnh viễn (cha không hợp lệ) log và bỏ, không chặn hoàn tất Request.
3. Nối vào `ReportTaskOutcome` sau `request.completed` (qua cổng `FollowUpExecutor`); thất bại không làm Request mất trạng thái `completed`.
4. `type_policy_integration_test.go`: một bộ test chạy mỗi loại qua `PolicyFor` với fake ngoại vi: `hotfix` (cổng, hoàn tất, hai follow-up), `security` (pre_deploy, recheck), `performance` (baseline, đo lại đạt và không đạt), `refactor` (ba điều kiện), `ops_request` (cổng từng bước, rollback), và loại `bug`/`task`/`docs`/`change_request` chạy y như trước.
5. Test hợp đồng JSON `metrics` mẫu cho từng `kind` (file `testdata/request_check_*.json`).

## Kiểm thử

- `TestHotfixOnCompleted_TwoFollowUps_Idempotent` (chạy hai lần vẫn hai Request, hai dòng `request_links` `followup_hotfix`).
- `TestExecuteFollowUps_AlreadyExists_Success`, `_TransientError_Retried`, `_PermanentError_DoesNotBlockCompletion`.
- `TestPolicyFlows_AllTypes` (bảng 11 loại: 5 có chính sách, 6 noop).
- Integration hai dialect (khi CR-REQ-006 đã merge): `OnCompleted` idempotent qua `client_request_id` với DB thật.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'Hotfix|PolicyFlows|FollowUp' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] `hotfix` hoàn tất sinh đúng hai Request theo dõi (`bug`, `task`) có `request_links` `followup_hotfix`; chạy `OnCompleted` hai lần vẫn hai Request.
- [x] Loại khác chạy y như trước khi có CR này.
- [x] Mỗi kịch bản mục 4 của CR-REQ-014 có test.
- [x] Lỗi follow-up không làm Request `completed` bị thụt trạng thái.

## Rủi ro và lưu ý

- Quy ước actor hệ thống cho `SpawnChildRequest` chưa có; nếu CR-REQ-006 không chấp nhận, task này bị chặn ở bước 2 (ghi rõ trong PR).
- Hai Request theo dõi mặc định có thể thừa; chưa có dữ liệu dùng thực, cân nhắc cờ cấu hình `REQUEST_HOTFIX_FOLLOWUPS` (đề xuất, chưa trong CR).
- Test tích hợp đầy đủ cần `approval` và `lifecycle` đã merge; nếu chưa, dùng fake và ghi phần chưa chạy.

## Ghi chú triển khai

Lệch so với task và điểm chưa kiểm chứng: xem `IMPLEMENTATION-NOTES.md` mục "Đợt 3, phần request-service (exec)".
