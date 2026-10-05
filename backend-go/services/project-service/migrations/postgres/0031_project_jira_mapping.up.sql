ALTER TABLE project.projects ADD COLUMN jira_project_key TEXT NOT NULL DEFAULT '', ADD COLUMN jira_site_id TEXT NOT NULL DEFAULT '';
ALTER TABLE project.worktrees ADD COLUMN linked_issue_site TEXT;
