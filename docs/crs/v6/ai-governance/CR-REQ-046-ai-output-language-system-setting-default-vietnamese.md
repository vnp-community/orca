# CR-REQ-046 — Ngôn ngữ đầu ra của AI là cấu hình hệ thống, mặc định tiếng Việt (tab Analysis và Plan hiện bằng tiếng Việt)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-046 |
| **Tên** | Biến cấu hình triển khai `REQUEST_DEFAULT_OUTPUT_LANGUAGE` (`vi` mặc định, `en`), một chỉ thị ngôn ngữ trong mọi prompt phân tích (Solution, chẩn đoán, Findings, Answer, câu hỏi mở), Plan native và Plan/đề xuất OpenSpec; khoá JSON, enum, id, đường dẫn và ngữ pháp OpenSpec giữ tiếng Anh; sửa lỗi cắt chuỗi theo byte làm hỏng UTF-8 |
| **Loại** | Feature (kèm sửa một lỗi cắt chuỗi theo byte, mục 2.6) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | ✅ Đã triển khai (2026-10-20), chưa merge; e2e stub xanh, chưa chạy thật hạng C |
| **Phụ thuộc** | CR-REQ-007, 008 (Solution và phân tích chỉ đọc), CR-REQ-012 (Plan native), CR-REQ-026 (engine OpenSpec), CR-REQ-041 (prompt phân tích có phản hồi và bản cũ), CR-REQ-042 (danh mục prompt `backend-go/prompts/`, golden, ngân sách kích thước) |
| **Mở khoá** | [CR-REQ-034](./CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md) (provenance có thể ghi ngôn ngữ đầu ra); ghi đè theo dự án hoặc theo Request về sau (mục 2.8) mà không sửa mẫu prompt |
| **Tác động** | `backend-go/prompts/` (gói mới `output_language`, hai partial mới, năm mẫu `analysis/`, `plan_native/plan.tmpl`, `plan_openspec/plan.tmpl`), `backend-go/services/request-service` (`internal/config`, `cmd/server`, `internal/usecase` ba struct đầu vào và sáu nơi gọi, `internal/domain/evidence.go`), `skills/tdd-openspec-author/SKILL.md`, `deploy/dev/` (compose, `.env.example`, README, tài liệu phát hành), `tests/stages/e2e/orca-flows/` (claude giả, suite `OLG`, spec UI). **Không** đổi proto, migration, kênh gateway, MCP hay frontend |

> Nguồn: phản hồi người dùng ngày 2026-10-20 (bug 2): "Tab Analysis phải hỗ trợ kết quả bằng tiếng Việt". Điều tra bằng đọc mã và chạy `openspec validate` trên máy cục bộ (phiên bản 1.3.0); chưa chạy AI thật nên chưa có số đo token hay mẫu đầu ra tiếng Việt thật (mục 6).

---

## 1. Bối cảnh và vấn đề

### 1.1 Hiện trạng: không prompt nào nói viết bằng ngôn ngữ nào

Mọi prompt AI gửi tới dev server là mẫu trong `backend-go/prompts/`. Đã tìm `vietnam`, `language`, `locale`, `english`, `tiếng` trong `*.tmpl`, `*.md`, `*.go` của module: chỉ `task_decompose` có trường `Languages []string`, và đó là danh sách ngôn ngữ lập trình của dự án, không liên quan.

| Luồng | Mẫu | Chỉ thị ngôn ngữ | Hậu quả |
|---|---|---|---|
| Solution (`ai.complete`) | `analysis/solution.tmpl` | Không | Mô hình theo ngôn ngữ của chỉ dẫn (tiếng Anh) và của `<request>` |
| Solution do agent viết | `analysis/agent_solution.tmpl` | Không | Như trên |
| Chẩn đoán, Findings, Answer | `analysis/{diagnosis,findings,answer}.tmpl` | Không | Như trên |
| Câu hỏi mở (Clarification) | không có mẫu riêng: là trường `open_questions` của tài liệu Solution/chẩn đoán | Không | Theo ngôn ngữ của tài liệu |
| Plan native | `plan_native/plan.tmpl` | Không | Tiêu đề, mô tả task bằng tiếng Anh |
| Plan/đề xuất OpenSpec | `plan_openspec/plan.tmpl` + kỹ năng `tdd-openspec-author`, `tdd-knowledge`, `tdd-orca-task` | Không (`skills/*/SKILL.md` không nhắc ngôn ngữ) | proposal, design, specs, tasks bằng tiếng Anh |
| Phân loại | `classify_request/classify.tmpl` | Không | Ngoài phạm vi: `reason` là một câu nội bộ |

Request tiếng Anh cho ra phân tích tiếng Anh; Request tiếng Việt thường ra tiếng Việt nhưng không chắc, và không ai điều khiển được.

### 1.2 Không có nơi nào lưu ngôn ngữ

- `request.tenant_settings` (`migrations/postgres/0090_tenant_settings.up.sql`) chỉ có `request_flow_enabled`.
- `request.project_engine_settings` (`internal/domain/engine_settings.go:11`, migration 0007, 0095, 0098) có `engine`, `solution_agent`, `execution_mode`.
- Không migration nào của `request-service` có cột `language` hay `locale` (đã tìm trong `migrations/postgres/*.up.sql`).
- Bộ chọn `vi`/`en` duy nhất trong mã là `ResolveLocale(Accept-Language)` của prompt MCP (`api-gateway/internal/adapter/mcpserver/prompts/locale.go:12`), dành cho client MCP bên ngoài, không đi qua dev server.
- Ngôn ngữ giao diện (`frontend/src/shared/ui-language.ts`) hỗ trợ `system`, `en`, `zh`, `ko`, `ja`, `es`; **không có `vi`**. `request-service` không bao giờ nhận locale của giao diện.

### 1.3 Vấn đề

1. Người dùng Việt Nam đọc tab Analysis (và Plan) bằng tiếng Anh dù Request của họ bằng tiếng Việt hoặc họ muốn tiếng Việt.
2. Không có cách nào, kể cả cho quản trị viên, đặt ngôn ngữ đầu ra của AI cho cả hệ thống.
3. Nếu chỉ thêm "hãy viết tiếng Việt" mà không giữ các khoá, enum, id và ngữ pháp OpenSpec bằng tiếng Anh thì validator sẽ từ chối tài liệu (mục 1.5), và một lỗi nhỏ ở lớp cắt chuỗi theo byte (mục 2.6) sẽ lộ ra với tiếng Việt trước.

