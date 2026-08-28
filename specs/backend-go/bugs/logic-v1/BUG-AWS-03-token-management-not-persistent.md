# BUG-AWS-03: Agent tokens are neither per-DevServer, persistent, named, nor admin-UI-manageable

**Business Logic:** [BL-AWS-03](../../../../docs/logic/agent-ws/BL-AWS-03-token-management.md) — Agent Token Management
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** An admin looking for "DevServer Settings → Agent Tokens tab" to generate a named, listable, individually-revocable token for one Dev Server finds nothing — backend-go has no REST/gRPC surface, no UI, and no durable storage for that lifecycle at all. The only token-issuing endpoint that exists (`POST /api/agent-token`) is gated behind a deployment-wide admin API secret (not per-user DevServer settings), mints a single-use, in-memory-only token that vanishes on process restart, and has no name/list/revoke/lastUsedAt concept.

---

## Spec summary

BL-AWS-03 describes an admin/user-facing token lifecycle scoped to a `DevServer`: generate a named 64-char hex token (displayed once), store `{id, name, hash: SHA-256(token), createdAt, lastUsedAt}` inside that DevServer's own config record (not the database), list up to 10 tokens per DevServer with last-used timestamps, and revoke any one of them individually — revoking closes existing WS connections authenticated with that token via code 4001.

## What backend-go has

A token creation+hashing+single-use-validation mechanism exists, but for a different purpose (bootstrapping one direct-websocket connection) than the spec's persistent multi-token admin lifecycle:
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/token_endpoint.go:59-83` (`TokenIssuer.ServeHTTP`) — `POST/GET /api/agent-token`, gated by `Authorization: Bearer <ORCA_AGENT_API_SECRET>` (`:97-103`, `isAuthorized`) — a single deployment-wide admin secret, not a per-user DevServer-settings action.
- `token_endpoint.go:153-208` (`handlePost`) — mints a token (`agt-<devServerId>-<unixMilli>`, `:178`), with a TTL policy (ephemeral default 300s capped at 600s, or a 30-day "permanent" option — `:14-23,210-225`).
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/slots.go:24-42,52-84` (`Registry`) — stores only `SHA-256(token) -> devServerID` in an **in-memory map**, single-use (`Consume` at `:99-111` deletes on first successful handshake).

## What's missing

- **No persistence.** `Registry.slots` (`slots.go:28`) and `TokenIssuer.meta` (`token_endpoint.go:39`) are both plain in-process maps — nothing is written to `domain.DevServer` (which per BUG-AWS-01 has no token field at all), to Postgres, or to any config file. A process restart loses every pending token.
- **No "name" concept surfaced to storage** — `postTokenRequest.Name` (`token_endpoint.go:145,172-174`) is accepted and echoed back in the POST response but never stored in `meta`/`Registry`; `handleGet`'s listing (`:112-136`) doesn't return it at all.
- **No `lastUsedAt` tracking anywhere** — `Consume` (`slots.go:99-111`) deletes the slot outright on use; there is no field, log, or read path recording when a token was last used.
- **No per-DevServer multi-token list/revoke UI or API.** `handleGet` (`token_endpoint.go:112-136`) lists all *currently-pending, not-yet-consumed* tokens across the whole deployment (for debugging), not "the tokens registered for DevServer X." There is no `DELETE`/revoke-by-id endpoint at all — a token is either consumed (single-use, gone) or left to expire on its own TTL; nothing lets an admin explicitly kill a live token.
- **No "10 tokens max per DevServer" enforcement** — meaningless in the current design since tokens aren't tracked per-DevServer at all.
- **No close(4001) on revoke** — moot without a revoke path, but also: even the existing close paths never use code 4001 (see BUG-AWS-02).
- **relay-websocket mode has no token concept at all** beyond the single shared `ORCA_AGENT_TOKEN` (see BUG-AWS-01) — the spec's lifecycle is meant to apply uniformly to whichever mode a DevServer uses.

## See also

- [BUG-AWS-01](./BUG-AWS-01-relay-websocket-single-shared-token.md) — relay-websocket's single deployment-wide token, the other half of this same "no per-DevServer token model" gap.
- [BUG-AWS-02](./BUG-AWS-02-direct-websocket-protocol-diverges-from-spec.md) — the handshake this token gates, and its own close-code gaps.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/token_endpoint.go:1-249` — full `TokenIssuer` (POST/GET `/api/agent-token`)
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/slots.go:1-139` — full `Registry` (in-memory, single-use, SHA-256-keyed slots)
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go:53-59` — `DevServer` struct, no token field to persist against
- `docs/logic/agent-ws/BL-AWS-03-token-management.md` — spec (token lifecycle, storage shape, UI, 10-token cap)
