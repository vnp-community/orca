# Ghi chú triển khai: solution-analysis (đợt R2)

Ngày: 2026-10-08. Nhánh `rf/sol`. Phạm vi: TASK-REQ-007-01..06 và TASK-REQ-008-01..05 (BE-REQ-SOL-007, 008) trong `backend-go/services/request-service`. Không sửa `.proto`, `common/*` hay service khác.

## 1. Hợp nhất với nhánh Approval (rf-appr): đã nối (2026-10-08)

| # | Điểm nối | Kết quả |
|---|---|---|
| M1 | Mở Approval thật | `usecase.OpenApprovalOpener` bọc `OpenApproval`; `wireApproval` lộ `SolutionOpener()`, `main.go` gọi `solution.bindApprovals(approval.SolutionOpener())`. Approval được mở SAU `TransitionRequest(analysis_ready)` trong cùng giao dịch của worker, nên `Approval.Stage = awaiting_analysis_approval` (rf-appr ghi Stage theo trạng thái Request lúc mở). Digest do handler tính; opener đối chiếu với digest của đề xuất |
| M2 | Registry | `wireSolution` chạy sau `wireRequestLifecycle` và trước `wireApproval`; `wireApproval(..., owned)` đăng ký ba handler của tính năng này thay `TransitionSubjectHandler` (đã gỡ phần trùng). Không cần `SubjectArtifacts` cho `solution`/`findings`/`answer`; `approvalSubjectArtifacts()` vẫn trống cho Plan, Phase, task list, pre-deploy |
| M3 | Chữ ký `SubjectHandler` | Không đổi. Handler nay theo hợp đồng chung: từ chối subject lạ và request ngoài `awaiting_analysis_approval` |
| M4 | Canceller | Dùng `requestLifecycle.Canceller` (canceller thật của rf-appr); `lateApprovalCanceller` đã bỏ |
| M5 | Quyền chọn phương án | `ApproverAwareAuthorizer`: người báo cáo, admin/lead/owner, hoặc người mà `AuthorizeApprovalDecision` (snapshot người duyệt: user, team, vai trò) cho phép quyết định approval pending. Sinh/sinh lại vẫn chỉ người báo cáo và admin. Kiểm trước giao dịch (có thể gọi tenant-service) |
| M6 | `RunSubjectHandlerContract` | Bộ thật chạy trên DB thật cho `solution` (kind solution và diagnosis), `findings`, `answer` (`SolutionContract/SubjectHandlerContract`). `SubjectHandlerFixture` thêm trường `Ctx` tuỳ chọn để handler đọc DB được chạy dưới tenant thật |
| M7 | CR-REQ-005 `ChangeRequestType` | Nay dùng canceller thật nên đường `OnClosedWithoutDecision(why="type_changed")` hoạt động khi có Approval pending. Khi chưa có Approval, `SupersedeForTypeChange` vẫn chưa được `ChangeRequestType` gọi (test `ChangeKeepsSolutions` của life-b khẳng định Solution không đổi); để mở |

Lệch có chủ đích sau hợp nhất: `ValidateForRequest` KHÔNG đòi `chosen_option` (một Solution được đề xuất trước khi ai chọn, mà Approval phải mở ngay với digest "không chọn"); `OnApproved` mới đòi chọn (`REQUEST_SOLUTION_OPTION_NOT_CHOSEN`) và tự so lại digest.

