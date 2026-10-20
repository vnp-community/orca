# Luồng Request trong Orca: hướng dẫn người dùng, quản trị và vận hành

**Cập nhật:** 2026-10-09 · Đối chiếu với: `backend-go/services/request-service` (README, `internal/domain/request_flow_registry.go`,
`internal/adapter/grpc/flow_gate.go`, `cmd/server/main.go` và `cmd/server/wire_*.go`), bộ e2e `tests/stages/e2e/orca-flows`
(mỗi dòng trạng thái dưới đây ghi test chứng minh nó), [CR v6](../../crs/v6/README.md),
[CR-REQ-024](../../crs/v6/request-quality-rollout/CR-REQ-024-jira-status-sync-audit-observability.md),
[CR-REQ-025](../../crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md).

Một **Request** là một yêu cầu công việc (từ Jira, GitHub, nhập tay, agent qua MCP, webhook) đi qua một luồng có cổng duyệt
theo loại của nó. Tài liệu này chỉ mô tả điều **đã có trong mã**; mỗi giai đoạn ghi rõ phần nào đã được bộ e2e chạy thật trên stack Docker và phần nào mới có mã.

## Trạng thái hiện thực (đọc trước)

Luồng chạy từ tạo Request tới lập Plan và mở cổng duyệt Plan. "E2E" là bộ `tests/stages/e2e/orca-flows` chạy qua WebSocket của gateway trên stack
Docker thật; "chỉ có mã" nghĩa là handler đã nối và có test đơn vị/tích hợp nhưng **chưa** được chạy xuyên các tầng.

| Giai đoạn | Trong mã | E2E | Ghi chú |
|---|---|---|---|
| Tạo, phân loại, xác nhận / đổi loại (`CreateRequest`, `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType`, `ReopenRequest`, `ReturnToBacklog`, `CancelRequest`) | có | `RQL-01..20`, `MTX-01..13` (đủ 11 loại), `EXT-01..11` | AI đề xuất loại chạy nền; luôn kết thúc ở `awaiting_type_confirmation` kể cả khi AI hỏng |
| Làm rõ (Clarification: `ListClarifications`, `AnswerClarification`, `GetRequestReadiness`) | có | `RQL-14..17` | câu hỏi mặc định theo loại; nháp, hoàn tất, không ghi đè |
| Solution kiểu `solution` (`change_request`, `refactor`): `GenerateSolution`, `ListSolutions`, `ChooseSolutionOption`, duyệt cổng `solution` | có, chạy qua `ai.complete` của agent | `AIS-02`, `PLN-08` | cần dev server có AI (Claude Code đã đăng nhập hoặc khoá API); chạy nền, trả `runId` rồi `solution.list` |
| Chẩn đoán, Findings, Answer (`bug`, `hotfix`, `security`, `performance`, `spike`, `question`) | có, agent đọc repo chỉ-đọc (`agent.execPrompt`, `claude --print` với chế độ plan và công cụ `Read,Glob,Grep`) | `RAN-01..03` (`bug`, `spike`, `question`) với `claude` giả; với Claude thật chỉ khi đặt `ORCA_E2E_READONLY_ANALYSIS=1` (tốn lượt AI, **chưa chạy**); `hotfix`, `security`, `performance` chưa có test riêng | tài liệu đúng `kind`, Request dừng ở `awaiting_analysis_approval`, cổng duyệt `solution`/`findings`/`answer` đang chờ; trả lời Clarification chuẩn bị tự khởi động lượt phân tích |
| Plan: `GeneratePlan` (đề xuất, chạy nền), `GetPlanProposal`, `CommitPlan` | có | `PLN-01..08`, `AIS-03/04` | đề xuất không ghi gì; commit ghi cây Plan qua task-service, chạy lại an toàn; `change_request` đi theo Phase |
| Cổng duyệt Plan (`plan`, `task_list`, `pre_deploy`, `phase`) | có (`PlanArtifacts`, `PhaseArtifacts`, `PreDeployArtifacts`) | mở và **từ chối** Approval (`PLN-04..06`, `PLN-08`) | duyệt Plan thì agent AI chạy trên máy dev nên bộ e2e không bao giờ duyệt |
| Thực thi: `StartPhase`, `ReportTaskOutcome`, `RecordRequestCheck`, trả kết quả task về Request | có | **chưa** (`TSK-08` tuỳ chọn, `ORCA_E2E_EXECUTE_AGENT=1`) | cần một lượt chạy tay có chủ đích |
| Backlog (`ListBacklog`; kênh `backlog.requests`, `backlog.tasks`, `backlog.execute`) | có | `AIS-06` (chỉ kiểm kênh trả lời) | nội dung từng view chưa được e2e kiểm |
| Quyết định (Decision), artifact, tuyên bố nguồn (provenance), kiểm tra bao phủ AC | có | gián tiếp (Solution của `AIS-02`/`PLN-08` qua kiểm tra bao phủ) | |
| Impact, rủi ro (`AcceptRisk`, `GetRiskPolicy`), kiểm tra readiness (`CheckReadiness`, `GetReadinessReport`), nguồn ngữ cảnh (`ListContextSources`...), evidence, `ExportTenantRequests`, `AiBudgetAdminService` | **chưa có**: không có handler, trả `Unimplemented` | không | CR tương ứng chưa làm |
| Cờ theo project: engine `openspec` cho Plan và Solution bằng agent có tri thức (`GetProjectEngineSettings`, `SetProjectEngineSettings`; `orca engine show|set`) | có, mặc định tắt; chỉ admin đổi; kiểm dev server (`--check`) không chặn việc lưu | **chưa** (chưa chạy qua stack Docker) | [knowledge-hub-setup.md](knowledge-hub-setup.md) |
| Outcome Review (`GenerateOutcomeReview`, `GetOutcomeReview`, quyết định, duyệt, sinh lại hình, hồ sơ vẽ; kênh `request.review.*`; `orca engine set --outcome-review`) | có, mặc định tắt; admin bật theo project; agent MCP chỉ đọc | bộ `outcome_review` (`ORV-01..09`, `ORV-20` tốn tiền) **chưa chạy trên stack**, xem hướng dẫn | [outcome-review-setup.md](outcome-review-setup.md) |

