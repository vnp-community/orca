# CR-REQ-028: Hỏi lại (Clarification), trạng thái `awaiting_information` và ghi nhận quyết định (Decision)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-028 |
| **Tên** | Thực thể Clarification có câu hỏi có kiểu; trạng thái mới `awaiting_information`; kiểm tra sẵn sàng của Request (Definition of Ready); câu trả lời tạo revision mới của Request; thực thể Decision, xác nhận lần hai cho lựa chọn rủi ro cao |
| **Loại** | Feature (lõi miền, mở rộng máy trạng thái) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-002, 003 (máy trạng thái), 005 (xác nhận loại), 006 (trả backlog), 007 (chọn phương án), 009 (Approval), 010 (người duyệt, thông báo), 027 (`ValidateRequestContent`, `request_revisions`) |
| **Mở khoá** | CR-REQ-029 (`needs_info` từ cổng sẵn sàng thực thi), UI hỏi đáp (CR-REQ-019, 022) |
| **Tác động** | `request-service` (domain, usecase, adapter postgres/mysql, migration, proto, consumer outbox); `notification-service` (hai hàng cấu hình, như CR-REQ-010); sửa CR-REQ-002, 003, 005, 006, 007, 009, 010, 012, 013 (mục 9) |

## 1. Bối cảnh và vấn đề

1. Chưa có trạng thái hay thực thể nào cho việc hỏi lại. README v6 mục 3.3 chỉ có `request_backlog` với `returned_category=missing_info` (CR-REQ-006): dừng hẳn, không có vòng hỏi đáp, người báo cáo không biết cần bổ sung gì.
2. Request từ Jira và GitHub thường thiếu thông tin (không có bước tái hiện, không có tiêu chí chấp nhận). CR-REQ-027 đã định nghĩa trường bắt buộc theo loại và hàm `ValidateRequestContent(level=ready)`, nhưng chưa có nơi dùng.
3. Nhiều chỗ trong luồng phát sinh câu hỏi: `open_questions` của Solution (CR-REQ-007), `assumptions` của Plan (CR-REQ-012), task thiếu dữ liệu khi thực thi (CR-REQ-013, 029). Hiện không có cơ chế chung; người dùng phải sinh lại hoặc trả backlog.
4. `Approval` chỉ có duyệt hoặc từ chối một đối tượng. Phương án được chọn nằm ở `solutions.chosen_option`, không ghi ai chọn, vì sao chọn khác đề xuất, chọn lại bao nhiêu lần, và không có bước xác nhận cho lựa chọn rủi ro cao (`breaking_change`).
5. `notification-service` chỉ gửi cho người có tên trong payload; không biết team hay vai trò (CR-REQ-010 mục 1). Mô hình quyền hiện chỉ có `admin|user` và team.

CR này sở hữu: bảng và vòng đời Clarification, Decision; các trigger vào và ra `awaiting_information`; kiểm tra sẵn sàng; hết hạn. Không sở hữu: định nghĩa trường bắt buộc và `request_revisions` (CR-REQ-027); bảng `approvals` (CR-REQ-009); hợp đồng `TaskSpec` và cổng sẵn sàng thực thi (CR-REQ-029).

## 2. Giải pháp đề xuất

### 2.1 Trạng thái và trigger (sửa bảng chuyển của CR-REQ-003)

Request có 12 trạng thái: thêm `awaiting_information`. `requests.status` CHECK thêm giá trị này (cả hai dialect, mục 2.7).

