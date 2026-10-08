# FE-REQ-TASK-019-06: `CreateRequestDialog` và nút "Tạo Request" trên trang Tasks

**From Solution:** [FE-REQ-SOL-019](../solutions/FE-REQ-SOL-019-request-list-detail-classification-ui.md) mục 2.6
**Priority:** P0
**Area:** frontend / request + Tasks page
**File:** `frontend/src/renderer/src/components/request/CreateRequestDialog.tsx`, `CreateRequestButton.tsx`, `CreateRequestIssueButton.tsx`, `create-request-from-issue.ts` (mới); `components/task-page-jira-issue-list.tsx` (sửa, quanh dòng 225-245), `components/JiraIssueWorkspace.tsx` (sửa, dòng 437 và 774), `components/GitHubItemDialog.tsx` (sửa); test `create-request-from-issue.test.ts`, `CreateRequestDialog.test.tsx`, bổ sung vào test hiện có của danh sách Jira
**Depends on:** FE-REQ-TASK-018-03, 018-05
**Status:** [~] PARTIAL — CreateRequestDialog/IssueButton, create-request-from-issue + tests pass and buttons mounted in Jira list/workspace and GitHubItemDialog; e2e not written, no row-level test that Start workspace is untouched

## Context

- Kênh: `request.create {projectId,title(<=500),body(<=100000),source?{provider,ref,url,site},hints?,clientRequestId?}` → `{request, created}` (CR-016 2.3). Nguồn: gateway chỉ chấp nhận `jira|github|gitlab|linear` kèm `ref`; `mcp|webhook|manual` tường minh bị `REQUEST_SOURCE_FORBIDDEN` (CR-016 2.6). Không gửi `source` thì `manual`.
- Nút hiện có: `onStartWorkspace` ("Start workspace", icon `ArrowRight`) trong `task-page-jira-issue-list.tsx`; `onUse` trong `JiraIssueWorkspace.tsx:56` và `GitHubItemDialog.tsx:332`. `TaskPage.tsx` (~8.280 dòng, baseline max-lines) KHÔNG được thêm logic.
- `JiraIssue`: `key`, `siteId?`, `title`, `description?`, `url`. `GitHubWorkItem`: `type`, `number`, `title`, `url`; `owner/repo` không nằm trong `GitHubWorkItem` (chỉ `prRepo?`).
- `useJiraProjectPreselect.ts` và `lib/jira-project-matching.ts` (`matchProjectForJiraIssue`) cho gợi ý dự án.

## Việc cần làm

1. `create-request-from-issue.ts`: `jiraIssueToCreateParams(issue, projectId)`, `githubItemToCreateParams(item, repoIdentity, projectId)` (ref `owner/repo#number`, `site` rỗng; xác định `repoIdentity` từ ngữ cảnh `GitHubItemDialog` trước khi viết: đọc props/`repoPath`, chưa kiểm chứng), từ chối `item.type==='pr'` (trả `null`).
2. `CreateRequestDialog({initial, source?, onCreated})`: tiêu đề, nội dung (điền sẵn), chọn dự án (mặc định theo ánh xạ Jira/repo GitHub; nếu chưa chọn thì nút "Tạo" khoá kèm lý do), mức khẩn, gợi ý loại ("Để AI phân loại" mặc định). `clientRequestId` sinh một lần khi mở dialog.
3. Kết quả `created===false`: toast "Issue này đã có Request" kèm nút "Mở"; ngược lại `openRequestPage({section:'requests', requestId})`.
4. `CreateRequestIssueButton({issue|item})`: `variant="outline"`, icon `Inbox`, nhãn "Tạo Request", tooltip "Gửi issue vào luồng phân tích. Khác với Start workspace, không mở worktree ngay". Không gọi `task.create`. Không render khi `requestFlowSupport!=='supported'` hoặc là PR.
5. Gắn nút: một dòng trong ba file Tasks nói trên (mount `CreateRequestIssueButton`); thêm nút trong hàng, đặt cạnh nút "Start workspace" nhưng không thay.
6. `CreateRequestButton` ở `RequestPageHeader` (tạo thủ công, không `source`).
7. `isScreenSubmitShortcut` gửi từ ô nội dung; nhãn qua `ShortcutKeyCombo`.
8. i18n: `CreateRequestButton.label`, `CreateRequestDialog.{title,duplicate,project,projectRequired,urgency,typeHint,submit}`, `CreateRequestIssueButton.{label,tooltip}`; 5 locale.

## Kiểm thử

- `create-request-from-issue.test.ts`: Jira (`key`, `siteId` → `source.site`), GitHub (`owner/repo#n`), PR bị từ chối, `description` rỗng; không bao giờ trả `provider` là `mcp|webhook|manual`.
- `CreateRequestDialog`: điền sẵn, khoá khi chưa chọn dự án, trùng lặp (`created=false`), lỗi `validation`/`rate_limited` (`REQUEST_PENDING_LIMIT`), phím `Mod+Enter` đúng nền tảng.
- Test danh sách Jira: có nút "Tạo Request" cạnh "Start workspace"; bấm không gọi `onStartWorkspace`.
- E2E `tests/e2e/request-create-from-issue.spec.ts` (mới; cần backend CR-REQ-004; chưa chạy).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/CreateRequest src/renderer/src/components/request/create-request-from-issue src/renderer/src/components/task-page-jira`.

## Tiêu chí hoàn thành

- [ ] Nút có trên issue Jira và issue GitHub, không có trên PR; "Start workspace" không đổi hành vi.
- [ ] Tạo từ issue đã có Request không tạo bản mới, hiện toast và nút "Mở".
- [ ] `TaskPage.tsx` không thêm quá một dòng; không thêm `max-lines` disable.
- [ ] Ẩn nút khi runtime không hỗ trợ.

## Rủi ro và lưu ý

- Chạy GitNexus `impact` cho `JiraIssueRow`/`JiraIssueWorkspace`/`GitHubItemDialog` trước khi sửa (chưa chạy).
- GitLab, Linear ngoài phạm vi v6.
