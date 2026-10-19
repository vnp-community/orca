# TASK-REQ-042-06: Module `prompts/`: classify, analysis, clarification, plan, OpenSpec; xoá `PromptRegistry` chết

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `backend-go/prompts, request-service`
**File:** `backend-go/prompts/{go.mod,fence,redact,classify_request,analysis,clarification_answers,plan_native,plan_openspec,solution_openspec}/` (mới), `backend-go/go.work` (sửa), `services/request-service/internal/usecase/{solution_prompt,plan_prompt,openspec_plan_prompt,openspec_solution_prompt,clarification_answers_prompt,prompt_redaction,prompt_registry}.go` và `adapter/grpcclient/classification_prompt.go` (thay bằng ánh xạ mỏng; xoá hàm cũ)
**Depends on:** TASK-REQ-042-05
**Status:** [x] DONE (2026-10-20)

---

## Context

- Hàm cũ: xem SOL-042 mục 1. `fenceUntrusted`, `truncateRunes`, `promptBodyRunes=12000`, `promptArtifactRunes=8000`.
- Golden của task 05 là điều kiện.
- `analysis/` tách theo loại tài liệu (Solution, chẩn đoán, Findings, Answer).

## Việc cần làm

1. Tạo module `prompts` (`github.com/stablyai/orca-go/prompts`), thêm vào `go.work` và CI.
2. `fence` (thay `fenceUntrusted`) và `redact` (thay `prompt_redaction.go`): chuyển nguyên hành vi, có test.
3. Mỗi luồng một gói: `//go:embed *.tmpl`, `const ID`, `Input` thuần, `Render`; `Option("missingkey=error")`. `analysis/` có `solution.tmpl`, `diagnosis.tmpl`, `findings.tmpl`, `answer.tmpl`, `agent_solution.tmpl` và partial `_request_block.tmpl`, `_feedback.tmpl`, `_prior_artifacts.tmpl`. Phần logic quá phức tạp cho mẫu (ví dụ chọn quy tắc theo loại Request) giữ trong Go của gói `prompts`, **không** ở service.
4. Ánh xạ miền → `Input` ở request-service (hàm mỏng + test).
5. Đổi nơi gọi (`BuildSolutionPrompt`, `BuildPlanPrompt`, `BuildOpenSpecPlanPrompt`, `BuildOpenSpecSolutionPrompt`, `BuildClassificationPrompt`, khối câu trả lời Clarification) sang `prompts/...Render`.
6. Chạy golden của task 05: phải trùng từng byte; sửa mẫu cho đến khi trùng, không sửa golden.
7. Xoá hàm cũ và `PromptRegistry` chết; không để mã chết hay tham chiếu treo.
8. Phối hợp CR-REQ-041: khối phản hồi và `prior_artifacts` (TASK-REQ-041-02) phải nằm trong partial tương ứng; nếu 041-02 chưa xong thì làm sau hoặc cập nhật golden có chủ đích kèm giải thích.

## Kiểm thử

- Golden của task 05 trùng từng byte cho mọi ca của các luồng này.
- Test `Render` với dữ liệu thiếu trường (lỗi `missingkey`), với nội dung đối nghịch (đóng khối `</request>`) vẫn bị bọc.
- Acceptance `request`, `matrix`; UI native (stub).
- `go test ./prompts/... ./services/request-service/...`.

## Tiêu chí hoàn thành

- [x] Mọi prompt của request-service nằm trong `prompts/` và render trùng golden.
- [x] `PromptRegistry` chết đã xoá, không còn tham chiếu.
- [x] Không còn hàm dựng prompt trong `usecase/` hay `adapter/` của request-service (còn hàm ánh xạ miền → `Input`, không có câu chữ prompt).
- [x] Acceptance xanh. Acceptance `--ai-stub platform devserver request matrix exits ai plan analysis task notfound aistub request_base_ref analysis_revision` (2026-10-20, sau khi triển khai lại request/task/git-gateway/orchestration): 121 đạt, 0 lỗi, 3 chặn (AIS-xx cần AI thật, MCP-01/02 có sẵn), 2 bỏ qua; chạy lại tập nhỏ hơn sau lần triển khai cuối: 102 đạt, 0 lỗi. UI `qa-bug-task-native-flow` 3/3; `--real native-execute` (Claude thật, repo tạm) 2/2 đạt trong 11,8 phút.

