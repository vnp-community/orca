# SOL-TDD-001: tích hợp tdd-agent vào luồng Request của Orca; pilot e2e trên vnp-qa-platform (BL-H01, BL-H02)

**Yêu cầu của người dùng (2026-10-14):**
1. Áp dụng tdd-agent (sinh TDD, quản lý ontology/KG, MCP, sinh OpenSpec cho agent code) vào luồng xử lý task của Orca; giữ nguyên luồng Request, được sửa code Orca và skill của agent.
2. Bước `plan generate` là bước sinh OpenSpec, dùng ngữ cảnh KG và skill.
3. Bước `solution generate` (giai đoạn Làm rõ và phân tích) cũng phải nhận ngữ cảnh lấy từ knowledge của tdd-agent để đề xuất.
4. Chạy luồng mới trên CR thật, end-to-end: (a) vào worktree của TMS; (b) chạy pipeline tdd-agent để khởi tạo knowledge và sinh TDD; (c) chạy luồng Orca để thực thi CR. Use case: BL-H01 và BL-H02 của `vnp-qa-platform`.

**Checklist triển khai:** [CHECKLIST-TDD-001-pilot-vnp-qa-platform.md](../tasks/CHECKLIST-TDD-001-pilot-vnp-qa-platform.md)
**Trạng thái (2026-10-14): hướng pipeline TẠM DỪNG sau spike T1.9** (mục 2.2b); người dùng đổi sang hướng knowledge hub: xem [SOL-TDD-002](SOL-TDD-002-tdd-agent-as-knowledge-hub.md). Tài liệu này giữ làm tham chiếu và làm cơ sở nếu tái xét pipeline khi phân rã ổn định. Thiết kế, chưa triển khai code Orca. Mọi chỗ ghi "chưa kiểm chứng" phải được kiểm ở Giai đoạn 1 của checklist trước khi sửa Orca.

## 1. Mục tiêu và không-mục tiêu

Mục tiêu:
1. `solution generate` nhận ngữ cảnh thiết kế từ KG của tdd-agent (phần tử SRS/SAD/TDD liên quan tới Request), đưa vào prompt như khối dữ liệu có rào chắn.
2. Với `change_request` (sau này cả `refactor`), `plan generate` sinh gói OpenSpec thật (`openspec/changes/<id>/`) bằng pipeline tdd-agent, rồi dựng Plan của Orca từ `tasks.md`.
3. Agent thực thi nhận spec, scenario, ngữ cảnh KG, và báo kết quả test thật về tdd-agent.
4. Luồng người dùng và các gate duyệt của Orca (Solution, Plan, Phase) giữ nguyên hình dạng.
5. Có runbook e2e ba bước chạy được trên CR thật (mục 4) và tiêu chí nghiệm thu đo được (mục 7).

Không-mục tiêu: luồng cấu hình được theo tenant (luồng mỗi loại vẫn nằm trong `flowDefinitions`); thay mô hình an toàn của agent (B4 vẫn mở); UI mới.

## 2. Hiện trạng (đã đọc code hoặc chạy lệnh, ngày 2026-10-14)

### 2.1 Orca
| Điều | Bằng chứng |
|---|---|
| `solution generate` (chế độ Complete) và `plan generate` đều chạy `ai.complete`; Plan hard-code `Mode: AnalysisModeComplete` | `request-service/internal/usecase/generate_plan.go`, `generate_solution.go`, `server_solution.go` |
| `ai.complete` qua Claude CLI chạy `--tools ''` trong thư mục tạm rỗng: không đọc repo, không MCP, không skill | `agent/src/relay/ai-complete-cli-session.ts` |
| Chế độ `agent_readonly` chỉ mở `Read`, `Glob`, `Grep` | `agent/src/relay/agent-readonly-tool-policy.ts` |
| `agent.execPrompt` (agent thực thi) không có cờ MCP | `agent/src/relay/agent-print-mode-exec.ts` |
| Prompt Solution gồm các khối dữ liệu có rào chắn (`<request>`, `<project>`, `<prior_artifacts>`, `<clarifications>`, `<feedback>`); `fenceUntrusted` vô hiệu thẻ đóng; giới hạn body 12000 ký tự, artifact 8000 | `usecase/solution_prompt.go` |
| Prompt Plan sinh JSON theo schema, có giới hạn Phase/task, nhãn theo loại, thử lại khi sai | `usecase/plan_prompt.go`, `plan_validation.go` |
| Khung OpenSpec có sẵn nhưng chưa nối: `OpenSpecProfileFor`, bảng/repository `openspec_changes`, `BuildOpenSpecPlanPrompt`, `BuildOpenSpecSolutionPrompt` (không có chỗ gọi ngoài test), `OpenSpecArchiveConsumer` là stub rỗng | `domain/openspec_*.go`, `usecase/openspec_*.go` |
| Readiness của engine đã đọc phiên bản `openspec` | `usecase/engine_readiness_gate.go` |
| Luồng mỗi loại Request là bảng cứng trong code | `domain/request_flow_registry.go` |
| Prompt thực thi task = `prompt_template` (nếu có) + Task/Description/Context + task cha + task phụ thuộc đã xong; phần đầu "Orca Project Context" | `task-service/internal/adapter/grpcclient/simple_executor.go` (`buildExecutePrompt`) |
| `ai_context` mỗi task chỉ là "REQ-n: tiêu đề" + summary Plan, tối đa 2000 ký tự | `request-service/internal/adapter/grpcclient/task_plan_writer.go` |
| Tiêu chí chấp nhận của Request chỉ vào prompt Solution và Plan, không vào prompt thực thi | `solution_prompt.go` (`writeRequestBlock`), `simple_executor.go` |
| Lease run phân tích mặc định 90 s; UI chờ Plan tối đa 240 s | `usecase/analysis_settings.go`; `frontend/.../request-plan-generation.ts` |
| Worktree task do Orca tạo cạnh thư mục repo đã đăng ký (`<repo>-task-<id>`) | quan sát khi chạy CLI trên stack local |
| CLI `orca` có lệnh luồng Request (login, request, solution, approval, plan, clarification) và 18 route REST mới ở gateway: **đã chạy thật nhưng chưa commit** | `backend-go/cmd/orca-cli`, `api-gateway/.../request_routes_flow.go` |

