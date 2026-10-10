# TASK-V2-023: Loại phân tích chỉ đọc không có `repo_path`; câu hỏi chặn của Solution không có đường trả lời

**From Solution:** (phát hiện khi chạy Playwright `request-live` trên stack thật, xem `specs/frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-008-live-stack-playwright-smoke.md` mục "Kết quả thực thi" và `TASK-FE-E2E-009`) · **Resolves:** `REQUEST_ANALYSIS_NO_REPO_PATH` (BE) và `SOLUTION-BLOCKING-QUESTIONS` (phần backend)
**Priority:** P0 (loại chỉ đọc không chạy được), P1 (câu hỏi chặn) · **Service/Area:** request-service, api-gateway
**Files:**
- backend-go/services/request-service/internal/adapter/grpcclient/{ai_connection_resolver.go,dev_server_repo_path.go} (+ `ai_connection_resolver_test.go`)
- backend-go/services/request-service/internal/domain/{solution_errors.go (thông báo), rpc_catalog.go (+ test)}, internal/adapter/grpc/security_chain_test.go, internal/usecase/run_agent_readonly_analysis.go (thông báo, `Clarifications` dep)
- backend-go/services/request-service/internal/usecase/{clarification_answers_prompt.go,solution_prompt.go,run_solution_generation.go,run_agent_readonly_analysis.go} (+ test)
- backend-go/services/request-service/cmd/server/wire_solution.go
- backend-go/services/request-service/e2e/{readonly_analysis_test.go,blocking_question_test.go,flow_ai_answers_test.go,stubs/servers.go}
- backend-go/services/api-gateway/internal/adapter/wscompat/{channels_request_clarification_ask.go,channels_request.go} (+ test, `request_rpc_channel_map_test.go`, `testdata/channel_inventory.json`)
- specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md (mục 2.6, bảng lỗi mục 5), backend-go/services/request-service/README.md
**Depends on:** TASK-V2-022 (cùng đường tự khởi động sau Clarification)
**Status:** `[x]` XONG về mã và test (cập nhật 2026-10-10, chưa commit); **chưa chạy trên stack thật** (cấm redeploy trong đợt này)

---

## Lỗi 1: `REQUEST_ANALYSIS_NO_REPO_PATH` với mọi loại chỉ đọc (đã sửa)

**Triệu chứng:** `bug`, `hotfix`, `security`, `performance`, `spike`, `question` dùng `agent_readonly`; `GenerateSolution` thủ công trả `REQUEST_ANALYSIS_NO_REPO_PATH`, consumer tự khởi động log `resume skipped`.

**Nguyên nhân gốc:** `AIConnectionResolver.ResolveForProject` có hai nhánh. Nhánh `infra.connections` (khi `ResolveConnection(connectionId=projectID)` trả `connected`) có `RepoPath`, nhưng id của `infra.connections` do server sinh nên nhánh này gần như không bao giờ trúng (BUG-025 của task-service). Nhánh dự phòng "dev server của repo mặc định" trả `AIConnection{DevServerID}` và **bỏ rơi** đường dẫn dù `repos[0]` có. `generate_solution.go` và `run_agent_readonly_analysis.go` đòi `RepoPath != ""` cho chế độ chỉ đọc.

