# Tasks frontend: bug nghiệm thu e2e (v1)

Ký hiệu: `[x]` xong, `[~]` một phần, `[ ]` chưa làm, `[-]` không làm.

## Trạng thái thực thi (2026-10-10, cuối đợt)

**FE-E2E-001..014 đã làm; FE-007 còn `[~]`.** E2E qua UI với **Claude Code thật, không stub** (`run_ui_flow.py --real all`): 6/6 xanh, 21.4 phút — thực thi `change_request` có Phase (5 task phụ thuộc), task có phụ thuộc, task 1 bước; phân tích chỉ đọc `question`, `bug`; theo dõi phiên agent trên web (panel Agent activity có dòng `▸ Read/Edit/Bash` thật, dev server busy/idle + `macOS · arm64`, tab Result "Agent output", Open in Git hiện file đổi, terminal worktree mở được). Với AI stub: 3/3 test request-live xanh, `ask-back` 6/6. Playwright mock `request-web` xanh. Mọi lượt: 0 vi phạm an toàn, 0 Request mồ côi, 0 console error chưa phân loại.

Còn mở (chi tiết ở [README backend](../../../../backend-go/bugs/e2e-acceptance/README.md) mục 5): G5b `SCM_EMPTY_BRANCH`; G5d/G5e diff của file đã đổi vỡ (đang ghi `known-gap` trong spec); lịch sử từng dòng hoạt động không được lưu sau khi tải lại; kiểm Electron; vitest/lint toàn frontend đỏ do lỗi có sẵn trên HEAD; quyết định hướng A/B (phiên PTY theo TDD hay `execPrompt`) và B4 (quyền `--dangerously-skip-permissions`).

## Thứ tự

```
TASK-E2E-003 ──► FE-001
TASK-E2E-001 ──► FE-002 ──► FE-008
TASK-E2E-032 ──► FE-003
TASK-E2E-015/024/031 ──► FE-004 ──► FE-005, FE-006
TASK-E2E-011/012 ──► FE-007
```

FE-001 và FE-002 làm trước (P0): chúng làm cho lỗi hợp đồng kế tiếp **hiện ra** thay vì im lặng.

## Checklist

### SOL-FE-E2E-001

- [x] [TASK-FE-E2E-001](./TASK-FE-E2E-001-rpc-method-contract-test.md) P0 · Test hợp đồng: mọi kênh frontend gọi đều có ở gateway · phụ thuộc: TASK-E2E-003
- [x] [TASK-FE-E2E-002](./TASK-FE-E2E-002-clarification-dead-end-state.md) P0 · Trạng thái chặn có tên khi không tải được câu hỏi làm rõ · phụ thuộc: TASK-E2E-001

### SOL-FE-E2E-002

- [x] [TASK-FE-E2E-003](./TASK-FE-E2E-003-typed-clarification-input.md) P2 · Ô nhập theo `valueType` · phụ thuộc: TASK-E2E-032

### SOL-FE-E2E-003

- [x] [TASK-FE-E2E-004](./TASK-FE-E2E-004-request-error-code-map.md) P1 · Bổ sung bảng ánh xạ mã lỗi Request · phụ thuộc: TASK-E2E-015, 024, 031 (mã lỗi)
- [x] [TASK-FE-E2E-005](./TASK-FE-E2E-005-ai-not-configured-banner.md) P1 · Thông báo cấu hình AI có hướng khắc phục · phụ thuộc: TASK-FE-E2E-004, TASK-E2E-024
- [x] [TASK-FE-E2E-006](./TASK-FE-E2E-006-rate-limit-retry-and-disable.md) P1 · Xử lý `rate_limited`: khoá nút trong khoảng chờ và thử lại thao tác idempotent · phụ thuộc: TASK-FE-E2E-004, TASK-E2E-031
- [~] [TASK-FE-E2E-007](./TASK-FE-E2E-007-task-tree-empty-vs-error.md) P2 · Cây task: phân biệt rỗng, lỗi và không có quyền · phụ thuộc: TASK-E2E-011, 012
- [x] [TASK-FE-E2E-008](./TASK-FE-E2E-008-live-stack-playwright-smoke.md) P2 · Playwright smoke chạy trên stack thật cho luồng làm rõ (có điều kiện) · phụ thuộc: TASK-E2E-001, 026

### Phát sinh khi chạy UI thật

- [x] [TASK-FE-E2E-009](./TASK-FE-E2E-009-ui-live-findings-frontend.md) P1 · Tab Plan, nút Generate solution, thanh duyệt, câu hỏi chặn + "Hỏi lại", lỗi `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS`, Solution cũ sau supersede · phụ thuộc: TASK-FE-E2E-004, 005, 006 và các sửa backend TASK-V2-022/023

- [x] [TASK-FE-E2E-010](./TASK-FE-E2E-010-real-claude-ui-execution-and-readonly.md) P0 · E2E thật qua UI với Claude Code thật (không stub): phân tích chỉ đọc, duyệt Plan và thực thi (Phase, phụ thuộc), thiết kế an toàn nhiều lớp
- [x] [TASK-FE-E2E-011](./TASK-FE-E2E-011-agent-session-monitoring-in-ui.md) P0 · Theo dõi phiên agent trên web (bản đồ, spec `--real monitor`): panel hoạt động, dev server busy, tab Result, Open in Git
- [x] [TASK-FE-E2E-012](./TASK-FE-E2E-012-agent-live-activity-stream-and-panel.md) P0 · Luồng hoạt động đọc được của agent (stream-json) và panel Agent activity
- [x] [TASK-FE-E2E-013](./TASK-FE-E2E-013-task-sheet-fixes.md) P1 · Sheet task của Request: "Open in Git" điều hướng thật, bố cục chật, Escape chỉ đóng sheet, gợi ý tab Result · phụ thuộc: TASK-FE-E2E-011, 012
- [x] [TASK-FE-E2E-014](./TASK-FE-E2E-014-open-in-git-source-control-empty.md) P1 · "Open in Git" mở Source Control nhưng không có thay đổi (một repo lỗi `worktree.detectedList` làm mất mọi worktree), spawn-helper của node-pty tự sửa quyền · phụ thuộc: TASK-FE-E2E-011, 013