### 2.2 tdd-agent (README, `docs/CLI_Project_Usage.md`, chạy lệnh trên dự án thật)
- Đầu vào: CR Markdown (`- AC-<id>: GIVEN … WHEN … THEN …`) + SRS/SAD/TDD trong repo. Luồng cổng G0–G7. Đầu ra: `openspec/changes/<tên>/{proposal,design,tasks}.md`, `specs/…/spec.md`, `trace-manifest.json`; TDD mới ở `docs/tdd/`.
- Khởi tạo: `tdd-agent init --urd … --srs … --sad … [--name --tdd-id --docs-dir --no-prose --no-generate --force --approver id:kind:roles]`. Chạy mất khoảng 2 phút với tài liệu cỡ vừa nếu có gọi mô hình. Làm mới: `generate`. Kiểm tra: `doctor`, `status`, `kg stats|search|conflicts`, `review --list|--all-pending|--ids`.
- Trạng thái nằm trong `.tdd-agent/` của dự án (SQLite); chạy lại `change <cr>` thì **tiếp tục** change đó. Có `--json`.
- Xuất bản gói OpenSpec vào repo chỉ dẫn tạm `.tdd-runs/<change>-<hash>/{authoring,instruction-repo,validation-repo}`; cách đưa gói vào nhánh làm việc là điểm cần xác nhận (T1.12).
- Truy vấn KG: CLI chỉ có `kg search <query> [--class --snapshot --limit]` (tìm toàn văn, kể cả theo mã một phần), `kg stats`, `kg conflicts`. Tool MCP đọc: `kg_get_element`, `kg_neighbors`, `kg_resolve_scope`, `kg_query`, `kg_search`, `kg_snapshots`, `source_get`, `impact_get`, `readiness_get`, `run_list`, `run_get`, `trace_get`, `metrics_get`. Riêng `kg_neighbors`, `kg_resolve_scope`, `impact_get` chỉ có qua MCP, không có CLI tương đương.
- `tdd-agent skills` (list, test, demotion-report) là khái niệm nội bộ của pipeline tdd-agent, không phải Claude Code skill (`SKILL.md`); skill của Orca phải viết mới.
- MCP chỉ đọc là chính sách trong config (`tdd-agent mcp enable --read-only` ghi `.mcp.json` và khối `[mcp]`); `mcp serve` chỉ có `--transport`.
- Duyệt cổng: `tdd-agent change approve --actor <id> --rationale <text> --hash <hash quyết định>` (hash lấy từ `change decision`); vai trò lấy từ `[[approvals.actors]]`.
- **Hai điểm dễ sai đã gặp khi chạy thật (2026-10-14):** (1) tdd-agent tìm gốc dự án bằng cách đi ngược lên thư mục cha; worktree nằm trong `.claude/worktrees/` của checkout chính nên chạy `tdd-agent` không có `-C` sẽ dùng `.tdd-agent/` của **checkout chính** (đọc hay ghi đều vào đó). Mọi lệnh phải có `-C <thư mục dự án>` (Orca cũng vậy: worktree task `<repo>-task-<id>` nằm cạnh worktree pilot, trong cùng checkout chính). (2) `tdd-agent -C <dir> init --urd docs/URD.md …` ghi **đường dẫn tuyệt đối** về `docs/` của checkout chính vào `[project.sources]`; phải sửa thành `docs/URD.md`, `docs/SRS.md`, `docs/SAD.md` (tương đối) trước khi `generate`, nếu không KG đọc tài liệu của thư mục khác.
- Mẫu `tasks.md` thật: `CS-001: modify backend/.../labels.go::KeyHandleUser; verified by TC-CS-001: GIVEN … WHEN … THEN …`: đủ cụ thể để làm task cho agent.
- Chi phí một change nhỏ (bug): 5 lượt gọi mô hình, 18 136 token, 126 s, báo $4.07 (`tdd-agent metrics`). Feature cỡ L sẽ lớn hơn nhiều; đây chỉ là mốc sàn.

