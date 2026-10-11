# TASK-FE-E2E-027: e2e live nghiệm thu "Add Project tạo project riêng, chặn trùng, Request chọn project, ô Answer" (Claude thật, qua UI)

**From Solution:** SOL-FE-E2E-005 (mục 5 "Live"), SOL-V2-003 · **Resolves:** nghiệm thu ba lỗi của lượt test Request #1136 (2026-10-11), sau TASK-FE-E2E-022 đến 026 và TASK-V2-036 đến 040
**Priority:** P1 · **Area:** test (không đổi mã sản phẩm)
**Files:**
- tests/stages/e2e/orca-flows/ui-live/add-project-own-project-request-real.web.e2e.ts (mới; spec) + module hỗ trợ mới: `add-project-two-repos-fixture.ts` (hai repo tạm alpha/bravo có hook chặn), `live-add-project-ledger.ts` (kiểm kê + dọn đúng dữ liệu của lượt chạy), `live-add-project-own-project-steps.ts` (bước UI), `live-add-project-own-project-checks.ts` (khẳng định), `live-add-project-request-about-bravo-steps.ts` (nửa Request), `live-user-data-snapshot.ts` (chụp dữ liệu thật của người dùng), `live-answer-snippet.ts` + `live-answer-snippet.test.ts` (đoạn câu trả lời so với màn hình; `node --test`)
- sửa nhỏ, tương thích ngược: `live-flow-harness.ts` (`provision?`), `real-flow-harness.ts` (`createFixture?`, `provision?`, `afterCleanup?`), `live-flow-steps.ts` (`create?`), `live-project-fixture.ts` (tách `cancelOpenRequests`, `deleteProjectWhenIdle`, `connectedDevServer`), `real-repo-fixture.ts` (tách `runToolGuardSelfTest`, `guardLogLines`, `fixtureGit`), `live-add-project-repro-steps.ts` (`browsableTempBase`; dọn xoá cả project riêng), `add-project-worktree-repro.web.e2e.ts` (thêm assert tên project = tên thư mục), `live-console-audit.ts` (3 luật allowlist có lý do), `run_ui_flow.py` (`--real addproject`, vào `--real all`), `orca-flows/README.md`
- tests/stages/e2e/legacy/request-web/add-project-own-project.web.e2e.ts (thêm ca `f`: đường dẫn gõ có `/` cuối bị chặn, mock)
**Depends on:** TASK-FE-E2E-022 đến 026, TASK-V2-036 đến 040 (đã redeploy: project-service migration 0032, api-gateway, frontend)
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

---

## Việc cần làm

- [x] Spec live đi qua UI với Claude Code thật, repo tạm `/private/tmp/e2e-ui-addproj-<mmddhhmmss>-alpha|bravo` (mỗi repo: README nhận biết, 2 hàm, test, hook chặn `.claude/tool-guard.mjs`); KHÔNG đụng project `codex`, Request #1136/#1137, repo codex, 'Demo (this Mac)', worktrees `codex-test-*`.
- [x] Lỗi 2: Add Project lần lượt alpha rồi bravo (host dev-local-mac, Browse folder, Select folder, ô `add-project-name` mặc định = tên thư mục, Add Git Project) => mỗi lần đúng 1 project MỚI chứa đúng 1 repo, `devServerId` = dev-local-mac, tên = tên thư mục; không project cũ nào có thêm repo; UI gửi 1 `project.create {name, devServerId, repoPath}` và không `repo.add`; sidebar có hai project riêng; không đi qua Project Workspace (Beta).
- [x] Lỗi 3: Add Project alpha lần hai => `add-project-duplicate` nêu tên project đang chứa, `add-project-open-existing` mở đúng project alpha (worktree đang chọn thuộc repo alpha), không RPC ghi nào, số project/repo không đổi; đường dẫn có `/` cuối vẫn bị chặn; backend `project.create {devServerId, repoPath=alpha}` => `PROJECT_REPO_ALREADY_REGISTERED ... already registered in project "<tên>" (<id>)`, không tạo gì.
- [x] Lỗi 2 (Request): Create request chọn project qua bộ chọn; `create-request-repo-note` theo project đang chọn (alpha rồi bravo: đúng repo, đường dẫn, host); Request thuộc project bravo; Claude thật phân loại, xác nhận Question/S, trả lời Clarification trên UI.
- [x] Lỗi 1: tab Analysis hiện `answer-view` với `answer_markdown` (không "No data"), một đoạn của câu trả lời trong `solution.list` có trên màn hình, `answer-confidence` `NN%`, nhắc tên hàm của bravo, `answer-citations` trỏ file bravo, không dấu vết alpha. Không bấm Approve; huỷ Request qua UI.
- [x] Chạy trên repo đúng: log hook của bravo 7 đến 8 lệnh gọi toàn `Read/Glob/Grep`, 0 từ chối; log hook của alpha rỗng; cả hai repo cây sạch, HEAD/nhánh/worktree không đổi.
- [x] An toàn/dọn dẹp trong `finally`: huỷ Request, `repo.rm` + `project.delete` chỉ project/repo mang tên/đường dẫn của lượt chạy (project khác lỡ chứa repo của lượt chạy chỉ bị gỡ repo), xoá thư mục; kiểm 0 project/repo `e2e-ui-addproj-*` còn lại; `report_orphan_requests.py` 0 orphaned; dấu vân tay dữ liệu thật trước/sau bằng nhau; watchdog + quiet-tree như `real-flow-harness`; console audit (mục ngoài allowlist làm đỏ).
- [x] Hồi quy: `--repro add-project-worktree` cập nhật (dọn xoá cả project riêng `e2e-ui-repro-*`; thêm assert tên project = tên thư mục) và XANH; `--real readonly` 2/2 XANH.
- [x] Spec Playwright mock: khoảng trống duy nhất so với `add-project-own-project.web.e2e.ts` và `request-answer-and-project-note.web.e2e.ts` là đường dẫn có `/` cuối ở hộp thoại; thêm ca `f` (6/6 xanh). Không nhân đôi phần còn lại.
- [ ] Chạy `--real all` đầy đủ: không thuộc phạm vi tác vụ này (người điều phối chạy).

