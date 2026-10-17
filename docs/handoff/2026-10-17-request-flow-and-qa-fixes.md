# Handoff 2026-10-17: Request flow (CR-REQ-037/038) và sửa lỗi QA 2026-10-16

Dành cho agent hoặc người nhận việc tiếp. Đọc hết mục 1 đến 3 trước khi sửa bất cứ gì. Hướng dẫn triển khai nằm ở
[`deploy/dev/RELEASE-2026-10-DEPLOY.md`](../../deploy/dev/RELEASE-2026-10-DEPLOY.md); tài liệu này tập trung vào **trạng thái, bản đồ mã, bẫy và việc còn lại**.

## 1. Trạng thái trong một màn hình

- Repo `/Users/datbeohbbh/Desktop/work/vnpay/agentic-app/orca`, nhánh `feat/e2e-acceptance-fixes`. **Chưa push.**
- Bốn commit của đợt này (mới nhất trước):

| Commit | Nội dung |
|---|---|
| `cdf8fc008` | sửa lỗi QA 2026-10-16: QA-01, 02, 04, 05, 06 + bộ test + tài liệu QA |
| `c05d46425` | frontend CR-REQ-037: màn hình chi tiết Request mới |
| `09702be57` | backend CR-REQ-038: thực thi liên tục, activity, summary, migration 0098 |
| `46426bd95` | skills knowledge-hub trỏ vào repo này (không phải upstream) |

- **Xong và đã kiểm trên stack local:** thực thi liên tục, activity feed, execution summary, màn hình Request mới (`--real continuous` pass), QA-01, 02, 04, 05, 06
  (`--real native-execute` 2/2: bug 187 giây, task 158 giây, 0 vi phạm guard).
- **Chưa làm:** QA-03 (nguồn task GitHub, hoãn theo yêu cầu), chạy OpenSpec thật trên `vnp-qa-platform` (hoãn, tốn 7 đến 13 USD), bản dịch es/ja/ko/zh của 192 khoá i18n.
- **Việc của người khác trong working tree, không phải của đợt này, đừng commit chung:** `package.json` (script `test:e2e:workflow-web`), `tests/shared/orca_route_catalog.py` (khối Workflow v2),
  `tests/backend/`, `tests/client/`, `tests/e2e/`, `tests/playwright.workflow-web.config.ts`, binary `backend-go/cmd/orca-cli/orca`, và hai `tsconfig.tsbuildinfo`.
- Tài liệu QA: [`specs/frontend/bugs/qa-2026-10-16/`](../../specs/frontend/bugs/qa-2026-10-16/README.md). Trạng thái từng bug đã cập nhật; **`qa/QA-CHECKLIST.md` và `tasks/README.md` chưa được tick** theo kết quả.

## 2. Quy ước của dự án mà agent hay vi phạm

Nguồn: `CLAUDE.md`, `AGENTS.md`. Những điều bị chặn thật bởi hook hoặc CI:

- **Không** thêm `max-lines` disable, cũng không thêm vào `config/max-lines-baseline.txt` (chỉ được co lại). File quá 300 dòng mã (không tính dòng trống và chú thích) thì **tách file**.
  Hook pre-commit lint file đã stage, nên chạm vào file đang vượt ngưỡng sẽ bị chặn: `pty-agent-bridge.ts` đã bị như vậy và đã tách thành `pty-agent-state.ts`, `pty-agent-cwd.ts`, `pty-agent-signals.ts`.
- Không đặt tên file `helpers`, `utils`, `common`, `misc`. Không dùng chú thích kiểu `import('mod').Type` (oxlint `consistent-type-imports`).
- Chú thích chỉ ghi **vì sao**, một đến hai dòng.
- Nhãn giao diện bằng **tiếng Anh**; thêm khoá i18n cho cả 5 locale (en, es, ja, ko, zh). Dùng token trong `frontend/src/renderer/src/assets/main.css` và `guides/STYLEGUIDE.md` (AGENTS.md ghi `docs/STYLEGUIDE.md`, đường dẫn đó không tồn tại trong checkout này).
- Mã chạy trên macOS, Linux, Windows và qua SSH: không giả định đường dẫn `/`, không `metaKey` cứng.
- Lệnh git có phiên bản tối thiểu 2.25 (`guides/reference/git-compatibility.md`; AGENTS.md ghi đường dẫn `docs/reference/...` không có); ngoài GitHub còn GitLab (đừng đặt tên chỉ-GitHub cho khái niệm chung).
- CLAUDE.md yêu cầu chạy `gitnexus_impact` trước khi sửa symbol và `detect_changes` trước commit. **Công cụ GitNexus không có trong phiên làm việc vừa rồi nên hai bước này chưa được chạy**; nếu bạn có công cụ, hãy chạy `detect_changes({scope:"compare", base_ref:"main"})` trên nhánh này.
- Không commit hoặc push khi chưa được yêu cầu. Không in bí mật (token, PAT) vào đâu cả; nhắc chúng bằng tên biến.
- Người dùng nói tiếng Việt; báo cáo bằng tiếng Việt, số liệu cụ thể.