## Rủi ro và lưu ý

- Lệch khoảng trắng: golden bắt được; ưu tiên sửa mẫu.
- Xung đột với CR-REQ-041: thứ tự trong kế hoạch tổng.

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

**Ngày:** 2026-10-20. **Kết quả:** mã, test, golden xong và đã chạy; acceptance chưa chạy (xem "Chưa kiểm chứng").

### Module `backend-go/prompts` (mới)

`go.mod` (`github.com/stablyai/orca-go/prompts`, chỉ require `common` cho `secretscan`; `go mod tidy` không đổi gì, không có `go.sum`), `README.md`, và các gói:

- `fence` (`Untrusted`, `Closing`, `Truncate`) thay `fenceUntrusted` và `truncateRunes`; `redact` (`Secrets`, `Redactor`) thay `prompt_redaction.go` (`PromptRedactor` chỉ có test gọi, chuyển nguyên hành vi và test sang `redact/redact_test.go`); `render` (bộ mẫu `text/template`, `missingkey=error`, hàm `fence/closing/truncate/redact/join/quote/inc`, `Require`/`ErrMissingField`).
- `request_context`: dữ liệu `Request`/`Project`, các partial dùng chung `_request_block`, `_project`, `_feedback`, `_retry_note`, `_approved_decision` và các trần độ dài (`BodyRunes=12000`, `ArtifactRunes=8000`, `FeedbackRunes=2000`, `RetryRunes=1000`).
- `classify_request`, `analysis` (`solution`, `agent_solution`, `diagnosis`, `findings`, `answer` + `_prior_artifacts`, `_prior_artifact_entry`, `_body`, `_solution_schema`, `_coverage_rule`, `_agent_coverage_rule`, `_read_only_preamble`, `_diagnosis_focus`), `clarification_answers`, `plan_native` (`plan`, `_shape_rules`, `_type_rules`, `_task_schema`), `plan_openspec` (`plan`, `result_block`, `TaskRange`), `solution_openspec`, `task_prompt_template`.
- Logic giữ trong Go của gói `prompts`: trần 40000 rune của khối `prior_artifacts` (bỏ bản cũ nhất trước), `MinOptions < 1 thành 1`, chọn mẫu theo loại tài liệu, ranh giới ngẫu nhiên của `classify_request` (có kiểm tra va chạm và lỗi khi nguồn ngẫu nhiên hỏng), `TaskRange`, `stripControl`. Quy tắc theo loại Request nằm trong mẫu (`_type_rules`, `_diagnosis_focus`); nhãn Plan do service truyền vào (`plan_native.Labels`) từ hằng của `domain`, nên prompt và bộ kiểm Plan không thể nêu hai nhãn khác nhau.
- Test: 11 gói, `go test ./... -race` xanh. Mỗi luồng: mọi mẫu render với `Input` đầy đủ, nội dung đối nghịch (`</request>`, `</prior_artifacts>`, `</clarifications>`, `</feedback>`, `</project>`, `</approved_decision>` trong mọi trường) chỉ đóng khối đúng một lần, khối tuỳ chọn bỏ trống thì không xuất hiện, thiếu trường bắt buộc trả `ErrMissingField`; `render`: khoá thiếu trong map, trường không tồn tại, mẫu hỏng, hàm.

### request-service (ánh xạ mỏng; hàm cũ đã xoá)

