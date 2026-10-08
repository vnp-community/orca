# TASK-REQ-033-01: Migration `0039` hồ sơ năng lực, domain `CapabilityProfile` và repository hai dialect

**From Solution:** [BE-REQ-SOL-033](../solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md) mục 2.C, 2.D
**Priority:** P1
**Service/Area:** `infra-fleet-service` / migration, domain, adapter postgres và mysql
**File:** `backend-go/services/infra-fleet-service/migrations/postgres/0039_dev_server_capability_profiles.{up,down}.sql` (mới), `migrations/mysql/0039_dev_server_capability_profiles.{up,down}.sql` (mới), `internal/domain/capability_profile.go` (mới), `internal/domain/agent_features.go` (mới), `internal/usecase/capability_ports.go` (mới), `internal/adapter/postgres/capability_profile_repository.go` (mới), `internal/adapter/mysql/capability_profile_repository.go` (mới), và các `_test.go`
**Depends on:** không (task đầu của solution)
**Status:** [x] DONE (đã kiểm chứng 2026-10-07: `go test -tags integration ./internal/adapter/postgres/ -run "Capability|Migration0039"` trên postgres:16-alpine thật; `go test -tags integration ./internal/adapter/mysql/ -run "Capability|Migration0039"` trên MySQL 8 thật; `go test ./...` module infra-fleet-service)

---

## Context

Đã đọc ngày 2026-10-06:
- `ls backend-go/services/infra-fleet-service/migrations/postgres | tail` dừng ở `0038_session_origin.{up,down}.sql`; bản `mysql` cũng dừng ở `0038`. Số tiếp theo là `0039`; **chạy lại `ls` ngay trước khi tạo file**, vì CR-REQ-031 có thể thêm migration cho cùng service.
- RLS mẫu: `0030_dev_server_approval_status_and_groups.up.sql` dòng 25 đến 26 dùng `ENABLE ROW LEVEL SECURITY` và `CREATE POLICY tenant_isolation ... USING (tenant_id = current_setting('app.tenant_id', true)::uuid)`. Chú ý: ở `task-service` RLS không chạy thật vì không có `set_config` (README feature request-service-foundation), `infra-fleet-service` cũng chưa chắc có. Repository vẫn **phải** lọc `tenant_id` trong mọi `WHERE`, đừng dựa vào RLS.
- Mẫu repository: `internal/adapter/postgres/dev_server_group_repository.go` và bản mysql; mẫu `shared.go` ở mysql. Tên schema Postgres là `infra.`, MySQL không có tiền tố schema.
- `internal/domain/dev_server.go` có `DevServer` (PK `id`). Bảng mới có FK tới `infra.dev_servers(id)`.
- MySQL: không DEFAULT literal cho cột JSON; `CHECK` cần MySQL 8.0.16 trở lên (đã là điều kiện chung của feature request-service-foundation).

Khác với CR gốc: CR viết `features JSON/JSONB` và `profile JSON/JSONB` không nói default; ở đây chốt `NOT NULL` và ghi giá trị rỗng từ ứng dụng (`[]`, `{}`) để dialect nào cũng giống nhau.

## Việc cần làm

