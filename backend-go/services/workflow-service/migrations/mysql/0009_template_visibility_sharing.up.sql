-- MySQL/TiDB variant of postgres/0009_template_visibility_sharing.up.sql.
ALTER TABLE templates
  ADD COLUMN visibility VARCHAR(16) NOT NULL DEFAULT 'private'
    CHECK (visibility IN ('private','team','company','public')),
  -- share_token: a plain UNIQUE index on a nullable MySQL column already
  -- allows any number of NULL rows (MySQL treats each NULL as distinct
  -- for uniqueness, same as a Postgres unique index over a nullable
  -- column) — no partial-index workaround needed for the Postgres
  -- variant's `WHERE share_token IS NOT NULL` semantics. VARCHAR(255) not
  -- TEXT: InnoDB rejects TEXT in a UNIQUE key without a prefix length.
  ADD COLUMN share_token VARCHAR(255) NULL UNIQUE,
  ADD COLUMN rating_sum   INT NOT NULL DEFAULT 0,
  ADD COLUMN rating_count INT NOT NULL DEFAULT 0;

CREATE INDEX idx_workflow_templates_visibility ON templates(tenant_id, visibility);
-- MySQL 8.0+ supports true descending indexes (unlike 5.7, which accepted
-- but silently ignored DESC) — this stays an index-only-scan-capable
-- match for ListTemplates(sort=trending)'s ORDER BY, same as Postgres.
CREATE INDEX idx_workflow_templates_trending ON templates(tenant_id, visibility, usage_count DESC, rating_sum DESC);
-- InnoDB FULLTEXT index — MySQL/TiDB's equivalent of Postgres's GIN
-- tsvector index, queried via MATCH(...) AGAINST(...) in
-- internal/adapter/mysql.Repository.ListTemplates. Word-boundary/stemming
-- rules differ from Postgres's 'english' text search config (InnoDB
-- fulltext has its own stopword list and a default 3-char minimum word
-- length as of MySQL 8), a real dialect difference in search recall, not
-- just a syntax translation.
CREATE FULLTEXT INDEX idx_workflow_templates_fts ON templates(name, description);

-- id has no server-side default (unlike Postgres's DEFAULT gen_random_uuid()
-- on ratings — wait, ratings has no id column in either dialect, see below);
-- template_id/user_id are always caller-supplied.
CREATE TABLE ratings (
  template_id CHAR(36) NOT NULL,
  user_id CHAR(36) NOT NULL,
  stars SMALLINT NOT NULL CHECK (stars BETWEEN 1 AND 5),
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE (template_id, user_id),
  CONSTRAINT fk_workflow_ratings_template FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE CASCADE
);

-- No RLS equivalent — ratings carries no tenant_id column of its own even
-- in the Postgres variant (its policy is an EXISTS join to templates); the
-- application-layer join in internal/adapter/mysql is the ONLY enforcement
-- here, same posture as every other table in this migration set.

-- approvals mirrors orchestration-service.md §5's decision_gates shape —
-- same table comment as the Postgres variant. id has no server-side
-- default: internal/usecase/publish_template.go always generates it in Go
-- (uuid.NewString()), same dead-default finding as templates.id.
--
-- fk_workflow_approvals_template deliberately has NO `ON DELETE CASCADE`
-- (unlike the Postgres variant) — confirmed empirically (not guessed):
-- MySQL/InnoDB (8.4.11, the `mysql:8` tag's current resolved version)
-- rejects ON DELETE CASCADE/SET NULL/ON UPDATE CASCADE/SET NULL on a
-- foreign key whose column is also a base column of a STORED generated
-- column elsewhere in the same table — error 1215 "Cannot add foreign key
-- constraint", reproduced directly against a real container, not
-- documented anywhere obvious in golang-migrate's or this rollout's prior
-- solutions. `pending_template_id` below reads template_id, which is
-- exactly this situation. Plain (default RESTRICT) is the only action
-- that coexists with the generated column. No functional loss in
-- practice: usecase.TemplateRepository has no Delete method at all (grep
-- confirmed, `internal/usecase/ports.go`) — the Postgres CASCADE has
-- never had a live delete path to fire from, on either dialect.
CREATE TABLE approvals (
  id CHAR(36) NOT NULL PRIMARY KEY,
  tenant_id CHAR(36) NOT NULL,
  template_id CHAR(36) NOT NULL,
  requested_by CHAR(36) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  resolved_by CHAR(36) NULL,
  resolved_at TIMESTAMP(6) NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_workflow_approvals_template FOREIGN KEY (template_id) REFERENCES templates(id)
);
CREATE INDEX idx_workflow_approvals_pending ON approvals(tenant_id, status);
-- Postgres's idx_workflow_approvals_one_pending_per_template is a PARTIAL
-- unique index (WHERE status = 'pending') — MySQL has no partial index, so
-- a plain UNIQUE(template_id) here would wrongly forbid a second
-- APPROVED/REJECTED row for the same template, which the Postgres variant
-- allows (history of past resolved approvals, only pending ones are
-- exclusive). A generated column that is only non-NULL while pending
-- reproduces the partial-uniqueness exactly: NULL values don't
-- participate in a MySQL UNIQUE index (same "many NULLs allowed" rule
-- used for share_token above), so the constraint only ever fires across
-- rows that are actually pending.
ALTER TABLE approvals
  ADD COLUMN pending_template_id CHAR(36)
    GENERATED ALWAYS AS (CASE WHEN status = 'pending' THEN template_id ELSE NULL END) STORED,
  ADD UNIQUE INDEX idx_workflow_approvals_one_pending_per_template (pending_template_id);

-- approvals carries its own tenant_id — no RLS equivalent, same posture.
