# CR-TG-008 — Liên kết Jira ↔ OrcaTask ↔ Worktree và Engine 1 (direct_agent) bền khi restart

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-008 |
| **Tên** | Nối "Start work" từ Jira với task/worktree; làm Engine 1 phục hồi được sau khi tiến trình chết |
| **Loại** | Feature / Reliability |
| **Priority** | P1 |
| **Ngày tạo** | 2026-10-02 |
| **Trạng thái** | 🟡 Backend + frontend đã code và test; chưa chạy end-to-end trên môi trường thật |
| **Tác động** | `task-service` (domain, usecase, adapter postgres/mysql, migration 0012–0013), `git-gateway-service` (`CreateWorktreeRequest`), `api-gateway` (wscompat), `frontend` (composer, store, Settings → Experimental) |

## 1. Vấn đề

1. **Hai đường tách rời.** "Start work" từ Jira mở composer tạo worktree (`worktree.create`) mà không có `OrcaTask`; `task.execute` thì tạo worktree riêng. Cùng một issue có thể sinh hai worktree và không có liên kết Jira nào ở phía task.
2. **Engine 1 mất việc khi restart.** `ExecuteTask.dispatchDirectAgentAsync` chạy agent trong một goroutine của tiến trình `task-service`. Restart/deploy giữa chừng để task kẹt `in_progress` vĩnh viễn, không RPC nào gỡ được.

## 2. Giải pháp đã triển khai

### 2.1 Lưu nguồn của task (`task.task_sources`)
- Bảng riêng (migration `0012_task_sources`), không thêm cột vào `task.tasks` để không đụng các câu SELECT/INSERT hiện có.
- Khoá unique `(tenant, project, provider, ref)`; `project_id` NULL vẫn được dedupe (Postgres `COALESCE`, MySQL generated column).
- `CreateTaskFromSource`: idempotent; thua race thì xoá task thừa và trả task của bên thắng. Trả task có sẵn cho người gọi thứ hai vẫn kiểm tra quyền đọc.
- RPC `CreateTaskFromSource`, `GetTaskSource`; kênh WS `task.createFromSource`, `task.getSource`, `task.generateAgentPrompt`.
- **Quyền:** `GetTask` chỉ kiểm tra tenant, không kiểm tra grant. Vì vậy trả task có sẵn (`CreateTaskFromSource`) và `GetTaskSource` gọi `ResolvePermission(read)` tường minh; `GenerateAgentPrompt` cần `read` để xem trước và `write` để lưu (trước đây không kiểm tra gì, trong khi `Save` ghi `PromptTemplate` mà agent sẽ chạy).

### 2.2 Dùng lại worktree theo issue
- `CreateWorktreeRequest` có thêm `linked_issue_provider/ref` (validate: đi cùng nhau, provider ∈ jira/linear/github/gitlab). `worktree.create` chuyển xuống; project-service đã lưu lineage và phát `worktree.created`.
- `WorktreeProvisioner.EnsureWorktree` tìm worktree `active` đã gắn issue của task (và chưa thuộc task khác) trước khi tạo mới. Lỗi tra cứu → quay về tạo mới.
- Chiều ngược lại: khi `task.execute` tự tạo worktree cho task có nguồn, request mang luôn `linked_issue_*` — nhờ đó issue cũng chuyển "In Progress" cho luồng bắt đầu từ task, và hai đường dùng chung một worktree.

