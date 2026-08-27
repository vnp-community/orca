# TASK-AUTH-05-07: `handleQueryAuditLog` extended filters + `GET /admin/api/audit/export` CSV streaming

**From Solution:** SOL-AUTH-05
**Priority:** P1
**Service:** `api-gateway` (httpgateway)
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go`, `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go`
**Depends on:** TASK-AUTH-05-06
**Status:** `[ ]` TODO

---

## Context

`GET /admin/api/audit` (and its `/v1/auth/audit-log` alias) only forwards `since`/`page_token`/`page_size` today — the admin console's audit view needs `action`/`userId`/`to` too, matching the spec's literal query param names (`userId`, not `actor_id`, on the wire). Separately, no CSV export exists; `QueryAuditLog` already returns paginated entries, so export is purely an `api-gateway`-side loop-to-completion-and-stream, no new RPC needed.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_admin_routes.go`, extend `handleQueryAuditLog`'s request construction:

```go
req := &authv1.QueryAuditLogRequest{
	TenantId:  identity.TenantID,
	PageToken: q.Get("page_token"),
	PageSize:  pageSize,
	Action:    q.Get("action"),
	ActorId:   q.Get("userId"), // "userId" matches the spec's literal query param name
}
if v := q.Get("since"); v != "" {
	// ... existing since-parsing unchanged, sets req.Since
}
if v := q.Get("to"); v != "" {
	to, err := time.Parse(time.RFC3339, v)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "to must be an RFC3339 timestamp")
		return
	}
	req.To = timestamppb.New(to)
}
```

Add `"google.golang.org/protobuf/types/known/timestamppb"` to imports if not already present.

In `backend-go/services/api-gateway/internal/adapter/httpgateway/admin_routes.go`, add a new mount and handler:

```go
// mountAdminRoutes — add:
//   sub.Get("/audit/export", handleExportAuditLog(client))

// handleExportAuditLog streams the full (paginated) audit log as CSV.
// Headers are sent before all pages are fetched — if a later page's RPC
// call fails, the client sees a truncated CSV rather than a clean JSON
// error, since headers can't be swapped mid-stream. Accepted for a first
// pass; a future improvement could buffer to a temp file and stream only
// once complete.
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
				TenantId:  identity.TenantID,
				PageToken: pageToken,
				PageSize:  200, // matches query_audit_log.go's existing pagination cap
			})
			if err != nil {
				return // best effort: stop writing rows, client sees a truncated CSV
			}
			for _, e := range resp.GetEntries() {
				_ = cw.Write([]string{
					e.GetId(), e.GetActorId(), e.GetAction(), e.GetTargetType(), e.GetTargetId(),
					e.GetIpAddress(), e.GetOccurredAt().AsTime().Format(time.RFC3339), e.GetMetadataJson(),
				})
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

Add `"encoding/csv"` and `"time"` to `admin_routes.go`'s imports if not already present. If the proto's `AuditEntry.metadata_json` field ended up named differently in TASK-AUTH-05-06, adjust `e.GetMetadataJson()` to match.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run "TestAuthAdminRoutes_QueryAuditLog|TestAdminRoutes_ExportAuditLog" -v
```

Expected: `GET /admin/api/audit?action=login.fail&userId=X&to=2026-01-01T00:00:00Z` forwards all three filters unchanged into the RPC request; a malformed `to` value → `400 INVALID_ARGUMENT` before the RPC call; `GET /admin/api/audit/export` returns `Content-Type: text/csv` and a body whose first line is the exact header row; a multi-page audit log (fake client returns 2 pages) produces one CSV with rows from both pages, no duplicate/missing rows at the page boundary.
