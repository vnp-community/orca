ALTER TABLE annotation.annotations DROP CONSTRAINT IF EXISTS uq_annotations_tenant_request;
ALTER TABLE annotation.annotations DROP COLUMN IF EXISTS request_id;
