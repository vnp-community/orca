# BE-DB-SOL-016: Dialect capability layer + MySQL/TiDB adapter cho `project-service`

> **✅ Implemented (2026-09-11).** Áp dụng nguyên vẹn pattern đã xác lập ở
> BE-DB-SOL-001/002 (`usage-service`, pilot) — không thiết kế lại. Service
> lớn thứ 2 trong rollout (9 repository, 48 file migration/24 cặp).

**CR:** CR-DB-002, CR-DB-003
**Service:** `backend-go/services/project-service`
**Depends on:** `backend-go/common/dbcapability` (dùng lại nguyên vẹn,
không sửa), `backend-go/common/testutil.StartMySQL` (dùng lại)
**Task tương ứng:** [TASK-BE-DB-021](../tasks/TASK-BE-DB-021-project-service-mysql-rollout.md)

---

## 1. Audit thật trước khi implement (đã Read đầy đủ, không suy đoán)

`project-service` là service lớn thứ 2 trong rollout theo audit
ROLLOUT-TRACKING.md (9 repo, 48 file migration = 24 cặp up/down, đánh số
`0001`-`0030` với khoảng trống `0009`-`0014` không tồn tại — xác nhận qua
`ls`, không phải thiếu file).

`internal/usecase/ports.go` (486 dòng) có **21 interface**, trong đó
**9 interface thật sự DB-backed** (implement bởi `internal/adapter/postgres`,
khớp đúng "9 repository" của ROLLOUT-TRACKING):

1. `ProjectRepository` (12 method) — `Repository` struct, bảng
   `project.projects`/`project.project_members`.
2. `RepoRepository` (14 method, gồm cả `repo_members` tier) —
   `RepoRepository` struct, bảng `project.repos`/`project.repo_members`.
3. `WorktreeRepository` (13 method) — `WorktreeRepository` struct, bảng
   `project.worktrees`, cũng implement `common/outbox.Store`.
4. `ProjectGroupRepository` (7 method, gồm `ImportNested`) —
   `ProjectGroupRepository` struct, bảng `project.project_groups`.
5. `SparsePresetRepository` (4 method) — `SparsePresetRepository` struct,
   bảng `project.sparse_presets`.
6. `SourceProjectRepository` (4 method) — `SourceProjectRepository` struct,
   bảng `project.source_projects`.
7. `HostSetupRepository` (7 method) — `HostSetupRepository` struct, bảng
   `project.project_host_setups`.
8. `FolderWorkspaceRepository` (6 method) — `FolderWorkspaceRepository`
   struct, bảng `project.folder_workspaces`.
9. `OutboxRepository`/`common/outbox.Store` (2 method,
   `FetchUnpublished`/`MarkPublished`) — `OutboxRepository` struct, bảng
   `project.outbox_events`; **trùng lặp có chủ đích** với
   `WorktreeRepository`'s own `FetchUnpublished`/`MarkPublished` (2 struct
   cùng implement `Store` trên cùng bảng) — pre-existing trong bản Postgres,
   giữ nguyên convention khi dịch, không hợp nhất (ngoài phạm vi rollout
   này).

12 interface còn lại trong `ports.go` **KHÔNG DB-backed** — outbound gRPC
client port (`WorkflowExecutionChecker`, `TaskExecutionChecker`,
`DevServerRelay`, `DevServerLister`, `TerminalStatusResolver`,
`DevServerHealthChecker`, `ProfileResolver`, `DevServerHostnameResolver`),
policy (`OPAClient`), event publisher (`AuditPublisher`, `MemberNotifier`),
và 2 "view" interface cấu trúc (`MembershipRepository`/
`RepoMembershipRepository` — subset method-set của `ProjectRepository`/
`RepoRepository`, thoả mãn tự động qua Go structural typing, không cần
implement riêng ở dialect MySQL).

### Migration audit (24 cặp, đọc đầy đủ nội dung từng file)

