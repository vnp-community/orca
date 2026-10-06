# TASK-REQ-027-01: Migration `NNNN_request_artifact_model` (hai dialect) và domain `RequestRevision`

**From Solution:** BE-REQ-SOL-027
**Priority:** P0
**Service:** `request-service`
**File:** `migrations/postgres/NNNN_request_artifact_model.{up,down}.sql`, `migrations/mysql/NNNN_request_artifact_model.{up,down}.sql`, `internal/domain/request_revision.go`, `internal/domain/request.go` (sửa), `internal/domain/solution.go` (sửa) và test (mới trừ các file sửa)
**Depends on:** TASK-REQ-002-01 (bảng `requests`, `solutions`), TASK-REQ-002-02 (domain)
**Status:** [ ] TODO

---

## Context

**Số migration không tự cấp.** Số `NNNN` của `request-service` đang chồng giữa CR-REQ-001/002/004/006 (xem README `request-artifact-model`); `request-service/migrations` chưa tồn tại lúc soạn. Bước 1 là `ls backend-go/services/request-service/migrations/postgres migrations/mysql`, lấy số kế tiếp lớn nhất và dùng **cùng số cho hai dialect**.

Quy ước theo BE-REQ-SOL-002 mục A: Postgres schema `request.`, `UUID`/`TIMESTAMPTZ`/`JSONB`, RLS `tenant_isolation` với `NULLIF(current_setting('app.tenant_id', true), '')::uuid` + `FORCE` (không dùng cách của `task-service` vì inert); MySQL `CHAR(36)`/`TIMESTAMP(6)`/`JSON`/`CHAR(64)` cho digest, MySQL không cho `DEFAULT` literal trên cột JSON ở mọi bản nên ứng dụng **luôn ghi giá trị**, và mọi truy vấn có `tenant_id`. `solutions` do SOL-002 tạo với `options` và `chosen_option INT NULL`; `requests.body` hiện là "phát biểu vấn đề".

Quyết định gộp hay tách: nếu migration `0002_request_core` chưa merge thì gộp các cột `solutions` (seq, schema_version, provenance, input_request_revision, content_digest) và `requests` vào đó (bớt backfill); nếu đã merge thì dùng file này kèm backfill (bước 3).

## Việc cần làm

1. Đọc thư mục migrations, chọn `NNNN`, ghi vào mô tả PR cùng quyết định gộp hay tách.
2. `requests`: thêm `content_schema_version INT NOT NULL DEFAULT 1`, `acceptance_criteria` (`JSONB NOT NULL DEFAULT '[]'` / MySQL `JSON NOT NULL` không default), `type_fields` (`'{}'`), `content_revision INT NOT NULL DEFAULT 1`, `content_digest TEXT NOT NULL DEFAULT ''` (MySQL `CHAR(64) NOT NULL DEFAULT ''`).
   - Backfill dòng có sẵn: `acceptance_criteria='[]'`, `type_fields='{}'`.
3. `solutions`: thêm `seq INT NULL`, `schema_version INT NOT NULL DEFAULT 1`, `provenance` (`JSONB`/`JSON` `NOT NULL`, giá trị `'{}'`), `input_request_revision INT NOT NULL DEFAULT 1`, `content_digest` (như trên).
   - Backfill `seq` bằng `ROW_NUMBER() OVER (PARTITION BY tenant_id, request_id ORDER BY created_at, id)` (Postgres `UPDATE ... FROM (SELECT ...)`; MySQL 8.0 `UPDATE solutions s JOIN (SELECT id, ROW_NUMBER() OVER (...) rn FROM solutions) t ON ...`)
   - sau đó `ALTER COLUMN seq SET NOT NULL` (MySQL `MODIFY COLUMN seq INT NOT NULL`) và `UNIQUE (tenant_id, request_id, seq)`.
4. `request_revisions`: `id`, `tenant_id`, `request_id`, `revision INT NOT NULL`, `cause` CHECK `created|clarification_answered|edited|type_changed`, `snapshot` JSON NOT NULL, `digest`, `actor_id TEXT NULL` (MySQL `VARCHAR(64) NULL`), `actor_kind` CHECK `ai|user|system`, `clarification_id` NULL, `created_at`;
   - `UNIQUE (tenant_id, request_id, revision)`.
   - Chỉ thêm, không có `UPDATE` nào: ghi chú trong migration.
