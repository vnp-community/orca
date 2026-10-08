# Jira ↔ Orca: ánh xạ giữa issue Jira, project Orca, repo và worktree

**Cập nhật:** 2026-10-05 (đã thêm ánh xạ project/site). Đọc từ code, **chưa chạy thử với Jira thật**. Những chỗ chưa kiểm chứng được ghi rõ ở mục 5.

## 1. Kết luận ngắn

**Mỗi project Orca có thể khai báo project Jira của nó** (`jiraProjectKey`, ví dụ `ENG`) **và site Jira** (`jiraSiteId`, là URL site) trong Project settings → Jira mapping. Khi "Start work" một issue `ENG-123`, composer tự chọn project Orca có khoá `ENG` (ưu tiên đúng site); người dùng vẫn đổi được. Project chưa khai báo thì hành vi như cũ (không tự chọn).

Liên kết được tạo **lúc "Start work"**, theo project và repo trong composer, và **mang theo site**: `task.task_sources.site_id` và `worktrees.linked_issue_site`. Từ đó liên kết đi theo worktree (và task, nếu bật cờ).

## 2. Các lớp, từ Jira tới worktree

| Lớp | Cách hoạt động | Lưu ở đâu |
|---|---|---|
| **Kết nối Jira** | Mỗi người dùng tự kết nối Jira của mình (URL site, email, token). Có thể kết nối nhiều site. Không gắn với project Orca nào. | `credential-broker`, khoá theo (tenant, user, provider) |
| **Xem issue** | Trang Tasks hiện issue theo project Jira (bộ chọn project Jira). Lựa chọn chỉ nằm trong trạng thái của trang. | Bộ nhớ trình duyệt, không lưu |
| **Ngữ cảnh nguồn task** | `JiraTaskProviderIdentity` có `siteId`, `siteUrl`, `projectKey`, nhưng `TaskPage.tsx` chỉ điền site, **không bao giờ điền `projectKey`**. | Không lưu |
| **Project Orca nào?** | Composer tự chọn project có `jiraProjectKey` trùng tiền tố khoá (và `jiraSiteId` trùng site nếu có). Nhiều project khớp mà không thu hẹp được thì không tự chọn. | `project.projects.jira_project_key`, `jira_site_id` |
| **"Start work"** | Composer mở với issue Jira; người dùng chọn project và repo Orca ở đó (mặc định lấy giá trị dự phòng). | Xem hai dòng dưới |
| **Liên kết lên worktree** | `linked_issue_provider = "jira"`, `linked_issue_ref = "ENG-123"` (khoá issue) và `linked_issue_site` (site). | `project.worktrees` cùng `project_id`, `repo_id` |
| **Liên kết lên task** (khi bật cờ Experimental "Jira task link") | `task.task_sources`: duy nhất theo `(tenant, project_id, provider, site_id, ref)`, kèm URL issue. | `task.task_sources` |

`Project.providerIdentity` ở frontend (`frontend/src/shared/types.ts`) chỉ hỗ trợ GitHub (`owner`/`repo`); không có chỗ cho Jira.

### Luồng

```
Jira issue ENG-123
   │  (trang Tasks, nguồn Jira, kết nối của chính người dùng)
   ▼
"Start work" ──► composer: người dùng chọn project + repo Orca
   │
   ├─► worktree.create  ──► project.worktrees(project_id, repo_id, linked_issue = jira:ENG-123)
   │                                 │
   │                                 └─► sự kiện worktree.created (kèm actor_user_id)
   │                                           └─► issue-status-sync ──► Jira: To Do → In Progress
   │
   └─► [cờ bật] task.createFromSource ──► task.task_sources(project_id, jira, ENG-123, url)
                                             └─► task.execute dùng lại worktree đã liên kết issue
```

## 3. Khoá issue được lấy từ đâu (PR)

Khi tạo/merge PR, `scm-integration-service` rút khoá Jira theo thứ tự: tên branch → tiêu đề PR → mô tả PR. Trong mô tả chỉ nhận khoá đi sau `fixes` / `closes` / `resolves` (một lời nhắc trần như "xem ENG-5" bị bỏ qua). Khoá phải viết hoa, không dính chữ/số hai bên, và loại các tiền tố không phải Jira như `CVE`, `SHA`, `ISO`, `UTF`. Khi merge, API không trả mô tả PR nên chỉ rút từ branch và tiêu đề. Quy tắc nằm ở `backend-go/services/scm-integration-service/internal/usecase/linked_jira_issue_parser.go`.

## 4. Hệ quả cần biết

