# BE-DB-SOL-017: Dialect capability layer + MySQL/TiDB adapter cho `infra-fleet-service`

> **✅ DONE (2026-09-12).** Áp dụng pattern BE-DB-SOL-001/002
> (`usage-service`, pilot) cho service **LỚN NHẤT** của toàn bộ rollout
> 15-service (15 repository file, 62 migration — gấp ~15 lần scope pilot).
> Migration + adapter code cho **CẢ 15/15 repository unit đã xong** (build/
> vet sạch, interface conformance đã assert). **`cmd/server/main.go` đã
> wire dialect switch** (2026-09-12, dùng kỹ thuật union interface `repoAll`
> giống task-service — xem §6 đã cập nhật cho cách làm thật và 2 lỗi build
> gặp phải khi wiring). Test tích hợp: **MySQL 23/23 PASS thật** (2 bug
> adapter thật phát hiện + sửa trong lượt 2026-09-12 — xem §7); **Postgres
> 35/37 PASS thật** (2 FAIL còn lại là bug tiền tồn tại KHÔNG liên quan
> CR-DB-002/003, flag không sửa — xem §7) — xem TASK-BE-DB-022's "Cập nhật
> (2026-09-12)" cho chi tiết đầy đủ, minh bạch từng bug.

**CR:** CR-DB-002, CR-DB-003
**Service:** `backend-go/services/infra-fleet-service`
**Depends on:** `backend-go/common/dbcapability` (dùng lại nguyên vẹn,
không sửa), `backend-go/common/testutil.StartMySQL` (dùng lại nguyên vẹn)
**Task tương ứng:** [TASK-BE-DB-022](../tasks/TASK-BE-DB-022-infra-fleet-service-mysql-rollout.md)

---

## 1. Audit thật trước khi implement — quy mô thật xác nhận đúng ROLLOUT-TRACKING.md

`find`/`grep` trực tiếp (không suy đoán) xác nhận đúng con số audit gốc:

- **15 file trong `internal/adapter/postgres/`** (không tính `_test.go`),
  implement tổng cộng **~20 port interface** khác nhau trong
  `internal/usecase` (phần lớn định nghĩa trong `ports.go`, 1 port —
  `EphemeralVmRuntimeRepository` — định nghĩa ở `list_ephemeral_vm_runtimes.go`
  thay vì `ports.go`, phát hiện khi enumerate đầy đủ trước khi viết code —
  xem §2's checklist). `repository.go` (932 dòng) là 1 file NHƯNG chứa 3
  struct (`Repository`, `SshTargetStore`, `PortForwardStore`) implement
  ~9 port khác nhau — file phức tạp nhất trong TOÀN BỘ rollout 15-service.
- **62 file migration = 31 cặp up/down** — số thứ tự nhảy từ 0006 sang
  0013 (0007-0012 không tồn tại trong nhánh này, xác nhận qua `ls`, không
  phải lỗi audit).
- **JSONB**: 3 file (`0025_outbox`, `0031_agent_status_outbox`,
  `0018_fleet_definitions`).
- **RLS**: 9 bảng có policy (`dev_servers`, `ssh_targets`, `connections`,
  `agent_sessions`, `agent_tokens`, `terminal_scrollback_snapshots`,
  `queued_prompts`, `dev_server_groups`, `dev_server_group_grants`,
  `dev_server_access_requests`, `terminal_sessions` — 11 thật, đếm lại
  chính xác ở §5).
- **`gen_random_uuid()`**: dùng ở 10 migration, nhưng — giống MỌI service
  trước trong rollout này — `grep "INSERT INTO.*id.*tenant_id"` xác nhận
  **mọi INSERT đều truyền `id` tường minh**, không dựa vào DEFAULT. Bỏ
  hoàn toàn ở bản MySQL, không mất chức năng (cùng kết luận BE-DB-SOL-001 §1).
- **KHÔNG có `RETURNING`** search ban đầu trả về 0 — sai, audit lại bằng
  cách đọc trực tiếp `internal/adapter/postgres/*.go` cho thấy **RẤT
  NHIỀU** method dùng `RETURNING` (đã bị bỏ sót ở lần audit migration-only
  đầu tiên vì `RETURNING` chỉ xuất hiện trong Go code, không trong file
  `.sql`) — 13 method khác nhau across `repository.go`,
  `agent_token_repository.go`, `fleet_definition_repository.go`,
  `dev_server_access_request_repository.go`,
  `terminal_scrollback_snapshot_repository.go`,
  `queued_prompt_repository.go`, `ephemeral_vm_runtime_repository.go`,
  `browser_profile_repository.go` — tất cả dịch theo pattern
  BE-DB-SOL-005 §3.1 (UPDATE/INSERT rồi SELECT lại, không suy luận
  RowsAffected trừ khi giá trị LUÔN đổi thật — xem §4).
- **`ON CONFLICT DO UPDATE`**: 4 chỗ (`SshTargetStore.Upsert`,
  `Repository.UpsertFleetHealth`, `TerminalScrollbackSnapshotStore.Upsert`,
  `QueuedPromptStore.Upsert`, `EphemeralVmSshTargetStore.Upsert` — 5 thật).
- **`xmax != 0` trick**: 1 chỗ (`SshTargetStore.Upsert`) — dịch bằng
  MySQL's documented `INSERT ... ON DUPLICATE KEY UPDATE` affected-rows
  convention (1=insert, 2=updated-with-change, 0=updated-no-change) — xem
  §4.
- **`TEXT[]` (Postgres array)**: 2 cột (`ssh_targets.tags`,
  `dev_servers.tags`) — dịch sang `JSON`, cùng pattern
  ai-provider-service's `accounts.models` đã xác lập trong rollout này.
- **Postgres advisory lock** (`pg_try_advisory_lock`): 1 chỗ
  (`Repository.TryLock`, `PollLockPort`) — dịch sang MySQL's
  `GET_LOCK`/`RELEASE_LOCK` (named user-level lock, session-scoped, không
  cần hash tên như Postgres vì MySQL nhận thẳng string).

## 2. Checklist 15 repository unit — enumerate TRƯỚC khi viết code (CLAUDE.md yêu cầu)

Danh sách đầy đủ (khớp `ls internal/adapter/postgres/*.go` không tính
`_test.go`), xử lý tuần tự, mỗi unit verify `go build` ngay sau khi viết
(không viết hết 15 rồi mới debug hàng loạt):

1. `dev_server_group_repository.go` → `DevServerGroupStore`
2. `dev_server_group_grant_repository.go` → `DevServerGroupGrantStore`
3. `browser_profile_repository.go` → `BrowserProfileStore`
4. `dev_server_access_request_repository.go` → `DevServerAccessRequestStore`
5. `queued_prompt_repository.go` → `QueuedPromptStore`
6. `ephemeral_vm_ssh_target_repository.go` → `EphemeralVmSshTargetStore`
7. `agent_rate_limited_outbox_repository.go` → `AgentRateLimitedOutboxStore`
8. `agent_token_repository.go` → `AgentTokenStore`
9. `fleet_definition_repository.go` → `FleetDefinitionStore`
10. `terminal_scrollback_snapshot_repository.go` → `TerminalScrollbackSnapshotStore`
11. `terminal_session_repository.go` → `TerminalSessionStore`
12. `agent_session_repository.go` → `AgentSessionStore`
13. `ephemeral_vm_runtime_repository.go` → `EphemeralVmRuntimeStore`
14. `outbox.go` → `Repository`'s `EnqueueOutboxEvent`/`FetchUnpublished`/`MarkPublished`
15. `repository.go` (LỚN NHẤT) → `Repository` (9 port) + `SshTargetStore` (2 port) + `PortForwardStore` (1 port)

**Tất cả 15/15 đã implement, `go build ./internal/adapter/mysql/...` +
`go vet ./internal/adapter/mysql/...` sạch sau mỗi unit** — xác nhận thật,
không suy đoán (xem TASK-BE-DB-022 §Kết quả thực tế cho log build cuối
cùng). Thêm 24 dòng compile-time interface assertion
(`internal/adapter/mysql/shared.go`) khẳng định mọi struct implement đúng
port tương ứng — `internal/adapter/postgres` chỉ assert 2/15, package
`mysql` assert ĐẦY ĐỦ để bắt lỗi signature ngay lúc build thay vì lúc wire
`main.go`.

## 3. `impact()` trước khi sửa — CLAUDE.md/AGENTS.md bắt buộc

Chạy trên repo `orca` (GitNexus):

| Symbol | Direction | Risk | impactedCount |
|---|---|---|---|
| `run` (`cmd/server/main.go`) | upstream | **LOW** | 1 |
| `Repository` (struct, `internal/adapter/postgres/repository.go`) | upstream | **LOW** | 3 (`run`, module `Usecase`) |
| `NewSshTarget` (`internal/domain/ssh_target.go`), 2026-09-12 | upstream | **LOW** | 3 (`CreateSshTarget.Execute`, `DeployFleetDefinition.Execute`, `ImportFleetInventory.Execute`) |

Không sửa `ports.go`/interface nào hiện có — mọi adapter MySQL implement
lại đúng signature interface hiện có (khẳng định bằng 24 assertion ở §2).
Không có symbol nào HIGH/CRITICAL. (`FleetDefinitionStore.Update`/
`TerminalScrollbackSnapshotStore.Upsert`, sửa 2026-09-12: GitNexus không
resolve được symbol — package `mysql` có vẻ chưa index đầy đủ — xác nhận
blast radius bằng `grep` hẹp thay thế, xem §6/§7 và TASK-BE-DB-022's mục
"gitnexus".)

## 4. `internal/adapter/mysql/` — điểm dịch không tầm thường

### 4.1. `SshTargetStore.Upsert`'s `xmax != 0` → affected-rows convention

Postgres dùng `RETURNING id, (xmax != 0) AS updated` để biết 1 lệnh
`INSERT ... ON CONFLICT DO UPDATE` vừa insert hay update thật. MySQL không
có tương đương — nhưng **MySQL Reference Manual tài liệu hoá rõ**: với
`INSERT ... ON DUPLICATE KEY UPDATE`, `RowsAffected()` trả **1** nếu insert
mới, **2** nếu update 1 dòng có sẵn, **0** nếu dòng có sẵn nhưng giá trị y
hệt (không đổi gì). Vậy `updated := affected != 1` tái tạo đúng ngữ nghĩa
`xmax != 0` (khác pitfall RowsAffected của BE-DB-SOL-005 §3.1, vì đây là
ngữ nghĩa RIÊNG của `ON DUPLICATE KEY UPDATE`, không phải UPDATE thường).
ID thật của dòng (có thể khác `target.ID` khi conflict trúng dòng có sẵn)
được đọc lại bằng 1 SELECT sau đó — test
`TestSshTargetStore_Upsert_InsertThenUpdate` xác nhận cả 2 nhánh PASS thật
trên MySQL 8 (xem TASK-BE-DB-022 §Kết quả thực tế).

### 4.2. 13 method dùng `RETURNING` — áp dụng ĐÚNG nguyên tắc BE-DB-SOL-005 §3.1, không máy móc

Với MỌI method dùng `RETURNING` sau `UPDATE`, quyết định dịch dựa trên câu
hỏi: "giá trị mới có thể trùng giá trị cũ trên 1 lần gọi lại hợp lệ
không?" — nếu CÓ (ví dụ `FleetDefinitionStore.Update`'s optimistic-lock,
`Repository.UpdateStatus`'s connection state machine retry), **không bao
giờ đọc `RowsAffected()` để quyết định not-found** — luôn UPDATE rồi SELECT
lại. Nếu KHÔNG THỂ trùng (ví dụ `AgentTokenStore.Revoke`'s `revoked_at`
luôn chuyển từ NULL sang 1 timestamp thật, `TerminalSessionStore.Touch`'s
`last_active_at` luôn là `time.Now()` mới), `RowsAffected() == 0` an toàn
để báo not-found trực tiếp — đơn giản hơn, không cần round-trip SELECT
thừa. Ghi rõ lý do trong comment tại từng call site, không áp dụng máy móc
1 pattern cho tất cả 13 chỗ.

### 4.3. `TEXT[]` → `JSON`, bao gồm cả containment query

`ssh_targets.tags`/`dev_servers.tags`: `TEXT[]` → `JSON`, marshal/unmarshal
`[]string` ở tầng Go (`marshalTags`/`unmarshalTags`, `repository.go`).
Khác `ai-provider-service`'s `accounts.models` (chỉ đọc/ghi nguyên khối,
không query VÀO mảng), `ListByTag`'s `$2 = ANY(tags)` (Postgres array
membership) **CÓ** query vào bên trong mảng — dịch sang
`JSON_CONTAINS(tags, JSON_QUOTE(?))`, không có GIN-index-equivalent (full
scan chấp nhận được, cardinality tag/dev-server nhỏ per tenant).

