# TASK-REQ-028-01: Migration `NNNN_clarifications_decisions` (12 trạng thái, 5 bảng) hai dialect

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service`
**File:** `migrations/postgres/NNNN_clarifications_decisions.{up,down}.sql`, `migrations/mysql/NNNN_clarifications_decisions.{up,down}.sql`, `internal/adapter/postgres/schema_contract_test.go` và `internal/adapter/mysql/schema_contract_test.go` (sửa, của TASK-REQ-002-06)
**Depends on:** TASK-REQ-002-01 (bảng `requests`), TASK-REQ-027-01 (số `NNNN` đi sau `NNNN_request_artifact_model`), TASK-REQ-009-01 (mẫu `pending_key`)
**Status:** [x] DONE

---

## Context

**Số migration không tự cấp.** Thư mục `request-service/migrations` chưa tồn tại lúc soạn; số của CR-REQ-001/002/004/006 chồng nhau (README `request-artifact-model`). Bước 1: `ls backend-go/services/request-service/migrations/postgres migrations/mysql`, lấy số kế tiếp lớn nhất, **cùng số hai dialect**, và thứ tự phải sau migration của TASK-REQ-027-01 vì cả hai đụng bảng `requests`.

Mẫu: BE-REQ-SOL-002 mục A (CHECK `status` inline, RLS `NULLIF(current_setting('app.tenant_id', true), '')::uuid` + `FORCE`; MySQL `CHAR(36)`, `TIMESTAMP(6)`, `VARCHAR(n)` cho cột trong khoá), BE-REQ-SOL-009 mục C (chỉ mục duy nhất một phần: Postgres `CREATE UNIQUE INDEX ... WHERE`; MySQL cột sinh `pending_key VARCHAR(120) GENERATED ALWAYS AS (IF(status='pending', CONCAT(subject_type,':',subject_id), NULL)) STORED` + `UNIQUE KEY`, vì NULL không va chạm). Đã đọc `task-service/migrations/mysql/0012_task_sources.up.sql` cho cách đặt tên ràng buộc.

Chỗ khó: đổi CHECK `requests.status` cần **tên** ràng buộc thật. Postgres đặt tên mặc định cho CHECK inline một cột là `<bảng>_<cột>_check`, nhưng nếu SOL-002 thực tế đặt tên khác thì `DROP CONSTRAINT` lỗi: lấy tên từ `SELECT conname FROM pg_constraint WHERE conrelid='request.requests'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%awaiting_type_confirmation%'`; MySQL `SELECT CONSTRAINT_NAME FROM information_schema.CHECK_CONSTRAINTS ...`.

## Việc cần làm

1. Chọn `NNNN`; ghi trong mô tả PR và thứ tự so với `NNNN_request_artifact_model`.
2. `requests.status`: Postgres `DROP CONSTRAINT <tên thật>` rồi `ADD CONSTRAINT requests_status_check CHECK (status IN (<12 giá trị, thêm 'awaiting_information'>))`;
   - MySQL `ALTER TABLE requests DROP CHECK <tên>` rồi `ADD CONSTRAINT requests_status_check CHECK (...)` (thực thi từ 8.0.16).
   - Ràng buộc `requests_backlog_stage` (`(status = 'request_backlog') = (returned_from_stage IS NOT NULL)`) giữ nguyên: `awaiting_information` không có `returned_from_stage`.
3. `clarifications`: cột và CHECK đúng SOL-028 mục C (`source`, `status`, `resume_status`, `round`, `asked_request_revision`, `answered_request_revision NULL`, `due_at NOT NULL`, `reminded_at NULL`, `cancel_reason`, `created_by`, `created_at`, `answered_at NULL`, `version`);
   - `UNIQUE (tenant_id, request_id, seq)`
   - chỉ mục `(tenant_id, status, due_at)` và `(tenant_id, request_id, created_at)`
   - **một `open` mỗi Request**: Postgres `CREATE UNIQUE INDEX clarifications_one_open ON request.clarifications (tenant_id, request_id) WHERE status='open'`
   - MySQL `open_key VARCHAR(80) GENERATED ALWAYS AS (IF(status='open', request_id, NULL)) STORED` + `UNIQUE KEY clarifications_one_open (tenant_id, open_key)`.
4. `clarification_questions`: `id`, `tenant_id`, `clarification_id`, `seq INT`, `question_key` (`VARCHAR(120)`), `kind` CHECK `text|single_choice|multi_choice|file|boolean`, `prompt` (`TEXT`, ≤ 1000 ở tầng ứng dụng), `reason` (`TEXT`, bắt buộc không rỗng bằng `CHECK (reason <> '')`), `options` JSON NULL, `suggested_default` JSON NULL, `required BOOLEAN`/`TINYINT(1)`, `target_path` NULL, `answer` JSON NULL, `answer_source` NULL CHECK `user|default_accepted`, `answered_by` NULL, `answered_at` NULL;
   - `UNIQUE (tenant_id, clarification_id, seq)`.
   - MySQL: ứng dụng luôn ghi giá trị JSON hoặc NULL tường minh (không `DEFAULT` literal trên JSON).
