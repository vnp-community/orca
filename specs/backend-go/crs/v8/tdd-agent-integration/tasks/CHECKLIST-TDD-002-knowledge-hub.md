# CHECKLIST-TDD-002: tdd-agent làm knowledge hub, agent tự khai thác, `plan generate` trả OpenSpec; pilot trên vnp-qa-platform

**From Solution:** [SOL-TDD-002 bản 2](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) (thay hướng pipeline của [CHECKLIST-TDD-001](CHECKLIST-TDD-001-pilot-vnp-qa-platform.md))
**Nền sẵn có:** engine `openspec` của CR-REQ-026 ([BE-REQ-SOL-026](../../../v6/solution-engines/solutions/BE-REQ-SOL-026-openspec-solution-engine.md), [TASK-REQ-026-01..08](../../../v6/solution-engines/tasks/README.md)): đã có khung domain, parser/renderer `tasks.md`, cổng readiness, registry; **còn stub** ở engine, `ProposalWorkspace`, archive; registry chưa được nối vào luồng. Giai đoạn 2 và 3 hiện thực hoá phần này, không tạo chế độ mới (ánh xạ ở Phụ lục E).
**Pilot:** `vnp-qa-platform`, worktree `.claude/worktrees/tdd-agent-bl-h01-h02`, nhánh `feat/tdd-agent-bl-h01-h02` (từ `claude/tms-bug-triage-report-09f310`); use case BL-H01 rồi BL-H02.
**Nguyên tắc (SOL P1 đến P4):** Orca không gọi MCP và không truy vấn KG, agent tự làm; `plan generate` trả bộ OpenSpec do AI viết; Orca chỉ chọn chế độ, cấp môi trường, kiểm cấu trúc, dựng Plan; KG là dữ liệu không tin cậy.

---

## Tiến độ

| Giai đoạn | Mục | Xong | Cổng ra |
|---|---|---|---|
| 0. Nền đã có | 5 | 5 | worktree sạch, Orca commit |
| 1. Kiểm chứng (không sửa Orca) | 9 | 3 | **go/no-go T1.9** |
| 2. Orca: engine, agent, kiểm cấu trúc | 15 | 0 | test xanh, cờ tắt = hành vi cũ |
| 3. Hoàn tất và ghi ngược | 5 | 0 | đồng bộ, archive, bàn giao |
| 4. e2e ba bước trên CR thật | 8 | 0 | nghiệm thu SOL-002 mục 5 |
| 5. Củng cố | 4 | 0 | báo cáo bảo mật, quyết định |
| **Tổng** | **46** | **8** | |

**Phụ thuộc:** G0 → G1 → (go) → G2 → G3 → G4 → G5. Trong G1: T1.4 chặn T2.9; T1.5 chặn T2.4, T2.5; T1.7 chặn T3.1; T1.8 chặn T2.6. Trong G2: T2.1 và T2.2 trước T2.4; T2.3 trước T2.4; T2.4 trước T2.5, T2.6, T2.7.

## Tiêu chí hoàn thành chung (áp cho mọi mục có code)
- Có test (đơn vị hoặc contract) chạy xanh; lỗi và nhánh từ chối đều có test.
- `codegraph` để khám phá; `gitnexus_impact` trước khi sửa symbol, báo mức rủi ro HIGH/CRITICAL; `detect_changes` trước commit; lint và hook của repo qua, không `--no-verify`; file `.ts` không quá 300 dòng, không tắt `max-lines`.
- Không có stub trong code sản phẩm: hàm trả `nil`/rỗng chỉ để qua biên dịch là chưa xong.
- Cờ tắt ⇒ hành vi cũ không đổi (có test).
- Tài liệu đi kèm (README, spec) được cập nhật trong cùng thay đổi.

---

## Giai đoạn 0: nền đã có

- [x] **T0.1** Worktree pilot, công cụ (`tdd-agent`, `openspec` 1.3.0, `claude`, `uv`), KG dựng trong worktree (915 phần tử, 1544 cạnh, 246 gap, 0 xung đột), hai CR đã rà và commit (`18d68d01`). Quyết định của người dùng: không merge pilot (BL-J01 còn `Open`), chấp nhận chi phí, `claude` được duyệt review.
- [x] **T0.2** Khảo sát tdd-agent (SOL-002 mục 2): MCP đọc chạy thật qua stdio; cơ chế ghi (ChangeSet chỉ đề xuất, `kg import` nhập cả đồ thị, SAD chỉ đọc); không có Claude Code skill nào do tdd-agent xuất bản; CLI `openspec` có `new change`, `status`, `instructions`, `validate --strict --json`; khung engine CR-REQ-026 đã có một phần.
- [x] **T0.3** Đưa worktree pilot về trạng thái sạch sau spike: khôi phục baseline KG (`~/.cache/orca-pilot/vnp-qa-platform/baseline-1`), xoá `.tdd-agent/changes`, `openspec/`, `TDD-NEW-*`, hoàn tác `docs/tdd/TDD-TMS.md` về bản đã commit; bỏ `[approvals] mode = "auto"` do spike; `tdd-agent -C <worktree> mcp enable --read-only`. Dùng `bash -c` cho lệnh dọn (zsh dừng cả chuỗi khi glob không khớp). Xong khi: `git status` chỉ còn thay đổi có chủ đích; `tdd-agent -C <worktree> status` báo 915/1544/246, "No change requests yet". Chi tiết: [TASK-TDD-002-T0-03-clean-pilot-worktree.md](TASK-TDD-002-T0-03-clean-pilot-worktree.md). **Kết quả (2026-10-14):** worktree pilot khôi phục từ baseline-1, `tdd-agent status` 915/1544/246/0, không có CR; checkout chính không đổi.
- [x] **T0.4** Viết bộ skill (bản nháp, chưa thử trên agent thật): `skills/tdd-knowledge`, `skills/tdd-openspec-author` (đã khớp văn phạm `tasks.md` chặt), `skills/tdd-knowledge-writeback`, `skills/tdd-orca-task`.
- [x] **T0.5** Chốt phiên bản Orca dùng cho pilot: commit CLI `orca` và 18 route REST của gateway (đã chạy thật trên stack local nhưng chưa commit), cùng `skills/` và `specs/.../v8`, theo nhóm, không `--no-verify`. Xong khi: `git status` không còn thay đổi của luồng này. Chi tiết: [TASK-TDD-002-T0-05-commit-orca-work.md](TASK-TDD-002-T0-05-commit-orca-work.md). **Kết quả (2026-10-14):** 4 commit trên `feat/e2e-acceptance-fixes`: `5f7f0bbad` (route gateway), `8f422d513` (CLI), `f4f386a06` (skills), `73fbfa342` (specs); hook qua, chưa push.

