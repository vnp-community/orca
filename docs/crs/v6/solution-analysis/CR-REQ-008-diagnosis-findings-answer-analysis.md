# CR-REQ-008 — Chẩn đoán, Findings, Answer và chạy agent không cần worktree

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-008 |
| **Tên** | Sinh `diagnosis`, `findings`, `answer` bằng agent chỉ đọc chạy trên repo gốc, không tạo worktree |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-007 (bảng `analysis_runs`, `GenerateSolution`, handler `solution`), CR-REQ-009 (Approval), CR-REQ-003 (registry luồng) |
| **Mở khoá** | CR-REQ-014 (đo baseline `performance`, hotfix nhanh, `security`), CR-REQ-006 (Request con từ `spike`/`question`) |
| **Tác động** | `request-service` (usecase, adapter grpcclient, domain). Không đổi `task-service`. Có thể cần đổi agent (TypeScript, ngoài `backend-go`) cho chế độ chỉ đọc thật sự: xem mục 2.3 và 7 |

## 1. Bối cảnh và vấn đề

1. README 3.4: `bug`, `hotfix`, `security`, `performance` có bước `diagnosis`; `spike` có `findings`; `question` có `answer`. `question` và `spike` không tạo Task nên không có worktree theo thiết kế ("không worktree, không Task", nghiên cứu mục 2).
2. **Code hiện tại không chạy được agent mà không có worktree:**
   - `ExecuteTask.Execute` luôn gọi `uc.worktrees.EnsureWorktree(...)` (`execute_task.go`, quanh dòng 202) và đổi `status` sang `in_progress`.
   - `SimpleExecutor.Execute` trả `TASK_EXECUTE_NO_WORKTREE_PATH` nếu `worktreePath` rỗng.
   - Executor này vứt `stdout`: nó chỉ trả `executionRef`, còn stdout chỉ đi ra kênh `agent_output_partial` để hiển thị. Không có nơi nào giữ lại câu trả lời của agent làm dữ liệu.
3. Ở tầng agent, RPC `agent.execPrompt` chỉ cần `prompt` và `worktreePath` (dùng làm `cwd`), trả `{stdout, stderr, exitCode, timedOut, stepId}` (doc comment của `simple_executor.go`). `ResolveConnection` của infra-fleet trả `repo_path`, là đường dẫn kết nối đã ghi cho dự án (comment BUG-028 trong `simple_executor.go` gọi nó là repo gốc dùng chung của dự án; chưa kiểm chứng trên dev server thật). Vậy về mặt kỹ thuật chạy trên `repo_path` là khả thi mà không cần worktree, nhưng đường này chưa ai dùng cho việc chỉ đọc.
4. `agent.execPrompt` không có tham số chỉ đọc. Tham số hiện có: `stepId, trustPreset ("full" thì thêm cờ YOLO, giá trị khác là no-op), model, accountId, env, timeoutMs`. Nền tảng không ép được "không ghi file".
5. `AICompleter` (`ai.complete`) không đọc được repo nên không thay được bước chẩn đoán lỗi thật.

## 2. Giải pháp đề xuất

### 2.1 Phạm vi theo loại (registry của CR-REQ-003)

| Loại Request | `kind` | Chế độ mặc định | Cổng (`Approval.subject_type`) |
|---|---|---|---|
| `bug`, `performance`, `security` | `diagnosis` | `agent_readonly` | `solution` (handler của CR-REQ-007) |
| `hotfix` | `diagnosis` (nhanh) | `agent_readonly`, timeout ngắn | không có cổng (`GateSubject` rỗng trong registry CR-REQ-003); Solution `proposed → approved` do hệ thống, event `solution.approved` mang `auto=true` |
| `spike` | `findings` | `agent_readonly` | `findings` |
| `question` | `answer` | `agent_readonly` | `answer` (người dùng chấp nhận) |

