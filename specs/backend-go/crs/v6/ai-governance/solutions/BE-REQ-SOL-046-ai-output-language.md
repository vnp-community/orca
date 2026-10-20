# BE-REQ-SOL-046: Ngôn ngữ đầu ra của AI là cấu hình hệ thống (`REQUEST_DEFAULT_OUTPUT_LANGUAGE`, mặc định `vi`), chỉ thị trong prompt phân tích, Plan native và OpenSpec

> **📝 Chưa triển khai (0/7 task).** Soạn 2026-10-20 từ đọc mã, có chạy `openspec validate` cục bộ. Mọi mô tả hành vi của mô hình thật chưa được kiểm bằng chạy thật.

**CR:** [CR-REQ-046](../../../../../../docs/crs/v6/ai-governance/CR-REQ-046-ai-output-language-system-setting-default-vietnamese.md)
**Service:** `backend-go/prompts` · `request-service` · `deploy/dev` · `skills/` · `tests/stages/e2e/orca-flows` (không đổi proto, migration, `api-gateway`, MCP, frontend)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần; kiểu ngôn ngữ ở module `prompts`, không ở `domain`), [`arch/10`](../../../../tdd/architecture/10-deployment-infrastructure.md) (cấu hình triển khai)
**Phụ thuộc solution:** [BE-REQ-SOL-042](./BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md) (danh mục prompt, golden, ngân sách), SOL-007, SOL-008, SOL-012, SOL-026, SOL-041. Mở khoá CR-REQ-034 (provenance). Không có solution frontend: xem CR mục 2.7.

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-20:

- **Không có chỉ thị ngôn ngữ**: tìm `vietnam|language|locale|english|tiếng` trong `backend-go/prompts/**` chỉ ra `task_decompose` (`Languages []string`, ngôn ngữ lập trình). Mẫu `analysis/{solution,agent_solution,diagnosis,findings,answer}.tmpl`, `plan_native/plan.tmpl`, `plan_openspec/plan.tmpl` là chỉ dẫn tiếng Anh kèm schema JSON nội tuyến (hoặc ngữ pháp OpenSpec).
- **Cấu trúc đầu vào**: `analysis.Input` (`prompts/analysis/analysis.go`), `plan_native.Input`, `plan_openspec.Input`; ba struct của service ánh xạ sang chúng: `SolutionPromptInput` (`usecase/solution_prompt.go:10`, `RenderSolutionPrompt` `:30`), `PlanPromptInput` (`usecase/plan_prompt.go:12`, `RenderPlanPrompt` `:28`), `OpenSpecPlanPromptInput` (`usecase/openspec_plan_prompt.go:12`, `RenderOpenSpecPlanPrompt` `:45`).
- **Sáu nơi dựng đầu vào/gọi render**: `run_solution_generation.go:50,59`; `engine_native.go:47,52` (chỉ test gọi `NewNativeEngine`, `engine_native.go:25`; cmd/server không dựng nó); `run_agent_readonly_analysis.go:109,125` và lần dựng lại không có đồ thị `:141`; `run_plan_generation.go:71,80`; `engine_openspec.go:225` (`runAgent`).
- **`AnalysisSettings`** (`usecase/analysis_settings.go`) đã đến cả bốn worker: `RunSolutionGeneration.settings`, `RunAgentReadonlyAnalysis.settings`, `RunPlanGeneration.settings` (`run_plan_generation.go` field `settings`), `OpenSpecEngineDeps.Settings` (`engine_openspec.go:29`). `cmd/server/wire_solution.go:65` dựng một `settings := analysisSettings(cfg)` (`:186`) dùng chung với `wire_plan.go:33,47,120,125` (qua `solution.settings`). `DefaultAnalysisSettings()` và `withDefaults()` không điền ngôn ngữ: giữ như vậy để test hiện có không đổi.
- **Cấu hình**: `internal/config/config.go` `Load()` không trả lỗi; `REQUEST_DEFAULT_ENGINE`/`_SOLUTION_AGENT`/`_EXECUTION_MODE` đọc ở `domain.EngineDefaultsFromEnv` (`engine_settings.go:76`, gọi từ `cmd/server/main.go:88`) và giá trị sai chỉ cảnh báo. `main()` gọi `config.Load()` rồi `run(ctx, cfg)`; lỗi của `run` in ra stderr và `os.Exit(1)` (`main.go:40-48`); lệnh `healthcheck` thoát trước `run`.
- **Bộ mẫu**: `render.NewSet(funcs, files...)` phân tích `*.tmpl` của mọi `fs.FS` truyền vào và đòi tên tệp duy nhất; `analysis`, `plan_native`, `plan_openspec` đều nhận `request_context.Files`. Hệ quả: một partial chung đặt ở `request_context/`, partial riêng của OpenSpec phải có tên khác (`_language_rule_openspec.tmpl`). `missingkey=error` chỉ ảnh hưởng khi mẫu chạy tới trường; `{{if .Language.Code}}` trên struct rỗng là an toàn.
- **Golden**: 158 tệp ở sáu gói dịch vụ (`request-service/internal/usecase/testdata/prompt-golden/{BuildSolutionPrompt (36), BuildPlanPrompt (29), BuildOpenSpecPlanPrompt (18), BuildOpenSpecSolutionPrompt (5), BuildTaskPromptTemplate (3), TaskRangeFor (1), approvedAnalysisText (5), writeClarificationAnswers (6)}`, ...); `ci/check-prompt-golden.sh` chạy `TestPromptGolden_*` ở sáu gói; `checkPromptGoldens` (`prompt_golden_assertion_test.go:16`) lỗi nếu có tệp golden thừa. Ngân sách: `prompts/prompt_size_budget_test.go` (`maxPromptBytes`).
- **Claude giả** `tests/stages/e2e/orca-flows/fake_claude_cli/claude`: nhận prompt bằng `startswith` các câu mở đầu (`SOLUTION_MARK`, `PLAN_MARK`, `READONLY_MARK`) và loại đọc-chỉ bằng chuỗi `"kind":"<loại>"` trong phần **trước** `<request>` (`readonly_kind`); ghi mỗi lời gọi vào `calls.jsonl` (`record`) với `promptKind`; `_stub(ctx).calls()` đọc lại trong `suite_ai_stub_login.py`. Marker sửa đổi `[revised, round N, asked: …]` do suite `ARV` đọc lại (`analysis_revision_driver.py`).
- **Claude giả có bộ kiểm với validator thật**: `fake_claude_cli/verify-against-request-service.sh` chạy claude giả trên prompt dựng bằng bộ dựng thật và đưa đầu ra qua parser, validator thật (`go test -overlay`, `request_service_contract.overlay.go.txt`); tự kiểm của stub ở `fake_claude_cli/test_fake_claude_cli.py`.
- **Kỹ năng**: `skills/tdd-openspec-author/SKILL.md` (133 dòng) không nhắc ngôn ngữ; cài từ nhánh mặc định của `vnp-community/orca` bằng `npx skills add` (`docs/guides/request/knowledge-hub-setup.md:19-24`).
- **Lỗi byte**: `internal/domain/evidence.go:45-46` (`len(excerpt) > 4096` rồi `excerpt[:4096]`); `NewEvidence` không có nơi gọi (`grep` toàn `backend-go`), `EvidenceRepository` (`usecase/ports.go:86`) không có bản cài đặt.

### Correction relative to CR-REQ-046

1. Hai mục của CR "Điều khác…" (1.6) áp xuống solution: không sửa `solution_openspec` (không có nơi gọi chạy thật); `NewNativeEngine` chỉ cần một method cấu hình và một test.
2. Mẫu OpenSpec cần tên tệp riêng `_language_rule_openspec.tmpl` (đụng tên với `request_context/_language_rule.tmpl` nếu trùng).
3. Biến cấu hình không đọc qua `domain.EngineDefaultsFromEnv` (nó chỉ cảnh báo): tạo `parseOutputLanguage` riêng ở `cmd/server` trả lỗi.
4. Kiểu `Language` ở module `prompts` (cạnh văn bản chỉ thị), không ở `domain`, vì `domain` không import `prompts` (kiểm bằng `grep orca-go/prompts` trong `internal/domain`: không có).

## 2. Giải pháp

### 2.1 Cây file (đường dẫn dưới `backend-go/` trừ khi ghi khác)