### 2.2b Kết quả chạy thật `tdd-agent change` trên CR-H01 (T1.9, 2026-10-14)
Bốn lần chạy trong worktree pilot (KG baseline 915/1544/246), cuối cùng dừng ở G5 chờ duyệt, **chưa duyệt vì gói chưa đạt**. Thời gian mô hình: G0–G2 khoảng 15–25 s; G3–G5 khoảng 3–5 phút mỗi vòng; lần chạy cuối 12 lượt gọi mô hình, báo $0.46 (chưa tính vòng viết gói).
| # | Quan sát | Hệ quả cho thiết kế |
|---|---|---|
| F1 | **Phân rã CR → yêu cầu SRS không ổn định**: cùng một CR cho 1, 3, 4 hoặc 5 thay đổi SRS tuỳ lần chạy; mã yêu cầu mới (FR-SYS-10/11/12/13) đổi nghĩa giữa các lần | Mã yêu cầu không được dùng làm khoá cố định trước G0; Plan hạt mịn không được trông cậy vào phân rã tự do của mô hình (xem F7) |
| F2 | G2 báo `SAD_CHANGE_REQUIRED`: ADR-12 xung đột FR-SYS-07 vì cột kiểm chứng cũ "[T] Chat/transform trả 501" bị giữ lại (G0 chỉ thay câu trong ô yêu cầu, không sửa cột kiểm chứng). ADR-12 tự ghi ngoại lệ 501 cho AI là tạm thời (OI-02). Sửa chữ trong CR không gỡ được | Cần `change waive` (SA) kèm lý do, lặp lại sau mỗi lần G0 chạy lại; skill phải biết việc này |
| F3 | `change update` có `--satisfied-by/--new-in/--verify` làm G0 đọc lại CR (mã yêu cầu đổi, waiver mất, G1/G2 mở lại); chỉ `--code-target` thì đi tiếp đúng. Lặp nhiều `update` ở chế độ manual để lại cạnh tới phiên bản cũ (QG-01 "edge endpoint does not exist", TS-07): change không cứu được, phải khôi phục baseline | Không lặp `update`; chế độ `auto` (tự duyệt G1/G2 và tự đề xuất phần còn thiếu) đi được xa hơn; run `spec` phải có bước "đặt lại về baseline" |
| F4 | G4 yêu cầu `--code-target` cho code mới; đường dẫn chưa tồn tại được chấp nhận. `CS` ghi "modify" cả cho file mới | Skill dặn "tạo mới nếu chưa có" |
| F5 | QG-03: 18–36 phần tử cần duyệt, gồm yêu cầu SRS có sẵn kéo vào qua BRL-64 (khớp nguyên văn SRS, đã đối chiếu bằng chương trình) và phần tử do change sinh ra | Duyệt review có chọn lọc, kèm rationale; không `--all-pending` |
| F6 | Chế độ `auto` vẫn chặn ở QG-03; phải `review --ids … --decision approved --actor claude --role SA` | Người dùng đã cho `claude` quyền này (2026-10-14) |
| F7 | **Gói qua được các luật nhưng chất lượng thấp** khi CR bị gộp vào một yêu cầu: `tasks.md` chỉ có một task "CS-001 ms-system-setting… chỉ bắt đầu sau khi giải quyết quyết định mở DEC-code-target" (không nêu file/hàm), gom bảy hành vi vào một việc; `spec.md` vẫn giữ kịch bản "chat/transform trả 501" cũ, mâu thuẫn với yêu cầu mới. Bộ phản biện độc lập đã tự nêu đúng các điểm này nhưng sau tối đa 2 vòng viết lại vẫn đi qua | Cổng G5 của tdd-agent không đủ để bảo đảm gói dùng được cho agent code. Orca phải có kiểm tra riêng trước khi dựng Plan (số task tối thiểu, mỗi task có đích mã và kịch bản, không còn kịch bản/thuật ngữ bị CR gỡ) |

**Kết luận của spike (chưa phải go/no-go):** pipeline chạy tới bước viết gói OpenSpec và phản biện, nhưng đầu ra phụ thuộc mạnh vào cách CR được cấu trúc. Muốn Plan hạt mịn, CR phải mô tả sẵn nhiều yêu cầu có tên và đích mã (mỗi nhóm "Phạm vi công việc" của backlog một yêu cầu), nêu rõ kiểm chứng cũ bị gỡ, và đưa mọi hint trong lần gọi đầu; hoặc Orca tự dựng bảng change spec từ backlog rồi chỉ dùng tdd-agent cho SRS delta, tác động TDD và truy vết.