### 2.3 Đồng bộ trạng thái về Jira (đã sửa 2026-10-03; chưa bật trên server)
Bản rà soát 2026-10-03 phát hiện luồng này **không cập nhật được Jira** (5 lỗi độc lập). Đã sửa:
| Lỗi | Sửa |
|---|---|
| Consumer chỉ gửi `tenant`, `UpdateIssue` đòi user và credential Jira lưu theo `(tenant, user)` | Sự kiện `worktree.created/deleted` mang thêm `actor_user_id` (project-service lấy từ ngữ cảnh; git-gateway và task-service chuyển user cho `RecordWorktree*`/`CreateWorktree`). Consumer gọi tracker bằng chính người đó. Sự kiện không có actor (cũ, hoặc tạo từ nơi không có user) bị **bỏ qua có log**, không thử lại vô ích. Field là `omitempty` nên tương thích ngược. |
| Adapter Jira bỏ qua `workflow_state_id` nhưng báo thành công | `jira.Client.UpdateIssue` giờ chọn transition khả dụng (id, rồi tên status đích, rồi tên transition) và `POST /issue/{id}/transitions`. Đã ở đúng status thì không làm gì. Không có đường tới status đó → `ErrTransitionUnavailable`; Jira từ chối → trả lỗi. Không còn "thành công giả". Chỉ cập nhật trạng thái thì không `PUT` fields rỗng. |
| `had_open_pr` luôn `false`, xoá worktree sẽ "Cancelled" | **Bỏ hẳn mapping `worktree.deleted → Cancelled/close`.** Xoá worktree là việc dọn dẹp thường ngày (kể cả sau khi PR đã merge) và không nói gì về issue; tự huỷ/đóng issue ở đây là mất dữ liệu trên tracker. |
| Có thể kéo lùi issue đã làm xong | `worktree.created → In Progress` chỉ áp dụng khi issue đang ở category `todo` (Jira `statusCategory` được chuẩn hoá `new/indeterminate/done → todo/in_progress/done` trong `GetIssue`). Đang `in_progress`, `done` hoặc không rõ → giữ nguyên. Không tra được category → thử lại rồi bỏ, **không** chuyển. |
| Provider chưa kiểm chứng | Chỉ **Jira** được đồng bộ. Linear cần UUID state (service gửi tên), GitHub chưa có đường credential theo user; hai provider đó bị bỏ qua có log thay vì lỗi 3 lần mỗi sự kiện. |

Còn lại (chưa làm, có chủ đích):
- Sự kiện PR không mang issue (`LinkedIssue*` luôn rỗng ở `CreatePullRequest/MergePullRequest`) và cũng không có actor, nên "In Review"/"Done" vẫn không kích hoạt. Cần parser closing-keyword hoặc tra issue từ worktree theo branch.
- `issue-status-sync` **chưa nằm trong deploy dev**. `Makefile SERVICES` đã có nó (build/vet/test). Không bật tự động vì `init-databases.sh` chỉ chạy khi khởi tạo volume lần đầu (server đang chạy sẽ thiếu DB) và `migrate.sh all` sẽ lỗi trên DB chưa tồn tại. Để bật:
  1. Tạo DB trên Postgres đang chạy (`CREATE DATABASE issuestatussync OWNER orca;`) rồi thêm `issuestatussync` vào `DATABASES` ở `deploy/dev/docker/postgres/init-databases.sh` cho môi trường mới.
  2. Chạy migration của service (`backend-go/services/issue-status-sync/migrations/postgres`) với DSN tới DB đó; thêm vào `SERVICES` của `migrate.sh`.
  3. Thêm `issue-status-sync` vào `ALL_SERVICES` của `build-local.sh` và một khối service trong `docker-compose.yml` theo mẫu `usage-service` (env: `DATABASE_DSN`, `NATS_URL`, `ISSUE_TRACKING_SERVICE_ADDR`, `SCM_INTEGRATION_SERVICE_ADDR`, `PROJECT_SERVICE_ADDR`; `depends_on` postgres, nats, issue-tracking-service, project-service).
  4. Stream JetStream `PROJECT` phải tồn tại (project-service tạo khi relay khởi động).
  Nên thử trên một project có Jira thật, đặt `IssueStatusSyncEnabled=false` ở các project còn lại cho tới khi yên tâm.

