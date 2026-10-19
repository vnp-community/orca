# CHECKLIST-TDD-001: triển khai tdd-agent trong luồng Request của Orca, pilot e2e trên vnp-qa-platform

**From Solution:** [SOL-TDD-001](../solutions/SOL-TDD-001-tdd-agent-in-request-flow.md)
**Pilot:** `vnp-qa-platform`, worktree `.claude/worktrees/tdd-agent-bl-h01-h02`, nhánh `feat/tdd-agent-bl-h01-h02` (từ `claude/tms-bug-triage-report-09f310`, commit `2932acb3`); use case BL-H01 rồi BL-H02.
**Trạng thái:** hướng pipeline tạm dừng sau spike (2026-10-14); checklist đang áp dụng là [CHECKLIST-TDD-002](CHECKLIST-TDD-002-knowledge-hub.md).
**Quy ước:** `[x]` đã xong và kiểm chứng · `[ ]` chưa làm · "Xong khi" là điều kiện kiểm được. Giai đoạn 1 có thể làm ngay (không cần code Orca). Giai đoạn 2 và 3 chỉ bắt đầu sau cổng go/no-go T1.14. Giai đoạn 4 (chạy e2e ba bước trên CR thật) cần Giai đoạn 2 và 3 xong.

---

## Giai đoạn 0: chuẩn bị

- [x] **T0.1** Worktree pilot đã tạo, không đụng thư mục chính. Xong khi: `git worktree list` của vnp-qa-platform có `tdd-agent-bl-h01-h02` trên `feat/tdd-agent-bl-h01-h02`, `git status` sạch.
- [x] **T0.2** Công cụ có trên máy: `tdd-agent` (`~/.local/bin`), `openspec` (`/opt/homebrew/bin`), `claude`, `uv`.
- [x] **T0.3** Đã đọc hiện trạng: KG cũ ở thư mục chính (935 phần tử, 585 quan hệ, 272 gap, `readiness: blocked`, 912 `needs_review`), 7 lần `change` cũ, change gần nhất dừng ở `awaiting_evidence (spec_authored)`. Worktree mới chưa có `.tdd-agent/`.
- [x] **T0.4** Quyết định đầu vào (người dùng xác nhận 2026-10-14): (a) BL-J01 còn `Open`: pilot chỉ trên nhánh thử, **không merge** vào nhánh phát hành; (b) `claude` (coding_agent) **được** làm approver của G5 và G7 (vai BA, SA/Dev, SA, BA/owner), người dùng chịu trách nhiệm qua các gate Plan/Phase của Orca; (c) chấp nhận chi phí theo mốc đã nêu (một change nhỏ khoảng 126 s, báo $4.07; feature cỡ L lớn hơn nhiều), không đặt trần cứng, nhưng mỗi lần chạy phải ghi `tdd-agent metrics` để theo dõi.
- [ ] **T0.5** Chốt phiên bản Orca dùng cho pilot: commit công việc CLI `orca` (login, request, solution, approval, plan, clarification) và 18 route REST ở gateway (hiện chưa commit, nhưng đã chạy thật trên stack local), theo nhóm (backend-go, tests/specs/docs), dùng hook bình thường, không `--no-verify`. Xong khi: `git status` không còn thay đổi của luồng này; stack local chạy đúng bản đã commit.

## Giai đoạn 1: bước 2 của runbook (tdd-agent) và spike thủ công. Cổng: go/no-go

Làm trong worktree pilot, không cần code Orca. Mục tiêu: dựng KG và TDD thật, và chứng minh tdd-agent + skill cho OpenSpec/Plan tốt trên BL-H01 trước khi sửa Orca.

