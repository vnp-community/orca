# BUG-014: `issuetracking` Postgres database was never created/migrated — every issue-tracking-service DB call fails, live on `b15.openledger.vn`

**Service:** deploy config (`deploy/dev/scripts/migrate.sh`)
**Severity:** High — blocks `issue-tracking-service` entirely at the persistence layer, independent of whether Jira/Linear auth itself succeeds
**Symptom:**
```
rpc error: code = Internal desc = ISSUETRACKING_GET_STATUS_FAILED: failed to read connection status
```
underlying cause (visible only after [SOL-012](./solutions/SOL-012-git-gateway-service-cause-logging.md)'s pattern was extended to `issue-tracking-service` in this same pass):
```
postgres: query connection status: failed to connect to `user=orca database=issuetracking`:
  192.168.32.3:5432 (postgres): server error: FATAL: database "issuetracking" does not exist (SQLSTATE 3D000)
```

**Status:** ✅ **Fixed & migration run successfully (2026-09-15)**. `issuetracking` database created, `issuetracking.connections`/`issuetracking.outbox_events` tables now exist, `schema_migrations` at version 2, clean.

---

## Root Cause — CONFIRMED

`deploy/dev/docker-compose.yml` correctly defines a `migrate-issuetracking` one-shot service (`DATABASE_DSN: postgresql://orca:...@postgres:5432/issuetracking?sslmode=disable`) — the migration tooling for this database exists and is correctly wired.

`deploy/dev/scripts/migrate.sh` (the script every deploy actually calls — `sync-to-server.sh` invokes `migrate.sh --remote` with no target, meaning "run every service in `SERVICES`") had:
```bash
SERVICES="auth tenant project infra aiprovider workflow task orchestration automation annotation notification usage credential scm"
```
**`issuetracking` was missing from this list** — a plain omission, most likely from whenever `issue-tracking-service` was added (its compose migration service exists, but whoever wired it forgot this one script's hardcoded list; the script's own header comment said "14 services," which was correct for the old 14-service list, one more evidence this was never updated when the 15th was added). Every deploy this session (and presumably every deploy before it) ran migrations for 14 services, never the 15th — so the `issuetracking` database was **never created on this Postgres instance**, not even once.

Confirmed live: `docker exec orca-go-postgres psql -U orca -l` (run earlier in this session, before this bug was found) lists exactly the 14 migrated databases (`aiprovider`, `annotation`, `auth`, `automation`, `credential`, `infra`, `notification`, `orchestration`, `postgres`, `project`, `scm`, `task`, `tenant`, `usage`) — `issuetracking` is absent.

## Impact

- **Every** `issue-tracking-service` RPC that touches Postgres fails — confirmed for `GetConnectionStatus` (repeated `ISSUETRACKING_GET_STATUS_FAILED`) and would equally block `Connect`'s own `connections.Upsert` step (storing a newly-verified Jira/Linear connection) even when the external API auth succeeds.
- This is **independent of** [BUG-013](./BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md) (the Cloud-vs-self-hosted API version bug) — even with a perfectly valid credential and a perfectly correct API call, `Connect` would still fail at the persistence step because the database to persist into doesn't exist.

## Fix

`deploy/dev/scripts/migrate.sh`: added `issuetracking` to `SERVICES` (between `credential` and `scm`, matching `docker-compose.yml`'s own service ordering), updated the header comment's "14 services" → "15 services".

## Deploy status

**✅ Done.** `golang-migrate` doesn't create the database itself (only runs migrations inside one that already exists) — first ran `CREATE DATABASE issuetracking;` directly, then `./deploy/dev/scripts/migrate.sh --remote issuetracking`, which applied both migrations cleanly (`1/u outbox`, `2/u connections`). Confirmed via `\dt issuetracking.*`: `connections` and `outbox_events` tables exist, `schema_migrations` at version 2, not dirty.

## Related

- [BUG-013](./BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md) — the other half of "why can't I connect Jira," found and fixed in the same investigation; both needed fixing for Connect to actually succeed end to end.
- Same "wire it in `docker-compose.yml`, forget the sibling script" shape as this session's other cross-cutting gaps (`apperrors.SetLogger` not wired per-service — SOL-009/SOL-012) — worth noting for whoever next adds a DB-owning service: `docker-compose.yml`'s `migrate-*` definition alone is not sufficient, `migrate.sh`'s `SERVICES` list must be updated too.