## Tiêu chí hoàn thành

- `run_ui_flow.py --real addproject` XANH trên stack đã redeploy; `--repro add-project-worktree` và `--real readonly` không hồi quy; dữ liệu thật không đổi; 0 orphaned.

## Rủi ro · Ước lượng

LOW (chỉ thêm test; sửa nhỏ có tham số tuỳ chọn ở harness cũ, đã chạy lại `--real readonly` và `--repro`). 1 ngày.

## Kết quả thực thi

**Lệnh và kết quả thật (2026-10-11, stack local, dev server `dev-local-mac` Claude Code thật; `ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/'`)**

| Lệnh | Kết quả | Thời lượng |
|---|---|---|
| `ORCA_E2E_ADDPROJECT_STOP=projects ... run_ui_flow.py --real addproject` (nửa không AI, 2 lượt: lần đầu ĐỎ vì tap WebSocket đăng ký muộn + console có 3 mục lạ, đã sửa/ghi allowlist; lần hai xanh) | 1 passed | 22 giây |
| `run_ui_flow.py --real addproject` (đặt `ORCA_E2E_QUIET_SECONDS=5` khi phát triển) | 1 passed, `0 orphaned` | 2 phút (118 giây trong test) |
| `run_ui_flow.py --real addproject` (cấu hình chính thức, quiet 90 giây) | 1 passed, 0 vấn đề an toàn, `0 orphaned` | 3 phút 19 (197 giây trong test; Claude thật: 1 phân loại + 1 phân tích chỉ đọc khoảng 65 giây) |
| `run_ui_flow.py --repro add-project-worktree` | 1 passed (worktree `test-02` tạo được; project `e2e-ui-repro-*` do Add Project tạo, gắn dev-local-mac, đã xoá cùng repo) | 9 giây |
| `run_ui_flow.py --real readonly` | 2 passed (question 2.8 phút, bug 2.7 phút), 0 vấn đề an toàn, `0 orphaned` | 5 phút 27 |
| `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web .../add-project-own-project.web.e2e.ts` | 6 passed | 21 giây |
| `node --test .../live-answer-snippet.test.ts` | 3 pass | dưới 1 giây |
| tsc ad hoc (strict) trên 3 spec ui-live | sạch | |

Tổng số lượt Claude thật: 2 lượt đầy đủ của spec mới (mỗi lượt 1 phân loại + 1 phân tích chỉ đọc) + 2 lượt `--real readonly` (4 lượt gọi AI mỗi cặp). `oxlint` không lint thư mục `tests` (không có tệp để kiểm); `oxfmt --list-different` sạch.

