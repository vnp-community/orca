# CR-PW-007 — Worktree creation & visibility không đáng tin cậy: relay lỗi mất chi tiết, sidebar mất worktree hợp lệ

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-007 |
| **Tên** | Chuỗi lỗi `WORKTREE_CREATE_FAILED`/`INFRA_AGENT_EXEC_FAILED` mất thông tin nguyên nhân thật; sidebar "Project Workspace (Beta)" có thể ẩn 1 worktree hợp lệ đã tạo thành công |
| **Loại** | Bug Fix / Reliability |
| **Priority** | 🔴 P0 — chặn hoàn toàn việc tạo/thấy worktree cho repo mới thêm vào project, không có thông báo lỗi đúng nguyên nhân |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | 🟡 Một phần đã fix (BUG-010 xong+deployed), phần còn lại 🔲 Proposed |
| **Tác giả** | Investigation từ báo cáo user thật trên `b15.openledger.vn` (repo "aiops"/"aiops-v3", nhiều lần `WORKTREE_CREATE_FAILED` liên tiếp) |
| **Tác động HLD** | `project-service`, `git-gateway-service`, `infra-fleet-service`, agent (`agent/src/relay/`) |
| **Tác động Features** | Project Workspace (Beta) — tạo repo mới trong project, tạo worktree, sidebar hiển thị worktree |

---

## Bối cảnh & Vấn đề gốc

Điều tra live trên `b15.openledger.vn` (không phải suy đoán) phát hiện **4 lớp lỗi xếp chồng lên nhau** trên cùng 1 luồng (thêm repo mới vào project → gán dev server → tạo worktree → thấy worktree trong sidebar):

### 1. `PROJECT_DEV_SERVER_LOOKUP_FAILED` — ✅ ĐÃ FIX (BUG-010/SOL-011, deployed 2026-09-15)

`InfraFleetDevServerLister.Exists`/`InfraFleetHostnameResolver.Hostname` (project-service) gọi sang infra-fleet-service mà không forward tenant metadata → `infra-fleet-service` fail closed `INFRA_NO_TENANT` → mọi lần gán dev server cho repo/project đều fail. Xem [BUG-010](../../../../specs/backend-go/bugs/missing-v2/BUG-010-rebind-repo-dev-server-lookup-failed-missing-tenant-metadata.md) / [SOL-011](../../../../specs/backend-go/bugs/missing-v2/solutions/SOL-011-dev-server-lister-forward-tenant-metadata.md).

### 2. `handleGitExec` (agent) nuốt exit code ≠ 0 — 🔲 Chưa fix

`agent/src/relay/agent-git-handler.ts`'s `handleGitExec` luôn trả JSON-RPC `result` (kể cả khi `git` thất bại thật, ví dụ `Permission denied`) — không bao giờ trả `error` trừ khi spawn-level thất bại (ENOENT thật). `RelayExecutor.CreateWorktree` (backend-go) truyền `nil` làm result pointer cho bước `git.worktree.add`, nên không đọc được `exitCode`/`stderr` — coi bước fail thật là thành công, rồi chạy tiếp bước `git rev-parse HEAD` trên 1 path **chưa từng được tạo**, sinh ra lỗi thứ cấp gây hiểu lầm (`"spawn git ENOENT"`, thực chất do `cwd` không tồn tại, không phải do thiếu binary git). Xem [BUG-AG-WT-001](../../../../specs/agent/bugs/worktree-management/BUG-AG-WT-001-git-exec-swallows-nonzero-exit-and-stale-show-ref-comment.md).

### 3. `mergeDetectedWorktrees` (api-gateway) disk-first — 🟡 Đã re-đánh giá lại, KHÔNG phải bug đơn giản

Ban đầu chẩn đoán đây là bug (disk-first bỏ qua DB) — **đọc kỹ lại comment trong chính code xác nhận đây là thiết kế có chủ đích**: kết quả `worktree.detectedList` được đánh dấu `authoritative: true`, và frontend dùng "không có trong list này" làm tín hiệu để **xoá worktree đã bị xoá thật** khỏi state cục bộ. Thêm union (hiện cả row DB không có trên disk) sẽ **phá vỡ đúng cơ chế đó** — không nên làm theo hướng union như đề xuất ban đầu.