- Xoá: `BuildSolutionPrompt`, `BuildPlanPrompt`, `BuildOpenSpecPlanPrompt`, `BuildOpenSpecSolutionPrompt` (không có nơi gọi), `BuildClassificationPrompt`, `BuildTaskPromptTemplate`, `writeClarificationAnswers`, `writeRequestBlock`, `writePriorArtifacts`, `fenceUntrusted`, `promptInstructions` và các hằng schema, `PromptRedactor`, `PromptRegistry` (`prompt_registry.go`), `prompt_redaction.go`, `solution_prompt_prior_artifacts.go`, `openspec_solution_prompt.go`.
- Giữ làm ánh xạ: `RenderSolutionPrompt`, `RenderPlanPrompt`, `RenderOpenSpecPlanPrompt`, `renderTaskPrompt`, `grpcclient.RenderClassificationPrompt` (xuất ra vì `fake_claude_cli/request_service_contract.overlay.go.txt` ở gói ngoài gọi), `promptRequest`, `promptProject`, `promptPriorArtifacts`, `promptAnswers`, `TaskRangeFor` (uỷ quyền cho `plan_openspec.TaskRange`; còn dùng bởi `openspec_bundle_checker.go`), `OpenSpecRef`, `approvedAnalysisText` (dữ liệu, không phải câu chữ prompt). Lỗi dựng prompt mang `errPromptBuild`; `promptBuildFailure` kết thúc run với `RunErrInternal` và chỉ log lỗi mẫu, không log nội dung Request (C9). `FillTaskPrompts` nay trả `(PlanProposal, error)`.
- Số dòng (request-service, không tính golden): file không phải test sửa/xoá **+120 / -506**; file mới `prompt_request_mapping.go` **+52**; test sửa **+87 / -81**; test mới `prompt_request_mapping_test.go` + `prompt_render_assertion_test.go` **+116**. Module `prompts`: Go không test **717** dòng, test **949** dòng, mẫu `.tmpl` **229** dòng (29 file), README + go.mod 73 dòng. File lớn nhất 149 dòng; không file nào gần 300.
- Test cũ đổi cách gọi (không đổi nội dung kỳ vọng): `Build*Prompt(x)` thành `must*Prompt(t, x)`; `promptBodyRunes` v.v. thành hằng của `request_context`; `TestPromptRedactor` chuyển sang `prompts/redact`. **Không golden nào bị sửa, sinh lại hay xoá** (không chạy `--update`).

### Lệnh đã chạy

- `bash backend-go/ci/check-prompt-golden.sh`: 6/6 gói ok (request-service usecase + grpcclient, task-service x3, git-gateway). 109 golden của request-service khớp từng byte ngay lần chạy đầu; các golden `*.golden` cũ (`checkGolden`) cũng khớp.
- `cd backend-go/prompts && go vet ./... && go test ./... -race -count=1`: 11 gói ok. `gofmt -l`: rỗng.
- `cd backend-go/services/request-service && go build ./... && go vet ./... && go vet -tags=integration ./... && go vet -tags=e2e ./... && go test ./... -race -count=1`: tất cả ok.
- `go build ./...` ở `common`, `proto`, `prompts`, `services/task-service`: ok (task-service không sửa).
- `GOWORK=off go build ./... && go vet ./...` ở `prompts`; `GOWORK=off go build ./...` và `go mod verify` ở request-service: ok (go.mod có `require` + `replace ../../prompts`; `go.sum` không đổi). `GOWORK=off go mod tidy -diff` ở request-service chỉ báo `genproto/googleapis/rpc` direct/indirect, có sẵn từ trước, không liên quan.
- Thay `deploy/dev/scripts/build-local.sh request-service` (script ghi vào `deploy/dev/bin` nên không chạy): chạy đúng lệnh của nó `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=dev" -o <thư mục tạm>/orca ./cmd/server` (ok, 48 MB), thêm linux/arm64 và `GOOS=windows go build ./...`. Mô phỏng ngữ cảnh Docker (go.work + cmd + common + proto + prompts + policy + services, chỉ request-service đầy đủ): build ok.
- `bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh -v`: 6 test `TestFakeClaude_*` pass (prompt thật qua `claude` giả, parser và validator thật).

### Blast radius (grep/`go list`, GitNexus không có)

Mọi nơi gọi trong request-service đã đổi: `run_solution_generation.go`, `run_agent_readonly_analysis.go` (2 chỗ, kể cả lần thử lại không có đồ thị), `engine_native.go`, `run_plan_generation.go`, `engine_openspec.go` (`runAgent`; `specAgentFailure` thêm nhánh `errPromptBuild`), `run_proposal_generation.go` (`assemble`, lỗi dựng prompt là `fatal`), `request_classifier.go`. Ngoài service: `tests/stages/e2e/orca-flows/fake_claude_cli/request_service_contract.overlay.go.txt` gọi `usecase.BuildSolutionPrompt/BuildPlanPrompt` và `grpcclient.BuildClassificationPrompt`; đã thêm 3 hàm nhỏ `rendered*` và đổi 6 chỗ gọi (file này đang do tác tử khác sửa; đã đọc lại ngay trước khi sửa, chỉ thay đúng các chỗ gọi và nối thêm cuối file). Không gói service nào khác import các hàm đã xoá (grep toàn repo, trừ `specs/`, `docs/`, `testdata/`).

