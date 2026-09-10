ALTER TABLE infra.fleet_health
  ADD COLUMN status TEXT NOT NULL DEFAULT 'unreachable'
    CHECK (status IN ('healthy','degraded','unhealthy','unreachable'));
