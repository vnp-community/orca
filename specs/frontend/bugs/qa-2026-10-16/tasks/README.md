# Checklist: sửa 5 lỗi QA 2026-10-16

Ký hiệu: `[x]` xong, `[~]` một phần, `[ ]` chưa làm, `[-]` không áp dụng. "Test" là test đã viết sẵn (đỏ khi lỗi còn, xanh sau khi sửa). Ngày soạn: 2026-10-16. **Chưa có bản sửa nào được áp dụng**; các test đã viết và đã chạy trên stack local để xác nhận chúng đỏ đúng lý do.

Tổng quan: 6 lỗi (5 do QA báo, 1 phát hiện thêm), 23 task sửa, 6 task test (đã viết) = 29 dòng.

## BUG-QA-01: terminal mở ở `$HOME` ([SOL-QA-01](../solutions/SOL-QA-01-terminal-cwd.md))

| ID | Việc | Nơi sửa | Test cho xanh | Trạng thái |
|---|---|---|---|---|
| T-QA-01-01 | `createRemoteRuntimePtyTransport` lấy `cwd` từ `opts` và gửi trong `terminal.create` | `frontend/.../terminal-pane/remote-runtime-pty-transport.ts` (+ test cùng thư mục) | QA-UI-TERM-01, QA-TERM-02 (sau B) | [ ] |
| T-QA-01-02 | Gateway: `worktree` (`id:<repoId>::<path>`, `id:<worktreeId>`) → `cwd` khi thiếu `cwd`; `cwd` có sẵn thắng | `api-gateway/.../channels_terminal.go` (+ `_test.go`) | QA-TERM-02 | [ ] |
| T-QA-01-03 | Agent/infra/gateway trả `cwdFellBack`/`cwdRequested`; lỗi `INFRA_TERMINAL_CWD_NOT_FOUND` khi `strictCwd` | `agent/src/relay/pty-handler.ts`, `infra-fleet-service/.../methods.go`, `channels_terminal.go` | QA-TERM-03 | [ ] |
| T-QA-01-04 | Frontend hiện dòng nhắc khi `cwdFellBack` (5 locale) | `pty-transport.ts`/`pty-connection.ts`, `i18n/locales/*.json` | QA-UI-TERM-01 (nhánh lùi) | [ ] |
| T-QA-01-05 | Test e2e đã viết | `suite_qa_regressions.py` QA-TERM-01..03; `ui-live/qa-regressions.web.e2e.ts` QA-UI-TERM-01 | — | [x] (QA-TERM-01 xanh; 02, 03 và UI đỏ đúng lý do, 2026-10-16) |

## BUG-QA-02: bug/task native ([SOL-QA-02](../solutions/SOL-QA-02-bug-task-native-e2e.md))

| ID | Việc | Nơi sửa | Test cho xanh | Trạng thái |
|---|---|---|---|---|
| T-QA-02-01 | **Quyết định sản phẩm**: loại `light` (`bug`, `security`, `performance`) có theo `REQUEST_DEFAULT_ENGINE` không (đề xuất: không) | chủ sản phẩm | — | [ ] |
| T-QA-02-02 | `EffectiveEngine`/`NewDefaultedEngineSettings`: mặc định triển khai chỉ áp cho profile `full` | `request-service/.../domain/engine_settings.go`, `cmd/server/main.go` | QA-ENG-01 | [ ] |
| T-QA-02-03 | Test miền bảng 11 loại × cài đặt × ghim | `engine_settings_test.go` | — | [ ] |
| T-QA-02-04 | Test e2e đã viết | `suite_qa_regressions.py` QA-ENG-01..03; `ui-live/qa-bug-task-native-flow.web.e2e.ts` (3 ca); `ui-live/qa-native-execute-real.web.e2e.ts` (2 ca thật) | — | [x] (QA-ENG-01 đỏ; 02, 03 xanh; xem kết quả ở README) |

## BUG-QA-03: nguồn GitHub ([SOL-QA-03](../solutions/SOL-QA-03-github-task-source.md))

| ID | Việc | Nơi sửa | Test cho xanh | Trạng thái |
|---|---|---|---|---|
| T-QA-03-01 | Chuẩn hoá `repo`/`repoId` (`id:` và `::path`) cho mọi kênh `github.*` nhận repo từ giao diện | `api-gateway/.../channels_scm.go` (+ test bảng) | QA-GH-01, QA-GH-02 | [ ] |
| T-QA-03-02 | Loại lỗi `auth_required` (gateway + `ClassifiedError`), ánh xạ `SCM_CREDENTIAL_NOT_FOUND`/401/403 | `channels_scm.go`, `frontend/src/shared/types.ts` | QA-GH-03 | [ ] |
| T-QA-03-03 | Giao diện: "Connect GitHub" (OAuth `github.startAuthLogin` hoặc PAT) ở trạng thái `auth_required` | `TaskPage` và `task-source-*` | QA-UI-GH-01 | [ ] |
| T-QA-03-04 | Trạng thái nguồn không phụ thuộc `gh`; host lấy từ `devServer.list`; nhãn tên thay UUID; gộp dòng trùng | `task-source-provider-availability.ts`, `task-source-context-summary.ts` (+ test) | QA-UI-GH-01 | [ ] |
| T-QA-03-05 | `onboarding.getPreflightStatus` hạn 5 giây; bỏ `skip` cố định của `github-cli-auth` | `channels_onboarding.go`, `channels.go` | — (test Go) | [ ] |
| T-QA-03-06 | Tài liệu triển khai: kết nối GitHub cho tenant (`SSO_GITHUB_*`) | `deploy/dev/.env.example`, README | — | [ ] |
| T-QA-03-07 | Test e2e đã viết | QA-GH-01..03; QA-UI-GH-01 | — | [x] (QA-GH-02 xanh; 01, 03 và UI đỏ đúng lý do) |