## 3. Những điều phải biết trước khi chạy e2e thật

1. **Quiet-checkout:** `run_ui_flow.py --real ...` đợi checkout orca yên 90 giây rồi coi **mọi** thay đổi (tài liệu, locale, build) là rò rỉ. Chuẩn bị sửa ở scratchpad, áp dụng sau khi chạy xong.
2. **Repo tạm không có `.tdd-agent/config.toml`:** mặc định deploy (compose, `.env.example`, `.env`) là `REQUEST_DEFAULT_ENGINE=openspec` và `REQUEST_DEFAULT_SOLUTION_AGENT=true`, nên Plan lỗi `KNOWLEDGE_CONFIG_MISSING`
   (và `--ai-stub` lỗi `KNOWLEDGE_HUB_UNSUPPORTED`). Với fixture, đặt dự án `engine=native`, `solutionAgent=false` bằng `request.engineSettingsSet`, rồi trả lại. Helper: `configureProject` trong `ui-live/live-continuous-execution-steps.ts`.
3. **`--ai-stub` không spawn được PTY.** Spec terminal (`QA-UI-TERM-01`) phải chạy với agent thật: `run_ui_flow.py -- qa-regressions -g TERM-01`.
4. **Sau `redeploy-local.sh`** restart theo thứ tự `infra-fleet → git-gateway → task → request → api-gateway`; api-gateway có thể crash-loop một phút (xem deploy guide mục 2).
5. **pty-daemon sống qua việc restart agent.** Sửa `pty-agent-bridge.ts` mà không thay daemon thì không có tác dụng; đã mất một vòng debug vì điều này (xem deploy guide mục 5).
   Đường PTY chế độ agent là `pty-agent-bridge.ts` (chạy trong daemon), **không phải** `pty-handler.ts` (relay/SSH).
6. **Token CLI `orca login` sống khoảng 10 phút** và không gia hạn; `plan generate` dài sẽ chết với `UNAUTHENTICATED` trong khi run vẫn chạy. Đăng nhập lại và poll `orca plan proposal <id>`.
7. **`AGENT_WORK_DIR`** của agent phải chứa các worktree anh em `<repo>-<branch>`, nếu không ghi file lỗi "Path outside project root".
8. **Trước một lượt thật tốn tiền** (một Plan 7 đến 13 USD, 13 đến 25 phút), chạy `tests/stages/e2e/orca-flows/workspace_probe.py` trên worktree thử; fake đã lọt sáu lỗi chỉ lộ khi chạy thật.
9. **Browser pane** phải mở bằng `preview_start` (url) trước khi chụp màn hình.
10. **zsh:** glob có `--include=*.ts` không khớp thì lệnh lỗi; đặt trong dấu nháy. `sed -i` trên macOS cần `-i ''`.
11. Stack local chỉ có một người dùng: cổng duyệt Solution/Plan cỡ L cần chính sách tự duyệt theo dự án. Người dùng chỉ cho phép ở dự án pilot; **không nới chính sách duyệt khi chưa hỏi**.
12. Bí mật Jira trong `.env` (`JR_URL`, `JR_EMAIL`, `JR_PAT_TOKEN`) được phép dùng **chỉ để test API nội bộ**; luồng giao diện do người dùng tự nhập tay. Script kiểm ngắt kết nối sau khi chạy.

## 4. Bản đồ mã theo tính năng

### 4.1 Thực thi liên tục và nhật ký (CR-REQ-038, backend)