1. Tạo `0039_dev_server_capability_profiles.up.sql` Postgres đúng như solution mục 2.C: PK `dev_server_id UUID REFERENCES infra.dev_servers(id) ON DELETE CASCADE`, `tenant_id UUID NOT NULL`, `source TEXT CHECK (source IN ('probe','handshake_only'))`, `agent_build_version TEXT NOT NULL DEFAULT ''`, `protocol_version INT NOT NULL DEFAULT 1`, `features JSONB NOT NULL DEFAULT '[]'::jsonb`, `profile JSONB NOT NULL DEFAULT '{}'::jsonb`, `fingerprint CHAR(64) NOT NULL`, `probed_at TIMESTAMPTZ NOT NULL`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`:
   - chỉ mục `idx_dev_server_capability_profiles_tenant (tenant_id, probed_at DESC)`
   - RLS như `0030`.
2. Tạo bản `.down.sql` Postgres: `DROP TABLE IF EXISTS infra.dev_server_capability_profiles;`.
3. Tạo bản MySQL tương ứng: `CHAR(36)` cho id, `source VARCHAR(16) NOT NULL` kèm `CONSTRAINT dev_server_capability_profiles_source_check CHECK (...)`, `features JSON NOT NULL`, `profile JSON NOT NULL` (không DEFAULT), `probed_at TIMESTAMP(6) NOT NULL`, `updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)`, `CONSTRAINT fk_dev_server_capability_profiles_dev_server FOREIGN KEY (dev_server_id) REFERENCES dev_servers(id) ON DELETE CASCADE`. Kiểm `SHOW CREATE TABLE dev_servers` để khớp kiểu cột `id` (nếu `CHAR(36)` thì giữ).
4. `internal/domain/agent_features.go`: các hằng `FeatureAgentExecPrompt = "agent.execPrompt"`, `FeatureReadonly = "agent.execPrompt.readonly"`, `FeatureWorkspaceKind = "agent.execPrompt.workspaceKind"`, `FeatureChanges = "agent.execPrompt.changes"`, `FeatureResultBlock = "agent.execPrompt.resultBlock"`, `FeatureCapabilities = "agent.capabilities"`, `FeatureAIComplete = "ai.complete"`, `FeatureAICompleteUsage = "ai.complete.usage"`.
5. `internal/domain/capability_profile.go`: kiểu `ProfileSource` (`ProfileSourceProbe`, `ProfileSourceHandshakeOnly`), `CapabilityProfile` với các trường ở solution 2.D, `Degraded()`, `HasFeature(name)`, `NormalizeFeatures([]string) []string` (sắp xếp, bỏ trùng, bỏ chuỗi rỗng, cắt tối đa 64 phần tử), và `ComputeFingerprint(profileJSON []byte) (string, error)`: giải JSON, xoá các khoá biến động (`probedAt`, `host.memFreeMb`, `host.diskFreeMb`, `host.loadAvg1`), mã hoá lại với khoá sắp xếp và không khoảng trắng, SHA-256 hex. JSON rỗng hoặc `{}` cho một fingerprint cố định hợp lệ.
6. `internal/usecase/capability_ports.go`: `CapabilityProfileStore` với `Get(ctx, tenantID, devServerID string) (domain.CapabilityProfile, bool, error)` và `Upsert(ctx, p domain.CapabilityProfile) (previousFingerprint string, existed bool, err error)`. `Upsert` đọc fingerprint cũ và ghi mới trong cùng một giao dịch để use case biết có đổi hay không (tránh đua giữa hai lần probe).
7. Cài `Upsert` Postgres: `INSERT ... ON CONFLICT (dev_server_id) DO UPDATE SET ...` kèm `SELECT fingerprint ... FOR UPDATE` trong giao dịch; chỉ cập nhật khi `tenant_id` khớp (`WHERE ... .tenant_id = EXCLUDED.tenant_id`), tenant khác trả lỗi `domain.ErrNotFound`.
8. Cài `Upsert` MySQL: `INSERT ... ON DUPLICATE KEY UPDATE`, đọc fingerprint cũ bằng `SELECT ... FOR UPDATE` trong cùng giao dịch.
9. Cài `Get` ở cả hai adapter, luôn có `WHERE tenant_id = ? AND dev_server_id = ?`.

## Kiểm thử

- `TestComputeFingerprint_IgnoresVolatileKeys`, `TestComputeFingerprint_KeyOrderIndependent`, `TestComputeFingerprint_DiffersOnToolVersion`, `TestNormalizeFeatures_SortsDedupes`, `TestCapabilityProfile_Degraded`.
- Integration (`-tags=integration`, theo mẫu `dev_server_group_repository_test.go` và `migration_0018_fleet_definitions_test.go`): `TestCapabilityProfileRepository_UpsertGet`, `_UpsertReturnsPreviousFingerprint`, `_TenantIsolation` (tenant B không thấy hồ sơ của A), `_CascadeOnDevServerDelete`, `_UnicodeInProfile` (tiếng Việt có dấu trong `profile`), `TestMigration0039_UpDownUp` (hai dialect).
- Postgres RLS: một test dùng role không phải superuser nếu hạ tầng test hỗ trợ; nếu không thì ghi vào Rủi ro, không bỏ qua im lặng.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/infra-fleet-service/internal/domain/... ./services/infra-fleet-service/internal/usecase/...` rồi `go test -tags=integration ./services/infra-fleet-service/internal/adapter/postgres/... ./services/infra-fleet-service/internal/adapter/mysql/...`.

## Tiêu chí hoàn thành

