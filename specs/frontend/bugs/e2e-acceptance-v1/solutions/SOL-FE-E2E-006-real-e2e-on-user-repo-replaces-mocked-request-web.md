# SOL-FE-E2E-006: e2e thật từ UI trên repo `deepseek-harness` của người dùng; thay bộ Playwright mock `request-web`

**Yêu cầu của người dùng (2026-10-11):** "phần 3. UI với mock cần sửa đổi là chạy thật, không mock, chạy trên stack local, dùng repo `/Users/datbeohbbh/Desktop/stuff/agentic/deepseek-harness` làm cases test. Test e2e từ UI phải xử lý các luồng bao gồm cả ra lệnh cho agents chạy code làm việc thực tế trên repo test được chỉ định. Chạy case 2 (`run_ui_flow.py --real`) và sửa case 3 (Playwright `request-web`, 74 test, 15 file) thành UI tương tác thật trên stack docker local, không mock thành phần nào."
**Tasks:** [TASK-FE-E2E-028](../tasks/TASK-FE-E2E-028-real-e2e-on-deepseek-harness.md), [TASK-FE-E2E-029](../tasks/TASK-FE-E2E-029-retire-mocked-request-web-and-coverage-matrix.md) · **Bảng ánh xạ:** [COVERAGE-request-web-mock-to-real.md](../COVERAGE-request-web-mock-to-real.md)

Tài liệu viết TRƯỚC khi chạy bộ mới; kết quả thật nằm ở phần "Kết quả thực thi" của hai task.

## 1. Khác gì so với các spec `--real` cũ

Các spec `--real` cũ chạy Claude thật nhưng trên **repo tạm** (`/tmp/e2e-ui-*`, có hook chặn công cụ `tool-guard`): không có gì của người dùng để mất. Lần này repo là **của người dùng**, có remote GitHub thật (`git@github.com:deepseek-ai/deepseek-harness.git`), có file chưa theo dõi của họ (`docs/ARCHITECTURE-vi.md`, `docs/KNOWLEDGE-GRAPH-MEMORY-vi.md`, `docs/openai-gateway/`), một worktree riêng (`deepseek-harness-test-01`) và agent chạy `claude --dangerously-skip-permissions` không sandbox (B4, người dùng chưa quyết). Vì vậy an toàn phải được thiết kế lại, không phải bỏ đi.

## 2. Mô hình an toàn (mọi spec `deepseek-*`)

| Nguy cơ | Biện pháp | Mã |
|---|---|---|
| Agent ghi vào checkout chính của người dùng | Agent chỉ làm việc trong worktree task `deepseek-harness-<nhánh>` do Orca tạo cạnh repo. Checkout chính được lấy dấu vân tay (HEAD, nhánh, từng file đã đổi/chưa theo dõi theo hash, config cục bộ, mọi ref) trước run, so mỗi 15 giây trong run (watchdog; sai lệch là dừng run và giết claude), và so lại sau dọn dẹp | `live-main-checkout-rules.ts` (quy tắc thuần, có `node --test`), `live-main-checkout-guard.ts` |
| Agent đẩy lên GitHub thật | `run_ui_flow.py --real deepseek*` khởi động lại agent với `GIT_CONFIG_*` trong môi trường: `remote.origin.pushurl` trỏ vào đường không tồn tại và `pushInsteadOf` cho cả URL ssh/https của GitHub. Không đụng `.git/config` hay `~/.gitconfig`. Fixture **từ chối chạy** nếu chưa có khối này ở MỌI tiến trình agent và `git push --dry-run` (theo tên remote và theo URL gõ tay) chưa thất bại cục bộ. Sau run: ref `refs/remotes/*` mới hay tag mới là vi phạm | `agent_push_block.py`, `deepseek-repo-fixture.ts` |
| Worktree/nhánh do run tạo còn sót | `removeGrowth`: chỉ gỡ cái MỚI so với trước run (worktree `<repo>-*`, rồi nhánh), sau đó đòi trạng thái giống hệt ban đầu | `live-main-checkout-guard.ts` |
| Ghi ra ngoài (thư mục cạnh repo, `$HOME`) | Danh sách thư mục cạnh repo, `$HOME`, `Desktop` được so trước/sau; chỉ cho phép `<repo>-*` mới sinh ra bởi run | rules + `live-safety-invariants.ts` |
| Repo khác của người dùng bị đụng | Mọi repo đang đăng ký trên dev server vào `ORCA_E2E_GUARD_PATHS` như `--real addproject` (trừ repo đích, vì repo đích có luật riêng: `ORCA_E2E_GUARD_EXCLUDE`) | `deepseek-flow-harness.ts` |
| Project/Request của người dùng bị đụng | Run tự tạo project qua UI tên `e2e-ds-<mmddhhmmss>-deepseek-harness`; dọn đúng project đó (huỷ Request, `repo.rm`, `project.delete`: chỉ bookkeeping, không xoá thư mục); so ảnh chụp dữ liệu người dùng trước/sau | `live-add-project-ledger.ts` (dùng lại), `live-user-data-snapshot.ts` |
| Task làm việc ngoài phạm vi | Yêu cầu giới hạn vào thư mục chưa tồn tại `scripts/e2e-sandbox/` (hai file mới), không commit; kế hoạch bị đọc trước khi duyệt (`UNSAFE_PLAN_TEXT`); sau run đòi chỉ có file dưới thư mục đó thay đổi và `node --test` xanh | `deepseek-change-checks.ts` |

