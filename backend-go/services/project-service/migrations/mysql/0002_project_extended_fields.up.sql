ALTER TABLE projects
    ADD COLUMN description    TEXT NOT NULL DEFAULT (''),
    ADD COLUMN default_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    ADD COLUMN visibility     VARCHAR(16) NOT NULL DEFAULT 'private'
                                 CHECK (visibility IN ('private', 'team', 'department', 'company')),
    ADD COLUMN created_by     CHAR(36);
