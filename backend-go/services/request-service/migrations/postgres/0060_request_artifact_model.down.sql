DROP TABLE request.request_coverage;
DROP TABLE request.artifact_relations;
DROP TABLE request.artifact_index;
DROP TABLE request.request_revisions;

ALTER TABLE request.solutions DROP CONSTRAINT solutions_request_seq;
ALTER TABLE request.solutions
    DROP COLUMN content_digest,
    DROP COLUMN input_request_revision,
    DROP COLUMN provenance,
    DROP COLUMN schema_version,
    DROP COLUMN seq;

ALTER TABLE request.requests
    DROP COLUMN content_digest,
    DROP COLUMN content_revision,
    DROP COLUMN type_fields,
    DROP COLUMN acceptance_criteria,
    DROP COLUMN content_schema_version;