AI khi không dùng được trả mã có cấu trúc (`REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_AI_SESSION_EXPIRED`,
`REQUEST_AI_QUOTA_EXHAUSTED`, `REQUEST_AI_BUSY`); Request đứng yên ở trạng thái hiện tại (`AIS-01`, `AIS-03`).

Khi AI đề xuất loại cần người duyệt (`hotfix`, `security`) mà tenant không có người duyệt hợp lệ khác người báo (ví dụ chỉ có một user),
Approval `request_type` không mở được (`REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER`). Phân loại vẫn chuyển Request sang `awaiting_type_confirmation` và
người báo xác nhận loại trực tiếp; test `AST-05` (chạy với `--ai-stub`) giữ bất biến này.

## Ai cần đọc gì

| Bạn là | Đọc |
|---|---|
| Người tạo Request, cần phân loại | [Tạo và phân loại Request](./creating-and-classifying-requests.md) |
| Người duyệt | [Phê duyệt Request](./approving-requests.md) |
| Cần hiểu backlog | [Backlog của Request và Task](./request-and-task-backlogs.md) |
| Dùng agent / MCP | [Request từ agent qua MCP](./requests-from-agents-mcp.md) |
| Admin tenant / ops muốn bật luồng | [Bật luồng Request](./admin-enable-request-flow.md) |
| Trực vận hành | [Runbook](./runbook-request-flow.md) |
| Đồng bộ Jira | [Jira ↔ Orca](../jira/jira-orca-mapping.md) (mục "Request sở hữu issue") |

## 11 loại Request

`change_request`, `bug`, `hotfix`, `task`, `spike`, `question`, `refactor`, `security`, `performance`, `docs`, `ops_request`.
Bảng luồng theo loại nằm ở `internal/domain/request_flow_registry.go` (`flowDefinitions`):

| Loại | Phân tích | Kế hoạch | Phase | Cổng đầu thực thi | Ghi chú |
|---|---|---|---|---|---|
| `change_request` | Solution (duyệt `solution`) | Plan | luôn | `plan` | cổng `phase` trong lúc thực thi |
| `bug` | Chẩn đoán (duyệt `solution`) | Plan | khi size L | `plan` | |
| `hotfix` | Chẩn đoán, không cổng | một task | không | `pre_deploy` | bắt buộc người xác nhận loại; phải `urgent` |
| `task` | không | danh sách task | không | `task_list` | |
| `spike` | Findings (duyệt `findings`) | không | không | không | xong sau phân tích |
| `question` | Answer (duyệt `answer`) | không | không | không | xong sau phân tích |
| `refactor` | Solution | Plan | khi size L | `plan` | |
| `security` | Chẩn đoán (duyệt `solution`) | Plan | không | `pre_deploy` | bắt buộc người xác nhận loại; cần lý do |
| `performance` | Chẩn đoán | Plan | không | `plan` | |
| `docs` | không | danh sách task | không | `task_list` | |
| `ops_request` | không | Plan | không | `plan` | thêm cổng `pre_deploy` trong lúc thực thi |

Các cột Solution, Plan, Phase là **thiết kế của registry**; `MTX-01..13` kiểm trạng thái đầu và đường đi của từng loại theo bảng này. Phần nào của cột đã chạy thật xem bảng trạng thái ở trên.

## 11 trạng thái

`new` → `classifying` → `awaiting_type_confirmation` → `analyzing` → `awaiting_analysis_approval` → `planning` →
`awaiting_plan_approval` → `executing` → `completed`; ngoài ra `request_backlog` và `cancelled`.

