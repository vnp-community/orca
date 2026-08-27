# Admin: Audit Log Page

**Route / trigger:** Admin SPA, nav item "📋 Audit Log" (`AdminRoute = '/audit'`), served at `/admin` under `AdminApp` → `AdminLayout`.
**Top-level component:** `AuditPage` — `frontend/src/renderer/src/components/admin/AuditPage.tsx`

## Purpose
Lets a server admin review the security/activity audit trail — logins, logouts, SSH connects, user/policy changes, agent runs — with date-range and action-type filtering, and export the current result set to CSV. Used by operators of Orca's multi-user web server for compliance/incident review.

## Layout
Renders inside the shared Admin SPA shell (see `admin-users.md` for the header/nav diagram):

```
admin-content
├─ admin-page-header:  "Audit Log"          [Export CSV] (only if entries.length > 0)
├─ admin-filters:  [From date] [To date] [Action <select>] [Refresh]
├─ admin-error (conditional)
└─ admin-table (or "No audit logs found." row)
    Time | User | Action | Detail | IP
    ...one row per entry, 50 per page...
    admin-pagination:  [Previous]  Page N of M  [Next]
```

No sub-components — `AuditPage` is a single self-contained function. Client-side pagination (`paginatedEntries`, `limit = 50`) is applied on top of whatever the server returned for the current filter; the pager only renders when there's more than one page.

## Data shown
All calls go through `frontend/src/renderer/src/components/admin/admin-api-client.ts`'s `adminFetch()` wrapper (`/admin/api/*`, `credentials: 'include'`), plain HTTP.

- **Audit entries** — `GET /admin/api/audit` (with optional `?from=&to=&action=` query params) via `fetchAdminAudit(filter?: AuditFilter)`. Returns `AuditEntry[]`:
  ```ts
  type AuditEntry = {
    id: string
    createdAt: number     // epoch ms
    userId?: string
    userEmail?: string
    action: string
    detail?: string
    ipAddress?: string
  }
  type AuditFilter = { from?: number; to?: number; action?: string }
  ```
  Re-fetched whenever `fromDate`, `toDate`, or `actionFilter` change (`useCallback` dep array on `loadAudit`, triggered by `useEffect`), or when the user clicks "Refresh".
- Backed server-side by `AuditLogger` (`backend/src/main/admin/audit-logger.ts`) via `AdminAuditHandlers`, writing/reading an audit table; entries are also produced by other subsystems (e.g. `AuthManager.login()` logs `auth.login.success`/`auth.login.failed`, `startHttpServer` logs `server.start`).
- **Action filter dropdown** is a hardcoded list of known action strings (`login.success`, `login.fail`, `logout`, `ssh.connect`, `ssh.disconnect`, `user.create`, `user.update`, `user.deactivate`, `agent.run`, `policy.create`, `policy.update`) rather than being derived from the data or server.

## Key interactions
- **From / To date pickers** — set `fromDate`/`toDate` (native `<input type="date">`), converted to epoch ms and sent as `from`/`to` query params; changing either re-triggers the fetch automatically via the `useEffect([loadAudit])` dependency.
- **Action filter** — `<select>` of known action strings; changing it also auto-refetches.
- **Refresh** — manually re-runs `loadAudit()` (redundant with the auto-refetch-on-change behavior, kept as an explicit affordance per the code's own comment).
- **Export CSV** — client-side only: builds a CSV string from the *currently loaded* `entries` array (not paginated — the full filtered result set), creates a `Blob`, and triggers a synthetic `<a download>` click to save `audit-log.csv`. No server round-trip.
- **Previous / Next** — page through `entries` 50 at a time, entirely client-side (`page` state, no server-side pagination).

## Notable implementation details / known issues
- Pagination is 100% client-side over whatever `GET /admin/api/audit` returned for the active filter — there's no `limit`/`offset`/cursor sent to the server, so a broad or unfiltered query on a long-lived deployment could pull an unbounded number of rows into the browser before paginating.
- CSV export only covers what's currently loaded in `entries`, consistent with the above — if the audit log is large, admins may need to narrow the date range first to get a complete export.
- Like the other three admin pages, this one only mounts once `AdminApp`'s `useAuthUser()` check passes — see the note in `docs/ui/pages/admin-users.md` about the Zustand `AuthSlice` never being populated in the Admin SPA bundle (`checkSession()`/`setCurrentUser()` are never invoked from `admin-main.tsx`). The `/admin/api/audit` endpoint is independently protected by the backend's `requireAdmin` middleware regardless of that client-side gate's state.
