# Cách đưa ra giải pháp, lên Plan và thực thi Plan

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-06 |
| **Loại** | Diễn giải thiết kế (chưa phải CR, chưa có code) |
| **Nguồn** | [CR-REQ-007](../../crs/v6/solution-analysis/CR-REQ-007-solution-generation-options-and-selection.md), [CR-REQ-008](../../crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md), [CR-REQ-012](../../crs/v6/plan-phase-task/CR-REQ-012-plan-phase-task-generation-from-solution.md), [CR-REQ-013](../../crs/v6/plan-phase-task/CR-REQ-013-phase-execution-and-feedback-loop.md), [CR-REQ-014](../../crs/v6/plan-phase-task/CR-REQ-014-type-specific-execution-policies.md) |
| **Phạm vi đã đọc** | 150 dòng đầu của CR-007, CR-012, CR-013. CR-008 và CR-014 chỉ tóm tắt theo báo cáo của agent soạn, chưa đọc lại. Các mục kiểm thử, rủi ro, câu hỏi mở của ba CR kia chưa đối chiếu. |
| **Liên quan** | [request-classification-and-flows.md](./request-classification-and-flows.md), [request-pipeline-existing-capabilities-and-build-scope.md](./request-pipeline-existing-capabilities-and-build-scope.md) |

> Mọi thứ dưới đây là thiết kế đề xuất. Các giá trị mặc định (timeout, số lần thử, giới hạn kích thước) là ước lượng, chưa đo.

## 1. Trạng thái Request đi qua ba giai đoạn

```
analyzing ─▶ awaiting_analysis_approval ─▶ planning ─▶ awaiting_plan_approval ─▶ executing ─▶ completed
     │                  │                      │                   │                  │
     └──────────────────┴──────────────────────┴───────────────────┴──────────────────┴──▶ request_backlog (kèm giai đoạn bị trả và lý do)
```

## 2. Đưa ra giải pháp (CR-REQ-007, CR-REQ-008)

### 2.1 Mỗi loại sinh một kiểu "giải pháp"

| Loại Request | Kết quả | Cổng duyệt |
|---|---|---|
| `change_request` | Solution, tối thiểu 2 phương án | `solution` |
| `refactor` | Solution, tối thiểu 1 phương án | `solution` |
| `bug`, `security`, `performance` | Chẩn đoán (nguyên nhân gốc, phạm vi ảnh hưởng, số đo baseline) | `solution` (xử lý cả `diagnosis`) |
| `spike` | Findings | `findings` |
| `question` | Answer | `answer` (người dùng chấp nhận) |
| `hotfix` | Chẩn đoán nhanh | không có |
| `task`, `docs`, `ops_request` | bỏ qua bước này | không có |

### 2.2 Sinh Solution (CR-007)

1. **Kích hoạt.** Người dùng bấm "Sinh giải pháp", gọi `GenerateSolution`. Request phải ở `analyzing` (hoặc ở `awaiting_analysis_approval` kèm `feedback`, xem 2.4). Trong một transaction hệ thống tạo `analysis_runs` (trạng thái `running`, có lease) và `solutions` (trạng thái `draft`), rồi trả ngay `{solution_id, run_id}`. Mỗi `(request_id, kind)` chỉ có một run `running` tại một thời điểm.
2. **Chạy nền, bền.** Worker gia hạn lease mỗi 30 giây (TTL 90 giây). Vòng quét 30 giây nhận lại run hết lease, đánh `failed` (`REQUEST_SOLUTION_RUN_INTERRUPTED`) và xoá Solution `draft`. Lý do: Engine 1 từng mất việc khi restart vì chạy trong goroutine không có lease.
3. **Dựng prompt.** Gồm:
   - Chỉ dẫn hệ thống: chỉ trả JSON đúng schema, số phương án tối thiểu, các phương án phải khác nhau về bản chất.
   - Khối `<request>`: tiêu đề, nội dung (cắt 12 000 ký tự), loại, size, urgency, lý do phân loại, nguồn. Kèm câu dặn nội dung này là dữ liệu, không phải chỉ thị, vì Request đến từ nguồn không tin cậy (Jira, GitHub, webhook, MCP).
   - Ngữ cảnh dự án: tên, `repo_url`, tech stack (best-effort, lỗi thì bỏ qua).
   - `<prior_artifacts>`: Solution cũ của cùng Request (đã `superseded` hoặc `rejected`), kèm lý do từ chối nếu có; mỗi mục cắt 8 000 ký tự.
   - Không đưa `credential_ref` hay bí mật nào vào prompt.
