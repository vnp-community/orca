# Login Page

**Route / trigger:** Served for any request to `/` in Orca's multi-user web deployment when the browser has no valid `orca_session` cookie. `WebRootBoundary` (in `main-web-bootstrap.tsx`) calls `GET /auth/me` on mount; if it returns 401 (no session), `WebRoot` renders `LoginPage` instead of the main `App`. There is no dedicated `/login` route in the router — it's a conditional render, though `useLogout` does a hard `window.location.href = '/login'` redirect after logout (served by the same static SPA shell via the catch-all static handler).
**Top-level component:** `LoginPage` — `frontend/src/renderer/src/web/login/LoginPage.tsx`

## Purpose
The entry screen for Orca's multi-user web server mode. Lets a user authenticate with local email/password or (once implemented) an SSO provider before the main IDE (`App`) is allowed to mount. Used by every web-mode user (not the desktop/Electron build, which has its own pairing flow).

## Layout
Single centered column, no sidebar:

```
┌─────────────────────────────────────┐
│  login-header                       │
│    "Orca" logo + tagline            │
├─────────────────────────────────────┤
│  login-content                      │
│    ┌─────────────────────────────┐  │
│    │ LoginForm (email/password)  │  │
│    └─────────────────────────────┘  │
│         — or — (divider, only if    │
│           SSO providers exist)      │
│    [SsoButton] [SsoButton] ...      │
└─────────────────────────────────────┘
```

- `LoginForm` (`frontend/src/renderer/src/web/login/LoginForm.tsx`) — always rendered. Controlled email + password inputs, client-side email regex validation (`EMAIL_RE`), submit button that shows "Signing in…" while `isLoading`.
- `SsoButton` (`frontend/src/renderer/src/web/login/SsoButton.tsx`) — one per entry in `availableProviders`; only rendered (with an "or" divider above it) when that list is non-empty. Renders as a plain `<a href="/auth/sso/:provider">` (not a fetch call) so the browser follows the OAuth redirect natively.
- Styling comes from `frontend/src/renderer/src/assets/login.css`, imported directly by `LoginPage`.

## Data shown
No server-fetched list data — this page only surfaces form state and one config call:

- `availableProviders: SsoProvider[]` — passed down as a prop from `WebRootBoundary`, sourced from `GET /auth/config` via `fetchAuthConfig()` (`frontend/src/renderer/src/auth/auth-api-client.ts`). Response shape: `{ providers: string[], localEnabled: boolean }`. **Currently the backend route (`backend/src/main/auth/auth-router.ts`, `GET /auth/config`) hardcodes `{ providers: [], localEnabled: true }`**, so in practice `availableProviders` is always empty and no `SsoButton`s ever render today.
- Local form state only: `email`, `password` (in `LoginForm`), `isLoading`, `error` (in `LoginPage`).
- All calls go through plain `fetch()` against `/auth/*` HTTP endpoints (not the app's RPC system) — see `frontend/src/renderer/src/auth/auth-api-client.ts`:
  - `POST /auth/local` — `loginLocal(email, password)`. Sends `{ email, password }` JSON, `credentials: 'include'`. On success, server sets an HttpOnly `orca_session` cookie and returns the `AuthUser` JSON body (`{ id, email, name, role, provider, avatarUrl? }`). On failure throws `AuthError('...', 'invalid_credentials')` built from the response's `error` field.
  - `GET /auth/sso/:provider` — full-page navigation via the `SsoButton` anchor; the backend stub currently just returns `501 { error: 'not_implemented' }` (SSO login is unimplemented).

## Key interactions
- Enter email + password and submit → `LoginForm.handleSubmit` validates the email format locally, then calls `LoginPage.handleLocalLogin`, which calls `loginLocal()`. On success it calls `onLoginSuccess(user)`, which in `WebRoot` is wired to `() => { window.location.href = '/' }` — a full page reload so `WebRootBoundary` re-checks `/auth/me` and mounts `App`.
- On failed local login, the `AuthError` message is shown inline in `LoginForm` via `role="alert"`.
- Click an SSO button (when providers are configured) → browser navigates to `/auth/sso/:provider` (server-side OAuth redirect, not yet implemented — returns 501).
- Inputs and the submit button are disabled while `isLoading` is true (mid-request).

## Notable implementation details / known issues
- **SSO is effectively dead code today.** `GET /auth/config` always returns `providers: []` server-side, so `availableProviders` is always empty and the "or / continue with GitHub/Google/Keycloak" UI never renders in the running app, even though `SsoButton` and its provider config are fully built. `GET /auth/sso/:provider` also just 501s.
- `PairCodeFallback` was deliberately removed from this page (see the file's header comment referencing CR-FE2E-002) — E2EE pairing has no valid use on this path since it's only shown for the multi-user backend, where local/SSO login is always available; pairing is preserved separately for the Desktop app's Pair Code flow.
- Auth session state resolved here (`sessionUser`, `availableProviders`) lives in **local component state** inside `WebRootBoundary`, not in the Zustand `AuthSlice` (`store/slices/auth.ts`). The slice's `checkSession()` action exists but is never called anywhere in the web bootstrap path — components that read auth via `useAuthUser()` / `useAuthSession()` (e.g. `AdminApp`) will see `currentUser: null` / `authStatus: 'unknown'` even after a successful login through this page, because nothing ever calls `setCurrentUser`/`checkSession` to sync the two. See `docs/ui/pages/admin-users.md` (and sibling admin docs) for the concrete downstream effect.
