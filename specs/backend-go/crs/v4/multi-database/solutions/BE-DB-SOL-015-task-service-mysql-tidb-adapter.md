# BE-DB-SOL-015: Adapter MySQL/TiDB cho `task-service`

> **✅ Implemented (2026-09-11).** Batch 3+4+5 (gộp) rollout của pattern
> BE-DB-SOL-001/002 ra `task-service` — service lớn nhất đã làm trong
> rollout tính tới thời điểm này (10 "repository"/file adapter, 22 file
> migration).

**CR:** [CR-DB-002](../../../../../../docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md), [CR-DB-003](../../../../../../docs/crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
**Service:** `backend-go/services/task-service`
**Task:** [TASK-BE-DB-020](../tasks/TASK-BE-DB-020-task-service-mysql-rollout.md)
**Phụ thuộc cứng:** [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) — pattern dùng lại nguyên vẹn, không thiết kế lại

---

## 1. Audit thật trước khi implement — `task-service` khác pilot đáng kể

`impact()`/`context()` + đọc trực tiếp `internal/usecase/ports.go`,
`internal/adapter/postgres/*.go` (10 file), 22 file migration xác nhận số
liệu thật (khớp đúng ước tính audit của ROLLOUT-TRACKING.md — "10 repos,
22 migrations"):

- **7 interface DB-backed trong `ports.go`**: `TaskRepository` (18
  method), `EdgeRepository` (5 method), `GrantRepository` (4 method),
  `ShareLinkRepository` (4 method), `CommentRepository` (2 method),
  `ExecutionLinkRepository` (5 method), `OutboxWriter` (1 method,
  `InsertOutboxEvent`) — cộng `TxRunner` (`RunInTx`) và `VelocityResolver`
  (`RecentCompletedTasks`, implement trực tiếp bởi `Repository` vì đây là
  dữ liệu CỦA CHÍNH `task-service`, không qua gRPC client như các port
  khác trong `ports.go`). Một interface `OutboxWriter` THỨ HAI, hẹp hơn
  (`WriteOutboxEvent`), sống riêng trong
  `internal/adapter/eventbus/publisher.go` — 2 interface trùng tên khác
  package, khác chữ ký, cả 2 đều implement bởi `postgres.Repository`.
- **10 file trong `internal/adapter/postgres/`** implement các interface
  trên: `repository.go` (TaskRepository CRUD chính + `New` + `RunInTx`),
  `edges.go` (EdgeRepository), `grants.go` (GrantRepository),
  `comments.go` (CommentRepository), `share_link.go`
  (`GetByShareToken` — 1 method của TaskRepository, tách file riêng vì lý
  do domain: public/unauthenticated), `share_links.go` (`ShareLinkStore`,
  1 struct riêng implement `ShareLinkRepository` — tên method
  `Create`/`Revoke` trùng với `Repository.Create`/`Repository.Revoke`
  nên KHÔNG thể gộp chung struct), `execution_links.go`
  (ExecutionLinkRepository), `outbox.go` (`WriteOutboxEvent` +
  `InsertOutboxEvent` + `common/outbox.Store`'s `FetchUnpublished`/
  `MarkPublished`), `subtree.go` (`GetSubtree`/`GetSubtreeWithChildPercents`/
  `BatchUpdateProgress` — 3 method widen TaskRepository), `velocity.go`
  (VelocityResolver). Khớp đúng "10 repos" của audit.
- **22 file migration** (11 cặp up/down): schema `task.tasks` (32 cột sau
  11 lần ALTER) + `task.task_edges`/`task.task_grants`/`task.task_comments`/
  `task.task_share_links`/`task.execution_links`/`task.outbox_events`.
- **CÓ RLS** trên 6/6 bảng chính (`0001_init`, `0004_grant_expiry_and_public_link`,
  `0010_execution_links` mỗi migration bật `ENABLE ROW LEVEL SECURITY` +
  policy) — nhiều nhất trong rollout tính tới nay.
- **CÓ `gen_random_uuid()`** (mọi `id UUID PRIMARY KEY DEFAULT
  gen_random_uuid()`) nhưng — giống mọi phát hiện trước trong rollout —
  **không ứng dụng nào dựa vào DEFAULT này**: `Repository.Create` luôn
  INSERT `id` tường minh (`task.ID`, sinh ở usecase qua `uuid.NewString()`),
  `Grant`/`AddComment`/`CreateExecutionLink`/`ShareLinkStore.Create` đều
  dùng `RETURNING id` để lấy giá trị DB sinh — đây là điểm khác biệt DUY
  NHẤT so với `usage-service`/`annotation-service`: các bảng phụ (`task_grants`,
  `task_comments`, `task_share_links`, `execution_links`) THỰC SỰ dựa vào
  `gen_random_uuid()` qua `RETURNING id` (không giống `tasks.id`, luôn
  set tường minh). MySQL không có `RETURNING` — id cho các bảng này phải
  sinh ở Go (`uuid.NewString()`, xem §3).
- **CÓ `JSONB`** (`ai_plan_json`) và **CÓ `RETURNING`** ở nhiều method
  (`Create`, `Grant`, `AddComment`, `CreateExecutionLink`, `ShareLinkStore.Create`).
- **CÓ `WITH RECURSIVE`** ở 3 chỗ (`GetAncestors`, `GetSubtree`,
  `GetSubtreeWithChildPercents`) — MySQL 8.0.1+ hỗ trợ recursive CTE nên
  dịch trực tiếp, không cần viết lại thuật toán.
- **CÓ `array_agg`** (`GetSubtreeWithChildPercents`) → MySQL's
  `JSON_ARRAYAGG`.
- **CÓ 1 cột kiểu mảng Postgres thật** (`labels TEXT[]`, migration
  `0011_task_widened_fields`) — **CHƯA từng gặp trong rollout tính tới
  nay** (annotation-service/issue-tracking-service không có cột mảng
  nào). MySQL không có kiểu mảng — dịch sang cột `JSON` (mảng chuỗi), xem
  §5.
- **CÓ 1 sequence Postgres thật đang được dùng** (`task.task_number_seq`,
  migration `0008_task_outbox_and_number`, gọi qua
  `nextval('task.task_number_seq')` ngay trong câu `INSERT` của `Create`)
  — cũng **chưa từng gặp trong rollout** (usage-service/annotation-service
  không dùng `nextval()` inline). MySQL/TiDB không có `CREATE SEQUENCE` —
  emulate bằng 1 bảng `AUTO_INCREMENT` riêng, xem §5.

## 2. `impact()` chạy thật trước khi sửa/thêm bất kỳ symbol nào (bắt buộc, AGENTS.md)

Repo `orca`, tất cả chạy `summaryOnly:true` để tránh output khổng lồ (theo
"Stop on Saturation"):

| Symbol | Kind | File | Risk | impactedCount |
|---|---|---|---|---|
| `TaskRepository` | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `EdgeRepository` | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `GrantRepository` | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `ShareLinkRepository` | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `CommentRepository` | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `ExecutionLinkRepository` | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `OutboxWriter` (usecase, disambiguated qua file_path) | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `TxRunner` (task-service, disambiguated) | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `VelocityResolver` | Interface | `internal/usecase/ports.go` | MEDIUM | 10 |
| `New` | Function | `internal/adapter/postgres/repository.go` | LOW | 2 |
| `run` | Function | `cmd/server/main.go` | LOW | 1 |
| `Load` | Function | `internal/config/config.go` | LOW | 2 |
| `Config` | Struct | `internal/config/config.go` | LOW | 3 |

Không HIGH/CRITICAL nào. MEDIUM/10 cho mọi interface trong `ports.go` là
cùng hiện tượng đã ghi nhận ở BE-DB-SOL-004/005/006: `impactedCount=10` là
depth=1 `IMPORTS` cấp FILE (mọi file `internal/adapter/*` import package
`usecase` dùng chung, không phải 10 lời gọi thực sự phụ thuộc chữ ký của
riêng interface đó). `OutboxWriter`/`TxRunner` cần `target_uid`/`file_path`
để disambiguate (trùng tên với `infra-fleet-service`'s `OutboxWriter`,
`credential-broker-service`'s `TxRunner`, và task-service's OWN
`adapter/eventbus.OutboxWriter` — 3 symbol trùng tên `OutboxWriter` toàn
repo). **Không đổi chữ ký interface nào** (chỉ thêm 1 implementation song
song trong `internal/adapter/mysql/`) — an toàn tiến hành theo cả 13 lần
chạy trên.

## 3. Adapter `internal/adapter/mysql/` — 9 file mới, implement đúng 10
"repository" (1 file `ids.go` phụ thêm cho tiện ích chung)

`Repository` (1 struct, 1 `*sql.DB`) implement TẤT CẢ: `TaskRepository`,
`EdgeRepository`, `GrantRepository`, `CommentRepository`,
`ExecutionLinkRepository`, `usecase.OutboxWriter`, `taskeventbus.OutboxWriter`,
`VelocityResolver`, `TxRunner`, `common/outbox.Store` — khớp đúng shape
Postgres's `Repository`. `ShareLinkStore` (1 struct riêng, cùng lý do đặt
tên method trùng như bản Postgres) implement `ShareLinkRepository`.

### 3.1. `RETURNING`/`gen_random_uuid()` cho các bảng phụ — id sinh ở Go

Khác pilot (`usage-service` không có bảng nào thực sự cần `RETURNING id`
cho 1 cột PK `gen_random_uuid()`): `Grant`, `AddComment`,
`CreateExecutionLink`, `ShareLinkStore.Create` mỗi hàm sinh `id` bằng
`uuid.NewString()` (hàm `newUUID()` dùng chung, `internal/adapter/mysql/ids.go`)
rồi INSERT tường minh — không có "translate `RETURNING`" nào phức tạp vì
chỉ cần thay 1 giá trị Postgres-side-generated bằng 1 giá trị Go-side-generated,
cùng shape `usage-service`'s `ID` field vốn đã luôn sinh ở Go. `AddComment`
cần thêm 1 `SELECT` sau `INSERT` để lấy lại `created_at` (giá trị server-side
`CURRENT_TIMESTAMP(6)`, usecase cần trả về timestamp thật, không phải giờ
client) — 2 round-trip thay vì 1 `INSERT ... RETURNING`, chấp nhận được vì
`AddComment` không nằm trên hot path.

### 3.2. Cạm bẫy RowsAffected — kiểm tra lại theo đúng BE-DB-SOL-005 §3.1's cảnh báo, KHÔNG áp dụng máy móc

Đọc lại từng usecase gọi `UpdateStatus`/`Update`/`CompleteExecution`/
`SetActiveExecutionLink`/`Delete`/`Revoke`/`SetExternalRef`/`Complete`
trước khi quyết định có giữ `RowsAffected() == 0` → not-found hay không
(annotation-service's bài học: MySQL đếm "dòng đổi giá trị", không phải
"dòng khớp WHERE"):

- `UpdateStatus`, `Update` (TaskRepository): **giữ nguyên** kiểm tra
  `RowsAffected`, có ghi chú tường minh trong code lý do an toàn — không
  usecase nào gọi lại 2 lần với cùng giá trị (`UpdateStatus` chỉ set
  `StatusInProgress` 1 lần lúc dispatch; `Update`'s usecase luôn áp ít
  nhất 1 thay đổi field thật trước khi gọi), khác hẳn
  `UpdateAnnotation`'s no-op-retry path.
- `Revoke` (GrantRepository), `UpdateStatusMirror` (ExecutionLinkRepository):
  **giữ nguyên "không kiểm tra RowsAffected"**, y hệt bản Postgres — cả 2
  đã tự tài liệu hoá là idempotent/no-op-by-design từ trước (không phải
  quyết định mới của solution này).
- `Delete`, `SetActiveExecutionLink`, `CompleteExecution`, `SetExternalRef`,
  `Complete` (execution_links): giữ `RowsAffected` check — cùng lý do
  "không có no-op-retry path hợp lệ nào đi qua các hàm này" như
  `UpdateStatus`.

Không phát hiện trường hợp nào giống `UpdateAnnotation`'s cạm bẫy thật
(no-op retry hợp lệ bị báo nhầm not-found) trong `task-service` — nhưng
đã kiểm tra CÓ chủ đích cho từng hàm, không bỏ qua bước này.

### 3.3. `MarkPublished`/`ListGrantsForAncestors` — `IN (?,...)` động

`id = ANY($1)` (outbox) và `task_id = ANY($2)` (grants) đều cần build
`IN (?,...)` động theo `len(ids)`, guard `len == 0` — cùng khuôn
BE-DB-SOL-002 §3.

## 4. Migration dialect-safe — điểm khác biệt lớn nhất so với mọi service trước trong rollout

`migrations/postgres/` (11 cặp, `git mv` nguyên văn) + `migrations/mysql/`
(11 cặp mới):

### 4.1. `labels TEXT[]` → cột `JSON`

Chưa từng gặp trong rollout — MySQL không có kiểu mảng. Dịch sang
`labels JSON NOT NULL DEFAULT (JSON_ARRAY())` (MySQL 8.0.13+ hỗ trợ
expression DEFAULT cho JSON). `internal/adapter/mysql`'s
`marshalLabels`/`unmarshalLabels` round-trip `domain.Task.Labels`
(`[]string`) qua `encoding/json`, luôn coalesce `nil` → `[]string{}`
trước khi marshal (§6's phát hiện phụ giải thích tại sao điều này quan
trọng hơn dự kiến).

### 4.2. `task.task_number_seq` (sequence) → bảng `AUTO_INCREMENT`

MySQL/TiDB không có `CREATE SEQUENCE`. `migrations/mysql/0008`'s
`task_number_seq` là 1 bảng `(id BIGINT AUTO_INCREMENT PRIMARY KEY)`;
`Repository.Create` lấy giá trị kế tiếp bằng `INSERT INTO task_number_seq
VALUES (NULL)` rồi đọc `sql.Result.LastInsertId()` — 2 câu lệnh riêng
(không bọc transaction nội bộ), CHỦ Ý: giống hệt tính chất sequence thật
của Postgres (`nextval()` không bao giờ bị rollback cùng transaction bao
quanh nó) — 1 giá trị "bị đốt" khi câu `INSERT INTO tasks` sau đó thất
bại là hành vi ĐÚNG, không phải bug.

### 4.3. Partial unique index `task_edges_single_parent` — phát hiện MySQL/InnoDB thật, không đoán

Postgres: `CREATE UNIQUE INDEX task_edges_single_parent ON task.task_edges
(to_task_id) WHERE edge_type = 'parent_child'` (enforce "tối đa 1 cạnh
parent_child mỗi con"). MySQL không có partial index — dùng cột generated
`single_parent_key CHAR(36) GENERATED ALWAYS AS (CASE WHEN edge_type =
'parent_child' THEN to_task_id ELSE NULL END) STORED` + `UNIQUE
(single_parent_key)` (MySQL's UNIQUE coi mọi NULL là phân biệt, y hệt
Postgres). **Phát hiện thật khi chạy migration lần đầu trên container
`mysql:8` thật**: InnoDB từ chối `fk_task_edges_to ... ON DELETE CASCADE`
với `ERROR 1215 (HY000): Cannot add foreign key constraint` — xác nhận
bằng bisect trực tiếp (tạo bảng thủ công, bỏ dần từng phần) rằng nguyên
nhân là **InnoDB không cho phép hành động `CASCADE`/`SET NULL` trên 1 FK
khi cột con của FK đó đồng thời là cột nguồn của 1 generated column
STORED trong CÙNG bảng** — ở đây `to_task_id` vừa là FK child column vừa
là input của `single_parent_key`. Giải pháp: bỏ `ON DELETE CASCADE` khỏi
riêng `fk_task_edges_to`, thay bằng 1 `TRIGGER BEFORE DELETE ON tasks`
xoá `task_edges` theo `to_task_id = OLD.id` — khôi phục đúng hành vi
cascade cũ (đã test thật: xoá task con → `task_edges` mất đúng dòng qua
trigger; `fk_task_edges_from`'s CASCADE thật không bị ảnh hưởng, vẫn xử
lý nhánh `from_task_id`). Đã verify cả 2 chiều `migrate up`/`migrate down
-all` chạy sạch qua CLI `migrate` thật (không chỉ `mysql` client thủ
công) trước khi coi migration này DONE — xem "Kết quả thực tế".

### 4.4. Các điểm dịch còn lại — cùng khuôn các service trước

- `JSONB` (`ai_plan_json`) → `JSON`.
- `gen_random_uuid()` cho `tasks.id`: bỏ (không dùng tới, xem §1);
  `DEFAULT (UUID())` giữ lại cho các bảng phụ dù cũng không dùng tới
  (tường minh id ở Go) — chỉ để giữ schema tự-mô-tả, không có tác dụng
  chức năng.
- `RETURNING` (5 chỗ) → INSERT/UPDATE tường minh + SELECT lại khi cần (Go
  sinh id, xem §3.1) hoặc bỏ hẳn (khi giá trị trả về không cần đọc lại
  DB).
- `array_agg` → `JSON_ARRAYAGG`, `COALESCE(..., '{}')` →
  `COALESCE(..., JSON_ARRAY())`.
- Partial index không phải UNIQUE (`idx_tasks_project_active`,
  `idx_tasks_assignee`, `idx_task_grants_expires`,
  `idx_task_share_links_task`) → index đầy đủ, cùng lý do BE-DB-SOL-005 §5
  (không đổi đúng/sai kết quả, chỉ độ chọn lọc).
- Partial UNIQUE index `idx_tasks_project_task_number` (task_number): giữ
  nguyên là index KHÔNG partial nhưng **không mất đúng-sai** — vì cột
  `task_number` (không phải `project_id`) mới là cột luôn NULL ở hàng
  loại trừ, và MySQL/Postgres đều coi NULL trong UNIQUE index là phân
  biệt-với-mọi-NULL-khác — logic y hệt §4.3's lý do nhưng áp dụng ngược
  (ở đây một index KHÔNG-partial vẫn đúng 100%, không phải compromise).
- `TEXT` không giới hạn trong UNIQUE key (`token_hash`) → `VARCHAR(64)`
  (SHA-256 hex, đúng-khớp-độ-dài, không phải prefix lossy như
  annotation-service's `repo_id`/`file_path`).
- RLS (6 bảng) bỏ, comment giải thích application-layer scoping đã là cơ
  chế duy nhất có thật (cùng lý do BE-DB-SOL-001 §4, xác nhận lại: không
  `SET LOCAL app.tenant_id` nào trong `task-service`).
- CHECK constraint: đặt tên tường minh (`CONSTRAINT tasks_status_check
  CHECK (...)`) ngay từ `0001_init` để `0003`'s `DROP CHECK
  tasks_status_check` trỏ đúng constraint — MySQL tự sinh tên nếu không
  đặt tường minh, sẽ không khớp Postgres's tên cứng.

## 5. Wiring `cmd/server/main.go` — điểm khác biệt so với pilot

Không giống `usage-service`/`issue-tracking-service`/`annotation-service`
(1-3 interface), `task-service`'s `repo` biến được dùng làm 8+ interface
khác nhau xuyên suốt file (vd. `usecase.NewCreateTask(repo, repo)` cần
`repo` thoả cả `TaskRepository` LẪN `GrantRepository` trong CÙNG 1 lời
gọi). Giải pháp: định nghĩa 1 interface cục bộ `repoAll` (union của toàn
bộ 9 interface DB-backed `Repository` implement) ngay trong `run()`, khai
báo `var repo repoAll`, gán 1 lần trong `switch caps.Dialect` — mọi lời
gọi usecase constructor hiện có (không đổi 1 dòng nào trong ~30 dòng
wiring usecase) tự động biên dịch đúng vì giá trị `repoAll` thoả mãn bất
kỳ interface con nào Go's structural typing cho phép truyền vào tham số
hẹp hơn. Cách này tránh phải sửa từng lời gọi usecase riêng lẻ (rủi ro
sai sót cao hơn nhiều so với pilot's 1-repo case) — `go build` tự xác
nhận đúng đắn (bắt lỗi thiếu method ngay, xem "Kết quả thực tế" mục phát
hiện `taskeventbus.OutboxWriter` phải thêm vào `repoAll`).

`shareLinkStore` (kiểu `usecase.ShareLinkRepository`, không nằm trong
`repoAll` vì `ShareLinkStore` là struct riêng) khai báo/gán song song
trong cùng switch.

`Config` thêm `DatabaseCredentialsFile` (trước đây thiếu — `main.go` đọc
thẳng `cfg.DatabaseDSN`), đóng gap README's "common/secrets (Vault) is
not wired into main.go" như tác dụng phụ hợp lý của việc thêm dialect
factory — cùng tiền lệ annotation-service's TASK-BE-DB-010. `toMySQLDriverDSN`
copy nguyên văn từ `usage-service` (không có package chung, đúng tiền lệ
mọi service trước trong rollout).

## 6. Phát hiện phụ — bug tiền tồn tại NGHIÊM TRỌNG ở `internal/adapter/postgres`, KHÔNG sửa (ngoài phạm vi)

Chạy `go test -tags=integration ./internal/adapter/postgres/... -v` (đối
chứng việc tách `migrations/postgres/` không phá vỡ gì) lộ ra **8/13 test
FAIL thật** — TẤT CẢ cùng 1 nguyên nhân gốc, xác nhận qua đọc code (không
suy đoán): `domain.NewTask` (constructor duy nhất mọi usecase/test dùng để
tạo 1 `Task` mới) **không khởi tạo trường `Labels`**, để nguyên giá trị
`nil` (`[]string` zero value). `internal/adapter/postgres/repository.go`'s
`Create` luôn INSERT cột `labels` (migration `0011`: `labels TEXT[] NOT
NULL DEFAULT '{}'`) bằng giá trị `task.Labels` TRỰC TIẾP, không qua
`COALESCE`/fallback nào — `pgx` chuyển `nil []string` Go thành SQL `NULL`
(khác 1 slice rỗng `[]string{}`, chuyển thành `{}`), vi phạm `NOT NULL`
ngay lập tức: `ERROR: null value in column "labels" of relation "tasks"
violates not-null constraint (SQLSTATE 23502)`.

**Mức độ nghiêm trọng — khác các phát hiện phụ trước trong rollout (vốn
chỉ là 1-2 test cá biệt)**: đây là lỗi chặn **MỌI** lời gọi `Create` không
tự tay set `Labels` trước — tức là chặn cả đường dẫn PRODUCTION thật (gRPC
`CreateTask` handler → `usecase.CreateTask` → không đụng `Labels` →
`Repository.Create`), không chỉ test. Xác nhận bằng `grep -n "Labels"
internal/usecase/create_task.go` → không kết quả nào. Trên 1 deployment
Postgres đã chạy migration `0011` (`TASK-TG-001-02`), **`CreateTask` sẽ
luôn trả lỗi**. Đây là bug production thật, không phải bug test fixture
(khác annotation-service/issue-tracking-service's phát hiện, vốn chỉ là
test dùng literal sai kiểu).

**Không sửa ở đây** — đúng nguyên tắc "mỗi agent chỉ sửa 1 phạm vi task
được giao" (nhiệm vụ này là rollout MySQL/mở rộng dialect, không phải
audit/fix toàn bộ bug Postgres tiền tồn tại) và chỉ dẫn "Do not touch task
execution/orchestration business logic, only the SQL adapter layer" —
`domain.NewTask`/`usecase.CreateTask` nằm ngoài "SQL adapter layer". Ghi
nhận RÕ RÀNG, NGHIÊM TRỌNG cho người sở hữu `task-service` xử lý riêng,
khẩn cấp hơn các phát hiện phụ trước trong rollout.

**MySQL adapter của solution này KHÔNG mắc lỗi tương đương**: `Repository.Create`
(`internal/adapter/mysql/repository.go`)'s `marshalLabels` luôn coalesce
`nil` → `[]string{}` trước khi `json.Marshal` (§4.1) — một quyết định
thiết kế độc lập (tránh lỗi JSON marshal/scan, không phải cố ý vá lỗi
Postgres) mà tình cờ khiến `internal/adapter/mysql`'s `Create` KHÔNG bị
lỗi tương tự. Đây là 1 sự bất đối xứng thật giữa 2 dialect cần lưu ý:
Postgres's Create bị hỏng, MySQL's Create thì không — không phải vì MySQL
adapter "đúng hơn" theo thiết kế, mà vì cách viết `marshalLabels` tình cờ
có 1 guard nil mà bản Postgres không có. 5/13 test Postgres KHÔNG dùng
`domain.NewTask`/`Create` với `Labels` rỗng (`TestRepository_Delete_NotFound_Fails`,
`TestShareLinks_Revoke_NonexistentLink_Fails`, 3 test khác không tạo task
mới) vẫn PASS — xác nhận đây đúng là lỗi cục bộ ở `Create`, không phải
môi trường/container hỏng.

## Không thuộc phạm vi solution này

- Sửa bug `labels NOT NULL` ở `internal/adapter/postgres`/`domain.NewTask`
  (§6) — ghi nhận, không sửa.
- Đóng nguyên tắc "AddEdge's cycle check và write không atomic" hay bất kỳ
  known gap nào khác của README — ngoài phạm vi rollout MySQL.
- 6 service còn lại của batch 3+4+5 (`tenant-service`, `automation-service`,
  `workflow-service`, `auth-service`, `project-service`,
  `infra-fleet-service`) — mỗi service 1 agent riêng, xem
  `ROLLOUT-TRACKING.md`.
- Driver TiDB thật chưa test — kế thừa nguyên trạng thái đã ghi ở
  BE-DB-SOL-002 (chưa service nào trong rollout chạy thật trên
  `pingcap/tidb`).

## Liên quan

- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (RowsAffected pitfall precedent, §3.2's checklist tham chiếu)
- `backend-go/services/task-service/internal/usecase/ports.go`
- `backend-go/services/task-service/internal/adapter/postgres/*.go` (10 file, bản gốc để dịch)
- `backend-go/services/task-service/internal/adapter/mysql/*.go` (9 file, mới)
- [TASK-BE-DB-020](../tasks/TASK-BE-DB-020-task-service-mysql-rollout.md)