### 1.4 Điều đã kiểm với văn bản tiếng Việt (đọc mã)

Mọi giới hạn độ dài của tài liệu AI đếm **ký tự (rune)**, không đếm byte, nên văn bản tiếng Việt qua được validator:

| Chỗ | Cách đếm | Vị trí |
|---|---|---|
| `runeLen` (tiêu đề option 120, tóm tắt 600, `approach` 4000) | `utf8.RuneCountInString` | `internal/domain/solution_options.go:150`, `:170`, `:173`, `:176` |
| Trích dẫn bằng chứng tối đa 600 | rune | `internal/domain/analysis_evidence.go:10`, `:29` (`maxEvidenceExcerpt`) |
| `answer_markdown` 1..8000 | rune | `internal/domain/answer_document.go:5`, `:42` |
| Plan: summary, notes, description, prompt_template, nhãn, tiêu đề | rune | `internal/domain/plan_validation.go:79`, `:112`, `:149`, `:159`, `:169` |
| Khối bao ngoài dữ liệu không tin cậy | `fence.Truncate` đếm rune | `backend-go/prompts/fence/fence.go:26` |
| Tách JSON từ đầu ra mô hình | quét dấu ASCII `{`, `}`, `"`, rồi `json.Valid` | `internal/usecase/solution_output_extraction.go:13` |
| Cắt đầu ra thô 256 KB | dừng ở ranh giới rune | `internal/domain/analysis_run.go` `TruncateRaw` (`:120-135`) |
| Cắt cửa sổ quét bí mật | bỏ byte cuối đến khi UTF-8 hợp lệ | `internal/domain/analysis_document.go:25-29` |
| Lưu trữ | Postgres `jsonb`/`text`, MySQL cột `JSON`, `MEDIUMTEXT` (raw_output) đều lưu UTF-8; test hợp đồng `UnicodeAndDigestSurviveTheDatabase` (`internal/adapter/contracttest/solution_contract.go:63`, `:867`) đã chạy văn bản tiếng Việt trên hai dialect | |

Ràng buộc theo **byte** duy nhất trên đường dữ liệu AI: `evidence.excerpt` (`CHECK (octet_length(excerpt) <= 4096)`, `migrations/postgres/0005_context_sources.up.sql:64`), và lỗi cắt chuỗi ở mục 2.6. Các giới hạn byte khác (`MaxArtifactBytes`, `internal/domain/artifact_kind.go:43`; `tasks.md` 1 MiB, `tasks_md_parser.go:32`; mỗi tệp OpenSpec đọc lại tối đa 256 KiB, `usecase/ports_workspace.go:7`; tổng gói 1 MiB, `run_proposal_generation.go:275`) co lại khoảng 1,5 lần theo ký tự với tiếng Việt, nhưng còn xa trần với tài liệu thông thường (chưa đo, mục 6).

### 1.5 Điều đã kiểm với `openspec validate` (tiếng Việt)

Chạy `openspec validate <id> --strict --json` (phiên bản 1.3.0 cài cục bộ, mã nguồn `@fission-ai/openspec/dist/core/validation/`) trên một thay đổi viết bằng tiếng Việt, ngày 2026-10-20:

| Biến thể | Kết quả | Kết luận |
|---|---|---|
| Văn bản tiếng Việt, tiêu đề `## Why`, `## What Changes`, `## ADDED Requirements`, câu yêu cầu có `SHALL`/`MUST`, `#### Scenario:` | `valid: true` | Văn bản tiếng Việt hợp lệ |
| Câu yêu cầu viết "phải" thay vì `SHALL`/`MUST` | `valid: false`: `ADDED "…" must contain SHALL or MUST` | Từ khoá `SHALL`/`MUST` phải còn bằng tiếng Anh (regex `\b(SHALL\|MUST)\b`, `validator.js:392`) |
| Tiêu đề delta dịch (`## Yêu cầu thêm mới`) | `valid: false`: không tìm thấy delta nào | Tiêu đề delta giữ tiếng Anh |
| `#### Kịch bản:` thay `#### Scenario:` | `valid: true` (validator chỉ đếm `^####\s+`, `validator.js:395`) **nhưng** Orca đòi đúng `#### Scenario:` để ghép tiêu chí chấp nhận (`scenarioNameRe`, `internal/domain/openspec_bundle.go:57`), nên mất phủ tiêu chí | Giữ `#### Scenario:` bằng tiếng Anh dù `openspec` không bắt |
| `**WHEN**`/`**THEN**` dịch thành `**KHI**`/`**THÌ**` | `valid: true` | `openspec` không kiểm; vẫn giữ tiếng Anh theo mẫu của `openspec` |
| Phần `## Why` dài hơn 1000 ký tự | `valid: true` | Giới hạn 50..1000 ký tự của `Why` (`constants.js`) không được `validate` ép ở 1.3.0 |

Ngữ pháp `tasks.md` là hợp đồng của Orca với parser chặt: `<!-- orca:begin plan request=<ref> schema=1 -->`, `## PH-n`/`## TASKS`, `T<p>.<n>`, thuộc tính `[type=…] [h=…] [ac=…] [labels=…] [depends=…]`, và dòng `Code:` có `modify`/`create` (`tasks_md_parser.go:16-21`, `openspec_bundle.go:56`). Tiêu đề task tối đa 200 rune (`tasks_md_parser.go:154`). Mã `NewChangeID` đã bỏ dấu tiếng Việt và đổi `đ` thành `d` khi sinh id thay đổi (`internal/domain/openspec_change.go:132-165`), nên id thay đổi vẫn là ASCII dù tiêu đề Request bằng tiếng Việt.

### 1.6 Điều khác với giả định ban đầu của người yêu cầu điều tra