### 2.3 vnp-qa-platform
- Worktree pilot đã tạo: `vnp-qa-platform/.claude/worktrees/tdd-agent-bl-h01-h02`, nhánh `feat/tdd-agent-bl-h01-h02`, tách từ `claude/tms-bug-triage-report-09f310` (commit `2932acb3`); nhánh gốc đang được checkout ở thư mục chính nên không dùng chung được.
- `docs/URD.md`, `SRS.md`, `SAD.md`, `docs/tdd/TDD-TMS.md` đã được theo dõi trong git. `.tdd-agent/` và `.tdd-runs/` chưa (untracked, chỉ ở thư mục chính), nên worktree mới **chưa có KG**: bước khởi tạo (mục 4.2) là việc thật phải làm, không phải sao chép.
- KG mới của worktree pilot (dựng 2026-10-14, 20 s, không gọi mô hình): 915 phần tử, 1544 quan hệ, 246 gap, 0 xung đột, `readiness: blocked`, TDD revision 1 (26 chỗ THIẾU, 250 câu hỏi cho chủ sở hữu); lớp phần tử: Requirement 149, Attribute 520, Entity 78, BusinessRule 66, QualityTarget 61, Decision 17, Constraint 16, Component 8; 915 phần tử đều `needs_review`. **Không có lớp UseCase hay Interface**: mã `UC-…` và `EIF-…` của SRS không phải phần tử KG.
- KG cũ ở thư mục chính (để đối chiếu, không dùng làm nền): 935 phần tử, 585 quan hệ, 272 gap, 0 xung đột, `readiness: blocked`, 912 `needs_review`. Cấu hình cũ dùng `prose = false` (ghi chú: FACT do mô hình suy ra gây xung đột `entity_match` chặn G5/QG-04); từng gặp `SAD_READONLY` khi `generate` lại sau khi SAD đổi, nên đổi sang store mới.
- 7 lần `change` trước (bug Jira auto-create); change mới nhất dừng ở `awaiting_evidence (spec_authored)`: pipeline chạy được tới bước sinh spec trên dự án này.
- Luật dự án cho agent: thư mục bảo vệ `services/infrastructure/integration-hub/`; luật GitNexus (`impact` trước khi sửa symbol, `detect_changes` trước commit).
- Repo có remote GitHub thật; agent `dev-local-mac` chạy `--dangerously-skip-permissions` (B4).

### 2.4 Hai use case
| Backlog | Loại/cỡ Orca | Nội dung | Phụ thuộc |
|---|---|---|---|
| BL-H01 nền AI (`shared/pkg/aiclient`, `AIService`, migration `ai_conversation_message`, route `/ai/conversation/*`, cờ `AI_ENABLED`, cấu hình mô hình, hạn mức) | `change_request`, L | thêm mới, **sửa FR-SYS-07** (SRS nói chat/transform "trả 501 trung thực"; OI-02, EIF-12, UC-16, UC-38, ENT-17) | `depends_on: BL-J01` (status **Open**) |
| BL-H02 AI sinh test case (`shared/pkg/aiprompt`, `Chat`/`Transform`/`BatchSave` ≤ 20 khối, ≤ 100 case/giao dịch, `case_ai_config`) | `change_request`, M | thêm mới trên case-management | `depends_on: BL-H01` |

## 3. Thiết kế

### 3.1 Luồng e2e
Các lệnh `orca` không đổi (xem README của orca-cli). Thay đổi:

| # | Giai đoạn | Thay đổi |
|---|---|---|
| 0 | Trước mọi Request (một lần mỗi repo) | Khởi tạo KG và TDD bằng `tdd-agent init` trong repo (mục 4.2). Đây là điều kiện của các bước sau, làm ngoài Orca. |
| A | `solution generate` | Orca truy vấn KG theo Request, đóng gói khối `<design_context>` có rào chắn vào prompt Solution (mục 3.2). Vẫn là `ai.complete`. |
| B | Trước `plan generate` | Orca dựng file CR từ Request (tiêu đề, body, câu trả lời clarification, tiêu chí chấp nhận). |
| C | `plan generate` | Run kiểu mới `spec`: agent có công cụ trong worktree riêng, chạy `tdd-agent change` theo skill; trả gói OpenSpec và Plan JSON dựng từ `tasks.md` (mục 3.3, 3.4). |
| D | Sau run | Kiểm đường dẫn thay đổi, kiểm Plan JSON bằng đường kiểm hiện có, ghi `openspec_changes`, `commit_plan` như cũ. |
| E | Thực thi và kết thúc | Agent thực thi có skill đọc spec và KG; kết quả test báo qua `tdd-agent receipt`; khi Request `completed` thì `OpenSpecArchiveConsumer` chạy `openspec archive` (mục 3.6). |

### 3.2 Ngữ cảnh KG cho `solution generate` (giai đoạn A)
Giữ `ai.complete` không công cụ (lý do an toàn: nội dung Request là dữ liệu không tin cậy). AI không tự gọi MCP; Orca lấy ngữ cảnh bằng truy vấn tất định rồi đưa vào prompt.

