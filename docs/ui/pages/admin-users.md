# Admin: Users Page

**Route / trigger:** Admin SPA, nav item "👥 Users" (`AdminRoute = '/users'`). The Admin SPA itself is served at `/admin` (static shell `admin-index.html`, mounted by `frontend/src/renderer/src/admin/admin-main.tsx`), separate from the main IDE bundle. Routing inside the SPA is plain `useState<AdminRoute>` prop-drilling in `AdminApp` — no `react-router-dom`.
**Top-level component:** `UsersPage` — `frontend/src/renderer/src/components/admin/UsersPage.tsx`

## Purpose
Lets an Orca server admin see every user account on the deployment, search/filter them, create new accounts, edit existing ones, and deactivate accounts. Used by operators of Orca's multi-user web server, not by regular developers.

## Layout
`UsersPage` renders inside the shared Admin SPA shell (`AdminApp` → `AdminLayout`, `frontend/src/renderer/src/components/admin/AdminLayout.tsx`):

```
┌───────────────────────────────────────────────────────┐
│ admin-header:  🔧 Orca Admin        user@email  Logout │
├───────────┬─────────────────────────────────────────┤
│ admin-nav │ admin-content (= UsersPage)              │
│  📊 Dash  │  admin-page-header: "Users"  [Create User]│
│  👥 Users │  admin-filters: [search box] [role select]│
│  🔐 Polic │  admin-table:                             │
│  📡 Sess  │   Status | Name | Email | Role | Provider │
│  📋 Audit │            | Actions (Edit / Deactivate)   │
│  🤖 AI    │                                            │
│  🏢 Prof  │                                            │
│  🧑‍🤝‍🧑 Teams│                                            │
│  🖥️ Fleet │                                            │
└───────────┴─────────────────────────────────────────┘
```

- `AdminLayout` (`frontend/src/renderer/src/components/admin/AdminLayout.tsx`) — always-visible header (logo, current admin's email, Logout button) + left `admin-nav` sidebar built from a static `NAV_ITEMS` list, and `admin-content` (`<main>`) holding whichever page is active.
- Inside `UsersPage`: a header row (title + "Create User" button), a filter bar (search input, role `<select>`), and an `admin-table` listing users. A loading state (`role="status"`) replaces the table while the initial fetch is in flight; an inline `admin-error` div shows fetch/action errors.
- "Create User" and each row's "Edit" button navigate (via `onNavigate`, passed down from `AdminApp`) to `UserForm` (`frontend/src/renderer/src/components/admin/UserForm.tsx`), rendered by `AdminApp`'s `PageContent` switch for routes `/users/new` and `/users/:id/edit` — not a separate page in this doc set, but the create/edit destination for this page's actions.

## Data shown
All calls go through plain `fetch()` wrappers in `frontend/src/renderer/src/components/admin/admin-api-client.ts` (`adminFetch`, base path `/admin/api/*`, always `credentials: 'include'`) — not the app's RPC system.

- **User list** — `GET /admin/api/users` via `fetchAdminUsers()`. Returns `AdminUser[]`:
  ```ts
  type AdminUser = {
    id: string
    email: string
    name: string
    role: 'developer' | 'lead' | 'admin'
    provider: 'none' | 'github' | 'google' | 'keycloak'
    isActive: boolean
    lastLoginAt: number | null
  }
  ```
  Loaded on mount via `loadUsers()`/`useEffect`, held in `users` state.
- **Filtered view** — `filteredUsers` is a `useMemo` over `users`, applying the `roleFilter` select (`all | developer | lead | admin`) and a case-insensitive substring match of `search` against `name` and `email`. Purely client-side; no server-side search/pagination.
- Each row shows a 🟢/🔴 status dot for `isActive`, plus `name`, `email`, `role`, `provider`.

## Key interactions
- **Search** — typing in the search box filters the table client-side by name/email substring.
- **Filter by role** — the role `<select>` narrows the table to one role or "All".
- **Create User** — button navigates to `/users/new` → `UserForm` in create mode → `POST /admin/api/users` via `createAdminUser()` with `{ email, name, role, provider, isActive, password? }` (password only sent for `provider: 'none'` local accounts; local creation double-checks `password === confirmPassword` client-side).
- **Edit** — per-row button navigates to `/users/:id/edit` → `UserForm` in edit mode, prefilled by refetching the full user list and finding the matching id → `PATCH /admin/api/users/:id` via `updateAdminUser()`.
- **Deactivate** — per-row button (only shown while `isActive`), guarded by a native `confirm()` dialog → `DELETE /admin/api/users/:id` via `deactivateAdminUser()` (soft-delete server-side — sets `is_active = 0`, row preserved for audit). On success the row is optimistically flipped to `isActive: false` in local state; on failure an `alert()` shows the error.

## Notable implementation details / known issues
- `deactivateAdminUser` issues an HTTP `DELETE` but is a **soft delete** — `AuthUserStore.deactivateUser` (`backend/src/main/auth/auth-user-store.ts`) only flips `is_active = 0`; the row and its audit trail are retained. There is no "reactivate" action in the UI even though the data model supports it (`isActive` is a normal field on `PATCH`).
- Filtering/search is entirely client-side over the full user list fetched once — there's no pagination or server-side query, so this will not scale well on deployments with very large user counts.
- Like all four admin pages, `UsersPage` only renders once `AdminApp`'s `useAuthUser()` guard passes. That guard reads `currentUser` from the Zustand `AuthSlice`, but nothing in the Admin SPA's bootstrap (`admin-main.tsx` → `AdminApp`) ever calls `checkSession()` or `setCurrentUser()` to populate it — the slice defaults to `currentUser: null`. In practice this means the "Not authenticated. Redirecting…" placeholder can show even for a browser holding a valid `orca_session` cookie, since the guard's data source is never wired up in this bundle (the comment in `AdminApp.tsx` — "backend redirects to /login" — does not correspond to any actual redirect logic). All API calls this page makes are independently protected server-side by `requireAdmin` middleware (401/403 on bad/missing session), so the page is not insecure, but the client-side gate itself appears effectively non-functional as written.