```
prompts/output_language/output_language.go                                   (mới) Language, Vietnamese, English, Supported, Parse
prompts/output_language/output_language_test.go                              (mới)
prompts/request_context/_language_rule.tmpl                                  (mới) partial cho các prompt trả JSON
prompts/analysis/{solution,agent_solution,diagnosis,findings,answer}.tmpl   (sửa) gọi partial trước _body
prompts/analysis/analysis.go                                                 (sửa) Input.Language
prompts/plan_native/{plan.tmpl,plan_native.go}                               (sửa) gọi partial; Input.Language
prompts/plan_openspec/_language_rule_openspec.tmpl                           (mới)
prompts/plan_openspec/{plan.tmpl,plan_openspec.go}                           (sửa) gọi partial trước result_block; Input.Language
prompts/{analysis,plan_native,plan_openspec}/*_test.go                       (sửa) ca mới
prompts/prompt_size_budget_test.go, prompts/README.md                        (sửa)
services/request-service/
  internal/usecase/analysis_settings.go                                      (sửa) OutputLanguage, OutputLanguageFor
  internal/usecase/{solution_prompt,plan_prompt,openspec_plan_prompt}.go     (sửa) trường Language, ánh xạ
  internal/usecase/{run_solution_generation,run_agent_readonly_analysis,
     run_plan_generation,engine_openspec,engine_native}.go                   (sửa) đặt Language ở sáu nơi
  internal/config/config.go                                                  (sửa) đọc REQUEST_DEFAULT_OUTPUT_LANGUAGE
  cmd/server/output_language_wiring.go                                       (mới) parseOutputLanguage, dừng khởi động khi sai
  cmd/server/{main.go,wire_solution.go}                                      (sửa)
  internal/domain/evidence.go                                                (sửa) cắt ở ranh giới rune
  internal/usecase/testdata/prompt-golden/{BuildSolutionPrompt,BuildPlanPrompt,BuildOpenSpecPlanPrompt}/   (thêm tệp, không sửa)
  internal/adapter/contracttest/solution_contract.go                         (sửa) ca tài liệu tiếng Việt cỡ lớn
skills/tdd-openspec-author/SKILL.md                                          (sửa, repo gốc) mục Language
deploy/dev/{docker-compose.yml,.env.example,README.md,RELEASE-2026-10-DEPLOY.md}   (sửa, repo gốc)
tests/stages/e2e/orca-flows/{fake_claude_cli/{claude,test_fake_claude_cli.py,request_service_contract.overlay.go.txt},suite_output_language.py,run_acceptance.py,run_ui_flow.py,README.md}
tests/stages/e2e/orca-flows/ui-live/{analysis-output-language,analysis-output-language-real}.web.e2e.ts
```

### 2.2 Gói `prompts/output_language`

`type Language struct{ Code, Name string }`; giá trị `Vietnamese = Language{"vi", "Vietnamese"}`, `English = Language{"en", "English"}`; `Supported() []Language` (thứ tự cố định: `vi`, `en`); `Parse(raw string) (Language, error)`: cắt khoảng trắng, hạ chữ thường, khớp `Code`; rỗng là lỗi (người gọi quyết định mặc định); lỗi liệt kê mã hợp lệ. Giá trị zero `Language{}` nghĩa là "không chỉ thị" và là giá trị duy nhất mà mẫu coi là rỗng (`{{if .Language.Code}}`). Tên tiếng Anh vì chỉ thị viết bằng tiếng Anh.

### 2.3 Hai partial

`request_context/_language_rule.tmpl` (kết thúc bằng xuống dòng, như quy ước partial của `prompts/README.md`; rỗng khi `Language` rỗng):

```
{{if .Language.Code -}}
Output language: {{.Language.Name}}.
Write every string value that a person will read in {{.Language.Name}}, whatever language <request> uses: titles, summaries, descriptions, approaches, pros, cons, risks, assumptions, questions, answers, reasons and notes.
Never translate: JSON keys; the value of "kind" and every value the schema lists as a choice (written a|b|c); ids such as opt-1, A-1, Q-1, AC-1, fix-1, f-1; labels; file paths, file:line references, code, identifiers, commands and error messages; text you quote from the repository or from <request>, which stays in its original language.
{{end -}}
```

