-- TASK-TG-001-02: widens task.tasks with the remaining BE-SOL-001 fields
-- that domain.Task already carries but the schema was still missing —
-- labels, reporter_id, workflow-execution tracking, subtree progress
-- counters, and the public share-link token (TASK-TG-003-05, SECURITY
-- REVIEW REQUIRED before merge — see domain.TaskShareView's doc comment).
--
-- Renumbered to 0011 at merge time: this migration's self-assigned number
-- (0004) collided with two migrations already claimed by main
-- (0004_grant_expiry_and_public_link, plus 0003_task_fields_and_comments
-- already having added description/task_type/priority/assignee_id/owner_id/
-- due_date/estimated_hours/actual_hours/prompt_template/ai_context/
-- ai_plan_json/visibility/worktree_id/agent_session_id/progress_percent —
-- see that migration's own columns) — see 0010_execution_links.up.sql's
-- identical renumbering note. Only the genuinely missing columns are added
-- here; task.task_grants.expires_at is likewise already
-- present via 0004_grant_expiry_and_public_link.
ALTER TABLE task.tasks
  ADD COLUMN labels           TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN reporter_id      UUID,          -- logical FK -> tenant-service, same convention as assignee_id/owner_id
  ADD COLUMN workflow_exec_id TEXT NOT NULL DEFAULT '', -- execution-tracking counterpart to workflow_template_id; same shape as execution_links.external_ref_id
  ADD COLUMN done_subtasks    INT NOT NULL DEFAULT 0,
  ADD COLUMN total_subtasks   INT NOT NULL DEFAULT 0;

-- share_token: unset until GenerateShareLink mints one. UNIQUE is
-- load-bearing — two tasks sharing a token would let GetTaskByShareToken
-- leak the wrong task.
ALTER TABLE task.tasks ADD COLUMN share_token TEXT UNIQUE;
CREATE INDEX idx_tasks_share_token ON task.tasks (share_token) WHERE share_token IS NOT NULL;
