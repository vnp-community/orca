# SOL-TDD-002: tdd-agent là knowledge hub; agent do Orca gọi tự khai thác knowledge; `plan generate` trả bộ OpenSpec

**Bản 2 (2026-10-14).** Bản 1 để Orca tự truy vấn KG và nhét khối ngữ cảnh vào prompt; người dùng bác bỏ điểm đó.

**Yêu cầu của người dùng (2026-10-14):**
1. Rà soát tdd-agent và viết bộ `SKILL.md` tương ứng.
2. Khai thác knowledge của tdd qua MCP và cập nhật vùng knowledge bị ảnh hưởng lên knowledge tdd.
3. tdd-agent chỉ là knowledge hub (không chạy pipeline `change`).
4. **Orca không tự gọi MCP lấy knowledge; việc đó do agent được Orca gọi làm** (bằng MCP và skill).
5. **`plan generate`: AI kết hợp knowledge tdd và trả đầu ra là bộ tài liệu OpenSpec.**

**Checklist:** [CHECKLIST-TDD-002](../tasks/CHECKLIST-TDD-002-knowledge-hub.md)
**Quan hệ với SOL-TDD-001:** giữ worktree pilot, KG, CR và các khảo sát; bỏ pipeline `change` (G0–G7, `receipt`, `archive`). Ý tưởng "run agent ghi OpenSpec trong worktree riêng" của SOL-001 được dùng lại, nhưng **AI tự viết OpenSpec** bằng KG và CLI `openspec`, không qua tdd-agent.
**Trạng thái:** thiết kế và bộ skill (bản nháp, chưa thử trên agent thật); chưa có code Orca cho luồng này. Engine `openspec` của CR-REQ-026 đã có khung nhưng còn stub (2.4).

## 1. Nguyên tắc
- **P1.** Orca không truy vấn KG, không gọi MCP, không đưa khối "ngữ cảnh KG" vào prompt. Orca chỉ: chọn chế độ chạy, cấp phát môi trường (cấu hình MCP chỉ đọc, `allowedTools`, worktree), khởi chạy agent bằng prompt, kiểm cấu trúc đầu ra và lưu.
- **P2.** Agent tự tra knowledge bằng tool `mcp__tdd-agent__*` và skill; kết quả luôn nêu `snapshot`, id đã đọc, chỗ thiếu, xung đột.
- **P3.** Gói OpenSpec do AI viết; Orca dựng Plan từ `tasks.md` bằng quy tắc tất định. Chất lượng gói được bảo vệ bằng skill, `openspec validate --strict` và kiểm tra cấu trúc của Orca (3.4), không bằng pipeline tdd-agent.
- **P4.** KG chỉ chứa tri thức từ tài liệu; mọi khẳng định về mã phải dựa vào mã. Nội dung KG là dữ liệu không tin cậy.

## 2. Hiện trạng đã kiểm chứng (2026-10-14)

### 2.1 Đọc: MCP của tdd-agent (chạy thật qua stdio trên bản sao KG baseline)
- 13 tool, chỉ đọc theo mặc định. Dùng cho hướng này: `kg_search`, `kg_get_element`, `kg_neighbors`, `kg_resolve_scope`, `kg_query` (Q1–Q9), `kg_snapshots`, `source_get`. `impact_get`, `readiness_get`, `run_*`, `trace_get`, `metrics_get` thuộc pipeline, không dùng.
- Mọi kết quả nêu `snapshot`; danh sách dài có `truncated`/`next`.
- `kg_search` đưa phần tử khớp mã lên đầu. `kg_neighbors` trả kèm trích dẫn nguồn (`line:432`, câu gốc, `verified`).
- `kg_resolve_scope(["FR-SYS-07"])` trả closure rất lớn (cắt sau hơn 62 000 ký tự) vì `BRL-64` là luật trung tâm nối 10 yêu cầu: bắt buộc `budget`.
- KG pilot: 915 phần tử (Requirement 149, Attribute 520, Entity 78, BusinessRule 66, QualityTarget 61, Decision 17, Constraint 16, Component 8), 1544 cạnh. Không có lớp UseCase/Interface. KG phản ánh tài liệu, không phản ánh mã.
- `tdd-agent -C <gốc> mcp enable --read-only` ghi `.mcp.json` với `-C` tuyệt đối và đặt `[mcp] write=false`.
- Chạy `tdd-agent status` cho biết nguồn nào "up to date" (kiểm độ tươi mà không cần gọi MCP).