**Xác định `repo_path` đúng nghiệp vụ (bằng chứng trong mã):**
- Quy ước có sẵn: `Repo.URL` của repo gắn dev server là **đường dẫn tuyệt đối trên dev server** (`CreateProject` với `dev_server_id` + `repo_path` ghi `repo.URL = repo_path` sau `fs.checkPath`; `SetupExistingFolder`/`ImportNested` cùng quy ước; git-gateway `resolver.go` và task-service `project_execution_resolver.go` dùng `repo.URL` làm đường dẫn khi có dev server).
- Repo nào: repo **đầu tiên theo `position`** (cùng repo mà task-service thực thi, để phân tích và thực thi đọc cùng một mã nguồn). `Request` chưa có trường chọn repo, nên project nhiều repo chỉ phân tích repo mặc định (giới hạn đã biết, ghi trong README request-service).
- Đường dẫn là của **dev server chạy agent** (agent.execPrompt đi qua `RelayByDevServer`), không phải máy backend. Vì vậy kiểm "tuyệt đối" bằng cú pháp trung tính HĐH (`dev_server_repo_path.go`): POSIX `/…`, `C:\…` hoặc `C:/…`, UNC `\\…`; không dùng `filepath.IsAbs` của máy backend (sai cho dev server Windows hoặc SSH).
- **Không đoán:** URL từ xa (`https://`, `git@host:`, `file://`), đường dẫn tương đối, rỗng, có ký tự điều khiển hoặc đoạn `..` cho `RepoPath` rỗng, và kết quả vẫn là `REQUEST_ANALYSIS_NO_REPO_PATH` (FailedPrecondition, mã có cấu trúc, không tạo run). Repo không gắn dev server hoặc dev server không `reachable` vẫn là `ErrNoDevServer` → `REQUEST_ANALYSIS_NO_CONNECTION`. Thông báo của mã đổi thành nói đúng nguyên nhân ("repo mặc định không có đường dẫn thư mục trên dev server").
- Không nới quyền: agent vẫn chạy với cwd = thư mục repo của project, vẫn qua ba lớp chỉ đọc; không có đường nào cho agent đọc ngoài repo của project (đường dẫn lấy từ repo của project, không từ người dùng, đã loại `..`). Chế độ `complete` không đọc repo nên dùng được kể cả khi URL là địa chỉ từ xa.
- `WorktreeID` ở nhánh dự phòng vẫn rỗng, nên lớp so sánh repo trước/sau ghi `repo_check=skipped` (như README đã nêu); đây không phải hồi quy.

**Rà người gọi của `ResolveForProject`** (grep, không có gitnexus/codegraph): `AnalysisConnections` (wire_solution.go; `GenerateSolution.start`, `RunAgentReadonlyAnalysis.Execute`, `GeneratePlan` chỉ kiểm tồn tại), `AICompletionRelay` và `RelayClassifier` (chỉ dùng `ConnectionID`/`DevServerID`; `RepoPath` thêm vào không đổi hành vi). Phân loại, Plan, fix task hotfix không đọc `RepoPath` nên không bị ảnh hưởng. `EngineReadinessGate` dùng bộ phân giải khác, không đổi.

**Test:** `ai_connection_resolver_test.go` (11 dạng URL, chỉ lấy repo đầu, nhánh connected giữ đường dẫn của infra); `generate_solution_test.go` (6 loại chỉ đọc nhận kết nối dev server có đường dẫn; thiếu đường dẫn → mã có cấu trúc và không có dòng nào; `complete` vẫn chạy); `run_agent_readonly_analysis_test.go` (agent chạy ở đúng thư mục, `repo_check=skipped`; mất đường dẫn giữa chừng → run `failed` đúng mã, agent không bị gọi); e2e Go `readonly_analysis_test.go` (bug, spike, question chạy `agent.execPrompt` với `worktreePath` là thư mục repo trên dev server stub; repo chỉ có URL từ xa → `REQUEST_ANALYSIS_NO_REPO_PATH`, không có run, agent không chạy, `COMPLETE` vẫn chạy). Bỏ bản sửa thì e2e đỏ đúng mã lỗi (đã thử). Stub `Project.ListRepos` giờ trả `Url` (mặc định `/srv/orca-e2e/repo`, đổi theo project bằng `SetRepoURL`) và stub agent trả lời `agent.execPrompt` bằng cùng tài liệu mẫu của `ai.complete`.

## Lỗi 2: Solution có câu hỏi chặn nhưng không có đường trả lời (đã triển khai)

