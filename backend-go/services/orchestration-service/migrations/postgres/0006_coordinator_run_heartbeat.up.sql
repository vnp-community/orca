-- BE-SOL-004's "no new migration" claim holds for 4 of its 5
-- CoordinatorRun RPCs, not RecordHeartbeat — that one needs a liveness
-- timestamp column that doesn't exist on coordinator_runs today (see
-- TASK-TG-004-01's own correction of that claim).
ALTER TABLE orchestration.coordinator_runs ADD COLUMN heartbeat_at TIMESTAMPTZ;