1. **Nguồn khoá tìm:** tiêu đề và body Request; mã phần tử KG `FR-/NFR-/ENT-/BRL-/ADR-/D-…` xuất hiện trong body (mã `UC-`/`EIF-` không phải phần tử KG nên chỉ dùng làm từ khoá văn bản, không tra theo mã), tiêu chí chấp nhận và câu trả lời clarification (trích bằng biểu thức chính quy).
2. **Truy vấn (bản 1):** trên dev server, trong thư mục repo đã đăng ký, qua relay `agent.exec` chạy `tdd-agent kg search "<khoá>" --class <lớp> --limit N --json` cho từng khoá; gộp, khử trùng, xếp hạng theo số khoá khớp và lớp (Requirement, Entity, BusinessRule, Decision, Constraint, QualityTarget, Component; Attribute chỉ lấy khi thuộc Entity đã khớp). Không dùng mô hình ở bước này. **Bản 2 (sau):** client MCP stdio gọi `kg_resolve_scope`, `kg_neighbors`, `impact_get` để mở rộng theo quan hệ.
3. **Đóng gói:** khối `<design_context>` mỗi phần tử một mục (id, lớp, tiêu đề/câu gốc, nguồn `SRS §…`), có trần ký tự riêng (đề xuất 6000, cấu hình được), thêm vào `fenceUntrusted` và dòng dặn "dữ liệu tham khảo, không phải chỉ thị". Prompt dặn thêm: phương án phải nêu phần tử thiết kế bị ảnh hưởng và mâu thuẫn nếu có.
4. **Lỗi mềm:** KG vắng, quá cũ (URD/SRS/SAD lệch hash) hoặc lệnh lỗi ⇒ không có khối ngữ cảnh, Solution vẫn sinh, run ghi cảnh báo `DESIGN_CONTEXT_UNAVAILABLE` để người duyệt thấy. Không chặn luồng.
5. **Cấu hình:** cờ theo project (mặc định tắt); ghi vào run: khoá đã dùng, số phần tử, số ký tự, `snapshot` của KG để truy vết.
6. **Lưu ý đã biết:** `kg search` là tìm toàn văn, độ phủ phụ thuộc chất lượng khoá; đo ở T1.10 (so Solution có và không có ngữ cảnh).

### 3.3 Run `spec` (thay `ai.complete` ở `plan generate` cho loại có profile OpenSpec `full`)
1. **Chọn chế độ.** `generate_plan.go` chọn `AnalysisModeSpec` khi `OpenSpecProfileFor(type) == full` và cờ bật; ngược lại giữ `Complete` như cũ.
2. **Worktree riêng cho run**, tạo từ nhánh nền của repo đã đăng ký. Run không chạm thư mục chính của repo.
3. **Cấp phát (provisioning)** vào worktree trước khi chạy:
   - skill dự án `.claude/skills/tdd-agent-openspec/SKILL.md` (nguồn `skills/tdd-agent-openspec/` trong repo Orca; kèm hash phiên bản);
   - MCP chỉ đọc: chạy `tdd-agent mcp enable --read-only` trong worktree (ghi `.mcp.json` và `[mcp] write=false, approvals=false`);
   - **mọi lệnh `tdd-agent`/MCP trong run phải có `-C <worktree>`** (xem 2.2: không có `-C` thì dùng KG của checkout chính);
   - snapshot `.tdd-agent/` (config + KG + operational) sao chép từ repo đã đăng ký bằng `sqlite3 .backup`; kiểm `tdd-agent status` báo URD/SRS/SAD "up to date". Lý do snapshot: `change` ghi vào `.tdd-agent/` và `docs/tdd/`, không được làm bẩn repo nguồn hay chạy song song hai run trên cùng một SQLite.
4. **Lệnh agent.** `agent.execPrompt` mở rộng: `mcpConfig` và `allowedTools` (thêm `mcp__tdd-agent__*`, `Bash(tdd-agent:*)`, `Bash(openspec:*)`, `Read/Glob/Grep/Write` giới hạn); agent cũ bỏ qua khoá lạ.
5. **Prompt của run** (`BuildOpenSpecPlanPrompt` mở rộng): change id, đường dẫn file CR, cấm sửa ngoài `openspec/changes/<id>/` và `docs/tdd/`, trả duy nhất một JSON Plan theo schema Plan hiện có.
6. **Skill dặn agent:** (a) `doctor` và `status`; (b) ghi CR từ nội dung Orca đưa; (c) `tdd-agent change <cr>`; (d) khi cổng dừng vì thiếu thông tin: dùng MCP đọc (`kg_resolve_scope`, `impact_get`, `kg_neighbors`) rồi `change update --code-target …`/`--acceptance …`, không tự bịa; (e) được duyệt G1, G2, G5 bằng actor `claude` khi luật của change đã đạt (không ép duyệt khi `change status` báo blocker), không bao giờ duyệt gate của Orca, ghi rõ ai duyệt gì trong báo cáo run; (f) dựng Plan JSON theo mục 3.4; (g) dừng và báo `blocked` kèm `change status --why` khi không tiến được.
7. **Sau run:** `git status` của worktree chỉ được đổi dưới `openspec/changes/<id>/` và `docs/tdd/`; ngoài ra ⇒ run `failed` (`REQUEST_SPEC_RUN_OUT_OF_SCOPE`), giữ worktree để điều tra. Plan JSON qua `plan_validation.go` và thử lại như cũ. Ghi `openspec_changes` (change id, đường dẫn, commit, trạng thái, hash skill, snapshot KG).
8. **Lỗi và thời gian.** `change status` ở `blocked`/`failed`/`awaiting_*` ⇒ run `failed` kèm `change status --why` (hiện ở `plan proposal`). Giới hạn run `spec` riêng; bảo đảm gia hạn lease khi run dài hơn 90 s (T2.9); tăng `PLAN_RUN_TIMEOUT_MS` của UI cho chế độ này.

