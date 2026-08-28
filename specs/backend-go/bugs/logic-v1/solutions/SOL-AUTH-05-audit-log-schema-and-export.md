# SOL-AUTH-05: Audit log gains `target_type`/`target_id`/`metadata`/`ip_address`, `login.fail`, broader filters, and CSV export

**Resolves:** [BUG-AUTH-05](../BUG-AUTH-05-audit-log-partial.md)
**Service:** `auth-service` (schema, domain, usecase, proto) + `api-gateway` (CSV export, extended query params) + `infra-fleet-service` (`ssh.connect` event source)
**Affected files (proposed):**
- `backend-go/services/auth-service/migrations/000X_audit_log_metadata.up.sql` (+ `.down.sql`)
- `backend-go/proto/orca/auth/v1/auth.proto` — `AuditEntry` gains `target_type`/`target_id`/`metadata_json`/`ip_address`; `QueryAuditLogRequest` gains `action`/`actor_id`/`to`
- `backend-go/services/auth-service/internal/domain/audit.go` — `AuditEntry` struct + `NewAuditEntry` signature change
- `backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go` — extended `Append`/`Query`
- `backend-go/services/auth-service/internal/usecase/login.go`, `logout.go`, `create_user.go`, `deactivate_user.go`, `reactivate_user.go`, `update_user_role.go`, `revoke_session.go`, `force_revoke_all_sessions.go`, `bootstrap.go` — every existing `NewAuditEntry` call site updated to the new signature
- `backend-go/services/auth-service/internal/usecase/query_audit_log.go` — extended filters
- `backend-go/services/infra-fleet-service/internal/usecase/` — `ssh.connect` outbox publish at the connection-establish usecase (new)
- `backend-go/services/auth-service/internal/adapter/natsconsumer/audit_ingest.go` (new) — consumes cross-service audit events
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go` — `handleQueryAuditLog` extended filters; new `handleExportAuditLog`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go` — mount `/admin/api/audit/export`
- `backend-go/services/auth-service/internal/domain/audit_test.go`, `internal/usecase/query_audit_log_test.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

- **The domain model already specifies this exact split.** `auth-service.md` §4 (`auth-service.md:153-158`) describes `AuditEntry` as `actorUserID`, `action`, `resourceType`, `resourceID`, `payload` (structured, redacted of secret material) — i.e. `resourceType`/`resourceID` split and a structured `payload`, not the current codebase's single `Target string`. This solution's `TargetType`/`TargetID`/`Metadata` fields are that split, renamed to match this bug's own vocabulary (`target_type`/`target_id`/`metadata`, matching `docs/logic/auth/BL-AUTH-05-audit-log.md`'s literal schema cited in the bug report) — the same concept the TDD already names.
- **`ip_address` is a first-class column in both the TDD and the spec, not metadata.** `auth-service.md` §5's `sessions` table already carries `ip inet` (`auth-service.md:170`) as a typed column rather than JSON metadata, and BUG-AUTH-05 itself notes "The spec's audit schema explicitly calls out `ip_address` as a top-level column, not just embedded in `metadata`" (BUG-AUTH-05 line 33) — this solution follows that, giving `audit_log` its own `ip_address INET` column rather than folding it into the JSON blob.
- **Append-only integrity is preserved, not touched.** `domain.AuditEntry`'s doc comment ("there is deliberately no usecase method that updates or deletes an AuditEntry, only Append and Query", `audit.go:21-22`) and `auth-service.md` §9's "Audit log integrity" (database-permission-level `INSERT`/`SELECT` only, `auth-service.md:328-330`) are unaffected by this schema extension — new columns, same append-only contract, same missing `UPDATE`/`DELETE` grants.
- **`ssh.connect` requires cross-service ingestion, per `07-security-architecture.md`'s own audit-logging design** — "every service emits structured audit events (via the outbox pattern, same mechanism as domain events) for security-relevant actions in its own domain" (`07-security-architecture.md:71-73`), and `auth-service.md`'s overview already frames the audit log as "own + ingested from other services' outbox streams" (`auth-service.md:25-26`). SSH connection lifecycle is owned by `infra-fleet-service`, not `auth-service` (`02-microservices-decomposition.md:47`: "connection lifecycle" under `infra-fleet-service`'s row) — so `ssh.connect` can only reach the admin console's single audit feed via `infra-fleet-service` publishing to its outbox and `auth-service` consuming it, exactly as the TDD's own "own + ingested" framing describes. This solution's NATS-consumer design is the mechanical realization of a sentence the TDD already states but doesn't spell out — flagged as an extension of *how*, not *whether*.
- **CSV export needs no new RPC** — `QueryAuditLog` (`auth-service.md:116-119`) already returns paginated entries; CSV formatting is a `api-gateway`-side concern (looping the existing RPC to completion and streaming rows), consistent with `api-gateway`'s role as "response aggregation" (`02-microservices-decomposition.md:67`).

## Design — schema

```sql
-- 000X_audit_log_metadata.up.sql
ALTER TABLE auth.audit_log
  ADD COLUMN target_type TEXT,
  ADD COLUMN target_id   TEXT,
  ADD COLUMN metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN ip_address  INET;