### 4.4. `pg_try_advisory_lock` → `GET_LOCK`/`RELEASE_LOCK`

`PollLockPort.TryLock`: Postgres dùng session-scoped advisory lock
(`pg_try_advisory_lock(hashtext($1))`, non-blocking, giữ trên 1 connection
mượn từ pool). MySQL's `GET_LOCK(name, 0)`/`RELEASE_LOCK(name)` cùng ngữ
nghĩa named user-level lock, session-scoped — dùng `sql.Conn` (1 connection
giữ riêng) thay vì `sql.DB` trần, cùng lý do Postgres cần `pool.Acquire`.
Không cần hash tên (MySQL nhận thẳng string, dưới giới hạn 64 ký tự —
`devServerID` là UUID, luôn đủ ngắn).

### 4.5. `rows` là từ khoá dành riêng ở MySQL 8.0.19+

`terminal_scrollback_snapshots.rows` (tên cột trùng MySQL's window-function
frame keyword `ROWS`) — phát hiện thật khi chạy `migrate up` lần đầu
(`Error 1064` cụ thể, không suy đoán trước — xem §7). Dịch bằng
backtick-quote (`` `rows` ``) xuyên suốt migration + Go adapter.

### 4.6. `DEFAULT` trên cột TEXT cần biểu thức có ngoặc