5. `clarification_assignees`: `clarification_id`, `tenant_id`, `principal_kind` CHECK `user|team|role|reporter`, `principal_id` (Postgres `TEXT NOT NULL DEFAULT ''`; MySQL `VARCHAR(64) NOT NULL DEFAULT ''`, `reporter` dùng chuỗi rỗng vì khoá chính không nhận NULL), PK `(clarification_id, principal_kind, principal_id)`;
   - chỉ mục `(tenant_id, principal_kind, principal_id)` cho `ListPendingClarificationsForUser`.
6. `decisions`: cột theo CR 2.8 (`seq`, `subject_kind` CHECK `solution_option|plan_assumption|other`, `subject_id`, `subject_digest`, `question`, `options` JSON, `recommended_option_id`, `recommendation_reason`, `chosen_option_id`, `chooser_id`, `chosen_at`, `rationale NOT NULL DEFAULT ''`, `risk_level` CHECK `normal|high`, `confirmed_by`, `confirmed_at`, `status` CHECK `open|chosen|effective|superseded`, `version`, `created_at`);
   - `UNIQUE (tenant_id, request_id, seq)`
   - một Decision "sống" mỗi chủ thể: Postgres `CREATE UNIQUE INDEX decisions_one_live ON request.decisions (tenant_id, subject_kind, subject_id) WHERE status IN ('open','chosen','effective')`
   - MySQL `live_key VARCHAR(120) GENERATED ALWAYS AS (IF(status IN ('open','chosen','effective'), CONCAT(subject_kind,':',subject_id), NULL)) STORED` + `UNIQUE KEY decisions_one_live (tenant_id, live_key)` (độ dài 20+1+64 < 120).
   - `decision_history`: `id`, `tenant_id`, `decision_id`, `action` CHECK `chosen|rechosen|confirmed|superseded`, `option_id`, `actor_id`, `rationale`, `at`
   - chỉ thêm.
7. RLS Postgres (`ENABLE`/`FORCE`, `tenant_isolation`) cho năm bảng mới; MySQL lọc `tenant_id` ở repository.
8. `down` cả hai dialect: `UPDATE requests SET status='request_backlog', returned_from_stage='task', return_reason='rollback_awaiting_information' WHERE status='awaiting_information'` (hoặc stage `classification` tuỳ nguồn; chọn một giá trị cố định và ghi lý do), khôi phục CHECK 11 giá trị, `DROP TABLE` ngược thứ tự.
   - Ghi comment "chỉ dùng khi rollback toàn bộ v6".
9. Mở rộng hai `schema_contract_test.go` (đọc `information_schema`): tên bảng, cột, NULL, và **tập giá trị** của CHECK `status` (đúng 12) so với hằng `domain.AllRequestStatuses()` (task 028-02).

## Kiểm thử

- `TestSchemaContract_Clarifications_Decisions` (hai DB): bảng, cột, kiểu, NULL, chỉ mục duy nhất.
- `TestRequestsStatusCheck_Accepts12Values_Rejects13th` (hai DB; MySQL chỉ có hiệu lực từ 8.0.16, test bỏ qua có chú thích nếu phiên bản thấp hơn).
- `TestClarifications_OneOpenPerRequest` (chèn hai `open` cho cùng Request: lỗi UNIQUE; chèn `open` sau khi cái cũ `answered`: thành công) cả hai dialect.
- `TestDecisions_OneLivePerSubject` (`open`
- `chosen`
- `effective` đều chặn bản thứ hai; `superseded` thì không).
- `TestClarificationAssignees_ReporterEmptyPrincipalKey`.
- `TestMigration_UpDownUp_BothDialects` và `TestDown_MovesAwaitingInformationToBacklog`.
- `TestRLS_ClarificationTables_NoBypassRole` (Postgres, role `NOSUPERUSER NOBYPASSRLS`).
- Lệnh: `cd /opt/repos/orca/backend-go && go test -tags=integration ./services/request-service/internal/adapter/... -run 'SchemaContract|StatusCheck|OneOpen|OneLive|Migration|Down|RLS'` (cần Postgres 14+ và MySQL 8.0.16+).

## Tiêu chí hoàn thành

- [x] `up`/`down`/`up` sạch ở hai dialect, `NNNN` và thứ tự ghi trong PR.
- [x] CHECK `status` có đúng 12 giá trị và khớp hằng Go.
- [x] Hai Clarification `open` cho một Request bị từ chối ở cả hai DB; Decision sống duy nhất mỗi chủ thể.
- [x] `down` chuyển Request `awaiting_information` về `request_backlog` trước khi khôi phục CHECK.
- [x] Postgres có RLS thật cho năm bảng.

## Rủi ro và lưu ý

- `DROP CHECK` MySQL không có DDL transaction; sai tên giữa chừng để DB ở trạng thái nửa vời: thử trên DB trống, và viết `down` thủ công trong mô tả PR.
- MySQL cũ hơn 8.0.16 phân tích rồi bỏ qua CHECK: `awaiting_information` và các CHECK mới không có tác dụng (rủi ro đã nêu ở SOL-002).
- Hai migration (027-01, 028-01) cùng đổi bảng `requests`; nếu hai nhánh cùng lấy một `NNNN`, hợp nhất một file.
- `down` làm mất Clarification và Decision (xoá bảng); chấp nhận cho rollback toàn bộ.
