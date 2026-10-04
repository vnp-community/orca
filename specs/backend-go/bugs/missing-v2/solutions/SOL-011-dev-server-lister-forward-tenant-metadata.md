# SOL-011: `InfraFleetDevServerLister.Exists`/`InfraFleetHostnameResolver.Hostname` call `withTenantMetadata(ctx)` before the outbound `ListDevServers` RPC

> **✅ IMPLEMENTED (2026-09-15)** — code fix applied to both call sites, `go build` clean.
> **✅ Deployed** (`sync-to-server.sh 2026.09.15-bug010-fix`) — `docker logs orca-go-project`
> confirms `version":"2026.09.15-bug010-fix"` live on `b15.openledger.vn`, all 17 services +
> frontend `Up`, frontend health check passed.

## Bug Reference
- **Bug:** BUG-010
- **Severity:** High
- **Pattern:** identical to [SOL-008](./SOL-008-profile-resolver-forward-tenant-metadata.md) — same helper, same fix shape, different call site.

---

## Impact Analysis (required before editing, per AGENTS.md/CLAUDE.md)

```
gitnexus impact({target: "InfraFleetDevServerLister", direction: "upstream", repo: "orca"})
→ risk: LOW, impactedCount: 3 (NewInfraFleetDevServerLister → project-service's own run()/main() wiring only)

gitnexus impact({target: "InfraFleetHostnameResolver", direction: "upstream", repo: "orca"})
→ risk: LOW, impactedCount: 3 (same shape)
```

Both types are constructed once in `project-service/cmd/server/main.go` and injected as ports (`DevServerLister`, `DevServerHostnameResolver`) — no other caller, safe to change their outbound context handling without touching any usecase.

## Root Cause

See [BUG-010](../BUG-010-rebind-repo-dev-server-lookup-failed-missing-tenant-metadata.md).

## Fix

**File:** `backend-go/services/project-service/internal/adapter/grpcclient/infra_fleet_dev_server_lister.go`

```diff
 func (c *InfraFleetDevServerLister) Exists(ctx context.Context, tenantID, devServerID string) (bool, error) {
-	resp, err := c.client.ListDevServers(ctx, &infrafleetv1.ListDevServersRequest{})
+	outCtx, err := withTenantMetadata(ctx)
+	if err != nil {
+		return false, fmt.Errorf("grpcclient: dev server lister: %w", err)
+	}
+	resp, err := c.client.ListDevServers(outCtx, &infrafleetv1.ListDevServersRequest{})
```

**File:** `backend-go/services/project-service/internal/adapter/grpcclient/dev_server_hostname_resolver.go` — same shape:

```diff
 func (c *InfraFleetHostnameResolver) Hostname(ctx context.Context, tenantID, devServerID string) (string, error) {
-	resp, err := c.client.ListDevServers(ctx, &infrafleetv1.ListDevServersRequest{})
+	outCtx, err := withTenantMetadata(ctx)
+	if err != nil {
+		return "", fmt.Errorf("grpcclient: hostname resolver: %w", err)
+	}
+	resp, err := c.client.ListDevServers(outCtx, &infrafleetv1.ListDevServersRequest{})
```

Both files already had `withTenantMetadata` available in-package (`tenant_forwarding.go`) — no new import needed beyond what's already there (`fmt` was already imported in both files).

## Why the `tenantID` parameter stays unused

Both methods keep taking a `tenantID string` parameter (unused in the body, same as before this fix) — it's part of the `DevServerLister`/`DevServerHostnameResolver` port's interface contract shared with other implementations; `withTenantMetadata` re-derives the tenant from `ctx` itself (via `tenant.RequireTenantID`) rather than trusting the caller-supplied string, matching every other `withTenantMetadata` call site in this codebase. Not touched by this fix — out of scope, and changing a shared port signature would need its own impact analysis.

## Not done in this solution

- **Not investigating how `aiops-v3`/`vnp-asm`'s existing correct `dev_server_id` bindings got set** despite this bug making that path always fail — noted as an open question in BUG-010, not blocking this fix (the fix is correct regardless of how those rows got their value).
- **Not auditing every other `grpcclient` file in every other service** for the same missing-`withTenantMetadata` shape — BUG-008 and this bug are now 2 independent instances of the same pattern; a 3rd occurrence would be a strong signal to do that audit as its own task, but 2 data points isn't enough to justify the scope yet (same reasoning BUG-009's "lesson worth keeping" already called out for a different pair of data points).

## Testing

- `go build ./services/project-service/...` — clean.
- No new unit test added yet — see TASK-023 for the planned regression test (same shape as BUG-008/SOL-008's `profile_resolver_test.go`: assert outbound context carries the tenant metadata key, using a fake `InfraFleetServiceClient`).

## Deploy status

**✅ Deployed** (2026-09-15, `sync-to-server.sh 2026.09.15-bug010-fix`) — same run also carried
`SOL-FE-PW-003` (frontend dialog scroll fix). Live-verified via `docker logs orca-go-project`
reporting the new version; end-to-end repro (actually rebinding "aiops-v3" to `test-01` through
the UI) not yet confirmed — see BUG-010's status line.