### 2.2 Ghi: ba kênh, chỉ một kênh nằm trong MCP
| Kênh | Cơ chế | Giới hạn |
|---|---|---|
| Tài liệu → KG | sửa SRS/TDD rồi `tdd-agent -C <repo> generate` (khoảng 20 s, không gọi mô hình với `--no-prose`) | SAD chỉ đọc (`SAD_READONLY`); phải chạy lại sau mỗi lần tài liệu đổi |
| ChangeSet | MCP `changeset_propose`: chạy thử trên head, lưu đề xuất, **không bao giờ commit**, từ chối thay đổi chạm SAD | Không có lệnh áp đề xuất đã lưu; commit chỉ bằng `tdd-agent kg import <graph.json>` (nhập cả đồ thị) |
| Pipeline | `run_start`, `run_resume`, `approval_submit` | đã loại |
- `apply_changeset` đánh dấu `needs_review` cho phần tử bị chạm và phần liên thông với chúng (đọc `transaction5.py`): với đồ thị có hub, một lần ghi có thể kéo cả vùng lớn về chờ duyệt. Cần đo (T1.6).
- "Vùng bị ảnh hưởng" gồm **sự kiện từ tài liệu** (đi qua tài liệu) và **sự kiện chỉ có trong KG** (mã nào thực hiện yêu cầu, test nào kiểm chứng; đi qua ChangeSet).

### 2.3 Skill
tdd-agent không xuất bản Claude Code skill nào (`tdd-agent skills list` là các bước nội bộ của pipeline). Bộ skill do ta viết, đã có trong `skills/`:
- `tdd-knowledge`: tra cứu tri thức (thứ tự tool, `budget`, trích id và snapshot, nêu chỗ thiếu và xung đột).
- `tdd-openspec-author`: viết `openspec/changes/<id>/` (proposal, design, spec delta, `tasks.md` theo hợp đồng) từ Request + Solution + KG + mã; dùng `openspec new/status/instructions/validate`.
- `tdd-knowledge-writeback`: xác định vùng ảnh hưởng; sự kiện từ tài liệu về tài liệu, sự kiện KG-only qua `changeset_propose`; không commit.
- `tdd-orca-task`: cách agent làm từng bước của Request và khối kết quả chuẩn.