### SOL-FE-E2E-004: web luôn gắn host (dev server) khi tạo project/repo ([solution](../solutions/SOL-FE-E2E-004-web-repo-host-binding.md); backend: [SOL-V2-002](../../../../backend-go/bugs/e2e-acceptance/v2/solutions/SOL-V2-002-repo-host-binding-and-no-silent-local.md))

Quyết định đã chọn: repo luôn gắn dev server, project tạo gắn dev server được chọn và repo thừa hưởng, chọn sẵn host duy nhất, repo cũ không tự migrate (công cụ dry-run), Browse folder có giới hạn.

```
FE-016 (truyền devServerId) ──► FE-017 (các đường còn lại) ──► FE-021 (i18n, test, spec live)
FE-016 ──► FE-018 (chọn host) ──► FE-020 (Browse có giới hạn) ──► FE-021
FE-016 ──► FE-019 (hiển thị/đổi host, lỗi có mã)               ──► FE-021
TASK-V2-030 ──► FE-020      TASK-V2-028/029 ──► FE-019 (mã lỗi)
```

- [x] [TASK-FE-E2E-015](./TASK-FE-E2E-015-web-add-project-drops-dev-server.md) P1 · Bug gốc (đã tái hiện bằng spec live `--repro add-project-worktree`, ĐỎ cho tới khi 016 + backend xong)
- [x] [TASK-FE-E2E-016](./TASK-FE-E2E-016-pass-dev-server-id-on-add-project.md) P0 · Truyền `devServerId` từ hộp thoại Add Project xuống `repo.add` (preload, store, hook), project mặc định theo host · phụ thuộc: không
- [x] [TASK-FE-E2E-017](./TASK-FE-E2E-017-pass-dev-server-id-on-other-creation-paths.md) P1 · Clone, create, folder không phải git, `addRuntimeRepoRemote`, onboarding, Create New Project mang host · phụ thuộc: 016
- [x] [TASK-FE-E2E-018](./TASK-FE-E2E-018-add-project-host-selection-ux.md) P0 · Chọn sẵn host duy nhất, bắt chọn khi nhiều, chặn khi không có, giữ lựa chọn khi host ngắt · phụ thuộc: 016
- [x] [TASK-FE-E2E-019](./TASK-FE-E2E-019-show-and-change-repo-host.md) P1 · Hiển thị host của repo, "Đổi host", lỗi có mã khi tạo worktree · phụ thuộc: 016; mã lỗi TASK-V2-028/029
- [x] [TASK-FE-E2E-020](./TASK-FE-E2E-020-bounded-remote-file-browser.md) P1 · Browse folder có giới hạn, một lần gọi cho đường dẫn tuyệt đối, lỗi inline · phụ thuộc: TASK-V2-030, 018
- [x] [TASK-FE-E2E-021](./TASK-FE-E2E-021-i18n-tests-and-live-spec.md) P1 · i18n 5 ngôn ngữ, test hợp đồng, Playwright mock, spec live thành hồi quy · phụ thuộc: 016 đến 020

Định nghĩa xong của SOL-FE-E2E-004:

- [ ] `run_ui_flow.py --repro add-project-worktree` XANH (repo tạm, tự dọn; `repo.list` trả `devServerId` của host đã chọn).
- [ ] Vitest các file đã đổi xanh; Playwright `request-web` (kể cả spec mới) xanh; `tsc`/`oxlint` sạch trên file đã đổi.
- [ ] Không hồi quy: `run_ui_flow.py --real all` 6/6.
- [ ] Chuỗi mới có đủ en/es/ja/ko/zh; không màu/cỡ chữ mới (STYLEGUIDE ở `guides/STYLEGUIDE.md`).
- [x] Tài liệu: ghi "Kết quả thực thi" vào từng task; TASK-FE-E2E-015 sang `[x]`.
- [ ] Dọn dữ liệu: 0 repo/project `e2e-ui-repro-*` còn lại; repo thật của người dùng (ví dụ `codex`) không bị đổi.

## Định nghĩa "xong"

- [~] Vitest của các file đã đổi xanh; `tsc` và `oxlint` sạch trên file đã đổi. Cả bộ `pnpm lint`/`typecheck`/vitest toàn frontend **không xanh** do lỗi có sẵn trên HEAD (thiếu `src/main`, `src/relay`, `window.api`, `jsdom`; `FleetServerStatusBadge`; `RequestBacklogRow`...), đã đối chiếu bằng bản sao HEAD sạch và không có test nào xanh ở HEAD mà đỏ bây giờ.
- [x] Mỗi chuỗi mới có i18n đủ 5 ngôn ngữ (en, es, ja, ko, zh); không màu/cỡ chữ mới.
- [x] Suite Playwright `request-web` xanh (35 xanh, 1 skipped có sẵn) sau khi sửa hạ tầng mock (project `request-web`, import, `webServer.cwd`, phím Mod+Enter theo userAgent); spec live xanh trên stack local.
- [~] Kiểm tra trên cả Electron và web (G2): mã dùng chung `callRequestRpc`/`callRuntimeRpc`; chỉ kiểm bằng trình duyệt web, **chưa kiểm Electron**.