| Việc | Vị trí |
|---|---|
| Chế độ thực thi, engine mặc định, `FromDeploymentDefault` | `backend-go/services/request-service/internal/domain/engine_settings.go`, `internal/usecase/engine_settings_defaults.go` |
| Profile OpenSpec của từng loại | `internal/domain/openspec_profile.go` |
| Bắt đầu thực thi, chạy Phase kế tiếp, kiểm quyền người duyệt | usecase `StartExecution`, `EvaluateExecution` (`completePhases`), `StartPhase`. Blast radius cao: chạy `gitnexus_impact` trước khi sửa |
| Activity (UNION ALL, phân trang keyset, hai dialect) | `internal/adapter/postgres/request_activity.go`, `internal/adapter/mysql/request_activity.go`, proto `backend-go/proto/orca/request/v1/activity.proto` |
| Migration | `services/request-service/migrations/{postgres,mysql}/0098_execution_mode.*` |
| Kênh gateway, MCP, OPA | kênh `request.activity`, `request.executionSummary`; MCP `request_activity`, `request_executionSummary`; OPA `agent_rpcs` |
| Hợp đồng | `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` mục 2.10 |
| Task checklist | `specs/backend-go/crs/v6/plan-phase-task/tasks/TASK-REQ-038-0x-*.md` (8/8 xong) |

Lưu ý thiết kế đã chốt: `StartPrecheck` được gắn sau `wireApproval` nên là tuỳ chọn trong kiểm tra khởi động; liên tục dừng ở drift, backlog, `awaiting_information`, task hết lượt thử; chế độ ghim vào Request khi vào `executing`.

### 4.2 Màn hình Request (CR-REQ-037, frontend)

Thư mục `frontend/src/renderer/src/components/request/`:

| Việc | Vị trí |
|---|---|
| Dòng thời gian theo luồng từng loại | `RequestStageTimeline.tsx`, `request-stage-timeline-model.ts` |
| Thanh việc tiếp theo | `NextActionBar.tsx`, `request-action-rules.ts` |
| Tab Execution | `execution/` (`ExecutionTab.tsx`, `ExecutionSummaryStrip.tsx`, `ExecutionNeedsYou.tsx`, `ExecutionFailureCard.tsx`, `HandoverRow.tsx`) |
| Activity feed | `activity/` (`RequestActivityTab.tsx`, `activity-entry-presentation.ts`) |
| Plan một thẻ duyệt | `plan/` (`PlanApprovalBar.tsx`, `PlanApprovalCard.tsx`, `plan-approval-model.ts`) |
| Hook | `hooks/useRequestFlow.ts`, `useExecutionSummary.ts`, `useRequestActivity.ts` |
| Parser dây | `frontend/src/shared/request-wire-parsers.ts` (đã tách scalar và Solution parser ra file riêng để dưới ngưỡng dòng) |
| Hợp đồng spec FE | `specs/frontend/crs/v6/request-frontend/` (tasks `FE-REQ-TASK-037-*`) |

Bẫy hay gặp: parser dây **gấp** `task_list` thành `plan`, `findings`/`answer` thành `solution`; loại gốc nằm ở `rawSubjectType`. Mọi chỗ cần phân biệt (nhãn, quick approve) phải đọc `rawSubjectType`
(`subjectKindOf` trong `approval/approval-inbox-rules.ts`). Dòng duyệt của `task_list` trong hộp thư hiện nhãn **"Task list"**, không phải "Plan".

### 4.3 Sửa lỗi QA 2026-10-16

| Bug | Sửa ở đâu | Test chứng minh |
|---|---|---|
| QA-01 terminal | FE `components/terminal-pane/remote-runtime-pty-transport.ts` (+ `shared/runtime-types.ts`, khoá i18n `auto.components.terminal.cwdFellBack`); gateway `api-gateway/.../wscompat/channels_terminal.go` (`terminalCwd`); infra-fleet `usecase/spawn_terminal_session.go`, `domain/terminal_session.go`, `adapter/devserveragent/methods.go`, proto `infrafleet.proto`; agent `pty-agent-bridge.ts`, `pty-agent-cwd.ts` | `QA-TERM-01..03`, `QA-UI-TERM-01`, `agent/src/relay/pty-cwd.test.ts`, `remote-runtime-pty-transport.test.ts`, `channels_terminal_test.go` |
| QA-02 engine | `request-service` `engine_settings.go`, `engine_settings_defaults.go` | `QA-ENG-01..03`, `qa-bug-task-native-flow`, `--real native-execute`, `engine_settings_test.go` |
| QA-04 Jira | `issue-tracking-service/internal/adapter/jira/client.go` (`resolveSiteAuth`, cache `siteAuth` theo (site, băm token)), `usecase/connect.go`; FE `jira-connect-dialog.tsx` (+ hai khoá i18n đổi nội dung) | `QA-JIRA-01..03`, `QA-JIRA-LIVE-01`, `client_test.go` |
| QA-05 log | FE `components/task/agent-activity/agent-activity-lines.ts` (loại dòng `diagnostic`), `TaskAgentActivityPanel.tsx` | `agent-activity-diagnostics.test.ts`, `qa-native-execute-real.web.e2e.ts` |
| QA-06 nhãn | FE `plan/PlanApprovalBar.tsx`, `PlanApprovalCard.tsx` (`isTaskList` đọc `rawSubjectType`) | `PlanApprovalCard.test.tsx`, `plan-approval-bars.test.tsx` |