| Trigger | Từ | Đến | Điều kiện |
|---|---|---|---|
| `information_required` | `awaiting_type_confirmation`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing` | `awaiting_information` | tạo Clarification `open` trong cùng transaction; mang `resume_status`; hủy Approval `pending` của Request (`CancelPendingForRequest`, lý do `information_required`) |
| `information_provided` | `awaiting_information` | `resume_status` của Clarification (`analyzing`, `planning` hoặc `executing`) | Clarification chuyển `answered`, revision mới đã ghi; `TransitionInput.ResumeStatus` bắt buộc và phải thuộc ba giá trị trên |
| `return_to_backlog` | thêm `awaiting_information` vào danh sách nguồn | `request_backlog` | `category=missing_info` khi hết hạn |
| `type_change` | thêm `awaiting_information` vào danh sách nguồn | `awaiting_type_confirmation` | Clarification `open` thành `cancelled` (lý do `type_changed`) |
| `cancel` | thêm `awaiting_information` | `cancelled` | Clarification `open` thành `cancelled` |

`awaiting_information` không có trigger nào khác; `completed` và `cancelled` vẫn là trạng thái cuối. Mọi bất biến của CR-REQ-003 giữ nguyên: chỉ `TransitionRequest` ghi `status`, chuyển trạng thái và outbox cùng transaction, `ExpectedFrom` làm giao lặp thành công.

`resume_status` do registry quyết định lúc tạo Clarification (hàm `ResumeStatusFor(flow, source)`):

| Nguồn (`source`) | Phát sinh từ trạng thái | `resume_status` | Cách kích hoạt |
|---|---|---|---|
| `readiness` | `awaiting_type_confirmation` (lúc `ConfirmRequestType`) | trạng thái đầu của luồng: `analyzing` nếu `Analysis.Kind != none`, ngược lại `planning` | tự động, 2.3 |
| `solution_open_question` | `awaiting_analysis_approval` hoặc `analyzing` | `analyzing` | người duyệt hoặc admin gọi `RequestClarification`; `Approve` bị chặn khi còn `blocking` chưa giải (2.5) |
| `plan_assumption` | `awaiting_plan_approval` hoặc `planning` | `planning` | như trên với giả định `needs_confirmation` |
| `task_blocked` | `executing` | `executing` | nội bộ, do CR-REQ-013/029 gọi (2.6) |
| `manual` | bất kỳ trạng thái hợp lệ ở bảng trên | theo trạng thái nguồn | admin hoặc người duyệt hỏi tay |

Trường hợp `awaiting_type_confirmation`: `ConfirmRequestType` vẫn ghi `type`, `size`, `urgency` và phát `orca.request.request.type_confirmed` (CR-REQ-005), nhưng dùng trigger `information_required` thay cho `type_confirmed` khi kiểm tra sẵn sàng không đạt. Người dùng không phải xác nhận loại lần hai sau khi trả lời.

Đích khi hết hạn: `returned_from_stage` là `classification` cho nguồn `readiness`; `analysis` cho `resume_status=analyzing`; `plan` cho `planning`; `task` cho `executing` (giá trị cho phép của cột theo CR-REQ-002).

### 2.2 Bảng dữ liệu (migration `request-service`, hai dialect; số do CR-REQ-002 cấp)

**`clarifications`**: `id`, `tenant_id`, `request_id`, `seq INT` (hiển thị `CLR-<reqnum>.<seq>`, UNIQUE `(tenant_id, request_id, seq)`), `source` (CHECK `readiness|solution_open_question|plan_assumption|task_blocked|manual`), `source_ref` (id Solution, id task Plan hoặc id Task; không FK), `status` (CHECK `open|answered|expired|cancelled`), `resume_status` (CHECK `analyzing|planning|executing`), `round INT NOT NULL DEFAULT 1`, `asked_request_revision INT`, `answered_request_revision INT NULL`, `due_at` (`TIMESTAMPTZ` / `TIMESTAMP(6)`), `reminded_at` NULL, `cancel_reason` (TEXT, rỗng nếu không), `created_by` (id người hoặc `system`), `created_at`, `answered_at` NULL, `version BIGINT`. Chỉ mục `(tenant_id, status, due_at)` (quét hết hạn), `(tenant_id, request_id, created_at)`.

Một Clarification `open` mỗi Request: Postgres `CREATE UNIQUE INDEX ... (tenant_id, request_id) WHERE status='open'`; MySQL cột sinh `open_key VARCHAR(80) GENERATED ALWAYS AS (IF(status='open', request_id, NULL)) STORED` kèm `UNIQUE KEY (tenant_id, open_key)` (cùng thủ thuật `pending_key` của CR-REQ-009; NULL không va chạm).

**`clarification_questions`**: `id`, `tenant_id`, `clarification_id`, `seq INT`, `question_key` (khóa ngữ nghĩa, ví dụ `type_fields.severity`, `acceptance_criteria`, `open_question:Q-1`), `kind` (CHECK `text|single_choice|multi_choice|file|boolean`), `prompt` (TEXT, tối đa 1000), `reason` (TEXT, lý do hỏi, bắt buộc, tối đa 500), `options` JSON NULL (`[{id,label}]`, bắt buộc cho hai kiểu chọn), `suggested_default` JSON NULL, `required BOOL`, `target_path` NULL (đường dẫn trong nội dung Request nơi câu trả lời được áp, ví dụ `type_fields.severity`), `answer` JSON NULL, `answer_source` NULL (`user|default_accepted`), `answered_by` NULL, `answered_at` NULL. UNIQUE `(tenant_id, clarification_id, seq)`.

**`clarification_assignees`** (đối tượng trả lời, dạng bảng vì `ListPendingClarificationsForUser` cần join portable giữa hai DB, cùng lý do `approval_approvers` của CR-REQ-010): `clarification_id`, `tenant_id`, `principal_kind` (`user|team|role|reporter`), `principal_id`; khóa chính `(clarification_id, principal_kind, principal_id)`.

**`decisions`** và **`decision_history`**: mục 2.8.

### 2.3 Kiểm tra sẵn sàng (Definition of Ready)

Chạy trong `ConfirmRequestType` (CR-REQ-005), sau khi người dùng xác nhận loại và trước khi chuyển trạng thái, cùng transaction và CAS `version`:

1. `ReadinessPolicy.Evaluate(type, content)` (`internal/domain/readiness_policy.go`, mới) gọi `ValidateRequestContent(type, content, level=ready)` của CR-REQ-027 và trả `ReadinessReport{ready bool, missing[]{path, rule, blocking}}`. Khóa bắt buộc của loại và AC là `blocking`; khóa khuyến nghị (ví dụ `scope_out`) là `blocking=false`, không tạo câu hỏi.
2. `ready` thì đi tiếp bằng `type_confirmed`. Không thì `QuestionBuilder.Build(report, hints)` dựng danh sách câu hỏi từ bảng mẫu theo `path` (văn bản cho `repro_steps`, chọn một cho `severity` và `exploitability`, có/không cho các cờ, văn bản có `suggested_default` cho `acceptance_criteria`), rồi `information_required` với `source=readiness`, người trả lời `reporter` (mặc định), `due_at` theo 2.9.
3. Mặc định đề xuất chỉ lấy từ dữ liệu có sẵn (gợi ý nguồn `source_hints` của CR-REQ-004, `urgency`, `size`). Với AC, một lượt `ai.complete` tùy chọn (cờ `REQUEST_READINESS_AI_DRAFT`, mặc định tắt) có thể soạn nháp AC đặt vào `suggested_default`; lỗi thì bỏ qua, không chặn. Cùng ranh giới tin cậy như CR-REQ-007 (nội dung Request nằm trong khối dữ liệu có rào).
4. Bỏ qua kiểm tra (`WaiveReadiness`): chỉ `role=admin`, `reason` bắt buộc, không áp dụng cho `hotfix`, `security`, `ops_request` (dữ liệu của ba loại này liên quan an toàn). Ghi vào `request_revisions` (`cause=edited`, ghi chú waiver) và sự kiện; mã `REQUEST_READINESS_WAIVE_FORBIDDEN` cho trường hợp bị cấm.
5. Giới hạn vòng: sau câu trả lời, kiểm tra lại; vẫn thiếu thì tạo Clarification mới (`round + 1`) trong cùng transaction. Tối đa `REQUEST_CLARIFICATION_MAX_ROUNDS` (mặc định đề xuất 3); vượt thì `return_to_backlog` (`missing_info`, `reason` liệt kê khóa còn thiếu). Mục đích: tránh vòng hỏi vô hạn.

### 2.4 Trả lời và revision mới của Request

`AnswerClarification(clarification_id, answers[{question_id, value | accept_default}], complete, expected_version)`:

1. `tenant.RequireTenantID`; Clarification `open`; người gọi thuộc `clarification_assignees` hoặc `role=admin` (`REQUEST_CLARIFICATION_NOT_ASSIGNEE`); không phải danh tính máy (cơ chế nhận biết nguồn máy chưa có, như CR-REQ-010 mục 2.4 bước 2; tool MCP chỉ làm được khi người dùng cấp quyền theo chính sách `mcp-service`).
2. Kiểm kiểu câu trả lời theo `kind`: `single_choice` thuộc `options`; `multi_choice` tập con; `boolean`; `text` tối đa 4000 ký tự; `file`: chưa có kho tải lên nào trong backend (không có proto upload), nên v1 chỉ nhận `{filename, mime, size, text}` với `text` là nội dung văn bản tối đa 64 KB; tệp nhị phân ngoài phạm vi (Q2). Câu bắt buộc phải có giá trị hoặc `accept_default` khi có `suggested_default`.
3. `complete=false`: lưu nháp (cập nhật `answer`, giữ `open`), không đổi Request. `complete=true` mà thiếu câu bắt buộc: `REQUEST_CLARIFICATION_INCOMPLETE`.
4. `complete=true`, một transaction: dựng nội dung mới bằng `ApplyAnswers(content, questions)` (thuần, theo `target_path`); `AppendRequestRevision(cause=clarification_answered, clarification_id)` (CR-REQ-027, CAS `version`); Clarification `answered`, `answered_request_revision`; kiểm tra sẵn sàng lại (2.3 bước 5); các Solution `proposed` và Plan sinh từ revision cũ chuyển `superseded` (nếu `resume_status` là `analyzing` hoặc `planning`), Decision tương ứng thành `superseded` (2.8); `TransitionRequest(information_provided, ExpectedFrom=awaiting_information, ResumeStatus=...)`; outbox `orca.request.clarification.answered`.
5. Kích hoạt lại bước đang chờ: consumer durable của `request-service` đọc `status_changed` có `trigger=information_provided`, dedup bằng `processed_events`. `to=analyzing` thì tự gọi `GenerateSolution` nội bộ với `feedback="clarification:<id>"` (CR-REQ-007 đã có đường sinh lại bằng phản hồi); `to=planning` thì không tự chạy (PROPOSE của CR-REQ-012 không lưu gì, người dùng bấm lại ở UI); `to=executing` thì gọi `AdvanceExecution` (CR-REQ-013). Tự chạy hay để người bấm là Q3.
6. Giao lặp: `AnswerClarification` lặp cùng nội dung trên Clarification đã `answered` trả kết quả hiện có; nội dung khác thì `REQUEST_CLARIFICATION_ALREADY_ANSWERED`.

### 2.5 Nguồn câu hỏi từ Solution và Plan

CR-REQ-027 đổi `open_questions` thành `{id, text, blocking}` và `assumptions` thành `{id, text, needs_confirmation}`. Quy tắc:
- Không tự động hỏi từ các mục này: người duyệt thấy chúng ở UI và chọn "Hỏi lại" (gọi `RequestClarification(source=solution_open_question|plan_assumption, items[])`, mỗi mục thành một câu hỏi `text` hoặc `boolean`).
- Chặn duyệt: `SubjectHandler.ValidateForRequest` của `solution` (CR-REQ-007) và `plan` (CR-REQ-012) trả `REQUEST_SOLUTION_BLOCKING_QUESTIONS` / `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS` khi còn mục `blocking` hoặc `needs_confirmation` chưa có câu trả lời ghi nhận (một Clarification `answered` có `source_ref` là Solution hay Plan đó, tham chiếu `question_key`). Người duyệt thấy rõ lý do; không bị chặn câu chữ im lặng.
- Hỏi từ `analyzing` (run chạy xong chưa sang `awaiting_analysis_approval`) dùng cùng trigger, `resume_status=analyzing`.

### 2.6 Nguồn `task_blocked` (chuẩn bị cho CR-REQ-029)

RPC nội bộ `RequestClarification(source=task_blocked, source_ref=task_id, questions[])` do CR-REQ-013 (khi consumer `ReportTaskOutcome` phân loại `needs_info`) hoặc cổng sẵn sàng của CR-REQ-029 gọi, chỉ khi Request đang `executing`. Hiệu ứng: Request sang `awaiting_information`; `AdvanceExecution` không phát task mới khi Request không còn `executing` (đúng hành vi CR-REQ-013 mục 2.5 bước 2: chỉ ghi outcome); task đang chạy kết thúc bình thường. Khi trả lời xong, `information_provided` về `executing`, gọi `AdvanceExecution`. Việc đưa câu trả lời vào prompt của task thuộc CR-REQ-029 (`ExecutionPacket`); CR này chỉ cung cấp thực thể, RPC và kiểm thử với bên gọi giả. Chưa kiểm chứng ở CR này: trạng thái task bị chặn quay về `open` (previous_status, theo CR-REQ-013 mục 1) có đủ để chạy lại sạch hay không.

### 2.7 Hai dialect, hết hạn, nhắc, thông báo

- `requests.status` CHECK 12 giá trị: Postgres `ALTER TABLE ... DROP CONSTRAINT ...; ADD CONSTRAINT ... CHECK (...)`; MySQL `DROP CHECK` rồi `ADD CONSTRAINT` (thực thi từ 8.0.16; cùng cách `0015` của CR-REQ-011 làm với `task_type`). Down: chuyển `awaiting_information` về `request_backlog` (`missing_info`) trước khi khôi phục CHECK cũ; ghi chú trong migration là chỉ dùng khi rollback toàn bộ v6.
- Hết hạn: vòng quét 60 giây `ExpireClarifications`, chiếm dòng `open` quá `due_at` bằng `FOR UPDATE SKIP LOCKED` (Postgres `UPDATE ... RETURNING`; MySQL transaction `SELECT ... FOR UPDATE SKIP LOCKED` rồi `UPDATE`, cần MySQL 8.0.1+ theo CR-DB-002), đặt `expired`, rồi gọi `ReturnToBacklog(stage, category=missing_info, reason="clarification_expired")` (CR-REQ-006), `actor_kind=system`. Hết hạn ghi lười như `EffectiveStatus` của Approval: `AnswerClarification` quá hạn nhưng chưa quét trả `REQUEST_CLARIFICATION_EXPIRED`.
- Nhắc: một lần ở 50% thời hạn (`reminded_at`), qua cùng cơ chế thông báo của CR-REQ-010.
- `notification-service`: thêm vào `Subjects` của `internal/adapter/eventbus/consumer.go` hai ràng buộc stream `REQUEST` cho `orca.request.clarification.requested` và `orca.request.clarification.expired`, và hai hàng `subjectRules` ở `internal/domain/notification_event.go` (`request.clarification_requested` mức `warning`, kênh ws và push; `request.clarification_expired` mức `warning`). Payload do `request-service` đặt: `user_ids` (mở rộng `reporter`, `team:` bằng `tenant-service.ListTeamMembers`, `role:admin` như CR-REQ-010 mục 2.6), `title`, `body` ngắn (không chứa nội dung câu hỏi hay câu trả lời, vì thông báo được lưu), `deep_link`.

### 2.8 Decision

Một Decision là bản ghi một lựa chọn có xác nhận, dùng chung cho mọi loại lựa chọn.

**`decisions`**: `id`, `tenant_id`, `request_id`, `seq INT` (hiển thị `DEC-<reqnum>.<seq>`), `subject_kind` (CHECK `solution_option|plan_assumption|other`), `subject_id`, `subject_digest` (`TEXT` / `CHAR(64)`; với Solution là digest mới nhất của Approval `pending`, đồng bộ qua `UpdatePendingDigest` của CR-REQ-009), `question` TEXT, `options` JSON (`[{id,label,summary,risk:{level,reasons[]}}]`), `recommended_option_id` NULL, `recommendation_reason` TEXT, `chosen_option_id` NULL, `chooser_id` NULL, `chosen_at` NULL, `rationale` TEXT NOT NULL DEFAULT '', `risk_level` (CHECK `normal|high`), `confirmed_by` NULL, `confirmed_at` NULL, `status` (CHECK `open|chosen|effective|superseded`), `version`, `created_at`. Một Decision `open|chosen|effective` mỗi `(tenant_id, subject_kind, subject_id)` (chỉ mục duy nhất một phần, như 2.2). **`decision_history`**: chỉ thêm, `decision_id`, `action` (`chosen|rechosen|confirmed|superseded`), `option_id`, `actor_id`, `rationale`, `at`; ghi mỗi lần chọn và đổi.

Quy tắc, thi hành trong `usecase/record_decision.go` và `confirm_decision.go` (mới):
1. **Tạo và chọn.** `ChooseSolutionOption` (CR-REQ-007) gọi `RecordDecision` trong cùng transaction: tạo Decision (`subject_kind=solution_option`, `options` lấy từ Solution) nếu chưa có, rồi ghi `chosen_option_id`, `chooser_id`, `subject_digest` mới.
2. **Lý do bắt buộc khi khác đề xuất.** `chosen_option_id != recommended_option_id` thì `rationale` không rỗng, ngược lại `REQUEST_DECISION_RATIONALE_REQUIRED` (InvalidArgument).
3. **Rủi ro cao.** `DecisionRisk.Assess(option)` (thuần): `high` nếu `breaking_change=true`, hoặc có `risks[].severity=high`, hoặc số `affected_areas` kind `service` đạt `REQUEST_DECISION_HIGH_RISK_SERVICES` (mặc định đề xuất 3). Phương án `normal`: `status=effective` ngay khi chọn. Phương án `high`: `status=chosen`, chưa có hiệu lực.
4. **Xác nhận lần hai.** `ConfirmDecision(decision_id, confirmation_text, expected_version)`: `confirmation_text` phải trùng tiêu đề phương án sau chuẩn hóa NFC, cắt khoảng trắng, không phân biệt hoa thường (`REQUEST_DECISION_CONFIRMATION_MISMATCH`); người xác nhận là chính `chooser_id` hoặc `role=admin`; danh tính máy bị từ chối (`REQUEST_DECISION_AGENT_FORBIDDEN`; tool MCP `decision_confirm` không đăng ký, cùng lý do CR-REQ-010). Thành công: `effective`, `confirmed_by`, `confirmed_at`. Chọn lại (`ChooseSolutionOption` với phương án khác khi Approval còn `pending`) đưa Decision về `open` hoặc `chosen` mới và xóa xác nhận cũ.
5. **Chặn duyệt.** `SubjectHandler.ValidateForRequest` của `solution` (`kind=solution`) và `plan`/`task_list` (CR-REQ-012) yêu cầu Decision của Solution là `effective` và `subject_digest` khớp: `REQUEST_DECISION_NOT_EFFECTIVE` (FailedPrecondition). `diagnosis`, `findings`, `answer` không có phương án nên không cần Decision.
6. **Quyền chọn.** Người gọi phải qua `ApprovalAuthorizer.CanDecide` trên Approval `pending` của chủ thể (cùng tập principal của CR-REQ-010). Nếu `approvals.self_approval_allowed=false` và người gọi là `reporter_id` thì `REQUEST_DECISION_SELF_CHOICE_FORBIDDEN`, kể cả admin.
7. **Hết hiệu lực.** Solution `superseded` (đổi loại, sinh lại, trả lời Clarification) thì Decision thành `superseded` cùng transaction.

### 2.9 Proto, lỗi, sự kiện, hạn mặc định

Proto `proto/orca/request/v1/clarification.proto` và `decision.proto` (mới), thêm vào `RequestService`:

```proto
enum ClarificationStatus { CLARIFICATION_STATUS_UNSPECIFIED=0; OPEN=1; ANSWERED=2; EXPIRED=3; CANCELLED=4; }
enum QuestionKind { QUESTION_KIND_UNSPECIFIED=0; TEXT=1; SINGLE_CHOICE=2; MULTI_CHOICE=3; FILE=4; BOOLEAN=5; }
message ClarificationQuestion { string id=1; int32 seq=2; string question_key=3; QuestionKind kind=4; string prompt=5; string reason=6;
  string options_json=7; string suggested_default_json=8; bool required=9; string target_path=10; string answer_json=11; }