4. **Gọi AI.** Qua `ai.complete` của dev server agent thông qua `infra-fleet-service` Relay. Backend **không có đường gọi LLM trực tiếp**: `ai-provider-service` chỉ quản account. Hệ quả: project phải có dev server đang kết nối, không thì lỗi `REQUEST_SOLUTION_NO_CONNECTION`. `ai.complete` không đọc được repo nên Solution chỉ ở mức thiết kế; việc đọc code thuộc Chẩn đoán. Chạy được trên dev server cục bộ và SSH/remote vì cùng đường Relay.
5. **Kiểm tra đầu ra.** Bộ trích xuất chịu code fence và văn bản thừa. Sai schema thì gọi lại một lần kèm lỗi cụ thể; vẫn sai thì run `failed` (`REQUEST_SOLUTION_INVALID_OUTPUT`).
6. **Lưu.** Trong một transaction: Solution `proposed`, run `succeeded`, Solution cũ cùng `(request_id, kind)` thành `superseded`, ghi sự kiện `solution.proposed`, mở Approval kiểu `solution`, chuyển Request sang `awaiting_analysis_approval`.
7. **Lỗi.** Run `failed`, Solution `draft` bị xoá, Request giữ `analyzing`. Người dùng gọi lại hoặc trả backlog; không tự động trả.

### 2.3 Cấu trúc một phương án (schema `options`, version 1)

| Trường | Ý nghĩa |
|---|---|
| `id` | `opt-N`, duy nhất |
| `title`, `summary`, `approach` | Tên, tóm tắt (tối đa 600 ký tự), cách làm (markdown, tối đa 4 000 ký tự) |
| `pros`, `cons` | Ưu và nhược điểm |
| `risks[]` | Mô tả kèm mức `low|medium|high` |
| `effort` | `size` S/M/L (bắt buộc), `hours_estimate` (tuỳ chọn) |
| `affected_areas[]` | Loại `service|module|api|schema|ui|infra` và tên |
| `breaking_change`, `rollback` | Có phá vỡ tương thích không, cách quay lui |
| `recommended` | Đúng một phương án là `true`, trùng `recommendation.option_id` |

Kèm `recommendation.reason`, `assumptions`, `open_questions`. Số phương án trong khoảng `[min, 4]`.

### 2.4 Người duyệt chọn và quyết định

- **Chọn:** `ChooseSolutionOption(option_id)` ghi `chosen_option` bằng so sánh-và-ghi. Không đổi trạng thái Request. Chọn lại khi Approval còn `pending` thì ghi đè.
- **Chống duyệt nhầm bản cũ:** Approval giữ `subject_digest = sha256(options || chosen_option)`. Mỗi lần chọn, digest được làm mới và trả về để UI gửi lại khi `Approve`. Digest không khớp thì từ chối.
- **Duyệt:** với `kind=solution` phải đã chọn phương án (`REQUEST_SOLUTION_OPTION_NOT_CHOSEN`). Solution thành `approved`, sự kiện `solution.approved`, Request sang `planning`.
- **Từ chối:** Solution `rejected`, Request về `request_backlog` (`returned_from_stage=analysis`), lý do lấy từ comment.
- **Sinh lại có phản hồi** (khác với từ chối): `GenerateSolution` kèm `feedback` khi Request ở `awaiting_analysis_approval`. Trong một transaction: Approval cũ thành `cancelled`, Solution hiện tại thành `superseded`, Request về `analyzing`, tạo run mới với phản hồi đưa vào `<prior_artifacts>`.
- **Đổi loại Request:** Solution đang `proposed` thành `superseded`, không bị xoá.

### 2.5 Chẩn đoán, Findings, Answer (CR-008, theo báo cáo của agent)

