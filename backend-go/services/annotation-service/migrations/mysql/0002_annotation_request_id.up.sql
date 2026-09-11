-- Adds the idempotency key annotation-service was the only service missing
-- despite CreateAnnotationRequest already carrying request_id — mirrors
-- the Postgres variant + usage-service/automation-service's (tenant_id,
-- request_id) convention exactly.
--
-- MySQL variant: request_id is added straight as NOT NULL with no
-- DEFAULT. The Postgres variant backfills existing rows with
-- gen_random_uuid()::text before dropping the default, because it may run
-- against a database that already has annotation rows (an existing
-- Postgres deployment evolving forward). A MySQL-dialect deployment of
-- this service starts from migration 0001 onward, so 0002 always runs
-- against an empty `annotations` table — there is no pre-existing row to
-- backfill, so the DEFAULT-then-DROP dance the Postgres variant needs is
-- unnecessary here, not merely translated away.
ALTER TABLE annotations ADD COLUMN request_id VARCHAR(255) NOT NULL;

ALTER TABLE annotations ADD CONSTRAINT uq_annotations_tenant_request UNIQUE (tenant_id, request_id);