- [x] Migration lên, xuống, lên sạch trên Postgres 14+ và MySQL 8.0.16+. (TestMigration0039_UpDownUp: Postgres 16 và MySQL 8 thật)
- [x] `CHECK` từ chối `source` lạ ở cả hai dialect. (TestCapabilityProfileRepository_SourceCheckRejectsUnknown hai dialect)
- [x] Xoá dev server xoá hồ sơ (cascade).
- [x] `Upsert` trả `previousFingerprint` đúng; hai `Upsert` đồng thời không làm mất sự khác biệt (một trong hai thấy `existed=true`). (TestCapabilityProfileRepository_ConcurrentUpsertsSerialize: khoá hàng dev_servers, đúng một bên thấy existed=true)
- [x] Không file nào tên `helpers`, `utils`, `common`, `misc`; không thêm `max-lines` disable. (không thêm max-lines disable)
- [x] `buf` không liên quan (task này không đổi proto).

## Thứ tự thực hiện gợi ý

1. Commit 1: hai cặp migration (`up`/`down`) và `TestMigration0039_UpDownUp`.
2. Commit 2: `agent_features.go`, `capability_profile.go` (domain thuần) cùng test fingerprint; chưa có DB.
3. Commit 3: port `CapabilityProfileStore` và adapter Postgres kèm integration test; commit 4: adapter MySQL cùng bộ test bảng để hai dialect có cùng hành vi.
4. Chạy `gitnexus_impact` cho `Repository` của `infra-fleet-service` nếu thêm phương thức vào struct có sẵn (nên dùng struct riêng `CapabilityProfileRepository` để tránh chạm struct lớn).

## Kiểm tra thủ công sau khi xong

- `psql` vào DB `infra`: `\d infra.dev_server_capability_profiles`, kiểm RLS bằng `SELECT * FROM pg_policies WHERE tablename = 'dev_server_capability_profiles'`.
- Chèn một hồ sơ mẫu bằng test, xoá dev server tương ứng, xác nhận hồ sơ biến mất (cascade).

## Điểm cần người review soi kỹ

- `Upsert` đọc fingerprint cũ và ghi mới trong **cùng** giao dịch (nếu tách hai câu lệnh thì sự kiện `capabilities_changed` có thể phát hai lần hoặc không phát).
- Không câu SQL nào thiếu `tenant_id` trong `WHERE`; test `TenantIsolation` phải thử cả `Get` lẫn `Upsert` với tenant sai.
- `NormalizeFeatures` không làm thay đổi thứ tự có ý nghĩa nào (feature là tập, không là danh sách có thứ tự); fingerprint không phụ thuộc thứ tự của agent gửi.

## Rủi ro và lưu ý

- FK sang `dev_servers`: nếu bảng này ở MySQL dùng kiểu id khác thì FK lỗi lúc migrate; kiểm `SHOW CREATE TABLE` trước.
- Nếu sau này có dev server xoá mềm thay vì xoá cứng thì hồ sơ mồ côi; hiện `dev_servers` xoá cứng (chưa kiểm chứng toàn bộ đường xoá).
- `profile` có thể lớn (hàng chục công cụ): giới hạn 64 KB ở `Upsert` (từ chối với `domain.ErrProfileTooLarge`) để không phình bảng.

## Ghi chú triển khai (2026-10-07)

- Migration `0039` Postgres dùng `FORCE ROW LEVEL SECURITY`, `NULLIF(current_setting('app.tenant_id', true), '')::uuid` và `WITH CHECK`; store đặt `set_config('app.tenant_id', $1, true)` trong mọi giao dịch (đọc cũng vậy). Test `RLSEnforcedForNonSuperuser` đổi sang role thường để chứng minh RLS thật: tenant khác và thiếu GUC đều thấy 0 hàng.
- `Upsert` khoá hàng `dev_servers` (Postgres `FOR NO KEY UPDATE`, MySQL `FOR UPDATE`) trước khi đọc fingerprint cũ, thay cho `INSERT ... SELECT` của bản đầu (bản đó cho hai probe đồng thời cùng thấy `existed=false` và nuốt lỗi tenant lạ thành "0 hàng").
- MySQL: cột JSON phải nhận `string`, không phải `[]byte` (charset binary bị từ chối); `agent_build_version` đổi từ `TEXT NOT NULL` sang `VARCHAR(128) NOT NULL DEFAULT ''`.
- `NormalizeFeatures` sắp xếp rồi mới cắt 64 phần tử (bản đầu cắt trước, giữ tập khác nhau tuỳ thứ tự agent gửi).
- Thêm `DevServerTenantLookup` (`TenantIDForDevServer`) trên cùng store, cần cho đường kích hoạt sau handshake (phiên chỉ biết `devServerID`).
- Cột `fingerprint CHAR(64)` đệm khoảng trắng nếu ngắn hơn 64: store cắt khoảng trắng phải khi đọc; fingerprint thật luôn là SHA-256 hex nên không ảnh hưởng.
