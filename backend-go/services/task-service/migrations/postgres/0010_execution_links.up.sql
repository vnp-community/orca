-- BE-SOL-001 (CR-FLOW-TASK-001): task.execution_links gives every
-- ExecuteTask dispatch — across all three engines, including the
-- synchronous Engine 1 (direct-agent) path — a history row, so
-- CR-FLOW-TASK-003's Activity Feed (BE-SOL-003) has something to read.
--
-- Renumbered to 0010 at merge time: 0003/0004/0005 were already claimed by
-- independently-landed migrations by the time this branch merged into main
-- — see docs/backlog's migration-numbering notes on why a task's
-- self-assigned number can't be trusted once multiple branches diverge.
--
-- active_execution_link_id is deliberately a distinct column/name from
-- TASK-TG-04-05's proposed (still unimplemented, per this migration's
-- writing) task.tasks.active_execution_id — see that task's own note on
-- coordinating if both land.
CREATE TABLE task.execution_links (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    task_id           UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    engine            TEXT NOT NULL CHECK (engine IN ('direct_agent','orchestration','workflow')),
    external_ref_id   TEXT NOT NULL DEFAULT '', -- coordinator_run_id (Engine 2) / workflow execution_id (Engine 3, BE-SOL-002); empty for Engine 1
    status_mirror     TEXT NOT NULL DEFAULT 'in_progress', -- updated by BE-SOL-003's consumer for Engine 2/3, written directly here for Engine 1
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_execution_links_task ON task.execution_links (task_id, started_at DESC);

ALTER TABLE task.execution_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.execution_links
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- workflow_template_id is NOT added here — 0009_task_workflow_template_id.up.sql
-- already added it (BACKLOG-016/CR-FLOW-TASK-002), landed independently of
-- this migration; adding it twice would fail migrate at deploy time.
ALTER TABLE task.tasks
  ADD COLUMN active_execution_link_id UUID REFERENCES task.execution_links(id);