## Giai đoạn 1: kiểm chứng agent + MCP + skill (không sửa Orca). Cổng go/no-go

Chạy bằng `claude --print` trong một **bản sao thử** của worktree pilot (skill đặt ở `.claude/skills/` của bản sao, không động tới `~/.claude`), mô phỏng đúng cách Orca sẽ chạy (Phụ lục A).

- [ ] **T1.1** Recall của việc tra KG do agent làm: bộ ≥ 10 Request mẫu (BL-H01, BL-H02 và ≥ 3 backlog khác của TMS) với danh sách id đúng ghi trước (từ bảng SRS); cho agent chạy `tdd-knowledge`; ghi recall, độ chính xác, số lần gọi tool, độ dài câu trả lời (mẫu ở Phụ lục C). Xong khi: bảng số liệu và kết luận về `classes`, `budget`. Chi tiết: [TASK-TDD-002-T1-01-measure-kg-recall.md](TASK-TDD-002-T1-01-measure-kg-recall.md).
- [ ] **T1.2** Solution có KG so với `ai.complete`: cùng Request BL-H01, chạy `orca solution generate` thường và một lượt agent với skill; chấm phần tử bị ảnh hưởng nêu đúng, xung đột ADR-12, nhiễu. Xong khi: bảng so sánh có ví dụ. Chi tiết: [TASK-TDD-002-T1-02-solution-with-kg-vs-complete.md](TASK-TDD-002-T1-02-solution-with-kg-vs-complete.md).
- [x] **T1.3** Tuân thủ skill: log (`--output-format stream-json`) cho thấy agent gọi `mcp__tdd-agent__*`, trả khối `Knowledge/Read/Gaps/Conflicts`, không gọi tool bị cấm; lặp ≥ 2 lần; sửa skill theo chỉ dẫn bị bỏ qua. Xong khi: ≥ 2 lần liên tiếp đạt. Chi tiết: [TASK-TDD-002-T1-03-skill-compliance-on-real-agent.md](TASK-TDD-002-T1-03-skill-compliance-on-real-agent.md). **Kết quả (2026-10-14):** 2/2 lần liên tiếp đạt (38 và 67 lượt, $0.81 và $1.01); xem [evidence/T1.3-T1.5-spike-results.md](../evidence/T1.3-T1.5-spike-results.md).
- [x] **T1.4** Chế độ chỉ đọc có gọi được tool MCP không (H1): chạy với `--permission-mode plan` và danh sách `--tools`/`--allowedTools` giống `agent.execPrompt` readonly, kèm `--mcp-config`; ghi cấu hình cần để tool MCP chạy được. Xong khi: có cấu hình cụ thể, hoặc kết luận cần chế độ khác (ghi hẹp) cho Solution. Chi tiết: [TASK-TDD-002-T1-04-readonly-mode-mcp-feasibility.md](TASK-TDD-002-T1-04-readonly-mode-mcp-feasibility.md). **Kết quả (2026-10-14):** **có**: `--permission-mode plan --tools Read,Glob,Grep` vẫn chạy tool MCP chỉ đọc; `--allowedTools` không hạn chế MCP (rào thực là server `--read-only`); server chỉ đọc không phơi `changeset_propose` (ảnh hưởng T1.6/T1.7); xem [evidence/T1.3-T1.5-spike-results.md](../evidence/T1.3-T1.5-spike-results.md).
- [x] **T1.5** **Spike OpenSpec do AI viết:** agent với `tdd-openspec-author` cho BL-H01 (Request + Solution đã chọn làm đầu vào, `request=<ref>` và id AC như Orca sẽ đưa), ghi gói vào `openspec/changes/<id>/` của bản sao thử; chạy script kiểm Phụ lục B. Lặp ≥ 3 lần. Xong khi: mỗi lần `openspec validate --strict` đạt, `ParsePlanRegion` (chạy bằng test Go nhỏ hoặc script) không báo `Violation`, ≥ 6 task mỗi task đủ bốn dòng với `Code:` hợp lệ, mỗi AC một scenario, không còn kịch bản "trả 501"; ghi mức giống nhau giữa các lần (số Phase/task, id được dẫn). Chi tiết: [TASK-TDD-002-T1-05-ai-authored-openspec-spike.md](TASK-TDD-002-T1-05-ai-authored-openspec-spike.md). **Kết quả (2026-10-14):** 3/3 lần đạt: 17/20/18 task, 4/4/5 phase, `validate --strict` đạt, 0 `Violation`, 7/7 AC có scenario và task, 0 id `Serves` ngoài KG; 2 đường dẫn `modify` sai ở lần 1; $10.07; xem [evidence/T1.3-T1.5-spike-results.md](../evidence/T1.3-T1.5-spike-results.md).
- [ ] **T1.6** Đo ghi-ngược trên **bản sao** KG (không bao giờ trên KG pilot): `changeset_propose` một ChangeSet nhỏ, rồi áp thử bằng script Python (`apply_changeset` + `commit_graph` trên bản sao) và đếm phần tử chuyển `needs_review` (H4). Xong khi: số liệu cho vài kích thước thay đổi và kết luận chấp nhận được hay cần gom. Chi tiết: [TASK-TDD-002-T1-06-measure-writeback-review-debt.md](TASK-TDD-002-T1-06-measure-writeback-review-debt.md).
- [ ] **T1.7** Quyết định kênh commit ghi-ngược: (a) thêm `tdd-agent kg apply <proposal_id> --actor --rationale --hash` (khuyến nghị), (b) Orca áp bằng script, (c) chỉ đề xuất, người áp tay. Xong khi: người dùng chọn; nếu (a) mở task trong repo tdd-agent. Chi tiết: [TASK-TDD-002-T1-07-decide-writeback-commit-channel.md](TASK-TDD-002-T1-07-decide-writeback-commit-channel.md).
- [ ] **T1.8** Chốt nơi lưu và cách đọc gói OpenSpec (Q1): CR-026 dùng nhánh `request/<số>-proposal`; thử worktree task tách từ commit đó thấy `tasks.md`/`spec.md`, hoặc đưa nội dung vào `prompt_template`. Xong khi: lệnh cụ thể chạy được trên repo thử. Chi tiết: [TASK-TDD-002-T1-08-proposal-bundle-location-and-reading.md](TASK-TDD-002-T1-08-proposal-bundle-location-and-reading.md).
- [ ] **T1.9** **Cổng go/no-go:** đạt khi T1.3 và T1.4 cho thấy agent dùng được MCP và skill, T1.5 cho gói đạt cấu trúc và ổn định giữa các lần, T1.1/T1.2 cho thấy KG có ích (đề xuất recall ≥ 80%, Solution tốt hơn hoặc không tệ hơn), có quyết định T1.7 và T1.8. Không đạt: chỉnh skill/prompt, bật prose có kiểm soát cho KG, hoặc dừng. Xong khi: quyết định ghi lại cùng số liệu. Chi tiết: [TASK-TDD-002-T1-09-go-no-go-gate.md](TASK-TDD-002-T1-09-go-no-go-gate.md).