**Điều tra theo CR (bằng chứng):**
- CR-REQ-028 mục 2.5: "người duyệt thấy chúng ở UI và chọn 'Hỏi lại' (gọi `RequestClarification(source=solution_open_question|plan_assumption, items[])`, mỗi mục thành một câu hỏi `text` hoặc `boolean`)"; `ValidateForRequest` chặn bằng `REQUEST_SOLUTION_BLOCKING_QUESTIONS` khi chưa có "một Clarification `answered` có `source_ref` là Solution đó, tham chiếu `question_key`" (`open_question:<Q-id>`). Bảng 2.1: nguồn `solution_open_question` "người duyệt hoặc admin gọi `RequestClarification`".
- Backend đã có: use case `RequestClarification` + RPC công khai (cho nguồn không phải của nền tảng), `AnswerClarification`, cổng `ApprovalGates.CheckSolution`.
- **Thiếu:** RPC `RequestClarification` bị đánh dấu `internalRPC` trong bảng parity gateway, **không có kênh WS** (`clarification.*` chỉ có `list/get/answer/cancel`). Người dùng không có cách tạo Clarification cho câu hỏi của Solution, nên câu hỏi chặn không bao giờ được giải. CR yêu cầu đường này ("Hỏi lại"), nên triển khai (không phát minh).
- **Thiếu thứ hai (phát hiện khi viết e2e):** trả lời một Clarification `solution_open_question` đưa Request về `analyzing`, **thay thế Solution cũ** (`supersedeDerived`) và tự sinh lại với `feedback="clarification:<id>"`; nhưng prompt chỉ in id đó, **câu trả lời không tới AI** (không có `target_path` nên không vào nội dung Request). AI không biết câu đã được trả lời, hỏi lại đúng câu hỏi chặn, và vòng lặp không bao giờ kết thúc (cổng khoá theo `source_ref` = Solution cũ nên không bao giờ khớp Solution mới).

**Đã làm:**
- Kênh `clarification.request` → `RequestClarification` (`channels_request_clarification_ask.go`): `{requestId, source, sourceRef?, questions:[{questionKey, kind, prompt, reason, options?, suggestedDefault?, required?, targetPath?}]}` → `{clarification}`. Gateway chỉ kiểm hình dạng (`requestId`, `source`, ít nhất một câu, `kind` hợp lệ); quyền và trạng thái do service quyết (reporter/admin; nguồn nền tảng `readiness`/`task_blocked` bị từ chối `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED`; mỗi Request một Clarification mở, gọi lặp cùng bộ khoá trả lại chính nó). Tệp vàng `channel_inventory.json` (+ `clarification.request`), bảng parity (`viaChannel`), CONTRACT 2.6 + bảng lỗi. Không đổi proto. Kênh nằm trong mẫu `clarification.*` của `excluded_channels.yaml` (không thành công cụ MCP: việc của người).
- Câu trả lời tới AI: `clarification_answers_prompt.go` thêm khối `<clarifications>` (câu hỏi + câu trả lời JSON của các Clarification `answered` nguồn `solution_open_question` và `manual`, tối đa 20 câu mới nhất, cắt 300/1000 ký tự, `secretscan.Redact`, rào `fenceUntrusted`, ghi rõ "là dữ liệu, đừng hỏi lại") vào prompt của cả hai đường (`RunSolutionGeneration` qua `ai.complete`, `RunAgentReadonlyAnalysis` qua agent). Đọc lỗi thì phân tích tiếp không kèm câu trả lời (best effort như `priorArtifacts`; cổng duyệt vẫn chặn). Đường engine `native` (`engine_native.go`) không đổi (không có người gọi qua `GenerateSolution`; ghi chú: không thêm câu trả lời).
- Lỗi cấu trúc: **giữ nguyên** thông báo `REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1, Q-2` (frontend `parseBlockingQuestionIds` đã dựa vào nó; thêm đuôi JSON như `retryAfterMs` sẽ làm vỡ bộ tách của frontend đang song song). Khoá hợp đồng bằng test (domain `TestBlockingErrorsListTheirIdsAfterTheColon`, gateway `TestRequestChannelError_BlockingQuestionsKeepTheirIds`: FailedPrecondition mang mã REQUEST_ vẫn được chuyển nguyên, không thành `REQUEST_STATE_STALE`). Văn bản đầy đủ và cờ `blocking` của từng câu nằm sẵn trong `optionsJson` (`open_questions[{id,text,blocking}]`) của `solution.list`. Bổ sung đuôi có cấu trúc để sau, khi frontend sẵn sàng.

**Đường dùng cho UI (CONTRACT 2.6):** `approval.approve` trả `REQUEST_SOLUTION_BLOCKING_QUESTIONS` → UI liệt kê câu `blocking` của Solution → `clarification.request {source:"solution_open_question", sourceRef:<solution.id>, questions:[{questionKey:"open_question:<Q-id>", kind:"text", prompt, reason, required:true}]}` → `clarification.answer {complete:true}` → Request về `analyzing`, Solution cũ `superseded`, tự sinh Solution mới (id mới) có câu trả lời trong prompt → chọn lại phương án và duyệt.