- **CÓ RLS trên HẦU HẾT bảng** (10/11 bảng): `projects`, `project_members`,
  `repos`, `worktrees`, `project_groups`, `folder_workspaces`,
  `project_host_setups`, `sparse_presets`, `repo_members`,
  `source_projects`, `outbox_events` — TẤT CẢ 11 bảng thật ra đều có
  `ENABLE ROW LEVEL SECURITY` + `CREATE POLICY tenant_isolation`. Áp dụng
  đúng phát hiện BE-DB-SOL-001 §4: RLS này chưa từng thật sự enforce (không
  có `SET LOCAL app.tenant_id` nào trong Go code), nên drop RLS cho MySQL
  không giảm bảo vệ thực tế — application-layer tenant scoping (rõ ràng ở
  `ProjectRepository`/`ProjectGroupRepository`/`HostSetupRepository`/
  `FolderWorkspaceRepository`'s `tenant_id = ?` filter) là lớp bảo vệ thật
  duy nhất, kể cả trên Postgres hôm nay.
- **KHÔNG có `gen_random_uuid()` được ứng dụng dựa vào** cho phần lớn ID —
  `grep` xác nhận toàn bộ `internal/usecase/*.go` tự sinh ID qua
  `uuid.NewString()` trước khi gọi repository (giống phát hiện
  `usage-service`). **Ngoại lệ duy nhất**:
  `internal/adapter/postgres/project_group_repository.go` dùng
  `gen_random_uuid()` trực tiếp trong SQL ở 3 chỗ (`UpsertLeafGroupForProject`,
  `ImportNested` ×2) — dịch sang MySQL bằng cách sinh `uuid.NewString()`
  trong Go trước INSERT (nhất quán với mọi ID khác của service, xem §3).
- **CÓ JSONB**: `repos.hook_settings` (0018), `worktrees.metadata` (0028) —
  dịch sang `JSON`.
- **CÓ mảng Postgres-only**: `sparse_presets.directories TEXT[]` (0016) —
  không có tương đương MySQL, dịch sang cột `JSON` + marshal/unmarshal
  `[]string` ở tầng Go (xem §5).
- **CÓ `RETURNING`**: hầu hết INSERT/UPDATE trong 6/9 repository — dịch
  bằng "ghi rồi đọc lại" (không dựa vào `RowsAffected()` cho việc phát hiện
  not-found, xem §3.1 — phát hiện quan trọng nhất kế thừa từ
  BE-DB-SOL-005).
- **CÓ multi-table `UPDATE ... FROM`** (Postgres-only cú pháp):
  `0017_repo_dev_server.up.sql`'s backfill — dịch sang MySQL's
  `UPDATE ... JOIN ... SET ...`.
- **CÓ partial index** (3 chỗ): `0008`'s
  `CREATE UNIQUE INDEX ... WHERE project_id IS NOT NULL`, `0019`'s
  `CREATE INDEX ... WHERE parent_worktree_id IS NOT NULL`, `0020`'s
  `CREATE UNIQUE INDEX ... WHERE idempotency_key IS NOT NULL`, `0025`'s
  `CREATE INDEX ... WHERE published_at IS NULL` — MySQL không có partial
  index. 2 chỗ UNIQUE dịch sang plain UNIQUE INDEX (MySQL coi mỗi NULL là
  distinct trong UNIQUE index — cùng ngữ nghĩa hiệu quả, xem migration
  comment); 2 chỗ non-unique dịch sang full index (theo tiền lệ
  BE-DB-SOL-005 §5).
- **CÓ cột TEXT tham gia UNIQUE constraint** (2 chỗ, gap thật so với mọi
  service trước trong rollout): `folder_workspaces.path` (UNIQUE
  `(tenant_id, dev_server_id, path)`) và `worktrees.idempotency_key`
  (UNIQUE `(project_id, idempotency_key)`) — MySQL/InnoDB không unique-index
  được cột `TEXT` không giới hạn mà không dùng prefix (yếu hoá guarantee).
  Dịch bằng cách nới rộng kiểu cột — `path` → `VARCHAR(1024)`,
  `idempotency_key` → `VARCHAR(255)` — đủ cho mọi giá trị thực tế (path
  Linux tối đa 4096 byte nhưng hiếm khi path thực tế vượt 1024; SHA-256 hex
  digest = 64 ký tự), ghi rõ trong comment migration, không âm thầm.
- **CÓ data-migration đặc biệt**: `0022_backfill_creator_membership` (INSERT
  ... SELECT ... ON CONFLICT DO NOTHING) — dịch sang `INSERT IGNORE`, và
  **cải thiện có chủ đích** so với bản gốc: thêm `WHERE p.created_by IS NOT
  NULL` tường minh (bản Postgres không có, dựa vào ON CONFLICT bắt lỗi
  unique — nhưng `user_id` là NOT NULL nên 1 project không có `created_by`
  sẽ lỗi ở cả 2 dialect theo cách khác nhau; MySQL's `INSERT IGNORE` với
  NULL vào cột NOT NULL có thể chèn giá trị default ngầm thay vì báo lỗi rõ
  ràng — filter tường minh tránh hẳn nhóm dữ liệu này ở cả 2 ý định).

### Bug tiền tồn tại phát hiện khi dịch (flag, không fix — ngoài phạm vi)

`internal/adapter/postgres/project_group_repository.go`'s `ImportNested`
(dòng 174-182) build `INSERT ... RETURNING` với `projectColumns` (12 cột)
nhưng `.Scan(...)` chỉ liệt kê **10** đích scan (thiếu
`IssueStatusSyncEnabled`, `MobileEmulatorAgentID`) — pgx sẽ lỗi
"number of field descriptions must equal number of destinations" mỗi lần
`ImportNested` chạy trên Postgres thật. Đây là bug có thật, tiền tồn tại,
**không sửa ở đây** (ngoài phạm vi rollout multi-database, cùng posture
"flag don't fix" như TASK-BE-DB-009's UUID-column finding cho
`issue-tracking-service`) — bản MySQL viết mới không tái tạo lỗi này (xây
dựng `domain.Project` trực tiếp từ giá trị Go đã biết, không `Scan` một
`RETURNING` không khớp cột).

## 2. `impact()` chạy trước khi sửa (bắt buộc theo CLAUDE.md/AGENTS.md)

Chạy trên repo `orca`, tất cả **risk LOW**, không HIGH/CRITICAL nào — an
toàn tiến hành không cần cảnh báo thêm:

| Symbol | Kind | impactedCount | Risk |
|---|---|---|---|
| `run` (`cmd/server/main.go`) | Function | 1 | LOW |
| `Load` (`internal/config/config.go`) | Function | 2 | LOW |
| `ProjectRepository` | Interface | 3 | LOW |
| `RepoRepository` | Interface | 3 | LOW |
| `WorktreeRepository` | Interface | 3 | LOW |
| `ProjectGroupRepository` | Interface | 3 | LOW |
| `SparsePresetRepository` | Interface | 3 | LOW |
| `SourceProjectRepository` | Interface | 3 | LOW |
| `HostSetupRepository` | Interface | 3 | LOW |
| `FolderWorkspaceRepository` | Interface | 3 | LOW |

Không interface nào đổi signature — adapter MySQL implement nguyên vẹn
shape hiện có. `internal/config/config.go` đã có sẵn `DatabaseCredentialsFile`
+ `secrets.DatabaseCredentialsFromFile` wiring từ trước (khác
`annotation-service`/`credential-broker-service`, vốn phải thêm field này
— `project-service` được "Project Workspace unification" work trước đó đã
wire sẵn phần Vault, chỉ còn thiếu dialect switch) — xác nhận tươi qua Read
trực tiếp file, không giả định theo doc cũ.

## 3. `internal/adapter/mysql/` — 9 file, implement lại đúng port hiện có

Dịch từng file 1:1 theo cấu trúc `internal/adapter/postgres/` (đã Read đầy
đủ cả 9 file postgres trước khi viết bản MySQL, không suy đoán shape):
`repository.go` (Project+Member), `repo_repository.go`,
`worktree_repository.go`, `project_group_repository.go`,
`sparse_preset_repository.go`, `source_project_repository.go`,
`host_setup_repository.go`, `folder_workspace_repository.go`,
`outbox_repository.go`. Không đổi signature nào.

### 3.1 RowsAffected()-counts-CHANGED-not-MATCHED pitfall (BE-DB-SOL-005 §3.1)

Với 6/9 repository dùng `UPDATE ... RETURNING` ở Postgres, dịch ngây thơ
"UPDATE rồi check `RowsAffected() == 0` để suy ra not-found" SAI trên
MySQL với **mọi** UPDATE có thể no-op hợp lệ (rename về đúng tên cũ, rebind
về đúng dev server cũ, resend cùng giá trị...) — driver mysql mặc định đếm
"dòng thực sự đổi giá trị", không phải "dòng khớp WHERE". Áp dụng thống
nhất trên toàn bộ 9 file: **UPDATE luôn luôn không đọc `RowsAffected()` để
quyết định not-found** — hoặc (a) check tồn tại bằng `SELECT EXISTS(...)`
riêng trước UPDATE (khi cần phân biệt not-found với no-op mà UPDATE thân nó
không tự nhiên trả lại), hoặc (b) UPDATE rồi `SELECT` lại (khi cần trả về
row mới nhất cho caller — hầu hết trường hợp, thay thế cho `RETURNING`).
`DeleteProject`/`RemoveMember`/`RemoveRepo`/... (mọi `DELETE`) **vẫn dùng
`RowsAffected()` bình thường** — DELETE luôn đếm theo WHERE-match ở mọi
dialect, không có ambiguity này (đúng phát hiện BE-DB-SOL-005 §4 cho
`DeleteAnnotation`).

Test trực tiếp phát hiện này: `TestRepository_UpdateProject_NoopPatchStillSucceeds`,
`TestRepository_UpdateDevServerID_NoopStillSucceeds`,
`TestProjectGroupRepository_CreateGetUpdateDelete_RoundTrip`'s no-op-rename
case (xem §6).

### 3.2 `sparse_presets.directories`: TEXT[] → JSON

Không tương đương MySQL trực tiếp cho Postgres array — cột MySQL dịch sang
`JSON`, adapter `json.Marshal`/`json.Unmarshal` giữa `[]string` (kiểu domain
không đổi) và cột JSON ở mọi INSERT/SELECT. Test
`TestSparsePresetRepository_SaveAndList_DirectoriesJSONRoundTrips` xác nhận
thứ tự phần tử giữ nguyên qua round-trip + qua upsert-replace.

### 3.3 `worktrees.metadata` merge-patch: jsonb `||` → `JSON_MERGE_PATCH`

Postgres's `metadata || $1::jsonb` merge nông (shallow merge, key trùng bị
ghi đè, `null` tường minh lưu lại làm giá trị null) — MySQL's
`JSON_MERGE_PATCH(metadata, ?)` theo đúng RFC 7396: merge nông tương tự,
NHƯNG 1 khác biệt thật: `null` tường minh trong patch **xoá hẳn key** thay
vì lưu `null`. Với callsite duy nhất hiện có (frontend's "clear this field"
convention qua `UpdateWorktreeMeta`), cả 2 hành vi đều biểu hiện quan sát
được là "field biến mất/bị xoá" với mọi reader hiện có trong codebase
(không ai query raw JSON shape phía server) — coi là tương đương chức năng,
ghi rõ trong code comment, không che giấu khác biệt.