### 2.4 Engine 1 bền (lease + heartbeat + recovery)
- Migration `0013_execution_leases`: `execution_links.lease_expires_at / lease_owner / previous_status`.
- `ExecuteTask.WithExecutionLeases`: ghi lease **trước** khi dispatch; heartbeat gia hạn mỗi 30s (TTL 90s) cho tới khi run xong.
- `RecoverInterruptedExecutions` (vòng quét 30s): `ClaimExpired` nguyên tử đánh dấu link `failed` rồi trả task về `previous_status` — chỉ khi task còn `in_progress` **và** `active_execution_link_id` đúng link đó.
- Nhiều instance quét cùng lúc không nhận trùng: Postgres `UPDATE … RETURNING` + `FOR UPDATE SKIP LOCKED`; MySQL transaction `SELECT … FOR UPDATE SKIP LOCKED` rồi `UPDATE` (cần MySQL ≥ 8.0.1). Thời gian lease dùng đồng hồ DB.
- **Claim nguyên tử**: `ExecuteTask.WithExecutionClaim` đổi `status → in_progress` bằng compare-and-set (`UPDATE … WHERE status = <đã đọc>`); trong nhiều `Execute` đồng thời chỉ một cái thắng, các cái còn lại nhận `TASK_EXECUTE_ALREADY_IN_PROGRESS` và không ghi link/dispatch. Đóng race check-then-write cũ.
- **Dọn task kẹt `in_progress`** (vòng quét chung, mỗi lần trả task về đều là compare-and-set trên link đang active):
  - run direct_agent hết lease → trả về `previous_status` (không có thì `open`);
  - run direct_agent cũ không có lease (trước migration 0013), quá 30 phút (lớn hơn trần 15 phút của executor);
  - task mồ côi: link đã kết thúc mà task vẫn `in_progress` — direct_agent `completed` → `review` (chỉ mất bước ghi hoàn tất), `failed` → `previous_status`. Với Engine 2/3 chỉ link `failed` mới tính là kết thúc, vì `Execute` đánh dấu link `completed` ngay sau khi dispatch (chỉ là ghi sổ, run vẫn đang chạy);
  - task `in_progress` không có link nào (`active_execution_link_id IS NULL`, vd. từ trước khi có `execution_links`), `updated_at` quá 30 phút (`UnlinkedGrace`, đồng hồ DB) → `open` (không biết trạng thái trước). Một câu `UPDATE` compare-and-set (`status='in_progress' AND link IS NULL AND updated_at < now()-grace`); task healthy chỉ ở trạng thái này vài giây giữa claim và ghi link nên grace lớn không đụng tới. Task có link không bao giờ bị sweep này chạm vào.
- **Engine 2/3 thất bại không còn kẹt `in_progress`:** `ReportTaskExecutionResult` trả task về `previous_status` (ghi trên link lúc dispatch, `open` nếu thiếu). Comment cũ "chưa có trạng thái blocked" đã lỗi thời — `blocked` có trong DB từ migration 0003.
- **Chặn `prompt` override trên task không phải direct_agent** (`TASK_EXECUTE_PROMPT_UNSUPPORTED`): vòng spec/duyệt/code gửi prompt theo pha, nhưng coordinator bỏ qua nó và chạy phần implement, tức là âm thầm bỏ qua bước duyệt spec.
- Rollout an toàn: link không có lease (trước migration) không bao giờ bị quét; nếu ghi lease lỗi (migration chưa chạy) thì chỉ log và chạy như cũ.

### 2.5 Frontend
- Composer truyền khoá Jira (`jiraIdentifier`) vào `createWorktree` → `worktree.create` (chỉ runtime remote; chỉ Jira, để Linear/GitHub/GitLab giữ hành vi cũ).
- `TaskDetail` hiện badge nguồn ("Jira ENG-1", link http(s) tới issue) qua `task.getSource`; runtime không có RPC này thì không hiện gì.
- Cờ `experimentalJiraTaskLink` (Settings → Experimental, mặc định tắt): khi bật, sau khi tạo worktree từ Jira gọi `task.createFromSource` (best-effort, lỗi chỉ `console.warn`).

## 3. Triển khai
1. Chạy migration `0012` rồi `0013` cho task-service (Postgres và MySQL).
2. Deploy task-service, git-gateway-service, api-gateway; sau đó frontend.
3. Bật cờ Experimental cho nhóm thử nghiệm.
4. Cập nhật trạng thái Jira: code đã sửa (2.3) nhưng service chưa được triển khai; làm theo 4 bước ở 2.3 để bật.