### 3.4 Quy tắc dựng Plan từ `tasks.md` (mặc định, chỉnh sau spike)
| `tasks.md` | Plan của Orca |
|---|---|
| mỗi mục `## N. <khu vực>` | một Phase, đúng thứ tự; `depends_on_phase_indices` = Phase liền trước, trừ khi skill ghi chú song song |
| mỗi dòng `- [ ] N.M CS-xxx: <hành động> <đường dẫn::symbol>; verified by TC-CS-xxx: GIVEN … WHEN … THEN …` | một task: `title` = `CS-xxx <symbol>`; `description` = nguyên dòng; `prompt_template` = khối chuẩn (mục 3.6) trỏ tới `spec.md` |
| `trace-manifest.json` | giữ trong gói; `ai_context` của task kèm `CS-xxx`, `TC-CS-xxx` |
Số Phase/task phải nằm trong `PlanLimits`; vượt thì skill gộp theo khu vực và báo trong `notes`.

### 3.5 Gate và người duyệt
Quyết định của người dùng (2026-10-14): `claude` (coding_agent) được làm approver của G5 và G7 trong tdd-agent; BL-J01 còn `Open` nên pilot không merge vào nhánh phát hành; chấp nhận chi phí đã ước lượng.
- Gate Solution, Plan, Phase của **Orca** vẫn do người quyết định; agent không tự duyệt (luật hiện có của Orca). Đây là điểm chịu trách nhiệm của người.
- Trong tdd-agent: G1, G2, G5, G7 có thể do `claude` duyệt, nên run `spec` tự đi tới `spec_authored` mà không dừng chờ người. Hệ quả cần ghi nhận: lịch sử tdd-agent sẽ ghi G5/G7 do agent duyệt; sự đồng ý của người nằm ở gate Plan của Orca (và gate Phase). Vì vậy gate Plan phải hiển thị người duyệt thấy gì đã được agent duyệt.
- G6 (kết quả test thật) không bao giờ tự động: agent chạy test và ghi `tdd-agent receipt`; G7 do `claude` duyệt sau khi có receipt đủ cho các tiêu chí. Orca không coi G7 là bằng chứng nghiệm thu: bằng chứng là receipt + test xanh trong worktree + gate Orca.
- Cấu hình pilot: khai báo `claude:coding_agent` với vai BA, SA/Dev, SA, BA/owner; `datbeohbbh` (human) vẫn khai báo để dùng khi cần ghi đè hoặc duyệt tay. Ghi chú trong config nêu rõ đây là quyết định của người dùng cho pilot, không áp dụng mặc định cho dự án khác.
- Người duyệt gate Plan của Orca thấy tóm tắt gói (`review.md` của change), danh sách CS/TC, các cảnh báo gap, và ai đã duyệt từng cổng tdd-agent.
- Cách khác (không dùng ở pilot, giữ làm tuỳ chọn cứng hơn): Orca gọi `tdd-agent change approve --actor <người> --rationale … --hash <hash từ change decision>` khi gate Plan được duyệt, để G5 mang tên người.

### 3.6 Agent thực thi và kết thúc
- Skill `tdd-agent-exec`: đọc `tasks.md` và `spec.md` của task; dùng MCP chỉ đọc tra phần tử thiết kế; chạy đúng test của `TC-CS-xxx`; tuân luật dự án (thư mục bảo vệ, GitNexus); không sửa `openspec/`, `docs/tdd/`, `.tdd-agent/`.
- `prompt_template` chuẩn mỗi task: tên change, `CS-xxx`, đường dẫn spec, scenario cần pass, lệnh test, điều cấm. Đây là chỗ đưa tiêu chí chấp nhận vào prompt thực thi (khoảng trống hiện nay; chú ý trần 2000 ký tự của `ai_context`).
- Kết thúc task: chạy test thật và `tdd-agent receipt <run> <CS-id> --criteria … --result pass|fail --actor <id>`; fail ⇒ task không `done`.
- Khi Request `completed`: `OpenSpecArchiveConsumer` chạy `openspec archive <id>`, cập nhật `openspec_changes`; lỗi archive không làm mất trạng thái `completed` mà ghi cảnh báo.

### 3.7 An toàn
- Agent vẫn chạy `--dangerously-skip-permissions` (B4). Giảm rủi ro: worktree riêng, kiểm `git status` sau run, `allowedTools` hẹp ở run `spec`, nội dung Request đưa qua file CR có rào chắn, **chặn push** như bộ e2e (`agent_push_block.py`, bắt buộc vì repo có remote GitHub thật).
- Với pilot trên repo thật: dấu vân tay thư mục chính và worktree pilot trước/sau (HEAD, nhánh, file đổi/untracked theo hash, ref), watchdog giữa run; chỉ cho phép đổi trong worktree task do Orca tạo.
- Run `spec` có công cụ và quyền ghi khác `ai.complete`: coi là rủi ro mới, cần đánh giá bảo mật riêng (T5.2) trước khi bật ngoài pilot.
- Chi phí: trần số lượt `change` và tổng chi phí mỗi Request (cấu hình); ghi `tdd-agent metrics` vào run.

