# BACKLOG-018: `POST /api/mobile/push-subscribe` route — needs a mobile SSO CR before it can be built

**Origin:** `specs/backend-go/crs/v4/mobile-companion/tasks/TASK-BE-MOBILE-010-mobile-push-subscribe-route.md`, `specs/backend-go/crs/v4/mobile-companion/solutions/BE-MOBILE-SOL-002-mobile-subscribe-auth-endpoint.md` §1.2/§1.3/§1.4/§2
**Priority:** Medium — blocks the only remaining piece of F03 Mobile Companion's backend-go work
**Blocked on:** **A new CR for mobile SSO infrastructure** (backend-go `auth-service` + `frontend`), not engineering time on this task itself
**Owner:** whoever owns backend-go auth architecture — the CR needs to be written before this task can move to TODO

---

## What this is

F03 (Mobile Companion)'s push-delivery pipeline is fully built and working
end-to-end (`notification-service`'s `DeliverMobilePush` — APNs/FCM/WebPush,
9/9 tasks done, 2026-09-09). The one missing piece is the route mobile apps
actually call to register a device for push: `POST
/api/mobile/push-subscribe` on `api-gateway`. This route can't use the
existing `authMiddleware`/`CookieSessionValidator` path — a mobile app has
no browser cookie — so it needs its own caller-authentication mechanism.

Without it: either the route doesn't exist (mobile push registration is
impossible), or it gets built without real auth, which is a security hole
(anyone could register a push subscription for an arbitrary `user_id`).

## The decision that's already been made

**Option B — mobile has its own SSO flow — is chosen** (2026-09-09, decided
directly by the person who owns this). Option A (relay auth through
`desktop/`) is ruled out. Scope is confirmed: only `backend-go` (likely
`auth-service` + `api-gateway`) and `frontend` are touched; `desktop/`'s
current mechanism does not change.

## Why it's still blocked

Choosing B didn't unblock the task — B itself needs a CR that doesn't exist
yet: a mobile SSO flow (token issuance, refresh, revocation) that
`AuthMiddleware` can validate as `Authorization: Bearer <jwt>`. Until that
CR exists and specifies the mobile JWT's format/claims, this task can't be
written correctly — writing a `DeviceTokenValidator` against a guessed JWT
shape would just lock in an unreviewed security decision.

**Do not** default this to some ad hoc mechanism (e.g. "HMAC with a shared
secret") just to get code written. That was explicitly ruled out when this
task was first scoped, and remains ruled out now that B is chosen — the
CR needs to specify the real mechanism.

## What unblocks this

1. Write the mobile SSO infrastructure CR (token issuance/refresh/revocation
   flow, JWT claims shape) — `auth-service` + `frontend`, scoped explicitly to
   NOT touch `desktop/`.
2. Once that CR exists with enough detail to know the JWT's claims,
   `TASK-BE-MOBILE-010` can be rewritten against Option B's real shape (the
   Option-A-flavored route skeleton currently in the task file is historical
   reference only, not an implementation template) and moved to TODO.