## 4. Hành vi cần biết
- Cờ `IssueStatusSyncEnabled` mặc định bật; khi `issue-status-sync` được triển khai, mọi worktree tạo từ issue Jira (có actor) sẽ đẩy issue đang `todo` sang "In Progress". Xoá worktree **không** đổi gì trên Jira.
- Dọn worktree không báo cho task-service nên `task.worktree_id` giữ id cũ. `EnsureWorktree` giờ coi `NotFound` từ project-service là "đã bị xoá" và tạo worktree mới (lưu id mới); các lỗi khác (không kết nối, từ chối quyền) vẫn dừng, không đoán.
- Recovery **không chạy lại** agent; nó chỉ trả task về trạng thái trước. Tiến trình agent trên dev server có thể vẫn chạy tới khi bị dọn; nếu hoàn tất muộn nó vẫn ghi `review`.
- Nếu `WithExecutionClaim` không được cấu hình (test/embedding khác), `Execute` quay về check-then-write như cũ.

## 5. Chưa làm / theo dõi
- UI cho DecisionGate (`ListPendingDecisionGates`/`ResolveGate`) và nguồn tự mở gate — vẫn chưa có.
- Tự tạo task cho mọi workspace Jira (hiện chỉ khi bật cờ).
- Đã xoá `ExecuteBatch` (code chết, không có caller production; frontend tự điều phối batch qua `task.execute`) cùng test của nó — data race trong `fakeExecutionLinkRepository` biến mất, `go test -race` qua toàn bộ. `domain.TopologicalWaves` còn lại (có test riêng, chưa có caller).
- `GenerateAgentPrompt` mới có kênh WS; chưa có nút trên UI (`TaskPromptEditor.tsx` đang có thay đổi chưa commit khác).

## Acceptance Criteria
- [x] Hai lần "start work" trên cùng issue/project trả cùng một task (test usecase + test tích hợp Postgres/MySQL).
- [x] `worktree.create` chuyển `linkedIssueProvider/Ref`; request cũ không gửi thì trường vẫn nil.
- [x] `EnsureWorktree` dùng lại worktree gắn issue, bỏ qua worktree thuộc task khác/không active.
- [x] Lease hết hạn → link `failed`, task về trạng thái trước; task đã xong/đã dispatch lại thì không bị đụng.
- [x] 6 sweeper đồng thời không nhận trùng link (Postgres và MySQL).
- [x] Lỗi ghi lease không làm hỏng run.
- [x] 12 lệnh claim đồng thời chỉ một thắng (Postgres và MySQL).
- [x] Khôi phục trên DB thật: claim → lease hết hạn → quét → task về `review`, link `failed`, quét lần hai không làm gì.
- [x] Worktree do task tạo mang `linked_issue_*`; task không nguồn thì không gửi.
- [x] Task kẹt từ trước migration (không lease, quá 30 phút) và task mồ côi được trả về trạng thái hợp lệ; link `completed` của Engine 2/3 không bị coi là đã xong (test tích hợp Postgres + MySQL).
- [x] Engine 2/3 báo thất bại → task về trạng thái trước, không kẹt.
- [x] Người không có grant không đọc được task/nguồn của người khác và không gọi được `GenerateAgentPrompt`.
- [x] Xoá worktree không còn nằm trong mapping: không bao giờ Cancelled/close issue (test).
- [x] Adapter Jira chuyển trạng thái thật, không thành công giả, không kéo lùi issue (test với Jira giả).
- [x] Task có `worktree_id` đã bị dọn vẫn chạy được: tạo worktree mới; lỗi khác vẫn fail closed.
- [ ] Bật `issue-status-sync` và chạy thử trên Jira thật (chưa).
- [ ] Chạy end-to-end với tiến trình task-service thật bị kill giữa lúc agent chạy (hiện mới mô phỏng ở mức DB + use case).