- Cần đọc repo nên không dùng `ai.complete`. Dùng `agent.execPrompt` qua thành phần mới `AgentReadonlyRunner` trong `request-service`.
- **`spike` và `question` chưa chạy được khi không có worktree** trong code hiện tại: `ExecuteTask` luôn `EnsureWorktree`, `SimpleExecutor` từ chối đường dẫn rỗng và bỏ `stdout`. CR-008 đề xuất `AgentReadonlyRunner` với `cwd=repo_path`.
- **Chưa ép được chỉ đọc.** `agent.execPrompt` không có tham số chỉ đọc. CR chỉ giảm thiểu bằng prompt, che bí mật, và kiểm trạng thái repo trước và sau run. Cần quyết định chấp nhận cho v1 hay chờ thay đổi phía agent.

## 3. Lên Plan (CR-REQ-012)

### 3.1 Hình dạng Plan theo loại

| Loại | Cấu trúc tạo ra |
|---|---|
| `change_request` (mọi size), `bug`/`refactor` size L | Plan → Phase (≥1) → task làm việc |
| `bug`/`refactor` size S/M, `security`, `performance`, `ops_request` | Plan → task trực tiếp |
| `task`, `docs` | Một task `plan` làm vỏ danh sách → task. Approval `task_list` trỏ vào task vỏ này |
| `hotfix` | Đúng một task, không Plan (`plan_task_id` để trống, liên kết bằng `request_id` của task) |
| `spike`, `question` | Không có Plan (`REQUEST_PLAN_NOT_APPLICABLE`) |

Plan và Phase là bản ghi Task với `type=plan` và `type=phase` (quyết định D2). Nhãn do bước này sinh để các cổng sau kiểm tra:

| Nhãn | Dùng cho |
|---|---|
| `test:regression` | `bug`, `security`: bắt buộc có ít nhất một task |
| `gate:pre_deploy` | Bước không đảo ngược (`irreversible=true`) |
| `rollback` | `ops_request`: ít nhất một task |
| `check:baseline`, `check:after` | `performance`: đo đầu và đo cuối |
| `check:tests_before`, `check:tests_after` | `refactor` |

### 3.2 Hai bước, theo mẫu `AIDecompose` và `AIApply`

**PROPOSE** (không ghi gì):
1. Kiểm tra loại có Plan, Request đang ở `planning` (hoặc `analyzing` với `hotfix`).
2. Nạp Request, Solution đã `approved` (hoặc Chẩn đoán) và tech stack. Chưa duyệt thì `REQUEST_PLAN_SOLUTION_NOT_APPROVED`.
3. Gọi AI qua `ai.complete` (cùng ràng buộc dev server như mục 2.2) để sinh `PlanProposal`: Phase, task, phụ thuộc theo chỉ số, estimate, prompt template, nhãn.
4. Kiểm tra đề xuất, trả cho người dùng xem và **sửa tay**:

| Quy tắc | Mã lỗi |
|---|---|
| Có Phase khi không được phép | `REQUEST_PLAN_PHASES_NOT_ALLOWED` |
| Thiếu Phase khi bắt buộc | `REQUEST_PLAN_PHASES_REQUIRED` |
| Vừa Phase vừa task trực tiếp | `REQUEST_PLAN_MIXED_CHILDREN` |
| Quá lớn (đề xuất: 8 Phase, 20 task mỗi cấp, 100 task tổng) | `REQUEST_PLAN_TOO_LARGE` |
| Phụ thuộc sai hoặc có vòng | `REQUEST_PLAN_INVALID_DEPENDENCY` |
| `bug`/`security` thiếu test hồi quy | `REQUEST_PLAN_REGRESSION_TEST_MISSING` |
| Type task ngoài `task|bug|feature` | `REQUEST_PLAN_INVALID_TASK_TYPE` |

**COMMIT:**
1. Kiểm tra lại đề xuất (không tin dữ liệu từ client).
2. Gọi RPC mới `CreatePlanTree` ở `task-service`: tạo Plan, Phase, task, cạnh cha-con và cạnh phụ thuộc trong một transaction, cấp quyền chủ sở hữu theo cây cho người tạo. Thua đua thì trả `already_exists=true`.
3. Một transaction ở `request-service`: ghi `plan_task_id` (so sánh-và-ghi theo version), ghi sự kiện `plan.generated`.
4. Mở Approval: `plan`, hoặc `task_list`, hoặc `pre_deploy` (với `security`). `hotfix` không mở ở đây.
5. Chuyển Request sang `awaiting_plan_approval`.
6. Mỗi bước chạy lại an toàn nếu gián đoạn giữa chừng.