- **Tự chọn project chỉ khi đã khai báo.** Project chưa có `jiraProjectKey` thì không được gợi ý; chọn sai ở composer vẫn làm task và worktree nằm sai chỗ.
- **Một issue có thể có nhiều task/worktree.** Cùng một issue "Start work" ở hai project Orca khác nhau tạo hai task và hai worktree riêng. Bất biến chỉ là: trong **một** project, một issue có đúng một task.
- **Dữ liệu cũ không có site.** Dòng `task_sources` và worktree tạo trước thay đổi này có site rỗng và khớp với mọi site; hai site dùng chung khoá có từ trước vẫn bị dồn vào một dòng cho đến khi liên kết lại.
- **PR chưa mang site.** Sự kiện PR không chứa issue/site nên chưa đồng bộ theo site.
- **Đồng bộ trạng thái đi theo worktree và người tạo.** Service gọi Jira bằng chính kết nối của người đã tạo worktree (sự kiện mang `actor_user_id`). Sự kiện không có người thực hiện thì bị bỏ qua có log.
- **Xoá worktree không đổi issue.** Chỉ có `worktree.created → In Progress` (khi issue đang `To Do`), PR tạo → "In Review" và PR merge → "Done" (khi issue đang ở nhóm "đang làm").

## 4b. Request sở hữu issue (cập nhật 2026-10-08)

Khi issue Jira có một **Request chưa kết thúc** (nguồn `jira`, chưa `completed`/`cancelled`), `issue-status-sync` đồng bộ theo **Request** và **bỏ qua**
đồng bộ theo worktree và PR cho issue đó (metric `orca_issuesync_skipped_request_owned_total{source}`, `result=skipped_request_owned`). Lý do: một Request
có thể có nhiều Phase và nhiều PR, nên PR merge đầu tiên không được đẩy Jira sang "Done".

| Sự kiện Request | Jira |
|---|---|
| vào `executing` (hoặc `analyzing` với `spike`/`question`) | "In Progress" |
| `completed` | "Done" |
| `request_backlog`, `cancelled` | không đổi |

Chi tiết:
- Việc tra "issue có Request không" gọi `LookupRequestBySource` của `request-service` (nội bộ, cần token chia sẻ). Lỗi tra cứu thì sự kiện được giao lại tối đa 3 lần rồi bỏ.
  Chỉ trả "có" khi cờ luồng Request của tenant bật; cờ tắt thì đồng bộ theo worktree/PR chạy như cũ.
- Cấu hình: `REQUEST_SERVICE_ADDR`, `REQUEST_SERVICE_INTERNAL_TOKEN` (khớp `SERVICE_INTERNAL_TOKEN` của `request-service`),
  `ISSUE_SYNC_JIRA_STATUS_IN_PROGRESS`, `ISSUE_SYNC_JIRA_STATUS_DONE`, `ISSUE_SYNC_REQUEST_COMMENTS_ENABLED` (mặc định `false`), `ISSUE_SYNC_ORCA_BASE_URL`.
- **Chưa kiểm chứng trên Jira thật.** Và `executing`/`completed` chưa đạt được bằng luồng thật cho tới khi các giai đoạn thực thi của Request xong
  ([tài liệu Request](../request/README.md)).

## 5. Chưa kiểm chứng / điểm yếu đã biết

- **Nhiều site Jira dùng chung tiền tố khoá.** Đồng bộ trạng thái truyền site của liên kết làm `workspace_id` nên chọn đúng kết nối; liên kết cũ không có site vẫn dùng cách chọn theo (tenant, user, provider).
- **Workflow Jira tuỳ biến.** Chuyển trạng thái so khớp theo **tên** trạng thái đích ("In Progress", "In Review", "Done"). Workflow không có đúng tên đó thì ghi log rồi bỏ, không chuyển bừa.
- **Chưa chạy trên Jira thật.** `task_sources` trên server dev hiện có 0 dòng.

## 6. Hướng cải thiện (chưa làm)

1. Đưa issue/site vào sự kiện PR để đồng bộ In Review/Done theo site.
2. Điền site khi tạo worktree từ issue qua `CreateWorktreeFromIssue`.

## 7. Nơi trong code

| Chủ đề | File |
|---|---|
| Kiểu ngữ cảnh nguồn task | `frontend/src/shared/task-source-context.ts` |
| Ngữ cảnh Jira của trang Tasks | `frontend/src/renderer/src/components/TaskPage.tsx` (`jiraTaskSourceContext`, `fallbackTaskSourceProjectId`) |
| Chọn project/repo trong composer | `frontend/src/renderer/src/hooks/useComposerState.ts` |
| Chuyển khoá Jira vào `worktree.create` | `frontend/src/renderer/src/lib/linked-external-issue.ts` |
| Liên kết task ↔ issue | `backend-go/services/task-service/internal/usecase/create_task_from_source.go`, migration `0012_task_sources` |
| Lưu liên kết trên worktree và khai báo Jira của project | `backend-go/services/project-service` (`linked_issue_*`, `jira_project_key`, `jira_site_id`, migration `0031`) |
| Đồng bộ trạng thái | `backend-go/services/issue-status-sync`, `backend-go/services/issue-tracking-service/internal/adapter/jira/client.go` |