- [x] **T1.1** Vào worktree và kiểm công cụ: `git status -sb` sạch, nhánh đúng, `tdd-agent doctor` không có dòng FAIL (cần `claude` đã đăng nhập, `openspec`). Xong khi: ảnh chụp đầu ra `doctor` lưu cạnh báo cáo. Kết quả (2026-10-14): `doctor` ok (claude 2.1.266, openspec 1.3.0). Lưu ý: chạy không `-C` trong worktree lồng sẽ trỏ về dự án ở checkout chính.
- [x] **T1.2** Khởi tạo KG và sinh TDD: `tdd-agent init --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md --name TMS --tdd-id TDD-TMS --docs-dir docs/tdd --no-prose --approver datbeohbbh:human:SA,SA/Dev,BA/owner --approver claude:coding_agent:BA,SA/Dev,SA,BA/owner`. Nếu từ chối vì TDD đã tồn tại thì xác nhận rồi `--force`. Xong khi: `.tdd-agent/` tồn tại, `tdd-agent status` báo URD/SRS/SAD "up to date". Kết quả: chạy `tdd-agent -C <worktree> init --no-generate …` rồi sửa `[project.sources]` về đường dẫn tương đối, rồi `tdd-agent -C <worktree> generate` (20 s, không gọi mô hình): KG 915 phần tử, 1544 quan hệ, 246 gap, 0 xung đột; TDD revision 1, `readiness: blocked`; checkout chính không đổi (đã kiểm mtime).
- [x] **T1.3** Kiểm KG mới: `status` (có dòng `readiness`), `kg stats`, `kg conflicts`, `review --list`. Đối chiếu số phần tử/quan hệ/gap/xung đột với KG cũ ở thư mục chính (935/585/272/0). Xong khi: bảng đối chiếu ghi số liệu; mọi xung đột chưa giải quyết có người xử lý hoặc ghi lý do. Kết quả: 915 phần tử đều `needs_review`; không có lớp UseCase/Interface (xem SOL 2.3); số quan hệ khác KG cũ (1544 so với 585) do phiên bản tdd-agent khác nhau, cần ghi nhận khi so sánh.
- [ ] **T1.4** Đọc diff TDD: `git diff -- docs/tdd/TDD-TMS.md` so với bản đã commit. Xong khi: người đọc ghi quyết định giữ bản mới hay hoàn tác, kèm lý do. Ghi nhận ban đầu: diff chủ yếu là siêu dữ liệu (revision 2→1, danh sách nguồn, snapshot id, số quan hệ), 562 chỗ THIẾU so với 563; chưa có quyết định giữ/hoàn tác của người.
- [ ] **T1.5** Duyệt phần tử liên quan tới BL-H01/H02 (FR-SYS-07, ENT-17, UC-16, UC-38, EIF-12, case-management): `tdd-agent review --ids … --actor datbeohbbh --role SA --decision approved --rationale "…"`; không dùng `--all-pending` khi chưa đọc. Xong khi: các phần tử đó không còn `needs_review`; `readiness` không chặn `change` vì lý do này.
- [ ] **T1.6** Commit `.tdd-agent/config.toml` và `docs/tdd/TDD-TMS.md` lên nhánh pilot; xác nhận `*.db` không bị đưa vào (`.tdd-agent/.gitignore`). Xong khi: `git show --stat HEAD` không có tệp `.db`.
- [ ] **T1.7** Viết CR cho BL-H01 và BL-H02 vào `docs/crs/v5/` (SOL 4.3), AC dạng `- AC-<mã>: GIVEN … WHEN … THEN …`. Xong khi: người viết backlog rà từng AC. Tiến độ (2026-10-14): đã soạn dự thảo `docs/crs/v5/CR-BL-H01_ai-foundation.md` (7 AC) và `CR-BL-H02_ai-case-generation.md` (6 AC) trong worktree pilot, chưa commit; chờ người rà từng AC.
- [ ] **T1.8** Sao lưu baseline KG ra ngoài worktree (`sqlite3 <db> ".backup <đích>"` cho cả `knowledge` và `operational`, kèm `config.toml`). Xong khi: có thư mục sao lưu và lệnh khôi phục đã thử (khôi phục vào thư mục tạm, `status` đúng).
- [x] **T1.9** Chạy `tdd-agent change docs/crs/v5/<CR-H01>.md --owner "<nhóm>"` tới chỗ dừng đầu tiên; ghi từng cổng G0–G5, thời gian, `tdd-agent metrics`, chỗ nào cần người. Xong khi: báo cáo ngắn (cổng, thời gian, chi phí, lý do dừng) lưu cạnh CR. Kết quả (2026-10-14): xem SOL 2.2b (F1 đến F7). Change của CR-H01 đã tới G5 nhưng **chưa duyệt** vì gói chỉ có một task thiếu đích mã và còn kịch bản 501 cũ; thời gian G0–G2 khoảng 15–25 s, G3–G5 3–5 phút mỗi vòng, lần cuối $0.46 cho 12 lượt gọi.
- [ ] **T1.9a** Ổn định phân rã (kết quả của T1.9): viết lại CR-H01 và CR-H02 thành các yêu cầu có tên theo nhóm "Phạm vi công việc" của backlog (BL-H01: aiclient; AIService + migration; route + AI_ENABLED; cấu hình mô hình + khoá mã hoá + hạn mức), nêu rõ cột kiểm chứng "trả 501" của FR-SYS-07 bị gỡ, kèm đích mã dự kiến cho từng yêu cầu; khôi phục baseline (T1.8/T1.14) rồi chạy lại **một lần** ở chế độ `auto` (waive ADR-12 nếu G2 báo, `update` chỉ `--code-target`). Xong khi: ≥ 4 CS cùng có đích mã, `tasks.md` ≥ 4 task cụ thể, `spec.md` không còn kịch bản 501, lặp lại cho kết quả tương đương ở ít nhất 2 lần; người dùng duyệt các thay đổi trong CR. Trạng thái: chờ người dùng quyết định cách tiếp cận (xem báo cáo).
- [ ] **T1.10** Thử ngữ cảnh KG cho Solution (chưa cần code Orca): viết script chạy `tdd-agent kg search` theo khoá lấy từ Request BL-H01 (SOL 3.2.1 và 3.2.2), dựng khối `<design_context>` ≤ 6000 ký tự; tạo hai Request giống nhau bằng CLI `orca` trong một project thử, một Request kèm khối ngữ cảnh dán vào body, một không; chạy `solution generate`. Xong khi: bảng so sánh (phần tử thiết kế nêu đúng, mâu thuẫn được phát hiện, nhiễu) và kết luận về độ phủ của `kg search` và ảnh hưởng của `--no-prose`.
- [ ] **T1.11** Viết bản đầu skill `tdd-agent-openspec` (SOL 3.3.6) và chạy tay `claude --print` trong worktree với đúng prompt dự kiến + `mcp enable --read-only`. Xong khi: agent tự chạy `change` đến `spec_authored`, duyệt G1, G2, G5 bằng actor `claude` chỉ khi luật đã đạt, và không chạm gate của Orca, và trả Plan JSON; lặp lại ≥ 2 lần cho kết quả ổn định.
- [ ] **T1.12** Chọn chỗ đặt gói OpenSpec để agent thực thi đọc (Q1): publish/copy từ `.tdd-runs/<change>/instruction-repo` vào `openspec/changes/<id>/` của nhánh, hoặc giữ ở repo chỉ dẫn. Xong khi: có lệnh cụ thể và worktree task tạo từ HEAD của nhánh thấy được `tasks.md` và `spec.md`.
- [ ] **T1.13** Duyệt G5 (do `claude` hoặc `datbeohbbh` qua `tdd-agent change decision` lấy hash rồi `change approve --actor … --rationale … --hash …`); dựng Plan JSON từ `tasks.md` theo SOL 3.4, đưa qua kiểm của Orca (`plan_validation.go` hoặc test tương đương); so với Plan sinh bằng `plan generate` cũ trên cùng Request (truy vết CS↔TC, độ cụ thể, số task chạy được không cần sửa tay). Xong khi: Plan hợp lệ trong `PlanLimits`; bảng so sánh có số liệu; lịch sử change ghi đúng actor và vai.
- [ ] **T1.14** Khôi phục baseline KG (T1.8) để spike không làm bẩn trạng thái trước e2e; **cổng go/no-go**: đạt khi T1.11 ổn định, Plan từ OpenSpec không kém Plan cũ ở T1.13, ngữ cảnh KG cho Solution có ích rõ (T1.10), chi phí/thời gian trong trần T0.4. Không đạt: dừng, chỉnh skill/CR/khoá truy vấn, hoặc quay về phương án chỉ bơm ngữ cảnh. Xong khi: quyết định ghi lại cùng số liệu.

