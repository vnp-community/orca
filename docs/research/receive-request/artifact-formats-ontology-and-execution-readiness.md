# Định dạng, ontology, kiểm soát và điều kiện để AI thực thi được ngay

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-06 |
| **Loại** | Đề xuất thiết kế (chưa phải CR, chưa có code) |
| **Căn cứ code** | `task-service/internal/adapter/grpcclient/simple_executor.go` (`buildExecutePrompt`, `interpolateOutputs`), `task-service/internal/domain/agent_environment.go` (`BuildProjectContext`), `agent/src/relay/agent-print-mode-exec.ts`; schema trong CR-REQ-007 và CR-REQ-012; bảng `requests` ở README v6 |
| **Liên quan** | [openspec-and-ai-tooling-integration.md](./openspec-and-ai-tooling-integration.md), [ai-steps-and-dev-server-connection-flows.md](./ai-steps-and-dev-server-connection-flows.md), [solution-plan-execution-walkthrough.md](./solution-plan-execution-walkthrough.md), [CR v6](../../crs/v6/README.md) |

> Mọi thứ trong file này là đề xuất mới, chưa nằm trong CR v6. Các điểm chưa kiểm chứng nằm ở mục 11.

## Phần A. Định dạng và ontology để tích hợp nhiều công cụ

### 1. Nguyên tắc

1. **Một mô hình chuẩn duy nhất (JSON, có `schema_version`) nằm ở `request-service`.** Đây là nguồn sự thật.
2. **Mỗi công cụ chỉ là bản chiếu (projection) của mô hình chuẩn.** OpenSpec và Spec Kit đọc và ghi bản Markdown. Adapter chuyển qua lại giữa mô hình chuẩn và định dạng của công cụ.
3. **Bản Markdown có phần mở đầu YAML cố định** (`id`, `type`, `status`, `version`, `refs`, `digest`) và các đề mục cố định, để công cụ và người cùng đọc.
4. **Mọi đối tượng có ID ổn định và quan hệ tường minh**, để đối chiếu và truy vết giữa các công cụ.

### 2. Ontology

| Thực thể | ID đề xuất | Ghi chú |
|---|---|---|
| Request | `REQ-<số>` | Số theo tenant (`request_counters`, CR-REQ-002) |
| Solution, Option | `SOL-<số>`, `SOL-<số>.opt-N` | Option đã có `opt-N` ở CR-REQ-007 |
| Plan, Phase | `PLN-<số>`, `PH-<số>.<n>` | Là Task `type=plan`/`phase` (quyết định D2) |
| Task | `TSK-<số>` | Task làm việc dùng id thật của `task-service` |
| Approval | id thật | Duyệt một đối tượng ở một phiên bản |
| Clarification (mới) | `CLR-<số>` | Câu hỏi bổ sung dữ liệu |
| Decision (mới) | `DEC-<số>` | Bản ghi một lựa chọn đã xác nhận |
| Evidence (mới) | `EVD-<số>` | Bằng chứng: file ảnh hưởng, kết quả quét, tài liệu |
| Check | `check:*` | Điều kiện kiểm chứng được |

**Quan hệ chuẩn:** `derived_from` (Solution từ Request), `implements` (Plan thực hiện một Option), `contains` (Plan chứa Phase, Phase chứa Task), `depends_on`, `blocks`, `verifies` (Check kiểm Task hoặc tiêu chí), `supersedes` (phiên bản mới thay cũ), `spawned_by` (Request con), `evidenced_by`.

### 3. Nội dung bắt buộc của từng loại

**Request.** Bắt buộc với mọi loại: `title`, `body` (phát biểu vấn đề), `source`, `reporter`, **tiêu chí chấp nhận** (danh sách, mỗi tiêu chí có ID `AC-1`...). Tiêu chí chấp nhận là trường mới, README v6 chưa có.

