# BUG-013: `ISSUETRACKING_AUTH_FAILED` — Jira adapter hardcodes Cloud-only `/rest/api/3/` + email/API-token auth, always fails against self-hosted Jira Server/Data Center

**Service:** `issue-tracking-service`
**File:** `internal/adapter/jira/client.go` (every method — `Whoami`, `ListIssues`, `CreateIssue`, etc. all build URLs as `cred.BaseURL + "/rest/api/3/..."` and auth as `Basic base64(email:token)`)
**Severity:** High — blocks connecting to any Jira instance that isn't Atlassian-hosted Cloud, with an error message ("could not authenticate with the provided credential") that actively points the user toward the wrong fix (re-checking/regenerating credentials that were never the problem)
**Symptom:**
```
rpc error: code = InvalidArgument desc = ISSUETRACKING_AUTH_FAILED: could not authenticate with the provided credential
```
Reported live on `b15.openledger.vn`, connecting to `https://jr.servicehub.vn` (a VNPay-internal, custom-domain Jira instance — not `*.atlassian.net`) with valid Atlassian email + a freshly-created API token.

**Status:** ✅ **Fixed & confirmed working (2026-09-15)** — the "OAuth-required gateway" theory below was a **false alarm from testing with an invalid token**; verified live with the user's real PAT via `curl`: `Bearer <real token>` → `200 OK`, real user data (`luatnc@vnpay.vn`). `serverInfo` with the same token confirms `deploymentType: "Server"`. The fix is correct as designed — the only remaining issue was **UI usage**: `authHeaderValue` picks Basic when `Email` is non-empty, so a user who fills in both Email and Token gets Basic auth (rejected) instead of Bearer (works) — must leave Email blank. See "Correction" below.

---

## Root Cause — CONFIRMED

`jira-connect-dialog.tsx`'s own UI copy says only *"Use a **Jira Cloud** site URL"* — but the input field itself doesn't restrict or warn on a non-`*.atlassian.net` URL, and nothing in the connect flow tells the user their site type is unsupported. `jr.servicehub.vn` is a self-hosted Jira instance (Server or Data Center) — confirmed live:

```
curl https://jr.servicehub.vn/rest/api/3/myself  → HTTP 302  (Cloud-only endpoint — this site
                                                                 doesn't recognize it as an API
                                                                 route, falls through to the web
                                                                 app's login redirect)
curl https://jr.servicehub.vn/rest/api/2/myself  → HTTP 401  (Server/Data Center's real endpoint
                                                                 — recognized, just needs valid
                                                                 auth, which we didn't send)
```

`/rest/api/3/` is a **Jira Cloud-only** API version — it does not exist on Jira Server or Data Center, which top out at `/rest/api/2/`. `internal/adapter/jira/client.go` hardcodes `/rest/api/3/` in **every single method** (`Whoami`, `SearchIssues`, `ListIssues`, `GetIssue`, `CreateIssue`, `UpdateIssue`, comments, `ListProjects`, `createmeta` for issue types/fields, assignable users — every URL construction in the file). Against a Server/Data Center instance, `Whoami`'s request to `/rest/api/3/myself` never reaches Jira's real auth check at all — the site's own routing treats the unknown path as a web request and redirects (302) toward its login page, so the HTTP client either follows the redirect into an HTML page (not the JSON `jiraMyselfResponse` shape the code expects to unmarshal) or fails some other way downstream — either way, `provider.Whoami(ctx, cred)` returns an error **regardless of whether the credential is valid**, and `connect.go`'s `Execute` wraps every `Whoami` error identically as `ISSUETRACKING_AUTH_FAILED: could not authenticate with the provided credential` (`connect.go:60`) — a message that is actively misleading here, since the real problem has nothing to do with the credential.

## Compounding: the UI's help text is also Cloud-only

`jira-integration-card.tsx`: *"Connect a Jira Cloud site with your Atlassian email and an API token"*, and the dialog's own footer: *"Create a token in Atlassian account settings"* (linking to `id.atlassian.com`'s API token page). Neither applies to a self-hosted instance:
- Jira Server/Data Center users create **Personal Access Tokens from their OWN instance** (typically `https://<site>/plugins/servlet/personal-access-tokens`), not `id.atlassian.com` (Atlassian Cloud identity — doesn't exist for a self-hosted install).
- Server/Data Center's REST API v2 auth model differs too — Basic Auth with `email:api-token` is a **Cloud-specific** convention (Cloud's API tokens are tied to the Atlassian account email); Server/Data Center typically expects a username (not necessarily an email) paired with a password, or a PAT sent as a Bearer token, depending on version/config — not guaranteed to be interchangeable with Cloud's scheme even if the API version were fixed.

So even after fixing the API-version mismatch, Server/Data Center support would need its own auth-scheme branch — this bug's scope is "Cloud-only assumption baked into every layer" (URL version, auth header shape, AND the UI's own guidance text), not a single one-line fix.

## Impact