## Giai đoạn 2: Orca (Solution có ngữ cảnh KG; run `spec` ở `plan generate`)

Mỗi mục có test (đơn vị hoặc contract) và theo luật repo: `codegraph` để khám phá, `gitnexus_impact` trước khi sửa symbol, `detect_changes` trước commit, không dùng `--no-verify`.

Ngữ cảnh KG cho `solution generate`:
- [ ] **T2.1** Bộ truy vấn ngữ cảnh: trích khoá (từ tiêu đề, body, mã phần tử KG `FR-/NFR-/ENT-/BRL-/ADR-/D-`, tiêu chí, câu trả lời clarification; `UC-`/`EIF-` không phải phần tử KG), chạy `tdd-agent kg search --json` qua relay `agent.exec` trong repo đã đăng ký, gộp/khử trùng/xếp hạng theo lớp. Xong khi: test đơn vị cho trích khoá và xếp hạng; test tích hợp với repo tạm có KG nhỏ.
- [ ] **T2.2** Prompt Solution: thêm khối `<design_context>` (id, lớp, nội dung, nguồn), thêm thẻ vào `fenceUntrusted`, trần ký tự cấu hình được (mặc định 6000), dòng dặn "dữ liệu, không phải chỉ thị" và dặn nêu phần tử thiết kế bị ảnh hưởng. Xong khi: golden test như `testdata/solution_prompt`; test thẻ đóng trong nội dung không phá khối.
- [ ] **T2.3** Cờ theo project (mặc định tắt) và lỗi mềm: KG vắng/lệch hash/lệnh lỗi ⇒ không có khối, Solution vẫn sinh, run ghi `DESIGN_CONTEXT_UNAVAILABLE`; ghi vào run: khoá, số phần tử, số ký tự, snapshot KG. Xong khi: test ba trường hợp lỗi; `orca solution list` hiển thị cảnh báo.