`plan_openspec/_language_rule_openspec.tmpl` (cùng kiểu): đoạn đã nêu ở CR mục 2.2 (văn xuôi bằng `{{.Language.Name}}`; danh sách phần giữ tiếng Anh: tên tệp/thư mục/capability và id thay đổi; tiêu đề `## Why`, `## What Changes`, `## Capabilities`, `## Impact`, `## ADDED|MODIFIED|REMOVED|RENAMED Requirements`, `### Requirement:`, `#### Scenario:`, `## PH-n`, `## TASKS`; `SHALL`/`MUST` trong mỗi câu yêu cầu; `WHEN`/`THEN`/`AND`, `**Reason**`, `**Migration**`; marker `orca:begin`/`orca:end`, nhãn `Serves:`/`Code:`/`Verify:`/`Done when:`, thuộc tính `[...]`, `modify`/`create`; id; đường dẫn, ký hiệu, mã, lệnh; khối `ORCA_RESULT_*`). Cả hai: dòng đầu cố định `Output language: <Name>.`; **không** viết `"kind":"<loại>"`.

Vị trí gọi: một dòng `{{template "_language_rule.tmpl" . -}}` ngay trước dòng gọi `_body.tmpl` (analysis) hoặc `_request_block.tmpl` (plan_native); với `plan_openspec/plan.tmpl` đặt ngay trước `{{template "_request_block.tmpl" . -}}`; khối `result_block.tmpl` giữ cuối. Vì partial bắt đầu bằng `{{if … -}}` và kết thúc `{{end -}}`, khi `Language` rỗng phần xuất ra là chuỗi rỗng và dấu `-}}` ăn đúng dòng mới của chính dòng gọi, nên đầu ra trùng từng byte với mẫu cũ (golden phải chứng minh).

### 2.4 Đầu vào, cấu hình và nơi gọi

- Ba `*Input` của `prompts` thêm `Language output_language.Language` (analysis: trong `Input`, struct `view` nhúng sẵn). Ba struct của service thêm `Language output_language.Language` và chuyển tiếp.
- `AnalysisSettings` thêm `OutputLanguage output_language.Language` và `func (s AnalysisSettings) OutputLanguageFor(domain.Request) output_language.Language { return s.OutputLanguage }` (một chỗ để thay khi có ghi đè theo dự án). `withDefaults` không đụng trường này.
- Sáu nơi gọi đặt `in.Language = settings.OutputLanguageFor(req)` ngay khi dựng đầu vào (trước vòng thử lại, để lần dựng lại ở `run_agent_readonly_analysis.go:141` cũng mang). `nativeEngine` thêm `WithOutputLanguage(l)`; `engine_openspec.go:225` dùng `e.d.Settings`.
- `internal/config.Config` thêm `OutputLanguage string` (giá trị thô, đã `TrimSpace`); `cmd/server/output_language_wiring.go`: `parseOutputLanguage(raw string) (output_language.Language, error)`: rỗng → `Vietnamese`; lỗi bọc thành `REQUEST_DEFAULT_OUTPUT_LANGUAGE "<giá trị>" is not supported; use one of: vi, en`. `run()` gọi nó **trước** `tracing.Init` và `openStores` (khởi động nhanh thất bại), ghi `log.Info("output language", slog.String("code", …))`, và `analysisSettings(cfg)` nhận ngôn ngữ đã phân tích (đổi chữ ký thành `analysisSettings(cfg, lang)` hoặc đặt trường sau khi gọi; `wire_solution_test.go:100-117` gọi `analysisSettings(config.Config{...})`, cập nhật theo).
- Không ghi nội dung Request hay phân tích vào log (C9); chỉ mã ngôn ngữ.

### 2.5 OpenSpec và kỹ năng

- Prompt `plan_openspec` mang `_language_rule_openspec.tmpl`. Kết quả mong đợi: `openspec validate --strict` xanh với văn xuôi tiếng Việt khi `SHALL`/`MUST` và tiêu đề được giữ (đã chạy thử cục bộ, CR mục 1.5).
- `skills/tdd-openspec-author/SKILL.md` thêm mục `## Language` (khoảng 6 dòng, sau `## Inputs you are given`): ngôn ngữ văn xuôi do prompt nêu; mọi thứ có ngữ pháp hoặc là từ khoá (bảng ở trên) giữ nguyên; không có chỉ thị thì theo ngôn ngữ của Request. Không đổi `tdd-knowledge`, `tdd-orca-task`, `tdd-knowledge-writeback`. Ghi vào PR rằng hiệu lực cần hợp nhất vào nhánh mặc định và `npx skills update tdd-openspec-author --global` trên từng dev server, và mã băm kỹ năng (`SkillHash`) của các thay đổi tạo từ nay sẽ khác.
- Test miền: một gói OpenSpec mẫu tiếng Việt (proposal, spec, tasks.md, cây kho giả) qua `ParsePlanRegion`, `CheckBundle` và, khi `openspec` có trong PATH, `openspec validate --strict`.