## Giai đoạn 2: Orca (engine `openspec`, agent có MCP, kiểm cấu trúc, dựng Plan; không truy vấn KG)

- [ ] **T2.1** `agent.execPrompt`: tham số `knowledgeRoot` (đường dẫn tuyệt đối của repo có `.tdd-agent/`) và `proposeChangesets`; **agent** tự dựng cấu hình MCP chỉ đọc (lệnh cố định `tdd-agent -C <realpath> mcp serve`, danh sách tool cố định) rồi truyền `--mcp-config`/`--allowedTools` cho `claude`, Orca không gửi tệp cấu hình hay lệnh; khai báo feature `agent.execPrompt.knowledgeHub` và skill đã cài trong báo cáo khả năng; xoá stub `usecase/agent_capabilities.go`. Xong khi: test của `agent-print-mode-exec`, parser tùy chọn (từ chối đường dẫn tương đối/symlink/thiếu `.tdd-agent`) và `agent_prompt_relay`; chạy thật gọi được `kg_search`. Chi tiết: [TASK-TDD-002-T2-01-agent-knowledge-hub-param.md](TASK-TDD-002-T2-01-agent-knowledge-hub-param.md).
- [ ] **T2.2** Thêm `AnalysisModeAgentProposal` (migration không cần: CHECK/ENUM đã có `agent_proposal`); nối `EngineRegistry` vào `GenerateSolution` và `GeneratePlan` với engine `native` mặc định, **không đổi hành vi** (CR-026 TASK-05). Xong khi: toàn bộ test hiện có xanh; test chứng minh `native` gọi đúng đường cũ. Chi tiết: [TASK-TDD-002-T2-02-analysis-mode-agent-proposal-and-engine-registry.md](TASK-TDD-002-T2-02-analysis-mode-agent-proposal-and-engine-registry.md).
- [ ] **T2.3** `ProposalWorkspace` thật trên git-gateway (CR-026 mục F): `Ensure` (idempotent, nhánh `request/<số>-proposal`, không dùng `CreateWorktreeFromIssue`), `ReadFile`, `WriteFile`, `ChangedPaths`, `DiscardAll`, `Commit`, `Remove`; **và `DevServerExecutor` thật** (hiện trả cứng "openspec version 1.2.3" nên cổng readiness luôn đạt): relay `agent.exec`. Bỏ mọi stub. Xong khi: test với git-gateway và infra-fleet giả, một test tích hợp trên repo tạm. Chi tiết: [TASK-TDD-002-T2-03-real-workspace-and-executor-adapters.md](TASK-TDD-002-T2-03-real-workspace-and-executor-adapters.md).
- [ ] **T2.4** **Trước hết bổ sung test cho `tasks_md_parser.go`, `tasks_md_render.go`, `proposal_scope_check.go` (hiện không có test nào) theo fixture `testdata/openspec/tasks_*.md` của CR-026 TASK-03 (cả fuzz, CRLF, tiếng Việt có dấu, checkbox giả ngoài vùng), sửa lỗi test phát hiện;** rồi `openspecEngine.GeneratePlan` (CR-026 TASK-06, phần Plan): `Preflight`, tạo worktree, prompt (`openspec_plan_prompt` mở rộng: change id, `request=<ref>`, Request kèm id AC, Solution đã duyệt, giới hạn, skill), chạy `agent.execPrompt` với `mcpConfig`/`allowedTools`, `proposal_scope_check`, đọc `tasks.md` thô, `ParsePlanRegion`. Xong khi: golden test prompt; test với agent giả ghi gói mẫu; chạy thật một lần trên repo thử. Chi tiết: [TASK-TDD-002-T2-04-openspec-engine-generate-plan.md](TASK-TDD-002-T2-04-openspec-engine-generate-plan.md).
- [ ] **T2.5** Kiểm cấu trúc sau run và thử lại một lần (Phụ lục B): `validate --strict`, số task theo cỡ, đủ bốn dòng mô tả, `Code:` hợp lệ, AC ↔ scenario, `ac=` chỉ dùng id có thật. Xong khi: test từng mã lỗi; lần hai vẫn lỗi ⇒ run `failed` kèm lý do và giữ worktree. Chi tiết: [TASK-TDD-002-T2-05-bundle-structure-checks.md](TASK-TDD-002-T2-05-bundle-structure-checks.md).
- [ ] **T2.6** Dựng Plan và lưu: `PlanProposal` → `ValidateProposal` → `commit_plan`; `PlanRaw` giữ `tasks.md`; commit gói lên nhánh proposal; ghi `openspec_changes` (id, nhánh, commit, snapshot KG và id đã đọc do agent báo, hash skill). Xong khi: `commit_plan` chạy trên Plan từ gói; bản ghi đủ trường; test với `tasks.md` lỗi định dạng. Chi tiết: [TASK-TDD-002-T2-06-build-plan-and-persist-bundle.md](TASK-TDD-002-T2-06-build-plan-and-persist-bundle.md).
- [ ] **T2.7** Gate Plan và CLI: gate hiển thị gói OpenSpec, kết quả kiểm cấu trúc, tham chiếu và xung đột agent báo; `orca plan show`/`request show` nêu nhánh, commit, đường dẫn gói. Xong khi: test hiển thị; README orca-cli cập nhật. Chi tiết: [TASK-TDD-002-T2-07-gate-and-cli-bundle-display.md](TASK-TDD-002-T2-07-gate-and-cli-bundle-display.md).
- [ ] **T2.8** Thời gian: lease đã được gia hạn mỗi 30 s (`Heartbeat`, `LeaseTTL` 90 s) nên run dài không bị thu hồi; còn phải đặt `SpecTimeoutMS` riêng (mặc định 900 000, bằng trần 15 phút của `agent.execPrompt`), kiểm deadline của relay/gRPC, và làm `PLAN_RUN_TIMEOUT_MS` của UI (240 s) và `--timeout` của CLI (4 phút) phụ thuộc engine. Xong khi: run giả 5 phút hoàn tất không bị thu hồi; UI và CLI không báo hết thời gian sớm với engine `openspec`; engine `native` giữ giá trị cũ. Chi tiết: [TASK-TDD-002-T2-08-long-run-timeouts.md](TASK-TDD-002-T2-08-long-run-timeouts.md).
- [ ] **T2.9** Solution bằng agent chỉ đọc: sửa `modeFor` cho phép `agent_readonly` với loại `solution` khi project bật; prompt dùng `tdd-knowledge`; schema Solution thêm `design_references` tuỳ chọn (`snapshot`, `read`, `gaps`, `conflicts`, `available`), kiểm và lưu; cấu hình MCP là file tạm ngoài repo; kiểm repo "clean" và `MaxAgentRuns` giữ nguyên; KG vắng ⇒ Solution vẫn sinh với `available=false`. Xong khi: golden test prompt; chạy thật trên repo thử (phụ thuộc T1.4). Chi tiết: [TASK-TDD-002-T2-09-solution-by-readonly-agent-with-kg.md](TASK-TDD-002-T2-09-solution-by-readonly-agent-with-kg.md).
- [ ] **T2.10** Cờ theo project và cổng khả năng: dùng `ProjectEngineSettings` (CR-026 TASK-08, RPC get/set) cho engine `openspec`; cờ riêng cho Solution bằng agent; `Preflight` mở rộng kiểm `openspec`, MCP, skill đã cài, `.tdd-agent/config.toml` có trong repo (kiểm sự hiện diện tệp, không truy vấn KG); thiếu thì giữ chế độ cũ và hiển thị lý do. Cài skill: `npx skills add … --skill tdd-knowledge tdd-openspec-author tdd-knowledge-writeback tdd-orca-task --global`, ghi hash vào run. Xong khi: test tắt cờ và thiếu khả năng; cài thử trên máy dev server pilot. Chi tiết: [TASK-TDD-002-T2-10-engine-settings-preflight-skill-install.md](TASK-TDD-002-T2-10-engine-settings-preflight-skill-install.md).
- [ ] **T2.11** Thực thi: gửi `knowledgeRoot` (repo gốc) cho agent task; `prompt_template` mỗi task (id phục vụ, tiêu chí, `Verify`, điều cấm, đường dẫn spec) và tiêu chí chấp nhận trong prompt (trần 2000 ký tự của `ai_context`); **dùng hợp đồng thực thi có sẵn của task-service** (`TaskSpecV2` phạm vi `Include/Create` từ `Code:`, result block có nonce, `checks_run[{id,exit}]` làm bằng chứng `Verification`) nếu request-service nối được, nếu không thì bản tối thiểu với result block; khối `knowledge`/`writeback` được lưu; task có `Verify` chỉ `done` khi có kiểm tra `exit=0` (đề xuất, chờ xác nhận). Xong khi: test `buildExecutePrompt`, parser khối kết quả mở rộng, trạng thái hoàn thành. Chi tiết: [TASK-TDD-002-T2-11-execution-prompt-and-result-contract.md](TASK-TDD-002-T2-11-execution-prompt-and-result-contract.md).
- [ ] **T2.12** Dọn: worktree và nhánh proposal bị gỡ khi Request `cancelled` (giữ nhánh theo CR-026: `abandoned`) hoặc sau thời hạn; không để worktree mồ côi. Xong khi: `report_orphan_requests.py` và danh sách worktree không còn dư. Chi tiết: [TASK-TDD-002-T2-12-cleanup-proposal-worktrees-and-branches.md](TASK-TDD-002-T2-12-cleanup-proposal-worktrees-and-branches.md).
- [ ] **T2.13** Hồi quy: với cờ tắt, chạy lại bộ e2e thật hiện có (`run_ui_flow.py --real deepseek-review` và `--real deepseek-execute`, hoặc bản rút gọn) và test hiện có của request-service, agent, task-service. Xong khi: xanh như trước, ghi số liệu. Chi tiết: [TASK-TDD-002-T2-13-regression-with-flags-off.md](TASK-TDD-002-T2-13-regression-with-flags-off.md).
- [ ] **T2.14** Quan sát: ghi vào run số lần gọi tool (từ stream), snapshot KG, hash skill, thời gian, chi phí; metric theo CR-026 (`request_engine_runs_total`…). Xong khi: có trong `orca request show` và metric. Chi tiết: [TASK-TDD-002-T2-14-observability-tool-calls-and-metrics.md](TASK-TDD-002-T2-14-observability-tool-calls-and-metrics.md).
- [ ] **T2.15** Tài liệu: cập nhật README orca-cli, CR-026 (trạng thái từng TASK), mục lục `crs/v8`; ghi các thay đổi hợp đồng (prompt, schema `design_references`, tham số `execPrompt`). Xong khi: tài liệu khớp code đã merge. Chi tiết: [TASK-TDD-002-T2-15-documentation-and-contract-notes.md](TASK-TDD-002-T2-15-documentation-and-contract-notes.md).

