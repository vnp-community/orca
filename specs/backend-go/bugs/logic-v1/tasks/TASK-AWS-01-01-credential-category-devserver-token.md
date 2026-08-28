# TASK-AWS-01-01: Add `CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN` to `credentialbroker.proto`

**From Solution:** SOL-AWS-01
**Priority:** P0 — TASK-AWS-01-02 and TASK-AWS-03-05 both depend on this
**Service:** `credential-broker-service` (proto shared with `infra-fleet-service`)
**File:** `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

None of `CredentialCategory`'s existing 5 values fit a relay-websocket
agent bearer token — `SSH` is semantically SSH cert/key material, not a
bespoke WS bearer token, and reusing it would make
`ResolveCredential`'s category-to-`VaultEngine` branching incoherent. This
adds a sixth category, mapped to Vault KV v2 (static, versioned secret) —
see SOL-AWS-01's rationale section for why this is the closest existing
engine mapping.

## Changes to make

In `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto`,
extend the enum:

```protobuf
enum CredentialCategory {
  CREDENTIAL_CATEGORY_UNSPECIFIED = 0;
  CREDENTIAL_CATEGORY_SCM_OAUTH = 1;
  CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH = 2;
  CREDENTIAL_CATEGORY_AI_PROVIDER_KEY = 3;
  CREDENTIAL_CATEGORY_SSH = 4;
  CREDENTIAL_CATEGORY_SERVICE_SECRET = 5;
  // CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN is a relay-websocket
  // Authorization: Bearer token infra-fleet-service presents outbound to a
  // dev server's agent-connection-relay.ts WebSocket server — see
  // specs/backend-go/bugs/logic-v1/solutions/SOL-AWS-01-relay-websocket-per-devserver-token.md.
  // Mapped to Vault KV v2 (static, versioned secret) — the closest existing
  // category by shape, not something Vault signs fresh per connection.
  CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN = 6;
}
```

In `backend-go/services/credential-broker-service/internal/domain/credential_metadata.go`,
add the mirrored domain constant and register it in `Valid()` (`Engine()`
needs no change — it already defaults every non-`CategoryAiProviderKey`
category to `VaultEngineKV2`, which is the correct mapping for this new
category too):

```go
const (
	CategoryScmOAuth          Category = "scm_oauth"
	CategoryIssueTrackerOAuth Category = "issue_tracker_oauth"
	CategoryAiProviderKey     Category = "ai_provider_key"
	CategorySsh               Category = "ssh"
	CategoryServiceSecret     Category = "service_secret"
	// CategoryDevServerAgentToken mirrors
	// CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN — see SOL-AWS-01.
	CategoryDevServerAgentToken Category = "dev_server_agent_token"
)

func (c Category) Valid() bool {
	switch c {
	case CategoryScmOAuth, CategoryIssueTrackerOAuth, CategoryAiProviderKey, CategorySsh, CategoryServiceSecret, CategoryDevServerAgentToken:
		return true
	default:
		return false
	}
}
```

In `backend-go/services/credential-broker-service/internal/adapter/grpc/server.go`,
add the new category to both `toDomainCategory` and `toProtoCategory`
(around `:218-246`):

```go
// toDomainCategory
case credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN:
	return domain.CategoryDevServerAgentToken

// toProtoCategory
case domain.CategoryDevServerAgentToken:
	return credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./...
```

Expected: clean build; `buf breaking` reports only an enum-value addition
(never remove/renumber an existing value).