## BUG-QA-04: Jira `jr.servicehub.vn` ([SOL-QA-04](../solutions/SOL-QA-04-jira-servicehub.md))

| ID | Việc | Nơi sửa | Test cho xanh | Trạng thái |
|---|---|---|---|---|
| T-QA-04-01 | `resolveAuth`: Bearer trước cho Server/DC, Basic cho Cloud; lưu `AuthScheme`; đệm theo `(url, hash(token))`; lỗi rõ | `issue-tracking-service/.../jira/client.go`, `usecase/connect.go` (+ test Go) | QA-JIRA-01..03 | [ ] |
| T-QA-04-02 | `Whoami` Server/DC: `name`/`key` thay `accountId` | `jira/client.go` | QA-JIRA-01 | [ ] |
| T-QA-04-03 | Hộp thoại kết nối: email tuỳ chọn cho Server/DC, hiện loại site, lý do lỗi | `frontend/.../jira-connect-dialog.tsx` (+ test) | QA-UI-JIRA-01 | [ ] |
| T-QA-04-04 | `deploy/dev/.env.example` + `scripts/connect-jira-from-env.sh` (nhận `JIRA_*` và `JR_*`, không in token) | `deploy/dev` | QA-JIRA-LIVE-01 | [ ] |
| T-QA-04-05 | (Tuỳ chọn) bootstrap khi khởi động qua `ISSUETRACKING_BOOTSTRAP_JIRA_*` hoặc Vault | `issue-tracking-service/cmd/server`, compose | — | [ ] |
| T-QA-04-06 | Kiểm tab Tasks → Jira: liệt kê, phân trang, "assigned to me", mở issue | `TaskPage` | QA-UI-JIRA-01 | [ ] |
| T-QA-04-07 | Test e2e đã viết (máy chủ Jira giả + bản thật bật bằng cờ) | QA-JIRA-01..03, QA-JIRA-LIVE-01; QA-UI-JIRA-01 | — | [x] (QA-JIRA-01, 03 đỏ; 02 bị chặn bởi 01; LIVE chưa chạy: cần người có `.env` chạy) |

## BUG-QA-05: log `unrecognized_model` ([SOL-QA-05](../solutions/SOL-QA-05-agent-diagnostic-lines.md))

| ID | Việc | Nơi sửa | Test cho xanh | Trạng thái |
|---|---|---|---|---|
| T-QA-05-01 | `parseAgentOutput`: `[claude-code:<code>]` → `kind: 'diagnostic'`, tông theo kết quả phiên | `agent-activity-lines.ts` | `agent-activity-diagnostics.test.ts` (bỏ `.fails`) | [ ] |
| T-QA-05-02 | `TaskAgentActivityPanel`: nhánh `diagnostic` trung tính, thu gọn "Diagnostics (n)" (5 locale) | `TaskAgentActivityPanel.tsx`, locales | test component + QA-UI-LOG-01 | [ ] |
| T-QA-05-03 | Agent báo một lần khi CLI cũ hơn mô hình cấu hình; tài liệu nâng cấp CLI | `agent/src/relay`, README triển khai | — | [ ] |
| T-QA-05-04 | Test đã viết | `agent-activity-diagnostics.test.ts` (2 ca `it.fails` + 1 ca đối chứng); QA-UI-LOG-01 trong `qa-native-execute-real` | — | [x] |

## BUG-QA-06: nhãn duyệt danh sách việc ([BUG-QA-06](../BUG-QA-06-task-list-approval-labelled-plan.md))

| ID | Việc | Nơi sửa | Test cho xanh | Trạng thái |
|---|---|---|---|---|
| T-QA-06-01 | So sánh `rawSubjectType === 'task_list'` (hoặc hàm `isTaskListApproval`) ở `PlanApprovalBar` và `PlanApprovalCard` | `frontend/.../request/plan/` (+ test) | ca `task` của `qa-bug-task-native-flow` | [ ] |
| T-QA-06-02 | Test e2e đã viết | `qa-bug-task-native-flow.web.e2e.ts` ca `task` | — | [x] (đỏ: nhãn "Approve plan") |

## Thứ tự đề xuất

1. T-QA-01-01 (một dòng, giải quyết lỗi 1 cho giao diện) → T-QA-01-02/03.
2. T-QA-03-01 (một dòng gateway, mở khoá danh sách) → 03-02 → 03-04 → 03-03/05/06.
3. T-QA-04-01/02 (nền của Jira) → 04-03 → 04-04.
4. T-QA-02-01 (quyết định) → 02-02/03.
5. T-QA-05-01/02.

Phụ thuộc: QA-JIRA-02 cần QA-JIRA-01; QA-UI-GH-01 cần cả 03-01 và 03-04; QA-UI-LOG-01 cần 05-01/02.