## Giai đoạn 3: hoàn tất và ghi ngược

- [ ] **T3.1** Theo quyết định T1.7: nếu (a), thêm `kg apply` vào repo tdd-agent (có test, báo số phần tử đổi trạng thái); nếu (b), script áp có kiểm hash; nếu (c) bỏ qua. Xong khi: áp được một đề xuất `ok: true` trên bản sao KG. Chi tiết: [TASK-TDD-002-T3-01-kg-apply-command-in-tdd-agent.md](TASK-TDD-002-T3-01-kg-apply-command-in-tdd-agent.md).
- [ ] **T3.2** Orca lưu `proposal_id`, `ok`, số id bị chạm trong kết quả task và hiển thị ở gate; không tự áp. Xong khi: test hiển thị. Chi tiết: [TASK-TDD-002-T3-02-store-and-show-writeback-proposals.md](TASK-TDD-002-T3-02-store-and-show-writeback-proposals.md).
- [ ] **T3.3** `TasksMdSyncer` và archive thật (CR-026 TASK-07): tick `[x]` khi task xong (không bao giờ bỏ tick, lô tối đa mỗi 30 s mỗi Request); khi Request `completed` chạy `openspec archive <id> --yes` trong worktree proposal, kiểm `ChangedPaths`, commit lên nhánh proposal; lỗi thử lại tối đa 5 lần rồi `tasks_sync_state=failed`, không ảnh hưởng `completed`; không merge, không mở PR. Xong khi: test với `openspec` thật hoặc giả có kiểm tra lệnh; thay stub `OpenSpecArchiveConsumer`. Chi tiết: [TASK-TDD-002-T3-03-tasks-md-syncer-and-archive.md](TASK-TDD-002-T3-03-tasks-md-syncer-and-archive.md).
- [ ] **T3.4** Bàn giao khi `completed`: Orca liệt kê việc còn lại cho người (hợp nhất code, `tdd-agent -C <repo> generate`, áp đề xuất ChangeSet, xem nhánh proposal) trong `orca request show` (đề xuất, chờ xác nhận). Xong khi: test hiển thị. Chi tiết: [TASK-TDD-002-T3-04-completion-handover-list.md](TASK-TDD-002-T3-04-completion-handover-list.md).
- [ ] **T3.5** Báo cáo nợ duyệt: số phần tử `needs_review` trước và sau mỗi lần ghi (agent báo trong kết quả). Xong khi: có số liệu trong kết quả task. Chi tiết: [TASK-TDD-002-T3-05-review-debt-report.md](TASK-TDD-002-T3-05-review-debt-report.md).

