# TASK-REQ-030-01: Migration `impact_risk` hai dialect (`impact_assessments`, `impact_tool_runs`, `risk_acceptances`, `risk_policies`, `risk_outcomes`)

**From Solution:** [BE-REQ-SOL-030](../solutions/BE-REQ-SOL-030-impact-assessment-and-risk-scoring.md) mục 2.B
**Priority:** P1
**Service/Area:** `request-service` (mới) / migration
**File:** `backend-go/services/request-service/migrations/postgres/NNNN_impact_risk.{up,down}.sql` (mới), `backend-go/services/request-service/migrations/mysql/NNNN_impact_risk.{up,down}.sql` (mới), `backend-go/services/request-service/internal/adapter/postgres/migration_impact_risk_test.go` (mới), `internal/adapter/mysql/migration_impact_risk_test.go` (mới)
**Depends on:** TASK-REQ-002-01 (migration `0002_request_core`, bảng `requests`), TASK-REQ-001-04 (RLS mẫu và `TxRunner`)
**Status:** [ ] TODO

---

## Context

- `request-service` chưa có thư mục (ngày 2026-10-06). Số migration **chồng nhau** giữa các CR 001/002/004/006 và các CR sau (nhiều CR cùng dùng `0002`, `0005`). `NNNN` ở đây là số lớn nhất hiện có cộng một **lúc tạo file**; bắt buộc `ls migrations/postgres migrations/mysql` trước; ghi số và danh sách vào mô tả PR. Thứ tự: sau migration `requests` (FK nội bộ), không phụ thuộc migration của `approvals` (không FK).
- Quy ước chung: mọi bảng có `tenant_id` (README v6 mục 8 điều 3); schema Postgres `request`; RLS thật theo mẫu `mcp-service` (`ENABLE` + `FORCE ROW LEVEL SECURITY`, policy dùng `NULLIF(current_setting('app.tenant_id', true), '')::uuid`, `set_config` mỗi giao dịch) như SOL-001 mục 2.D; MySQL lọc `tenant_id` ở mọi `WHERE`. JSON: Postgres `JSONB`, MySQL `JSON` không DEFAULT literal. `CHECK` MySQL cần 8.0.16 trở lên; `FOR UPDATE SKIP LOCKED` cần 8.0.1 trở lên.
- Thủ thuật "một hàng đang hoạt động": `task_sources` ở `task-service` dùng chỉ mục duy nhất với `COALESCE` (Postgres, `0012_task_sources.up.sql`) và cột sinh `project_key` + `UNIQUE` (MySQL, `0012` bản mysql). Dùng cùng cách: Postgres partial unique index, MySQL generated column nullable (NULL không va chạm UNIQUE).
- Mẫu lease: `analysis_runs` (SOL-007 migration, `lease_owner TEXT`, `lease_expires_at TIMESTAMPTZ`, chỉ mục `analysis_runs_lease_scan (status, lease_expires_at)`).
- Cột chi tiết theo CR-REQ-030 mục 2.1; các điểm cần chốt: `score SMALLINT` có `CHECK (score BETWEEN 0 AND 100)`; `level` nullable; `digest CHAR(64)`; `narrative` NULL; `impact_tool_runs.output` cắt 256 KB ở ứng dụng.

## Việc cần làm

1. `ls` và chốt `NNNN`; đọc migration `requests` để biết kiểu `id` (UUID/CHAR(36)) và tên bảng thật.
2. Bảng `request.impact_assessments` (Postgres):
   ```sql
   CREATE TABLE request.impact_assessments (
     id UUID PRIMARY KEY, tenant_id UUID NOT NULL,
     request_id UUID NOT NULL REFERENCES request.requests(id),
     subject_type TEXT NOT NULL CHECK (subject_type IN ('solution_option','plan','phase','task','actual_task','actual_phase')),
     subject_id TEXT NOT NULL, subject_digest CHAR(64) NOT NULL, revision INT NOT NULL,
     rules_version TEXT NOT NULL, policy_id UUID NULL,
     mode TEXT NOT NULL CHECK (mode IN ('shadow','enforce')),
     status TEXT NOT NULL CHECK (status IN ('collecting','ready','partial','failed','superseded')),
     level TEXT NULL CHECK (level IN ('low','medium','high','critical')),
     score SMALLINT NULL CHECK (score BETWEEN 0 AND 100),
     dimensions JSONB NOT NULL DEFAULT '[]'::jsonb, triggers JSONB NOT NULL DEFAULT '[]'::jsonb, findings JSONB NOT NULL DEFAULT '[]'::jsonb,
     confidence TEXT NULL CHECK (confidence IN ('high','medium','low')),
     digest CHAR(64) NULL, narrative JSONB NULL,
     lease_owner TEXT NULL, lease_expires_at TIMESTAMPTZ NULL,
     created_by TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), finished_at TIMESTAMPTZ NULL,
     UNIQUE (tenant_id, subject_type, subject_id, revision));
   CREATE UNIQUE INDEX impact_assessments_one_collecting ON request.impact_assessments (tenant_id, subject_type, subject_id) WHERE status = 'collecting';
   CREATE INDEX impact_assessments_request ON request.impact_assessments (tenant_id, request_id, created_at DESC);
   CREATE INDEX impact_assessments_lease_scan ON request.impact_assessments (status, lease_expires_at) WHERE status = 'collecting';
   ```
   MySQL: `CHAR(36)`, `VARCHAR(20)` + `CONSTRAINT ... CHECK`, `JSON NOT NULL` (ứng dụng ghi `[]`), `subject_id VARCHAR(80)`, `active_key VARCHAR(120) GENERATED ALWAYS AS (IF(status = 'collecting', CONCAT(subject_type, ':', subject_id), NULL)) STORED`, `UNIQUE KEY impact_assessments_one_collecting (tenant_id, active_key)`; chỉ mục lease `(status, lease_expires_at)`.