-- Backfill: split the existing `target` column on the pre-existing
-- action-name convention (BUG-AUTH-05's own findings: user.* actions target
-- a user, session.* actions target a session) — best-effort, historical
-- rows may have target_type left NULL where the action name doesn't map
-- cleanly; this is a one-time backfill script, not application code.
UPDATE auth.audit_log SET
  target_type = split_part(action, '.', 1),
  target_id   = target
WHERE target_type IS NULL;

CREATE INDEX IF NOT EXISTS idx_audit_log_action ON auth.audit_log (action);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON auth.audit_log (target_type, target_id);
```

`target`/`Target` is kept as a column during a transition window (not
dropped in this migration) — every existing call site is updated in the
same change, but keeping the column one release longer costs nothing and
avoids a forced simultaneous cutover if this lands across multiple PRs.

## Design — `domain/audit.go`

```go
type AuditEntry struct {
    ID         string
    TenantID   string
    ActorID    string
    Action     string
    TargetType string            // "user" | "session" | "ssh_host" | ...
    TargetID   string
    Metadata   map[string]any    // JSON-serializable; redacted of secret material per auth-service.md §4's invariant
    IPAddress  string            // may be "" — not every action has a resolvable client IP (e.g. bootstrap, reaper-initiated revokes)
    OccurredAt time.Time
}