Không cài được hook `tool-guard` trong repo này: `.claude/settings.json` bị `.gitignore` nên worktree task không thừa hưởng. Điều đó (và B4) được nêu rõ thay vì giả vờ che.

## 3. Thiết kế bộ e2e thật (thay cho 15 file mock)

Mọi thứ đi qua UI thật trên frontend `localhost:28080`, gateway thật, backend thật, agent `dev-local-mac` với Claude Code thật. Không `page.route`, không WebSocket giả, không `mockRequestApp`.

| Spec (`ui-live/`) | Chạy bằng | AI | Nội dung |
|---|---|---|---|
| `deepseek-ui-real` | `--real deepseek-ui` | không | Add Project trên repo thật (tên mặc định, sửa được, project riêng gắn host), chặn trùng (có/không `/` cuối), "Mở project đó", nhãn host của repo, tạo worktree từ "+" (nằm cạnh repo `<repo>-<tên>`) |
| `deepseek-readonly-real` | `--real deepseek-readonly` | Claude thật, chỉ đọc | `question` về `package.json` gốc (sự thật đọc từ repo lúc chạy: tên gói, `engines.node`, số workspaces); Create request (ghi chú repo/host, RPC không có `source`), vỏ trang Requests (danh sách, j/k, số chờ duyệt, tab, Escape), ô Answer, repo chỉ bị đọc |
| `deepseek-review-real` | `--real deepseek-review` | Claude thật | `change_request` đòi hai phương án: thẻ phương án và bảng so sánh, Reject lý do ngắn bị khoá, Mod+Enter, Request quay lại phân tích và màn hình theo Solution MỚI không cần tải lại, chọn phương án không khuyến nghị cần lý do, `solution.choose` trước `approval.approve`; `hotfix` không có thanh quyết định; không bao giờ duyệt Plan |
| `deepseek-execute-real` | `--real deepseek-execute` | Claude thật, GHI mã | `change_request` có Phase: Plan tab khớp `task.list` (phase theo thứ tự, tiến độ), Reject ngắn bị khoá, Tree/Graph, "View graph", cổng Phase (Start bị khoá tới khi duyệt), agent viết `slugify()` + test trong worktree task, đọc lại từ đĩa và chạy `node --test`, task sheet sau run (Result, độ rộng, Escape) |
| `deepseek-task-real` | `--real deepseek-task` | Claude thật, GHI mã | Plan một task: theo dõi agent đang chạy (task sheet, dev server bận, dòng hoạt động trực tiếp, phát lại sau tải lại, Open in Git), phán quyết bằng `live-agent-session-judge.ts`; cuối cùng gỡ worktree rồi kiểm "Open in Git" giải thích đúng |

Hạ tầng chung: `deepseek-flow-harness.ts` (bọc `runRealFlow`: fixture là repo của người dùng, project tạo qua UI, so dữ liệu người dùng), `deepseek-add-project-steps.ts`, `deepseek-request-steps.ts`, `deepseek-plan-checks.ts`, `deepseek-task-sheet-checks.ts`, `deepseek-change-checks.ts`.

Sửa nhỏ, tương thích ngược, ở hạ tầng cũ: `RealFixture.workRoots?`, `killClaudeUnder`/`claudeRunningUnder` nhận nhiều thư mục, `guardedRepoPaths` đọc `ORCA_E2E_GUARD_EXCLUDE`, `AgentSessionMonitor` nhận `MonitorTargets` (mặc định là calc sandbox), cổng Phase kiểm "Start" bị khoá trước khi duyệt (áp cho cả spec cũ).

## 4. Cái không thể "thật"

Một số ca của bộ mock chỉ tồn tại nhờ **tiêm lỗi** vào giao diện (gateway cũ thiếu kênh, luồng bị server đóng, host rớt giữa lúc duyệt thư mục, hai host cùng kết nối, repo cũ không có host...). Trên stack thật, backend hiện chặn tạo repo không host (`ORCA_ALLOW_LOCAL_REPOS` tắt), chỉ có một dev server kết nối, và không cách nào ép gateway trả lỗi mà không động vào hạ tầng. Chúng **không** được giữ dưới dạng "mock Playwright": được liệt kê từng ca trong [COVERAGE-request-web-mock-to-real.md](../COVERAGE-request-web-mock-to-real.md) với lý do, và hành vi đó vẫn được vitest cấp component bao phủ ở nơi đã có. Không khẳng định "đã phủ bằng e2e thật" cho ca nào chưa chạy thật.

## 5. Rủi ro còn lại (nêu thẳng)

- B4: agent vẫn chạy `--dangerously-skip-permissions`. Khối push, dấu vân tay và watchdog **phát hiện và dừng**, không **ngăn** mọi ghi ngoài worktree. Muốn ngăn thật cần sandbox cho agent (quyết định của người dùng).
- Claude thật không tất định: các nhánh "AI có hỏi ngược / có phương án thứ hai" được ghi `not-exercised` nếu AI không đưa ra, không giả vờ đã phủ.
- Mỗi lượt đầy đủ tốn lượt dùng Claude (khoảng 12 lượt gọi AI cho cả bộ) và ~45 phút.

## 6. Cách chạy

```bash
ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek          # cả bộ
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-ui                                                                                     # một phần
node --test tests/stages/e2e/orca-flows/ui-live/live-main-checkout-rules.test.ts                                                                          # quy tắc an toàn, không cần stack
```

Đường dẫn repo mặc định `/Users/datbeohbbh/Desktop/stuff/agentic/deepseek-harness`, đổi bằng `ORCA_E2E_DEEPSEEK_REPO`.