Không có RPC mới: dùng `GenerateSolution` của CR-REQ-007; `kind` suy ra từ loại Request qua registry; `analysis_mode` tùy chọn ghi đè (`COMPLETE` khi chỉ cần câu trả lời văn bản, không đọc repo). Không tự hạ cấp từ `agent_readonly` xuống `complete` khi lỗi, vì hạ chất lượng âm thầm. `performance`: đo baseline cần chạy lệnh thật, không phải chỉ đọc; CR này chỉ chừa trường `measurements[]` trong schema, việc chạy đo thuộc CR-REQ-014.

### 2.2 Đường chạy: `AgentReadonlyRunner` trong `request-service`

Không dùng `ExecuteTask` (cần Task, worktree, đổi status, và vứt stdout). Thay vào đó:

1. `ResolveConnection(project_id)` của infra-fleet lấy `connection_id` và `repo_path`. Không kết nối: `REQUEST_ANALYSIS_NO_CONNECTION` (FailedPrecondition). `repo_path` rỗng: `REQUEST_ANALYSIS_NO_REPO_PATH`.
2. Gọi `Relay` method `agent.execPrompt` với `{prompt, worktreePath: repo_path, stepId: run_id, trustPreset: "default", env: {ORCA_REQUEST_ID, ORCA_PROJECT_ID}, timeoutMs}`. Không đặt `trustPreset="full"` (khác `SimpleExecutor`, đặt `"full"`). Không gửi token Orca, credential hay biến môi trường nhạy cảm vào `env`.
3. Giữ `stdout` làm đầu vào của bộ trích JSON (như 2.5 của CR-REQ-007). `exitCode != 0` hoặc `timedOut`: run `failed` với `REQUEST_ANALYSIS_AGENT_FAILED` / `REQUEST_ANALYSIS_TIMEOUT`.
4. Chạy qua Relay nên hoạt động giống nhau với dev server cục bộ, SSH, WSL (AGENTS.md "SSH Use Case"). Không giả định tiến trình cục bộ.
5. Dùng chung `analysis_runs`, lease, vòng quét phục hồi và chỉ mục "một run `running` mỗi `(request, kind)`" của CR-REQ-007. Run `mode=agent_readonly`.
6. Giới hạn đồng thời: tối đa 2 run `agent_readonly` đang `running` cho mỗi `(tenant, project)` (cấu hình `REQUEST_AGENT_READONLY_MAX_PER_PROJECT`); vượt thì `REQUEST_ANALYSIS_BUSY` (FailedPrecondition). Lý do: `execute_task.go` ghi nhận bấm nhiều lần làm sập kết nối dev server ("connection lost: EOF"). Con số 2 là đề xuất, chưa đo.

### 2.3 Bảo vệ "chỉ đọc" (ba lớp, không lớp nào tuyệt đối)

Do `agent.execPrompt` không ép được chỉ đọc (mục 1.4), `agent_readonly` là cam kết ở mức quy trình:

1. **Prompt:** cấm sửa/xoá/tạo file, cấm `git commit/checkout/reset/clean/push`, cấm gọi mạng ngoài; chỉ được đọc và chạy lệnh đọc.
2. **Kiểm sau chạy:** trước và sau run lấy trạng thái working tree và `HEAD` của `repo_path` qua git-gateway (RPC `GetStatus` có thật trong `proto/orca/gitgateway/v1/gitgateway.proto`; việc nó có trả `HEAD` hay không chưa kiểm chứng, nếu không thì thêm RPC đọc `HEAD` hoặc dùng lệnh `git rev-parse HEAD`, lệnh Git cơ bản; tuân `docs/reference/git-compatibility.md`; không thêm lệnh Git mới). Sau run so sánh; khác nhau thì run `failed` với `REQUEST_ANALYSIS_REPO_MODIFIED`, kết quả bị loại, thông báo cho admin. Chỉ phát hiện, không hoàn tác. Nếu thay đổi nằm ngoài thư mục theo dõi của Git thì không thấy: giới hạn đã biết.
3. **Dài hạn (không thuộc CR này):** thêm tham số `readOnly`/công cụ cho phép (`allowedTools`) vào `agent.execPrompt` phía agent. Cần xác minh trong mã agent (chưa đọc); ghi ở mục 7.

