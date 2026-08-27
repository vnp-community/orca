# Admin: Policies Page

**Route / trigger:** Admin SPA, nav item "🔐 Policies" (`AdminRoute = '/policies'`), served at `/admin` under `AdminApp` → `AdminLayout`.
**Top-level component:** `PoliciesPage` — `frontend/src/renderer/src/components/admin/PoliciesPage.tsx`

## Purpose
Lets a server admin define and manage access policies — which teams/roles can use which servers, and coarse-grained permission toggles (worktree creation/deletion, production access). Used by operators of Orca's multi-user web server to control what different user cohorts are allowed to do.

## Layout
Renders inside the shared Admin SPA shell (see `admin-users.md` for the header/nav diagram). Unlike the table-based pages, this one uses a card grid:

```
admin-content
├─ admin-page-header:  "Access Policies"   [+ New Policy]
├─ admin-error (conditional)
└─ admin-policy-cards  (or "No policies found." if empty)
    ┌ policy-card ─────────────────────────┐
    │ <policy name>                        │
    │ Applies to: Teams: ...  Roles: ...   │
    │ Allowed Servers: ...                 │
    │ Permissions: Create/Delete Worktrees,│
    │              Access Production       │
    │ [Edit]  [Delete]                     │
    └───────────────────────────────────────┘
    ...one card per policy...
```

- No sub-components inside `PoliciesPage` itself — cards are rendered inline.
- "New Policy" and each card's "Edit" button navigate (via `onNavigate`, passed from `AdminApp`) to `PolicyForm` (`frontend/src/renderer/src/components/admin/PolicyForm.tsx`), mounted by `AdminApp`'s `PageContent` switch for `/policies/new` and `/policies/:id/edit`.

## Data shown
All calls go through `frontend/src/renderer/src/components/admin/admin-api-client.ts`'s `adminFetch()` wrapper (`/admin/api/*`, `credentials: 'include'`), plain HTTP.

- **Policy list** — `GET /admin/api/policies` via `fetchAdminPolicies()`. Returns `AdminPolicy[]`:
  ```ts
  type AdminPolicy = {
    id: string
    name: string
    teams: string[]
    roles: ('developer' | 'lead' | 'admin')[]
    allowedServers: string[]   // '*' means all
    canCreateWorktrees: boolean
    canDeleteWorktrees: boolean
    canAccessProduction: boolean
  }
  ```
  Loaded on mount via `loadPolicies()`, stored in `policies` state.
- Each card renders `teams`/`roles` joined with `, '` (or "None" if empty), `allowedServers` joined similarly, and the three boolean permissions as "Yes"/"No".
- `PolicyForm` (the create/edit destination) parses `teams` and `allowedServers` from comma-separated text inputs client-side, and `roles` from a `Set` built off three checkboxes; it round-trips through `createAdminPolicy()` (`POST /admin/api/policies`) or `updateAdminPolicy()` (`PATCH /admin/api/policies/:id`).

## Key interactions
- **+ New Policy** — navigates to `/policies/new` → `PolicyForm` in create mode → `POST /admin/api/policies`.
- **Edit** (per card) — navigates to `/policies/:id/edit` → `PolicyForm` in edit mode, prefilled by refetching the full policy list and finding the matching id → `PATCH /admin/api/policies/:id`.
- **Delete** (per card) — `confirm()` dialog → `DELETE /admin/api/policies/:id` via `deleteAdminPolicy()` → removes the card from local state on success; `alert()`s on failure (no rollback needed since it's not optimistic — the local list is only updated after the request resolves).

## Notable implementation details / known issues
- Unlike Users/Sessions, `PoliciesPage`'s delete is **not optimistic** — it awaits the `DELETE` response before touching local state, so there's no need for the revert-on-failure pattern used elsewhere in this SPA.
- Team names and server names are free-text, comma-separated strings typed into `PolicyForm` — there's no validation against the actual list of teams/servers registered elsewhere in Orca, so a typo silently creates a policy that never matches anything.
- Like the other three admin pages, this one only mounts once `AdminApp`'s `useAuthUser()` check passes — see the note in `docs/ui/pages/admin-users.md` about the Zustand `AuthSlice` never being populated in the Admin SPA bundle. The `/admin/api/policies` endpoints are independently protected by the backend's `requireAdmin` middleware regardless of that client-side gate's state.