**Sinh lại Plan:** khi Request quay về `planning` mà đã có Plan, Approval cũ bị huỷ. `CreatePlanTree` kiểm không có task `in_progress` (`TASK_PLAN_HAS_RUNNING_TASKS`), rồi đặt Plan cũ và mọi con chưa `done` thành `cancelled`.

## 4. Thực thi Plan (CR-REQ-013, CR-REQ-014)

### 4.1 Điều kiện nền (đọc từ code `task-service`)

| Sự thật hiện có | Hệ quả |
|---|---|
| Chỉ `UpdateTask` phát sự kiện outbox; `ClaimForExecution`, `CompleteExecution`, `ReleaseExecution`, `UpdateStatus` không phát | `request-service` không biết task chạy xong hay lỗi. CR-013 bổ sung sự kiện, ghi cùng transaction, chỉ cho task có `request_id` |
| Run lỗi trả task về `previous_status` (không có trạng thái failed); `execution_links` không lưu thông điệp lỗi | Phải đưa `error_message` vào sự kiện |
| Run thành công đưa task lá về `review`, không `done`; chỉ `done` mới mở khoá task phụ thuộc | Cần cờ `REQUEST_AUTO_COMPLETE_TASKS` (mặc định bật) để tự đặt `done` |
| Mỗi task có worktree riêng (`task/<id>`) | Task sau không thấy code của task trước, xem 4.4 |
| Chỉ `ExecuteTask` được đặt `in_progress` | Trạng thái Plan và Phase suy ra từ con, không đặt tay |

### 4.2 Bắt đầu chạy

- **`change_request`:** người dùng duyệt từng Phase (Approval `phase`) rồi bấm "Bắt đầu Phase" (`StartPhase`). Hệ thống kiểm: Request đang `executing`; Phase thuộc Plan của Request; Phase đã được duyệt; mọi Phase mà nó phụ thuộc đã `done` hoặc `cancelled`. Việc bắt đầu ghi vào `phase_starts` nên gọi lặp không chạy trùng.
- **Loại không có Phase** (Plan trực tiếp, `task_list`, `hotfix`): Request vào `executing` thì tự chạy, không cần người bấm.

### 4.3 Điều phối (`AdvanceExecution`, idempotent)

1. Lấy các task lá còn `open` của container (task phụ thuộc chưa xong đang `blocked` nên không được chọn).
2. Trừ số task đang `in_progress` khỏi `REQUEST_MAX_PARALLEL_TASKS` (mặc định 1).
3. Hỏi cổng riêng theo loại. Chưa duyệt thì mở Approval và dừng task đó, không báo lỗi.
4. Gọi `Execute` có sẵn của `task-service`; `request-service` không tự chọn engine. Task có phụ thuộc đi Engine 2, task thường Engine 1, task gắn workflow đi Engine 3. Đường này đã hỗ trợ SSH/remote qua dev server.
5. Danh tính khi gọi là người duyệt Phase (hoặc người duyệt Plan nếu không có Phase); cần quyền `execute` trên task. Thiếu quyền thì `REQUEST_EXECUTE_FORBIDDEN`, không tự đổi danh tính. Cơ chế danh tính service-to-service chưa kiểm chứng.
6. Lỗi tạm thời khi dispatch (không có kết nối, tạo worktree lỗi) không tính vào số lần thử; thử lại tối đa 15 phút rồi trả backlog.

### 4.4 Worktree dùng chung và chạy tuần tự

Cả Plan dùng một worktree: task đầu để hệ thống tạo, sau đó `request-service` gán `worktree_id` đó cho các task còn lại. Hệ quả là chạy **tuần tự** (`REQUEST_MAX_PARALLEL_TASKS=1`, hiện là giá trị duy nhất an toàn). Chưa kiểm chứng: project-service có chấp nhận một worktree gắn nhiều task không, và hai run nối tiếp trong một worktree có đúng không.

### 4.5 Phản hồi ngược

`request-service` nhận sự kiện `orca.task.task.statuschanged` bằng consumer bền, khử trùng theo `event_id` (`processed_events`), bỏ qua task không có `request_id`.