Phương án thay thế đã cân nhắc: tạo worktree tạm (detached) qua git-gateway cho từng run. An toàn hơn cho checkout chính nhưng tạo bản ghi lineage ở project-service, phát `worktree.created` (kích hoạt đồng bộ Jira "In Progress" qua `issue-status-sync` cho issue liên kết) và cần dọn dẹp. Hoãn sang bản sau nếu kiểm sau chạy không đủ.

### 2.4 Schema JSON (lưu trong cột `options` của `solutions`, `schema_version: 1`)

README đặt tên cột là `options` cho mọi `kind`; với ba `kind` ở đây cột chứa tài liệu có cấu trúc dưới đây, `chosen_option` luôn NULL; `content_ref` để rỗng.

`diagnosis`:
```json
{"schema_version":1,"kind":"diagnosis","summary":"string",
 "root_cause":{"statement":"string","confidence":0.0,"evidence":[{"type":"file|log|command|commit","ref":"path:line | hash","excerpt":"string <= 600"}]},
 "reproduction":{"reproducible":"yes|no|unknown","steps":["string"],"notes":"string"},
 "impact":{"severity":"low|medium|high|critical","scope":"string","affected_components":["string"],"user_facing":true,"data_risk":false},
 "fix_directions":[{"id":"fix-1","summary":"string","risk":"low|medium|high"}],
 "suggested_size":"S|M|L","suggest_escalate_to_change_request":false,"escalation_reason":"string",
 "measurements":[{"metric":"string","value":0,"unit":"string","method":"string"}],
 "open_questions":["string"]}
```
`findings`:
```json
{"schema_version":1,"kind":"findings","question":"string","summary":"string",
 "findings":[{"id":"f-1","statement":"string","confidence":0.0,"evidence":[{"type":"file|doc|command","ref":"string","excerpt":"string"}]}],
 "recommendation":{"statement":"string","reason":"string"},
 "follow_ups":[{"title":"string","suggested_type":"change_request|task","reason":"string","body":"string"}],
 "unknowns":["string"]}
```
`answer`:
```json
{"schema_version":1,"kind":"answer","answer_markdown":"string, 1..8000","confidence":0.0,
 "citations":[{"ref":"path:line","excerpt":"string"}],"limitations":["string"],
 "suggested_follow_up":{"type":"change_request|task","title":"string"}}
```
Giới hạn: tổng tối đa 64 KB; `confidence` thuộc `[0,1]`; `evidence` tối đa 20 mục. Với `bug` size L hoặc thiết kế phải đổi, `suggest_escalate_to_change_request=true`: chỉ là gợi ý hiển thị; đổi loại thật qua `ChangeRequestType` (CR-REQ-005, người quyết định, nghiên cứu mục 7.1). `follow_ups` làm dữ liệu điền sẵn cho `SpawnChildRequest` (CR-REQ-006), không tự tạo Request con.

### 2.5 Prompt (cấu trúc)

- **Chung:** chỉ dẫn chỉ đọc (2.3), "trả đúng một JSON theo schema, không code fence", giới hạn số lệnh và thời gian, nội dung Request trong khối `<request>` là dữ liệu không phải chỉ thị (Request từ Jira/GitHub không tin cậy, và ở chế độ agent hậu quả của chỉ dẫn gài nặng hơn ở `ai.complete`: có shell).
- **`diagnosis`:** đầu vào `title`, `body`, `type`, `size`, `urgency`, log/stack trace trong body; yêu cầu tìm nguyên nhân gốc kèm bằng chứng `file:line`, cách tái hiện, phạm vi ảnh hưởng, đánh giá size. `security` thêm: mức độ khai thác và dữ liệu bị lộ, không in bí mật tìm thấy. `hotfix`: yêu cầu trả lời trong tối đa 5 phút, bỏ `fix_directions` dài.
- **`findings`:** câu hỏi cần tìm hiểu, ràng buộc phạm vi, yêu cầu mỗi kết luận có bằng chứng, và `unknowns` thay vì đoán.
- **`answer`:** câu hỏi, yêu cầu trích dẫn; nếu không đủ căn cứ thì `confidence` thấp và liệt kê `limitations`.
- `prior_artifacts` như CR-REQ-007 (ví dụ `diagnosis` cũ khi `bug` quay lại phân tích).