- `solution_openspec/solution.tmpl` **không có nơi gọi trong mã chạy thật**: `solution_openspec.Render` chỉ xuất hiện trong test golden (`usecase/prompt_golden_openspec_test.go:103`) và test ngân sách kích thước; Solution của engine OpenSpec đi qua mẫu `analysis/*` (`engine_openspec.go:53` trả `ErrEngineStepNotSupported` cho `GenerateAnalysis`). Vì vậy chỉ thị cho Solution của OpenSpec chính là chỉ thị của `analysis/*`; không sửa `solution_openspec`.
- `NewNativeEngine` (`usecase/engine_native.go:25`) chỉ được test gọi; cmd/server không dựng nó. Vẫn truyền ngôn ngữ cho nhất quán (một test đủ), không có hành vi chạy thật thay đổi.
- `domain.NewEvidence` (`evidence.go:44`) không có nơi gọi và `EvidenceRepository` (`usecase/ports.go:86`) không có bản cài đặt trong cây mã hiện tại: lỗi cắt byte là có thật nhưng chưa chạm tới ở runtime. Vẫn sửa (mục 2.6) vì ràng buộc `octet_length` đang chờ nối vào và tiếng Việt là dữ liệu đầu tiên làm nó lộ ra.
- Golden: không có thư mục `backend-go/testdata/prompt-golden`. Golden nằm ở sáu gói dịch vụ (`request-service` hai gói, `task-service` ba gói, `git-gateway-service` một gói), tổng 158 tệp; chỉ ba hàm của `request-service/internal/usecase` bị chạm bởi CR này: `BuildSolutionPrompt`, `BuildPlanPrompt`, `BuildOpenSpecPlanPrompt`.
- Các kỹ năng không đi theo bản triển khai: `npx skills add https://github.com/vnp-community/orca --skill …` cài từ **nhánh mặc định** của repo vào `~/.claude/skills/` của từng dev server (`docs/guides/request/knowledge-hub-setup.md:19-24`). Sửa `skills/*/SKILL.md` chỉ có hiệu lực sau khi hợp nhất vào nhánh mặc định và chạy `npx skills update` trên từng dev server; đổi mã băm kỹ năng (`SkillHash`, `domain/dev_server_capability.go:60`).
- Mọi `REQUEST_DEFAULT_*` hiện có (`engine_settings.go:76`) xử lý giá trị sai bằng **cảnh báo và mặc định an toàn**, không dừng service. Biến mới theo quyết định của người dùng thì **dừng service** khi sai (mục 3, L2); đây là ngoại lệ cố ý và phải ghi vào tài liệu triển khai.
- "Lớp chi phí B" (kiểm tra `--real` một lượt phân tích) không được định nghĩa trong repo; CR này dùng theo brief của điều phối: một lượt phân tích chỉ đọc (loại `question`), chi phí chưa đo. Run OpenSpec thật (7 đến 13 USD) không nằm trong CR này.

## 2. Giải pháp đề xuất

### 2.1 Cấu hình `REQUEST_DEFAULT_OUTPUT_LANGUAGE`

- Tên theo họ `REQUEST_DEFAULT_ENGINE`, `REQUEST_DEFAULT_SOLUTION_AGENT`, `REQUEST_DEFAULT_EXECUTION_MODE` (`deploy/dev/docker-compose.yml:470-473`).
- Giá trị: `vi` (mặc định khi vắng hoặc rỗng) hoặc `en`; so sánh không phân biệt hoa thường và cắt khoảng trắng hai đầu. Danh sách đóng nằm ở **một nơi**: gói `backend-go/prompts/output_language` (`Language{Code, Name}`, `Vietnamese`, `English`, `Parse`, `Supported`), cạnh văn bản chỉ thị dùng tên đó. Thêm ngôn ngữ thứ ba là thêm một dòng ở gói này.
- Giá trị khác (`fr`, `vietnamese`, `vi-VN`): `request-service` **không khởi động**, thoát mã 1 trước khi mở kho dữ liệu, với thông báo `REQUEST_DEFAULT_OUTPUT_LANGUAGE "fr" is not supported; use one of: vi, en`. Lệnh `healthcheck` của container không bị ảnh hưởng.
- Mặc định `vi` áp cho cả binary lẫn `docker-compose.yml` (`${REQUEST_DEFAULT_OUTPUT_LANGUAGE:-vi}`). Hệ quả: sau lần triển khai tiếp theo mọi phân tích và Plan mới của mọi tenant ra tiếng Việt; phân tích và Plan đã tạo giữ nguyên.
- Giá trị được đọc một lần lúc khởi động, vào `AnalysisSettings.OutputLanguage` (cùng struct đã đi tới `RunSolutionGeneration`, `RunAgentReadonlyAnalysis`, `RunPlanGeneration`, `OpenSpecEngine`). Đổi biến cần `redeploy-local.sh request-service`.

### 2.2 Chỉ thị trong prompt

Một trường `Language output_language.Language` (kiểu cấu trúc, không phải bool) trên `analysis.Input`, `plan_native.Input`, `plan_openspec.Input`. **`Language` rỗng không phát gì**, nên mọi golden hiện có giữ nguyên từng byte; chỉ code đường chạy thật (`AnalysisSettings.OutputLanguage` do `cmd/server` đặt) mới bật chỉ thị. Mẫu mới:

