-- MySQL/TiDB variant of postgres/0006_coordinator_run_heartbeat.up.sql.
ALTER TABLE coordinator_runs ADD COLUMN heartbeat_at TIMESTAMP(6) NULL;
