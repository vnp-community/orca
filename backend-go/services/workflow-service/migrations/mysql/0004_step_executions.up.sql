-- MySQL/TiDB variant of postgres/0004_step_executions.up.sql.
--
-- dispatch_token has no server-side default (unlike Postgres's
-- `DEFAULT gen_random_uuid()`): both call sites
-- (internal/usecase/wave_dispatcher.go, execute_ad_hoc_step.go) always
-- generate it in Go (uuid.NewString()) before calling NewStepExecution,
-- confirmed by reading both call sites — same dead-default finding as
-- id's comment in 0001_init.up.sql.
CREATE TABLE step_executions (
    id             CHAR(36) NOT NULL PRIMARY KEY,
    execution_id   CHAR(36) NOT NULL,
    step_id        VARCHAR(255) NOT NULL, -- bounded (not TEXT) so it can share a composite UNIQUE key below
    wave           INT NOT NULL,
    status         VARCHAR(16) NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    dispatch_token CHAR(36) NOT NULL,
    output         JSON NULL,
    error_message  TEXT,
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_workflow_step_executions_execution FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE,
    UNIQUE (execution_id, step_id)
);

CREATE INDEX idx_workflow_step_executions_execution ON step_executions (execution_id, wave);

-- No RLS equivalent — step_executions carries no tenant_id column of its
-- own even in the Postgres variant (its policy is an EXISTS join to
-- executions); the application-layer join in
-- internal/adapter/mysql/repository.go's ListStepExecutions is the ONLY
-- enforcement here, same posture as every other table in this migration
-- set.