- `request_context/_language_rule.tmpl` cho các prompt trả về một đối tượng JSON (năm mẫu `analysis/*` và `plan_native/plan.tmpl`): viết mọi chuỗi dành cho người đọc bằng `{{.Language.Name}}` bất kể ngôn ngữ của `<request>`; không dịch khoá JSON, giá trị `kind`, mọi giá trị enum của schema (`low|medium|high`, `S|M|L`, `covered|partial|out_of_scope`, `yes|no|unknown`, `file|log|command|commit`, `task|bug|feature|epic`, ...), id (`opt-N`, `A-N`, `Q-N`, `AC-N`, `fix-N`, `f-N`), nhãn (`test:regression`, `check:baseline`, `rollback`, ...), đường dẫn, tham chiếu `file:line`, mã, định danh, lệnh, thông báo lỗi; trích dẫn nguyên văn từ kho mã hoặc `<request>` giữ ngôn ngữ gốc.
- `plan_openspec/_language_rule_openspec.tmpl` (tên khác để không đụng tên tệp trong cùng bộ mẫu, vì `render.NewSet` đòi tên duy nhất qua `request_context.Files`): văn xuôi của `proposal.md`, `design.md`, câu yêu cầu và bước kịch bản trong specs, tiêu đề task và văn bản `Serves`, `Verify`, `Done when` bằng tiếng Việt; giữ tiếng Anh theo bảng 1.5: tên tệp/thư mục/capability, id thay đổi, mọi tiêu đề của mẫu `openspec` (`## Why`, `## What Changes`, `## Capabilities`, `## Impact`, `## ADDED|MODIFIED|REMOVED|RENAMED Requirements`, `### Requirement:`, `#### Scenario:`, `## PH-n`, `## TASKS`), các từ `SHALL` và `MUST` trong mỗi câu yêu cầu, `WHEN`/`THEN`/`AND`, `**Reason**`/`**Migration**`, nhãn dòng `Serves:`/`Code:`/`Verify:`/`Done when:`, thuộc tính `[...]`, từ `modify`/`create` trong dòng `Code:`, id (`AC-n`, `T1.1`, id yêu cầu của đồ thị tri thức), đường dẫn, ký hiệu, mã, lệnh và khối `ORCA_RESULT_*`.
- Vị trí: ngay trước khối dữ liệu (`_body.tmpl` / `_request_block.tmpl`), là dòng cuối của phần chỉ dẫn; với OpenSpec đặt trước `result_block.tmpl` để "không gì sau khối kết quả" vẫn đúng. Hiệu quả của vị trí này chưa kiểm bằng AI thật (mục 6, R2).
- Dòng đầu của chỉ thị có dạng cố định `Output language: Vietnamese.` để claude giả và test đọc được mà không phụ thuộc phần còn lại.
- Cạm bẫy đã thấy ở claude giả: nó nhận loại phân tích bằng chuỗi `"kind":"diagnosis"` trong phần trước `<request>` (`fake_claude_cli/claude`, `readonly_kind`). Chỉ thị chỉ được nhắc `"kind"` đứng riêng, không bao giờ viết `"kind":"<loại>"`.

### 2.3 Phạm vi

| Áp dụng | Không áp dụng trong CR này |
|---|---|
| Solution (`ai.complete` và do agent viết), chẩn đoán, Findings, Answer; `open_questions`, `assumptions`, `design_references.gaps` | `classify_request` (`reason` là câu nội bộ) |
| Plan native (`plan_native`) | `task_execution` (agent chạy task, khối kết quả `summary`/`questions`), `task_prompt_template` |
| Plan/đề xuất OpenSpec (`plan_openspec`) và văn xuôi do kỹ năng viết | `task_decompose`, `task_agent_prompt`, `git_commit_message`, `git_pull_request`, `git_agent_prompt` |
| `clarification_answers` không cần chỉ thị (chỉ phát lại câu trả lời của người duyệt) | Prompt MCP (`api-gateway/.../prompts/builtin/*.tmpl`, đã có `vi`/`en` theo `Accept-Language`) |

Câu hỏi mở 1 hỏi có mở rộng sang cột phải không.

### 2.4 Kỹ năng OpenSpec

- Chỉ thị trong prompt (`plan_openspec`) là nguồn chính vì đi cùng mỗi lần chạy. `skills/tdd-openspec-author/SKILL.md` thêm một mục ngắn "Language": ngôn ngữ văn xuôi do prompt nêu; ngữ pháp, tiêu đề, từ khoá giữ nguyên; khi prompt không nêu ngôn ngữ thì viết theo ngôn ngữ của Request. `tdd-knowledge` và `tdd-orca-task` không đổi (chỉ đọc đồ thị và báo cáo; `tdd-orca-task` cũng được agent thực thi task dùng, nằm ngoài phạm vi).
- Hiệu lực của sửa kỹ năng cần hợp nhất vào nhánh mặc định và `npx skills update` trên từng dev server; trước đó prompt vẫn đủ. Ghi vào tài liệu triển khai (task 06).

### 2.5 Không phát hiện ngôn ngữ, không theo giao diện

Ngôn ngữ giao diện của người dùng không điều khiển ngôn ngữ AI (backend không nhận locale giao diện; một người dùng đổi giao diện sẽ làm đổi nội dung chung của cả nhóm). Ngôn ngữ của thân Request không được phát hiện (phát hiện ngôn ngữ không chắc với văn bản ngắn hoặc trộn Anh-Việt, và khó kiểm thử). Cả hai là quyết định cuối của người dùng.

### 2.6 Sửa lỗi cắt chuỗi theo byte

`domain.NewEvidence` (`internal/domain/evidence.go:45-46`): `if len(excerpt) > 4096 { excerpt = excerpt[:4096] }` cắt theo byte và có thể để lại nửa ký tự (UTF-8 không hợp lệ; cột Postgres `TEXT` từ chối, và `octet_length` ≤ 4096 là ràng buộc theo byte, nên phải giữ ngưỡng byte nhưng cắt ở ranh giới rune). Sửa: lùi về ranh giới rune gần nhất không vượt 4096 byte; thêm test với chuỗi tiếng Việt chạm đúng ngưỡng. Đã rà các chỗ cắt `s[:n]` khác trong `request-service` (`domain/request_activity.go:306`, `usecase/get_execution_summary.go:279`, `usecase/report_task_outcome.go:138`, `domain/analysis_run.go:120`, `domain/analysis_document.go:25`): đều dùng `ToValidUTF8`/`utf8.ValidString` nên hợp lệ UTF-8, chỉ cho ra ít ký tự hơn (400 byte ≈ 150 đến 200 ký tự tiếng Việt); không đổi. `api-gateway/.../mcpserver/tools/executor.go:435` (`shorten`, `s[:200]`) cắt thông báo chính sách, không phải nội dung AI; không đổi.

### 2.7 Frontend: không có task

Không cần đổi frontend để nội dung hiển thị đúng; không viết solution hay task frontend cho CR này. Lý do đã kiểm:

1. Nội dung phân tích hiển thị nguyên văn qua `RequestMarkdownContent`, `SolutionOptionCard`, `DiagnosisView`, `FindingsView`, `AnswerView` (`frontend/src/renderer/src/components/request/solution/`); không có biến đổi chữ hoa/thường, cắt theo byte hay độ rộng cố định lên nội dung AI. `uppercase` chỉ có ở nhãn tĩnh (`RequestRelatedTab.tsx:81`); `truncate` ở `SolutionVersionSwitcher.tsx:150` là cắt bằng CSS theo độ rộng; `RequestMarkdownContent.tsx:16,58` thu gọn theo `length` UTF-16 (4000), đúng với tiếng Việt dựng sẵn (cắt nửa cặp dấu chỉ có thể xảy ra với văn bản dạng NFD, chỉ là thẩm mỹ).
2. Font `Geist-Variable.woff2` (`frontend/src/renderer/src/assets/main.css:10-16`) có đủ chữ cái tiếng Việt: tôi giải mã bảng `cmap` (726 glyph) trên máy này và kiểm 146 ký tự (mọi nguyên âm tiếng Việt với đủ năm dấu thanh, chữ hoa và thường, cùng `đ`/`Đ`); thiếu 0 ký tự, có cả dấu kết hợp U+0300 và U+0323.
3. Tiêu đề mục và nhãn của giao diện đi theo `translate('auto.components.request.<Component>.<key>', 'English default')`, tức theo ngôn ngữ giao diện, không theo ngôn ngữ nội dung: dưới giao diện tiếng Anh, nội dung tiếng Việt sẽ đi cùng tiêu đề tiếng Anh. Đây là hành vi chấp nhận được và ghi vào hướng dẫn.
4. **Tùy chọn, tách riêng, ngoài phạm vi:** thêm ngôn ngữ giao diện `vi` (`frontend/src/shared/ui-language.ts`, `frontend/src/renderer/src/i18n/locales/vi.json`, các bài kiểm phủ khoá như `solution-locale-coverage.test.ts` đang ghim năm locale). Nên làm ở một CR frontend riêng nếu người dùng muốn giao diện Việt hoàn toàn.

### 2.8 Mở rộng về sau (không làm ở CR này)

Ghi đè theo dự án (cột trong `request.project_engine_settings` cạnh `execution_mode`, qua `NewDefaultedEngineSettings`) hoặc theo Request. Thiết kế sẵn chỗ cắm: ba struct đầu vào mang `Language` kiểu cấu trúc, và mọi nơi gọi lấy ngôn ngữ qua **một** hàm `AnalysisSettings.OutputLanguageFor(domain.Request)` (hiện chỉ trả giá trị cấu hình). Khi cần ghi đè, chỉ hàm đó và nguồn dữ liệu đổi; mẫu prompt và golden không đổi.

### 2.9 Golden, ngân sách và CI

- Mẫu đổi nên golden phải được giữ nguyên **với `Language` rỗng**: bước kiểm đầu tiên của task 01 là chạy `backend-go/ci/check-prompt-golden.sh` (chế độ so sánh, không `--update`) và thấy xanh.
- Thêm ca mới (chỉ thêm tệp, không sửa tệp cũ): `BuildSolutionPrompt` (vi cho `solution_ai_complete`, `solution_agent_knowledge`, `diagnosis_bug`, `findings`, `answer`; en cho `solution_ai_complete`), `BuildPlanPrompt` (vi cho `shape_plan_phases` và `type_bug_plan_tasks`; en cho `shape_plan_tasks`), `BuildOpenSpecPlanPrompt` (vi cho `phases_medium_knowledge`; en cho `tasks_medium_no_knowledge`). Khoảng 11 tệp mới (158 → 169). `check-prompt-golden.sh --update` chỉ chạy để sinh các tệp mới; `git diff --stat` phải chỉ có tệp thêm (`A`), không tệp sửa (`M`).
- `prompts/prompt_size_budget_test.go`: đặt `Language` ở fixture để ngân sách phủ phần chỉ thị (thêm khoảng 1 KB, trong biên 10 đến 20% hiện có; số đo ghi vào `prompts/README.md`).
- `make check-prompts` và job `prompt-catalog` phải xanh.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| L1 | Ngôn ngữ đầu ra là cấu hình hệ thống (triển khai), không theo người dùng, dự án hay Request (quyết định cuối của người dùng) | Một nơi cấu hình; không migration, không đổi proto và gateway |
| L2 | Mặc định `vi`; tập đóng `vi`, `en`; giá trị sai làm service không khởi động | Sai ngôn ngữ âm thầm là lỗi người dùng thấy ngay; khác `REQUEST_DEFAULT_ENGINE` (chỉ cảnh báo) có chủ ý |
| L3 | Ngôn ngữ giao diện không điều khiển ngôn ngữ AI; không phát hiện ngôn ngữ thân Request | Backend không có locale giao diện; phát hiện không chắc và khó kiểm thử |
| L4 | Chỉ thị nằm trong prompt, không ở `--system-prompt` của `ai.complete` hay `CLAUDE.md` của repo | Prompt là chỗ duy nhất đi qua cả `ai.complete` và `agent.execPrompt`, và golden hoá được |
| L5 | `Language` rỗng không phát gì; golden hiện có giữ nguyên byte | Cổng golden của CR-REQ-042 vẫn là bằng chứng "chỉ thêm" |
| L6 | `Language` là cấu trúc `{Code, Name}`; mọi nơi gọi lấy qua `OutputLanguageFor(request)` | Mở rộng theo dự án sau này không đụng mẫu |
| L7 | Khoá, enum, id, nhãn, đường dẫn, ngữ pháp OpenSpec và `SHALL`/`MUST` giữ tiếng Anh; validator không đổi và không nhận enum tiếng Việt | Validator ép `^opt-[0-9]+$`, `^A-[1-9][0-9]*$`, `^Q-[1-9][0-9]*$`, enum cố định, và `openspec validate` ép `SHALL`/`MUST` và tiêu đề delta |
| L8 | Không sửa `solution_openspec` | Không có nơi gọi chạy thật; thêm khi có |
| L9 | Phạm vi như bảng 2.3; mở rộng là câu hỏi mở 1 | Giữ CR nhỏ; người dùng chỉ yêu cầu tab Analysis và Plan |
| L10 | Chỉ thị trong prompt là nguồn chính, kỹ năng chỉ thêm một đoạn | Kỹ năng phân phối qua `npx skills` từ nhánh mặc định, không theo bản triển khai |
| L11 | Sửa lỗi cắt byte `NewEvidence` kèm test dù chưa có nơi gọi | Lỗi thật, sửa rẻ; tiếng Việt là dữ liệu làm nó lộ ra |
| L12 | Không lưu ngôn ngữ trên run hay artifact; ghi mã ngôn ngữ vào log khởi động | Không migration; provenance thuộc CR-REQ-034 |
| L13 | Claude giả sinh chuỗi tiếng Việt khi chỉ thị là `vi` và ghi `outputLanguage` vào `calls.jsonl`; marker `[revised, round N, asked: …]` giữ nguyên | Để suite xác nhận chỉ thị tới mô hình và văn bản có dấu đi hết đường lưu và hiển thị, không phá suite `ARV` |