### 3.4 `id = ANY($1)` → `IN (?,...)` động

`MarkPublished` (cả `WorktreeRepository` và `OutboxRepository`) build mệnh
đề `IN` động theo `len(ids)`, giữ nguyên guard `len(ids) == 0` return nil
sớm (MySQL `IN ()` là lỗi cú pháp, giống phát hiện `usage-service`).
`ListWorktrees`'s `statusIn []string` filter cũng dịch theo cùng mẫu (không
có tương đương `= ANY($2)` cho optional-array-filter trên MySQL/
`database/sql`).

### 3.5 Multi-table UPDATE trong transaction — MySQL "same table in subquery" hạn chế

`RepoRepository.ReassignProject`'s vị trí (`position = MAX(position)+1`)
subquery trên chính bảng `repos` đang UPDATE — MySQL cấm trực tiếp
(`ERROR 1093: You can't specify target table 'repos' for update in FROM
clause`), khắc phục bằng derived-table wrapper 1 lớp
(`SELECT x.m FROM (SELECT MAX(position)+1 AS m FROM repos WHERE ...) x`) —
kỹ thuật chuẩn MySQL cho hạn chế này, xác nhận qua chạy test thật (§6).

### 3.6 FK/UNIQUE violation error-code mapping

`FolderWorkspaceRepository.Create` map lỗi MySQL sang sentinel giống hệt
bản Postgres: mã lỗi `1062` (`ER_DUP_ENTRY`, vi phạm
`UNIQUE(tenant_id, dev_server_id, path)`) → `ErrPathAlreadyRegistered`, mã
`1452` (`ER_NO_REFERENCED_ROW_2`, `project_group_id` không tồn tại) →
`ErrProjectGroupNotFound` — dùng `errors.As` với
`*github.com/go-sql-driver/mysql.MySQLError`, tương đương
`pgconn.PgError.Code` (SQLSTATE `23505`/`23503`) ở bản Postgres.

