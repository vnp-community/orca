# CR-TG-001 — OrcaTask Data Model Widening &amp; Structural Management

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-001 |
| **Tên** | Mở rộng `Task` domain/proto lên đúng field spec F37 + `GetSubtree` + `RecalculateProgress` + auto-block + comment CRUD + atomic `AddEdge` |
| **Loại** | Feature / Schema |
| **Priority** | P0 (nền tảng — mọi CR khác trong series cần field ở đây) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | Không — CR đầu tiên của series |
| **Áp dụng thiết kế** | [SOL-TG-01-task-graph-structural-management.md](../../../../specs/backend-go/bugs/logic-v1/solutions/SOL-TG-01-task-graph-structural-management.md) (465 dòng) — CR này KHÔNG thiết kế lại, chỉ đóng khung thành acceptance criteria + sequencing |
| **Tác động** | `backend-go/services/task-service/internal/domain/task.go`, `internal/usecase/add_edge.go`, `internal/adapter/postgres/repository.go`, `proto/orca/task/v1/task.proto`, migration mới |

---

## 1. Vấn đề

`docs/features/F37-task-graph-management.md` (dòng 36-91) định nghĩa `OrcaTask`
với ~20 field bao trùm classification, assignment, AI, access control, execution
tracking. Code thật hiện tại chỉ có 6:

```go
// backend-go/services/task-service/internal/domain/task.go:52-69
type Task struct {
    ID        string
    TenantID  string
    Title     string
    Status    Status
    ParentID  *string
    ProjectID string
}
```

khớp 1-1 với `task.proto:59-65` — không có `description, type, priority, labels,
assignee_id, reporter_id, owner_id, due_date, estimated_hours, actual_hours,
prompt_template, ai_context, ai_plan_json, visibility, worktree_id,
agent_session_id, workflow_exec_id`.

Hệ quả cụ thể đã xác nhận trong code:

1. **Status enum thiếu 4/7 giá trị** — `validStatus` (`task.go:71-78`) chỉ chấp
   nhận `open/in_progress/done/cancelled`; spec yêu cầu thêm `backlog/todo/
   blocked/review`. Không có "review" nghĩa là auto-advance sau khi Run Agent
   xong (F37 spec dòng 265-279) không có trạng thái đích hợp lệ để chuyển tới.
2. **Không có progress cascade** — `grep -rn "progress" backend-go/services/task-service`
   chỉ khớp chuỗi literal `"in_progress"`; không có `CalculateProgress`/
   `RecalculateProgress` nào. Spec yêu cầu `done_subtasks/total_subtasks`
   hiển thị real-time trên mỗi task cha.
3. **Không có auto-block khi dependency chưa xong** — thêm 1 `depends_on` edge
   trỏ tới task chưa `done` phải tự chuyển task đích sang `blocked`; hiện
   không có logic này.
4. **`task.task_comments` table tồn tại nhưng chết** — có RLS đầy đủ
   (`migrations/0001_init.up.sql:82-93`) nhưng `grep -rn "task_comments"
   internal/` = 0 kết quả Go code nào đọc/ghi bảng này.
5. **`AddEdge` có race cycle-check-rồi-ghi** — `internal/usecase/add_edge.go:41-54`
   tự nhận trong comment: kiểm tra cycle rồi INSERT không nằm trong cùng 1
   transaction, 2 request `AddEdge` chạy đồng thời có thể cùng pass cycle-check
   rồi cùng ghi, tạo cycle thật.
6. **Không có `GetSubtree` public RPC** — chỉ có `GetAncestors` nội bộ
   (`postgres/repository.go:101-143`, recursive CTE dùng riêng cho
   `ResolvePermission`), khiến frontend phải tự filter toàn bộ task đã load
   theo `parentId` phía client (xem CR-TG-007 §C1) thay vì gọi 1 RPC scalable.

## 2. Giải pháp đề xuất (theo SOL-TG-01)

### 2.1 Schema — additive-only migration

```sql
-- backend-go/services/task-service/migrations/000N_task_field_widening.up.sql
ALTER TABLE task.tasks
  ADD COLUMN description      TEXT,
  ADD COLUMN type              TEXT,
  ADD COLUMN priority          TEXT,
  ADD COLUMN labels            TEXT[] DEFAULT '{}',
  ADD COLUMN assignee_id       TEXT,
  ADD COLUMN reporter_id       TEXT,
  ADD COLUMN owner_id          TEXT,
  ADD COLUMN due_date          TIMESTAMPTZ,
  ADD COLUMN estimated_hours   NUMERIC,
  ADD COLUMN actual_hours      NUMERIC,
  ADD COLUMN prompt_template   TEXT,
  ADD COLUMN ai_context        JSONB,
  ADD COLUMN ai_plan_json      JSONB,
  ADD COLUMN visibility        TEXT DEFAULT 'private',
  ADD COLUMN worktree_id       TEXT,
  ADD COLUMN agent_session_id  TEXT,
  ADD COLUMN workflow_exec_id  TEXT,
  ADD COLUMN done_subtasks     INT DEFAULT 0,
  ADD COLUMN total_subtasks    INT DEFAULT 0;

-- status CHECK constraint mở rộng thêm backlog/todo/blocked/review
```