## Giai đoạn 4: chạy e2e ba bước trên CR thật (SOL mục 4)

Điều kiện vào: T1.9 đạt; Giai đoạn 2 xong và deploy lên stack local; T0.3, T0.5 xong; (Giai đoạn 3 cho mục ghi-ngược).

- [ ] **T4.1** An toàn: khởi động lại agent với khối chặn push (`agent_push_block.py`); dấu vân tay thư mục chính và worktree pilot (HEAD, nhánh, file đổi/untracked theo hash, mọi ref, danh sách worktree) và ảnh chụp dữ liệu người dùng trong Orca trước/sau; watchdog giữa run. Xong khi: `git push --dry-run` trong worktree pilot thất bại cục bộ; script lặp được. Chi tiết: [TASK-TDD-002-T4-01-pilot-safety-harness.md](TASK-TDD-002-T4-01-pilot-safety-harness.md).
- [ ] **T4.2** **Bước 1:** vào worktree pilot, `git status` sạch, nhánh đúng. **Bước 2:** `tdd-agent -C <worktree> generate` (KG "up to date", số liệu khớp T0.1), `mcp enable --read-only`. Xong khi: ghi HEAD, snapshot, số phần tử. Chi tiết: [TASK-TDD-002-T4-02-steps-1-2-worktree-and-kg.md](TASK-TDD-002-T4-02-steps-1-2-worktree-and-kg.md).
- [ ] **T4.3** Đăng ký vnp-qa-platform thành project Orca (repo = đường dẫn worktree pilot, dev server `dev-local-mac`), bật cờ engine `openspec` và Solution bằng agent. Xong khi: `orca request create` và `classify` chạy được; `Preflight` báo đủ. Chi tiết: [TASK-TDD-002-T4-03-register-project-and-enable-flags.md](TASK-TDD-002-T4-03-register-project-and-enable-flags.md).
- [ ] **T4.4** **Bước 3, BL-H01:** `request create` (body từ CR) → `classify` → `confirm-type change_request --size L` → `solution generate --wait` → `choose` → duyệt → `plan generate --wait` (OpenSpec) → duyệt Plan (đọc gói) → từng Phase → `completed`. Xong khi: tiêu chí SOL-002 mục 5 (1 đến 6). Chi tiết: [TASK-TDD-002-T4-04-step-3-bl-h01-end-to-end.md](TASK-TDD-002-T4-04-step-3-bl-h01-end-to-end.md).
- [ ] **T4.5** Kiểm mã và phạm vi: test của dự án cho phần đã làm xanh trong worktree task; không đổi ngoài phạm vi (kể cả `services/infrastructure/integration-hub/`); không push; dấu vân tay thư mục chính không đổi; 0 Request mồ côi. Xong khi: báo cáo kèm diff tóm tắt. Chi tiết: [TASK-TDD-002-T4-05-verify-code-and-scope.md](TASK-TDD-002-T4-05-verify-code-and-scope.md).
- [ ] **T4.6** BL-H02 sau khi BL-H01 `completed`, cùng quy trình. Xong khi: như T4.4 và T4.5. Chi tiết: [TASK-TDD-002-T4-06-bl-h02-end-to-end.md](TASK-TDD-002-T4-06-bl-h02-end-to-end.md).
- [ ] **T4.7** Ghi số liệu (Phụ lục C) và đối chứng có/không KG, `ai.complete`/engine `openspec`. Xong khi: bảng đối chiếu SOL-002 mục 5. Chi tiết: [TASK-TDD-002-T4-07-collect-metrics-and-compare.md](TASK-TDD-002-T4-07-collect-metrics-and-compare.md).
- [ ] **T4.8** Dọn: huỷ/hoàn tất Request thử, gỡ worktree task và worktree proposal, khôi phục agent. Xong khi: `report_orphan_requests.py` báo 0 mồ côi. Chi tiết: [TASK-TDD-002-T4-08-cleanup-after-pilot.md](TASK-TDD-002-T4-08-cleanup-after-pilot.md).