| Loại | Trường bắt buộc thêm |
|---|---|
| `bug` | Các bước tái hiện, kết quả thực tế và mong đợi, môi trường, mức nghiêm trọng |
| `change_request`, `refactor` | Mục tiêu, giá trị đem lại, phạm vi bao gồm và loại trừ |
| `security` | Thành phần bị ảnh hưởng, mức độ khai thác, dữ liệu bị lộ |
| `performance` | Chỉ số đo, số đo hiện tại, mục tiêu |
| `ops_request` | Môi trường đích, cửa sổ thực hiện, cách quay lui |
| `hotfix` | Tác động production, thời điểm bắt đầu |
| `spike`, `question` | Câu hỏi cần trả lời, giới hạn thời gian (spike) |
| `docs` | Đối tượng đọc, phạm vi |

**Solution** (bổ sung vào schema CR-REQ-007). Giữ `options[]` hiện có. Thêm: bảng phủ yêu cầu (mỗi `AC-n` được phương án nào đáp ứng), ràng buộc và yêu cầu phi chức năng, chiến lược kiểm thử, danh sách bằng chứng (`evidence_refs`), và `decision` (xem mục 6).

**Plan và Phase.** Plan: mục tiêu, phạm vi, rủi ro, cách quay lui, chiến lược kiểm chứng, liên kết tới Option đã chọn. Phase: mục tiêu, **điều kiện vào và điều kiện ra** (kiểm chứng được), phụ thuộc Phase khác.

**Task.** Chi tiết ở mục 7 (TaskSpec). `Task` ở `task-service` hiện chỉ có `Description`, `AIContext`, `PromptTemplate`, `Labels`, chưa có cột cho tiêu chí hoàn thành và Check; cần thêm hoặc lưu JSON có cấu trúc.

### 4. Định dạng trao đổi với công cụ

| Công cụ | Chiếu từ mô hình chuẩn |
|---|---|
| OpenSpec | Request, Solution → `proposal.md` và `design.md`; yêu cầu hành vi → delta spec; Plan, Phase, Task → `tasks.md` |
| Spec Kit | Request → spec; Solution → plan; Task → tasks |
| CodeGraph, GitNexus, Semgrep | Không chiếu; là **nguồn Evidence** (`EVD-n`) ghi vào Solution |
| Skill, hook của Claude Code | Chỉ dẫn theo loại Request, đọc từ bản Markdown trong repo |
| MCP | Cùng mô hình chuẩn, mở ra dạng resource đọc và tool ghi |

Quy tắc phân tích đầu ra của công cụ: chỉ nhận khối có cấu trúc (phần mở đầu YAML hoặc khối JSON trong fence) và các đề mục cố định; ngoài ra là văn bản tự do, không dùng để suy ra dữ liệu.

### 5. Kiểm soát

| Lớp | Nội dung |
|---|---|
| **Cấu trúc** | Kiểm theo JSON Schema có phiên bản. Sai thì từ chối và thử lại một lần (như CR-REQ-007) |
| **Ngữ nghĩa** | Không có vòng phụ thuộc; mỗi `AC-n` có ít nhất một task phủ và một Check kiểm; số phương án và số task trong giới hạn; `bug`/`security` có test hồi quy |
| **Chính sách theo loại** | Cổng bắt buộc theo registry của CR-REQ-003; không bỏ cổng khi chưa đủ dữ liệu |
| **Phiên bản và bất biến** | Đối tượng đã duyệt không sửa tại chỗ; sửa tạo phiên bản mới, bản cũ thành `superseded`. Approval gắn `digest` của đúng phiên bản (đã có ở CR-REQ-007 và 009) |
| **Nguồn gốc** | Mỗi Solution và Plan ghi: công cụ sinh, model, phiên bản prompt, `run_id`, `digest` đầu vào, thời điểm |
| **Ranh giới tin cậy** | Nội dung Request là dữ liệu không tin cậy: đặt trong khối có rào, không đưa bí mật vào prompt, không để nội dung đó thành chỉ thị (đã có ở CR-REQ-007) |
| **Kích thước** | Giới hạn theo trường (đã có một phần ở CR-REQ-007 và 012) |
| **Kiểm toán và truy vết** | Ghi ai sinh, ai chọn, ai duyệt, từ chối; bảng phủ yêu cầu được tính lại mỗi phiên bản |
| **Đo lường** | Tỉ lệ sai schema, số lần thử, tỉ lệ bị từ chối theo loại, thời gian đến duyệt |

