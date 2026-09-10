# TASK-SSH-01-03: Migration — add `port`, `known_hosts_fingerprint`, `jump_host_target_id` to `infra.ssh_targets`

**From Solution:** SOL-SSH-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/migrations/0007_ssh_targets_port_knownhosts_jumphost.up.sql`
**Depends on:** none
**Status:** `[x] DONE — 0007_ssh_targets_port_knownhosts_jumphost up/down migration added, integration round-trip test passes`

---

## Context

`infra-fleet-service.md` §5's `ssh_targets` DDL already specifies `port`,
`known_hosts_fingerprint`, and `jump_host_target_id` columns; the table as
migrated today (`0001_init.up.sql`) only has
`id, tenant_id, host, user_name, vault_ssh_role`. This is an additive
migration per `05-data-architecture.md`'s expand/contract policy — the
existing highest migration in this service is `0006_browser_profiles`.

## Changes to make

Create `backend-go/services/infra-fleet-service/migrations/0007_ssh_targets_port_knownhosts_jumphost.up.sql`:

```sql
-- Fills in port/known-hosts/jump-host from the fuller design-doc entity
-- (specs/backend-go/services/infra-fleet-service.md §5) that
-- domain.SshTarget's original scaffold left unmodeled. See SOL-SSH-01.
ALTER TABLE infra.ssh_targets ADD COLUMN port INTEGER NOT NULL DEFAULT 22;
ALTER TABLE infra.ssh_targets ADD COLUMN known_hosts_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE infra.ssh_targets ADD COLUMN jump_host_target_id UUID REFERENCES infra.ssh_targets(id);
```

Create `backend-go/services/infra-fleet-service/migrations/0007_ssh_targets_port_knownhosts_jumphost.down.sql`:

```sql
ALTER TABLE infra.ssh_targets DROP COLUMN jump_host_target_id;
ALTER TABLE infra.ssh_targets DROP COLUMN known_hosts_fingerprint;
ALTER TABLE infra.ssh_targets DROP COLUMN port;
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" up
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" down 1
migrate -path migrations -database "$INFRA_FLEET_DATABASE_URL" up
```

Expected: up/down/up round-trips cleanly (per `05-data-architecture.md`'s CI
rule), no orphaned `jump_host_target_id` FK errors on down since the column
itself is dropped.
