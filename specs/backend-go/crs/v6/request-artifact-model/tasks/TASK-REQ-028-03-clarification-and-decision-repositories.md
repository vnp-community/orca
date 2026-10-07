# TASK-REQ-028-03: Repository `Clarification`, `Decision` và `TransitionRequest` hỗ trợ `ResumeStatus`

**From Solution:** BE-REQ-SOL-028
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/ports.go` (sửa), `internal/adapter/postgres/clarification_repository.go`, `internal/adapter/postgres/decision_repository.go`, `internal/adapter/mysql/clarification_repository.go`, `internal/adapter/mysql/decision_repository.go`, `internal/usecase/transition_request.go` (sửa), `internal/usecase/status_write_guard_test.go` (sửa) và test
**Depends on:** TASK-REQ-028-01, TASK-REQ-028-02, TASK-REQ-003-03 (`TransitionRequest`), TASK-REQ-001-04 (executor trong ctx), TASK-REQ-002-04/-05 (mẫu repository)
**Status:** [x] DONE

---

## Context

Mẫu: BE-REQ-SOL-002 mục C và D (`RequestRepository` với CAS `version`, executor trong ctx, hành vi từng dialect: Postgres `ON CONFLICT`/`RETURNING`, MySQL `INSERT IGNORE`/`ON DUPLICATE KEY` và đọc lại; JSON luôn ghi giá trị tường minh), BE-REQ-SOL-009 mục D (chữ ký repository có `tx Tx`, nhận biết trùng khoá: Postgres SQLSTATE `23505` kèm tên chỉ mục, MySQL lỗi `1062` kèm tên khoá, bọc thành lỗi domain). `task-service/internal/adapter/postgres/execution_leases.go` và bản MySQL là mẫu `FOR UPDATE SKIP LOCKED` hai dialect cho vòng quét hết hạn (SOL-007 `ClaimExpiredRuns` dùng cùng cách).

Hợp đồng hai repository dùng chung một bộ test chạy cho cả hai adapter (`*_contract_test.go`), như TASK-REQ-002-06. Chỉ mục duy nhất `clarifications_one_open` và `decisions_one_live` (task 028-01) làm điểm chặn cuối cùng cho đua ghi: adapter nhận biết lỗi trùng và trả lỗi domain (`REQUEST_CLARIFICATION_STATE_NOT_ALLOWED` cho "đã có Clarification mở"; `REQUEST_DECISION_...` cho Decision sống).

## Việc cần làm

1. `ports.go`: `ClarificationRepository{Insert(ctx, c domain.Clarification) error` (kèm câu hỏi và `assignees` trong cùng giao dịch của caller);
   - `Get(ctx, id string) (domain.Clarification, error)`
   - `GetOpenByRequest(ctx, requestID string) (*domain.Clarification, error)`
   - `ListByRequest(ctx, requestID string) ([]domain.Clarification, error)`
   - `UpdateAnswers(ctx, id string, answers []AnswerRecord, expectedVersion int64) error` (lưu nháp: cập nhật `answer`, `answer_source`, `answered_by`, `answered_at` của từng câu)
   - `MarkAnswered(ctx, id string, answeredRevision int, at time.Time, expectedVersion int64) error`
   - `MarkCancelled(ctx, id, reason string, at time.Time, expectedVersion int64) error`
   - `ClaimExpired(ctx, now time.Time, batch int) ([]domain.Clarification, error)` (đặt `expired` và trả các dòng, đồng hồ DB cho `now` nếu repository có `NowDB`)
   - `ClaimNeedingReminder(ctx, now time.Time, batch int) ([]domain.Clarification, error)` (đặt `reminded_at`, điều kiện `reminded_at IS NULL AND now >= created_at + (due_at - created_at)/2`)
   - `ListPendingForUser(ctx, f PendingFilter) ([]domain.Clarification, string, error)` với `PendingFilter{UserID string; Teams []string; IsAdmin bool; PageSize int; PageToken string}`
   - `NextSeq(ctx, requestID string) (int, error)}`.
2. `DecisionRepository{Insert(ctx, d domain.Decision) error; GetLiveBySubject(ctx, kind domain.DecisionSubjectKind, subjectID string) (*domain.Decision, error); Get(ctx, id string) (domain.Decision, error); Update(ctx, d domain.Decision, expectedVersion int64) (domain.Decision, error); AppendHistory(ctx, h domain.DecisionHistory) error; ListByRequest(ctx, requestID string) ([]domain.Decision, error); SupersedeLiveBySubject(ctx, kind, subjectID string) (int, error); SupersedeLiveByRequest(ctx, requestID string) (int, error)}`.
3. Postgres `clarification_repository.go`: `Insert` bắt `23505` với tên `clarifications_one_open` thành `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED` (chi tiết "đã có Clarification mở"), tên `clarifications_tenant_id_request_id_seq_key` thành lỗi `seq` (retry bởi caller).
   - `ClaimExpired`: `UPDATE request.clarifications SET status='expired' WHERE id IN (SELECT id FROM request.clarifications WHERE tenant_id=$1 AND status='open' AND due_at <= now() ORDER BY due_at LIMIT $2 FOR UPDATE SKIP LOCKED) RETURNING ...`.
   - MySQL: `SELECT ... FOR UPDATE SKIP LOCKED` trong transaction (cần MySQL ≥ 8.0.1, `dbcapability` kiểm), rồi `UPDATE ... WHERE id IN (...)`
   - nhận biết `1062` với tên khoá `clarifications_one_open`.
   - Vòng quét **không** lọc theo một tenant duy nhất: quét liên tenant cần quyền đọc mọi tenant
   - dùng đường truy vấn riêng không đặt `app.tenant_id` (service-level, bỏ RLS) hoặc lặp theo danh sách tenant
   - chọn và ghi rõ trong comment (xem rủi ro).
4. `ListPendingForUser`: một SQL không dùng toán tử JSON: `SELECT c.* FROM clarifications c WHERE c.tenant_id=? AND c.status='open' AND ( EXISTS (SELECT 1 FROM clarification_assignees a WHERE a.clarification_id=c.id AND ((a.principal_kind='user' AND a.principal_id=?) OR (a.principal_kind='team' AND a.principal_id IN (?...)) OR (a.principal_kind='role' AND a.principal_id=?)) ) OR (EXISTS (SELECT 1 FROM clarification_assignees a2 WHERE a2.clarification_id=c.id AND a2.principal_kind='reporter') AND EXISTS (SELECT 1 FROM requests r WHERE r.id=c.request_id AND r.reporter_id=?)) OR ? /*admin*/ )` phân trang keyset `(created_at, id)`.
   - Cùng kết quả hai DB (test).
5. Decision adapter: `GetLiveBySubject` lọc `status IN ('open','chosen','effective')`;
   - `Update` CAS theo `version` (0 hàng thì `SELECT` phân biệt `NOT_FOUND`/`VERSION_CONFLICT`)
   - `SupersedeLiveBy*` là `UPDATE ... SET status='superseded' WHERE status IN (...)` trả số dòng, ghi `decision_history` từng dòng bằng caller (không trong repository).
6. `transition_request.go` (sửa): `TransitionInput` thêm `ResumeStatus domain.RequestStatus` (và `Category` nếu SOL-006 đã thêm);
   - `once` gọi `NextStatusWithResume`
   - `information_provided` mà `ResumeStatus` sai thì `REQUEST_RESUME_STATUS_INVALID`
   - payload `status_changed` thêm `resume_status` (cộng thêm, theo quy ước additive của `common/eventbus.Event`).
   - Cập nhật `status_write_guard_test.go` nếu cần (không có trường mới được gán ở nơi khác).
7. Hai `TxRunner` fake trong test usecase thêm hai repository mới.

## Kiểm thử

- `TestClarificationRepository_Contract` (một bộ kịch bản, hai adapter): `Insert` kèm câu hỏi và assignee; `GetOpenByRequest`; chèn Clarification `open` thứ hai cùng Request bị `STATE_NOT_ALLOWED`; `UpdateAnswers` nháp rồi `MarkAnswered` đúng `expectedVersion`, sai `version` thì `VERSION_CONFLICT`; `NextSeq` tăng; tenant A không đọc dòng tenant B.
- `TestClarificationRepository_ClaimExpired_TwoWorkersNoOverlap` (hai goroutine, mỗi bên claim, tổng khác nhau và hợp bằng tập hết hạn)
- `TestClarificationRepository_ClaimNeedingReminder_OncePerClarification`.
- `TestListPendingForUser_SameResultBothDialects` (user, team, role, reporter, admin; kiểm không trả dòng của Request người khác; phân trang ổn định).
- `TestDecisionRepository_Contract`: `Insert` Decision thứ hai cùng chủ thể khi cái đầu `effective` bị chặn; sau `Supersede` thì chèn được; `Update` CAS; `AppendHistory` chỉ thêm.
- `TestTransitionRequest_InformationRequired_And_Provided` (fake repo): `ResumeStatus` hợp lệ và không hợp lệ, payload có `resume_status`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'TransitionRequest' && go test -tags=integration ./services/request-service/internal/adapter/... -run 'ClarificationRepository|DecisionRepository|ListPendingForUser'`.