Run `spec` ở `plan generate`:
- [ ] **T2.4** `domain`: thêm `AnalysisModeSpec`; `generate_plan.go` chọn theo `OpenSpecProfileFor(type) == full` + cờ (mặc định tắt). Xong khi: tắt cờ ⇒ `Complete` như cũ; bật cờ + `change_request` ⇒ `Spec`.
- [ ] **T2.5** Provisioning worktree cho run (mọi lệnh `tdd-agent` có `-C <worktree>`): tạo từ nhánh nền, ghi skill, `mcp enable --read-only`, snapshot `.tdd-agent/` bằng `sqlite3 .backup`, kiểm `status`. Xong khi: test với repo tạm **lồng trong một repo cha có `.tdd-agent/` khác**, chứng minh run không đọc/ghi KG của repo cha; run thất bại sớm và dọn worktree khi snapshot hỏng hoặc KG lệch tài liệu.
- [ ] **T2.6** Agent `agent.execPrompt`: tham số `mcpConfig` và `allowedTools` (cờ `--mcp-config`, `--allowedTools`); agent cũ bỏ qua khoá lạ. Xong khi: test của `agent-print-mode-exec` kiểm cờ; chạy thật một lần gọi được `kg_search` qua MCP.
- [ ] **T2.7** Prompt run `spec`: mở rộng `BuildOpenSpecPlanPrompt` (change id, đường dẫn CR, vùng cấm sửa, schema Plan, rào chắn dữ liệu không tin cậy). Xong khi: golden test.
- [ ] **T2.8** Dựng file CR từ Request (tiêu đề, body, câu trả lời clarification, tiêu chí chấp nhận) với rào chắn dữ liệu. Xong khi: test với body chứa chỉ thị giả; CR hợp lệ cho `tdd-agent doctor`/`change`.
- [ ] **T2.9** Thời gian và lease: giới hạn run `spec` riêng; xác nhận hoặc thêm cơ chế gia hạn lease khi run dài hơn `LeaseTTL` (mặc định 90 s); tăng `PLAN_RUN_TIMEOUT_MS` của UI cho chế độ này. Xong khi: run giả 5 phút không bị thu hồi lease; UI không báo timeout sớm.
- [ ] **T2.10** Kiểm sau run: `git status` worktree chỉ đổi dưới `openspec/changes/<id>/` và `docs/tdd/`; ngoài ra ⇒ `REQUEST_SPEC_RUN_OUT_OF_SCOPE`, giữ worktree. Xong khi: test với một thay đổi ngoài phạm vi.
- [ ] **T2.11** Dựng và kiểm Plan từ kết quả run (đường kiểm hiện có, thử lại một lần); ghi `openspec_changes` (id, đường dẫn, commit, trạng thái, hash skill, snapshot KG). Xong khi: `commit_plan` chạy trên Plan từ OpenSpec; bản ghi đủ trường.
- [ ] **T2.12** Ánh xạ lỗi: `change status` ở `blocked`/`failed`/`awaiting_*` ⇒ run `failed` kèm `change status --why`; `orca plan proposal` hiển thị. Xong khi: test từng trạng thái.
- [ ] **T2.13** Gate Plan của Orca hiển thị tóm tắt `review.md`, danh sách CS/TC, cảnh báo gap và ai đã duyệt từng cổng tdd-agent (G1, G2, G5; actor, vai, thời điểm); ghi vào `openspec_changes`. Tuỳ chọn cứng hơn (SOL 3.5): gọi `change approve` với người duyệt khi gate Plan được duyệt. Xong khi: test hiển thị; gate Plan không duyệt được nếu thiếu gói hoặc `change` đang `blocked`/`failed`.
- [ ] **T2.14** CLI `orca`: hiển thị gói OpenSpec và trạng thái change của Request; cảnh báo ngữ cảnh KG ở `solution list`; `--wait` cho `plan generate` chế độ `spec`. Xong khi: test lệnh; README orca-cli cập nhật.
- [ ] **T2.15** Dọn: worktree run `spec` bị xoá khi Request `cancelled`/`completed` hoặc sau thời hạn. Xong khi: `report_orphan_requests.py` và danh sách worktree không còn dư.