### 6. Bổ sung dữ liệu và xác nhận lựa chọn

#### 6.1 Yêu cầu bổ sung dữ liệu (Clarification)

Hiện **chưa có trạng thái nào cho việc hỏi lại**. CR v6 chỉ có trả về Request backlog với lý do thiếu thông tin, tức là dừng hẳn, không có vòng hỏi đáp.

Đề xuất:
1. **Kiểm tra sẵn sàng của Request** ngay sau phân loại, so với danh sách trường bắt buộc theo loại (mục 3). Thiếu thì tạo Clarification, không đi tiếp.
2. **Clarification** gồm danh sách câu hỏi có kiểu: văn bản, chọn một, chọn nhiều, tệp đính kèm, xác nhận có/không. Mỗi câu có lý do hỏi, mặc định đề xuất (nếu có) và đối tượng trả lời (người báo cáo hoặc người khác).
3. **Trạng thái mới `awaiting_information`** trong máy trạng thái Request, có hạn trả lời. Hết hạn thì về Request backlog với phân loại `missing_info`.
4. **Câu trả lời tạo phiên bản mới của Request** (không ghi đè), rồi chạy lại bước đang chờ (phân loại, Solution hoặc Plan).
5. **Nguồn câu hỏi:** kiểm tra sẵn sàng; `open_questions` của Solution; `assumptions` của Plan cần xác nhận; task bị chặn vì thiếu dữ liệu khi thực thi (xem mục 9).

#### 6.2 Xác nhận lựa chọn (Decision)

Approval hiện chỉ duyệt hoặc từ chối một đối tượng. Chọn phương án mới có ở Solution (`chosen_option`). Đề xuất bản ghi **Decision** dùng chung cho mọi lựa chọn:

| Trường | Ý nghĩa |
|---|---|
| `question` | Quyết định cần đưa ra |
| `options[]` | Các lựa chọn, kèm lựa chọn được đề xuất và lý do |
| `chosen`, `chooser`, `at` | Lựa chọn, người chọn, thời điểm |
| `rationale` | **Bắt buộc khi chọn khác với đề xuất** |
| `subject_digest` | Gắn với đúng phiên bản đang xem (chống chọn nhầm bản cũ) |

Quy tắc kiểm soát:
- Lựa chọn **rủi ro cao** (`breaking_change`, không đảo ngược, ảnh hưởng nhiều dịch vụ) cần xác nhận lần hai rõ ràng, ví dụ gõ lại tên phương án.
- Không cho duyệt Plan khi Solution tương ứng chưa có Decision.
- Quyền chọn theo chính sách của CR-REQ-010; tự chọn của người báo cáo có thể bị chặn theo cấu hình.
- Chọn lại được khi Approval còn `pending` (đã có ở CR-REQ-007); mỗi lần chọn ghi lịch sử.

## Phần B. Điều kiện để task gửi đi là AI thực thi được ngay

### 7. Hiện trạng: không có gì đảm bảo task thực thi được

Đã đọc code. Prompt thực thi do `buildExecutePrompt` (`simple_executor.go:497`) dựng bằng cách nối: `PromptTemplate` (hoặc câu "Complete the following task."), `Title`, `Description`, `AIContext`, tiêu đề và mô tả của task cha, tiêu đề và mô tả của các phụ thuộc đã xong. Token `{{outputs.<id>.*}}` được thay bằng `LastExecutionOutput` của phụ thuộc. Phần bối cảnh dự án do `BuildProjectContext` thêm (tên dự án, mô tả, repo, thư mục làm việc, nhánh, dev server, người dùng, team).

