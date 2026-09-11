# BE-DB-SOL-011: Adapter MySQL/TiDB thật cho `tenant-service`

**CR:** CR-DB-002, CR-DB-003
**Service:** `tenant-service`
**Pattern gốc:** BE-DB-SOL-001/002 (`usage-service`, pilot), replicate nguyên vẹn — không thiết kế lại. Tham khảo thêm BE-DB-SOL-005 (annotation-service, RLS-drop + UUID-column translation + MySQL RowsAffected()-pitfall) và BE-DB-SOL-006 (credential-broker-service, RLS + tenant_id column thật).

---

## 1. Audit thật (không suy đoán từ CR-DB-001's sizing cũ)

`tenant-service` có **8 repository interface** trong `internal/usecase/ports.go`
(nhiều nhất trong toàn bộ rollout tính tới thời điểm này), implement bởi
**7 struct** trong `internal/adapter/postgres/` (1 struct —
`UserProfileRepository` — implement CẢ `usecase.UserProfileRepository` LẪN
`usecase.ClientStateRepository`, 2 interface trên cùng 1 struct):

| Interface (ports.go) | Struct postgres | Bảng |
|---|---|---|
| `CompanyRepository` | `CompanyRepository` | `tenant.companies` |
| `CompanyEmailDomainRepository` | `CompanyEmailDomainRepository` | `tenant.company_email_domains` |
| `DepartmentRepository` | `DepartmentRepository` | `tenant.departments` |
| `UserProfileRepository` | `UserProfileRepository` | `tenant.user_profiles` |
| `ClientStateRepository` | `UserProfileRepository` (cùng struct) | `tenant.user_profiles` (5 cột JSON opaque) |
| `WorkspaceSessionRepository` | `UserWorkspaceSessionRepository` | `tenant.user_workspace_sessions` |
| `TeamRepository` | `TeamRepository` | `tenant.teams` + `tenant.team_members` |
| `StarNagStateRepository` | `StarNagStateRepository` | `tenant.star_nag_state` |

**12 file migration** (6 cặp up/down): `0001_init`, `0002_backfill_legacy_bootstrap_company`,
`0003_add_onboarding_state`, `0004_company_email_domains`, `0005_star_nag_state`,
`0006_client_state_and_workspace_sessions`.

**Lock-in thật xác nhận qua đọc trực tiếp** (không suy đoán):
- **UUID**: mọi cột id/company_id/department_id/user_id/team_id là `UUID`
  (companies.id, departments.id/company_id, user_profiles.user_id/company_id/department_id,
  teams.id/company_id, team_members.team_id/user_id, star_nag_state.user_id/company_id,
  company_email_domains.company_id, user_workspace_sessions.user_id/company_id).
  ID sinh ở Go (`uuid.NewString()`-style, xác nhận qua đọc usecase — không
  `gen_random_uuid()` nào trong 12 file migration, `grep` xác nhận 0 kết quả).
