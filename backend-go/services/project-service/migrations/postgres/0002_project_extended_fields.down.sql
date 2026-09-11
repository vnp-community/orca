ALTER TABLE project.projects
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS default_branch,
    DROP COLUMN IF EXISTS visibility,
    DROP COLUMN IF EXISTS created_by;