3. Bảng `impact_tool_runs`: `id`, `tenant_id`, `assessment_id` (FK `ON DELETE CASCADE`), `tool` CHECK `gitnexus_impact|gitnexus_detect_changes|gitnexus_status|codegraph|buf_breaking|migration_scan|go_cover|contract_scan|policy_scan`, `tool_version TEXT`, `command_digest CHAR(64)`, `exit_code INT NULL`, `duration_ms INT`, `index_commit TEXT NULL`, `index_age_seconds INT NULL`, `status` CHECK `ok|timeout|error|skipped`, `output` (Postgres `TEXT`, MySQL `MEDIUMTEXT`), `output_digest CHAR(64)`, `created_at`; chỉ mục `(tenant_id, assessment_id)` và `(tenant_id, command_digest, created_at DESC)` (bộ nhớ đệm theo lệnh).
4. Bảng `risk_acceptances`: `id`, `tenant_id`, `request_id` (FK nội bộ), `assessment_id` (FK), `assessment_digest CHAR(64)`, `finding_id TEXT`, `level` CHECK, `accepted_by TEXT`, `rationale TEXT`, `approval_id UUID NULL` (không FK), `created_at`; `UNIQUE (tenant_id, assessment_id, finding_id, accepted_by)`.
5. Bảng `risk_policies`: `id`, `tenant_id`, `version INT`, `status` CHECK `draft|shadow|active|retired`, `thresholds`, `weights`, `hard_rules`, `gate_mapping` (JSON, NOT NULL), `created_by`, `created_at`, `activated_at NULL`:
   - `UNIQUE (tenant_id, version)`
   - một `active` và một `shadow` mỗi tenant: Postgres hai partial unique index `(tenant_id) WHERE status='active'` và `WHERE status='shadow'`
   - MySQL hai cột sinh `active_key`, `shadow_key` (NULL khi không khớp) + `UNIQUE (tenant_id, active_key)`, `UNIQUE (tenant_id, shadow_key)`.
6. Bảng `risk_outcomes`: PK `(tenant_id, request_id)`, `predicted_level`, `actual_level NULL`, `incident BOOLEAN NOT NULL DEFAULT FALSE`, `rolled_back BOOLEAN NOT NULL DEFAULT FALSE`, `override_count INT NOT NULL DEFAULT 0`, `recorded_at TIMESTAMPTZ NOT NULL`.
7. RLS Postgres cho cả năm bảng (như SOL-001 mục 2.D); thêm policy cho vai trò relay chỉ khi cần đọc xuyên tenant (không cần ở bảng này).
8. `.down.sql`: `DROP TABLE` theo thứ tự ngược (`impact_tool_runs`, `risk_acceptances`, `impact_assessments`, `risk_policies`, `risk_outcomes`).
9. Test migration (mẫu `migration_0018_fleet_definitions_test.go` ở `infra-fleet-service`): áp dụng `up`, `down`, `up` trên container thật.

## Kiểm thử

- `TestMigrationNNNN_UpDownUp` hai dialect.
- `TestImpactAssessments_CheckRejectsUnknownValues` (`subject_type`, `status`, `mode`, `level`, `confidence`, `score=101`).
- `TestImpactAssessments_OnlyOneCollectingPerSubject` (chèn hai `collecting` cùng `(tenant, subject_type, subject_id)`: một lỗi UNIQUE; sau khi một bản `ready` thì chèn `collecting` mới được; `revision` trùng thì lỗi).
- `TestRiskAcceptances_UniquePerFindingAndUser`.
- `TestRiskPolicies_OneActiveOneShadowPerTenant` (hai `active` cùng tenant lỗi; hai tenant khác nhau không va chạm; `retired` nhiều bản được).
- `TestImpactToolRuns_CascadeOnAssessmentDelete`.
- Postgres: `TestRLS_ImpactTables_TenantIsolation` với role không phải superuser.
- Lệnh: `cd /opt/repos/orca/backend-go && go test -tags=integration ./services/request-service/internal/adapter/postgres/... ./services/request-service/internal/adapter/mysql/... -run "Migration|Impact|RiskPolic|RiskAcceptance"`.

## Tiêu chí hoàn thành

- [ ] `up`, `down`, `up` sạch trên Postgres 14+ và MySQL 8.0.16+.
- [ ] CHECK từ chối giá trị lạ ở cả hai dialect; một `collecting` mỗi chủ thể; một `active` và một `shadow` mỗi tenant.
- [ ] Mọi bảng có `tenant_id`; không FK sang service khác; `NNNN` ghi trong PR.
- [ ] Không dùng DEFAULT literal cho cột JSON MySQL.

## Rủi ro và lưu ý

- Cột sinh MySQL `STORED` với `IF(...)`: một số bản MySQL cũ không cho dùng hàm không xác định trong cột sinh; `IF` và `CONCAT` xác định nên được, nhưng kiểm trên đúng phiên bản CI (chưa kiểm chứng).
- Chồng số migration: hai PR cùng `NNNN`; điều phối viên chốt thứ tự trước khi merge.
- `output` 256 KB × nhiều lần chạy làm phình `impact_tool_runs`: đề xuất dọn theo Request `completed` quá N ngày (CR-REQ-035), không làm ở task này.