Agent (`agent.execPrompt`) nhận `prompt`, `worktreePath`, `trustPreset`, `model`, `env`, `timeoutMs` và chỉ trả `{stdout, stderr, exitCode, timedOut}`. `stdout` được lưu tối đa 8 KB cho task sau.

Hệ quả: không có chỗ nào ràng buộc **phạm vi**, **tiêu chí hoàn thành**, **cách kiểm chứng**, hay **đầu ra có cấu trúc**. Cần ba thứ: hợp đồng thực thi của task, cổng sẵn sàng trước khi gửi, và hợp đồng kết quả kèm kiểm chứng độc lập sau khi chạy.

### 8. Hợp đồng thực thi của task (TaskSpec)

Mỗi task làm việc phải mô tả đủ để agent chạy mà không cần hỏi lại. Task `plan` và `phase` không được thực thi nên không cần.

| Nhóm | Trường | Quy tắc |
|---|---|---|
| Mục tiêu | `objective` | Một câu mệnh lệnh, một thay đổi kiểm chứng được |
| Truy vết | `request_id`, `solution_option`, `plan_id`, `phase_id`, `satisfies[]` (các `AC-n`) | Mỗi task phải phủ ít nhất một `AC-n` |
| Phạm vi | `scope.include[]`, `scope.exclude[]` (đường dẫn hoặc glob), `scope.max_files` | Đường dẫn phải tồn tại hoặc được khai là sẽ tạo |
| Ràng buộc | `constraints[]` | Quy ước code, không đụng tới, yêu cầu bảo mật |
| Đầu vào | `inputs[]` có tên, kiểu và nguồn (task phụ thuộc hoặc file) | Thay cho `{{outputs.<id>.*}}` thô |
| Cách làm | `approach` (gợi ý, không bắt buộc) | Tách khỏi mục tiêu |
| Tiêu chí hoàn thành | `acceptance[]`, mỗi mục kiểm chứng được | Không dùng từ mơ hồ ("tốt", "hợp lý") |
| Kiểm chứng | `checks[]`: `id`, `kind` (`command`, `test`, `lint`, `typecheck`, `diff_rule`), `command`, `expect` (exit code hoặc mẫu), `timeout` | Ít nhất một Check cho mỗi task |
| Đầu ra khai báo | `outputs[]` có tên và kiểu (ví dụ `api_schema`, `file_list`) | Task sau đọc theo tên |
| Năng lực cần | `requires.tools[]`, `requires.env_names[]` (chỉ tên biến, không giá trị) | Dùng cho cổng sẵn sàng |
| Rủi ro | `irreversible`, `needs_approval_before` | Đã có nhãn `gate:pre_deploy` |
| Ngân sách | `estimate_minutes`, `timeout_minutes` | Không vượt 15 phút (giới hạn tối đa của `agent.execPrompt`) |
| Điều kiện dừng | `stop_conditions[]` | Khi nào hỏi lại thay vì làm tiếp |

Ví dụ rút gọn:

```yaml
id: TSK-412
objective: "Thêm cột `jira_site_id` vào bảng projects và cập nhật repository"
satisfies: [AC-2]
scope: { include: ["backend-go/services/project-service/**"], exclude: ["**/*_test.go"], max_files: 8 }
constraints: ["Migration phải có bản down", "Chạy được trên Postgres và MySQL"]
acceptance: ["Migration up/down chạy sạch trên cả hai DB", "Repository trả về trường mới"]
checks:
  - { id: c1, kind: command, command: "go test ./services/project-service/...", expect: { exit: 0 } }
  - { id: c2, kind: diff_rule, rule: "changed_files subset_of scope.include" }
requires: { tools: [go, git], env_names: [DATABASE_DSN] }
timeout_minutes: 12
stop_conditions: ["Thiếu quyết định về kiểu cột", "Test hiện có đã đỏ trước khi sửa"]
```