## 4. Runbook e2e trên CR thật (vnp-qa-platform)

### 4.1 Bước 1: vào worktree của TMS
```bash
cd /Users/datbeohbbh/Desktop/work/vnpay/agentic-app/vnp-qa-platform/.claude/worktrees/tdd-agent-bl-h01-h02
git status -sb                      # nhánh feat/tdd-agent-bl-h01-h02, sạch
git log --oneline -1                # 2932acb3
tdd-agent doctor                    # không có dòng FAIL
```
Điều kiện: thư mục chính không bị sửa; dấu vân tay hai nơi đã lấy (T4.2).

### 4.2 Bước 2: pipeline tdd-agent, khởi tạo knowledge và sinh TDD
```bash
tdd-agent init --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md \
  --name TMS --tdd-id TDD-TMS --docs-dir docs/tdd --no-prose \
  --approver datbeohbbh:human:SA,SA/Dev,BA/owner \
  --approver claude:coding_agent:BA,SA/Dev,SA,BA/owner
tdd-agent status ; tdd-agent kg stats ; tdd-agent kg conflicts ; tdd-agent review --list
git diff --stat -- docs/tdd/TDD-TMS.md   # TDD đã được theo dõi: diff so với bản đã commit
```
- `--no-prose` theo ghi chú của cấu hình cũ (FACT từ mô hình gây xung đột chặn G5). Đổi lại KG nghèo ngữ cảnh văn xuôi hơn; đo ảnh hưởng ở T1.10.
- Nếu `init` từ chối vì TDD đã tồn tại: xác nhận rồi dùng `--force`, đọc diff TDD trước khi giữ.
- Người duyệt xử lý `needs_review` và `readiness: blocked` cho các phần tử BL-H01/H02 chạm tới (FR-SYS-07, ENT-17, UC-16, UC-38, EIF-12, case-management) bằng `tdd-agent review --ids … --actor datbeohbbh --role SA --decision approved --rationale "…"`. Không duyệt hàng loạt `--all-pending` nếu chưa đọc.
- Commit `.tdd-agent/config.toml` và `docs/tdd/TDD-TMS.md` lên nhánh pilot; **không** commit `*.db`.
- Sao lưu baseline KG ra ngoài worktree (`sqlite3 … ".backup …"`) trước mọi lần chạy `change`, để khôi phục sau spike (T1.8, T1.14).

### 4.3 Viết CR
Hai file CR trong `docs/crs/v5/` từ BL-H01 và BL-H02: mục tiêu, thay đổi, phạm vi trong/ngoài, và tiêu chí `AC-…` dạng GIVEN/WHEN/THEN từ "Tiêu chí nghiệm thu" của backlog (BL-H01: hội thoại với LLM mock-ext OpenAI-compatible và một LLM thật được duyệt; khoá API không lộ ở UI/log; hết hạn mức báo rõ. BL-H02: từ mô tả sinh ≥ 10 case hợp lệ, chọn và lưu, có audit; lỗi định dạng không lưu một phần). Người viết backlog rà từng AC (G4 dừng nếu thiếu).

### 4.4 Bước 3: chạy luồng Orca để thực thi CR
Điều kiện: stack local chạy bản mới nhất (gồm code ở Giai đoạn 2 và 3); `dev-local-mac` khởi động lại với khối chặn push; `orca login` xong.
```bash
# project Orca: repo = đường dẫn worktree pilot, dev server dev-local-mac (tạo qua UI hoặc REST /v1/projects)
orca request create --project <id> --title "BL-H01 nền AI" --body-file docs/crs/v5/CR-BL-H01.md
orca request classify <R> ; orca request wait <R> --until awaiting_type_confirmation
orca request confirm-type <R> --type change_request --size L
orca clarification list <R>   # trả lời nếu AI hỏi
orca solution generate <R> --wait          # có ngữ cảnh KG; xem cảnh báo DESIGN_CONTEXT_UNAVAILABLE
orca solution choose <R> <S> --option <opt> ; orca approval approve <gate Solution>
orca plan generate <R> --wait              # run spec: OpenSpec + Plan
orca plan tree <R> ; orca approval approve <gate Plan>   # G5: người duyệt
orca approval list <R> --subject phase ; orca approval approve <gate Phase 1>
orca plan start-phase <R> <phase-1> ; ...  # lặp theo từng Phase
orca request wait <R> --until completed --timeout 4h
```
Sau đó BL-H02 cùng quy trình (sau khi BL-H01 `completed`). Kiểm kết quả ở mục 7.

### 4.5 Thứ tự và tiền đề
BL-H01 trước, BL-H02 sau. BL-J01 còn `Open`: pilot chỉ trên nhánh thử, không merge vào nhánh phát hành trước khi BL-J01 chốt. BL-H01 sửa FR-SYS-07 và gỡ OI-02: tdd-agent đề xuất `SRS.proposed.md` và `srs.diff` ở G1 để BA duyệt; luồng không ghi đè `docs/SRS.md`.