## 4. Tiêu chí chấp nhận

- [ ] Không đặt biến: `request-service` dùng `vi`; mọi prompt Solution, chẩn đoán, Findings, Answer, Plan native và Plan OpenSpec mới có dòng `Output language: Vietnamese.` trước khối dữ liệu.
- [ ] `REQUEST_DEFAULT_OUTPUT_LANGUAGE=en` cho `Output language: English.`; `VI ` (hoa, khoảng trắng) được nhận là `vi`; `fr`, `vietnamese`, `vi-VN` làm service thoát mã 1 với thông báo nêu biến, giá trị và tập hợp lệ, trước khi mở kho dữ liệu.
- [ ] Với `Language` rỗng, golden hiện có (158 tệp) trùng từng byte; golden chỉ thêm tệp mới, không sửa tệp cũ; `make check-prompts` và test ngân sách kích thước xanh.
- [ ] Mẫu JSON nói rõ giữ tiếng Anh: khoá, `kind`, enum, id `opt-N`, `A-N`, `Q-N`, `AC-N`, `fix-N`, `f-N`, nhãn Plan, đường dẫn và `file:line`; mẫu OpenSpec nói rõ giữ tiếng Anh các tiêu đề ngữ pháp, `SHALL`/`MUST`, `#### Scenario:`, nhãn dòng task, từ `modify`/`create`, `ORCA_RESULT_*`.
- [ ] Văn bản tiếng Việt có dấu (kể cả sát trần 4000 rune của `approach`) qua `SolutionOptions.Validate`, `ValidateByKind`, `ValidatePlanProposal`, `ParsePlanRegion` và `CheckBundle`; một gói OpenSpec tiếng Việt mẫu qua `openspec validate --strict` khi CLI có mặt.
- [ ] `NewEvidence` không bao giờ trả `excerpt` UTF-8 không hợp lệ và không vượt 4096 byte.
- [ ] Claude giả ghi `outputLanguage` vào `calls.jsonl`; suite `OLG` xác nhận chỉ thị tới mọi lời gọi phân tích và Plan, Solution lưu có dấu tiếng Việt, id và enum vẫn tiếng Anh, và tab Analysis hiển thị đúng.
- [ ] Một lần chạy `--real` (lớp chi phí B) trên Request tiếng Anh cho Answer bằng tiếng Việt, JSON hợp lệ, khoá tiếng Anh; kết quả đo (token so với tiếng Anh, có chạm giới hạn ký tự không) ghi vào PR. Nếu chưa chạy thì ghi rõ "chưa kiểm chứng" ở trạng thái.
- [ ] Tài liệu triển khai (`docker-compose.yml`, `.env.example`, `deploy/dev/README.md`, `RELEASE-2026-10-DEPLOY.md` hoặc tài liệu phát hành kế tiếp) nêu biến, mặc định `vi`, việc sai giá trị làm service không khởi động, và tác động tới tenant hiện có.
- [ ] Không file nào vượt `max-lines`; không thêm `max-lines` disable.

## 5. Kiểm thử

- **Unit, `backend-go/prompts`:** `output_language` (`Parse` hợp lệ, hoa/thường, khoảng trắng, sai, rỗng; `Supported` ổn định thứ tự); mỗi luồng: `Language` rỗng không phát gì, `vi` và `en` phát đúng một lần, chỉ thị nằm trước `<request>`, không chứa chuỗi `"kind":"` kèm loại; văn bản thù địch (thẻ đóng trong mọi trường) không đổi; ngân sách kích thước.
- **Unit, `request-service`:** ánh xạ ba struct đầu vào; sáu nơi gọi (`run_solution_generation.go:59`, `engine_native.go:52`, `run_agent_readonly_analysis.go:125` và `:141` kể cả lần dựng lại không có đồ thị, `run_plan_generation.go:80`, `engine_openspec.go:225`) đều mang ngôn ngữ cấu hình; cấu hình (`unset`, `vi`, `en`, `VI `, sai); khởi động thất bại với giá trị sai; `domain.NewEvidence` (ASCII, tiếng Việt ở ranh giới, đúng 4096, 4097).
- **Tài liệu tiếng Việt qua validator:** `SolutionOptions` với `approach` 4000 rune có dấu (vừa trần, 4001 thì bị từ chối); `ValidatePlanProposal` với tiêu đề 200 rune có dấu; `tasks.md` tiếng Việt qua `ParsePlanRegion` và `CheckBundle`; gói OpenSpec mẫu qua `openspec validate --strict` (bỏ qua có thông báo nếu thiếu CLI).
- **Hợp đồng Postgres 16 và MySQL 8:** lưu trữ không đổi, **không migration, không test hợp đồng mới bắt buộc**. Chạy lại bộ hợp đồng Solution hiện có (đã gồm `UnicodeAndDigestSurviveTheDatabase`); thêm một ca tài liệu bốn option với `approach` 4000 ký tự có dấu để chứng minh không chạm trần cột trên hai dialect (task 04).
- **Golden:** như mục 2.9.
- **Claude giả với validator thật:** mở rộng `tests/stages/e2e/orca-flows/fake_claude_cli/request_service_contract.overlay.go.txt` (chạy bằng `verify-against-request-service.sh`) để chuỗi tiếng Việt của claude giả đi qua `SolutionOptions.Validate`, `ValidatePlanProposal`, `ValidateByKind` thật; tự kiểm ở `fake_claude_cli/test_fake_claude_cli.py`.
- **Acceptance (stack local, `--ai-stub`, claude giả):** suite mới `OLG-01..06` (mục dưới); chạy lại `request`, `matrix`, `analysis`, `plan`, `analysis_revision` (`ARV`), `aistub` (`AST`) để chắc claude giả đổi không làm hồi quy.
  - `OLG-01` mọi lời gọi `solution`, `plan`, `readonly:*` trong `calls.jsonl` có `outputLanguage == "vi"` (mặc định compose).
  - `OLG-02` Request tiếng Anh trên fixture LedgerLite: Solution lưu có chuỗi tiếng Việt có dấu; `opt-1`, `A-1`, `severity`, `size` vẫn tiếng Anh và document hợp lệ.
  - `OLG-03` Plan native: tiêu đề tiếng Việt, nhãn `test:regression` tiếng Anh, Plan qua validator và duyệt được.
  - `OLG-04` chẩn đoán và Answer (đọc chỉ đọc) bằng tiếng Việt, schema hợp lệ.
  - `OLG-05` thân Request tiếng Việt và thân tiếng Anh cho cùng chỉ thị (không phát hiện ngôn ngữ).
  - `OLG-06` chỉ chạy khi `ORCA_E2E_EXPECT_OUTPUT_LANGUAGE=en` (stack đã đặt biến này): chỉ thị `English`, đầu ra giả tiếng Anh. Suite không đổi được môi trường dịch vụ nên ca này là thủ công, `SKIP` có lý do khi không đặt.
