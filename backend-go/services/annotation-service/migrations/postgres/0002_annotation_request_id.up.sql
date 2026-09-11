-- Adds the idempotency key annotation-service was the only service missing
-- despite CreateAnnotationRequest already carrying request_id — see
-- docs/execution-plan.md §3 Phase 1 and usage-service/automation-service's
-- (tenant_id, request_id) convention this mirrors exactly.
--
-- Backfilled with gen_random_uuid()::text (never colliding) rather than ''
-- so the NOT NULL + UNIQUE constraint below can apply to any pre-existing
-- rows too, not just rows created after this migration.
ALTER TABLE annotation.annotations ADD COLUMN request_id TEXT NOT NULL DEFAULT gen_random_uuid()::text;
ALTER TABLE annotation.annotations ALTER COLUMN request_id DROP DEFAULT;

ALTER TABLE annotation.annotations ADD CONSTRAINT uq_annotations_tenant_request UNIQUE (tenant_id, request_id);