message Clarification { string id=1; string display_id=2; string request_id=3; string source=4; string source_ref=5; ClarificationStatus status=6;
  string resume_status=7; int32 round=8; repeated ClarificationQuestion questions=9; google.protobuf.Timestamp due_at=10; int64 version=11; }
message AnswerItem { string question_id=1; string value_json=2; bool accept_default=3; }
message AnswerClarificationRequest { string clarification_id=1; repeated AnswerItem answers=2; bool complete=3; int64 expected_version=4; }
message AnswerClarificationResponse { Clarification clarification=1; string request_status=2; int32 request_revision=3; bool still_missing=4; }
// RPC: GetRequestReadiness, RequestClarification, ListClarifications, GetClarification, AnswerClarification, CancelClarification,
//      ListPendingClarificationsForUser, WaiveReadiness, ListDecisions, GetDecision, ConfirmDecision
```

Lỗi (`apperrors`, tiền tố `REQUEST_`): `REQUEST_CLARIFICATION_NOT_FOUND` (NotFound), `REQUEST_CLARIFICATION_NOT_OPEN`, `REQUEST_CLARIFICATION_EXPIRED`, `REQUEST_CLARIFICATION_ALREADY_ANSWERED`, `REQUEST_CLARIFICATION_INCOMPLETE`, `REQUEST_CLARIFICATION_NOT_ASSIGNEE` (PermissionDenied), `REQUEST_CLARIFICATION_INVALID_ANSWER` (InvalidArgument), `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED` (trạng thái Request không cho hỏi), `REQUEST_CLARIFICATION_VERSION_CONFLICT`, `REQUEST_READINESS_WAIVE_FORBIDDEN`, các mã `REQUEST_DECISION_*` ở 2.8, `REQUEST_SOLUTION_BLOCKING_QUESTIONS`, `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS`.

Sự kiện outbox: `orca.request.clarification.requested|answered|expired|cancelled`, `orca.request.decision.recorded|confirmed`; trigger `information_required` và `information_provided` đi trong `orca.request.request.status_changed`. Payload không chứa nội dung câu hỏi hay câu trả lời.

Hạn mặc định đề xuất (cần xác nhận, không phải số đã đo): `readiness` 7 ngày (`urgency=urgent`: 24 giờ); `solution_open_question`, `plan_assumption` 72 giờ (8 giờ khi `urgent`); `task_blocked` 24 giờ; cấu hình bằng chính sách của CR-REQ-010 khi có.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Một trạng thái `awaiting_information` cho mọi nguồn, đích quay lại lưu ở Clarification | Không nhân trạng thái theo từng nguồn; bảng chuyển CR-REQ-003 chỉ thêm hai trigger |
| 2 | Kiểm tra sẵn sàng lúc xác nhận loại, không lúc phân loại AI | Trường bắt buộc phụ thuộc loại; người đã xác nhận loại mới là căn cứ |
| 3 | Mỗi Request chỉ một Clarification `open` | Tránh hai nơi cùng giữ Request ở `awaiting_information`; chỉ mục duy nhất ở cả hai dialect |
| 4 | Hết hạn về `request_backlog` qua `ReturnToBacklog`, không đường riêng | Giữ một nơi ghi lý do trả về (CR-REQ-006) |
| 5 | Câu trả lời thành revision mới (CR-REQ-027), không ghi đè | Solution và Plan truy vết được tới đúng revision; Approval không mất căn cứ |
| 6 | Hỏi từ Solution và Plan là thao tác người, chỉ `blocking` mới chặn duyệt | AI sinh `open_questions` tùy tiện; tự dừng Request theo AI làm tắc luồng |
| 7 | Decision tách khỏi `solutions.chosen_option` và Approval | `chosen_option` chỉ lưu kết quả; lý do, người chọn, lịch sử và xác nhận lần hai cần chỗ riêng |
| 8 | Xác nhận lần hai bằng gõ lại tên phương án | Chống bấm nhầm, không cần người thứ hai (tách nhiệm vụ do Approval lo) |
| 9 | Chặn waive với `hotfix`, `security`, `ops_request` | Thiếu dữ liệu ở ba loại này gây hại thật, README mục 3.4 đã đòi người xác nhận |

## 4. Tiêu chí chấp nhận

- [ ] `awaiting_information` có trong CHECK của `requests.status` trên Postgres 14+ và MySQL 8.0.16+; migration lên và xuống sạch.
- [ ] Bảng chuyển: `information_required` từ mỗi trong 6 trạng thái nguồn tới `awaiting_information`; `information_provided` chỉ tới `analyzing`, `planning`, `executing`; mọi cặp khác bị `REQUEST_TRANSITION_NOT_ALLOWED` (test bảng đầy đủ 12 trạng thái nhân mọi trigger).
- [ ] `ConfirmRequestType` trên `bug` thiếu `repro_steps` và AC: Request vào `awaiting_information` (không qua `analyzing`), có đúng một Clarification `open` với câu hỏi cho từng khóa thiếu, có sự kiện `type_confirmed` và `clarification.requested`; Request đủ dữ liệu vào `analyzing` như cũ.
- [ ] Hai `RequestClarification` đồng thời cho cùng Request: đúng một thành công (chỉ mục duy nhất, cả hai dialect).
- [ ] `AnswerClarification complete=true` đủ câu bắt buộc: Clarification `answered`, `content_revision` tăng một, `request_revisions` có dòng `clarification_answered`, Solution `proposed` cũ thành `superseded`, Request vào `resume_status`, và một run `GenerateSolution` mới khi `analyzing` (consumer dedup khi giao lặp).
- [ ] `complete=false` chỉ lưu nháp; câu hỏi `single_choice` với giá trị ngoài `options` bị `REQUEST_CLARIFICATION_INVALID_ANSWER`.
- [ ] Sau câu trả lời vẫn thiếu: Clarification mới `round=2`; sau `REQUEST_CLARIFICATION_MAX_ROUNDS`, Request về `request_backlog` với `returned_category=missing_info`.
- [ ] Quá hạn: vòng quét đặt `expired` và Request về `request_backlog` (`missing_info`, stage đúng theo 2.1); hai instance quét cùng lúc không xử lý trùng; `AnswerClarification` sau hạn chưa quét trả `REQUEST_CLARIFICATION_EXPIRED`.
- [ ] `type_change` và `cancel` từ `awaiting_information` hủy Clarification `open`; Approval `pending` bị hủy khi vào `awaiting_information` từ trạng thái duyệt.
- [ ] `WaiveReadiness` bị từ chối cho `hotfix`, `security`, `ops_request` và cho người không phải admin; thành công thì có dấu vết.
- [ ] Người không thuộc `clarification_assignees` và không phải admin bị `REQUEST_CLARIFICATION_NOT_ASSIGNEE`; thông báo không chứa nội dung câu hỏi.
- [ ] Chọn phương án khác đề xuất mà thiếu `rationale` bị `REQUEST_DECISION_RATIONALE_REQUIRED`; phương án `breaking_change` ở trạng thái `chosen` chặn `Approve` bằng `REQUEST_DECISION_NOT_EFFECTIVE` đến khi `ConfirmDecision` đúng tên phương án.
- [ ] Chọn lại khi Approval còn `pending` ghi `decision_history` và xóa xác nhận cũ; Solution `superseded` kéo theo Decision `superseded`.
- [ ] Giao lặp `AnswerClarification`, `ConfirmDecision` và consumer `information_provided` không tạo revision hay run thứ hai.

## 5. Kiểm thử

- **Unit (domain, không DB):** `ReadinessPolicy` theo 11 loại; `QuestionBuilder`; `ApplyAnswers` theo `target_path`; bảng chuyển mới; `ResumeStatusFor`; `DecisionRisk.Assess`; so khớp `confirmation_text` với chữ tiếng Việt có dấu (NFC và dạng tổ hợp).
- **Unit (usecase, repo và clock giả):** nhánh `AnswerClarification` (nháp, thiếu, sai kiểu, quá hạn), vòng tối đa, waive, chọn lại, chặn duyệt khi Decision chưa `effective`.
- **Integration hai dialect:** chỉ mục duy nhất `open`, `SKIP LOCKED` hai worker, transaction đa bảng (`clarifications` + `request_revisions` + `requests` + outbox) rollback khi một bước lỗi, CHECK `status` 12 giá trị.
- **Hợp đồng:** `buf breaking`; test đối chiếu danh sách trạng thái và loại trong mã với README v6 (đã đề xuất ở CR-REQ-003) cập nhật thành 12 trạng thái.
- **Chưa kiểm chứng:** thời gian trả lời thực tế của người báo cáo (hạn mặc định chỉ là đề xuất); chất lượng nháp AC do AI; trạng thái task quay lại sau `needs_info` (2.6).

## 6. Rủi ro và điểm chưa kiểm chứng

- Thêm trạng thái chạm mọi nơi liệt kê trạng thái: `ListBacklog`, Board, thống kê, frontend `RequestStatus`, test hợp đồng. Rà bằng `codegraph` hoặc grep trước khi sửa.
- Người báo cáo từ Jira có thể không có tài khoản Orca; Clarification gửi tới `reporter_id` mà người đó không vào được sẽ hết hạn. Cần quyết định có trả lời ngược vào Jira không (Q4), hiện ngoài phạm vi.
- Hủy Approval `pending` khi vào `awaiting_information` làm mất lựa chọn đã chọn nhưng chưa duyệt; chấp nhận vì Solution sẽ bị thay.
- Vòng hỏi nhiều lần làm chậm; giới hạn 3 vòng là đề xuất.
- Kiểu `file` chỉ nhận văn bản (không có kho tải lên); Request cần ảnh chụp lỗi hay log nhị phân chưa được hỗ trợ.

## 7. Câu hỏi mở

1. Quyền ghi và quyền hỏi tay ở mức Request vẫn chưa được chốt (README v6 mục 8, CR-REQ-010): ai được `RequestClarification` thủ công.
2. Có thêm kho tệp đính kèm cho `file` không, hay hoãn.
3. `information_provided` có nên tự chạy `GenerateSolution` (đề xuất: có) và `GeneratePlan` (đề xuất: không).
4. Có đẩy câu hỏi và câu trả lời ngược lại Jira/GitHub không (liên quan CR-REQ-024).
5. Ngưỡng `high` của rủi ro: `breaking_change`, mức `high`, ba service, đã đủ chưa; có cần "không đảo ngược" khi `rollback` rỗng.
6. Xác nhận lần hai có cần người khác `chooser` ở một số loại (`security`) không.

## 8. Tham chiếu

- `/opt/repos/orca/docs/research/receive-request/artifact-formats-ontology-and-execution-readiness.md` mục 6, 14, 15
- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.5, 3.7, 8; CR-REQ-002, 003, 005, 006, 007, 009, 010, 012, 013, 027
- `/opt/repos/orca/backend-go/services/notification-service/internal/adapter/eventbus/consumer.go` (`Subjects`), `internal/domain/notification_event.go` (`subjectRules`)
- `/opt/repos/orca/backend-go/services/mcp-service/internal/domain/approval.go` (`ParamsHash`, `EffectiveStatus`, tiền lệ chống duyệt nội dung đã đổi và hết hạn lười)
- `/opt/repos/orca/backend-go/common/tenant/tenant.go` (`UserID`, `Role`), `common/outbox/outbox.go`, `common/dbcapability/capability.go`
- Mới: `request-service/internal/domain/clarification.go`, `clarification_question.go`, `readiness_policy.go`, `question_builder.go`, `decision.go`, `decision_risk.go`; `internal/usecase/request_clarification.go`, `answer_clarification.go`, `expire_clarifications.go`, `record_decision.go`, `confirm_decision.go`, `waive_readiness.go`; `internal/adapter/{postgres,mysql}/clarification_repository.go`, `decision_repository.go`; `internal/adapter/eventbus/clarification_resume_consumer.go`

## 9. Tác động tới CR hiện có (không sửa trong CR này; người duyệt series sửa theo bảng)

| CR | Cần sửa gì |
|---|---|
| README v6 | Mục 3.3: 12 trạng thái, thêm `awaiting_information`; 3.5: các bảng ở 2.2 và 2.8; 3.6: RPC ở 2.9; 3.7: sự kiện `clarification.*`, `decision.*`; mục 8: ghi hai trigger mới |
| CR-REQ-002 | `requests.status` CHECK 12 giá trị; thêm bảng `clarifications`, `clarification_questions`, `clarification_assignees`, `decisions`, `decision_history`; `returned_from_stage` đã đủ giá trị |
| CR-REQ-003 | Mục 2.2: thêm hai trigger, mở rộng nguồn của `return_to_backlog`, `type_change`, `cancel` thêm `awaiting_information`; `TransitionInput` thêm `ResumeStatus`; mục 4: test bảng đầy đủ thành 12 trạng thái; mục 2.4 thêm `REQUEST_RESUME_STATUS_INVALID`; bảng 2.1 không đổi |
| CR-REQ-005 | `ConfirmRequestType` gọi `ReadinessPolicy.Evaluate` và dùng `information_required` khi chưa sẵn sàng; `type_confirmed` event vẫn phát; mã lỗi thêm `REQUEST_READINESS_WAIVE_FORBIDDEN` |
| CR-REQ-006 | `ReturnToBacklog` nhận nguồn `awaiting_information` với `category=missing_info`; `ReopenRequest` từ `missing_info` nên gợi ý Clarification mới; hạn `stage` suy theo 2.1 |
| CR-REQ-007 | `ChooseSolutionOption` gọi `RecordDecision` cùng transaction; `ValidateForRequest` kiểm Decision `effective` và `blocking` open_questions; `GenerateSolution` chấp nhận `feedback="clarification:<id>"` từ consumer; `OnClosedWithoutDecision` và đổi loại cũng làm Decision `superseded` |
| CR-REQ-009 | `CancelPendingForRequest` thêm lý do `information_required`; `stage` của Approval không đổi; chỉ mục `pending_key` làm mẫu cho `open_key` |
| CR-REQ-010 | Dùng lại tập principal cho `clarification_assignees`; thêm hai hàng thông báo; hạn mặc định của 2.9 vào bảng mặc định; tránh ghi trùng `consumer.go` và `notification_event.go` (hai CR sửa cùng tệp) |
| CR-REQ-012 | `plan_subject_handler` kiểm `needs_confirmation` và Decision `effective`; `planning` có thêm đường vào `awaiting_information` |
| CR-REQ-013 | Phân loại outcome `needs_info` gọi `RequestClarification(source=task_blocked)`; `AdvanceExecution` được gọi lại khi `information_provided` về `executing` |
| CR-REQ-015, 023 | Request backlog gom `missing_info`; xem xét view "đang chờ thông tin" (`awaiting_information`) |
| CR-REQ-016, 017 | Kênh `clarification.list|get|answer|cancel`, `request.readiness`, `decision.list|confirm`; MCP: `clarification_list`, `clarification_get`, `clarification_answer` (theo chính sách người dùng cấp quyền), không đăng ký `decision_confirm` |
| CR-REQ-018 đến 022 | `RequestStatus` thêm giá trị; form trả lời Clarification; Hộp duyệt (CR-REQ-022) hiển thị Clarification chờ người dùng |
| CR-REQ-024, 025 | Đo: số vòng hỏi, thời gian trả lời, tỉ lệ hết hạn; e2e thêm kịch bản hỏi đáp |
