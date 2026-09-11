-- directories: Postgres TEXT[] has no MySQL equivalent — translated to a
-- JSON array column. internal/adapter/mysql.SparsePresetRepository
-- marshals/unmarshals []string <-> JSON on every read/write; the domain
-- type (domain.SparsePreset.Directories []string) is unchanged.
CREATE TABLE sparse_presets (
    id          CHAR(36) PRIMARY KEY,
    repo_id     CHAR(36) NOT NULL,
    name        TEXT NOT NULL,
    directories JSON NOT NULL,
    created_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT fk_sparse_presets_repo FOREIGN KEY (repo_id) REFERENCES repos (id) ON DELETE CASCADE
);
CREATE INDEX idx_sparse_presets_repo ON sparse_presets (repo_id);

-- RLS dropped — see 0001_init.up.sql's comment.