Chi tiết nguyên nhân gốc từng lỗi: `specs/frontend/bugs/qa-2026-10-16/BUG-QA-0x-*.md`; phương án: `solutions/SOL-QA-0x-*.md`.
Lưu ý: `SOL-QA-04` mô tả vài thứ **chưa được làm** (script `connect-jira-from-env.sh`, khối `JIRA_*` trong `.env.example`, `ISSUETRACKING_BOOTSTRAP_JIRA_*`, `jira.status.source`). Thứ thực sự có là `deploy/dev/scripts/check-jira-from-env.sh` + `tests/stages/e2e/orca-flows/jira_env_check.py`, chỉ để kiểm.

## 5. Bộ test và cách chạy

Tất cả từ thư mục gốc repo. Bộ acceptance nói chuyện với stack local qua WebSocket gateway.

| Mục đích | Lệnh |
|---|---|
| Danh sách suite | `python3 tests/stages/e2e/orca-flows/run_acceptance.py --list` |
| QA backend (đã có) | `python3 tests/stages/e2e/orca-flows/run_acceptance.py devserver qa` → 17 pass, 2 fail GitHub, 1 skip Jira live |
| Jira site thật | `ORCA_E2E_JIRA_LIVE=1 python3 tests/stages/e2e/orca-flows/run_acceptance.py qa` |
| UI native bug/task (stub) | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- qa-bug-task-native-flow` |
| UI terminal (agent thật) | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py -- qa-regressions -g TERM-01` |
| Thực thi native thật | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real native-execute` |
| Thực thi liên tục thật | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real continuous` |
| Playwright trực tiếp | `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live <tên spec>` |
| Kiểm Jira từ `.env` | `bash deploy/dev/scripts/check-jira-from-env.sh` |
| Go | `cd backend-go && go test ./services/request-service/... ./services/issue-tracking-service/... ./services/api-gateway/internal/adapter/wscompat/...` |
| Agent | `cd agent && npx vitest run src/relay/pty-agent-bridge.test.ts src/relay/pty-cwd.test.ts src/relay/pty-daemon-server.test.ts` |
| Frontend | `cd frontend && npx vitest run <đường dẫn test>` |

Tệp test mới của đợt này: `tests/stages/e2e/orca-flows/suite_qa_regressions.py` (đăng ký trong `run_acceptance.py` là suite `qa`, thuộc nhóm dùng AI), `ui-live/live-qa-steps.ts`,
`ui-live/qa-regressions.web.e2e.ts`, `ui-live/qa-bug-task-native-flow.web.e2e.ts`, `ui-live/qa-native-execute-real.web.e2e.ts`.
Bước dùng chung `rejectPlanApprovalAndCancel` (`ui-live/live-flow-steps.ts`) giờ chấp nhận cả "Plan" lẫn "Task list".

Lỗi có sẵn từ trước, **không do đợt này** (đừng mất thời gian): test `infra-fleet` về đường dẫn socket SSH identity-agent quá dài; vài test frontend; lỗi kiểu trong `pty-agent-bridge.test.ts` (`env` trên `{}`)
và trong `__tests__/agent-connection-relay.test.ts`; fixture `__fixtures__/quality-mini-repo` cố ý lỗi.

## 6. Việc còn lại, theo thứ tự đề xuất

