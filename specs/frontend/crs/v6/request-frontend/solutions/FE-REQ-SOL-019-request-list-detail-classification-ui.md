# FE-REQ-SOL-019: Danh sách, chi tiết Request, xác nhận phân loại, "Tạo Request" từ Tasks

> ✅ **Done.** Verified 2026-10-08: 019-01..07 DONE; web e2e request-list-detail.web.e2e.ts 6/6 pass, Jira row test for Create request vs Start workspace. Notes: contract has no `request.links` yet so Related tab reads `links` from request.get; CreateRequestDialog sends the type hint as `hints.issueType`.

**CR:** [CR-REQ-019](../../../../../../docs/crs/v6/request-frontend/CR-REQ-019-request-list-detail-classification-ui.md)
**Area:** frontend (`components/request/`, `components/TaskPage.tsx` qua component riêng)
**Hợp đồng backend:** CR-REQ-016 mục 2.3 (`request.*`), 2.6 (nguồn Request), 2.8 (mã lỗi). CONTRACT backend chưa có: chỗ tạm ghi "(tạm)".
**TDD tham chiếu:** [v5/05-ui-components](../../../../tdd/v5/05-ui-components.md), [v5/15-task-graph-ui](../../../../tdd/v5/15-task-graph-ui.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md), [v5/02-state-management](../../../../tdd/v5/02-state-management.md)

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `components/TaskPage.tsx` (`handleUseJiraItem`, `onUse`), `components/task-page-jira-issue-list.tsx` (nút "Start workspace" dòng 225-245, prop `onStartWorkspace`), `components/JiraIssueWorkspace.tsx` (`onUse` dòng 56, nút ở 437 và 774), `components/GitHubItemDialog.tsx` (`onUse` dòng 332, 5557, 5699), `hooks/useJiraProjectPreselect.ts`, `shared/jira-types.ts` (`JiraIssue`: `key`, `siteId`, `title`, `description?`, `url`), `shared/types.ts:1493` (`GitHubWorkItem`: `type 'issue'|'pr'`, `number`, `title`, `url`, `prRepo?`), `lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx`, `components/ui/*` (có `tabs`, `table`, `resizable`, `select`, `progress`, `skeleton`, `dialog`, `textarea`, `sheet`; không có `alert`).

**Correction relative to CR-REQ-019:**

| # | CR-019 | Thực tế | Quyết định |
|---|---|---|---|
| 1 | Nút "Tạo Request" cạnh `onStartWorkspace` | Nút hiện nhãn "Start workspace" (icon `ArrowRight`), không phải "Start work" | Dùng đúng nhãn hiện có khi nhắc; nút mới nhãn "Tạo Request" khác hẳn |
| 2 | `RequestDetailPane` đọc lịch sử và Request con từ `GetRequest` | CR-016: `request.get` không kèm lịch sử; `request.typeHistory` riêng; không có kênh Request con | `RequestHistoryTab` dùng `request.typeHistory`; `RequestRelatedTab` đọc `links` của `request.get` nếu có, không thì hiện "chưa hỗ trợ" (tạm) |
| 3 | `request.create` tham số snake_case (`source_provider`) | CR-016: `source {provider, ref, url, site}`, camelCase | Dùng CR-016 |
| 4 | Gửi `type_source=human` khi đổi loại | `request.confirmType {id,type,size?,urgency?,reason?}` không có `typeSource`; backend tự đặt nguồn theo người gọi | UI không gửi `typeSource`; hiển thị nhãn "Sửa bởi người" từ kết quả `typeSource` |
| 5 | Chip GitHub `owner/repo#number` | `GitHubWorkItem` không có `owner/repo` thẳng (chỉ `prRepo?`); danh tính repo lấy từ ngữ cảnh TaskPage | Task 019-06 phải xác định nguồn `owner/repo` (xem rủi ro); chưa kiểm chứng |
| 6 | Tạo Request con (`spawnChild`) với "loại con theo README 3.4" | CR-016: `request.spawnChild {id, reason, title, body, type?}`; `reason ∈ request_links.reason` | Bảng loại cha → `reason` + loại con gợi ý nằm trong `request-flow-registry.ts` (xem 2.5) |

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/components/request/
  RequestsTab.tsx                   RequestListToolbar.tsx      RequestFilterBar.tsx
  RequestList.tsx                   RequestRow.tsx              RequestListStates.tsx (skeleton, rỗng, lỗi)
  request-list-keyboard.ts          (j/k/Enter/Escape)
  RequestDetailPane.tsx             RequestDetailHeader.tsx     RequestStageTimeline.tsx
  request-stage-timeline-model.ts   (hàm thuần dựng bước từ registry)
  RequestOverviewTab.tsx            RequestHistoryTab.tsx       RequestRelatedTab.tsx
  RequestBacklogBanner.tsx
  TypeConfirmationCard.tsx          ChangeTypeConfirmDialog.tsx
  ReturnToBacklogDialog.tsx         CancelRequestDialog.tsx     SpawnChildRequestDialog.tsx
  CreateRequestDialog.tsx           CreateRequestButton.tsx
  create-request-from-issue.ts      (ánh xạ JiraIssue/GitHubWorkItem → CreateRequestParams)