### 2.6 Sửa lỗi cắt byte

`NewEvidence`: nếu `len(excerpt) > 4096` thì `cut := 4096; for cut > 0 && !utf8.RuneStart(excerpt[cut]) { cut-- }; excerpt = excerpt[:cut]` (`excerpt[cut]` hợp lệ vì `cut < len`). Ngưỡng giữ theo byte để khớp `CHECK (octet_length(excerpt) <= 4096)`. Test: ASCII đúng 4096 và 4097; tiếng Việt (2 và 3 byte mỗi ký tự) với ranh giới rơi giữa ký tự; kết quả `utf8.ValidString` và `len <= 4096`; chuỗi ngắn không đổi.

### 2.7 Claude giả, suite `OLG` và kiểm tra thật

**Thay đổi tối thiểu của `fake_claude_cli/claude`:**

1. Hàm `language_directive(prompt)`: lấy phần trước `\n<request>\n`, tìm dòng `^Output language: (\w+)\.$`, ánh xạ `Vietnamese→vi`, `English→en`, không có → `""`.
2. `record(...)` của `run_print_stdin` và `run_print_arg` thêm `"outputLanguage": language_directive(prompt)` vào `extra` (calls.jsonl).
3. `solution_answer`, `plan_answer`, `readonly_answer`: khi mã là `vi`, các chuỗi cho người đọc (`title`, `summary`, `approach`, `pros`, `cons`, `rollback`, `statement`, `answer_markdown`, `description`, `notes`) lấy từ một bảng nhỏ `VI` (khoảng 15 chuỗi có đủ dấu, ví dụ "Phương án 1: mở rộng mô-đun hiện có"); id, enum, nhãn, khoá giữ nguyên; `note` của `revision_note` giữ nguyên chữ để suite `ARV` không đổi. Cập nhật docstring đầu tệp.

Kiểm bằng validator thật: ca `Language: Vietnamese` trong `request_service_contract.overlay.go.txt` (chạy `verify-against-request-service.sh`), để chuỗi tiếng Việt của stub không bao giờ lệch khỏi `Validate`.

Suite `suite_output_language.py` (tiền tố `OLG`, đăng ký sau `analysis_revision` trong `run_acceptance.py` `SUITES` và `AI_USING`; chỉ `--ai-stub`) với ca `OLG-01..06` như CR mục 5. Spec UI `ui-live/analysis-output-language.web.e2e.ts` (stub). Spec thật `ui-live/analysis-output-language-real.web.e2e.ts` + nhóm `"language": ["/analysis-output-language-real"]` trong `run_ui_flow.py` (lưu ý: Playwright đọc chuỗi này như regex trên đường dẫn tệp, xem chú thích ở `run_ui_flow.py:40`).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| S1 | Danh sách ngôn ngữ đóng ở `prompts/output_language`, không ở `domain` | `domain` không import `prompts`; tên hiển thị đi cùng văn bản chỉ thị |
| S2 | Hai partial (JSON và OpenSpec) | Hai ngữ pháp giữ-tiếng-Anh khác nhau; một partial lẫn lộn sẽ nói sai cho một trong hai |
| S3 | Dòng đầu cố định `Output language: <Name>.` | Claude giả và test đọc được không phụ thuộc văn bản còn lại |
| S4 | `Language` rỗng không phát gì; cấu hình sản xuất luôn khác rỗng | Golden hiện có là bằng chứng không đổi |
| S5 | Một hàm `OutputLanguageFor(request)` | Chỗ cắm duy nhất cho ghi đè theo dự án |
| S6 | Phân tích giá trị ở `cmd/server`, không ở `EngineDefaultsFromEnv` | Hàm đó cố ý chỉ cảnh báo; giá trị này phải dừng service |
| S7 | Không đổi `DefaultAnalysisSettings`/`withDefaults` | Test hiện có và golden dùng ngôn ngữ rỗng |
| S8 | Sửa `NewEvidence` giữ ngưỡng byte | Khớp CHECK `octet_length` ở Postgres |

## 4. Phụ thuộc và thứ tự