### 2.4 Orca (đọc code)
| Điều | Bằng chứng |
|---|---|
| `solution generate` loại `solution` luôn `ai.complete`; ghi đè sang `agent_readonly` bị từ chối (`ErrSolutionModeNotAllowed`); các loại khác dùng agent chỉ đọc | `usecase/generate_solution.go` (`modeFor`) |
| Đường agent chỉ đọc đã có: cần repo path và dev server, `EnsureProjectGate`, giới hạn `MaxAgentRuns`, kiểm repo "clean" sau run | `generate_solution.go`, `run_agent_readonly_analysis.go` |
| Agent chỉ đọc chạy trong thư mục repo, công cụ chỉ `Read`/`Glob`/`Grep`, `--permission-mode plan` | `agent/src/relay/agent-readonly-tool-policy.ts` |
| `plan generate` hard-code `Mode: AnalysisModeComplete` (`ai.complete`, không công cụ, thư mục tạm rỗng) | `usecase/generate_plan.go`, `agent/src/relay/ai-complete-cli-session.ts` |
| `agent.execPrompt` không có cờ MCP; Orca đã có khái niệm khả năng của agent | `agent-print-mode-exec.ts`, `usecase/agent_capabilities.go` |
| Khung OpenSpec có sẵn nhưng chưa nối (profile theo loại, bảng `openspec_changes`, hai hàm prompt, `OpenSpecArchiveConsumer` stub) | `domain/openspec_*.go`, `usecase/openspec_*.go` |
| Skill của Orca cài toàn cục trên dev server bằng `npx skills add … --global` | `agent/src/shared/agent-feature-install-commands.ts` |
| **Khung engine OpenSpec của CR-REQ-026 đã có một phần**: domain (`engine_name`, `openspec_profile`, `engine_settings`, `openspec_change`), parser và renderer `tasks.md` (mã thật, văn phạm chặt có vùng `orca:begin/end`), cổng readiness, giao diện `SolutionEngine` + registry. **Còn là stub**: `openspecEngine.GenerateAnalysis/GeneratePlan/OnRequestCompleted` trả rỗng, `ProposalWorkspace` trả `nil` ở mọi hàm, `OpenSpecArchiveConsumer` rỗng, registry chưa được nối vào `GenerateSolution`/`GeneratePlan` | `usecase/engine_openspec.go`, `adapter/grpcclient/proposal_workspace.go`, `usecase/solution_engine.go`, `domain/tasks_md_parser.go`; thiết kế: `specs/backend-go/crs/v6/solution-engines/` (BE-REQ-SOL-026, TASK-REQ-026-01..08, trạng thái Proposed) |
| Cột `mode` của `analysis_runs` đã cho phép `agent_proposal` (CHECK/ENUM trong migration 0006) nhưng Go chỉ có `complete` và `agent_readonly` | `migrations/*/0006_analysis_runs.up.sql`, `domain/analysis_run.go` |
| `DevServerExecutor` của request-service là stub trả cứng `"openspec version 1.2.3"`, exit 0 (nên cổng readiness luôn đạt); `PinSolutionEngine` là bản giả trả `native`; `ManageProjectEngineSettings.Set…` không kiểm quyền và luôn trả `preflight_not_run`; `usecase/agent_capabilities.go` là stub không ai gọi | `adapter/grpcclient/dev_server_executor.go`, `usecase/{pin_solution_engine,manage_project_engine_settings,agent_capabilities}.go` |
| Lease của run được gia hạn mỗi 30 s (`Heartbeat`), `LeaseTTL` 90 s; `AgentTimeoutMS` mặc định 600 000; agent giới hạn `timeoutMs` tối đa 15 phút (mặc định 5) | `usecase/{analysis_runner,analysis_settings}.go`, `agent/src/relay/agent-exec-prompt-timeouts.ts` |
| task-service đã có hợp đồng thực thi (CR-REQ-029): `TaskSpecV2`, result block có nonce `ORCA_RESULT_BEGIN/END`, `ExecuteWithContract`, `checks_run[{id,exit}]`; mọi lần thực thi task dùng `TrustPreset: "full"` | `task-service/internal/usecase/contract_executor.go`, `adapter/grpcclient/simple_executor*.go`, migration 0020 |
| Parser/renderer `tasks.md`, `CheckProposalScope`, cổng readiness: mã thật nhưng chưa có test | `domain/tasks_md_*.go`, `usecase/proposal_scope_check.go`, `usecase/engine_readiness_gate.go` |
Hệ quả: vì agent chỉ đọc chạy ngay trong thư mục repo và Orca kiểm repo "clean", **không được ghi skill hay `.mcp.json` vào repo** ở chế độ này. Skill cài toàn cục; cấu hình MCP truyền bằng cờ trỏ tới file nằm ngoài repo.

## 3. Thiết kế

### 3.1 Luồng
| Bước | Chạy thế nào | Knowledge |
|---|---|---|
| classify, clarification | Không đổi | Không dùng |
| `solution generate` | **Agent chỉ đọc có MCP** (chế độ mới cho loại `solution`, theo cờ project) | Agent tự tra bằng `tdd-knowledge`; trả Solution JSON kèm `design_references` |
| duyệt Solution | Không đổi | Gate hiển thị tham chiếu và xung đột |
| `plan generate` | **Run `spec`: agent ghi trong worktree riêng** | Agent kết hợp KG + mã, viết gói OpenSpec; Orca dựng Plan từ `tasks.md` |
| duyệt Plan và Phase | Không đổi (người duyệt đọc gói OpenSpec và Plan) | Gate hiển thị gói và kết quả kiểm cấu trúc |
| Thực thi task | Agent có MCP + skill (worktree task) | Agent tra id của task; trả khối kết quả chuẩn |
| Cuối task | Cùng agent | `tdd-knowledge-writeback` |