## Giai đoạn 3: agent thực thi, kết quả test, lưu trữ

- [ ] **T3.1** Skill `tdd-agent-exec` và `prompt_template` chuẩn cho mỗi task (change, `CS-xxx`, spec, scenario, lệnh test, điều cấm, luật dự án: thư mục bảo vệ, GitNexus). Xong khi: golden test; có điều khoản "không sửa `openspec/`, `docs/tdd/`, `.tdd-agent/`".
- [ ] **T3.2** Đưa tiêu chí chấp nhận và `CS/TC` vào prompt thực thi (qua `ai_context` hoặc `prompt_template`, chú ý trần 2000 ký tự). Xong khi: test `buildExecutePrompt` thấy AC và TC.
- [ ] **T3.3** Báo kết quả: agent chạy test của `TC-CS-xxx` và ghi `tdd-agent receipt`; task không `done` khi test fail. Xong khi: test hai nhánh pass/fail.
- [ ] **T3.4** Điền `OpenSpecArchiveConsumer`: khi `completed` chạy `openspec archive <id>`, cập nhật `openspec_changes`; lỗi archive chỉ ghi cảnh báo. Xong khi: test với `openspec` thật hoặc giả có kiểm tra lệnh.
- [ ] **T3.5** Phân phối skill: `skills/tdd-agent-openspec` và `skills/tdd-agent-exec` trong repo Orca, có hash phiên bản; Orca ghi đúng phiên bản vào worktree. Xong khi: đổi skill đổi hash và hash được ghi vào `openspec_changes`.