### Build, CI, Docker

- `backend-go/go.work`: thêm `./prompts`. `backend-go/Makefile`: `prompts` vào các vòng `build/vet/test/lint/tidy-all`, thêm mục tiêu `check-prompt-golden`. `.github/workflows/backend-go-request-contracts.yml`: path `backend-go/prompts/**` và job `prompt-catalog` (vet, gofmt, test `-race` của `prompts`, rồi `ci/check-prompt-golden.sh`); `backend-go-request-service.yml`: path `backend-go/prompts/**`. `actionlint` sạch.
- **19 Dockerfile** (`services/*/deploy/Dockerfile`) thêm `COPY prompts ./prompts` sau `COPY proto ./proto`: `go.work` liệt kê `./prompts`, thiếu thư mục này thì mọi service đều không build được trong Docker. Mới kiểm bằng mô phỏng ngữ cảnh (ở trên), chưa chạy `docker build`.
- Các workflow của service khác (task-service, workflow-service, ...) chưa thêm path `backend-go/prompts/**`; thuộc task 07 khi các service đó dùng module.

### Lệch so với task/brief và lý do

1. Partial `_request_block.tmpl` và `_feedback.tmpl` nằm ở gói `request_context` (cùng `_project`, `_retry_note`, `_approved_decision`), không nằm trong `analysis/`: `plan_native` và `plan_openspec` cũng cần đúng các khối này, để trong `analysis/` thì có ba bản sao hoặc plan phải import analysis. `analysis/` vẫn có `_prior_artifacts.tmpl` và 5 mẫu theo loại tài liệu như yêu cầu.
2. `task_prompt_template` (mới): `BuildTaskPromptTemplate` (câu chữ prompt trong `usecase/openspec_plan_builder.go`, có golden của task 05) không có trong danh sách của brief nhưng vi phạm tiêu chí "không còn hàm dựng prompt trong usecase"; đã chuyển.
3. `solution_openspec` bọc `fence` quanh tiêu đề và thân (hàm cũ không bọc). Chỉ khác byte khi nội dung chứa `</request`; 5 golden của nó không chứa nên vẫn khớp. Hàm này không có nơi gọi nên không đổi hành vi thật; làm vậy để mọi mẫu nhận chữ của bên ngoài đều qua `fence`.
4. `plan_openspec`, `solution_openspec` trả `ErrMissingField` khi thiếu `ChangeID` (và `Ref`, `ResultNonce` cho plan): trước đây ra prompt ghi vào `openspec/changes//`. Nơi gọi thật luôn có đủ ba giá trị (đã kiểm `ValidChangeID` trước khi gọi). `task_prompt_template` cố ý không bắt buộc `Branch` để khớp hành vi cũ (test dùng workspace giả có nhánh rỗng).
5. Gói dùng tên có gạch dưới (`classify_request`, `plan_native`, ...) cho khớp tên thư mục trong CR.
6. Một lần chạy nhầm `git rm --cached solution_prompt.go` (đã đảo ngay bằng `git reset -q HEAD -- solution_prompt.go`, chỉ chạm một đường dẫn; `git diff --cached` rỗng sau đó). Không có commit, stash hay đổi nhánh.

### Chưa kiểm chứng

- Acceptance `request`, `matrix` và UI native (stub): chưa chạy (không được chạy e2e/stack). Vì vậy tiêu chí "Acceptance xanh" để trống và trạng thái là PARTIAL.
- `docker build` thật của 19 Dockerfile; `deploy/dev/scripts/build-local.sh` chạy nguyên văn (chỉ chạy lệnh `go build` của nó).
- Nhánh lỗi dựng prompt trong các worker (`promptBuildFailure`) và `RenderClassificationPrompt` chỉ có test trực tiếp ở mức hàm ánh xạ/`specAgentFailure`; mẫu của `analysis`, `plan_native`, `classify_request` không có đường lỗi chạm tới được từ dữ liệu thật, nên không có test tích hợp cho nhánh đó.
- Text trong `tests/stages/e2e/orca-flows/README.md` và comment của `fake_claude_cli/claude` còn nhắc tên cũ `BuildSolutionPrompt`... (chỉ là chữ, các file đang do tác tử khác sửa; chưa đổi).