`owner_id` mặc định = người tạo task (không NULL) — CR-TG-003's owner
short-circuit cần cột này tồn tại trước.

### 2.2 `RecalculateProgress` — cascade bottom-up

```go
// internal/usecase/recalculate_progress.go (mới)
// Gọi sau MỌI thay đổi status của 1 task có parent — cascade lên toàn bộ
// tổ tiên bằng 1 câu WITH RECURSIVE duy nhất, không N+1 query theo từng level.
func (u *RecalculateProgress) Execute(ctx context.Context, taskID string) error {
    return u.repo.RecalculateAncestorProgress(ctx, taskID) // WITH RECURSIVE UPDATE
}
```

Gọi từ `UpdateTask`/`ExecuteTask`'s hoàn tất mọi lần status đổi thành
`done`/`cancelled`.

### 2.3 Auto-block khi thêm dependency edge

```go
// add_edge.go — sau khi AddEdge(fromID, toID, EdgeKindDependsOn) thành công:
if edgeKind == EdgeKindDependsOn {
    fromTask, _ := repo.Get(ctx, fromID)
    if fromTask.Status != StatusDone {
        _ = repo.UpdateStatus(ctx, toID, StatusBlocked) // toID chờ fromID
    }
}
```

### 2.4 `GetSubtree` RPC mới + `AddComment`/`ListComments`

```protobuf
// task.proto
rpc GetSubtree(GetSubtreeRequest) returns (GetSubtreeResponse);
rpc AddComment(AddCommentRequest) returns (AddCommentResponse);
rpc ListComments(ListCommentsRequest) returns (ListCommentsResponse);
```

`GetSubtree` dùng 1 `WITH RECURSIVE` duy nhất trả toàn bộ subtree (không phải
load-all-rồi-filter-client như frontend đang làm) — CR-TG-007's Tree/Board view
sẽ gọi RPC này thay vì `task.list`.

### 2.5 `AddEdge` atomic — 1 transaction

```go
// add_edge.go — bọc cycle-check + INSERT trong cùng 1 tx, dùng
// `SELECT ... FOR UPDATE` trên node liên quan để chặn race giữa 2 request
// AddEdge đồng thời trên cùng subtree.
func (u *AddEdge) Execute(ctx context.Context, fromID, toID string, kind EdgeKind) error {
    return u.txRunner.RunInTx(ctx, func(tx Tx) error {
        wouldCycle, err := u.validator.WouldCreateCycleTx(tx, fromID, toID)
        if err != nil || wouldCycle { return ErrCycleDetected }
        return u.repo.InsertEdgeTx(tx, fromID, toID, kind)
    })
}
```

## 3. Rủi ro / Không thuộc phạm vi

- Không thiết kế lại UI hiển thị các field mới — đó là CR-TG-007.
- Không implement AI decompose dùng field mới (`description`/`aiContext`/
  `promptTemplate`) — đó là CR-TG-002, chỉ phụ thuộc field CR này thêm.
- Migration additive-only, không backfill dữ liệu cũ theo field mới (mặc định
  NULL/`''`/`0` là chấp nhận được cho task đã tồn tại).
- Không đổi `EdgeKind` enum hiện có (`parent_child`/`depends_on`) — chỉ sửa
  cách `AddEdge` ghi, không đổi model quan hệ.

## Acceptance Criteria

- [ ] `Task` domain struct + proto message có đủ ~20 field theo spec F37 §Data
      Model, migration additive-only chạy được trên DB đã có data.
- [ ] `Status` enum có đủ 7 giá trị (`backlog/todo/in_progress/blocked/review/
      done/cancelled`), `validStatus` cập nhật tương ứng.
- [ ] `RecalculateProgress` cascade đúng lên mọi tổ tiên bằng 1 câu query,
      không N+1; `done_subtasks/total_subtasks` khớp thực tế sau mỗi lần
      status con đổi.
- [ ] Thêm `depends_on` edge trỏ tới task chưa `done` → task đích tự chuyển
      `blocked`; task nguồn hoàn tất → task đích tự thoát `blocked` (nếu không
      còn dependency chưa xong nào khác).
- [ ] `AddEdge` chạy 2 request đồng thời tạo cycle trên cùng subtree → chỉ 1
      request thành công, request kia nhận `CYCLE_DETECTED` (test race
      condition bằng goroutine đồng thời).
- [ ] `GetSubtree` trả đúng toàn bộ subtree bằng 1 query, có test trên cây ≥ 4
      cấp.
- [ ] `AddComment`/`ListComments` hoạt động, tôn trọng RLS đã có sẵn trên
      `task.task_comments`.
- [ ] `detect_changes()`/`gitnexus_impact` xác nhận không phá vỡ symbol nào
      đang gọi `Task` struct cũ (rà soát toàn bộ call site trước khi đổi field).
