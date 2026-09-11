-- MySQL has no `ON CONFLICT` — `INSERT IGNORE` relies on the
-- (project_id, user_id) PRIMARY KEY to silently skip a row that already
-- has a membership row, same net effect as the Postgres source's
-- `ON CONFLICT (project_id, user_id) DO NOTHING`.
--
-- Deviation from the Postgres source (documented, not silent): added an
-- explicit `p.created_by IS NOT NULL` filter. The Postgres version has no
-- such guard, relying on ON CONFLICT to only catch unique-violations — a
-- project with a NULL created_by (nullable since 0002) would hit a NOT
-- NULL violation on user_id there too, just via a different failure mode.
-- MySQL's INSERT IGNORE, unlike Postgres, does NOT skip a NOT-NULL
-- violation cleanly in all modes — it can substitute the column's implicit
-- default instead of skipping the row, which would insert a corrupt
-- membership row rather than erroring. The explicit filter avoids this class
-- of row entirely, on both dialects' logical intent (a project with no
-- recorded creator has no membership row to backfill).
INSERT IGNORE INTO project_members (project_id, user_id, role, added_at)
SELECT p.id, p.created_by, 'owner', CURRENT_TIMESTAMP(6)
FROM projects p
WHERE p.created_by IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM project_members m
    WHERE m.project_id = p.id AND m.user_id = p.created_by
);