### 9. Gói thực thi (ExecutionPacket): sinh bằng quy tắc, không bằng AI

Prompt gửi cho agent do hệ thống **render xác định** từ dữ liệu có cấu trúc, không để AI viết lại ở bước thực thi. AI chỉ viết nội dung `TaskSpec` một lần ở bước Plan, và nội dung đó được kiểm tra.

Các mục cố định của gói, theo thứ tự:
1. **Mục tiêu** và tiêu chí hoàn thành.
2. **Bối cảnh rút gọn:** tóm tắt Request, quyết định đã chọn của Solution (kèm lý do), mục tiêu của Plan và Phase.
3. **Phạm vi và ràng buộc** (được phép, bị cấm).
4. **Đầu vào:** giá trị các `outputs` có tên của task phụ thuộc, có cắt độ dài và đánh dấu nguồn.
5. **Cách kiểm chứng:** danh sách Check agent phải tự chạy trước khi kết thúc.
6. **Hợp đồng kết quả** (mục 11).
7. **Điều kiện dừng:** làm gì khi thiếu thông tin.

Nội dung từ Request, Jira, GitHub nằm trong khối có rào và được đánh dấu là dữ liệu không tin cậy, giống CR-REQ-007. Gói có `digest` và phiên bản template để truy vết.

### 10. Cổng sẵn sàng thực thi chạy trước mỗi lần `Execute`

Tự động, chạy bằng `agent.exec`, `fs.*` và truy vấn dữ liệu. Không đạt thì **không gửi** task. Ba tầng:

| Tầng | Kiểm tra | Cách kiểm |
|---|---|---|
| **Cấu trúc** | `TaskSpec` đúng schema; mọi trường bắt buộc có; mỗi `AC-n` của Request có ít nhất một task phủ; mỗi task có Check | Schema và bảng phủ yêu cầu |
| **Ngữ nghĩa** | Đường dẫn trong `scope` tồn tại hoặc được khai tạo mới; lệnh trong Check có thật (script trong `package.json`, `Makefile`, target `go test`); phụ thuộc đã `done`; kích thước task không vượt ngân sách (số file, phút) | `fs.stat`, `fs.glob`, `command -v`, đọc script |
| **Môi trường** | Dev server đang kết nối; `claude` đã đăng nhập; công cụ trong `requires.tools` có mặt; biến trong `requires.env_names` được thiết lập (chỉ kiểm có hay không); worktree sạch, đúng nhánh gốc, đã cập nhật; Check nền (build, test hiện có) xanh **trước** khi sửa | `agent.exec` |

Kết quả cổng:

| Kết quả | Ý nghĩa | Xử lý |
|---|---|---|
| `ready` | Đủ điều kiện | Gửi |
| `needs_info` | Thiếu dữ liệu | Tạo Clarification (mục 6.1) |
| `spec_defect` | Spec sai | Trả về bước Plan, sinh lại task |
| `env_defect` | Môi trường lỗi | Báo người vận hành, không đốt lần thử |

Mỗi kết quả ghi lý do có cấu trúc. **Nguyên tắc chia task:** một task là một thay đổi kiểm chứng được trong ngân sách thời gian. Plan ước lượng vượt ngưỡng thì chia nhỏ, không đưa vào thực thi.

### 11. Hợp đồng kết quả và kiểm chứng độc lập

Agent phải kết thúc bằng một khối JSON ở cuối đầu ra (ngoài văn bản tự do):

```json
{ "status": "done|blocked|failed|needs_info",
  "summary": "...",
  "files_changed": ["..."],
  "checks_run": [{"id":"c1","exit":0}],
  "outputs": {"api_schema": "..."},
  "questions": ["..."], "notes": "..." }
```

