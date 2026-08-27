# Admin: Sessions Page

**Route / trigger:** Admin SPA, nav item "📡 Sessions" (`AdminRoute = '/sessions'`), served at `/admin` under `AdminApp` → `AdminLayout`.
**Top-level component:** `SessionsPage` — `frontend/src/renderer/src/components/admin/SessionsPage.tsx`

## Purpose
Gives a server admin visibility into every currently-active login session and a way to forcibly terminate one or all of them (e.g. to respond to a compromised account or force a global logout). Used by operators of Orca's multi-user web server.

## Layout
Renders inside the same Admin SPA shell as the other admin pages (see `admin-users.md` for the shared header/nav diagram):

```
admin-content
├─ admin-page-header:  "Active Sessions"     [Kill All]  (only if sessions.length > 0)
├─ admin-error (conditional)
└─ admin-table
    User Email | IP Address | Started | Last Seen | Action
    ...one row per session...            [Kill]
```

No sub-components — `SessionsPage` is a single self-contained function with an inline `formatRelative()` helper for the "Started"/"Last Seen" columns. Shows a `role="status"` loading message while the initial fetch is in flight, and a single "No active sessions." row when the list is empty.

## Data shown
All calls go through `frontend/src/renderer/src/components/admin/admin-api-client.ts`'s `adminFetch()` wrapper (`/admin/api/*`, `credentials: 'include'`), plain HTTP — not the RPC system.

- **Session list** — `GET /admin/api/sessions` via `fetchAdminSessions()`. Returns `AdminSession[]`:
  ```ts
  type AdminSession = {
    sessionId: string
    userId: string
    userEmail: string
    ipAddress: string
    userAgent?: string
    createdAt: number   // epoch ms
    lastSeenAt: number  // epoch ms
  }
  ```
  Loaded on mount via `loadSessions()`, stored in `sessions` state.
- **"Started" / "Last Seen" columns** — computed client-side by `formatRelative(ts)`, which buckets `Date.now() - ts` into whole hours (`"Nh ago"`) or otherwise minutes (`"Nm ago"`). Note `userAgent` is fetched but not currently displayed in the table.

## Key interactions
- **Kill** (per row) — confirm() dialog → optimistically removes the row from local state → `DELETE /admin/api/sessions/:sessionId` via `killAdminSession()`. On failure, shows an `alert()` and calls `loadSessions()` again to resync (undoing the optimistic removal if the kill actually failed).
- **Kill All** (header button, only shown when there's at least one session) — confirm() dialog warning it logs everyone out → clears local `sessions` state immediately → fires `killAdminSession()` for every session id in parallel via `Promise.all`. On any failure, `alert()`s and reloads the list.

## Notable implementation details / known issues
- "Kill All" is genuinely destructive — it logs out every user on the deployment, including the admin performing the action (their own session is in the same list and gets revoked like any other). The only guard is the browser `confirm()` dialog.
- The optimistic-update pattern (remove from state immediately, revert via `loadSessions()` only on `Promise` rejection) means a *partial* failure in "Kill All" (`Promise.all` — one rejected kill call rejects the whole batch) still results in a reload that could show sessions that were actually killed as still present until eyeballed again; there is no per-item success/failure reporting.
- Like the other three admin pages, this one only mounts once `AdminApp`'s `useAuthUser()` check passes — see the note in `docs/ui/pages/admin-users.md` about the Zustand `AuthSlice` never actually being populated in the Admin SPA bundle (`checkSession()`/`setCurrentUser()` are never called from `admin-main.tsx`), which can make the "Not authenticated. Redirecting…" placeholder show even with a valid session cookie. The `/admin/api/sessions` calls themselves are independently protected by the backend's `requireAdmin` middleware.