- Blocks 100% of self-hosted Jira instances from ever connecting successfully, with zero diagnostic signal pointing at the real cause — every failure looks identical to a wrong-credential problem.
- Directly blocks this session's own investigated end-to-end flow (BUG-013 was found while checking configuration for the Jira→worktree flow described in this same investigation) — `luatnc@vnpay.vn`'s real, valid credential against `jr.servicehub.vn` cannot work today regardless of retries.
- Given VNPay's own internal Jira is confirmed self-hosted (this exact site), this isn't a hypothetical edge case — it's the primary real-world Jira deployment this session's user actually needs to connect to.

## Fix direction (not designed into a SOL yet)

1. **Detect or let the user specify Server/Data Center vs Cloud** — either probe (`/rest/api/2/serverInfo` distinguishes Cloud's `deploymentType: "Cloud"` from Server/DC) or add an explicit toggle in `JiraConnectDialog`.
2. **Branch the API version** (`/rest/api/2/` vs `/rest/api/3/`) per connection, not hardcoded per file.
3. **Branch the auth scheme** — Cloud's `email:api-token` Basic Auth vs Server/Data Center's own (commonly `username:password` Basic Auth or a PAT Bearer token) — needs product/design input on which Server/DC auth modes to actually support, not assumed here.
4. **Fix the UI copy** conditionally (token-creation instructions, help text) once the toggle in #1 exists.

This is a multi-layer change (adapter, usecase `Credential` shape, proto if a new field is needed, UI) — deliberately not scoped into a full SOL in this pass; needs its own design discussion given the auth-scheme ambiguity in #3.

## Fix implemented (2026-09-15)

`internal/adapter/jira/client.go` rewritten:

1. **`resolveAPIVersion(ctx, cred)`** — probes `GET {baseURL}/rest/api/2/serverInfo` (exists on both Cloud and Server/Data Center), reads `deploymentType`. Returns `"2"` for `"Server"`/`"Data Center"`, else `"3"` (safe fallback, matches pre-fix Cloud-only behavior for Cloud/unknown sites). Cached per `baseURL` for 10 minutes (`apiVersionCache`, `sync.Map`) to avoid doubling every single API call's latency.
2. **`authHeaderValue(cred)`** — `Basic email:token` when `cred.Email != ""` (unchanged Cloud contract), else `Bearer <token>` (Server/Data Center Personal Access Token convention) — lets a self-hosted caller authenticate by leaving Email blank and putting a PAT in Token, with **zero UI change required**.
3. Every method's URL construction now goes through `apiURL(baseURL, apiVersion, path)` instead of a hardcoded `/rest/api/3/...` literal.
4. `CreateIssue`/`UpdateIssue`/`AddIssueComment`'s `Description`/comment body is version-branched: ADF document on v3 (Cloud, unchanged), plain string on v2 (Server/Data Center — v2 doesn't support ADF).

**Verified via `go test`**: 2 new regression tests (`TestWhoami_SelfHostedDataCenter_UsesV2AndBearerAuth`, `TestWhoami_CloudSite_StillUsesV3AndBasicAuth`) — the first proves a Data-Center-detected site uses `/rest/api/2/myself` + `Bearer` auth; the second proves an existing Cloud caller's behavior (`/rest/api/3/myself` + `Basic email:token`) is byte-for-byte unchanged. All 10 tests in the package pass; 2 pre-existing tests needed updating to account for the new `serverInfo` probe call (not a regression — those tests' fake HTTP servers now also handle that one extra request).

**Not yet verified against the real `jr.servicehub.vn` site** (no live connect attempt with the deployed fix) — the underlying auth SCHEME assumption (PAT via Bearer when Email is blank) is a reasonable default per Atlassian's own Data Center documentation, but wasn't confirmed with VNPay's Jira ops team beforehand (CR-JIRA-001 originally flagged this as needing that confirmation). **Ask the user to retry Connect with Email left blank and a Personal Access Token (created on `jr.servicehub.vn` itself, not `id.atlassian.com`) in the Token field, once deployed.**

## Correction (2026-09-15, same day) — "OAuth-required gateway" was a false alarm

