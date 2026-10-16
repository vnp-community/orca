# Lỗi QA 2026-10-16: terminal, bug/task native, nguồn task GitHub và Jira, log `unrecognized_model`

> **Trạng thái (2026-10-16):** đã sửa QA-01, 02, 04, 05, 06; QA-03 (GitHub) tạm hoãn theo yêu cầu. Acceptance `qa`: 17 pass, 2 fail (cả hai GitHub); UI native bug/task 3/3; terminal UI xanh; chạy thật `--real native-execute` 2/2 (bug 187 giây, task 158 giây, 0 vi phạm guard); Jira thật OK. Chưa commit.

## 1. Bảng lỗi

| Lỗi | Tóm tắt | Nguyên nhân gốc | Sửa | Test (đỏ hôm nay) |
|---|---|---|---|---|
| [QA-01](./BUG-QA-01-terminal-opens-in-home.md) | Terminal worktree luôn mở ở `$HOME` | Transport web làm rơi `cwd`; gateway bỏ qua `worktree`; agent im lặng lùi về `$HOME` | [SOL-QA-01](./solutions/SOL-QA-01-terminal-cwd.md) | QA-TERM-02, QA-TERM-03, QA-UI-TERM-01 |
| [QA-02](./BUG-QA-02-bug-task-native-flow-not-covered.md) | Chưa có e2e bug/task native; **bug đang chạy OpenSpec** dưới mặc định triển khai | `EffectiveEngine` cho profile `light` theo `REQUEST_DEFAULT_ENGINE=openspec` | [SOL-QA-02](./solutions/SOL-QA-02-bug-task-native-e2e.md) | QA-ENG-01, ca 3 của `qa-bug-task-native-flow` |
| [QA-03](./BUG-QA-03-github-task-source-unavailable.md) | Nguồn GitHub ở tab Tasks không kết nối | 4 lớp: tiền tố `id:` ở gateway; điều kiện `gh` và nhãn host ở frontend; không có credential và không có lỗi `auth_required`; preflight treo | [SOL-QA-03](./solutions/SOL-QA-03-github-task-source.md) | QA-GH-01, QA-GH-03, QA-UI-GH-01 |
| [QA-04](./BUG-QA-04-jira-servicehub-task-source.md) | Cấu hình Jira `jr.servicehub.vn` bằng email + PAT trong `.env` | Có email → Basic (Server/DC chỉ nhận Bearer); dò API v2 hỏng nên rơi về v3; không đường cấu hình từ `.env` | [SOL-QA-04](./solutions/SOL-QA-04-jira-servicehub.md) | QA-JIRA-01, QA-JIRA-03 (02 bị chặn), QA-UI-JIRA-01, QA-JIRA-LIVE-01 (cờ) |
| [QA-05](./BUG-QA-05-unrecognized-model-log-shown-as-error.md) | Log đỏ `unrecognized_model sonnet-5-5` ở Execution của #1252 | CLI 2.1.266 chưa biết mô hình; giao diện coi mọi stderr là lỗi | [SOL-QA-05](./solutions/SOL-QA-05-agent-diagnostic-lines.md) | `agent-activity-diagnostics.test.ts` (`it.fails`), QA-UI-LOG-01 |
| [QA-06](./BUG-QA-06-task-list-approval-labelled-plan.md) | (thêm) `task` hiện "Approve plan" thay "Approve task list" | So sánh `subjectType` đã bị gộp thay vì `rawSubjectType` | trong chính tài liệu bug | ca `task` của `qa-bug-task-native-flow` |

Chi tiết từng lỗi (tái hiện, bằng chứng, file và dòng) nằm trong file BUG tương ứng. Checklist việc: [tasks/README.md](./tasks/README.md) (23 task sửa, 6 task test). Checklist QA: [qa/QA-CHECKLIST.md](./qa/QA-CHECKLIST.md).

## 2. Kết quả chạy test trên stack local (2026-10-16, trước khi sửa)

