-- ExecuteAdHocStep (§3.1) now persists a synthetic one-step execution with
-- no backing WorkflowTemplate (see README) — this needs template_id to
-- actually be optional. The original narrowed 0001_init made it NOT NULL
-- with ON DELETE CASCADE because nothing wrote a templateless execution
-- yet; the fuller design doc (workflow-service.md §5) always specified
-- `template_id UUID REFERENCES templates(id) ON DELETE SET NULL` (no NOT
-- NULL) — this migration brings the scaffold in line with that, once
-- something (ExecuteAdHocStep) actually needs it.
ALTER TABLE workflow.executions ALTER COLUMN template_id DROP NOT NULL;
ALTER TABLE workflow.executions DROP CONSTRAINT executions_template_id_fkey;
ALTER TABLE workflow.executions ADD CONSTRAINT executions_template_id_fkey
    FOREIGN KEY (template_id) REFERENCES workflow.templates(id) ON DELETE SET NULL;