## Giai đoạn 4: chạy e2e ba bước trên CR thật (SOL mục 4)

Điều kiện vào: Giai đoạn 1 đạt go/no-go; Giai đoạn 2 và 3 xong và đã deploy lên stack local; T0.4 và T0.5 xong.

- [ ] **T4.1** Chuẩn bị stack: redeploy bản mới nhất (`deploy/dev/scripts/redeploy-local.sh …`), `orca daemon status` khoẻ, `dev-local-mac` approved/healthy; khởi động lại agent với khối chặn push (`agent_push_block.py`) và kiểm nó có ở mọi tiến trình agent; cài `tdd-agent`, `openspec`, `claude` đăng nhập trên máy dev server. Xong khi: `git push --dry-run` trong worktree pilot thất bại cục bộ; `orca request list` chạy; từ một worktree task thử, `tdd-agent -C <worktree task> status` không trỏ về KG của checkout chính.
- [ ] **T4.2** An toàn: lấy dấu vân tay thư mục chính của vnp-qa-platform và worktree pilot (HEAD, nhánh, file đổi/untracked theo hash, mọi ref, danh sách worktree) và ảnh chụp dữ liệu người dùng trong Orca (project, Request) trước run; watchdog giữa run; so lại và dọn phần mới sau run. Xong khi: có script/quy trình lặp được và đã chạy trước run đầu.
- [ ] **T4.3** **Bước 1, vào worktree TMS:** `cd` vào worktree pilot; `git status` sạch; nhánh `feat/tdd-agent-bl-h01-h02`; `tdd-agent doctor` ổn. Xong khi: ghi lại HEAD và trạng thái.
- [ ] **T4.4** **Bước 2, pipeline tdd-agent:** nếu KG đang ở baseline sau T1.14 thì xác nhận `status` "up to date" và số liệu khớp T1.3; nếu cần dựng lại thì chạy lại T1.2 đến T1.6. Xong khi: KG và TDD sẵn sàng, `readiness` không chặn BL-H01, CR BL-H01/H02 đã commit.
- [ ] **T4.5** Đăng ký vnp-qa-platform thành project Orca qua UI (repo = đường dẫn worktree pilot, dev server `dev-local-mac`); bật cờ ngữ cảnh KG và cờ run `spec` cho project này. Xong khi: `orca request create` vào project và `classify` chạy được; project không có repo nào khác của người dùng.
- [ ] **T4.6** **Bước 3, luồng Orca cho BL-H01** theo SOL 4.4: `request create` (body từ CR) → `classify` → `confirm-type change_request --size L` → clarification → `solution generate --wait` (có ngữ cảnh KG) → `choose` → duyệt Solution → `plan generate --wait` (run `spec`) → `plan tree` → duyệt Plan (G5 người) → duyệt từng gate Phase theo thứ tự → `start-phase` → `request wait --until completed`. Xong khi: Request `completed`, mọi Phase `done`, không can thiệp tay ngoài các gate duyệt.
- [ ] **T4.7** Kiểm kết quả BL-H01: bộ test của `TC-CS-*` xanh trong worktree task; mỗi task có `receipt`; không có thay đổi ngoài phạm vi (kể cả `services/infrastructure/integration-hub/`); không push; dấu vân tay thư mục chính không đổi; 0 Request mồ côi; dữ liệu người dùng trong Orca không đổi. Xong khi: báo cáo kèm diff tóm tắt.
- [ ] **T4.8** BL-H02 sau khi BL-H01 `completed`, cùng quy trình T4.6 và T4.7. Xong khi: như T4.6 và T4.7.
- [ ] **T4.9** Ghi số liệu: thời gian từng giai đoạn, số lượt gọi mô hình và chi phí (`tdd-agent metrics` + Orca), số lần phải can thiệp ngoài gate, số task phải sửa tay, đối chứng Solution có/không ngữ cảnh KG và Plan OpenSpec/Plan cũ (SOL 7.4). Xong khi: bảng đối chiếu tiêu chí SOL mục 7.
- [ ] **T4.10** Quyết định về SRS: `SRS.proposed.md` cho FR-SYS-07 được BA xem; ghi chấp nhận hay không. Xong khi: có ghi chú quyết định cạnh CR.
- [ ] **T4.11** Dọn: huỷ/hoàn tất Request thử, xoá project thử nếu có, gỡ worktree run `spec` và worktree task theo quy định, khôi phục agent (bỏ khối chặn push nếu chỉ dùng cho pilot). Xong khi: `report_orphan_requests.py` báo 0 mồ côi.

