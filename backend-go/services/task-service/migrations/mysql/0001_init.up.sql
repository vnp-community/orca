-- MySQL/TiDB translation of migrations/postgres/0001_init.up.sql — see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-015-task-service-mysql-tidb-adapter.md
-- for the full dialect-mapping rationale (this file only carries schema, not
-- the reasoning behind each choice).
--
-- No CREATE SCHEMA/DATABASE statement — MySQL has no schema/database split
-- (they're synonyms); DATABASE_DSN's own database name IS the isolation
-- unit, mirroring usage-service/annotation-service/issue-tracking-service's
-- established convention (BE-DB-SOL-002 §3). Tables carry NO "task_"
-- prefix, since the database itself already provides that scoping.
--
-- Explicit CONSTRAINT names on every CHECK (MySQL auto-generates one like
-- `tasks_chk_1` otherwise, which migrations/mysql/0003 would then be unable
-- to DROP by a known name the way migrations/postgres/0003 drops
-- `tasks_status_check`).
CREATE TABLE tasks (
    id          CHAR(36) NOT NULL PRIMARY KEY DEFAULT (UUID()),
    tenant_id   CHAR(36) NOT NULL,
    title       TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    -- parent_id is denormalized directly onto the task row so
    -- GetAncestors (usecase.TaskRepository) can walk the hierarchy with one
    -- WITH RECURSIVE query — MySQL 8.0.1+ supports recursive CTEs, so this
    -- translates directly (see migrations/mysql's subtree/ancestor queries
    -- in internal/adapter/mysql).
    parent_id   CHAR(36) NULL,
    created_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    -- updated_at is set explicitly by application SQL on every write
    -- (`updated_at = now()` throughout internal/adapter/postgres), never by
    -- a DB-side trigger — deliberately NOT given `ON UPDATE
    -- CURRENT_TIMESTAMP` here, which would double-write it via a different
    -- mechanism than the Postgres adapter uses and risk skew between the
    -- two dialects' semantics.
    updated_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT tasks_status_check CHECK (status IN ('open', 'in_progress', 'done', 'cancelled')),
    CONSTRAINT fk_tasks_parent FOREIGN KEY (parent_id) REFERENCES tasks(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX idx_tasks_tenant ON tasks (tenant_id);
CREATE INDEX idx_tasks_parent ON tasks (parent_id);

-- No RLS equivalent in MySQL/TiDB (dbcapability.Capabilities.SupportsRLS is
-- false) — every query in internal/adapter/mysql filters explicitly by
-- tenant_id, which is already the ONLY tenant-isolation enforcement that is
-- real today even on the Postgres side (see BE-DB-SOL-001 §4: no code in
-- backend-go ever calls SET LOCAL app.tenant_id, so Postgres's RLS policy
-- has never actually activated). Dropping RLS here does not lower the real
-- protection level.

-- edge_type is VARCHAR(20), not TEXT, because it participates in the
-- table's UNIQUE key below — InnoDB rejects TEXT/BLOB in a key without an
-- explicit prefix length, and the CHECK already bounds it to 2 short
-- values, so a short VARCHAR is both correct and simpler than a prefix
-- index.
CREATE TABLE task_edges (
    id           CHAR(36) NOT NULL PRIMARY KEY DEFAULT (UUID()),
    tenant_id    CHAR(36) NOT NULL,
    from_task_id CHAR(36) NOT NULL,
    to_task_id   CHAR(36) NOT NULL,
    edge_type    VARCHAR(20) NOT NULL,
    created_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    -- single_parent_key emulates Postgres's partial unique index
    -- (`task_edges_single_parent ... WHERE edge_type = 'parent_child'`,
    -- enforcing "one parent_child edge per child") — MySQL has no partial
    -- index, but a generated column that is NULL for every non-parent_child
    -- row gets the same effect: MySQL's UNIQUE index treats every NULL as
    -- distinct from every other NULL, so only parent_child rows ever
    -- collide on to_task_id, exactly matching the Postgres invariant this
    -- service's domain.DetectCycle/AddEdge usecase relies on.
    single_parent_key CHAR(36) GENERATED ALWAYS AS (CASE WHEN edge_type = 'parent_child' THEN to_task_id ELSE NULL END) STORED,
    CONSTRAINT task_edges_edge_type_check CHECK (edge_type IN ('parent_child', 'depends_on')),
    CONSTRAINT task_edges_unique UNIQUE (from_task_id, to_task_id, edge_type),
    CONSTRAINT task_edges_single_parent UNIQUE (single_parent_key),
    CONSTRAINT fk_task_edges_from FOREIGN KEY (from_task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    -- fk_task_edges_to deliberately has NO "ON DELETE CASCADE" — confirmed
    -- empirically (not guessed): InnoDB rejects it with error 1215 "Cannot
    -- add foreign key constraint" whenever the FK's child column is also a
    -- base column of a STORED generated column on the same table (here,
    -- single_parent_key is generated from to_task_id) — MySQL disallows a
    -- CASCADE/SET NULL action in that combination since it can't guarantee
    -- the generated column stays consistent through a cascaded write. The
    -- trigger below restores equivalent cascade-delete behavior for the
    -- to_task_id side without hitting that restriction.
    CONSTRAINT fk_task_edges_to FOREIGN KEY (to_task_id) REFERENCES tasks(id)
) ENGINE=InnoDB;

CREATE INDEX task_edges_from_idx ON task_edges (tenant_id, from_task_id, edge_type);
CREATE INDEX task_edges_to_idx   ON task_edges (tenant_id, to_task_id, edge_type);

-- Restores "deleting a task removes every task_edges row referencing it as
-- to_task_id" (fk_task_edges_from's real ON DELETE CASCADE above already
-- covers the from_task_id side) — see that FK's comment for why a plain
-- CASCADE action isn't available here. Fires BEFORE DELETE so the child
-- rows are gone before InnoDB's own fk_task_edges_from cascade (if this
-- task is also a from_task_id elsewhere) and this trigger run in the same
-- statement; verified empirically against a real MySQL 8 container that
-- this produces the same end state internal/adapter/postgres's native
-- Postgres CASCADE does.
CREATE TRIGGER trg_task_edges_cascade_to
BEFORE DELETE ON tasks
FOR EACH ROW
DELETE FROM task_edges WHERE to_task_id = OLD.id;

-- level folds grantee-kind into the permission tier itself — see
-- internal/domain/grant.go's doc comment. VARCHAR(10) covers the longest
-- value ("company").
CREATE TABLE task_grants (
    id         CHAR(36) NOT NULL PRIMARY KEY DEFAULT (UUID()),
    tenant_id  CHAR(36) NOT NULL,
    task_id    CHAR(36) NOT NULL,
    subject_id CHAR(36) NOT NULL,
    level      VARCHAR(10) NOT NULL,
    apply_tree BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT task_grants_level_check CHECK (level IN ('owner', 'admin', 'user', 'team', 'company')),
    CONSTRAINT fk_task_grants_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX task_grants_task_idx ON task_grants (tenant_id, task_id);

CREATE TABLE task_comments (
    id         CHAR(36) NOT NULL PRIMARY KEY DEFAULT (UUID()),
    tenant_id  CHAR(36) NOT NULL,
    task_id    CHAR(36) NOT NULL,
    author_id  CHAR(36) NOT NULL, -- logical FK -> tenant-service
    content    TEXT NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_task_comments_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
) ENGINE=InnoDB;

CREATE INDEX task_comments_task_idx ON task_comments (tenant_id, task_id);