| Trạng thái | Nghĩa |
|---|---|
| `new` | vừa tạo |
| `classifying` | AI đang đề xuất loại, size, urgency (chạy nền) |
| `awaiting_type_confirmation` | có đề xuất, chờ người xác nhận (cổng `request_type`) |
| `analyzing` | đang phân tích: Solution / chẩn đoán chạy nền, tạo bằng `GenerateSolution` |
| `awaiting_analysis_approval` | chờ duyệt Solution/Findings/Answer |
| `planning` | đang lập kế hoạch: `GeneratePlan` đề xuất, `CommitPlan` ghi |
| `awaiting_plan_approval` | chờ duyệt Plan/task list/pre-deploy |
| `executing` | đang thực thi các Phase/task (có mã, chưa được e2e chạy) |
| `completed` | xong |
| `request_backlog` | trả về backlog kèm lý do |
| `cancelled` | đã huỷ |

## Ba view backlog

| View | Định nghĩa (thiết kế, README v6 mục 3.8) | Tình trạng |
|---|---|---|
| Request backlog | `requests.status = request_backlog` | có: trạng thái, các lệnh vào/ra và RPC `ListBacklog` (kênh `backlog.requests`) |
| Task backlog | Task dưới Plan chưa được duyệt, hoặc Plan chưa chia Phase | có: `ListBacklog` view task (kênh `backlog.tasks`) |
| Execute backlog | Task dưới Phase đã duyệt, `open`/`blocked`, hoặc `execution_link` gần nhất `failed` | có: `ListBacklog` view execute (kênh `backlog.execute`) |

`AIS-06` kiểm hai kênh `backlog.requests` và `backlog.tasks` trả lời được; nội dung từng view chưa được e2e kiểm.
Chi tiết: [Backlog của Request và Task](./request-and-task-backlogs.md).

## Cờ `request_flow_enabled`

Mặc định **tắt**. Cần bật ở hai tầng (biến môi trường của `request-service` và cài đặt của tenant). Xem
[Bật luồng Request](./admin-enable-request-flow.md).

## Kiểm thử và công cụ kiểm tra

| Thứ | Nơi | Ghi chú |
|---|---|---|
| T1: e2e trong tiến trình | `backend-go/services/request-service/e2e` (thẻ build `e2e`, `E2E_DIALECT=postgres` hoặc `mysql`) | chặn PR |
| **orca-flows**: e2e xuyên tầng trên stack Docker thật | `tests/stages/e2e/orca-flows` (`run_acceptance.py`) | đi đúng đường UI dùng (đăng nhập, WebSocket `/ws`); 11 suite; xem [README của bộ test](../../../tests/stages/e2e/orca-flows/README.md) |
| orca-flows với `claude` giả | `run_acceptance.py --ai-stub` | AI do `fake_claude_cli/claude` trả lời xác định; chứng minh nối dây và hợp đồng, **không** đánh giá chất lượng AI; báo cáo ghi rõ "STUB" |
| CI hằng đêm / hằng tuần | `.github/workflows/orca-flows-e2e.yml` | dựng stack bằng compose local, agent chạy ngay trong job, dùng `claude` giả; chưa chạy trên runner thật; chưa là cổng chặn PR |
| Lan can hợp đồng (chặn PR) | `.github/workflows/backend-go-request-contracts.yml` | method backend gọi agent phải tồn tại ở agent; mọi RPC `RequestService` có kênh gateway; tệp vàng danh mục kênh; dial gRPC qua `grpcmw` |
| T2: e2e với agent giả | `backend-go/ci/request-e2e/run-request-e2e.sh`, workflow `request-e2e.yml` | **không** chặn PR; chưa chạy trên runner thật. Script gọi `tests/request/check_request_flow_types.py`, thư mục này đã bị xoá khỏi cây (commit `8beb6703d`), nên T2 hiện không chạy được cho tới khi sửa; `orca-flows` thay vai trò đó |
| T3: AI thật | cùng T2 với `ORCA_REQUEST_E2E_REAL_AI=true`, hoặc `run_acceptance.py` không có `--ai-stub` trên máy có Claude Code đã đăng nhập | không chặn, chỉ báo cáo |
| Stub agent cho đường AI của T1/T2 | `backend-go/services/request-service/e2e/stubs`, `e2e/cmd/agent-stub` | trả lời relay `ai.complete` của `infra-fleet-service` theo dấu `[e2e:type=bug size=S]` trong tiêu đề |
| Kiểm đăng ký service | `backend-go/ci/check-request-service-wiring.sh` (+ `_test.sh`) | báo lỗi nếu thiếu `request` ở go.work, Makefile, compose, migrate... |

Trong T1, các giai đoạn phụ thuộc RPC chưa có sẽ bị bỏ qua có ghi task chặn, và tự chạy khi RPC được hiện thực.