## Giai đoạn 5: củng cố và quyết định

- [ ] **T5.1** Test e2e tự động cho luồng `spec` trên repo tạm (cách bộ e2e hiện có: stack local, Claude thật, kiểm không động vào dữ liệu người dùng). Xong khi: chạy xanh bằng `run_ui_flow.py` hoặc kịch bản CLI tương đương.
- [ ] **T5.2** Đánh giá bảo mật run `spec` (có công cụ và quyền ghi, đầu vào không tin cậy): chèn lệnh qua body Request, vùng ghi, mạng, push. Quyết định B4 (sandbox agent) trước khi bật ngoài pilot. Xong khi: báo cáo và quyết định.
- [ ] **T5.3** Quyết định mở rộng: bật cho `refactor`; ngữ cảnh KG bản 2 qua MCP (`kg_neighbors`, `impact_get`); đưa `flowDefinitions` thành cấu hình theo tenant. Xong khi: ba quyết định có ghi chú.
- [ ] **T5.4** Dọn nợ phát hiện: danh sách từ pilot (lỗi tdd-agent, sửa skill, thay đổi Orca) chuyển thành task riêng.

---

## Rủi ro đang theo dõi
R1 → T1.9, T1.11 · R2 → T1.13 · R3 → T1.3, T1.5 · R4 → T1.9, T4.9 · R5 → T2.5 · R6 → T5.2 · R7 → T2.9 · R8, R9 → T1.10 · R10 → T1.8, T1.14 · R11 → T0.5 · R13 → T2.5, T4.1 · R14 → T1.9a, T2.11 · R15 → T2.5, T2.12 · R12 → T2.13 · Q1 → T1.12 (xem SOL mục 5).

## Nhật ký thực hiện
- 2026-10-14: tạo worktree pilot, đọc hiện trạng, viết SOL-TDD-001 và checklist. Cập nhật: thêm ngữ cảnh KG cho `solution generate` (SOL 3.2, T2.1 đến T2.3, T1.10); viết lại runbook e2e ba bước (SOL 4, Giai đoạn 1 và 4), thêm sao lưu/khôi phục baseline KG (T1.8, T1.14) và chốt phiên bản Orca (T0.5). Chưa có thay đổi code Orca cho luồng này. Đã chạy T1.1 đến T1.3 trong worktree pilot (sau khi người dùng xác nhận T0.4); phát hiện và ghi lại: gốc dự án bị suy từ thư mục cha, `init -C` ghi đường dẫn tuyệt đối, KG không có lớp UseCase/Interface. Thư mục chính của vnp-qa-platform không bị sửa (chỉ chạy lệnh đọc của tdd-agent: `status`, `config`, `change status`, `change list`, `metrics`, `review --list`; các lệnh này có thể thêm dòng log trong `.tdd-agent/logs`).