## Giai đoạn 5: củng cố và quyết định

- [ ] **T5.1** Test e2e tự động cho luồng mới trên repo tạm (cách bộ e2e hiện có: stack local, Claude thật, không động vào dữ liệu người dùng). Xong khi: chạy xanh. Chi tiết: [TASK-TDD-002-T5-01-automated-e2e-for-new-flow.md](TASK-TDD-002-T5-01-automated-e2e-for-new-flow.md).
- [ ] **T5.2** Đánh giá bảo mật engine `openspec` và agent có MCP: chèn lệnh qua Request và qua nội dung KG; vùng ghi; mạng; push; quyết định B4. Xong khi: báo cáo và quyết định. Chi tiết: [TASK-TDD-002-T5-02-security-assessment.md](TASK-TDD-002-T5-02-security-assessment.md).
- [ ] **T5.3** Quyết định mở rộng: phản biện độc lập gói OpenSpec (Q2), loại Request khác (Q3), `light` profile cho bug/security/performance, bật prose có kiểm soát để KG giàu hơn, tái xét pipeline `change`. Xong khi: ghi chú quyết định. Chi tiết: [TASK-TDD-002-T5-03-expansion-decisions.md](TASK-TDD-002-T5-03-expansion-decisions.md).
- [ ] **T5.4** Tổng kết pilot: số liệu T4.7, những gì skill/prompt đã đổi, nợ còn lại chuyển thành task riêng. Xong khi: báo cáo ngắn và danh sách task. Chi tiết: [TASK-TDD-002-T5-04-retrospective-and-debt.md](TASK-TDD-002-T5-04-retrospective-and-debt.md).