## 4. Wiring `cmd/server/main.go`

Thêm `switch caps.Dialect` đúng khuôn `usage-service`: 9 biến port khai báo
kiểu interface (`usecase.ProjectRepository`, ...) thay vì kiểu struct cụ
thể, để 1 khối code chọn implementation theo dialect thay vì nhân đôi toàn
bộ phần wiring usecase phía dưới (khác `usage-service`'s 2-biến vì
`project-service` có 9 port DB-backed, không phải 1). `healthSrv` khởi tạo
trước switch, `Register("postgres"/"mysql", ...)` bên trong mỗi nhánh —
cùng mẫu `usage-service`. `DatabaseCredentialsFile`/Vault wiring giữ
nguyên hoàn toàn (đã có sẵn từ trước task này, xem §2).

## 5. Migration `migrations/postgres/` (git mv, 48 file) + `migrations/mysql/` (mới, 48 file)

24 cặp up/down mỗi bên — quyết định tên bảng/schema theo đúng chốt
BE-DB-SOL-002 §3: DSN của `project-service` trỏ thẳng database MySQL tên
`project`, bên trong dùng tên bảng KHÔNG prefix (`projects`, `repos`,
`worktrees`, ...) vì bản thân database đã là đơn vị cách ly tương đương
`project.` schema-qualifier của Postgres.