Orca **không tin lời agent**. Sau khi chạy:
1. **Phân tích khối kết quả.** Không có hoặc sai schema thì coi là thất bại cấu trúc (thử lại một lần kèm nhắc định dạng).
2. **Tự chạy lại Check** bằng `agent.exec` và so với `expect`.
3. **Kiểm phạm vi:** `git diff` so với `scope`; file ngoài phạm vi hoặc vượt `max_files` thì từ chối.
4. **Quét bí mật** trong diff.
5. Mọi Check đạt thì task sang `review` (hoặc `done` nếu bật tự hoàn tất); không thì phân loại lỗi.

Phân loại lỗi để định tuyến:

| Loại | Dấu hiệu | Xử lý |
|---|---|---|
| `retryable` | Hết thời gian, lỗi tạm thời | Chạy lại, có giới hạn lần |
| `needs_info` | Agent trả `needs_info` hoặc chạm điều kiện dừng | Tạo Clarification, Request về `awaiting_information` |
| `spec_defect` | Check sai, phạm vi không đúng, tiêu chí mâu thuẫn | Trả về bước Plan, sinh lại task |
| `env_defect` | Thiếu công cụ, chưa đăng nhập, Check nền đỏ | Báo vận hành, không tính lần thử |
| `agent_defect` | Ra ngoài phạm vi, bỏ qua Check, đầu ra sai | Thử lại có phản hồi, hết lần thì Request backlog |

### 12. Ontology bổ sung cho phần thực thi

Thêm vào danh sách ở mục 2: `TaskSpec`, `Check`, `Constraint`, `ScopeRule`, `NamedOutput` (đầu ra có tên và kiểu), `ExecutionPacket` (bản render, có `digest`), `ExecutionResult` (khối kết quả đã phân tích), `ReadinessReport` (kết quả cổng), `Failure` (có `class` theo bảng ở mục 11).

Quan hệ thêm: `satisfies` (Task → AC), `verifies` (Check → Task hoặc AC), `consumes` và `produces` (Task ↔ NamedOutput), `compiled_from` (Packet → TaskSpec và các ancestor), `reported_by` (Result → Run), `blocked_by`.

### 13. Quy trình kiểm soát tổng thể

```
Plan sinh TaskSpec ──▶ kiểm cấu trúc và phủ yêu cầu ──▶ người duyệt Plan (Approval)
                                  │ lỗi → sinh lại
                                  ▼
          [trước mỗi Execute] Cổng sẵn sàng: cấu trúc → ngữ nghĩa → môi trường
                                  │ needs_info / spec_defect / env_defect → định tuyến
                                  ▼
          Render ExecutionPacket (xác định, có digest) ──▶ agent.execPrompt
                                  ▼
          Phân tích khối kết quả ──▶ chạy lại Check ──▶ kiểm phạm vi ──▶ quét bí mật
                                  ▼
          Phân loại lỗi và định tuyến  hoặc  review/done
```

**Đo lường để cải thiện:** tỉ lệ task qua cổng ngay lần đầu, tỉ lệ `spec_defect` theo loại Request, tỉ lệ khớp giữa lời agent và kết quả Check độc lập, số lần thử trung bình. Dùng một bộ task mẫu cố định (kiểu Promptfoo) để so sánh khi đổi prompt Plan.

## Phần C. Khoảng trống và bước tiếp

### 14. Các chỗ cần thay đổi so với hiện trạng

**Với CR v6 hiện có:**

| Khoảng trống | CR liên quan |
|---|---|
| Không có trạng thái và thực thể hỏi lại (`awaiting_information`, Clarification) | 003, 006 |
| `requests` thiếu tiêu chí chấp nhận và trường bắt buộc theo loại | 002, 004 |
| Chưa có Decision và Evidence | 007, 008, 009 |
| Chưa có bảng phủ yêu cầu (`AC-n` → task → Check) và kiểm tra liên quan | 007, 012, 014 |
| Solution và Plan chưa ghi nguồn gốc (model, phiên bản prompt, digest đầu vào) | 007, 012 |
| Task chưa có chỗ cho tiêu chí hoàn thành và Check có cấu trúc | 011, 012 |
| Chưa có bản chiếu Markdown/YAML và bộ adapter | mới |
| Chưa có JSON Schema có phiên bản làm hợp đồng chung | mới |

