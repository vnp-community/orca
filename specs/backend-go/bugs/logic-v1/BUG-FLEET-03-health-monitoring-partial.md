# BUG-FLEET-03: Fleet health read path exists but nothing ever writes health samples — `fleet.health.checkAll` always returns empty

**Business Logic:** [BL-FLEET-03](../../../../docs/logic/fleet/BL-FLEET-03-health-monitoring.md) — Fleet Health Monitoring
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** An Admin calling `fleet.health.checkAll` gets a `200`-shaped response with an empty/stale statuses list for every dev server, forever — because the table it reads from (`infra.fleet_health`) has no writer anywhere in backend-go. There is also no `/health/metrics` Prometheus endpoint and no webhook alerting on status change, so none of the spec's three observability surfaces (poller, metrics, alerts) function end-to-end.

---

## Spec summary

BL-FLEET-03 describes `FleetHealthMonitor`: a 60s-interval poller (`FLEET_POLL_INTERVAL_SEC`) that, per server, SSH-connects, HTTP-health-checks the relay through an SSH tunnel, collects CPU/RAM/disk/latency metrics, computes a `healthy`/`degraded`/`unhealthy`/`unreachable` status from thresholds (CPU<80%/RAM<85% etc.), diffs it against the previous status to emit a `status_change` event, writes the sample to an in-memory store, and updates the server record. It also exposes Prometheus metrics at `GET /health/metrics` and POSTs a webhook (`fleet.server.status_change`) on status transitions.

## What backend-go has

- A real read path: `GetFleetHealth` RPC (`backend-go/proto/orca/infrafleet/v1/infrafleet.proto:19,157-167`) → usecase (`backend-go/services/infra-fleet-service/internal/usecase/get_fleet_health.go:15-34`) → Postgres query joining `infra.fleet_health` to `infra.dev_servers` by tenant (`backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go:329-356`).
- `domain.DevServerHealth` models `{DevServerID, Reachable, CPUPercent, RAMPercent, DiskPercent, LatencyMS}` with input validation (0-100 range, non-negative latency) (`backend-go/services/infra-fleet-service/internal/domain/dev_server_health.go:20-53`).
- `fleet.health.checkAll` is wired in the wscompat WS layer, calling `GetFleetHealth` with an 8s per-RPC deadline and mapping into a `serverHealthView` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:487-519`).
- The `infra.fleet_health` table exists with the right columns and a `checked_at DESC` index (`backend-go/services/infra-fleet-service/migrations/0001_init.up.sql:59-69`).

## What's missing

- **No writer to `infra.fleet_health` anywhere in the codebase.** Both the usecase doc comment ("The 30s-cadence poller... is not implemented in this scaffold", `get_fleet_health.go:11-14`), the port doc comment (`ports.go:102-105`), and the migration's own comment ("A periodic 30s-cadence poller... is the intended writer of this table; it is not implemented in this scaffold", `0001_init.up.sql:56-58`) confirm this. A `grep` for `RecordHealth`/`UpsertHealth`/`InsertHealth`/`fleet_health` across `internal/adapter/postgres/*.go` finds only the `SELECT` in `GetFleetHealth` — no `INSERT`/`UPSERT` statement exists. In a real deployment this table is permanently empty and every `fleet.health.checkAll` call returns `[]`.
- No 60s/30s-interval poller at all — `infra-fleet-service/cmd/server/main.go` has no ticker/cron (`grep` for `time.Ticker`/`time.NewTicker`/`cron` returns nothing).
- No SSH-tunnel-based relay HTTP health check, no CPU/RAM/disk collection commands (`/proc/stat`, `free -b`, `df -P`) anywhere in `sshrelay`/`devserveragent`.
- No computed status string (`healthy`/`degraded`/`unhealthy`/`unreachable`) or threshold logic (CPU<80%, RAM<85%) — `DevServerHealth` only carries raw numeric fields and a `Reachable` bool, no derived status enum.
- No `status_change` event emission, no in-memory store, no `last_checked_at` update on the dev server record.
- No `GET /health/metrics` Prometheus endpoint — `grep` for `prometheus`, `orca_fleet_`, `health/metrics` across all of `backend-go/` returns zero hits. `infra-fleet-service`'s health/readiness HTTP server (per its README) is a liveness probe, not a metrics endpoint.
- No webhook alert delivery (`FLEET_WEBHOOK_URL`, per-server webhook config, or a POST-on-status-change call) anywhere in `backend-go/`.

## See also

None of the existing `missing-v1`/`api-v1` bug files cover fleet health specifically; `fleet.health.checkAll`'s wiring itself is not the gap (it is real) — the gap is entirely upstream, in the absent write side.

## References

- `docs/logic/fleet/BL-FLEET-03-health-monitoring.md`
- `backend-go/services/infra-fleet-service/internal/usecase/get_fleet_health.go:11-34` — read usecase, doc comment confirms poller absent
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:102-108` — `FleetHealthPort`, read-only, doc comment confirms writer absent
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go:329-356` — `GetFleetHealth` (only consumer of `infra.fleet_health`)
- `backend-go/services/infra-fleet-service/migrations/0001_init.up.sql:56-69` — `infra.fleet_health` DDL, comment confirms no writer
- `backend-go/services/infra-fleet-service/internal/domain/dev_server_health.go:20-53` — `DevServerHealth`, no derived status field
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:487-519` — `fleet.health.checkAll`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:19,157-167` — `GetFleetHealth` RPC/messages