---

## Phụ lục A: lệnh cho spike (T1.3, T1.4, T1.5)

Tạo **bản sao thử** (không dùng worktree pilot): `git worktree add <đường dẫn> -b spike/tdd-knowledge feat/tdd-agent-bl-h01-h02`; chép `.tdd-agent/` (config và KG sao lưu bằng `sqlite3 .backup`); đặt skill vào `<bản sao>/.claude/skills/`.

`/tmp/tdd-mcp.json` (đường dẫn tuyệt đối vì worktree lồng):
```json
{"mcpServers": {"tdd-agent": {"command": "tdd-agent", "args": ["-C", "<đường dẫn tuyệt đối của bản sao>", "mcp", "serve"]}}}
```
Chạy chỉ đọc (T1.3, T1.4):
```bash
cd <bản sao>
claude --print "<prompt>" --mcp-config /tmp/tdd-mcp.json --permission-mode plan \
  --allowedTools "mcp__tdd-agent__kg_search" "mcp__tdd-agent__kg_get_element" "mcp__tdd-agent__kg_neighbors" \
  "mcp__tdd-agent__kg_resolve_scope" "mcp__tdd-agent__kg_query" "mcp__tdd-agent__source_get" \
  --output-format stream-json --verbose
```
Chạy viết OpenSpec (T1.5): như trên nhưng bỏ `--permission-mode plan`, thêm `Bash(openspec:*)`, `Bash(tdd-agent -C * status)`, `Write`, `Edit` giới hạn trong prompt vào `openspec/changes/<id>/`.
Prompt T1.5 gồm: change id, `request=<ref>`, Request (tiêu đề, body từ CR, tiêu chí kèm id `AC-<n>`), Solution đã chọn, giới hạn số task, lệnh "dùng tdd-openspec-author và tdd-knowledge".
Sau mỗi lần chạy: `git status` bản sao (chỉ `openspec/changes/<id>/` đổi), rồi script Phụ lục B.

## Phụ lục B: hợp đồng kiểm cấu trúc (T1.5, T2.5)

| Kiểm | Mã lỗi đề xuất | Cách |
|---|---|---|
| Vùng ghi | `REQUEST_SPEC_RUN_OUT_OF_SCOPE` | `ChangedPaths` chỉ dưới `openspec/changes/<id>/` |
| Văn phạm `tasks.md` | `TASKSMD_*` (có sẵn trong parser) | `ParsePlanRegion` |
| Giới hạn Plan | mã của `ValidateProposal` | `PlanLimits` |
| `openspec validate` | `REQUEST_SPEC_VALIDATION_FAILED` | `openspec validate <id> --strict --json` qua `agent.exec` |
| Số task theo cỡ | `REQUEST_SPEC_TASK_COUNT` | S 1–5, M 3–12, L 6–30 (cấu hình) |
| Bốn dòng mô tả | `REQUEST_SPEC_TASK_INCOMPLETE` | Serves, Code, Verify, Done when |
| `Code:` hợp lệ | `REQUEST_SPEC_CODE_TARGET_INVALID` | tệp có thật, hoặc `create` với thư mục cha có thật (qua `ReadFile`/`StatFile`) |
| AC ↔ scenario | `REQUEST_SPEC_AC_NOT_COVERED` | mỗi id AC của Request có scenario cùng tên trong `spec.md` |
| `ac=` | `REQUEST_SPEC_AC_UNKNOWN` | chỉ id có trong Request |
| Khối kết thúc | `REQUEST_SPEC_REPORT_MISSING` | có `Change:`, `Knowledge:`; thiếu thì cảnh báo, không chặn |

## Phụ lục C: mẫu ghi số liệu

- **T1.1/T4.7 recall:** Request | id đúng (số) | id agent nêu (số) | đúng | thiếu | thừa | số lần gọi tool | độ dài câu trả lời.
- **T1.2:** Request | cách (ai.complete / agent+KG) | phần tử bị ảnh hưởng đúng/thừa/thiếu | xung đột nêu | nhiễu | ghi chú.
- **T1.5:** lần | Phase | task | task đủ bốn dòng | `validate` | `Violation` | AC có scenario | kịch bản cũ còn | thời gian | chi phí.
- **T1.6:** kích thước ChangeSet (số op) | id bị chạm | phần tử chuyển `needs_review` | tổng phần tử.
- **T4.7:** giai đoạn | thời gian | lượt gọi mô hình/tool | chi phí | số lần can thiệp ngoài gate.

