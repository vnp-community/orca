-- Adds the version column UpdateTemplate's optimistic-concurrency check
-- (SOL-030, mirroring SOL-001's AccessPolicy versioning pattern) needs —
-- workflow.templates had no version column until this task added the
-- UpdateTemplate RPC (see internal/domain/template.go's WorkflowTemplate.Version
-- doc comment). DEFAULT 1 matches domain.NewWorkflowTemplate's constructor.
ALTER TABLE workflow.templates ADD COLUMN version INT NOT NULL DEFAULT 1;