// NewAuditEntry's signature changes from (id, tenantID, actorID, action,
// target string, occurredAt) to split target into type+id and add the two
// new optional fields. This is a breaking signature change across all 9
// existing call sites (see "Affected files" and "Migration of existing call
// sites" below) — unavoidable given the fields being added are exactly what
// every call site under-specifies today.
func NewAuditEntry(id, tenantID, actorID, action, targetType, targetID string, metadata map[string]any, ipAddress string, occurredAt time.Time) (AuditEntry, error) {
    if id == "" { return AuditEntry{}, ErrEmptyID }
    if tenantID == "" { return AuditEntry{}, ErrEmptyTenant }
    if action == "" { return AuditEntry{}, ErrEmptyAction }
    if occurredAt.IsZero() { return AuditEntry{}, ErrZeroOccurredAt }
    // targetType/targetID are NOT required — a system-initiated event (the
    // reaper, bootstrap) may have neither, matching ActorID's existing
    // "may be empty for a system-initiated event" allowance (audit.go:18).
    if metadata == nil {
        metadata = map[string]any{}
    }
    return AuditEntry{
        ID: id, TenantID: tenantID, ActorID: actorID, Action: action,
        TargetType: targetType, TargetID: targetID, Metadata: metadata,
        IPAddress: ipAddress, OccurredAt: occurredAt,
    }, nil
}
```

### Migration of existing call sites

Every current call follows the shape `domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, "action.name", targetID, now)`.
Each becomes `domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, "action.name", "targetType", targetID, metadata, ipAddress, now)`:

| Call site | Old `target` | New `targetType`/`targetID` | New `metadata` |
|---|---|---|---|
| `login.go:94` (`user.login`) | `user.ID` | `"user"`, `user.ID` | `{"ip": ..., "userAgent": ...}` — from `LoginInput.IP`/`.UserAgent`, see [SOL-AUTH-01](./SOL-AUTH-01-local-login-error-mapping-rate-limit.md) |
| `login.go` new `login.fail` write (SOL-AUTH-01) | `in.Email` | `"user"`, best-effort resolved user ID or `""` | `{"ip": in.IP, "email": in.Email, "reason": reason}` |
| `logout.go:45` (`user.logout`) | session-derived user ID | `"session"`, token hash | `{}` |
| `create_user.go:77` (`user.created`) | new user's ID | `"user"`, new user's ID | `{"targetEmail": ..., "role": ...}` |
| `deactivate_user.go:49` (`user.deactivated`) | user ID | `"user"`, user ID | `{}` |
| `reactivate_user.go:47` (`user.reactivated`) | user ID | `"user"`, user ID | `{}` |
| `update_user_role.go:43` (`user.role_updated`) | user ID | `"user"`, user ID | `{"from": oldRole, "to": newRole}` |
| `revoke_session.go:50` (`session.revoked`) | token hash | `"session"`, token hash | `{}` |
| `force_revoke_all_sessions.go:47` (`session.force_revoke_all`) | user ID | `"user"`, user ID | `{"revokedCount": n}` |
| `bootstrap.go:90` (`user.bootstrap_created`) | new admin's ID | `"user"`, new admin's ID | `{}` |

`update_user_role.go`'s `{"from": oldRole, "to": newRole}` is the one call
site that needed a real, structured payload the old single-`Target` string
could never carry at all — the clearest concrete illustration of why this
migration is necessary rather than cosmetic.

## Design — `usecase/ports.go` / `postgres/audit_repository.go`

```go
// ports.go — AuditRepository.Query gains filter params
type AuditRepository interface {
    Append(ctx context.Context, entry domain.AuditEntry) error
    Query(ctx context.Context, filter AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error)
}

type AuditQueryFilter struct {
    TenantID string
    Since    time.Time
    To       time.Time // zero value = no upper bound
    Action   string    // "" = no filter
    ActorID  string    // "" = no filter
}
```

```go
func (r *Repository) Append(ctx context.Context, entry domain.AuditEntry) error {
    metadataJSON, err := json.Marshal(entry.Metadata)
    if err != nil {
        return fmt.Errorf("postgres: marshal audit metadata: %w", err)
    }
    var ip any
    if entry.IPAddress != "" { ip = entry.IPAddress }
    _, err = r.pool.Exec(ctx, `
        INSERT INTO auth.audit_log (id, tenant_id, actor_id, action, target_type, target_id, metadata, ip_address, occurred_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
    `, entry.ID, entry.TenantID, nullIfEmpty(entry.ActorID), entry.Action, entry.TargetType, entry.TargetID, metadataJSON, ip, entry.OccurredAt)
    // ... error wrap unchanged
}