| Bộ | Lệnh | Kết quả |
|---|---|---|
| Acceptance Python (suite mới `qa`) | `python3 tests/stages/e2e/orca-flows/run_acceptance.py qa` | 11 xanh (7 DEV + 4 QA), **7 đỏ** (TERM-02, TERM-03, ENG-01, GH-01, GH-03, JIRA-01, JIRA-03), 1 bị chặn (JIRA-02 chờ JIRA-01), 1 bỏ qua (JIRA-LIVE-01, cờ). Xanh làm đối chứng: TERM-01, ENG-02, ENG-03, GH-02 |
| UI live `qa-regressions` | `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live qa-regressions` | QA-UI-TERM-01 **đỏ** ("opened in /Users/<user>"), QA-UI-GH-01 **đỏ** (dải "Some GitHub source hosts unavailable: 4"), QA-UI-JIRA-01 bỏ qua (cờ) |
| UI live `qa-bug-task-native-flow` (AI giả) | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- qa-bug-task-native-flow` | ca bug (engine native tường minh) **xanh**; ca task **đỏ** (nhãn "Approve plan", BUG-QA-06); ca "mặc định triển khai" **đỏ** (bug báo `openspec`, BUG-QA-02) |
| Vitest | `src/renderer/src/components/task/agent-activity` | xanh: hai ca `it.fails` ghim hành vi lỗi + một ca đối chứng |
| Không chạy | `run_ui_flow.py --real native-execute` (tốn AI), QA-JIRA-LIVE-01 và QA-UI-JIRA-01 (cần PAT thật trong `.env`) | chờ người có `.env`/ngân sách chạy |

## 3. Bằng chứng nổi bật (lấy từ stack thật)

- Terminal: khung WS `terminal.create` của giao diện không có `cwd`; 12/12 dòng gần nhất của `infra.terminal_sessions` có `cwd = /Users/<user>`; gọi thẳng với `cwd` đúng thì mở đúng.
- Engine: `request.list` trả #1240 (bug) `engine = openspec`, #1245 (task) `native`.
- GitHub: `github.listWorkItems` cho mọi repo → `WORKTREE_REPO_NOT_FOUND`; log git-gateway cho thấy `GetRepo` nhận `repo_id = "id:<uuid>"`; `which gh` rỗng; `github.rateLimit` → `SCM_CREDENTIAL_NOT_FOUND`.
- Jira: `GET https://jr.servicehub.vn/rest/api/2/serverInfo` không thông tin xác thực trả `401` (Server/Data Center); máy chủ Jira giả chỉ nhận Bearer cho thấy adapter gửi Basic khi có email; `jira.status` → `connected:false`.
- Log: dòng `! [claude-code:unrecognized_model] {"model":"claude-sonnet-5-5","query_source":"sdk"}` sau `■ Done · 6 turns`.

## 4. Cách chạy

```bash
# Acceptance (không dùng AI): dev server của máy này phải kết nối
python3 tests/stages/e2e/orca-flows/run_acceptance.py devserver qa
# UI live, không AI
ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live qa-regressions
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- qa-bug-task-native-flow
# Thực thi thật trên repo tạm (tốn AI), native bug + task, kèm kiểm log chẩn đoán
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real native-execute
# Jira thật (người có .env): token chỉ đọc từ .env, không in
ORCA_E2E_JIRA_LIVE=1 python3 tests/stages/e2e/orca-flows/run_acceptance.py devserver qa
```

Lưu ý khi chạy `--real`: bộ test chặn mọi thay đổi trong checkout đang bảo vệ (repo này); không sửa file khi nó chạy. `--ai-stub` hiện chỉ chạy được với dự án đặt `engine=native, solutionAgent=false` vì `.env` bật `REQUEST_DEFAULT_SOLUTION_AGENT=true`; các ca ở đây tự đặt như vậy và khôi phục.

## 5. Thứ tự sửa và rủi ro

Xem cuối [tasks/README.md](./tasks/README.md). Hai sửa nhỏ nhất, hiệu quả lớn nhất: T-QA-01-01 (gửi `cwd`) và T-QA-03-01 (cắt tiền tố `id:`). Cần quyết định sản phẩm trước khi sửa QA-02 (T-QA-02-01). QA-04 cần người có PAT chạy bản thật; mọi thứ khác kiểm được bằng máy chủ Jira giả.

## 6. Bảo mật

- Không có bí mật nào trong tài liệu hay test: PAT, email và URL chỉ được đọc từ `.env` lúc chạy và không in (kể cả khi lỗi, giá trị bị thay bằng `***`).
- `.env` gốc đã được gitignore; không thêm vào commit.