### 3.2 Môi trường mà Orca cấp (không truy vấn KG)
- **Cấu hình MCP (do agent dựng, không do Orca):** Orca chỉ gửi `knowledgeRoot` (đường dẫn tuyệt đối của repo có `.tdd-agent/`) và `proposeChangesets` trong `agent.execPrompt`; agent kiểm đường dẫn (tuyệt đối, `realpath` hợp lệ, có `.tdd-agent/config.toml`, không thoát qua symlink), ghi một file cấu hình tạm trong thư mục làm việc của agent (mode 0600, xoá sau run) với đúng một server `tdd-agent` chạy `tdd-agent -C <realpath> mcp serve`, rồi truyền cho `claude` bằng `--mcp-config`. Không có chuỗi lệnh hay đường dẫn tệp nào của Orca/Request đi vào tiến trình. Máy chủ chỉ đọc theo `[mcp]` trong config của repo.
- **`allowedTools` (cố định, do agent đặt):** `mcp__tdd-agent__kg_get_element`, `kg_neighbors`, `kg_resolve_scope`, `kg_query`, `kg_search`, `kg_snapshots`, `source_get`; thêm `changeset_propose` chỉ khi `proposeChangesets`; run `spec` thêm `Bash(openspec:*)`, `Bash(tdd-agent -C * status)` và ghi giới hạn trong vùng được phép. Không bao giờ có `run_start`, `run_resume`, `approval_submit`.
- **Skill:** cài toàn cục trên dev server (`npx skills add … --skill tdd-knowledge tdd-openspec-author tdd-knowledge-writeback tdd-orca-task --global`). Agent báo skill đã cài và có MCP trong khả năng (`agent_capabilities`); thiếu thì Orca không bật chế độ mới cho project đó và báo lý do.
- **KG:** ở `<gốc repo đã đăng ký>/.tdd-agent/`, dùng chung chỉ đọc cho mọi run (run `spec` không cần bản sao vì không ghi KG). Dùng `-C` tuyệt đối vì worktree nằm lồng trong repo.

### 3.3 `solution generate` bằng agent chỉ đọc
- `modeFor`: cho phép `agent_readonly` với loại `solution` khi project bật cờ; mặc định vẫn `Complete`.
- Prompt Solution cho chế độ này: yêu cầu dùng skill `tdd-knowledge`, nêu phần tử bị ảnh hưởng và xung đột Decision/Constraint trong từng phương án, chỗ thiếu thành câu hỏi mở; cho phép thêm trường tuỳ chọn `design_references` (`snapshot`, `read`, `gaps`, `conflicts`) vào schema Solution (kiểm schema và lưu).
- `agent.execPrompt` readonly: thêm `knowledgeRoot` (cờ `--tools` hiện giới hạn `Read,Glob,Grep`; cách cho phép tool MCP trong chế độ chỉ đọc xác định ở T1.4). Kiểm repo "clean" vẫn áp dụng.

### 3.4 `plan generate` trả bộ OpenSpec (engine `openspec` của CR-REQ-026)
Hiện thực hoá khung đã có của CR-REQ-026 thay vì tạo chế độ mới; phần thêm của SOL-002 là **agent dùng MCP và skill** trong run.
1. **Chọn engine:** nối `EngineRegistry` vào `GeneratePlan`; engine theo `ProjectEngineSettings` của project (mặc định `native` = `ai.complete` như cũ). Với `openspec`, run dùng chế độ `agent_proposal` (cần thêm hằng `AnalysisModeAgentProposal`; migration không cần vì CHECK/ENUM đã có). Thiếu khả năng của agent (MCP, skill) hoặc `Preflight` lỗi ⇒ không bật, hiển thị lý do.
2. **Worktree riêng** qua `ProposalWorkspace` (git-gateway: `CreateWorktree`, `ReadFile`, `WriteFile`, `GetStatus`, `Commit`), nhánh `request/<số>-proposal`, không dùng `CreateWorktreeFromIssue`. Hiện là stub: phải hiện thực thật.
3. **Prompt** (`openspec_plan_prompt` mở rộng): change id, `request=<ref>`, Request kèm id tiêu chí chấp nhận, Solution đã duyệt, giới hạn Plan (số Phase/task theo cỡ), yêu cầu dùng `tdd-openspec-author` + `tdd-knowledge`, vùng được ghi, khối kết thúc chuẩn. Chạy qua `agent.execPrompt` với `mcpConfig` và `allowedTools` (3.2).
4. **Hợp đồng đầu ra:** `proposal.md`, `design.md`, `specs/<capability>/spec.md`, `tasks.md` có vùng `<!-- orca:begin plan request=<ref> schema=1 -->` … `<!-- orca:end plan -->` theo văn phạm chặt của parser (PH-n, `T<p>.<q>`, thuộc tính `type|h|ac|labels|depends|irreversible`); bốn dòng Serves/Code/Verify/Done when là phần mô tả thụt hai dấu cách của task.
5. **Kiểm sau run (Orca, không truy vấn KG):**
   - `ProposalWorkspace.ChangedPaths` chỉ nằm dưới `openspec/changes/<id>/` (`proposal_scope_check`), ngoài ra ⇒ `REQUEST_SPEC_RUN_OUT_OF_SCOPE`.
   - `ParsePlanRegion` không có `Violation`; `ValidateProposal` đạt (`PlanLimits`).
   - `openspec validate <id> --strict --json` đạt (chạy qua `agent.exec`).
   - Số task trong khoảng theo cỡ (mặc định S 1–5, M 3–12, L 6–30; cấu hình); mọi task đủ bốn dòng mô tả; đường dẫn `Code:` hợp lệ (tệp có thật hoặc thư mục cha có thật); mỗi tiêu chí chấp nhận có scenario cùng tên trong spec; `ac=` chỉ dùng id có trong Request.
   - Không đạt: một lần thử lại với danh sách lỗi; lần hai vẫn lỗi ⇒ run `failed` kèm lý do (hiện ở `plan proposal`), worktree giữ lại.