frontend/src/renderer/src/components/
  task-page-jira-issue-list.tsx     (sửa) thêm CreateRequestIssueButton cạnh nút Start workspace
  JiraIssueWorkspace.tsx, GitHubItemDialog.tsx (sửa) thêm cùng nút
frontend/src/shared/request-flow-registry.ts (sửa của SOL-018): CHILD_REQUEST_RULES
```

`RequestAnalysisTab` và `RequestPlanTab` là chỗ cắm; CR-020/021 điền (SOL-020, SOL-021).

### 2.2 Danh sách

- `useRequests(filters)` (`request.list {projectId?, status[]?, type[]?, sourceProvider?, pageSize:30, pageToken}`). Cập nhật tại chỗ qua `request-event-bus` khi nhận `request.event` có `requestId` đang hiện.
- Bộ lọc: trạng thái, loại, nguồn, "Chờ tôi xác nhận" (= `status=['awaiting_type_confirmation']`), "Đang chạy" (= `analyzing|planning|executing`). Lọc theo người báo cáo và tìm chữ (`q`) không có trong tham số `request.list` của CR-016: KHÔNG làm ở v6 (CR-019 nêu `reporterId`, `q`; ghi mục 7). Sắp xếp cố định theo cập nhật mới nhất (backend quyết).
- Trạng thái: `RequestListSkeleton` 8 hàng; rỗng `emptyNone` (nút "Tạo Request") hoặc `emptyFiltered` ("Xoá bộ lọc"); lỗi `network` banner "Thử lại"; `forbidden` thông báo không có quyền; `unsupported` không render (khung `RequestPage` thay toàn thân).
- Bàn phím: `j`/`k` đổi hàng, `Enter` mở, `Escape` đóng chi tiết; bỏ qua khi tiêu điểm ở `input|textarea|select|[contenteditable]` hoặc có phím sửa đổi. Không dùng `metaKey` cứng (AGENTS.md). Tooltip nút "Đóng" hiển thị `Esc` bằng `ShortcutKeyCombo`.
- Bố cục: `ui/resizable.tsx` trái 40%; màn hẹp (dưới 768 px) thì chi tiết thay danh sách và có nút "Quay lại danh sách".

### 2.3 Chi tiết và dòng thời gian

`request-stage-timeline-model.ts`: `buildStageTimeline(type, size, status) → {steps: Step[], current: StepId, note?: 'hotfix_fast_diagnosis'}` dựng từ `REQUEST_FLOW_REGISTRY[type]` (không `if (type===...)`). Bước: `classification`, `analysis` (nhãn theo `analysisKind`), `plan` (`plan` hay `task_list` hay `single_task`), `phase` (khi `flowHasPhase`), `execution`. Ánh xạ `status → current`: `new|classifying|awaiting_type_confirmation` → classification; `analyzing|awaiting_analysis_approval` → analysis; `planning|awaiting_plan_approval` → plan; `executing` → execution (hoặc phase); `completed`: tất cả xong; `request_backlog`: giữ bước `returnedFromStage`.

Header: nút theo trạng thái (bảng CR-019 2.3), tất cả qua `useRequestActions`. Quyền: không có `viewerCan`, nên hiện nút và xử lý `forbidden` (toast, khoá nút ghi cho phiên). `request_backlog` hiện `RequestBacklogBanner` với `returnedFromStage`, `returnReason`.

### 2.4 `TypeConfirmationCard`

Hiện khi `status==='awaiting_type_confirmation'`. Nội dung: loại đề xuất, `confidence` (`ui/progress.tsx` + %), `classificationReason` (6 dòng, "Xem thêm"), size/urgency đề xuất, cảnh báo khi `isLowConfidence(confidence)` (`< LOW_CONFIDENCE_THRESHOLD`), `Select` 11 loại kèm tóm tắt luồng từ registry. Nút chính "Xác nhận loại" → `request.confirmType {id,type,size?,urgency?,reason?}`; "Phân loại lại" → `request.classify` (25 s: nút có trạng thái đang chạy, vô hiệu khi `REQUEST_CLASSIFICATION_LIMIT`). `isScreenSubmitShortcut` trong ô lý do gửi xác nhận; nhãn bằng `ShortcutKeyCombo keys={[getScreenSubmitModifierLabel(), 'Enter']}`. `classifying` thì `Loader2` "AI đang phân loại". `invalid_state`/`conflict` tải lại và đóng hộp.

`ChangeTypeConfirmDialog` mở khi đổi loại từ trạng thái sau `awaiting_type_confirmation`: gọi `request.changeType {id,toType,reason}` (lý do bắt buộc, `REQUEST_REASON_REQUIRED`); mã `REQUEST_TYPE_CHANGE_NOT_ALLOWED`/`REQUEST_TYPE_CHANGE_USE_CHILD` hiện lời giải thích và gợi ý "Tạo Request con".

### 2.5 Lịch sử, Request con

- `RequestHistoryTab`: `request.typeHistory` (`from → to`, `actorKind` AI/người, lý do, thời điểm; mới nhất trên cùng). Rỗng "Chưa đổi loại lần nào".
- `RequestRelatedTab`: nhóm theo `RequestLink.reason`; mỗi dòng mở Request. Rỗng "Không có Request liên quan"; khi `linksSupported=false`: "Runtime chưa hỗ trợ xem liên kết" (không lỗi đỏ).
- `CHILD_REQUEST_RULES` (suy từ README 3.4 đường nâng cấp, cần CR-REQ-006 xác nhận): `spike|question` → `reason` `spawned_by_spike|spawned_by_question`, loại gợi ý `change_request|task`; `hotfix` → `followup_hotfix`, loại `bug|task`; `bug|task|refactor|performance|docs` → `escalation`, loại `change_request`; `security` → `escalation`, loại `hotfix`.

### 2.6 "Tạo Request" từ Tasks

- `create-request-from-issue.ts`: `jiraIssueToCreateParams(issue, projectId)` → `{projectId, title: issue.title, body: issue.description ?? '', source:{provider:'jira', ref: issue.key, url: issue.url, site: issue.siteId ?? ''}}`; `githubItemToCreateParams(item, repoIdentity, projectId)` → `source:{provider:'github', ref: \`${owner}/${repo}#${item.number}\`, url: item.url, site: ''}` (định dạng `ref` GitHub phải khớp khoá idempotency của CR-REQ-004; chưa kiểm chứng). Không áp dụng cho `item.type==='pr'`. `clientRequestId = crypto.randomUUID()` giữ cho cả lần thử lại của một dialog.
- Frontend KHÔNG bao giờ gửi `provider` là `mcp`, `webhook`, `manual` tường minh (`REQUEST_SOURCE_FORBIDDEN`, CR-016 2.6). Tạo thủ công: bỏ `source` (gateway gán `manual`).
- Nút `CreateRequestIssueButton` (`variant="outline"`, icon `Inbox` hoặc `GitPullRequestCreateArrow` của lucide, nhãn "Tạo Request", tooltip nói rõ khác "Start workspace", không mở worktree). Không gọi `task.create` hay đường tạo worktree.
- `CreateRequestDialog`: tiêu đề (≤500), nội dung (≤100000) điền sẵn, dự án (mặc định Jira theo `matchProjectForJiraIssue`/mẫu `useJiraProjectPreselect.ts`, GitHub theo repo đang xem), mức khẩn, gợi ý loại ("Để AI phân loại" mặc định, gửi `hints`). Kết quả `created=false` thì toast "Issue này đã có Request" kèm "Mở". Thành công: `openRequestPage({section:'requests', requestId})`. Chưa chọn dự án: nút "Tạo" khoá kèm lý do. `Mod+Enter` (`isScreenSubmitShortcut`) gửi.
- Không render nút khi `requestFlowSupport!=='supported'`.

### 2.7 i18n, trạng thái

Khoá tiền tố `auto.components.request.`: như CR-019 2.7 cộng `RequestListToolbar.*`, `RequestFilterBar.*`, `ReturnToBacklogDialog.*`, `CancelRequestDialog.*`, `SpawnChildRequestDialog.*`, `RequestBacklogBanner.*`, `RequestRelatedTab.unsupported`, `CreateRequestIssueButton.label|tooltip`. Đủ 5 locale; test phủ khoá mở rộng `request-locale-coverage.test.ts`.

## 3. Quyết định thiết kế

- Một `RequestDetailPane` có tab; CR-020/021 chỉ cắm nội dung.
- Dòng thời gian dựng từ registry, thêm loại không phải sửa UI.
- "Tạo Request" tách khỏi "Start workspace" (nhãn, kiểu nút, hậu quả); không gộp menu để tránh bấm nhầm.
- Chống trùng ở backend (idempotency `request_idempotency`), UI chỉ báo.
- Không làm tìm chữ và lọc người báo cáo vì CR-016 không có tham số tương ứng.
- Component mới tách file; `TaskPage.tsx` (~8.280 dòng, trong baseline max-lines) chỉ nhận một dòng gắn component, không `max-lines` disable.

## 4. Phụ thuộc và thứ tự

Phụ thuộc FE-REQ-SOL-018 và backend CR-REQ-004/005/006 qua CR-016. Mở khoá SOL-020, 021, 022, 023. Thứ tự task: 019-01 → 019-02 và 019-03 → 019-04 → 019-05 → 019-06 (độc lập với 019-02..05 sau 019-01) → 019-07.

## 5. Kiểm thử

Vitest: timeline cho 11 loại × size; lọc; `jiraIssueToCreateParams`/`githubItemToCreateParams`; `isLowConfidence`; `TypeConfirmationCard`; `CreateRequestDialog`; `ReturnToBacklogDialog` (lý do bắt buộc); bàn phím danh sách; test nút trong `task-page-jira-issue-list` (không có trên PR). E2E `tests/e2e/request-list-detail.spec.ts` (mới, cần backend CR-REQ-004/005): tạo, phân loại, xác nhận, đổi loại. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/request`. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Nguồn `owner/repo` cho GitHub và định dạng `ref` chưa chốt.
- `LOW_CONFIDENCE_THRESHOLD` chưa có dữ liệu.
- Jira → dự án Orca chưa kiểm chứng trên Jira thật (README v6 mục 7).
- `classify` tới 25 s: cần tránh khoá giao diện.
- Phím `j/k` có thể xung đột phím tắt toàn cục: đăng ký chỉ khi danh sách giữ tiêu điểm.

## 7. Câu hỏi mở

1. `request.get` có trả `links`, `planTaskId`, `viewerCan`?
2. Có thêm tham số `q`, `reporterId` cho `request.list`?
3. GitLab và Linear có vào v6 không (README chỉ Jira/GitHub)?
4. Ai được "Sửa loại" sau khi đã có Plan (CR-REQ-010)?
5. Bảng loại con hợp lệ cho `spawnChild` do ai chốt (CR-REQ-006)?

## 8. Tham chiếu

`/opt/repos/orca/docs/crs/v6/README.md`, `/opt/repos/orca/frontend/src/renderer/src/components/TaskPage.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/task-page-jira-issue-list.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/JiraIssueWorkspace.tsx`, `/opt/repos/orca/frontend/src/renderer/src/components/GitHubItemDialog.tsx`, `/opt/repos/orca/frontend/src/renderer/src/hooks/useJiraProjectPreselect.ts`, `/opt/repos/orca/frontend/src/renderer/src/lib/screen-submit-shortcut.ts`, `/opt/repos/orca/frontend/src/renderer/src/components/ShortcutKeyCombo.tsx`, `/opt/repos/orca/frontend/src/shared/jira-types.ts`, `/opt/repos/orca/frontend/src/shared/types.ts`, `/opt/repos/orca/docs/ui/pages/tasks.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`.
