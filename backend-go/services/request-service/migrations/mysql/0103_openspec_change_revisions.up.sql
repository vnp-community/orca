-- One row per OpenSpec change a Plan run produced (CR-REQ-045), so the versions of a Plan stay listable after a revision
-- rewrites openspec_changes. The index (paths, sizes, hashes) is kept for every version; the full text only for the
-- newest ones, and bundle_json is NULL once a version falls out of that window.
CREATE TABLE openspec_change_revisions (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    request_id VARCHAR(36) NOT NULL,
    revision INT NOT NULL,
    run_id VARCHAR(36) NOT NULL,
    commit_sha VARCHAR(64) NOT NULL DEFAULT '',
    tasks_md_digest VARCHAR(64) NOT NULL DEFAULT '',
    bundle_digest VARCHAR(64) NOT NULL,
    files_json JSON NOT NULL,
    bundle_json JSON NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_openspec_change_revisions_request FOREIGN KEY (request_id) REFERENCES requests(id),
    CONSTRAINT openspec_change_revisions_revision CHECK (revision > 0),
    UNIQUE KEY openspec_change_revisions_number (tenant_id, request_id, revision),
    UNIQUE KEY openspec_change_revisions_run (tenant_id, run_id),
    INDEX openspec_change_revisions_by_request (tenant_id, request_id, revision)
);
