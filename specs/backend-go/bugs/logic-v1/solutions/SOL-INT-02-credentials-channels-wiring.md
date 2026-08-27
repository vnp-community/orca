# SOL-INT-02: Wire `credentials.set/get/delete/list` onto `credential-broker-service`'s existing RPCs

**Resolves:** [BUG-INT-02](../BUG-INT-02-credential-store-unreachable-and-different-architecture.md)
**Service:** `api-gateway` (wscompat wiring only) — no `credential-broker-service` proto/usecase changes needed
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_credentials.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_credentials_test.go` (new)
- `backend-go/services/api-gateway/internal/domain/registry.go` (doc-comment update only — see below)
- `backend-go/services/api-gateway/cmd/server/main.go` (construct + inject a `CredentialBrokerServiceClient`)
**Status:** 📋 Proposed — not yet implemented

---

## This is a thin wiring gap, as flagged, not a rearchitecture

BUG-INT-02's own "What backend-go has" section already establishes that
`credential-broker-service`'s RPC surface has grown to cover this
namespace's functional needs — `WriteCredential`, `ResolveCredentialByOwner`
/`RevokeCredentialByOwner` (both by-owner, added since BUG-007),
`GetCredentialMetadataByOwner`, and `ListCredentialsByCategory` (whose own
proto doc comment already states "backs `credentials.list`",
`credentialbroker.proto:64-66`). The blocking gap is exactly what
`registry.go:79-81`'s doc comment says it is: no client ever calls this
service through the gateway, because nothing in `wscompat` registers
`credentials.*`. Fixing that is four `Register` calls, not new
service-side work.

**Different storage mechanism is intentional, not a gap to close.**
Vault KV v2 + Postgres metadata vs. BL-INT-02's per-user AES-256-GCM
`.enc` file is exactly `06-secrets-vault-architecture.md`'s stated
purpose — collapsing "5 independent, inconsistent secret mechanisms" (of
which `WebCredentialStore` was one) into the one Vault-backed mechanism
every other credential category already uses. Building a parallel
`credentials.enc`-file store in Go, just to match BL-INT-02's storage
description literally, would be re-introducing exactly the fragmentation
`credential-broker-service` exists to remove — not attempted here.

## Design decision: `owner_id` convention for this namespace

`WriteCredentialRequest.owner_id` (`credentialbroker.proto:81`) is
documented only as "user id or service name" — ambiguous on its own.
`ResolveCredentialByOwnerRequest`'s doc comment
(`credentialbroker.proto:33-36`) establishes one existing convention:
`scm-integration-service` and `issue-tracking-service` use
`owner_id = provider name` (e.g. `"github"`) for their own OAuth-app-level
tokens — a **tenant-scoped**, not user-scoped, credential (one GitHub App
installation per tenant).

BL-INT-02's five services (bitbucket, azure-devops, gitea, linear, jira)
are a different logical resource: **per-user** pasted API tokens (the
`WebCredentialStore` design's own scoping — `~/.orca/users/<userId>/credentials.enc`).
Reusing the bare-provider-name convention would collide two unrelated
credentials under the same `(tenant, category, owner_id)` key if a
provider name ever overlapped, and would silently make personal tokens
tenant-shared, which is a real regression against the spec. This solution
proposes a distinct, explicit convention for this namespace only:

```
owner_id = "<userID>:<service>"   // e.g. "usr_abc123:bitbucket"
```

Flagged here as a **new convention this solution introduces**, not one
already established elsewhere — `scm-integration-service`'s existing
`owner_id="github"` usage is untouched and non-colliding (different
`CredentialCategory`, see below).

`service` → `CredentialCategory` mapping, using the two categories that
already exist rather than adding new ones (unlike SOL-AWS-01, no proto
enum extension needed — these five services are conceptually either SCM
or issue-tracker OAuth-shaped credentials even though BL-INT-02's tokens
happen to be pasted rather than OAuth-redirected):

| `service` | `CredentialCategory` |
|---|---|
| `bitbucket`, `azure-devops`, `gitea` | `CREDENTIAL_CATEGORY_SCM_OAUTH` |
| `linear`, `jira` | `CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH` |

## Design — wscompat channels

```go
// channels_credentials.go
//
// credentials.set/get/delete/list relay to credential-broker-service's
// existing gRPC API — see SOL-INT-02. No new service-side work; this file
// is the missing link BUG-007/BUG-INT-02 both identified.
package wscompat

var credentialServiceCategory = map[string]credentialbrokerv1.CredentialCategory{
    "bitbucket":    credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
    "azure-devops": credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
    "gitea":        credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
    "linear":       credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
    "jira":         credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
}

func ownerID(userID, service string) string { return userID + ":" + service }

