DROP TABLE request_coverage;
DROP TABLE artifact_relations;
DROP TABLE artifact_index;
DROP TABLE request_revisions;

ALTER TABLE solutions DROP INDEX solutions_request_seq;
ALTER TABLE solutions
    DROP COLUMN content_digest,
    DROP COLUMN input_request_revision,
    DROP COLUMN provenance,
    DROP COLUMN schema_version,
    DROP COLUMN seq;

ALTER TABLE requests
    DROP COLUMN content_digest,
    DROP COLUMN content_revision,
    DROP COLUMN type_fields,
    DROP COLUMN acceptance_criteria,
    DROP COLUMN content_schema_version;