6. **Dựng Plan (tất định):** `PlanProposal` từ parser đi qua `commit_plan` như hiện nay; `PlanRaw` giữ `tasks.md` thô. Commit gói lên nhánh `request/<số>-proposal` (cục bộ, chặn push); ghi `openspec_changes` (id, nhánh, commit, snapshot KG và id đã đọc do agent báo, hash phiên bản skill).
7. **Đồng bộ và archive (CR-026):** `TasksMdSyncer` đánh dấu `[x]` khi task xong (không bao giờ bỏ tick); khi Request `completed`, consumer chạy `openspec archive <id> --yes` trong worktree của proposal và commit lên nhánh proposal (không merge, không mở PR). Hiện là stub.
8. **Thời gian:** lease đã được gia hạn mỗi 30 s (`Heartbeat`) nên run dài không bị thu hồi; đặt `SpecTimeoutMS` riêng (mặc định 900 000, bằng trần 15 phút của agent); kiểm deadline của relay/gRPC; `PLAN_RUN_TIMEOUT_MS` của UI và `--timeout` của CLI phụ thuộc engine (T2.8).

### 3.5 Thực thi, nghiệm thu, ghi ngược (do agent)
- **Thực thi:** agent task nhận `knowledgeRoot` (repo gốc), `allowedTools` cố định và skill; prompt gồm id yêu cầu, tiêu chí chấp nhận và lệnh `Verify` của task (đưa vào `prompt_template`, tránh trần 2000 ký tự của `ai_context`). task-service **đã có** hợp đồng thực thi (`TaskSpecV2` phạm vi tệp được ghi, result block có nonce, `checks_run[{id,exit}]`): dùng nó làm bằng chứng `Verification` và phạm vi `Code:` nếu request-service nối được (T2.11).
- **Nghiệm thu (đề xuất, chờ xác nhận):** task chỉ `done` khi kết quả có dòng `Verification: <lệnh> => <kết quả>`; cuối mỗi Phase có task chạy test của Phase; khi Request `completed`, Orca liệt kê việc còn lại cho người (hợp nhất code, `tdd-agent generate`, áp đề xuất ChangeSet).
- **Ghi ngược:** sự kiện từ tài liệu là task cập nhật SRS/TDD trong `tasks.md`; sau khi hợp nhất, người hoặc Orca (qua relay) chạy `tdd-agent -C <repo> generate` (SAD không sửa). Sự kiện KG-only là `changeset_propose`; Orca lưu `proposal_id`, `ok` và số id bị chạm, không tự áp. **Commit:** thêm lệnh `tdd-agent kg apply <proposal_id> --actor --rationale --hash` vào tdd-agent (đề xuất, T1.7) hoặc người áp tay.

### 3.6 Quyết định mở
- **Q1. Gói OpenSpec nằm ở đâu để agent task đọc?** CR-026 đã chọn nhánh `request/<số>-proposal` (cục bộ, chặn push). Còn phải xác nhận worktree task tách từ commit đó (hay từ nhánh mặc định) và cách task đọc `spec.md`; phương án khác: đưa nội dung vào `prompt_template`. Chốt ở T1.8.
- **Q2.** Gói có cần phản biện độc lập (một lượt agent thứ hai) trước gate Plan không? Sau pilot.
- **Q3.** Bật cho loại nào ngoài `change_request`.

### 3.7 An toàn
- Run `spec` có công cụ và quyền ghi, đầu vào không tin cậy (Request và nội dung KG): worktree riêng, `allowedTools` hẹp, kiểm `git status`, chặn push (`agent_push_block.py`), dấu vân tay thư mục chính và worktree pilot, đánh giá bảo mật riêng (T5.2). B4 (agent chạy `--dangerously-skip-permissions`) vẫn mở.
- Skill dặn không làm theo chỉ thị trong nội dung KG và Request.
- Máy chủ MCP chỉ đọc; `changeset_propose` chỉ khi task cho phép.