1. **QA-03 GitHub** (P1, đang chờ). Làm theo [`SOL-QA-03`](../../specs/frontend/bugs/qa-2026-10-16/solutions/SOL-QA-03-github-task-source.md): (a) gateway `channels_scm.go` chuẩn hoá `repo` (`id:<uuid>`, `id:<uuid>::/path`, uuid trần) và nhận thêm `repoId`;
   (b) thêm loại lỗi `auth_required` cho thiếu credential; (c) FE `getRepoBackedProviderAvailability` không phụ thuộc `gh`, tên host lấy từ `devServer.list`, không in UUID; (d) preflight có hạn 5 giây.
   Test đỏ chờ sẵn là bằng chứng hoàn thành: `QA-GH-01`, `QA-GH-03`, `QA-UI-GH-01`. Người dùng nói: ưu tiên Jira trước, GitHub làm sau.
   Lưu ý: tên test trong `SOL-QA-03` (QA-GH-01/02/03) lệch với tên trong `suite_qa_regressions.py`; theo file suite.
2. **Tick `qa/QA-CHECKLIST.md` và `tasks/README.md`** theo kết quả ở mục 1 (hiện còn là bản nháp lúc chưa sửa).
3. **`.env.example`:** đã có `REQUEST_DEFAULT_*` (openspec, true, step_by_step); còn thiếu khối Jira (`JR_*`/`JIRA_*`, để trống).
4. **CLI:** `orca engine set` chưa có cờ `--execution-mode` (`backend-go/cmd/orca-cli/internal/command/engine_cmd.go`). Màn hình cài đặt engine/chế độ thực thi cũng chưa có.
5. **Rà bản dịch** es, ja, ko, zh cho 192 khoá CR-REQ-037 và khoá `auto.components.terminal.cwdFellBack`.
6. **OpenSpec thật:** chạy `--real` trên repo có `.tdd-agent/config.toml` (pilot `vnp-qa-platform`); chỉ làm khi người dùng đồng ý chi phí. Trước đó chạy `workspace_probe.py`.
7. **Tự động hoá thay agent/pty-daemon trên dev server từ xa** (`deploy/agent/scripts/deploy-agent.sh` chưa dừng daemon cũ). Cần xác nhận cách `orca-agent.service` quản lý tiến trình con trước khi sửa.
8. Chạy `detect_changes` của GitNexus (mục 2) và push/mở PR khi người dùng yêu cầu.

## 7. Dữ liệu và môi trường local (không có bí mật)

- Gateway `http://localhost:8081`, frontend `http://localhost:28080`. Admin lấy từ `BOOTSTRAP_ADMIN_*` trong `deploy/dev/.env`.
- Dev server mặc định `dev-local-mac`; log agent `/tmp/orca-local-agent.log`; chạy bằng `deploy/dev/scripts/start-local-agent.sh` (gỡ khoá API khỏi môi trường agent; AI đi qua Claude Code đã đăng nhập).
- Postgres trong container `orca-go-postgres`; mỗi service một database (`request`, `infra`, `auth`, ...). Truy vấn nhanh: `docker exec orca-go-postgres psql -U "$(docker exec orca-go-postgres printenv POSTGRES_USER)" -d request -c "..."`.
  Bảng terminal: `infra.terminal_sessions` (cột `cwd`). Bảng duyệt: `request.approvals`.
- Dự án pilot: `765ce554-ee6e-433c-b7a3-8bb0bf0f5c8c` (pilot-tdd-2026-10-14). Các Request e2e có tiền tố `e2e-ui-<ngày giờ>`; suite tự huỷ Request của nó, kiểm bằng dòng "open Requests ... 0 orphaned" cuối mỗi lượt.
- Bộ nhớ làm việc của phiên trước (ngoài repo): `~/.claude/projects/-Users-datbeohbbh-Desktop-work-vnpay-agentic-app-orca/memory/` (bẫy stack local, engine theo loại Request, bẫy e2e UI). Nội dung đã được tổng hợp ở mục 3.

## 8. Mẫu bắt đầu cho agent nhận việc

1. `git status` và `git log --oneline -6`; xác nhận bạn đang ở nhánh `feat/e2e-acceptance-fixes` và biết file nào không phải của mình (mục 1).
2. Đọc `specs/frontend/bugs/qa-2026-10-16/README.md` và bug bạn nhận.
3. Chạy test đỏ tương ứng để xác nhận nó đỏ **đúng lý do** trước khi sửa.
4. Sửa nhỏ nhất có thể; file chạm ngưỡng dòng thì tách. Chạy lại test đỏ, rồi `run_acceptance.py devserver qa` để chắc không làm hỏng mục khác.
5. Cập nhật trạng thái trong `BUG-QA-0x-*.md` và checklist; báo cáo bằng tiếng Việt kèm số liệu. Chỉ commit khi người dùng bảo.