01 gói `output_language` và partial → 02 nối cấu hình và nơi gọi ở `request-service` → 03 golden và ngân sách → 04 (độc lập, làm bất kỳ lúc nào) sửa cắt byte và ca lưu trữ lớn → 05 OpenSpec: kỹ năng và kiểm tiếng Việt (cần 01) → 06 tài liệu triển khai (cần 02) → 07 claude giả, suite `OLG`, spec UI và `--real` (cần 02, 03, 06). 03 và 05 song song sau 02.

## 5. Kiểm thử

- Unit: như CR mục 5 (prompts, service, validator với văn bản có dấu, `NewEvidence`).
- Golden: `check-prompt-golden.sh` ở chế độ so sánh **xanh trước** khi thêm ca; sau `--update` chỉ có tệp thêm.
- Hợp đồng hai dialect: chạy lại bộ hiện có; thêm ca `solLargeVietnameseDocument` (bốn option, `approach` 4000 ký tự có dấu, `RawOutput` lớn) cho Postgres 16 và MySQL 8 qua testcontainers.
- Acceptance và UI: suite `OLG`, `ARV`, `AST`, `qa-bug-task-native-flow` (stub).
- `--real` (lớp chi phí B, opt-in): spec `analysis-output-language-real`.
- CI: `make check-prompts`; `go test ./... -race` trong `prompts` và `request-service`.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Blast radius**: `RenderSolutionPrompt` được năm nơi gọi và nhiều test; `AnalysisSettings` đến bốn worker. Chạy `gitnexus_impact` cho `RenderSolutionPrompt`, `RenderPlanPrompt`, `RenderOpenSpecPlanPrompt`, `AnalysisSettings`, `NewEvidence` trước khi sửa và ghi kết quả vào PR.
- Lệch byte của golden khi chèn dòng gọi partial (khoảng trắng): cổng golden bắt; sửa mẫu, không sửa golden.
- Tên tệp trùng giữa các `fs.FS` của bộ mẫu làm lỗi phân tích lúc chạy (`render.Set.load`): test kết xuất mọi mẫu bắt.
- Tiếng Việt tốn token hơn; mô hình có thể dịch nhầm enum; vị trí chỉ thị có thể bị thân dài lấn át; kỹ năng cũ trên dev server; tất cả chưa đo (CR mục 6).
- `analysisSettings` đổi chữ ký: cập nhật `wire_solution_test.go`.
- `parseOutputLanguage` chạy trước `openStores`: kiểm tra thứ tự bằng test khởi động theo mẫu `TestRun_UnknownDSNExitsWithDialectError` (`cmd/server/startup_test.go:36`).
- Chưa kiểm hành vi trên SSH/WSL: chỉ thị đi trong prompt nên không phụ thuộc máy chủ chạy agent; locale của git/hệ điều hành không liên quan.

## 7. Câu hỏi mở

Như CR-REQ-046 mục 7 (bốn câu). Mặc định tạm cho đến khi được trả lời: (1) không mở rộng sang `task_execution`, commit, PR, `task_decompose`, `task_agent_prompt`; (2) không lưu ngôn ngữ trên run/artifact; (3) chỉ thêm dòng nhắc cuối prompt nếu số đo `--real` cho thấy cần; (4) chỉ `vi` và `en`.

## 8. Tham chiếu

- `backend-go/prompts/{analysis,plan_native,plan_openspec,request_context,render}/`, `prompts/README.md`, `prompts/prompt_size_budget_test.go`
- `backend-go/services/request-service/internal/usecase/{solution_prompt,plan_prompt,openspec_plan_prompt,analysis_settings,run_solution_generation,run_agent_readonly_analysis,run_plan_generation,engine_openspec,engine_native}.go`, `cmd/server/{main.go,wire_solution.go,wire_plan.go}`, `internal/config/config.go`, `internal/domain/evidence.go`
- `backend-go/ci/check-prompt-golden.sh`, `Makefile` (`check-prompts`)
- `skills/tdd-openspec-author/SKILL.md`, `docs/guides/request/knowledge-hub-setup.md`
- `tests/stages/e2e/orca-flows/fake_claude_cli/claude`, `suite_ai_stub_login.py`, `analysis_revision_driver.py`, `run_acceptance.py`, `run_ui_flow.py`
- [CR-REQ-046](../../../../../../docs/crs/v6/ai-governance/CR-REQ-046-ai-output-language-system-setting-default-vietnamese.md), [CR-REQ-042](../../../../../../docs/crs/v6/ai-governance/CR-REQ-042-prompt-catalog-shared-agent-relay-and-stdin.md)