MySQL 8.0.13+ từ chối `TEXT ... DEFAULT ''` (literal trần) — phải viết
`TEXT ... DEFAULT ('')`. Phát hiện thật khi chạy `migrate up` lần đầu
(`Error 1101`, không suy đoán trước — xem §7), sửa 16 cột across 15 file
migration. Với cột enum ngắn có CHECK constraint (`status`, `kind`,
`approval_status`, `connection_type`) đổi hẳn sang `VARCHAR(16..32)` thay
vì giữ `TEXT` + ngoặc, đơn giản và tự nhiên hơn.

## 5. Migration dialect-safe — 62 file, verify THẬT trên MySQL 8 (không chỉ đọc code)

```
backend-go/services/infra-fleet-service/migrations/
├── postgres/   (git mv, nội dung KHÔNG đổi, 62 file — 31 cặp)
└── mysql/      (mới, dialect-safe, 62 file)
```

Điểm dịch không lặp lại từ các service trước trong rollout:

- **Partial UNIQUE index** (2 chỗ: `connections`'s
  `idx_infra_connections_tenant_worktree` `WHERE worktree_id <> ''`,
  `agent_sessions`'s BR-AG-01 `idx_infra_agent_sessions_active_per_worktree_user`
  `WHERE status NOT IN (...)`) — dịch bằng generated STORED column
  (`CASE WHEN <điều kiện> THEN <key> ELSE NULL END`) + UNIQUE index trên
  cột đó, đúng kỹ thuật `ai-provider-service`'s
  `uq_accounts_one_default_per_dev_server_provider` đã xác lập (MySQL coi
  nhiều NULL trong UNIQUE index là phân biệt, giống Postgres) — **CHỨ
  KHÔNG PHẢI chỉ bỏ `WHERE`** (sẽ làm yếu invariant thật, khác 7 partial
  index KHÔNG-unique khác trong service này, chỉ cần bỏ `WHERE`).
