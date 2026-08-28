# TASK-INT-02-02: Wire `credentials.set/get/delete/list` onto `credential-broker-service`

**From Solution:** SOL-INT-02
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_credentials.go` (new), `channels.go`, `internal/domain/registry.go`
**Depends on:** TASK-INT-02-01
**Status:** `[ ]` BLOCKED — hard channel-name collision with a pre-existing, already-shipped `credentials.*` implementation this task's authors did not know about. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_credentials.go` already exists (TASK-042) and already registers `credentials.set`/`credentials.revoke`/`credentials.status`/`credentials.list` for the exact same 5 services (bitbucket/azure-devops/gitea/linear/jira), backed by scm-integration-service's/issue-tracking-service's `SetIntegrationCredential`/`RevokeAuth`/`GetIntegrationCredentialStatus`/`ListIntegrationCredentials` RPCs — a different backend than credential-broker-service. Two blockers: (1) this task's file (`channels_credentials.go`) and function name (`registerCredentialsChannels`) both already exist with a different signature — a straight copy-paste is a duplicate-symbol compile error; (2) even renamed, `credentials.set`/`credentials.list` are channel names `Registry.Register` would silently overwrite (last-registered wins, see channels.go's registerGitDeepChannels-vs-registerGitChannels comment for the same mechanism used deliberately elsewhere) — redirecting them to credential-broker-service would regress the already-working TASK-042 integration and silently drop its `{success}` response-field contract (`runtime-credentials-client.ts` reads `success`, not this task's planned `{ok}`) for every existing bitbucket/gitea/azure-devops/jira/linear user. Not resolved unilaterally here: needs a naming decision (e.g. new channel names for the credential-broker-service-backed path) or an explicit decision to retire/migrate TASK-042's SCM/issue-tracking-service-backed path first. TASK-INT-02-01's dial + signature threading landed regardless — `credentialBrokerClient` is available and unused, ready for whichever direction is chosen.

---

## Context

`credential-broker-service`'s RPC surface already covers this namespace's
needs (`WriteCredential`, `GetCredentialMetadataByOwner`,
`RevokeCredentialByOwner`, `ListCredentialsByCategory`) — this is four
`Register` calls, not new service-side work. Uses a new
`owner_id = "<userID>:<service>"` convention (distinct from
`scm-integration-service`'s existing bare-provider-name convention, and
non-colliding since it's a different `CredentialCategory`) so BL-INT-02's
5 per-user pasted-token services (bitbucket, azure-devops, gitea, linear,
jira) don't collide with or leak across tenants/users.

## Changes to make

Create `backend-go/services/api-gateway/internal/adapter/wscompat/channels_credentials.go`:

```go
// Package wscompat — credentials.set/get/delete/list channels.
//
// Relay to credential-broker-service's existing gRPC API — see SOL-INT-02.
// No new service-side work. owner_id for this namespace is
// "<userID>:<service>" (see ownerID below) — a convention this file
// introduces, distinct from scm-integration-service's existing bare
// provider-name owner_id, and non-colliding since it's a different
// CredentialCategory.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

var credentialServiceCategory = map[string]credentialbrokerv1.CredentialCategory{
	"bitbucket":    credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
	"azure-devops": credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
	"gitea":        credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
	"linear":       credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
	"jira":         credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
}

func credentialOwnerID(userID, service string) string { return userID + ":" + service }

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
		// No transport-layer envelope to decrypt — pasted directly over the
		// already-TLS-terminated api-gateway connection; sent as plaintext
		// bytes over the mTLS-secured internal mesh, same carve-out
		// WriteCredentialRequest's doc comment describes for
		// SERVICE_SECRET/VAPID_KEY.
		_, err = client.WriteCredential(rpcCtx, &credentialbrokerv1.WriteCredentialRequest{
			TenantId: id.TenantID, OwnerId: credentialOwnerID(id.UserID, in.Service),
			Category: cat, EncryptedEnvelope: []byte(in.Token),
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("credentials.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// Metadata-only — api-gateway never forwards plaintext secrets to a
		// browser-facing channel (api-gateway.md §9: "No secrets of its
		// own"). ResolveCredentialByOwner (which DOES return plaintext) is
		// deliberately NOT used here.
		type getArgs struct {
			Service string `json:"service"`
		}
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
			TenantId: id.TenantID, Category: cat, OwnerId: credentialOwnerID(id.UserID, in.Service),
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
		type deleteArgs struct {
			Service string `json:"service"`
		}
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
			TenantId: id.TenantID, Category: cat, OwnerId: credentialOwnerID(id.UserID, in.Service),
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("credentials.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// Two RPC calls (one per category) merged client-side — the RPC is
		// shaped per-category, this namespace spans two.
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
			prefix := id.UserID + ":"
			for _, m := range resp.GetCredentials() {
				if svc, ok := strings.CutPrefix(m.GetOwnerId(), prefix); ok {
					services = append(services, svc)
				}
			}
		}
		return map[string]any{"services": services}, nil
	})
}
```

Add `registerCredentialsChannels(r, credentialBrokerClient)` to
`RegisterRealChannels`'s body in `channels.go` (final integration pass
block).

In `backend-go/services/api-gateway/internal/domain/registry.go`, update
the now-stale comment (`:79-81`):

```go
// credential-broker-service has no REST RoutingRule — this stays a
// wscompat-only surface (credentials.set/get/delete/list, SOL-INT-02),
// matching how this namespace was always a frontend RPC-style call, not a
// REST resource. It IS reached directly from api-gateway now, via
// RegisterRealChannels' credentialBrokerClient — see channels_credentials.go.
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/...
```

Expected: clean build/tests. Add
`wscompat/channels_credentials_test.go` per SOL-INT-02's test plan: one
test per channel using a fake `CredentialBrokerServiceClient`;
`credentials.set` for an unknown service string fails fast with no RPC
call; `credentials.get` on a credential that doesn't exist yet returns
`{configured:false}`, not an error, and the response never contains a
`value`/plaintext field (regression guard against accidentally switching to
`ResolveCredentialByOwner`); `credentials.list` with two services in
different categories both appear in one merged response, and a credential
belonging to a *different* `userID` in the same tenant is correctly
excluded (owner-prefix filter proof).