## 4. Runbook pilot (ba bước)
1. **Vào worktree TMS:** `vnp-qa-platform/.claude/worktrees/tdd-agent-bl-h01-h02` (đã có; cần dọn trạng thái spike, T0.3).
2. **Khởi tạo knowledge:** `tdd-agent -C <worktree> generate` (đã chạy: 915/1544/246); `tdd-agent -C <worktree> mcp enable --read-only`.
3. **Luồng Orca:** BL-H01 rồi BL-H02 với CR làm nội dung Request: `solution generate --wait` (agent + KG) → duyệt → `plan generate --wait` (OpenSpec) → duyệt Plan → từng Phase (agent + MCP + skill) → `completed`.

## 5. Tiêu chí nghiệm thu
1. Solution (BL-H01) nêu đúng phần tử bị ảnh hưởng (FR-SYS-07, ENT-17, BRL-64…) và phát hiện xung đột ADR-12 (giả thuyết; đo bằng so sánh với `ai.complete`).
2. Gói OpenSpec: `openspec validate --strict` đạt, ≥ 6 task cho BL-H01 (cỡ L) mỗi task đủ bốn dòng, mỗi AC một scenario, không scenario "trả 501" còn lại.
3. Plan dựng từ `tasks.md` hợp lệ; người duyệt gate đọc gói.
4. Agent thực thi gọi tool KG thật (có trong log), trả khối kết quả chuẩn kèm dòng `Verification`, không sửa ngoài phạm vi.
5. Ghi ngược: danh sách vùng ảnh hưởng đúng; sự kiện từ tài liệu nằm trong diff; ít nhất một đề xuất ChangeSet `ok: true`; số phần tử bị chạm được báo.
6. An toàn: không push, không sửa SAD hay `.tdd-agent/`, thư mục chính của vnp-qa-platform không đổi, 0 Request mồ côi.
7. Ghi số liệu: số lần gọi tool, thời gian từng bước, chi phí, số lần phải can thiệp ngoài gate.

## 6. Rủi ro
| # | Rủi ro | Giảm thiểu |
|---|---|---|
| H1 | Agent chỉ đọc không gọi được tool MCP (cờ `--tools` giới hạn) | Kiểm sớm (T1.4); nếu không được, dùng chế độ ghi hẹp thay vì chỉ đọc |
| H2 | Agent bỏ qua skill hoặc viết `tasks.md` chung chung (giống F7 của spike) | Kiểm cấu trúc của Orca + thử lại; đưa quy tắc vào prompt; đo ở T1.5 |
| H3 | `kg_search` bỏ sót hoặc nhiễu (KG nghèo `--no-prose`; hub `BRL-64`) | Skill dặn `classes`, `budget`; đo T1.1 |
| H4 | Ghi ngược đẩy hàng loạt phần tử về `needs_review` | Đo T1.6; gom thay đổi; người duyệt quyết định áp |
| H5 | Chưa có lệnh áp đề xuất ChangeSet | T1.7 |
| H6 | Văn bản KG/Request chứa chỉ thị | Rào chắn dữ liệu; skill dặn; T5.2 |
| H7 | Skill chưa được cài trên dev server, hoặc cài lệch phiên bản | Kiểm khả năng agent trước khi bật; ghi hash phiên bản vào run |
| H8 | KG cũ so với tài liệu | Agent báo `status`; cảnh báo ở kết quả |
| H9 | Công việc CLI/gateway chưa commit | T0.5 |
| H10 | Run engine `openspec` dài hơn trần 15 phút của `agent.execPrompt` | Ngân sách `SpecTimeoutMS`, chia run hoặc nâng trần có chủ ý (T2.8); lease không còn là vấn đề (đã có heartbeat) |

## 7. Tái sử dụng từ SOL-TDD-001
Dùng lại từ SOL-001 và CR-REQ-026: khung engine `openspec` (domain, parser, renderer, readiness, registry; hiện thực nốt các stub), `ProjectEngineSettings` làm cờ theo project, nhánh `request/<số>-proposal`, `TasksMdSyncer` và archive; cờ MCP của `agent.execPrompt`, quy ước `-C`, chặn push, dấu vân tay an toàn, runbook e2e, phần chọn chế độ và kiểm sau run của run `spec`, quy tắc `tasks.md` → Plan. Bỏ: tdd-agent `change`, G0–G7, `receipt`, `archive`, CR→SRS delta.