**Test:** gateway `channels_request_clarification_ask_test.go` (ánh xạ tham số, hình dạng sai bị chặn trước service, mã từ chối của service giữ nguyên); `clarification_answers_prompt_test.go` (lọc nguồn, giới hạn, best effort, rào và che bí mật, cả hai worker đưa câu trả lời vào prompt); e2e Go `blocking_question_test.go`: Solution có câu hỏi chặn → `Approve` bị `REQUEST_SOLUTION_BLOCKING_QUESTIONS` nêu đúng id → `RequestClarification` → `AnswerClarification` → Solution mới được sinh với prompt chứa câu trả lời và không còn câu chặn → chọn và duyệt thành công. Bỏ dòng đưa câu trả lời vào prompt thì test đỏ đúng chỗ (đã thử).

## Lỗi 3 (phát hiện khi chạy UI live, đã sửa): kênh `clarification.request` bị `REQUEST_FORBIDDEN`

**Triệu chứng:** bấm "Send to reporter" nhận `clarification.request -> REQUEST_FORBIDDEN: not permitted`; log request-service `PermissionDenied INTERNAL_CALLER_REQUIRED: this method is restricted to internal services` cho `RequestService/RequestClarification` (người dùng admin, tenant đúng).

**Nguyên nhân gốc (lỗi của tôi):** `domain/rpc_catalog.go` khai `RequestClarification` là `unlocated(GroupInternal, RateNone, false)`, nên guard `internalcaller` chỉ nhận token dịch vụ nội bộ, còn kênh gateway gọi bằng token gateway. Khi thêm kênh tôi chỉ đổi bảng parity của gateway (`internalRPC` → `viaChannel`) mà không đổi danh mục phân quyền của service. Test e2e Go trước đó dùng token dịch vụ của bộ khung nên đi qua được guard, không bắt được. Bài học: parity "RPC ↔ kênh" ở gateway không kiểm nhóm phân quyền của service.

**Sửa:** `onRequest(GroupTriage, "request_id", RateWrite, false)`, giống `AnswerClarification`/`ConfirmRequestType` (nhóm `triage`: owner/reporter theo `request.rego`, admin toàn cục; `AgentAllowed=false` nên agent MCP không gọi được; `RateWrite`; định vị Request qua `request_id` nên sai tenant là `REQUEST_NOT_FOUND`; từ chối được ghi audit). Usecase vẫn chặn thêm: chỉ người báo cáo hoặc admin (`canWriteRequest`), nên chủ project không phải người báo cáo bị `REQUEST_FORBIDDEN`. Không có service nào khác gọi RPC này (grep: chỉ gateway; các nguồn nền tảng `readiness`/`task_blocked` đi thẳng use case trong tiến trình, không qua RPC). Không thể giả nguồn nền tảng qua RPC: handler từ chối `source.SystemOnly()` bằng `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED` trước khi gọi use case và luôn đặt `ActorKind=user` (test `TestClarificationServer_RequestClarification_RejectsSystemOnlySources` có sẵn).

**Test thật qua chuỗi interceptor (bufconn, `SecurityChain` + chính sách OPA thật + authorizer thật):** `TestChain_RequestClarificationIsAPersonRPCOfTheReporterAndAdmin`: (a) người báo cáo và admin với token gateway tới được handler; (b) người lạ, thành viên project, agent đứng sau admin bị `REQUEST_FORBIDDEN` (và có 3 bản ghi audit từ chối), người báo cáo của tenant khác không tới được handler; (c) không token, token sai, token dịch vụ nội bộ đều `INTERNAL_CALLER_REQUIRED`. Đã thử đảo catalog về `GroupInternal`: test đỏ đúng triệu chứng live. Thêm `TestNoGatewayChannelTargetsAnInternalRPC` (domain): đọc bảng parity của gateway và đòi mọi RPC có kênh không thuộc nhóm `internal`, để lỗi cùng lớp không lặp lại.

**Rà các RPC mà kênh gateway gọi:** mọi RPC trong bảng parity (`viaChannel`, 36 mục) đã đối chiếu với danh mục; RPC nhóm `internal` là `LookupRequestBySource`, `ReportTaskOutcome`, `RecordRequestCheck`, `RequestApproval`, không cái nào có kênh hay được api-gateway gọi (grep `api-gateway/internal`; `LookupRequestBySource` chỉ do `issue-status-sync` gọi bằng token dịch vụ). Không còn trường hợp tương tự. Kênh `clarification.request` là kênh duy nhất tôi thêm.