The section below was written after testing the gateway with **no auth** and a **deliberately invalid Bearer token** — both correctly got `401` + `WWW-Authenticate: OAuth` (that header is just this Jira Server install's configured challenge scheme name for *any* rejected request, not a statement that Bearer/PAT is unsupported). Once the user shared their real PAT, `curl -H "Authorization: Bearer <real token>" https://jr.servicehub.vn/rest/api/2/myself` returned `200 OK` with real profile data — Bearer auth works fine. The actual remaining gap was much simpler: **the UI form must be submitted with Email left blank** — `authHeaderValue(cred)` picks `Basic` whenever `Email != ""`, and the user had been filling in their email alongside the token, so the code (correctly, per its own logic) chose Basic — which this site does reject. Kept the disproven section below for the record, per this bug family's established practice (see BUG-009's own superseded-theory precedent) — do not act on it.

<details>
<summary>Disproven "OAuth-required gateway" theory — kept for the record, do not act on this</summary>

## Second blocker — OAuth-required gateway (found 2026-09-15, same day, on retry) — DISPROVEN, see Correction above

After the API-version fix was deployed, the user retried Connect and it **still failed**, now surfacing a genuine `401` from the real Jira endpoint (not the earlier 302 misrouting) — confirmed via [SOL-012](./solutions/SOL-012-git-gateway-service-cause-logging.md)'s logging extended to `issue-tracking-service` in this same pass:
```
"cause":"jira: whoami: unexpected status 401: <html>...<title>Unauthorized (401)</title>..."
```

Direct `curl` against the real site confirms this is **not a credential problem**:
```
curl -sD - https://jr.servicehub.vn/rest/api/2/myself
→ HTTP/1.1 401
  WWW-Authenticate: OAuth realm="https://jr.servicehub.vn"
  Server: VNPAY Server

curl -s -o /dev/null -w "%{http_code}" https://jr.servicehub.vn/rest/api/2/serverInfo
→ 401   (even the "public" serverInfo probe this fix relies on is gated)
```

`jr.servicehub.vn` sits behind a **mandatory OAuth-enforcing gateway** (`Server: VNPAY Server` — a VNPay-internal API gateway/SSO layer in front of the real Jira, not Jira's own auth). It rejects **every** request to `/rest/api/*` — including `serverInfo`, which this fix's own version-probe depends on — with `401` + `WWW-Authenticate: OAuth`, regardless of whether a Basic, Bearer, valid, or invalid credential is sent. Confirmed by testing with no auth, a deliberately invalid Bearer token, and (per the user's own retry) a real PAT — all return the identical `401`/`WWW-Authenticate: OAuth` response.

### Why this invalidates the "Bearer PAT" fix direction

The API-version detection fix is still correct and should stay (it fixed a real, separate bug — hitting the wrong API version entirely). But `authHeaderValue`'s Basic-vs-Bearer choice assumed Jira's own native auth (Cloud API token or Server/Data Center PAT) would be reachable at all. Against a site gated by a mandatory OAuth reverse proxy, **neither Basic nor Bearer-PAT can ever succeed** — the gateway itself, not Jira, is what's rejecting every request before it reaches Jira's own auth logic.

### What a real fix needs (not done — needs product/infra input first)

1. Confirm with VNPay's infra/security team **whether this OAuth gateway is optional-bypassable** for a service account (e.g., a direct internal URL/port that skips the gateway, or an IP-allowlist exception) — if so, connecting to that alternate address may work with today's code unchanged.
2. If the gateway is mandatory for all access, `issue-tracking-service`'s Jira adapter needs a **real OAuth 2.0 client** (authorization code grant with user-facing redirect/consent, token storage, refresh-token rotation) — a materially larger feature than this bug's original scope (which was "fix a wrong API version + auth header shape"). This is not a code-only fix Claude can safely design and ship without product input on: which OAuth flow variant the gateway implements (3-legged? client-credentials for a service account?), where the redirect URI / client registration lives, and how per-user vs per-tenant token storage should work given `Credential`'s current per-connection shape.

**(All of the above was wrong — see "Correction" above. A real PAT via Bearer works fine; no gateway OAuth requirement actually blocks this.)**

</details>

## Confirmed working (2026-09-15) — verified with the user's real PAT

```
curl -H "Authorization: Bearer <real PAT>" https://jr.servicehub.vn/rest/api/2/myself
→ 200 OK, {"key":"JIRAUSER12311","name":"luatnc","emailAddress":"luatnc@vnpay.vn",...}

curl -H "Authorization: Bearer <real PAT>" https://jr.servicehub.vn/rest/api/2/serverInfo
→ 200 OK, {"deploymentType":"Server","version":"10.3.1","serverTitle":"VNPAY Jira",...}
```

Both the API-version probe and Bearer-PAT auth work exactly as designed. **User-facing instruction**: when connecting, leave the **Email field blank** and put the PAT in **Token** — filling in Email makes the code choose Basic auth (rejected by this site) instead of Bearer (works).

## Related

- [`missing-v1`/BUG-015](../missing-v1/BUG-015-jira-channels-not-implemented.md) / [SOL-015](../missing-v1/solutions/SOL-015-jira-channels.md) — where this Jira integration was originally built; that work correctly implemented the Cloud contract it was scoped to, but never extended to Server/Data Center — this bug is a scope gap, not a regression of that original work.
- Same "generic wrapper error hides the real, diagnosable cause" shape as [BUG-009](./BUG-009-infra-agent-exec-failed-generic-relay-error.md) and [BUG-012](./BUG-012-gitgateway-status-failed-opaque-relay-error.md) — though here the underlying HTTP-level evidence was cheap to get directly (`curl` against the real site), so no observability-gap fix (à la SOL-009/SOL-012) was needed to find it.