**Với code thực thi hiện tại:**

| Hiện có | Cần thêm |
|---|---|
| `Task` chỉ có `Description`, `AIContext`, `PromptTemplate`, `Labels` | Chỗ lưu `TaskSpec` có cấu trúc (cột JSON có schema, hoặc bảng riêng) |
| Prompt dựng bằng nối chuỗi (`buildExecutePrompt`) | Bộ render `ExecutionPacket` xác định, có phiên bản template |
| Đầu ra chỉ `stdout` cắt 8 KB | Khối kết quả có cấu trúc và `NamedOutput` |
| `{{outputs.<id>.*}}` thay thẳng stdout | Tham chiếu `outputs` có tên và kiểu |
| `trustPreset=full` bật cờ bỏ qua hỏi quyền | Chỉ bật khi `scope` và Check đã qua cổng; kiểm phạm vi sau chạy là bắt buộc |
| Không kiểm tra trước khi `Execute` | `ReadinessGate` ở `request-service` gọi `agent.exec` và `fs.*` |
| Không phân loại lỗi; task lỗi về `previous_status` | `Failure.class` và định tuyến (đã có một phần trong CR-REQ-013) |
| `agent.execPrompt` bắt buộc `worktreePath`, chỉ `claude` | Giữ; ghi rõ là giới hạn của cổng môi trường |

### 15. Đề xuất CR

| CR | Nội dung | Phụ thuộc |
|---|---|---|
| **CR-REQ-027** | Lược đồ và ontology: JSON Schema có phiên bản cho Request, Solution, Plan, Task; ID và quan hệ; bản chiếu Markdown/YAML; quy tắc phân tích; kiểm tra phủ yêu cầu; trường nguồn gốc | CR-002, 007, 012 |
| **CR-REQ-028** | Hỏi lại và ghi nhận quyết định: Clarification, Decision, trạng thái `awaiting_information`, kiểm tra sẵn sàng của Request, quy tắc xác nhận rủi ro cao | CR-003, 006, 009 |
| **CR-REQ-029** | Hợp đồng thực thi và cổng sẵn sàng: `TaskSpec`, bộ render `ExecutionPacket`, `ReadinessGate`, hợp đồng kết quả, phân loại lỗi; sửa CR-011, 012, 013, 014 | CR-027, 028 |
| CR-REQ-026 | OpenSpec solution engine (đã đề xuất ở file openspec) | CR-027 |

Thứ tự: 027 → 028 → 029 → 026. CR-027 làm trước vì adapter OpenSpec và cổng sẵn sàng đều phụ thuộc định dạng chuẩn.

### 16. Chưa kiểm chứng

| Điểm | Trạng thái |
|---|---|
| Agent có luôn trả được khối JSON ở cuối đầu ra ổn định không | Chưa kiểm chứng; cần thử với `claude --print` |
| Thời gian chạy cổng sẵn sàng và độ nhanh của `fs.stat`, `agent.exec` khi chạy trước mỗi task | Chưa đo |
| Chạy lại Check bằng `agent.exec` có cùng môi trường với lần agent chạy (cùng worktree, cùng biến môi trường) không | Chưa kiểm chứng |
| Quét bí mật trong diff: dùng công cụ nào, có sẵn trên dev server không | Chưa chọn |
| Giới hạn 15 phút của `agent.execPrompt` có đủ cho task thực tế không | Chưa đo |
| Cách lưu `TaskSpec` (cột JSON hay bảng riêng) và ảnh hưởng tới hai dialect Postgres và MySQL | Chưa thiết kế |
| Các giá trị mặc định trong file này (ngưỡng, số lần thử, ngân sách) | Đề xuất, chưa đo |