func (r *Repository) Query(ctx context.Context, filter usecase.AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
    // Builds a WHERE clause incrementally — tenant_id + occurred_at >= since
    // always present (unchanged base predicate from audit_repository.go:33-37);
    // action/actor_id/to added only when non-empty/non-zero, via a small
    // query builder rather than a fixed positional-args SQL string, since
    // the number of optional predicates now varies.
}
```

## Design — `usecase/login.go` (`ssh.connect` ingestion, cross-service)

```go
// internal/adapter/natsconsumer/audit_ingest.go — auth-service subscribes
// to infra-fleet-service's outbox-published connection events, per
// auth-service.md:25-26's "own + ingested from other services' outbox
// streams" and 07-security-architecture.md:71-73's outbox-pattern audit
// design.
func (c *AuditIngestConsumer) handleSSHConnected(msg *nats.Msg) {
    var evt sshConnectedEvent // {actor_user_id, tenant_id, connection_id, host, occurred_at}
    if err := json.Unmarshal(msg.Data, &evt); err != nil {
        return // malformed event — logged, not fatal to the consumer loop
    }
    entry, err := domain.NewAuditEntry(uuid.NewString(), evt.TenantID, evt.ActorUserID,
        "ssh.connect", "ssh_host", evt.Host,
        map[string]any{"connectionId": evt.ConnectionID}, "", evt.OccurredAt)
    if err != nil {
        return
    }
    _ = c.audit.Append(context.Background(), entry)
}
```

`infra-fleet-service`'s connection-establish usecase (the one BUG-AUTH-05
confirms currently has no audit call at all, `grep -rn "ssh.connect"`
returning zero matches) publishes to its own outbox after a connection is
established, using the same outbox-insert pattern its other domain events
already use (not designed further here — mechanical, matches
`auth-service.md` §7's own outbox description for `user.created` etc.).
Trust boundary: the NATS subject is only publishable by services inside the
mesh (mTLS + default-deny `NetworkPolicy` per `07-security-architecture.md:19-22`),
so this consumer does not need to re-authenticate the event's claimed actor
beyond what `infra-fleet-service` already validated when the connection was
established.

## Design — wiring (`api-gateway`)

### Extended filters

```go
// auth_admin_routes.go — handleQueryAuditLog (auth_admin_routes.go:147-184)
// gains action/actor_id/to alongside the existing since/page_token/page_size
req := &authv1.QueryAuditLogRequest{
    TenantId: identity.TenantID, PageToken: q.Get("page_token"), PageSize: pageSize,
    Action: q.Get("action"), ActorId: q.Get("userId"), // "userId" matches the spec's literal query param name
}
if v := q.Get("from"); v != "" { /* parses into Since, same as existing "since" handling */ }
if v := q.Get("to"); v != "" {
    to, err := time.Parse(time.RFC3339, v)
    if err != nil { writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "to must be an RFC3339 timestamp"); return }
    req.To = timestamppb.New(to)
}
```

### CSV export

```go
// admin_routes.go — new route: sub.Get("/audit/export", handleExportAuditLog(client))
func handleExportAuditLog(client authv1.AuthServiceClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        identity, _ := identityFromContext(r.Context())
        ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)

        w.Header().Set("Content-Type", "text/csv")
        w.Header().Set("Content-Disposition", `attachment; filename="audit-log.csv"`)
        cw := csv.NewWriter(w)
        _ = cw.Write([]string{"id", "actor_id", "action", "target_type", "target_id", "ip_address", "occurred_at", "metadata"})

        pageToken := ""
        for {
            resp, err := client.QueryAuditLog(ctx, &authv1.QueryAuditLogRequest{
                TenantId: identity.TenantID, PageToken: pageToken, PageSize: 200, // repo's existing pagination cap (query_audit_log.go:36-51's cap/50/200)
            })
            if err != nil {
                // Streaming has already started (headers sent) — cannot
                // switch to a JSON error response mid-stream. Best effort:
                // stop writing rows; the client sees a truncated CSV.
                return
            }
            for _, e := range resp.GetEntries() {
                metaJSON, _ := json.Marshal(e.GetMetadata())
                _ = cw.Write([]string{e.GetId(), e.GetActorId(), e.GetAction(), e.GetTargetType(), e.GetTargetId(), e.GetIpAddress(), e.GetOccurredAt().AsTime().Format(time.RFC3339), string(metaJSON)})
            }
            if resp.GetNextPageToken() == "" {
                break
            }
            pageToken = resp.GetNextPageToken()
        }
        cw.Flush()
    }
}
```

The "headers already sent, can't error out cleanly mid-stream" tradeoff is
called out explicitly rather than silently accepted — an admin exporting a
very large audit log could see a truncated file on a transient RPC failure.
Acceptable for a first pass (matches this bug's Medium/P1 severity); a
future improvement could buffer to a temp file first and only stream once
complete, at the cost of holding the full export in temporary storage.

## Test plan

- `domain/audit_test.go`: `NewAuditEntry` with nil `metadata` → normalizes to `{}`, not `nil` (guards a `json.Marshal(nil)` → `"null"` surprise downstream); empty `targetType`/`targetID` still constructs successfully (system-initiated events); existing invariant tests (`ErrEmptyID`, `ErrEmptyTenant`, `ErrEmptyAction`, `ErrZeroOccurredAt`) still pass against the new signature.
- Per-call-site regression test (one per row in the migration table above): each usecase's audit write now includes the documented `targetType`/metadata shape — e.g. `update_user_role_test.go` asserts the appended entry's `Metadata["from"]`/`Metadata["to"]` match the actual role transition.
- `postgres/audit_repository_test.go`: `Append` round-trips `metadata` through JSONB correctly (including nested values); `Query` with `action`/`actor_id`/`to` filters each narrow results correctly in isolation and combined; backfill migration test — a pre-migration row with only `target` populated gets a sensible `target_type` after the `UPDATE` backfill statement runs against a fixture.
- `audit_ingest_test.go`: a well-formed `ssh.connect` NATS message produces exactly one `Append` call with `action: "ssh.connect"`, `target_type: "ssh_host"`; a malformed message is dropped without panicking or blocking the consumer loop.
- `admin_routes_test.go`: `GET /admin/api/audit/export` returns `Content-Type: text/csv` and a body whose first line is the exact header row; a multi-page audit log (fake client returns 2 pages) produces one CSV with rows from both pages, no duplicate/missing rows at the page boundary; `GET /admin/api/audit?action=login.fail&userId=X` forwards both filters to the RPC request unchanged.

## References

- `backend-go/services/auth-service/internal/domain/audit.go:1-56` — current `AuditEntry`/`NewAuditEntry`
- `backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go:1-60` — current `Append`/`Query`
- `backend-go/services/auth-service/internal/usecase/query_audit_log.go:11-51`, `ports.go:124-130` — current `QueryAuditLog` usecase and `AuditRepository` port
- Every existing `NewAuditEntry` call site: `login.go:93-99`, `logout.go:45`, `create_user.go:77`, `deactivate_user.go:49`, `reactivate_user.go:47`, `update_user_role.go:43`, `revoke_session.go:50`, `force_revoke_all_sessions.go:47`, `bootstrap.go:90`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go:147-184` — current `handleQueryAuditLog`
- `backend-go/proto/gen/go/orca/auth/v1/auth.pb.go:1035-1118` — current `AuditEntry` proto message (`Id, TenantId, ActorId, Action, Target, OccurredAt` — confirms the collapsed-`Target` gap)
- `specs/backend-go/tdd/services/auth-service.md:25-26` ("own + ingested from other services' outbox streams"), `:116-119` (§3 `QueryAuditLog`), `:153-158` (§4 `AuditEntry` domain model — `resourceType`/`resourceID`/`payload` already specified), `:328-333` (§9 audit log integrity)
- `specs/backend-go/tdd/architecture/07-security-architecture.md:68-80` ("Audit logging" — outbox pattern, per-service emission, retention)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:47` (`infra-fleet-service` owns "connection lifecycle")
- `specs/backend-go/bugs/logic-v1/BUG-AUTH-01-local-login-partial.md` — the `login.fail`/IP-capture dependency this solution's audit write assumes