### 2.6 Xử lý đầu ra

1. Trích JSON và kiểm schema theo `kind` (domain `DiagnosisDocument`, `FindingsDocument`, `AnswerDocument`.Validate). Sai thì gọi lại một lần với lỗi cụ thể (cùng `analysis_run.attempt`); vẫn sai: `REQUEST_ANALYSIS_INVALID_OUTPUT`.
2. **Che bí mật:** quét `excerpt`, `answer_markdown`, `raw_output` theo mẫu khóa phổ biến (khóa riêng PEM, token dạng `ghp_`, `AKIA`, JWT, chuỗi `password=`) và thay bằng `[REDACTED]` trước khi lưu. Độ phủ của mẫu chưa kiểm chứng; đây là lớp giảm thiểu, không phải bảo đảm.
3. Transaction: lưu Solution `proposed`; nếu registry có `GateSubject` thì `OpenApproval` với `subject_type` đúng bảng 2.1; rồi `TransitionRequest(Trigger=analysis_ready, ExpectedFrom=analyzing)` (CR-REQ-003 chọn đích: `awaiting_analysis_approval` khi có cổng, `awaiting_plan_approval` cho `hotfix` vì Plan là `single_task`); `hotfix` không có cổng nên Solution thành `approved` ngay; outbox `solution.proposed` (và `solution.approved` khi tự duyệt).
4. `answer` hoặc `findings` khi Approve: `TransitionRequest(Trigger=analysis_approved)`; registry có `CompletesAfterAnalysis=true` cho `spike` và `question` nên Request sang `completed`, không có Plan, không có Task. Request con tạo sau qua CR-REQ-006. Reject: `analysis_rejected` (về backlog). Muốn trả lời lại có phản hồi: `GenerateSolution` kèm `feedback` ở `awaiting_analysis_approval` (trigger `analysis_revision`, xem CR-REQ-007 mục 2.6).

### 2.7 Handler Approval

- `findings`, `answer`: `SubjectHandler` mới (`findings_approval_handler.go`, `answer_approval_handler.go`), cùng khuôn `solution_approval_handler.go`: `Validate` (Solution đúng `kind`, đang `proposed`), `subject_digest = sha256(options)`, `OnApproved`/`OnRejected` ghi trạng thái Solution trong transaction của Approve.
- `diagnosis`: do handler `solution` của CR-REQ-007 xử lý; CR này thêm bộ kiểm hợp lệ theo `kind` (không yêu cầu `chosen_option`).

### 2.8 Lỗi, quyền, sự kiện

- Lỗi mới (tiền tố `REQUEST_ANALYSIS_`): `NO_CONNECTION`, `NO_REPO_PATH`, `BUSY`, `AGENT_FAILED`, `TIMEOUT`, `REPO_MODIFIED`, `INVALID_OUTPUT`, `KIND_NOT_ALLOWED` (loại Request không có bước này). Mã `REQUEST_SOLUTION_*` của CR-REQ-007 vẫn dùng cho các lỗi chung (sai trạng thái, không tìm thấy).
- Quyền: như CR-REQ-007 mục 2.8. Chạy agent có ảnh hưởng chi phí và rủi ro cao hơn `ai.complete`, nên chỉ chạy sau khi loại đã được người xác nhận (`awaiting_type_confirmation` đã qua; README 3.3), kể cả `hotfix` và `security` (xác nhận bắt buộc).
- Sự kiện: dùng lại `solution.proposed`, `solution.approved`; payload thêm `kind` (đã có), `mode`.

### 2.9 Tệp sẽ tạo (mới)