Câu hỏi thật còn lại, hẹp hơn: `worktree.detectedList`'s handler dùng `errgroup.Wait()` — nếu `DetectWorktrees` lỗi thật, cả handler trả lỗi, không bao giờ tới `mergeDetectedWorktrees` với dữ liệu thiếu. Vậy tại sao 1 lần `DetectWorktrees` **thành công** lại thiếu 1 worktree mà `git worktree list --porcelain` chạy tay qua SSH lại thấy đủ? Cần log thật ở lần tái hiện tiếp theo mới trả lời được — chưa đủ bằng chứng để code fix ngay. Xem phần "Correction" trong [BUG-011](../../../../specs/backend-go/bugs/missing-v2/BUG-011-detected-worktrees-merge-disk-first-drops-db-rows.md).

### 4. Hạ tầng dev-server thật có vấn đề (không phải bug code, nhưng lặp lại)

Trong lúc điều tra, phát hiện thêm (đã tự khắc phục thủ công trên `test-01`, không phải fix trong code):
- File `.git` bị lẫn chủ sở hữu `ubuntu`/`luatnc` (2 OS user khác nhau cùng thao tác 1 repo) → `Permission denied`.
- `/opt` (thư mục cha nơi Orca tạo worktree mới làm sibling) không cho `ubuntu` ghi → mọi worktree mới tạo dưới `/opt/` trực tiếp đều fail, bất kể repo nào.
- 2 tiến trình `agent.js` cùng chạy trong 1 systemd cgroup (restart hỏng để sót tiến trình cũ) → routing không ổn định.

Đây không phải bug code cần fix trong CR này, nhưng **worth ghi nhận vào runbook onboard dev-server mới** — nếu lặp lại ở server khác sẽ gây đúng những triệu chứng khó chẩn đoán y hệt.

## Giải pháp đề xuất (tổng hợp)

1. **BUG-010** — đã xong, đã deploy. Không cần làm gì thêm ngoài verify cuối bằng UI thật (chưa có ai xác nhận qua UI, chỉ verify qua log + test).
2. **BUG-AG-WT-001**: `handleGitExec`/`git.worktree.add` cần phân biệt được "git thất bại thật" (exit code ≠ 0) và trả JSON-RPC `error` cho các subcommand có tính "phải thành công" (`worktree add`, `commit`, `push`...) — không áp dụng cho các subcommand mà exit code khác 0 là dữ liệu hợp lệ (`diff --exit-code`, `show-ref` dùng để check tồn tại). Cần thiết kế riêng theo subcommand, chưa làm vội.
3. **BUG-011**: KHÔNG sửa `mergeDetectedWorktrees` (thiết kế có chủ đích, xem correction trong bug file). Bước tiếp theo là **thu thập log thật** ở lần tái hiện kế tiếp để biết vì sao `DetectWorktrees` thành công nhưng thiếu dữ liệu — sau đó mới quyết định fix ở đâu (nhiều khả năng là `git-gateway-service`'s `DetectWorktrees`/relay, không phải hàm merge).
4. Viết lại/bổ sung vào tài liệu **runbook onboard dev-server** (không thuộc code): checklist ownership `.git`, quyền ghi `/opt` (hoặc thư mục worktree gốc tương đương), kiểm tra không có tiến trình `agent.js` trùng lặp sau mỗi lần restart service.

## Không thuộc phạm vi CR này

- `GITGATEWAY_STATUS_FAILED` (lỗi tải git status sau khi đã có worktree) — CR riêng, xem [CR-PW-008](./CR-PW-008-agent-session-persistence-and-observability-gaps.md) (observability) — nguyên nhân chưa xác định được vì thiếu log, không phải cùng lỗi với CR này.
- Việc chọn account AI cho từng session (`agent.switchAccount` không có UI) — xem [CR-PW-009](./CR-PW-009-close-backend-only-ui-gaps-in-project-workspace.md).

## Liên quan

- [BUG-010](../../../../specs/backend-go/bugs/missing-v2/BUG-010-rebind-repo-dev-server-lookup-failed-missing-tenant-metadata.md), [BUG-011](../../../../specs/backend-go/bugs/missing-v2/BUG-011-detected-worktrees-merge-disk-first-drops-db-rows.md), [BUG-AG-WT-001](../../../../specs/agent/bugs/worktree-management/BUG-AG-WT-001-git-exec-swallows-nonzero-exit-and-stale-show-ref-comment.md)
- [CR-PW-001](./CR-PW-001-git-status-shape-mismatch.md) — cùng khu vực tính năng, lớp lỗi khác (frontend type-cast, đã fix từ trước, không liên quan trực tiếp)