func registerCredentialsChannels(r *Registry, client credentialbrokerv1.CredentialBrokerServiceClient) {
    r.Register("credentials.set", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type setArgs struct {
            Service string `json:"service"`
            Token   string `json:"token"`
        }
        in, err := decodeArg[setArgs](args, 0)
        if err != nil {
            return nil, err
        }
        cat, ok := credentialServiceCategory[in.Service]
        if !ok {
            return nil, fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not one of the 5 supported services", in.Service)
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        // No transport-layer envelope to decrypt here — unlike AI provider
        // keys (ADR-008), these tokens are pasted directly over the
        // already-TLS-terminated api-gateway connection; sent as plaintext
        // bytes over the mTLS-secured internal mesh to the broker, same
        // carve-out WriteCredentialRequest's own doc comment describes for
        // SERVICE_SECRET/VAPID_KEY (credentialbroker.proto:87).
        _, err = client.WriteCredential(rpcCtx, &credentialbrokerv1.WriteCredentialRequest{
            TenantId: id.TenantID, OwnerId: ownerID(id.UserID, in.Service),
            Category: cat, EncryptedEnvelope: []byte(in.Token),
        })
        return map[string]bool{"ok": err == nil}, err
    })

    r.Register("credentials.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        // Metadata-only — api-gateway never forwards plaintext secrets to a
        // browser-facing channel (api-gateway.md §9: "No secrets of its
        // own"); credentials.get answers "is this configured", not "what
        // is the value". ResolveCredentialByOwner (which DOES return
        // plaintext) is deliberately NOT used here.
        type getArgs struct{ Service string `json:"service"` }
        in, err := decodeArg[getArgs](args, 0)
        if err != nil {
            return nil, err
        }
        cat, ok := credentialServiceCategory[in.Service]
        if !ok {
            return nil, fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not one of the 5 supported services", in.Service)
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetCredentialMetadataByOwner(rpcCtx, &credentialbrokerv1.GetCredentialMetadataByOwnerRequest{
            TenantId: id.TenantID, Category: cat, OwnerId: ownerID(id.UserID, in.Service),
        })
        if err != nil {
            return nil, err
        }
        if resp.GetMetadata() == nil {
            return map[string]any{"configured": false}, nil // normal "not set up yet" case, not an error
        }
        return map[string]any{"configured": true, "status": resp.GetMetadata().GetStatus()}, nil
    })

    r.Register("credentials.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type deleteArgs struct{ Service string `json:"service"` }
        in, err := decodeArg[deleteArgs](args, 0)
        if err != nil {
            return nil, err
        }
        cat, ok := credentialServiceCategory[in.Service]
        if !ok {
            return nil, fmt.Errorf("CREDENTIALS_UNKNOWN_SERVICE: %q is not one of the 5 supported services", in.Service)
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        _, err = client.RevokeCredentialByOwner(rpcCtx, &credentialbrokerv1.RevokeCredentialByOwnerRequest{
            TenantId: id.TenantID, Category: cat, OwnerId: ownerID(id.UserID, in.Service),
        })
        return map[string]bool{"ok": err == nil}, err
    })

    r.Register("credentials.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        // Two RPC calls (one per category) merged client-side — the RPC is
        // shaped per-category (credentialbroker.proto:64-66's own doc
        // comment), this namespace spans two.
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        var services []string
        for _, cat := range []credentialbrokerv1.CredentialCategory{
            credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
            credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
        } {
            resp, err := client.ListCredentialsByCategory(rpcCtx, &credentialbrokerv1.ListCredentialsByCategoryRequest{
                TenantId: id.TenantID, Category: cat,
            })
            if err != nil {
                return nil, err
            }
            for _, m := range resp.GetCredentials() {
                // owner_id is "<userId>:<service>" — filter to this caller's
                // own tokens and recover the bare service name.
                prefix := id.UserID + ":"
                if svc, ok := strings.CutPrefix(m.GetOwnerId(), prefix); ok {
                    services = append(services, svc)
                }
            }
        }
        return map[string]any{"services": services}, nil
    })
}
```

## Design — `registry.go` doc-comment update

`registry.go:79-81`'s comment ("credential-broker-service has no direct
rule... no client calls it through this gateway directly") becomes false
once this lands — update it to note `credentials.*` as the one
`wscompat`-level exception, still with no REST `RoutingRule` (this stays a
WS-compat-only surface, matching how `credentials.*` was always a
frontend RPC-style call, not a REST resource per BL-INT-02).

## Test plan

- `wscompat/channels_credentials_test.go` — one test per channel using a
  fake `CredentialBrokerServiceClient`; `credentials.set` for an unknown
  service string fails fast with no RPC call; `credentials.get` on a
  credential that doesn't exist yet returns `{configured:false}`, not an
  error.
- `credentials.get` test asserting the response never contains a `value`/
  plaintext field, structurally (regression guard against accidentally
  switching to `ResolveCredentialByOwner`).
- `credentials.list` test with two services in different categories both
  appearing in one merged response, and a credential belonging to a
  *different* `userID` in the same tenant correctly excluded (owner-prefix
  filter proof).

## References

- `backend-go/services/api-gateway/internal/domain/registry.go:79-81` — the doc comment this solution's landing makes stale
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:33-45,64-66,79-96,151-162` — `ResolveCredentialByOwner`'s owner-name convention (existing), `ListCredentialsByCategory`'s doc comment, `WriteCredentialRequest`'s envelope/mesh carve-out
- `specs/backend-go/tdd/services/api-gateway.md:284-316` (§9, "No secrets of its own" — grounds `credentials.get`'s metadata-only design)
- `specs/backend-go/tdd/services/credential-broker-service.md:369-378` (§7 dependency table — confirms api-gateway reaching this service is new, not previously modeled as "indirect only")
- `specs/backend-go/bugs/missing-v1/BUG-007-credentials-channels-not-implemented.md` — prior, more detailed audit this solution's gap analysis builds on