`internal/usecase/run_agent_readonly_analysis.go`, `analysis_document_validation.go`, `analysis_secret_redaction.go`, `findings_approval_handler.go`, `answer_approval_handler.go`; `internal/domain/diagnosis_document.go`, `findings_document.go`, `answer_document.go`; `internal/adapter/grpcclient/agent_prompt_relay.go`, `repo_state_probe.go` (git-gateway, `git status`/`HEAD`); mở rộng `solution_prompt.go` của CR-REQ-007 theo `kind`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Không dùng `ExecuteTask` để chạy phân tích | Cần Task + worktree + status, và không giữ stdout. Spike/question không có Task. |
| 2 | Chạy trên `repo_path` của dự án, không tạo worktree | Không tạo lineage/sự kiện `worktree.created` làm lệch Jira; code agent chỉ cần `cwd`. |
| 3 | `trustPreset: "default"`, không `full` | `full` thêm cờ YOLO; phân tích không cần quyền ghi. |
| 4 | Phát hiện sửa đổi sau chạy thay vì tin prompt | Prompt không phải rào chắn; kiểm `git status` rẻ và dùng Git cơ bản. |
| 5 | Không tự hạ xuống `ai.complete` khi agent lỗi | Chất lượng giảm âm thầm; để người chọn `analysis_mode`. |
| 6 | Giữ một RPC `GenerateSolution` | README 3.6 không có RPC riêng; `kind` do registry quyết, tránh bề mặt API nhân đôi. |
| 7 | Cột `options` chứa tài liệu theo `kind` | Giữ đúng 3.5 README, tránh thêm bảng/cột; `schema_version` + `kind` trong JSON để UI phân biệt. |
| 8 | Hotfix tự duyệt Solution | README 3.4: `diagnosis` nhanh không cổng; vẫn phát event để audit. |

## 4. Tiêu chí chấp nhận

- [ ] `GenerateSolution` trên Request `question` ở `analyzing` tạo run `mode=agent_readonly`, không tạo Task, không gọi `EnsureWorktree`/`CreateWorktree` (kiểm bằng fake git-gateway ghi nhận 0 lời gọi tạo worktree).
- [ ] Lời gọi Relay có `method=agent.execPrompt`, `worktreePath=repo_path`, `trustPreset="default"`, `env` không chứa khóa nhạy cảm.
- [ ] Kết quả hợp lệ lưu đúng `kind` với JSON đúng schema 2.4; Approval có `subject_type` đúng bảng 2.1 (`bug` → `solution`, `spike` → `findings`, `question` → `answer`).
- [ ] `hotfix`: không tạo Approval, Solution `approved`, outbox `solution.approved` có `auto=true`.
- [ ] `exitCode != 0`, `timedOut`, JSON sai hai lần: run `failed` với mã tương ứng, Request giữ `analyzing`, không còn Solution `draft`.
- [ ] Agent làm đổi `git status` hoặc `HEAD` của `repo_path`: run `failed` `REQUEST_ANALYSIS_REPO_MODIFIED`, kết quả không được lưu.
- [ ] Thứ ba cùng lúc trong một `(tenant, project)` trả `REQUEST_ANALYSIS_BUSY`.
- [ ] Chuỗi giống khóa riêng PEM và `ghp_...` trong đầu ra bị thay `[REDACTED]` ở `options` và `raw_output`.
- [ ] `suggest_escalate_to_change_request=true` chỉ hiển thị gợi ý, không đổi `requests.type`.
- [ ] `answer` được chấp nhận: Request `completed`, không có Plan, không có Task.
- [ ] Mọi bước dùng `tenant.RequireTenantID`; truy vấn chéo tenant trả `REQUEST_SOLUTION_NOT_FOUND`.

## 5. Kiểm thử

