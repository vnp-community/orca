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
- RPC `CreateTaskFromSource`, `GetTaskSource`; kênh WS `task.createFromSource`, `task.getSource`.

### 2.2 Dùng lại worktree theo issue
- `CreateWorktreeRequest` có thêm `linked_issue_provider/ref` (validate: đi cùng nhau, provider ∈ jira/linear/github/gitlab). `worktree.create` chuyển xuống; project-service đã lưu lineage và phát `worktree.created`.
- `WorktreeProvisioner.EnsureWorktree` tìm worktree `active` đã gắn issue của task (và chưa thuộc task khác) trước khi tạo mới. Lỗi tra cứu → quay về tạo mới.
- Chiều ngược lại: khi `task.execute` tự tạo worktree cho task có nguồn, request mang luôn `linked_issue_*` — nhờ đó issue cũng chuyển "In Progress" cho luồng bắt đầu từ task, và hai đường dùng chung một worktree.

### 2.3 Đồng bộ trạng thái về Jira
- Không viết mới: `issue-status-sync` đã nhận `worktree.created` qua JetStream (retry 3 lần, idempotent, kiểm tra cờ theo project). Việc ghi liên kết ở 2.2 là đủ để issue chuyển "In Progress" bất đồng bộ.

### 2.4 Engine 1 bền (lease + heartbeat + recovery)
- Migration `0013_execution_leases`: `execution_links.lease_expires_at / lease_owner / previous_status`.
- `ExecuteTask.WithExecutionLeases`: ghi lease **trước** khi dispatch; heartbeat gia hạn mỗi 30s (TTL 90s) cho tới khi run xong.
- `RecoverInterruptedExecutions` (vòng quét 30s): `ClaimExpired` nguyên tử đánh dấu link `failed` rồi trả task về `previous_status` — chỉ khi task còn `in_progress` **và** `active_execution_link_id` đúng link đó.
- Nhiều instance quét cùng lúc không nhận trùng: Postgres `UPDATE … RETURNING` + `FOR UPDATE SKIP LOCKED`; MySQL transaction `SELECT … FOR UPDATE SKIP LOCKED` rồi `UPDATE` (cần MySQL ≥ 8.0.1). Thời gian lease dùng đồng hồ DB.
- **Claim nguyên tử**: `ExecuteTask.WithExecutionClaim` đổi `status → in_progress` bằng compare-and-set (`UPDATE … WHERE status = <đã đọc>`); trong nhiều `Execute` đồng thời chỉ một cái thắng, các cái còn lại nhận `TASK_EXECUTE_ALREADY_IN_PROGRESS` và không ghi link/dispatch. Đóng race check-then-write cũ.
- Rollout an toàn: link không có lease (trước migration) không bao giờ bị quét; nếu ghi lease lỗi (migration chưa chạy) thì chỉ log và chạy như cũ.

### 2.5 Frontend
- Composer truyền khoá Jira (`jiraIdentifier`) vào `createWorktree` → `worktree.create` (chỉ runtime remote; chỉ Jira, để Linear/GitHub/GitLab giữ hành vi cũ).
- `TaskDetail` hiện badge nguồn ("Jira ENG-1", link http(s) tới issue) qua `task.getSource`; runtime không có RPC này thì không hiện gì.
- Cờ `experimentalJiraTaskLink` (Settings → Experimental, mặc định tắt): khi bật, sau khi tạo worktree từ Jira gọi `task.createFromSource` (best-effort, lỗi chỉ `console.warn`).

## 3. Triển khai
1. Chạy migration `0012` rồi `0013` cho task-service (Postgres và MySQL).
2. Deploy task-service, git-gateway-service, api-gateway; sau đó frontend.
3. Bật cờ Experimental cho nhóm thử nghiệm.
4. `issue-status-sync` phải đang chạy thì issue mới đổi trạng thái (repo chỉ có workflow CI cho service này, chưa thấy manifest deploy).

## 4. Hành vi cần biết
- Cờ đồng bộ ở project mặc định **bật** (`IssueStatusSyncEnabled: true`): mọi workspace Jira tạo trên web sẽ đẩy issue sang "In Progress" trừ khi project tắt.
- Recovery **không chạy lại** agent; nó chỉ trả task về trạng thái trước. Tiến trình agent trên dev server có thể vẫn chạy tới khi bị dọn; nếu hoàn tất muộn nó vẫn ghi `review`.
- Nếu `WithExecutionClaim` không được cấu hình (test/embedding khác), `Execute` quay về check-then-write như cũ.

## 5. Chưa làm / theo dõi
- UI cho DecisionGate (`ListPendingDecisionGates`/`ResolveGate`) và nguồn tự mở gate — vẫn chưa có.
- Tự tạo task cho mọi workspace Jira (hiện chỉ khi bật cờ).
- `ExecuteBatch` vẫn không có RPC nào gọi tới (code chết, ghi sẵn trong `dispatchDirectAgentAsync`).
- Test `ExecuteBatch` có data race trong `fakeExecutionLinkRepository` (`fakes_test.go`, không có mutex) khi chạy `go test -race`; có từ trước, không thuộc CR này.

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
- [ ] Chạy end-to-end với tiến trình task-service thật bị kill giữa lúc agent chạy (hiện mới mô phỏng ở mức DB + use case).