## 5. Rủi ro và quyết định mở
| # | Rủi ro / câu hỏi | Giảm thiểu / người quyết |
|---|---|---|
| R1 | `tdd-agent change` có chạy ổn định khi agent điều khiển (cổng dừng, hỏi người) | Spike thủ công (GĐ1) trước khi sửa Orca |
| R2 | Chất lượng `tasks.md` cho feature L và việc dựng Plan trong `PlanLimits` | Đo ở spike: số task, độ cụ thể, tỷ lệ task tự chạy được |
| R3 | KG mới `blocked` / nhiều phần tử chờ duyệt làm `change` dừng ở G2/G5 | Người duyệt xử lý trước (4.2); ghi quyết định vào KG |
| R4 | Chi phí và thời gian change feature lớn | Đo ở spike; trần chi phí/Request |
| R5 | Chạy song song nhiều run trên một KG | Snapshot theo run (3.3.3) |
| R6 | Run `spec` có công cụ và quyền ghi, đầu vào không tin cậy | Mục 3.7; T5.2 |
| R7 | Lease 90 s so với run dài | T2.9 |
| R8 | `kg search` toàn văn có thể bỏ sót hoặc kéo nhiễu cho khoá chung chung | Đo ở T1.10; bản 2 dùng MCP mở rộng theo quan hệ |
| R9 | `--no-prose` làm KG nghèo ngữ cảnh | Đo ở T1.10; thử bật prose có kiểm soát sau |
| R10 | Spike làm bẩn trạng thái KG trước e2e | Sao lưu baseline và khôi phục (T1.8, T1.14) |
| R12 | `claude` tự duyệt G5/G7 nên tdd-agent không còn cổng người; nếu gate Plan của Orca bị duyệt không đọc thì không ai đọc gói | Gate Plan hiển thị người/agent đã duyệt từng cổng; người duyệt bắt buộc mở `review.md`; đối chiếu receipt + test xanh (3.5) |
| R13 | tdd-agent tự dùng `.tdd-agent/` của thư mục cha khi chạy trong worktree lồng (cả worktree pilot lẫn worktree task của Orca) | Luôn `-C`; Orca ghi `-C` vào skill, prompt và cấu hình MCP; test chứng minh run không đọc/ghi KG của repo nguồn (T2.5) |
| R14 | Phân rã CR → yêu cầu không ổn định và gói có thể qua luật mà vẫn quá chung (F1, F7) | Cấu trúc CR theo yêu cầu có tên; kiểm chất lượng gói phía Orca trước khi dựng Plan (T2.11); đo ở T1.9a |
| R15 | `change update` đọc lại CR, mất waiver, mở lại cổng (F2, F3) | Một lần gọi đầy đủ, chỉ `--code-target` khi cần, đặt lại baseline khi hỏng (T2.5, T2.12) |
| R11 | Công việc CLI/gateway đang chưa commit; stack phải chạy đúng bản | T0.5 |
| Q1 | Gói OpenSpec cuối cùng nằm ở đâu để agent thực thi đọc: commit vào nhánh dự án hay giữ ở repo chỉ dẫn | Chọn ở T1.12 |
| Q2 | Bật cho `refactor` ngay không | Sau pilot |

## 6. Phương án đã cân nhắc và loại
- **MCP trong `ai.complete`** (kể cả ở Solution): vi phạm lý do tồn tại của chế độ đó; thay bằng truy vấn tất định phía Orca (3.2).
- **Để AI tổng quát tự viết OpenSpec từ MCP:** mất cổng kiểm, phản biện và truy vết của tdd-agent.
- **Chạy tdd-agent ngoài Orca rồi dán nội dung vào Request:** dùng làm spike (GĐ1), không phải thiết kế đích.
- **Dùng chung KG của thư mục chính bằng `-C`:** `change` ghi vào `.tdd-agent/` và `docs/tdd/`, làm bẩn thư mục chính và va chạm khi chạy song song; chọn snapshot.

## 7. Tiêu chí nghiệm thu của pilot (đo được)
1. Bước 2: KG của worktree dựng xong từ URD/SRS/SAD, `status` "up to date", TDD sinh ra và diff đã được người đọc; số phần tử/quan hệ/gap/xung đột được ghi và đối chiếu KG cũ.
2. Bước 3: BL-H01 đi hết luồng bằng CLI `orca`: Request → Solution có ngữ cảnh KG → OpenSpec → Plan → các Phase `done` → `completed`, không can thiệp tay ngoài các gate duyệt.
3. Mỗi task thực thi có `receipt` với kết quả test thật; bộ test của `TC-CS-*` xanh trong worktree task.
4. Đối chứng: Solution có và không có ngữ cảnh KG, Plan từ OpenSpec và Plan `ai.complete` cũ, trên cùng nội dung Request; ghi số liệu (phần tử thiết kế được nêu đúng, truy vết CS↔TC, số task tự chạy được không cần sửa tay).
5. An toàn: không đổi ngoài phạm vi (kể cả `services/infrastructure/integration-hub/`), không push, thư mục chính của vnp-qa-platform không đổi, 0 Request mồ côi, dữ liệu người dùng trong Orca không bị đụng.
6. Chi phí, thời gian từng giai đoạn và số lần phải can thiệp ngoài gate được ghi lại.
