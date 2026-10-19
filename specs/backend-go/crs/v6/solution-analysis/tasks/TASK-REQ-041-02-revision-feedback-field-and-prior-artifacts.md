# TASK-REQ-041-02: `revision_feedback` trên Solution và phản hồi cũ trong prompt

**From Solution:** [BE-REQ-SOL-041](../solutions/BE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Service:** `request-service`
**File:** `proto/orca/request/v1/solution.proto` (sửa) + mã sinh, `internal/domain/solution.go`, `internal/usecase/{list_solutions,run_solution_generation,solution_prompt}.go` (sửa), `internal/adapter/{postgres,mysql}/solution_repository.go` (sửa), `internal/adapter/grpc/{server_solution,solution_mapper}.go` (sửa)
**Depends on:** TASK-REQ-041-01
**Status:** [x] DONE (2026-10-20) — gateway đã ánh xạ `revisionFeedback` và `seq`; kiểm trên stack bằng ARV-01..05 và spec UI `analysis-revision` 5/5

---

## Context

- `Solution` proto dừng ở trường 17; `solutions.generation_run_id` trỏ tới `analysis_runs` có cột `feedback`.
- `priorArtifacts` không điền `PriorArtifact.Feedback`; `BuildSolutionPrompt` đã in `reviewer comment` khi có.
- Giao diện cần hiển thị ghi chú và nội dung yêu cầu chỉnh của từng phiên bản.

## Việc cần làm

1. Proto: `Solution.revision_feedback = 18`; sinh lại mã bằng quy trình của repo.
2. Repository: phương thức `RunFeedbackByIDs(ctx, runIDs []string) (map[string]string, error)` cho cả hai dialect (một truy vấn `IN`, giới hạn kích thước lô 500).
3. `ListSolutions` điền `RevisionFeedback` theo lô; `GetSolution` (nếu có) điền cho một bản.
4. `priorArtifacts`: nạp phản hồi của run từng bản, điền `PriorArtifact.Feedback` (đã bọc/cắt ở `BuildSolutionPrompt`).
5. **Giới hạn tổng** cho khối `prior_artifacts` (40000 rune): giữ các bản mới nhất, bỏ bản cũ nhất trước, thêm dòng "N earlier versions omitted". Hằng số có tên, không số ma thuật.
6. Mapper proto và JSON gateway: `revisionFeedback`.

## Kiểm thử

- Hợp đồng hai dialect: `RunFeedbackByIDs` (rỗng, nhiều, trùng, vượt lô).
- `TestListSolutions_RevisionFeedback_BatchedNoNPlusOne` (đếm truy vấn).
- Golden prompt: 3 vòng chỉnh, phản hồi cũ và mới đều có và đều nằm trong khối bọc; ca vượt 40000 rune cắt bản cũ nhất.
- Test log/span không chứa nội dung phản hồi.
- `go test ./services/request-service/... -run 'RevisionFeedback|PriorArtifacts|Prompt' -v`.

## Tiêu chí hoàn thành

- [x] `solution.list` trả `revisionFeedback` đúng cho từng phiên bản (RPC trên Postgres/MySQL, JSON gateway, và ARV-01 trên stack). Chạy thật phát hiện 2 lỗi đã sửa: nhãn hệ thống `clarification:<id>` bị trả như lời yêu cầu của người duyệt (nay lọc bằng `domain.ReviewerRevisionFeedback`), và `seq` chưa được ánh xạ sang proto nên màn hình không biết số phiên bản kế tiếp.
- [x] Prompt của lượt chỉnh chứa chuỗi phản hồi, trong khối bọc, trong giới hạn kích thước.
- [x] Không N+1; không lộ nội dung vào log.

## Rủi ro và lưu ý

- Tăng trưởng prompt theo số vòng (không giới hạn vòng): giới hạn tổng ở bước 5 là bắt buộc.
- Phản hồi cũ có thể chứa nội dung nhạy cảm: giữ nguyên quy tắc `fenceUntrusted` và redaction hiện có.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

---

## Ghi chú triển khai

**Ngày:** 2026-10-19. **Trạng thái:** PARTIAL (phần request-service xong; còn ánh xạ JSON ở api-gateway).

### Tệp đã đổi
- `backend-go/proto/orca/request/v1/solution.proto` (+ `revision_feedback = 18`) và `proto/gen/go/orca/request/v1/solution.pb.go` (sinh bằng protoc theo quy trình repo, diff chỉ gồm trường mới).
- `internal/domain/solution.go`: `Solution.RevisionFeedback` (nạp khi đọc, không lưu cột nào, không migration).
- `internal/usecase/ports_solution.go`: `SolutionStore.RunFeedbackByIDs(ctx, ids) (map[string]string, error)` và hằng `RunFeedbackBatchSize = 500`. Đặt trên `SolutionStore` (không phải `AnalysisRunStore`) để `ListSolutions` và `priorArtifacts` dùng được mà không đổi constructor hay `wire_*.go`.
- `internal/usecase/run_feedback_batches.go` (mới): khử trùng, bỏ id rỗng, chia lô 500 dùng chung cho hai dialect.
- `internal/adapter/{postgres,mysql}/solution_run_feedback.go` (mới): một truy vấn mỗi lô, luôn có `tenant_id`; Postgres `id = ANY($2::uuid[])` (bỏ qua id không phải UUID thay vì làm hỏng cả lô), MySQL `IN (?,...)`.
- `internal/usecase/list_solutions.go`: điền `RevisionFeedback` bằng một lần đọc theo lô; lỗi đọc được trả về (không nuốt).
- `internal/usecase/solution_prior_artifacts.go` (mới, chuyển `priorArtifacts` ra khỏi `run_solution_generation.go`): điền `PriorArtifact.Feedback` từ run của từng bản qua một lần đọc theo lô; lỗi đọc phản hồi chỉ ghi log (không kèm nội dung) và prompt đi tiếp không có phản hồi cũ.
- `internal/usecase/solution_prompt_prior_artifacts.go` (mới) + `solution_prompt.go`: hằng `promptPriorArtifactsRunes = 40000`; giữ bản mới nhất, bỏ bản cũ nhất trước, dòng `N earlier versions omitted` (`1 earlier version omitted` khi N=1); số thứ tự `artifact N` giữ nguyên theo vị trí gốc; bản mới nhất luôn được giữ (một mục tối đa khoảng 16100 rune, nhỏ hơn nhiều so với trần).
- `internal/adapter/grpc/server_solution.go`: `toProtoSolution` điền `RevisionFeedback`.
- Test: `usecase/{solution_prompt_revision_test,solution_revision_paths_test,list_solutions_revision_test}.go` (mới), sửa `solution_fakes_test.go` (fake `RunFeedbackByIDs` có bộ đếm `feedbackReads` và `failFeedback`), `solution_prompt_test.go` (nhãn mới), `adapter/grpc/server_solution_test.go`, `contracttest/solution_revision_feedback_contract.go` (mới) và hai dòng đăng ký trong `contracttest/solution_contract.go`.

### Độ lệch so với đề bài và lý do
1. Nhãn trong prompt đổi từ `reviewer comment:` sang `reviewer feedback this version was written under:`. Phản hồi của run của bản N là yêu cầu chỉnh đã dẫn tới bản N, không phải nhận xét về bản N; nhãn cũ dễ làm model hiểu sai. Test cũ đã cập nhật.
2. `RunFeedbackByIDs` nằm ở `SolutionStore` thay vì `AnalysisRunStore` (lý do ở trên).
3. Không có `GetSolution`: không có RPC hay use case đọc một bản. `ChooseSolutionOption` trả `Solution` nhưng không đi qua bước nạp phản hồi (thông tin đó đã có ở `solution.list`).

### Kết luận R4: đường nào thực sự sinh Solution, và phản hồi có tới prompt không
- `solutionAgent=true` và dev server có readonly + knowledge hub: `GenerateSolution` chọn mode `agent_readonly` -> `AnalysisRunner.worker` -> `RunAgentReadonlyAnalysis`, prompt từ `BuildSolutionPrompt` với `AgentSolution`/`UseKnowledge`; có `Feedback` và `PriorArtifacts`. Test: `TestRevisionFeedback_AgentSolutionSeesEveryEarlierAsk`.
- `solutionAgent=true` nhưng dev server thiếu knowledge hub hoặc repo path: tự lùi về `ai.complete` (`RunSolutionGeneration`, cũng `BuildSolutionPrompt`). Test: `..._OptedInSolutionWithoutKnowledgeHubStillSeesEveryEarlierAsk`.
- `solutionAgent=false`: `ai.complete`. Test: `TestRevisionFeedback_AiCompleteSolutionSeesEveryEarlierAsk`.
- diagnosis, findings, answer: `RunAgentReadonlyAnalysis` cùng `BuildSolutionPrompt`. Test: `TestRevisionFeedback_AgentReadonlyAnalysisKindsSeeEveryEarlierAsk` (bug, spike, question) và `TestBuildSolutionPrompt_PriorFeedbackReachesEveryKind`.
- Engine OpenSpec: `openspecEngine.GenerateAnalysis` trả `ErrEngineStepNotSupported`; engine OpenSpec chỉ sinh Plan (`BuildOpenSpecPlanPrompt`, đã có `Feedback` của plan và `Analysis` là Solution đã duyệt). `BuildOpenSpecSolutionPrompt` (`openspec_solution_prompt.go`) không có nơi gọi nào ngoài định nghĩa: là mã chết, không tham gia sinh Solution, nên không thêm Feedback/PriorArtifacts vào đó. Khuyến nghị xóa trong một task riêng.
- `nativeEngine.GenerateAnalysis` không có nơi gọi ngoài test; vẫn truyền `PriorArtifacts`/`Feedback` vào `BuildSolutionPrompt` (có test).
- Kết luận: không còn đường nào sinh Solution mà bỏ qua phản hồi chỉnh.

### Blast radius (grep, GitNexus không có)
- `priorArtifacts`: 2 nơi gọi (`RunSolutionGeneration.Execute`, `RunAgentReadonlyAnalysis.Execute`), chữ ký giữ nguyên.
- `BuildSolutionPrompt`: 3 nơi gọi (hai worker trên và `nativeEngine.GenerateAnalysis`); đổi duy nhất là khối `prior_artifacts` (nhãn, trần tổng, dòng bỏ bớt). Golden `TestBuildSolutionPrompt_Golden` (không có prior artifacts) không đổi.
- `SolutionStore` có thêm một phương thức: người cài đặt là hai `SolutionRecordRepository` và `fakeSolutions`; không còn cài đặt khác (`go build` và `go vet` toàn service xanh).
- `ListSolutions.Execute`: gọi từ `server_solution.go` và contract test; thêm đúng một truy vấn khi có ít nhất một bản.
- `toProtoSolution`: `ListSolutions` và `ChooseSolutionOption`; trường mới rỗng khi không điền.

### Lệnh đã chạy và kết quả
- `cd backend-go/services/request-service && go build ./... && go vet ./internal/{usecase,adapter/postgres,adapter/mysql,adapter/grpc,adapter/contracttest,domain}` xanh.
- `go test ./... -race -count=1` xanh toàn service (không còn gói nào FAIL).
- `go test ./internal/usecase/ -race -count=1 -run 'RevisionFeedback|PriorArtifacts|Prompt|RunFeedbackBatches' -v`: 38 test PASS.
- `go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -count=1 -p 1` (Docker/colima): cả hai gói `ok`, gồm các kịch bản `RunFeedbackByIDs` (rỗng, id rỗng và không hợp lệ, nhiều id, trùng, 1100+ id vượt lô với run thật ở lô đầu, giữa, cuối, unicode nhiều dòng 2000 rune, cách ly tenant, thiếu tenant) và `RevisionRoundsCarryFeedback` (bản đầu + 3 vòng chỉnh qua use case thật, đọc lại bằng `ListSolutions` và RPC, prompt vòng cuối chứa mọi phản hồi cũ trong khối bọc).
- Không N+1: `TestListSolutions_RevisionFeedback_BatchedNoNPlusOne` (30 bản, đúng 1 lần đọc phản hồi); `TestRunFeedbackBatches` kiểm tra chia lô 500.
- Không lộ nội dung: `TestRevisionFeedback_NeverReachesLogsRunErrorsOrEvents` bắt `slog` (kể cả nhánh tra cứu phản hồi lỗi và nhánh model lỗi), payload outbox và thông báo lỗi của run. Các worker này không có span/trace nên không có gì để kiểm tra ở đó.

### Chưa làm / chưa kiểm chứng
- Gateway: `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_views.go` (struct ở dòng ~118, ánh xạ ở dòng ~129) cần thêm `RevisionFeedback string `json:"revisionFeedback,omitempty"`` lấy từ `s.GetRevisionFeedback()`; cần cả golden/test của gateway. Ngoài phạm vi tệp được giao.
- Chưa chạy e2e acceptance hay UI (đề bài cấm).
- "Feedback đọc được bởi bất kỳ ai đọc được Request": `ListSolutions` chỉ kiểm tra tenant và sự tồn tại của Request như trước, không thêm kiểm tra quyền riêng cho trường này.
