# BE-DB-SOL-014: Adapter MySQL/TiDB thật cho `auth-service`

> **✅ Implemented.** Batch 3+4+5 (gộp) rollout của pattern
> BE-DB-SOL-001/002 ra `auth-service` — service lớn nhất/nhạy cảm bảo mật
> nhất đã làm tới nay trong rollout này (10 file adapter Postgres, 9
> repository interface, 20 file migration).

**CR:** [CR-DB-002](../../../../../../docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md), [CR-DB-003](../../../../../../docs/crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
**Service:** `auth-service`
**Task:** [TASK-BE-DB-019](../tasks/TASK-BE-DB-019-auth-service-mysql-rollout.md)
**Phụ thuộc cứng:** [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) — pattern dùng lại nguyên vẹn, không thiết kế lại. Tham chiếu gần nhất: [BE-DB-SOL-006](./BE-DB-SOL-006-credential-broker-service-mysql-tidb-adapter.md) (service nhạy cảm bảo mật tương tự) và [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (pitfall `RowsAffected()`).

---

## 1. `auth-service` khác các service đã rollout trước — đọc trước khi implement

Audit trực tiếp (`Read` đầy đủ `internal/usecase/ports.go` — 448 dòng,
toàn bộ 10 file `internal/adapter/postgres/*.go`, cả 20 file migration,
`cmd/server/main.go`, `internal/config/config.go`, `internal/domain/*.go`)
xác nhận:

- **9 repository interface SQL-backed** (không phải 10 như số liệu đếm
  file thô của `ROLLOUT-TRACKING.md` — 10 là số **file** trong
  `adapter/postgres/` bao gồm cả `repository.go`, file này chỉ chứa struct
  `Repository{pool}` + `New()`, không phải 1 interface riêng):
  `UserRepository`, `SessionRepository`, `ServiceTokenRepository`,
  `AccessPolicyRepository`, `AuditRepository`, `SsoIdentityRepository`,
  `SsoGroupRoleMappingRepository` (implement bởi CÙNG 1 struct
  `Repository`) + `PairingSessionRepository`, `PairedDeviceRepository`
  (implement bởi 2 struct riêng `PairingSessionStore`/`PairedDeviceStore`,
  cùng shape với `credential-broker-service`'s split, không phải
  `usage-service`'s 1-struct-duy-nhất).
- **`cmd/server/main.go` ĐÃ dùng `secrets.DatabaseCredentialsFromFile` +
  `DatabaseCredentialsFile` trong `Config` từ trước** (khác mọi service đã
  rollout trước, kể cả `usage-service` pilot) — không cần thêm field
  `DatabaseCredentialsFile` nào, chỉ cần chèn `dbcapability.DetectDialectFromDSN`
  + `switch caps.Dialect` vào đúng chỗ `pgxpool.New` hiện có.
- **KHÔNG có logic mã hoá/ký nào trong tầng SQL** — đúng ràng buộc
  "preserve password-hashing/JWT/session-token generation logic exactly":
  bcrypt hash đến từ `internal/adapter/bcrypt` (không đụng), chữ ký JWT từ
  Vault Transit qua `internal/adapter/vault` (không đụng), shared-secret
  seal/unseal của thiết bị paired cũng qua Vault Transit (không đụng). Cả
  2 adapter (Postgres/MySQL) chỉ lưu/đọc: password hash đã băm sẵn (cột
  `password_hash TEXT`), session-token-hash SHA-256 sẵn có
  (`token_hash`), JWT `jti`, và ciphertext Vault Transit trả về
  (`shared_secret_ciphertext`, `desktop_private_key_ciphertext` —
  `BYTEA`/`BLOB`, không bao giờ giải mã ở tầng này).
- **RLS thật trên 5 bảng** (`users`, `sessions`, `audit_log`,
  `sso_group_role_mapping`, `sso_identities`) — nhiều nhất trong các
  service đã rollout tới nay. Cùng phát hiện BE-DB-SOL-001 §4/BE-DB-SOL-006
  §1: không `SET LOCAL app.tenant_id` nào trong `backend-go`, không
  `FORCE ROW LEVEL SECURITY` nào trong migration — RLS này chưa từng thực
  sự enforce, application-layer `WHERE tenant_id = ?` tường minh đã là cơ
  chế bảo vệ duy nhất có thật, kể cả trên Postgres hôm nay.
- **3 method có pitfall `RowsAffected()==0`-là-not-found kiểu
  BE-DB-SOL-005 §3.1, cả 3 đều security-relevant** (khác annotation-service
  nơi pitfall chỉ ảnh hưởng data hygiene): `SessionRepository.RevokeSession`
  (session invalidation), `ServiceTokenRepository.Revoke` (CLI-token
  revocation), `PairedDeviceRepository.RevokeAndWipeSecret` (BR-MB-04 wipe
  thiết bị). Một retry hợp lệ (revoke 2 lần cùng giá trị, hoặc network
  retry) trên MySQL mặc định sẽ báo sai not-found dù dòng chắc chắn tồn
  tại — xem mục 3. Cộng thêm 2 method dùng `RETURNING` kiểu
  update-and-read (`UserRepository.UpdateUserRole`, `UpdateUser`) cần dịch
  sang UPDATE-rồi-SELECT-lại thay vì dựa `RowsAffected()`.
- **`AccessPolicyRepository.ListLatestPolicies` dùng
  `DISTINCT ON (id) ... ORDER BY id, version DESC`** — không có tương
  đương MySQL trực tiếp (khác mọi service trước, lần đầu gặp pattern này
  trong rollout) — dịch sang `ROW_NUMBER() OVER (PARTITION BY id ORDER BY
  version DESC)`, window function MySQL 8.0+/TiDB hỗ trợ.
- **`SsoGroupRoleMappingRepository.Upsert`** dùng
  `INSERT ... ON CONFLICT (...) DO UPDATE ... RETURNING` — dịch sang
  `INSERT ... ON DUPLICATE KEY UPDATE role = VALUES(role)` + SELECT lại
  theo unique key `(tenant_id, provider, group_name)` (không dùng
  `LAST_INSERT_ID()`/id truyền vào, vì trên conflict id phải là id CŨ,
  không phải id mới — xem mục 3).
- **`PairingSessionRepository.GetAndConsume`** dùng
  `UPDATE ... WHERE id=$1 AND consumed_at IS NULL RETURNING ...` —
  ĐÂY LÀ TRƯỜNG HỢP DUY NHẤT trong toàn bộ service này mà
  `RowsAffected()==0` AN TOÀN để dùng làm tín hiệu not-found, vì
  `consumed_at` chỉ chuyển NULL -> non-NULL đúng 1 lần (không có ca
  no-op-vì-giá-trị-không-đổi) — xem mục 3, không phải một ngoại lệ bị bỏ
  sót.
- **Không có `gen_random_uuid()` nào được ứng dụng thật dùng tới** — mọi
  ID sinh ở Go (`uuid.NewString()`/`usecase.newUUID()`), kể cả
  `paired_devices.id DEFAULT gen_random_uuid()` (chết, không migration
  MySQL nào cần dịch `DEFAULT`) — cùng phát hiện với mọi service trước.
- **Không có cột JSONB nào cần operator đặc biệt** — `access_policies.document`
  (JSONB->JSON) và `audit_log.metadata` (JSONB->JSON) chỉ đọc/ghi nguyên
  khối qua Go, không query nào dùng `->`/`@>`.

## 2. Chọn driver — giống hệt BE-DB-SOL-002 §2

`github.com/go-sql-driver/mysql` qua `database/sql`. Test container
`mysql:8` qua `common/testutil.StartMySQL` (không sửa).

## 3. `internal/adapter/mysql/` — 8 file mới, dịch lại đúng 9 interface

```
internal/adapter/mysql/
├── repository.go                      — struct Repository{db dbtx} + New(), dbtx/rowScanner interface local
├── user_repository.go                 — UserRepository (7 method)
├── session_repository.go              — SessionRepository (9 method)
├── service_token_repository.go        — ServiceTokenRepository (4 method)
├── access_policy_repository.go        — AccessPolicyRepository (5 method)
├── audit_repository.go                — AuditRepository (2 method)
├── sso_identity_repository.go         — SsoIdentityRepository (3 method)
├── sso_group_role_mapping_repository.go — SsoGroupRoleMappingRepository (2 method)
├── pairing_session_repository.go      — PairingSessionStore (struct riêng, 2 method)
└── paired_device_repository.go        — PairedDeviceStore (struct riêng, 6 method)
```

Không đổi signature bất kỳ method nào — `impact()` xác nhận LOW/MEDIUM cho
cả 9 interface trước khi viết (mục "gitnexus" bên dưới), không HIGH/CRITICAL
nào.

**Dịch không tầm thường, khác BE-DB-SOL-002/BE-DB-SOL-006:**

1. **`RevokeSession`/`Revoke`(service token)/`RevokeAndWipeSecret`** —
   KHÔNG dịch ngây thơ "check `RowsAffected()==0` → not-found" (bản gốc
   Postgres AN TOÀN làm vậy vì `pgconn.CommandTag.RowsAffected()` đếm
   "dòng khớp WHERE", MySQL's `sql.Result.RowsAffected()` mặc định đếm
   "dòng đổi giá trị"). Một retry hợp lệ (revoke session/token/device đã
   revoke rồi, cùng giá trị) sẽ khớp WHERE nhưng không đổi giá trị nào →
   `RowsAffected()==0` → báo sai not-found trên MySQL dù dòng chắc chắn
   tồn tại — **false negative trên đường revoke, vấn đề bảo mật thật**
   (session/token tưởng bị revoke fail trong khi thực ra đã revoke rồi;
   ngược lại, code gọi có thể log cảnh báo sai hoặc retry vô ích). Sửa:
   UPDATE vô điều kiện, rồi SELECT lại theo khoá chính để xác nhận tồn
   tại — same fix class BE-DB-SOL-005 §3.1, áp dụng ở 3 chỗ thay vì 1 vì
   auth-service có nhiều đường revoke hơn hẳn `annotation-service`.
2. **`UpdateUserRole`/`UpdateUser`** — bản Postgres dùng
   `UPDATE ... RETURNING` (không dùng `RowsAffected()`, nhưng MySQL không
   có `RETURNING`) — dịch sang UPDATE vô điều kiện + SELECT lại theo id,
   coi `sql.ErrNoRows` ở SELECT là not-found — tái hiện đúng ngữ nghĩa
   Postgres's RETURNING (chỉ fail khi id không tồn tại, không fail vì giá
   trị không đổi).
3. **`GetAndConsume`** — AN TOÀN dùng `RowsAffected()==0` làm tín hiệu
   not-found/đã-dùng, vì `consumed_at` transition NULL->non-NULL đúng 1
   lần (WHERE `consumed_at IS NULL` chặn no-op) — 2 dialect đồng nhất
   ngữ nghĩa ở đây, không cần sửa gì đặc biệt ngoài dịch cú pháp. Test
   `TestPairingSessionStore_SaveAndGetAndConsume` xác nhận GetAndConsume
   lần 2 fail đúng như bản Postgres's `pgx.ErrNoRows` trên `RETURNING`.
4. **`ListLatestPolicies`** — `DISTINCT ON (id) ORDER BY id, version DESC`
   → `ROW_NUMBER() OVER (PARTITION BY id ORDER BY version DESC)` + lọc
   `WHERE rn = 1` — MySQL 8.0+/TiDB window function, xác nhận chạy thật
   qua test `TestAccessPolicyRepository_VersioningAndListLatest` (mục 5).
5. **`SsoGroupRoleMappingRepository.Upsert`** — `ON CONFLICT DO UPDATE
   RETURNING` → `ON DUPLICATE KEY UPDATE role = VALUES(role)` + SELECT lại
   theo `(tenant_id, provider, group_name)`. Test
   `TestSsoGroupRoleMappingRepository_UpsertInsertsThenUpdates` xác nhận
   id của dòng CŨ được giữ nguyên khi conflict (không phải id mới truyền
   vào) — đúng ngữ nghĩa `EXCLUDED.role`-only-updates-role của bản gốc.
6. **`errors.Is(err, pgx.ErrNoRows)` → `errors.Is(err, sql.ErrNoRows)`**
   ở mọi Get/Find — xuyên suốt.
7. **Unique-violation** (`CreateUser` trùng `(tenant_id, email)`,
   `SsoIdentityRepository.Link` trùng `(provider, external_subject)`) —
   `pgconn.PgError.Code == "23505"` → `sqldriver.MySQLError.Number ==
   1062` (`github.com/go-sql-driver/mysql`, alias `sqldriver` để tránh
   trùng tên identifier với package `mysql` hiện tại).
8. **`ip`/`ip_address` (Postgres `INET`) → `VARCHAR(45)`** — đọc/ghi trực
   tiếp, không cần `host(ip)`-style unwrap MySQL không có `INET` để mà
   unwrap.
9. **`Query` (audit log) dynamic WHERE** — MySQL's `?` positional-theo-thứ-
   tự-xuất-hiện (không numbered như `$N`) — args slice build đồng bộ với
   clause list, không cần track index thủ công.

## 4. Compensating control cho tenant isolation — 5 bảng có RLS, nhiều nhất tới nay

`users`, `sessions`, `audit_log`, `sso_group_role_mapping` đều có ít nhất
1 method tenant-scoped được test — xem TASK-BE-DB-019's "Kết quả thực tế"
cho danh sách test `..._DoesNotLeakAcrossTenants` đầy đủ.
`sso_identities` CÓ RLS trên migration Postgres nhưng KHÔNG có method nào
trên `SsoIdentityRepository` lọc theo `tenant_id` (lookup là
`(provider, external_subject)` toàn cục — đúng thiết kế, 1 identity IdP
map 1 user duy nhất trên toàn hệ thống, không theo tenant) — không có gì
để viết test tenant-isolation cho bảng này, ghi nhận rõ thay vì giả vờ có
coverage.

## 5. Migration dialect-safe — 10 cặp file, TIMESTAMP/CHAR/VARCHAR convention theo BE-DB-SOL-006/BE-DB-SOL-005

```
backend-go/services/auth-service/migrations/
├── postgres/            (nội dung y hệt 20 file gốc, chỉ ĐỔI VỊ TRÍ, git mv)
└── mysql/                (mới, 20 file)
```

Quy ước dịch (nhất quán với `credential-broker-service`/`annotation-service`):
`UUID` → `CHAR(36)`; `TEXT PRIMARY KEY`/cột cần UNIQUE index (hash cố định
độ dài: `token_hash`, `pairing_sessions.id`, `issued_service_tokens.jti`,
`sessions.refresh_token_hash`) → `VARCHAR(N)` theo độ dài thật đã xác nhận
qua audit code (`domain.HashSessionToken` = SHA-256 hex 64 ký tự;
`jti` = base64url(32 byte) = 43 ký tự, dùng `VARCHAR(128)` cho biên độ
an toàn); `TIMESTAMPTZ` → `TIMESTAMP(6)` (+ `DEFAULT CURRENT_TIMESTAMP(6)`
khi bản gốc có `DEFAULT now()`); `INET` → `VARCHAR(45)`; `BYTEA` → `BLOB`;
`JSONB` → `JSON`; `BRIN` index → B-tree thường (MySQL/InnoDB không có
BRIN); partial index (`WHERE status='active'`, `WHERE refresh_token_hash
IS NOT NULL`) → index đầy đủ (MySQL không có partial index — ghi chú rõ
từng chỗ mất tính năng, không giấu); RLS bỏ + comment giải thích thay thế
(mục 1); `CHECK` constraint giữ nguyên (MySQL 8.0.16+ enforced). Chi tiết
đầy đủ từng file, xem chính nội dung migration (mỗi file có comment giải
thích riêng chỗ nào khác biệt hành vi thật giữa 2 dialect, không chỉ đổi
từ khoá).

`0010_audit_log_metadata.up.sql`'s `metadata JSON NOT NULL` (không
`DEFAULT '{}'`, khác Postgres's `DEFAULT '{}'::jsonb`) — mirror lý do
annotation-service's BE-DB-SOL-005: 1 deployment MySQL-dialect luôn chạy
từ migration 0001 trở đi, không có dòng cũ cần backfill retroactively khi
0010 chạy tới, và app luôn set `metadata` tường minh trong INSERT
(`Append`), default không bao giờ được đọc.

## 6. Wiring `cmd/server/main.go`

`DatabaseCredentialsFile` ĐÃ có sẵn trong `Config` (mục 1) — chỉ thêm
`dbcapability.DetectDialectFromDSN` + `switch caps.Dialect` bọc quanh
`pgxpool.New`/`sql.Open("mysql", ...)` hiện có, đúng khuôn
`usage-service`. `repo` khai báo với 1 interface ẩn danh gộp cả 7 port
implement bởi struct `Repository` chung (`UserRepository` qua
`SsoGroupRoleMappingRepository`) — auth-service's ~30 usecase constructor
call site cần các tổ hợp khác nhau của cùng 1 biến, giống lý do
`credential-broker-service`'s 3-port gộp (BE-DB-SOL-006 §6) nhưng rộng
hơn (7 port thay vì 3). `pairingSessions`/`pairedDevices` khai báo qua
interface `usecase.PairingSessionRepository`/`PairedDeviceRepository`
riêng, gán trong cùng switch (2 constructor riêng mỗi dialect, mirror
Postgres's tách-struct sẵn có). `healthSrv := health.New()` di chuyển lên
trước switch (dùng ở cả 2 nhánh); registration đổi tên theo
`caps.Dialect` (`"postgres"`/`"mysql"`) giống pilot; `"vault"` health check
giữ nguyên hoàn toàn không đổi. `toMySQLDriverDSN` copy nguyên văn từ
`usage-service`.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho cả 9 interface | Đã chạy — không HIGH/CRITICAL nào (xem TASK-BE-DB-019) | MEDIUM ở cả 9 chỉ vì fan-out import cùng package (không phải rủi ro thật, không đổi signature nào) |
| `RowsAffected()==0`-là-not-found ở 3 đường revoke bảo mật | Đã sửa + có test regression cho cả 3 | Xem mục 3.1 |
| Không có RLS-tương đương ở MySQL trên 5 bảng | Trung bình, đã compensate qua test cho 4/5 bảng (bảng thứ 5 không có method tenant-scoped để test) | Mục 4 |
| `ListLatestPolicies`'s window-function translation chưa test trên TiDB thật | Thấp-trung bình | Chỉ xác nhận trên `mysql:8`; TiDB hỗ trợ window function từ v3.0 nhưng chưa chạy thật ở đây (giống mọi service trước, TiDB thật luôn ngoài phạm vi local test) |

## Không thuộc phạm vi solution này

- Phần còn lại của batch gộp 3+4+5 (`tenant-service`, `automation-service`,
  `workflow-service`, `task-service`, `project-service`,
  `infra-fleet-service`) — xem `ROLLOUT-TRACKING.md`.
- Sửa `internal/adapter/vault`/`internal/adapter/bcrypt`/`common/secrets` —
  không đụng vào, đúng yêu cầu "chỉ retarget SQL storage mechanics, never
  security logic".
- Data migration tool / Vault infra thật.

## Liên quan

- `backend-go/services/auth-service/internal/usecase/ports.go` (9 interface)
- `backend-go/services/auth-service/internal/adapter/postgres/*.go` (bản gốc để dịch)
- `backend-go/services/auth-service/internal/adapter/mysql/*.go` (mới)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (phụ thuộc cứng)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (nguồn gốc fix `RowsAffected()` pitfall)
- [BE-DB-SOL-006](./BE-DB-SOL-006-credential-broker-service-mysql-tidb-adapter.md) (service nhạy cảm bảo mật gần nhất, split-struct pattern)