## Phụ lục D: bản đồ file sẽ chạm tới

| Việc | File |
|---|---|
| Tham số `mcpConfig`/`allowedTools`, khả năng | `agent/src/relay/agent-print-mode-exec.ts`, `agent/src/relay/agent-readonly-tool-policy.ts`, `request-service/internal/adapter/grpcclient/agent_prompt_relay.go`, `request-service/internal/usecase/agent_capabilities.go` |
| Chế độ và engine | `request-service/internal/domain/analysis_run.go`, `internal/usecase/generate_solution.go` (`modeFor`), `generate_plan.go`, `solution_engine.go`, `engine_native.go`, `engine_openspec.go`, `engine_readiness_gate.go` |
| Workspace thật | `request-service/internal/adapter/grpcclient/proposal_workspace.go`, `dev_server_executor.go`, `usecase/proposal_scope_check.go` |
| Prompt và schema | `usecase/openspec_plan_prompt.go`, `solution_prompt.go`, `plan_prompt.go`, `domain/solution_options.go` |
| Plan từ `tasks.md` | `domain/tasks_md_parser.go`, `tasks_md_render.go`, `usecase/commit_plan.go`, `plan_validation.go` |
| Lưu và đồng bộ | `domain/openspec_change.go`, `adapter/{postgres,mysql}/openspec_change_repository.go`, `usecase/openspec_archive_consumer.go` |
| Thực thi | `task-service/internal/adapter/grpcclient/simple_executor.go` (`buildExecutePrompt`) |
| CLI | `backend-go/cmd/orca-cli/internal/command/plan_cmd.go`, `request_cmd.go` |
| Skill | `skills/tdd-knowledge`, `tdd-openspec-author`, `tdd-knowledge-writeback`, `tdd-orca-task`; `agent/src/shared/agent-feature-install-commands.ts` |

## Phụ lục E: ánh xạ với CR-REQ-026

| TASK của CR-026 | Trạng thái code | Mục ở đây |
|---|---|---|
| 01 domain, profile, change id | có mã | kiểm lại test; không việc mới |
| 02 migration và repository | `openspec_changes` có; `engine`, `proposal_branch` cần kiểm | T2.2, T2.6 |
| 03 parser và renderer `tasks.md` | có mã (217 và 111 dòng) **nhưng chưa có test** (`proposal_scope_check` cũng không) | T1.5 (dùng parser), T2.4 (bổ sung test trước), T2.6 |
| 04 cổng readiness | có mã, chưa có test | T2.10 (thêm test và mở rộng) |
| 05 giao diện `SolutionEngine`, native, ghim | có giao diện, chưa nối | T2.2 |
| 06 engine sinh Solution/Plan | **stub** | T2.4 (Plan); Solution `openspec` ngoài phạm vi (Solution do agent chỉ đọc, T2.9) |
| 07 đồng bộ `tasks.md` và archive | **stub** | T3.3 |
| 08 cài đặt, RPC, metric | một phần | T2.10, T2.14 |

## Rủi ro đang theo dõi
H1 → T1.4 · H2 → T1.5, T2.5 · H3 → T1.1 · H4 → T1.6, T3.5 · H5 → T1.7, T3.1 · H6 → T5.2 · H7 → T2.1, T2.10 · H8 → T1.3 · H9 → T0.5 · H10 → T2.8 (xem SOL-002 mục 6; phần lease đã được heartbeat xử lý).

## Nhật ký thực hiện
- 2026-10-14: đổi hướng sang knowledge hub (bản 1); khảo sát MCP và cơ chế ghi; viết skill nháp và SOL-TDD-002. Bản 2: Orca không tự gọi MCP; Solution và Plan do agent làm với MCP + skill; `plan generate` trả bộ OpenSpec do AI viết. Hoàn thiện checklist: phát hiện engine `openspec` của CR-REQ-026 đã có khung nhưng còn stub (`openspecEngine`, `ProposalWorkspace`, archive; registry chưa nối), cột `mode` đã cho phép `agent_proposal` (không cần migration), parser `tasks.md` chặt (văn phạm vùng `orca:begin/end`) nên skill `tdd-openspec-author` được sửa cho khớp; chuyển các việc Giai đoạn 2 và 3 thành hiện thực hoá khung đó. Chưa có thay đổi code Orca; worktree pilot còn ở trạng thái spike (T0.3).
- 2026-10-14 (hoàn thiện tasks): mỗi task còn mở có một file riêng (43 file, mục lục ở [tasks/README.md](README.md)), viết sau khi đọc code thật. Phát hiện làm đổi thiết kế: `DevServerExecutor` là stub trả thành công giả (cổng readiness luôn đạt), `PinSolutionEngine` và `SetProjectEngineSettings` là bản giả, `agent_capabilities.go` là stub, lease đã có heartbeat (H10 chỉ còn trần 15 phút của agent), task-service đã có hợp đồng thực thi (`TaskSpecV2`, result block, `checks_run`), Orca chỉ gửi `knowledgeRoot` còn agent tự dựng cấu hình MCP. Số route REST của gateway là 18 (không phải 17).