Điểm dịch không tầm thường liệt kê đầy đủ ở §1 (RLS/JSONB/array/RETURNING/
multi-table UPDATE/partial index/TEXT-in-UNIQUE/data-migration) — không lặp
lại SQL cụ thể ở đây, xem trực tiếp `migrations/mysql/*.sql`.

## 6. Test — `internal/adapter/mysql/*_test.go` (9 file mới, build tag `integration`)

Mirror `internal/adapter/postgres`'s 5 file test hiện có
(`repository_test.go`, `repo_repository_test.go`, `worktree_repository_test.go`,
`project_group_repository_test.go`, `folder_workspace_repository_test.go`)
1:1 nơi có tiền lệ Postgres, cộng 4 file MỚI cho 4 repository trước đây
**chưa từng có integration test ở cả 2 dialect** (`sparse_preset_repository_test.go`,
`source_project_repository_test.go`, `host_setup_repository_test.go`, và
`outbox_repository`'s coverage qua `worktree_repository_test.go`'s
transactional-outbox test) — dùng `common/testutil.StartMySQL`
(không sửa package này).

Test mới đặc thù rollout này (không có tiền lệ 1:1 ở service nào trước):

- `TestRepository_UpdateProject_NoopPatchStillSucceeds`,
  `TestRepository_UpdateDevServerID_NoopStillSucceeds` — §3.1's RowsAffected
  pitfall, 2 lần gọi UPDATE liên tiếp với cùng giá trị.
- `TestSparsePresetRepository_SaveAndList_DirectoriesJSONRoundTrips` — §3.2's
  TEXT[]->JSON, xác nhận thứ tự phần tử giữ nguyên.
- `TestWorktreeRepository_UpdateWorktreeMeta_MergePatch` — §3.3's
  `JSON_MERGE_PATCH`, xác nhận null tường minh xoá key.
- `TestRepoRepository_ReassignProject_ClearsRepoMembersAndDetectsRace` —
  §3.5's derived-table workaround chạy được thật + TOCTOU guard
  (`ErrRepoProjectChanged` vs `ErrRepoNotFound`).