## Tiêu chí hoàn thành

- [x] Cùng bộ kịch bản chạy xanh trên Postgres và MySQL cho cả hai repository.
- [x] Đua hai `Insert` Clarification `open`: đúng một thành công (cả hai DB).
- [x] `ClaimExpired` hai worker không xử lý trùng.
- [x] `ListPendingForUser` cho cùng kết quả hai DB, không dùng toán tử JSON.
- [x] `TransitionRequest` nhận `ResumeStatus`; test kiến trúc ghi `status` vẫn xanh.

## Rủi ro và lưu ý

- Vòng quét hết hạn là tác vụ liên tenant: với Postgres RLS (`FORCE`) cần cách tắt policy cho đường quét (role riêng hoặc `SET LOCAL app.tenant_id` theo từng tenant); cách làm của `RecoverInterruptedAnalysisRuns` (SOL-007) cũng gặp vấn đề này, thống nhất một cách ở một chỗ và dùng lại, không mỗi nơi một kiểu.
- MySQL không có `RETURNING`, nên `ClaimExpired` hai bước; trong transaction REPEATABLE READ phải dùng `SKIP LOCKED` đúng để không đọc snapshot cũ.
- `ListPendingForUser` với danh sách team dài làm `IN (...)` lớn; giới hạn 100 team và ghi lỗi nếu vượt.
- Tên chỉ mục trong lỗi trùng khoá phải khớp migration 028-01; test hợp đồng bắt lệch tên.