**Chứng cứ của lượt chính thức** (log `[addproject]`): `active worktree after "Open that project": <repoId alpha>::/private/tmp/e2e-ui-addproj-1011002528-alpha`; ghi chú Create request: alpha "The request runs on repo e2e-ui-addproj-...-alpha (/private/tmp/...-alpha) on dev-local-mac." rồi bravo tương tự; `answer: stored 2638 chars, snippet checked on screen "Import dùng đường dẫn from src.bravo_mat", Confidence: 93%, citations: src/bravo_math.py:1 ... | tests/test_bravo_math.py:3 ...`; `bravo guard calls 8`; dữ liệu thật TRƯỚC = SAU: 7 project ("codex", 4x "probe-proj", "Demo (this Mac)", "My Repos"), 7 Request đã kết thúc gồm `#1136 completed v10 2026-10-10T23:32:50Z`, repo codex `HEAD c11ed24c2a, 4 worktree(s), 29 changed/untracked file(s)`, demo-repo `HEAD c1944b3ae1`. Sau lượt chạy: `git -C .../codex status --porcelain` chỉ còn `?? docs/multi-provider-llm-layer.md`, `?? examples/` (có sẵn), `worktree list` 4 mục như cũ; `/private/tmp` không còn `e2e-ui-*`; agent dev-server vẫn là Claude thật (cùng hai tiến trình, không stub). Ảnh từng bước ở `test-results/` (đã xoá khỏi gốc theo yêu cầu; sinh lại khi chạy).

**Thiết kế đáng nhớ**
- Không dùng project dựng sẵn qua gateway như các spec cũ: `runLiveFlow` nhận `provision` để trả "placeholder" project; project thật do UI tạo, spec điền `id/name` sau. `afterCleanup` của `runRealFlow` gom vấn đề dọn dẹp, thư mục tạm còn sót và khác biệt dữ liệu người dùng vào cùng phán quyết an toàn.
- Dữ liệu thật: mọi repo đã đăng ký còn trên đĩa được thêm vào `ORCA_E2E_GUARD_PATHS` trước khi chạy, nên cơ chế quiet-tree, watchdog 15 giây và so sánh trước/sau của `live-safety-invariants` bảo vệ cả repo codex.
- Tap WebSocket (`tapSentRpc`) phải đăng ký trước đăng nhập (socket mở lúc đăng nhập); đăng ký trong thân flow thì không thấy frame nào.
- Cố ý có một lệnh ghi qua gateway: `project.create` trùng alpha để kiểm hàng rào backend (kênh tắt, dữ liệu tạm; bị từ chối nên không tạo gì). Mọi bước sản phẩm khác đi qua UI.

## Lỗi tìm thấy

Không có lỗi trong ba lỗi đã sửa: cả ba hành vi đúng trên stack đã redeploy. Phát hiện phụ (có từ trước, không làm đỏ; ba mục đầu đã vào allowlist console với lý do trong `live-console-audit.ts`):

1. **[backend/agent, thật]** `project.scanNested` luôn lỗi: project-service gọi `fs.scanNestedRepos` (`backend-go/services/project-service/internal/usecase/scan_nested.go:45`) nhưng agent dev server KHÔNG có method này (`agent/src` không có handler; log `orca-go-infra-fleet`: `dev server agent does not implement the requested method: Method not found: fs.scanNestedRepos`; `orca-go-project`: `PROJECT_SCAN_NESTED_FAILED`). Mỗi lần Add Project một repo git, console UI có `Failed to scan nested repos`; nhập hàng loạt repo lồng nhau không bao giờ chạy được. Cần người điều phối giao agent/backend.
2. **[backend, đã biết]** kênh `orcaProfiles.findProjectProfiles` chưa có ở gateway (`... is not yet implemented in backend-go`), cảnh báo console mỗi lần Add Project.
3. **[frontend, nhỏ]** hộp thoại Add Project: Radix cảnh báo "Missing `Description` or `aria-describedby={undefined}` for {DialogContent}" mỗi lần mở (`frontend/src/renderer/src/components/sidebar/AddRepoDialogChrome.tsx:23`). Không sửa trong đợt này (không liên quan ba lỗi; cần redeploy frontend mới kiểm được).
4. **[frontend, UX thấp]** câu trả lời của AI có bảng markdown (`| Hàm | Chức năng |`) hiện dưới dạng chữ có dấu `|` thô trong ô Answer, vì `RequestMarkdownContent.tsx` dùng `react-markdown` không có `remark-gfm`. Nội dung đọc được, không phải lỗi "No data". Ảnh `12-answer-on-screen.png` của lượt chạy.

Kèm một quan sát: bốn project `probe-proj` cũ (host stub-in đã ngắt) làm sidebar thử tải worktree nhiều lần và ghi lỗi `GITGATEWAY_DEV_SERVER_UNREACHABLE` ra console (đã trong allowlist từ trước, không đổi).

## Chạy lại

```bash
ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real addproject
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --repro add-project-worktree
ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real readonly
```

Cần cây yên tĩnh (spec đợi 90 giây không ai sửa repo orca, codex, demo-repo) và stack đã có bản sửa (project-service migration 0032, api-gateway, frontend).