- **Unit:** bộ kiểm tài liệu theo `kind` (bảng đúng/sai), `secret redaction` (mẫu dương và âm, tiếng Việt có dấu), bảng quyết định mode/kind/cổng theo registry, tự duyệt hotfix.
- **Unit (usecase, fake Relay/git probe):** thành công, exit khác 0, timeout, sửa repo, quá giới hạn đồng thời, mất lease giữa chừng, JSON sai rồi đúng.
- **Integration trên Postgres và MySQL:** chạy lại bộ test repository của CR-REQ-007 với `mode=agent_readonly`; đếm run `running` đồng thời đúng ở cả hai DB (MySQL không có partial index: kiểm tra đường đếm trong transaction).
- **Hợp đồng:** proto không đổi so với CR-REQ-007 ngoài enum `AnalysisMode` đã có; test golden JSON cho ba schema; test `SubjectHandler` của `findings` và `answer`.
- **Thủ công, chưa kiểm chứng:** chạy `agent.execPrompt` thật với `trustPreset=default` để xem agent có thực sự bị chặn ghi file hay chỉ bị hỏi quyền rồi treo; chạy trên dev server SSH.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Không có chế độ chỉ đọc thật.** Agent có shell trên repo gốc của dự án; Request từ nguồn ngoài có thể gài chỉ dẫn. Giảm thiểu: người xác nhận loại trước; prompt; kiểm sau chạy; che bí mật. Còn lại: agent có thể đọc tệp bí mật trong repo hoặc gọi mạng. Cần quyết định chấp nhận rủi ro hay chờ tham số `readOnly` ở agent (câu hỏi 1).
- Hành vi của `trustPreset` khác `full` trong chế độ in (`-p`) chưa kiểm chứng: có thể agent bị chặn mọi công cụ nên không đọc được file.
- `repo_path` có thể là checkout đang được dùng cho việc khác (đang sửa dở); `git status` bẩn từ trước làm phép so sánh nhiễu. Cần so sánh với trạng thái trước run, không với "sạch".
- Chạy đồng thời với người dùng đang thao tác trong cùng repo: khóa Git (`index.lock`) có thể va chạm nếu agent chạy lệnh Git ghi; prompt cấm nhưng không ép.
- Chi phí và thời gian run agent chưa đo; `timeoutMs` mặc định đề xuất 600 000 (hotfix 300 000).
- `performance` cần đo thật; CR này không giải quyết (CR-REQ-014).

## 7. Câu hỏi mở

1. Chấp nhận mức bảo vệ "chỉ đọc" ở 2.3 cho v1, hay chặn `spike`/`question`/`diagnosis` đến khi agent có `readOnly`? (Khuyến nghị: chấp nhận cho v1 kèm cảnh báo trong tài liệu, vì chặn thì cả nhóm loại này không chạy.)
2. README 3.5: `content_ref` chưa được định nghĩa (cùng câu hỏi ở CR-REQ-007). Ba `kind` ở đây để rỗng (CR-REQ-002 đặt mặc định chuỗi rỗng).
3. README 3.6 không có RPC riêng để xem một run đang chạy; CR này dựa vào `ListSolutionsResponse.runs` (CR-REQ-007). Có cần `GetAnalysisRun` hay stream tiến độ cho UI không?
4. `analysis_revision` của CR-REQ-003 chỉ được kích hoạt bằng `GenerateSolution` kèm `feedback` (CR-REQ-007); chưa có RPC "yêu cầu sinh lại" riêng. Có cần không?
5. `hotfix` tự duyệt Solution: theo CR-REQ-003, hotfix không qua `awaiting_analysis_approval`. Nhưng `GateSubject` rỗng cũng làm Solution không có Approval nào để audit; chấp nhận ghi vết bằng `solution.approved` với `auto=true`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (3.3, 3.4, 3.5)
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md` (mục 2, 6)
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/execute_task.go` (`EnsureWorktree`, `selectEngine`, `dispatchDirectAgentAsync`)
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/simple_executor.go` (tham số `agent.execPrompt`, `TASK_EXECUTE_NO_WORKTREE_PATH`, `trustPreset: "full"`)
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/worktree_provisioner.go`, `project_execution_resolver.go`
- `/opt/repos/orca/backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (`ai.complete`, mẫu Relay)
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`ResolveConnectionResponse.repo_path`, `Relay`)
- `/opt/repos/orca/guides/reference/git-compatibility.md`
- `/opt/repos/orca/docs/crs/v6/solution-analysis/CR-REQ-007-solution-generation-options-and-selection.md`