- **Spec UI sống (stub, fixture LedgerLite):** `ui-live/analysis-output-language.web.e2e.ts`: mở tab Analysis của Request đã phân tích tiếng Việt; chữ có dấu hiện nguyên vẹn, không bị cắt hay đổi hoa; tiêu đề mục là nhãn giao diện tiếng Anh. Lớp chi phí A (không AI thật).
- **`--real` (lớp chi phí B, opt-in):** `ui-live/analysis-output-language-real.web.e2e.ts`, nhóm `run_ui_flow.py --real language`: Request `question` bằng tiếng Anh trên repo tạm; Answer thật hợp lệ, `answer_markdown` chủ yếu tiếng Việt (số ký tự có dấu trên ngưỡng và các từ thường gặp "và", "của", "không" xuất hiện), khoá tiếng Anh, run `succeeded`. Ghi `Usage` của run (`RunStats`, `internal/domain/run_stats.go:25`) để so với một lần chạy `en`. Kiểm tra theo heuristic, kèm đọc tay một đầu ra. **Run OpenSpec thật (7 đến 13 USD) không nằm trong CR này và cần đồng ý riêng.**
- **Thủ công:** `REQUEST_DEFAULT_OUTPUT_LANGUAGE=xx` rồi `docker compose up request-service` phải dừng với thông báo rõ.

## 6. Rủi ro và điểm chưa kiểm chứng