- **`TEXT PRIMARY KEY`** (`terminal_sessions.pty_id`,
  `queued_prompts.pty_id`) — InnoDB không cho phép PRIMARY KEY trên
  TEXT/BLOB (không có prefix-length escape hatch cho PK, khác index
  thường) → đổi hẳn sang `VARCHAR(255)`, kéo theo FK tham chiếu
  (`agent_sessions.pty_id`) cũng phải đổi theo.
- **UNIQUE constraint trên cột TEXT** (`ssh_targets`'s `host`/`user_name`,
  `fleet_definitions.name`, `dev_server_group_grants`'s
  `grantee_kind`/`grantee_id`) — 2 chiến lược tuỳ độ an toàn: prefix-length
  index `(255)` khi giá trị có giới hạn thật ngoài đời (DNS hostname ≤253
  ký tự) đảm bảo prefix không bao giờ làm sai uniqueness; đổi hẳn type
  sang `VARCHAR` khi không có giới hạn tự nhiên rõ ràng (`pane_key`,
  `grantee_id`) — prefix index trên UNIQUE constraint chỉ đảm bảo
  uniqueness của PHẦN PREFIX, không phải toàn giá trị, nên tránh dùng khi
  không chứng minh được an toàn.
- **FK bị DROP theo tên ở migration sau** (`terminal_sessions_connection_id_fkey`,
  migrations 0005→0034): MySQL không tự đặt tên constraint dự đoán được
  cho FK khai báo inline — phải đặt tên tường minh (`CONSTRAINT ... FOREIGN
  KEY`) ngay từ migration gốc (0005) để migration 0034 sau này có thể
  `DROP FOREIGN KEY` đúng tên (MySQL dùng `DROP FOREIGN KEY`, không phải
  `DROP CONSTRAINT` như Postgres cho FK cho tới rất gần đây).
- **`DROP COLUMN IF EXISTS`**: MySQL không hỗ trợ `IF EXISTS` ở cấp cột
  trong `ALTER TABLE` (khác `DROP TABLE IF EXISTS`, vẫn hoạt động bình
  thường ở cấp bảng) — phát hiện thật (`Error 1064`), sửa 5 file down
  migration.
- **`DROP INDEX`**: MySQL yêu cầu gắn với 1 bảng
  (`ALTER TABLE t DROP INDEX name` hoặc `DROP INDEX name ON t`), không có
  dạng "bare" `DROP INDEX schema.name` như Postgres.

**Verify thật, không suy đoán**: dựng `mysql:8` container qua Docker trực
tiếp (không qua Go test) + chạy `migrate -path migrations/mysql -database
mysql://... up` — PASS cả 31 migration lần đầu sau khi sửa các lỗi thật ở
§7. Chạy tiếp `migrate down -all` — PASS cả 31 migration theo chiều ngược,
xác nhận down migration cũng chạy được thật (không chỉ up, khác các service
trước trong rollout thường chỉ verify qua `go test`'s `migrate up`-only
path). Chạy `migrate up` lại lần 2 từ đầu — PASS, xác nhận idempotent
across container lifecycle.

## 6. `cmd/server/main.go` — đã wire dialect switch (2026-09-12)