- `TestFolderWorkspaceRepository_Create_DuplicatePathReturnsSentinel`,
  `TestFolderWorkspaceRepository_Create_InvalidProjectGroupReturnsSentinel`
  — §3.6's mã lỗi 1062/1452.
- `TestProjectGroupRepository_UpsertLeafGroupForProject_IsIdempotent`,
  `TestProjectGroupRepository_ImportNested_CreatesGroupsProjectsAndRepos`.

Tenant-isolation-without-RLS (TASK-BE-DB-003 pattern, cho bảng có cột
`tenant_id` trực tiếp — §1 xác nhận toàn bộ 11 bảng CÓ RLS trên Postgres,
nhưng chỉ những bảng dưới đây có tenant_id **trực tiếp** đủ để viết test
kiểu này mà không cần seed qua join nhiều tầng):
`TestRepository_ListForMember_DoesNotLeakAcrossTenants`,
`TestProjectGroupRepository_ListProjectGroups_DoesNotLeakAcrossTenants`,
`TestHostSetupRepository_Get_DoesNotLeakAcrossTenants`. Các bảng tenant-scope
gián tiếp qua FK (repos/worktrees/sparse_presets/repo_members/
source_projects/folder_workspaces qua project/project_group) đã KHÔNG có
filter tenant tường minh trong code Postgres từ trước (ví dụ
`RepoRepository.GetRepo(repoID)` không nhận `tenantID` — chỉ lọc theo `id`)
— đây là gap tiền tồn tại độc lập với multi-dialect, **ngoài phạm vi
rollout này** (flag, không fix — giống mọi service trước trong rollout).

Xem TASK-BE-DB-021's "Kết quả thực tế" cho số liệu PASS/FAIL thật từ lần
chạy `go test -tags=integration ./internal/adapter/postgres/...` VÀ
`./internal/adapter/mysql/...`.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| Quy mô (9 repo, 24 cặp migration) | Cao (trước khi làm) | Giảm xuống Trung bình sau khi hoàn thành — không có thiết kế mới, chỉ nhân rộng pattern đã xác lập |
| `path`/`idempotency_key` TEXT→VARCHAR widening | Trung bình | Weaker theo lý thuyết (giới hạn độ dài) nhưng an toàn thực tế cho mọi giá trị hệ thống này sinh ra — ghi rõ trong migration comment, không âm thầm |
| `ImportNested`'s Postgres bug tiền tồn tại | Thấp (ngoài phạm vi) | Flag ở §1, không fix — ảnh hưởng CẢ 2 dialect nếu có, nhưng KHÔNG do rollout này gây ra |
| Tenant-scoping gián tiếp qua FK (repos/worktrees/...) thiếu filter tường minh | Thấp (ngoài phạm vi, tiền tồn tại) | Độc lập với multi-dialect — RLS Postgres (chưa từng enforce) là lớp bảo vệ DUY NHẤT hiện có cho các bảng này trên CẢ 2 dialect trước rollout này; MySQL không làm gap này tệ hơn |

## Không thuộc phạm vi solution này

- 14 service còn lại — nhân rộng, ngoài phạm vi (xem
  `solutions/README.md`/`ROLLOUT-TRACKING.md`).
- Sửa `ImportNested`'s bug Postgres tiền tồn tại (§1) hoặc gap tenant-scoping
  gián tiếp (§6) — cả 2 ngoài phạm vi CR-DB, ghi nhận lại cho CR riêng nếu
  team muốn đóng.
- Data migration tool chuyển dữ liệu khách hàng thật — theo loại trừ của
  CR-DB-003.

## Liên quan

- `backend-go/services/project-service/internal/usecase/ports.go` (9 interface DB-backed)
- `backend-go/services/project-service/internal/adapter/postgres/*.go` (bản gốc để dịch)
- `backend-go/services/project-service/internal/adapter/mysql/*.go` (bản mới)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md) (`dbcapability`, phụ thuộc cứng)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (§3.1's RowsAffected pitfall, nguồn gốc kỹ thuật áp dụng lại ở đây)
- [TASK-BE-DB-021](../tasks/TASK-BE-DB-021-project-service-mysql-rollout.md)