- **R1: tiếng Việt tốn nhiều token hơn và mô hình có thể bị cắt giữa chừng.** Các giới hạn tính theo ký tự (tiêu đề 120, tóm tắt 600, `approach` 4000, `answer_markdown` 8000) có thể chạm sớm hơn, và đầu ra dài hơn có thể gặp trần token của `claude -p`. Chưa đo: không biết tỷ lệ token Việt/Anh của mô hình đang dùng, không biết trần đầu ra. Giảm thiểu: một lần thử lại sửa lỗi đã có, và lời nhắc "approach dưới 3500 ký tự" trong `_agent_coverage_rule.tmpl`; hotfix có ngân sách 5 phút (`HotfixTimeoutMS`) có thể chật hơn. Phép đo thuộc `--real` (mục 5).
- **R2: chỉ thị nằm trước dữ liệu có thể bị thân Request dài lấn át**, hoặc `CLAUDE.md` của repo đích có chỉ thị ngôn ngữ riêng thắng. Chưa kiểm. Nếu `--real` cho đầu ra tiếng Anh với thân dài, chuyển vị trí partial xuống sau `_body` (đổi một dòng mỗi mẫu, golden sinh lại).
- **R3: sau khi đổi cấu hình từ `en` sang `vi`, prompt tạo lại chứa các phân tích cũ bằng tiếng Anh** (`prior_artifacts`). Chỉ thị phải thắng; chưa kiểm.
- **R4: mô hình dịch cả enum hoặc khoá** (`"mức": "thấp"`): validator từ chối và lần thử lại sửa, nhưng nếu lặp lại thì run lỗi. Chỉ thị nêu rõ; mức độ xảy ra chưa đo.
- **R5: văn bản dạng NFD (dấu kết hợp)** đếm nhiều rune hơn dạng NFC và chạm giới hạn rune sớm hơn. Chưa thấy trường hợp thật; không chuẩn hoá ở CR này.
- **R6: kích thước theo byte của gói OpenSpec** (tệp ≤ 256 KiB, tổng ≤ 1 MiB, `tasks.md` ≤ 1 MiB) co lại khoảng 1,5 lần theo ký tự; còn xa trần với thay đổi thông thường nhưng chưa đo.
- **R7: kỹ năng đã cài trên dev server không có đoạn "Language"** cho đến khi cập nhật. Prompt vẫn đủ; hành vi khi kỹ năng và prompt bất đồng (ví dụ kỹ năng viết "Why" bằng tiếng Anh theo mẫu `openspec instructions`) chưa kiểm. Giảm thiểu: chỉ thị OpenSpec nêu rõ phần nào là văn xuôi.
- **R8: văn bản tiếng Việt trong mô tả task có thể lan sang agent thực thi** (chú thích mã, thông điệp commit bằng tiếng Việt), vì prompt `task_execution` nằm ngoài phạm vi. Chưa quan sát; xem câu hỏi mở 1.
- **R9: thông báo lỗi và nhãn hệ thống** (`REQUEST_*`, cảnh báo cổng duyệt, hoạt động) vẫn tiếng Anh và giao diện đi theo ngôn ngữ giao diện; một màn hình sẽ trộn hai ngôn ngữ cho đến khi có locale `vi` của giao diện.
- **R10: mặc định `vi` đổi hành vi của mọi triển khai** ở lần nâng cấp kế tiếp (tenant đang chạy thấy phân tích mới bằng tiếng Việt). Ghi vào ghi chú phát hành; đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=en` để giữ hành vi cũ gần đúng (chỉ gần đúng: với `en` chỉ thị tiếng Anh vẫn được phát, ép tiếng Anh ngay cả khi Request bằng tiếng Việt).
- **R11: chạm cùng tệp với CR đang soạn khác.** CR-REQ-045 (task 06) cũng sửa `plan_native/plan.tmpl`, `plan_openspec/plan.tmpl`, `prompt_size_budget_test.go` và golden Plan; CR-REQ-043 (task 06) và CR-REQ-044 (task 09) cũng sửa `fake_claude_cli/claude`. Hợp nhất theo dòng gọi partial và theo hàm; sinh golden sau khi các mẫu đã hợp nhất, không sinh ở nhánh riêng rồi ghép tệp golden.
- Chưa chạy hệ thống thật cho CR này. Đã chạy cục bộ: `openspec validate` (mục 1.5) và giải mã bảng `cmap` của font (mục 2.7).

## 7. Câu hỏi còn mở

1. Có mở rộng chỉ thị sang các prompt hiện nằm ngoài phạm vi và người dùng cũng đọc: khối kết quả task (`summary`, `questions` của `task_execution`, hiện trong tab Execution), thông điệp commit và mô tả PR (`git_commit_message`, `git_pull_request`), `task_decompose`, `task_agent_prompt`? Đề xuất: làm ở CR sau sau khi xem đầu ra `--real` của CR này; riêng commit và PR có thể muốn giữ tiếng Anh theo quy ước của từng repo.
2. Có ghi ngôn ngữ đã dùng lên run hoặc artifact (để màn hình báo "AI đã viết bằng vi" và để kiểm toán khi cấu hình đổi) không? Đề xuất: không ở CR này (cần migration), giao cho provenance của CR-REQ-034.
3. Nếu `--real` cho thấy chỉ thị trước dữ liệu bị lấn át (R2), có chấp nhận đặt thêm một dòng nhắc ở cuối prompt (sau `_body`, trước khối kết quả OpenSpec) không? Đề xuất: có, nếu số đo cho thấy cần.
4. Có thêm ngôn ngữ thứ ba (ví dụ `ja`, `zh`) vào tập đóng ngay không? Đề xuất: không; tập đóng được thiết kế để thêm bằng một dòng khi có nhu cầu.

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-007, 008 | Ghi chú: prompt phân tích mang chỉ thị ngôn ngữ đầu ra; khoá, enum, id giữ tiếng Anh |
| CR-REQ-012 | Ghi chú: Plan native mang chỉ thị; nhãn Plan giữ tiếng Anh |
| CR-REQ-026 | Ghi chú: chỉ thị OpenSpec và danh sách phần giữ tiếng Anh (mục 1.5); Solution của engine OpenSpec đi qua `analysis/*` |
| CR-REQ-034 | Provenance có thể ghi `output_language` khi làm `PromptRegistry` (câu hỏi mở 2); `ID` của prompt không đổi vì `Language` là tham số, không phải mẫu mới |
| CR-REQ-041 | Ghi chú: phản hồi và bản cũ của lần chỉnh sửa vẫn đi qua `_body.tmpl`; chỉ thị đặt trước nó |
| CR-REQ-042 | Ghi chú: `README.md` của `prompts/` có thêm gói `output_language` và mục "Output language"; golden thêm tệp, không sửa |

## 9. Tham chiếu

- `backend-go/prompts/analysis/{solution,agent_solution,diagnosis,findings,answer}.tmpl`, `analysis/analysis.go` (`Input`), `plan_native/plan.tmpl`, `plan_openspec/plan.tmpl`, `request_context/`, `render/render.go`, `prompts/README.md`, `prompt_size_budget_test.go`
- `backend-go/services/request-service/internal/usecase/{solution_prompt.go:10,30, plan_prompt.go:12,28, openspec_plan_prompt.go:12,45, run_solution_generation.go:59, run_agent_readonly_analysis.go:125,141, run_plan_generation.go:71,80, engine_openspec.go:225, engine_native.go:52, analysis_settings.go}`, `cmd/server/{main.go, wire_solution.go:186}`, `internal/config/config.go`, `internal/domain/{evidence.go:45, engine_settings.go:76, solution_options.go, analysis_evidence.go, plan_validation.go, openspec_bundle.go, tasks_md_parser.go}`
- `backend-go/ci/check-prompt-golden.sh`, `Makefile` (`check-prompts`), `.github/workflows/backend-go-request-contracts.yml` (`prompt-catalog`)
- `skills/tdd-openspec-author/SKILL.md`, `docs/guides/request/knowledge-hub-setup.md`, `agent/src/relay/agent-capability-skills.ts`
- `deploy/dev/docker-compose.yml:463-475`, `deploy/dev/.env.example:221-228`, `deploy/dev/README.md`, `deploy/dev/RELEASE-2026-10-DEPLOY.md` (mục 4.1)
- `tests/stages/e2e/orca-flows/fake_claude_cli/claude`, `suite_ai_stub_login.py`, `ui-live/readonly-real.web.e2e.ts`, `run_ui_flow.py`
- [CR-REQ-007](../solution-analysis/CR-REQ-007-solution-generation-options-and-selection.md), [CR-REQ-008](../solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md), [CR-REQ-012](../plan-phase-task/CR-REQ-012-plan-phase-task-generation-from-solution.md), [CR-REQ-026](../solution-engines/CR-REQ-026-openspec-solution-engine.md), [CR-REQ-034](./CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md), [CR-REQ-041](../solution-analysis/CR-REQ-041-analysis-revision-by-prompt.md), [CR-REQ-042](./CR-REQ-042-prompt-catalog-shared-agent-relay-and-stdin.md)
- Mới: `backend-go/prompts/output_language/`, `backend-go/prompts/request_context/_language_rule.tmpl`, `backend-go/prompts/plan_openspec/_language_rule_openspec.tmpl`, biến `REQUEST_DEFAULT_OUTPUT_LANGUAGE`, suite `tests/stages/e2e/orca-flows/suite_output_language.py`