5. `artifact_index`: `tenant_id`, `display_id` (`TEXT`/`VARCHAR(80)`), `kind` CHECK `request|solution|option|plan|phase|task`, `request_id`, `artifact_id`, `created_at`;
   - `PRIMARY KEY (tenant_id, display_id)`, `INDEX (tenant_id, artifact_id)`.
   - `artifact_relations`: `id`, `tenant_id`, `request_id`, `rel` CHECK `derived_from|implements|supersedes|evidenced_by|verifies`, `from_kind`, `from_id`, `to_kind`, `to_id` (MySQL `VARCHAR(160)` cho id dạng `SOL-142.2/opt-1`), `created_by_run_id NULL`, `created_at`
   - UNIQUE `(tenant_id, rel, from_kind, from_id, to_kind, to_id)` (MySQL: kiểm tổng độ dài khoá ≤ 3072 byte với `utf8mb4`, rút `VARCHAR` nếu vượt)
   - `INDEX (tenant_id, request_id)`.
   - `request_coverage`: `id`, `tenant_id`, `request_id`, `plan_task_id`, `ac_id` (`VARCHAR(16)`), `task_id`, `check_id NULL`, `created_at`
   - `INDEX (tenant_id, request_id, plan_task_id)`.
6. RLS: `ENABLE`/`FORCE` và policy `tenant_isolation` cho bốn bảng mới (khối `DO $$ ... FOREACH` như `mcp-service/0001`).
   - `down` cả hai dialect: `DROP TABLE` ngược thứ tự
   - xoá cột đã thêm của `solutions` và `requests`.
7. Domain: `RequestRevision{ID, TenantID, RequestID string; Revision int; Cause RevisionCause; Snapshot []byte; Digest, ActorID string; ActorKind ActorKind; ClarificationID string; CreatedAt time.Time}`;
   - `RevisionCause` bốn hằng
   - `ParseRevisionCause`.
   - `domain.Request` thêm `ContentSchemaVersion, ContentRevision int; AcceptanceCriteriaJSON, TypeFieldsJSON []byte; ContentDigest string`.
   - `domain.Solution` thêm `Seq, SchemaVersion, InputRequestRevision int; ProvenanceJSON []byte; ContentDigest string`.
   - Chỉ thêm trường
   - `NewRequest` đặt `ContentRevision=1`, `[]byte("[]")`, `[]byte("{}")`.
8. Cập nhật hai repository `Create/Update/Get` của `requests` và `solutions` (TASK-REQ-002-04/05) để đọc, ghi các cột mới (đọc: luôn trả; ghi: `Update` không đụng cột nội dung, chỉ `AppendRequestRevision` ở task 027-05 được ghi, nên thêm phương thức riêng `UpdateContent` ở đó, không ở đây).

## Kiểm thử

- `TestSchemaContract_RequestArtifactModel`: đọc `information_schema` hai DB, so tên bảng, cột, NULL, kiểu với danh sách trong test (mở rộng file hợp đồng của TASK-REQ-002-06).
- `TestMigration_UpDownUp_BothDialects`; `TestBackfill_SolutionSeq_PerRequestOrdered` (chèn 5 Solution của 2 Request trước khi chạy migration, kiểm `seq` 1..3 và 1..2, UNIQUE giữ).
- `TestRequestRevisions_UniquePerRevision`
- `TestRequestRevisions_CauseCheck`
- `TestArtifactRelations_UniqueTuple`
- `TestArtifactIndex_PrimaryKeyTenantScoped`.
- `TestRLS_ArtifactTables_NoBypassRole` (Postgres, role `NOSUPERUSER NOBYPASSRLS`).
- `TestDomain_NewRequest_ContentDefaults`
- `TestParseRevisionCause`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'Revision|NewRequest' && go test -tags=integration ./services/request-service/internal/adapter/... -run 'SchemaContract|Migration|Backfill|RequestRevisions|ArtifactRelations|ArtifactIndex|RLS'`.

## Tiêu chí hoàn thành

- [ ] `up`, `down`, `up` chạy sạch trên Postgres 14+ và MySQL 8.0.16+; `NNNN` ghi trong PR.
- [ ] Backfill `seq` đúng thứ tự theo Request; UNIQUE `(tenant_id, request_id, seq)` hoạt động.
- [ ] Bốn bảng mới có RLS thật ở Postgres và `tenant_id` trong khoá hoặc chỉ mục ở MySQL.
- [ ] Bộ test hợp đồng cũ của TASK-REQ-002-06 vẫn xanh.
- [ ] Domain mới không import gì ngoài stdlib.

## Rủi ro và lưu ý

- Khoá UNIQUE của `artifact_relations` trên MySQL có thể vượt giới hạn độ dài khoá InnoDB (3072 byte) nếu `VARCHAR(160) x 2 + ...` với `utf8mb4`; tính toán và rút độ dài thay vì bỏ ràng buộc.
- `ALTER ... ADD COLUMN JSON NOT NULL` không default trên MySQL không chạy được với bảng có dòng; dùng `ADD COLUMN ... NULL`, backfill, rồi `MODIFY ... NOT NULL`.
- `requests.body` giữ nghĩa "phát biểu vấn đề"; không đổi tên cột (SOL-004 và gateway đã dùng).
- CR-REQ-028 cũng sửa CHECK của `requests.status`; hai migration cùng chạm bảng `requests`: thống nhất thứ tự `NNNN` (027 trước 028) để không đè.