**LỊCH SỬ (2026-09-11):** khác mọi service trước trong rollout, việc wire
`cmd/server/main.go` bị hoãn lại 1 lượt — composition root 825 dòng, 15
lần gọi constructor `infrapostgres.*`, biến `sshTargetStore` đan sâu vào
nhiều subsystem — rủi ro cao không thể full-e2e-verify trong 1 lượt (xem
TASK-BE-DB-022's "Kết quả thực tế 2026-09-11" cho lý do đầy đủ lúc đó).

**Cách làm thật (2026-09-12):** `impact({target: "run", direction:
"upstream"})` risk **LOW** (impactedCount 1, chỉ `main`). Kỹ thuật: union
interface cục bộ `repoAll` (giống task-service's `cmd/server/main.go`,
BE-DB-SOL-015) cho biến `repo` — `Repository` implement 10 interface (9
usecase port + `outbox.Store`), CỘNG THÊM 1 interface thứ 11 phát hiện
thật lúc build (`infraeventbus.OutboxEnqueuer`'s `EnqueueOutboxEvent`,
dùng bởi `infraeventbus.NewHealthPublisher`) — không nằm trong 24 assertion
gốc của `shared.go` vì đó là interface CỤC BỘ của package `eventbus`, không
phải port `usecase`. 14 repository còn lại (`sshTargetStore` qua
`ephemeralVmSshTargetStore`) mỗi cái CHỈ dùng qua ĐÚNG 1 usecase port nên
giữ interface đơn (`usecase.SshTargetRepository`,
`usecase.TerminalSessionRepository`, ...) — khác `repo`, không cần union.
Riêng `agentRateLimitedOutboxStore` cần thêm 1 interface cục bộ
(`rateLimitedOutboxStore` = `outbox.Store` + `Enqueue(ctx, rec) error`,
method thứ 2 `infraeventbus.New`'s `rateLimitedOutboxEnqueuer` cần) — cùng
lý do `repoAll` cần `OutboxEnqueuer`: 1 struct implement nhiều interface
cục bộ của các package adapter khác nhau ngoài `usecase.ports.go`.

Cả 15 constructor gom vào MỘT `switch caps.Dialect` ngay đầu `run()` (thay
vì rải rác 15 chỗ như code gốc — dễ review, mỗi nhánh dialect chỉ viết 1
lần) — xoá 4 lệnh gọi `infrapostgres.NewX(pool)` rải rác phía sau
(`ephemeralVmRuntimeStore`, `portForwardStore`, `devServerGroupStore`×3,
`ephemeralVmSshTargetStore`), giữ nguyên MỌI usecase constructor call site
(không đổi tham số nào — đúng yêu cầu). `toMySQLDriverDSN` copy nguyên vẹn
từ `usage-service`. `healthSrv` cũng phải dời khai báo lên sớm hơn (trước
switch) vì outbox relay wiring (dùng `repo`) chạy trước điểm `healthSrv`
từng được khai báo ở code gốc.

2 lỗi build thật khi wiring (không suy đoán trước, đúng tinh thần audit đã
thiết lập ở §7): thiếu `infraeventbus.OutboxEnqueuer` trên `repoAll` và
thiếu `Enqueue` trên kiểu của `agentRateLimitedOutboxStore` — cả 2 sửa
bằng cách mở rộng interface cục bộ, KHÔNG đụng `internal/usecase/ports.go`
hay bất kỳ signature nào. `go build`/`go vet`/`gofmt -l` sạch sau khi wire
— xem TASK-BE-DB-022's "Cập nhật (2026-09-12)" mục 1 cho chi tiết đầy đủ.

## 7. Bug thật phát hiện khi verify migration/integration test thật — đã sửa

Cả 2 CHỈ phát hiện được bằng cách CHẠY THẬT `migrate up` trên container
`mysql:8` thật (không đọc code suông) — xác nhận giá trị của bước verify
độc lập trước khi viết adapter code:

1. **`Error 1101`**: MySQL 8.0.13+ từ chối `DEFAULT` trần (không ngoặc)
   trên cột TEXT/BLOB/JSON — 16 cột across 15 file cần sửa (§4.6).
2. **`Error 1064`**: `rows` là từ khoá dành riêng từ MySQL 8.0.19+ (window
   function frame unit) — 1 cột (§4.5).

Sau khi sửa cả 2, `migrate up` (31 migration) + `migrate down -all` (31
migration ngược) đều PASS thật trên `mysql:8` — xem §5.

**2026-09-12 — 2 bug thật khác phát hiện khi re-run integration suite
dưới tải thấp (khác 2 bug migration ở trên, đây là bug LOGIC trong
`internal/adapter/mysql`'s Go code, phát hiện qua `go test
-tags=integration`, không phải `migrate up`):**

1. `FleetDefinitionStore.Update` translate optimistic-locking sai —
   "UPDATE rồi SELECT lại độc lập scoped theo version MỚI" (bắt chước
   RETURNING) có thể khớp NHẦM 1 dòng mà 1 lần Update TRƯỚC ĐÓ đã đưa lên
   đúng version đó, che giấu version-conflict thật. Sửa: dùng thẳng
   `RowsAffected()` của chính câu UPDATE — an toàn ở đây vì cột `version`
   LUÔN đổi giá trị mỗi lần gọi (không giống pitfall chung BE-DB-SOL-005
   §3.1, nơi UPDATE CÓ THỂ là no-op thật).
2. `TerminalScrollbackSnapshotStore.Upsert` không insert cột `id` —
   MySQL's `id CHAR(36) PRIMARY KEY` không có DEFAULT (khác Postgres's
   `gen_random_uuid()`), và `domain.TerminalScrollbackSnapshot` không có
   field `ID` — ca duy nhất trong service này mà audit "mọi INSERT đều
   truyền id tường minh" (§1) bỏ sót vì domain struct không mang ID. Sửa:
   generate `uuid.NewString()` ngay trong `Upsert`, cùng pattern
   `EphemeralVmSshTargetStore.Upsert` đã dùng.

Cả 2 KHÔNG phát hiện được ở lượt 2026-09-11 vì integration suite chưa
từng chạy PASS/FAIL rõ ràng dưới Docker contention nặng lúc đó (test bị
timeout/chậm, không phải fail rõ ràng theo assertion) — xem TASK-BE-DB-022
"Cập nhật (2026-09-12)" mục 2 cho log đầy đủ.

**Postgres — 2 bug mechanical khác cũng phát hiện + sửa cùng lượt** (khác
2 bug MySQL ở trên, phát hiện khi re-run Postgres integration suite):
`domain.NewSshTarget`'s `tags []string` không default `nil` → `[]string{}`
(vi phạm NOT NULL ở cả 2 dialect khi test truyền `nil`); 1 test struct
literal `domain.DevServer{}` thiếu `Kind` (vi phạm CHECK migration 0036).
2 test Postgres còn FAIL sau đó (`TestMigration0018_DownDropsTable`,
`TestRepository_ResolveConnection_FoundAndNotFound`) là bug tiền tồn tại
thật, KHÔNG liên quan CR-DB-002/003, đòi hỏi viết lại test logic — flag
không sửa, xem TASK-BE-DB-022 "Cập nhật (2026-09-12)" mục 3.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Prefix-length UNIQUE index trên `host`/`user_name` (không phải VARCHAR hẳn) | Thấp | An toàn thật (DNS hostname ≤253 ký tự) — xem §5, không phải giả định chưa kiểm chứng |
| Driver TiDB thật chưa test | Trung bình | Kế thừa nguyên trạng thái đã ghi ở BE-DB-SOL-002 — chưa service nào trong rollout chạy thật trên `pingcap/tidb` |
| 2 test Postgres pre-existing còn FAIL (migration 0018 down, ResolveConnection) | Thấp (không liên quan CR-DB-002/003) | Flag không sửa — đòi hỏi viết lại test logic, xem §7 và TASK-BE-DB-022 "Cập nhật (2026-09-12)" mục 3 |

## Không thuộc phạm vi solution này

- Business logic provisioning/orchestration (SSH deploy, Vault, agent
  token issuance) — không đụng, chỉ tầng SQL adapter.
- Data migration tool Postgres→MySQL cho khách hàng thật — theo loại trừ
  chung CR-DB-003.
- Driver TiDB thật — xem Rủi ro.
- 2 test Postgres pre-existing còn FAIL — xem §7, ngoài phạm vi CR-DB-002/003.

## Liên quan

- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (RLS-drop pattern, RowsAffected pitfall gốc)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go`,
  `internal/usecase/list_ephemeral_vm_runtimes.go` (port thứ 2 nằm ngoài `ports.go`)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/*.go` (bản gốc để dịch)
- `backend-go/services/infra-fleet-service/internal/adapter/mysql/*.go` (mới, 15 file + `shared.go`)
- [TASK-BE-DB-022](../tasks/TASK-BE-DB-022-infra-fleet-service-mysql-rollout.md)
