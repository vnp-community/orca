-- Adds the description/default_branch/visibility/created_by fields to
-- project.projects that project-service.md §4/§5 always specified but
-- 0001_init.up.sql deferred (see that migration's header comment) —
-- UpdateProject/DeleteProject now exercise the fuller shape.
-- created_at/updated_at already exist from 0001_init.up.sql, so this
-- migration doesn't touch them.
ALTER TABLE project.projects
    ADD COLUMN description    TEXT NOT NULL DEFAULT '',
    ADD COLUMN default_branch TEXT NOT NULL DEFAULT 'main',
    ADD COLUMN visibility     TEXT NOT NULL DEFAULT 'private'
                                 CHECK (visibility IN ('private', 'team', 'department', 'company')),
    ADD COLUMN created_by     UUID;          -- logical FK -> tenant-service.users; nullable
                                              -- for rows that predate this column