## 2. Quyết định đã đưa ra (lệch hoặc bổ sung so với task)

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Migration `0030_solution_analysis` (dải `0030..0039`): thêm cột vào `solutions` (`kind`, `status`, `content_ref`, `generation_run_id`) và `analysis_runs` (`solution_id`, `project_id`, `actor_id`, `feedback`, `enforcement`, `repo_check`); bảng khoá `analysis_project_gates`; chính sách RLS `relay_scan/relay_claim` cho quét lease. `analysis_runs` đã có sẵn ở `0006` (đã kiểm: `kind`, `mode`, lease, `active_key` MySQL đủ) | Worker khởi động lại phải dựng lại được ngữ cảnh chỉ từ dòng run; `foundation` chưa lưu `kind/status` của Solution (ghi chú N2 của R1a) |
| D2 | Một struct `SolutionRecordRepository` đáp cả `SolutionCoreRepository` (R1a) và `SolutionStore` (mới); hai interface giả cũ trong `ports.go` bị xoá | Trả lời câu hỏi mở của R1a: gộp, không giữ hai cổng |
| D3 | Cổng đồng thời `agent_readonly` bằng hàng `analysis_project_gates` + `FOR UPDATE`, cả hai dialect | Cùng một cơ chế cho Postgres và MySQL (không partial index); khoá tư vấn Postgres không có tương đương MySQL. Hàng khoá tạo ngoài giao dịch (`EnsureProjectGate`) vì tạo trong giao dịch dễ deadlock gap lock |
| D4 | `FinishOwned(run, owner)` thay cho `Complete`/`Fail`, `StartRun` thay cho `InsertRunWithSolution` | Ghi kết quả chỉ khi còn giữ lease nên worker mất lease không ghi gì |
| D5 | `relayAIComplete` tách khỏi `RelayClassifier` để `AICompletionRelay` dùng chung; `AIConnection` thêm `RepoPath`, `WorktreeID` | Dùng lại relay và fallback `RelayByDevServer` của life-b thay vì nhân bản |
| D6 | Chế độ agent: `SelectReadonlyRoute` (CR-REQ-033) chọn `agent_enforced` khi `features` có `agent.execPrompt.readonly`; ngược lại `prompt_only`. `REQUEST_AGENT_READONLY_USE_AGENT_FLAG` mặc định BẬT; `REQUEST_REQUIRE_ENFORCED_READONLY` mặc định tắt | Task 008-02 viết khi agent chưa có `accessMode`; agent nay đã có, nên dùng nó nhưng chỉ khi `features` xác nhận |
| D7 | `ExecPrompt` nhận `AnalysisConnection` (không chỉ `connectionID`); `AgentPromptInput` không có `trustPreset` hay `env` | Dự án thường chỉ đi `RelayByDevServer`; kiểu dữ liệu không cho đặt `full` |
| D8 | `MinOptionsFor(type)` nằm ở domain (`refactor`=1, còn lại 2) | Registry CR-REQ-003 chưa có `MinOptions`; thêm trường cần sửa file của đợt khác |
| D9 | Kết quả AI được lưu ở dạng đã chuẩn hoá (chỉ trường có kiểu) và đã che bí mật, tài liệu 64 KB tối đa | Chặn trường lạ do AI bịa, giảm rò rỉ; digest tính trên dạng chuẩn tắc nên không đổi qua JSONB/JSON |
| D10 | `nativeEngine.GenerateAnalysis` (R2 để rỗng) nay làm việc thật bằng cùng prompt/kiểm hợp lệ với worker; engine OpenSpec (CR-026) không đụng | Không để stub trả `AnalysisOutput{}` rỗng; CR-026 sẽ nối registry |
| D11 | Phục hồi đánh run `failed` thay vì chạy tiếp | Agent có thể còn chạy ngoài tầm kiểm soát, chạy lại gấp đôi chi phí |
| D12 | Lỗi AI/agent lưu mã chung trong run (`error_message` không chứa văn bản lỗi nguồn); log chỉ ghi lỗi, không ghi prompt | Brief: không log nội dung prompt |
| D13 | `Get/SetProjectEngineSettings` không làm | Thuộc CR-REQ-026 |

## 3. Điều chưa kiểm chứng

- `ai.complete` và `agent.execPrompt` trên dev server thật (thời gian, giới hạn, độ tuân thủ schema; `accessMode=readonly` và `trustPreset=default` thật sự chặn ghi hay không). Kịch bản thủ công chưa chạy: yêu cầu agent sửa file, `git commit`, ghi bằng đường dẫn tuyệt đối trên dev server cục bộ và SSH, rồi quan sát; kết quả quyết định có nên bật `REQUEST_REQUIRE_ENFORCED_READONLY=true`.
- Relay có chuyển `error.data.reason` của lỗi JSON-RPC hay chỉ thông điệp: `READONLY_MODE_UNSUPPORTED` hiện nhận bằng chuỗi trong thông điệp.
- Các service tenant-service/auth-service thật cho kiểm tra người duyệt theo team/vai trò (chỉ test với bản giả ở rf-appr; đường user trực tiếp đã chạy trên DB thật).
- MySQL dưới 8.0.1 (không có `SKIP LOCKED`), TiDB. Đã chạy thật: Postgres 16-alpine và MySQL 8.0 (`go test -tags integration`).
- Độ phủ mẫu che bí mật trên dữ liệu thật; ngưỡng 120s/256 KB/64 KB/2 run đồng thời là đề xuất chưa đo.
- Thông báo admin khi `REPO_MODIFIED` (câu hỏi mở 4): mới chỉ log cảnh báo.

## 4. Câu hỏi mở còn lại