**Kiểm:** `request-service` `go build ./... && go vet ./... && go test ./...` PASS; `go vet -tags 'integration e2e' ./...` sạch; `E2E_DIALECT=postgres` và `mysql` `-tags e2e` PASS (44s, 51s); `go test -tags integration ./cmd/... ./internal/adapter/grpc/...` PASS. Cần redeploy `request-service` (đổi danh mục) và `api-gateway` (kênh mới, như trước).

## Việc cần làm

- [x] Sửa `AIConnectionResolver` (đường dẫn từ repo mặc định, không đoán) + test đơn vị, usecase, e2e Go.
- [x] Kênh `clarification.request` + tệp vàng + parity + CONTRACT + test.
- [x] Đưa câu trả lời vào prompt phân tích lại + test.
- [x] Ghi quyết định và bằng chứng CR.
- [ ] Redeploy `request-service` và `api-gateway`, chạy lại UI e2e (người điều phối); frontend nối nút "Hỏi lại" vào `clarification.request`.
- [ ] Commit.

## Kết quả thực thi

Chạy với `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock`, `TESTCONTAINERS_RYUK_DISABLED=true`:

- `cd backend-go/services/request-service && go build ./... && go vet ./... && go test ./...`: PASS.
- `go vet -tags 'integration e2e' ./...` và `go test -tags integration ./... -count=1`: PASS (Postgres và MySQL).
- `E2E_DIALECT=postgres go test -tags e2e ./e2e/... -count=1`: PASS (43 giây); `E2E_DIALECT=mysql ...`: PASS (49 giây). Các kịch bản E01 đến E12 vẫn xanh dù khởi động tự động nay chạy chế độ chỉ đọc thật trên stub.
- `cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./...`: PASS (kể cả `TestChannelInventoryMatchesGolden` sau `UPDATE_GOLDEN=1`).

**Blast radius đã rà tay (không có gitnexus/codegraph):**
- `AIConnectionResolver.ResolveForProject`: xem danh sách người gọi ở trên; thêm giá trị `RepoPath`, không đổi chữ ký.
- `SolutionPromptInput` thêm `Answers`: người gọi `BuildSolutionPrompt` là `RunSolutionGeneration`, `RunAgentReadonlyAnalysis`, `engine_native`; khi vắng thì prompt giữ nguyên từng byte.
- `NewRunAgentReadonlyAnalysis` thêm trường tuỳ chọn `Clarifications` trong `AgentReadonlyDeps` (nil an toàn); `RunSolutionGeneration.WithClarifications` tuỳ chọn. Nối ở `wire_solution.go`.
- Gateway: thêm một kênh unary; `registerRequestChannels` và danh mục kênh (tệp vàng) đổi; frontend nào đọc `channel_inventory.json` sẽ thấy thêm `clarification.request`.

**Chưa kiểm chứng / giới hạn còn lại:**
- Chưa chạy trên stack thật (cấm redeploy): chưa thấy với Claude thật loại `bug/question` chạy xong và câu hỏi chặn được giải. `repo.URL` của repo mặc định trên stack thật phải là đường dẫn tuyệt đối (project tạo bằng `CreateProject` với `repo_path` thì đúng); project tạo bằng URL git từ xa sẽ nhận `REQUEST_ANALYSIS_NO_REPO_PATH` có chủ đích.
- Project nhiều repo: chỉ phân tích repo đầu theo `position`. Repo thuộc VM tạm (hidden target) không được nhận biết ở nhánh dự phòng (ListRepos không trả); đường dẫn đó nằm trên VM nên agent trên dev server sẽ báo lỗi thư mục (lỗi hiển thị, không vượt quyền).
- `clarification.request` không kiểm `sourceRef` có đúng là Solution của Request và `questionKey` có đúng một câu hỏi của Solution; sai thì cổng không bao giờ thoả (không gây hại, chỉ vô ích). Chưa thêm vì CR không yêu cầu.
- Plan: `plan_assumption` vẫn chưa kiểm cùng đường (Plan không tự khởi động sau Clarification, CR-REQ-028 Q3); `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS` giữ như cũ.
- Cổng duyệt khoá theo `source_ref` = id Solution; sau khi sinh lại, câu hỏi chặn của Solution mới cần Clarification mới. Hành vi theo CR, không đổi.