- **JSONB**: `companies.settings_json`, `departments.settings_json`,
  `teams.settings_json`, `user_profiles.settings_json`,
  `user_profiles.onboarding_state_json`, `star_nag_state.active_prompt` — 6
  cột JSONB. `user_profiles`'s 5 cột client-state (`keybindings_json` etc.)
  và `user_workspace_sessions.session_json` là `TEXT`, KHÔNG JSONB (migration
  0006's comment tự giải thích: "opaque frontend JSON... no need for JSONB's
  indexing/containment operators") — không cần dịch.
- **RLS**: **5 bảng có policy** — `departments`, `user_profiles`, `teams`,
  `team_members`, `user_workspace_sessions` (0001_init.up.sql + 0006's mới
  thêm). `companies` và `company_email_domains` và `star_nag_state` KHÔNG có
  RLS (companies là tenant root tự thân, xác nhận qua migration's chính
  comment; 2 bảng còn lại chưa từng được thêm policy). Đây là **số RLS
  policy nhiều nhất trong rollout tính tới batch này** — khớp với mô tả
  nhiệm vụ gốc ("this service likely has the most RLS policies... given it's
  the tenancy root").
- **`RETURNING`**: `CompanyRepository.Update` và `DepartmentRepository.Update`
  dùng `RETURNING id, ..., settings_json` sau `UPDATE` — 2 điểm dịch không
  tầm thường (không `RETURNING` tương đương ở MySQL).
- **`BIGSERIAL`**: KHÔNG dùng ở bất kỳ đâu — mọi PK là UUID sinh ở Go.

## 2. Migration dialect-safe — điểm dịch không tầm thường

Tách `migrations/postgres/` (`git mv`, nội dung y hệt, 12 file) +
`migrations/mysql/` (mới, 12 file). Quy ước tên database/bảng theo
BE-DB-SOL-002 §"Quyết định tên bảng/schema MySQL": `DATABASE_DSN` trỏ thẳng
database MySQL tên `tenant` (song song schema Postgres `tenant`), bảng
KHÔNG prefix (`companies`, không phải `tenant_companies`).

- `UUID` → `CHAR(36)` (mọi cột id/company_id/... liệt kê ở §1).
- `JSONB` → `JSON`, `DEFAULT '{}'` → `DEFAULT (JSON_OBJECT())` — cú pháp
  expression-default MySQL 8.0.13+ (xác nhận chạy được thật trên `mysql:8`,
  image `common/testutil.StartMySQL` dùng — xem §5 test thật).
- `TIMESTAMPTZ` → `TIMESTAMP(6)`, `now()`/`DEFAULT now()` → `NOW(6)`/
  `DEFAULT CURRENT_TIMESTAMP(6)` (giữ độ chính xác micro-giây khớp cột).
- **RLS dropped** trên cả 5 bảng có policy, với comment giải thích trực tiếp
  trong migration — theo đúng phát hiện BE-DB-SOL-001 §4 (không có dòng code
  Go nào từng gọi `SET LOCAL app.tenant_id` trong toàn bộ `backend-go`, nên
  RLS trên Postgres **chưa từng thực sự enforce** — bỏ nó ở MySQL không hạ
  mức bảo vệ thật nào, chỉ gỡ 1 lớp tài liệu/ý định chưa từng vận hành).
  Compensating control thật = filter `company_id`/`companyID` tường minh
  trong mọi query của `internal/adapter/mysql` (đã có sẵn từ trước, không
  phải viết mới) + test tenant-isolation-without-RLS mới (§4).
- **`TEXT` dùng làm PRIMARY KEY/composite key → `VARCHAR`** (InnoDB yêu cầu
  key-length tường minh cho `TEXT` trong index, giới hạn 3072 byte/key,
  cùng pattern BE-DB-SOL-005 đã dùng cho `annotation-service`):
  - `company_email_domains.email_domain` (`TEXT PRIMARY KEY` →
    `VARCHAR(255) PRIMARY KEY` — domain DNS tối đa 253 ký tự, không mất dữ
    liệu thật).
  - `user_workspace_sessions.host_id` (phần của composite
    `PRIMARY KEY (user_id, host_id)` → `VARCHAR(255)` — cột này đã tự mô tả
    là short identifier ("'local'/environmentId"), không phải free text).
  - `team_members.role` (không phải key, nhưng narrowed `TEXT` →
    `VARCHAR(32)` để tránh phụ thuộc cú pháp expression-default của MySQL
    cho 1 cột **hoàn toàn không được Go code nào đọc/ghi** — xác nhận qua
    đọc `team_repository.go`: `AddMember` không set `role`,
    `domain.TeamMember` không có field `Role`. Không đổi hành vi ứng dụng.
  - `companies.name`/`departments.name`/`teams.name` giữ `TEXT` (không nằm
    trong index/PK nào, filter bằng `=` bình thường vẫn hợp lệ trên MySQL).
- `RETURNING` (2 điểm, §1) → luôn `UPDATE` (kể cả no-op) rồi `SELECT` lại
  theo khóa — **KHÔNG bao giờ dùng `RowsAffected()==0` để quyết định
  not-found** (xem §3 pitfall).

## 3. `internal/adapter/mysql/` — 7 file, implement đúng 8 interface

1 file/struct, mirror 1:1 tên với `internal/adapter/postgres/` (trừ
`settings_json.go` dùng chung logic marshal/unmarshal, tách riêng theo
package `mysql`):

`company_repository.go`, `company_email_domain_repository.go`,
`department_repository.go`, `star_nag_state_repository.go`,
`team_repository.go`, `user_profile_repository.go`,
`user_workspace_session_repository.go`, `settings_json.go`.

**Pitfall MySQL RowsAffected() (BE-DB-SOL-005's phát hiện, áp dụng lại ở
đây)**: `CompanyRepository.Update`/`DepartmentRepository.Update` — bản gốc
Postgres dùng `RETURNING` + `Scan` → `pgx.ErrNoRows` để phát hiện not-found,
KHÔNG dùng `RowsAffected()`. Dịch ngây thơ sang MySQL dễ sai theo hướng
ngược ("check `RowsAffected()==0` sau UPDATE rồi mới SELECT") vì MySQL đếm
"dòng ĐỔI GIÁ TRỊ", không phải "dòng khớp WHERE" — 1 patch no-op-value hợp
lệ (ví dụ đổi `Name` về đúng giá trị cũ) sẽ bị báo nhầm not-found. Giải
pháp áp dụng: **luôn `UPDATE` (kể cả no-op) rồi `SELECT` lại theo khóa**,
không bao giờ đọc `RowsAffected()` cho các method này —
`found=false` chỉ có nghĩa "không có row nào khớp khóa", xác nhận bằng
`Get`/`SELECT` sau đó, không phải suy luận từ `UPDATE`'s kết quả. Test
`TestCompanyRepository_Update_AppliesNonEmptyFieldsOnly` (mục 5) xác nhận
trực tiếp bằng 1 no-op-value update.

**Method KHÔNG bị ảnh hưởng bởi pitfall trên** —
`TeamRepository.RemoveMember` (DELETE): `RowsAffected()` sau DELETE đếm theo
WHERE-match ở MỌI dialect (không có ambiguity "đổi giá trị" — ambiguity đó
chỉ tồn tại với UPDATE), nên đọc trực tiếp an toàn, giữ nguyên 1:1 logic
Postgres.

**Upsert dịch `ON CONFLICT ... DO UPDATE` → `ON DUPLICATE KEY UPDATE`**, áp
dụng cho: `CompanyEmailDomainRepository.Add`, `StarNagStateRepository.Save`,
`TeamRepository.AddMember`, `UserProfileRepository.Upsert`/
`SetOnboardingState`/`SetClientStateColumn`,
`UserWorkspaceSessionRepository.Set`/`Patch` — không method nào trong nhóm
này dựa vào `RowsAffected()` cho ngữ nghĩa not-found (chúng luôn ghi thành
công, không có khái niệm "not found" ở upsert), nên không dính pitfall trên.

**`UserWorkspaceSessionRepository.Patch`** — giữ nguyên transaction shape
1:1 (`SELECT ... FOR UPDATE` trong 1 transaction để lock hàng trước
read-modify-write, InnoDB hỗ trợ cú pháp này giống hệt Postgres qua
`database/sql`'s `*sql.Tx`), chỉ đổi `errors.Is(err, pgx.ErrNoRows)` →
`errors.Is(err, sql.ErrNoRows)`.

**`clientStateColumn` whitelist** (chống SQL injection qua tên cột động,
`GetClientStateColumn`/`SetClientStateColumn`) — copy nguyên switch-case từ
bản Postgres, không đổi logic, chỉ đổi package.

## 4. Test tenant-isolation-without-RLS (TASK-BE-DB-003's pattern)

migrations/mysql KHÔNG có RLS trên 5 bảng liệt kê ở §1 — viết test MỚI
(không có ở bản Postgres, vì Postgres "có" RLS dù nó không thực sự chạy)
cho từng bảng, seed 2 company với dữ liệu/khóa cố ý trùng hình dạng, xác
nhận không rò rỉ chéo:

- `TestDepartmentRepository_List_DoesNotLeakAcrossTenants` — 2 department
  cùng tên "Engineering" ở 2 company khác nhau, `List` company A chỉ thấy
  department của mình.
- `TestTeamRepository_ListByCompany_DoesNotLeakAcrossTenants` — tương tự
  cho `teams`.
- `TestUserProfileRepository_ClientStateColumn_DoesNotLeakAcrossTenants` —
  cùng `user_id` ở 2 company, `GetClientStateColumn`/`ListUserIDsByCompany`
  scoped đúng company.
- `TestUserWorkspaceSessionRepository_DoesNotLeakAcrossTenants` — **quan
  trọng nhất**: `user_workspace_sessions`' PRIMARY KEY là `(user_id,
  host_id)`, KHÔNG có `company_id` trong khóa — cố ý dùng CÙNG
  `(user_id, host_id)` ở 2 company khác nhau, đây là hình dạng chính xác mà
  1 query thiếu filter `company_id` sẽ trả nhầm dữ liệu company khác thay vì
  not-found.
- `team_members` không có test riêng: mọi method (`ListMembers`,
  `RemoveMember`) chỉ nhận `teamID` (không nhận `companyID` trực tiếp) —
  cách ly tenant của nó phụ thuộc hoàn toàn vào `teamID` đã được resolve
  đúng company ở lớp usecase/`TeamRepository.Get` trước đó, không có gì
  thêm để test ở tầng repository này riêng cho `team_members`.

## 5. Wiring `cmd/server/main.go` — khác biệt lớn nhất so với pilot

`usage-service`'s `run()` chỉ có 1 biến `repo usecase.Repository` — pilot
không cần tổ hợp interface. `tenant-service` có **8 interface từ 7 struct**,
nhiều usecase constructor cần các tổ hợp khác nhau của CÙNG 1 biến
(`profiles` phải thỏa cả `usecase.UserProfileRepository` lẫn
`usecase.ClientStateRepository`). Go không tự "mở rộng" method set khi gán
1 giá trị kiểu interface A cho biến kiểu interface B trừ khi A đã là
superset của B tại compile time — nên 1 biến `profiles
usecase.UserProfileRepository` KHÔNG thể truyền thẳng vào hàm nhận
`usecase.ClientStateRepository`, dù struct cụ thể implement cả 2.

Giải pháp: khai báo 1 interface ẩn danh cục bộ gộp cả 2 port (**cùng pattern
`credential-broker-service`'s main.go đã dùng cho biến `repo` gộp 3 port** —
BE-DB-SOL-006 §4, không phải giải pháp mới sáng tác riêng cho service này):

```go
type profileRepository interface {
    usecase.UserProfileRepository
    usecase.ClientStateRepository
}
var profiles profileRepository
```

7 biến khai báo trước `switch caps.Dialect` (kiểu interface `usecase.*`),
gán bên trong mỗi case (`tenantpostgres.NewXRepository(pool)` hoặc
`tenantmysql.NewXRepository(db)`) — đúng khuôn `switch caps.Dialect` của
`usage-service`, `toMySQLDriverDSN` copy nguyên văn (thuần DSN-plumbing,
không đặc thù service nào, đã copy giống hệt ở BE-DB-SOL-005/006).
`internal/config/config.go` thêm `DatabaseCredentialsFile` (cùng tên/default
`/vault/secrets/database-credentials` như mọi service khác) — `main.go`
trước đó đọc thẳng `cfg.DatabaseDSN` + tự check rỗng, nay chuyển sang
`secrets.DatabaseCredentialsFromFile` + `dbcapability.DetectDialectFromDSN`,
cùng 1 đoạn code cần sửa để chọn dialect (không mở rộng phạm vi).

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `impact()` cho 8 interface + `run`/`Load` | Đã chạy — tất cả **LOW** | Xem TASK-BE-DB-016's "Kết quả thực tế" mục 1 |
| Interface ẩn danh gộp port cho biến `profiles` | Thấp | Pattern đã dùng ở `credential-broker-service`, không phải thiết kế mới |
| `RowsAffected()` pitfall trên `Update` (Company/Department) | Trung bình nếu bỏ sót | Đã tránh hoàn toàn bằng always-UPDATE-then-SELECT, xem §3 |
| `user_workspace_sessions` PK không có `company_id` | Cao nếu thiếu filter | Test riêng `DoesNotLeakAcrossTenants` xác nhận không rò rỉ, xem §4 |
| RLS drop trên 5 bảng | Thấp (theo BE-DB-SOL-001 §4's phát hiện) | RLS Postgres chưa từng thực sự enforce; application-layer filtering đã là bảo vệ thật duy nhất từ trước |

## Không thuộc phạm vi solution này

- 6 service còn lại của batch 3+4+5 (`automation-service`, `workflow-service`,
  `auth-service`, `task-service`, `project-service`, `infra-fleet-service`) —
  agent riêng, ngoài phạm vi.
- Sửa `common/dbcapability`/`common/testutil` — dùng lại nguyên vẹn, không
  phát hiện bug nào cần flag.
- Data migration tool chuyển dữ liệu khách hàng thật Postgres → MySQL/TiDB.

## Liên quan

- `backend-go/services/tenant-service/internal/usecase/ports.go` (8 interface)
- `backend-go/services/tenant-service/internal/adapter/postgres/*.go` (bản gốc để dịch)
- `backend-go/services/usage-service/cmd/server/main.go` (`run` — khuôn `switch caps.Dialect`)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md), [BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (RLS-drop, UUID-column translation, MySQL RowsAffected() pitfall)
- [BE-DB-SOL-006](./BE-DB-SOL-006-credential-broker-service-mysql-tidb-adapter.md) (anonymous combined-interface pattern cho biến repo đa-port)
- [TASK-BE-DB-016](../tasks/TASK-BE-DB-016-tenant-service-mysql-rollout.md) (task doc, "Kết quả thực tế" đầy đủ)