1. `HEAD` trong so sánh repo: thêm RPC ở git-gateway hay chấp nhận `(branch, files)` + `changes.headMoved` của agent mới.
2. `GetAnalysisRun` hoặc luồng tiến độ cho UI: hiện chỉ `ListSolutionsResponse.runs` (tối đa 10 run gần nhất).
3. `ListSolutions` chưa có phân trang thật (`page_token` bỏ qua, `page_size` ngoài 1..200 thì dùng 50).
4. Quyền mức Request khi CR-REQ-035 chốt (D6 của kế hoạch tổng): `ApproverAwareAuthorizer` là bản tạm.
5. Hotfix tự duyệt: `TransitionRequest(analysis_ready)` đi `awaiting_plan_approval` theo registry; người gọi nên xác nhận nghiệp vụ "Solution `approved` do hệ thống" trước khi mở cho `security`.

## 5. Kiểm chứng đã chạy (2026-10-08)

- `go build ./... && go vet ./... && go vet -tags integration ./...` PASS; `gofmt -l` sạch.
- `go test -race -count=1 ./...` PASS toàn module `request-service`.
- `go test -tags integration -count=1 ./cmd/server/ ./internal/adapter/postgres/ ./internal/adapter/mysql/` PASS với Postgres 16 và MySQL 8.0 thật (testcontainers): up/down/up gồm `0030`, `SolutionContract` 16 kịch bản trên cả hai DB (chỉ mục duy nhất của run, `StartRun`, 12 start đồng thời, cổng 8 goroutine giới hạn 2, lease/`ClaimExpired` hai sweeper, CAS/supersede/`DeleteDraft`, Unicode và digest qua DB, chéo tenant, luồng generate → choose → approve → `planning`, sinh lại có phản hồi, JSON sai không để lại draft, `question` hoàn tất không Plan, `hotfix` tự duyệt, phục hồi, rollback persist, RPC qua bufconn) và `TestPostgres_AnalysisRuns_TenantRLS` (role `NOBYPASSRLS`).
- Hai lỗi chỉ lộ khi chạy trên DB thật và đã sửa: đếm đồng thời MySQL dùng snapshot cũ (nay `FOR SHARE`), tra cứu run thắng sau khi thua cuộc đua chèn (nay đọc khoá).

## 6. Người gọi đã kiểm khi sửa symbol có sẵn

- `usecase.PriorArtifact` (struct rỗng giữ chỗ): chỉ `AnalysisInput.PriorArtifacts`, không ai khởi tạo trước đây; thêm trường.
- `grpcclient.AIConnection`: thêm trường; `RelayClassifier` và test life-b không đổi hành vi.
- `RelayClassifier.complete` → `relayAIComplete`: hàm private, test của life-b PASS nguyên vẹn.
- `domain.Solution` (bỏ trường `Options []string` không ai dùng), `domain.RedactSecrets`, `usecase.RedactApplicationSecrets`, `usecase.GenerateSolution/ChooseOptionApprovalHandler/RunAgentReadonlyAnalysis/FindingsAnswerApprovalHandler/CompleteRelayPrompt` (hàm rỗng), `SolutionRepository`/`AnalysisRunRepository`/`SolutionListFilter` trong `ports.go`: không có người gọi (build toàn module PASS sau khi xoá).
- `contracttest/expected_columns.go`: thêm 4 cột của `solutions` (một dòng).

## 7. Kiểm chứng sau hợp nhất (2026-10-08)

- `go build ./...`, `go vet ./...`, `go vet -tags integration ./...`, `gofmt -l` sạch; `go test -race ./...` PASS, trừ một đua dữ liệu có sẵn, chập chờn (khoảng 1 trong 15 lần) trong test của life-b `TestCreate_ClaimLoser_ReturnsWinner` (fake dùng chung giữa 12 goroutine), không thuộc phạm vi này.
- `go test -tags integration ./cmd/server/ ./internal/adapter/postgres/ ./internal/adapter/mysql/` PASS trên Postgres 16 và MySQL 8.0 thật, gồm `RPCFlow` (generate → choose → `ApprovalServer.Approve` đúng digest → `planning`, digest cũ và nội dung bị sửa bị `REQUEST_APPROVAL_DIGEST_MISMATCH`), `RPCReject` (reject → backlog `returned_from_stage=analysis`), `ApproverCanChoose`, `SubjectHandlerContract`, `RegenerateWithFeedback` (Approval cũ `cancelled`, mới `pending`), `TestRun_ServesApprovalServicesWhenEnabled`.
- `buf breaking --against main` sạch. Build, vet, test `proto`, `api-gateway`, `mcp-service` PASS.