| Sự kiện | Hành động |
|---|---|
| Task lá `review` sau khi chạy xong | Tự đặt `done` (nếu bật cờ) để mở khoá task sau |
| Task lá lỗi hoặc `recovery` | Đếm lần lỗi. Dưới 2 lần (mặc định) thì chạy lại; hết lần thì trả backlog |
| Mọi con của container bị huỷ | Trả backlog (`phase` hoặc `plan`) |
| Phase `done` (suy ra) | Phát `phase.completed`; còn Phase sau thì mở Approval `phase` cho Phase kế, Request vẫn `executing` |
| Hết Phase hoặc mọi task `done` | Chạy kiểm tra hoàn tất theo loại; đạt thì `execution_finished`, Request `completed`, phát `request.completed`; `hotfix` sinh thêm Request theo dõi |

Solution giữ `status=approved`, không ghi thêm. Kết quả nằm ở sự kiện `request.completed` và bảng `task_run_outcomes`.

### 4.6 Trả Request backlog khi lỗi

| Nguồn | Giai đoạn | Phân loại | Lý do mẫu |
|---|---|---|---|
| Task hết lần thử | `task` | `other` | `Task "<tiêu đề>" lỗi sau N lần: <lỗi cuối>` |
| Không dispatch được quá cửa sổ thử lại | `task` | `blocked_dependency` | `Không chạy được: <mã lỗi>` |
| Phase bị từ chối | `phase` | `rejected` | comment của người duyệt |
| Mọi task của container bị huỷ | `phase` hoặc `plan` | `other` | `Toàn bộ task đã huỷ` |
| Kiểm tra hoàn tất thất bại | `task` | `infeasible` (đo) hoặc `other` (test) | tên kiểm tra và tóm tắt |

Người thực hiện là hệ thống (`actor_kind=system`). **Task đang chạy khi Request bị trả hoặc huỷ vẫn chạy tới cùng**, vì chưa có RPC dừng run; kết quả của nó chỉ được ghi lại.

### 4.7 Đối soát

Vòng `ReconcileExecutingRequests` (mặc định 60 giây, khoá bằng `FOR UPDATE SKIP LOCKED` hoặc `GET_LOCK` của MySQL) xử lý Request `executing` không có sự kiện trong 5 phút. Nó chạy lại phân loại theo trạng thái hiện thời của task, là lưới an toàn cho sự kiện bị mất (ví dụ NATS không kết nối lúc khởi động, hoặc `ReleaseUnlinkedInProgress` hàng loạt không phát sự kiện).

### 4.8 Cổng và kiểm tra riêng theo loại (CR-014, theo báo cáo agent)

| Loại | Cổng hoặc kiểm tra |
|---|---|
| `hotfix`, `security` | Cổng `pre_deploy` trước khi chạy task fix; hotfix sinh Bug/Chore theo dõi sau khi xong |
| `ops_request` | Cổng trước mỗi bước nhãn `gate:pre_deploy`; phải có task `rollback` |
| `performance` | Đo baseline trước (`check:baseline`), đo lại sau (`check:after`) và so sánh |
| `refactor` | Test cũ phải xanh trước và sau (`check:tests_before`, `check:tests_after`) |

Orca không có bước deploy nên `pre_deploy` được hiểu là cổng duyệt trước khi chạy task có nhãn tương ứng, hoặc trước task fix của hotfix và security.

## 5. Điểm yếu và việc chưa kiểm chứng

| Điểm | Mức độ |
|---|---|
| Phụ thuộc dev server đang kết nối cho mọi bước dùng AI (Solution, Plan) | Cao: không có đường gọi LLM trực tiếp |
| Worktree dùng chung giữa các task, buộc chạy tuần tự | Cao: chưa kiểm chứng |
| Chưa ép được agent chỉ đọc khi chẩn đoán | Trung bình: chỉ giảm thiểu |
| `spike`, `question` chưa chạy được khi không có worktree | Trung bình: cần `AgentReadonlyRunner` |
| Task đang chạy không dừng được khi Request bị trả hoặc huỷ | Trung bình: chưa có RPC dừng run |
| Mọi giá trị mặc định (timeout 120 giây, 2 lần thử, 8 Phase, cửa sổ thử lại 15 phút, đối soát 60 giây) | Thấp: đề xuất, chưa đo |
| Quyền ghi ở mức Request (Request chưa có `Grant`) | Chưa chốt |
| `ai.complete` có trả JSON đúng khuôn và đủ nhanh với prompt cỡ Plan | Chưa kiểm chứng |
