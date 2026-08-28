# SOL-015: Build `jira.*` on `issue-tracking-service`'s already-sketched RPC surface

**Resolves:** [BUG-015](../BUG-015-jira-channels-not-implemented.md)
**Service:** `issue-tracking-service` (proto + usecase extension) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/issuetracking/v1/issuetracking.proto`
- `backend-go/services/issue-tracking-service/internal/domain/*.go` (richer `Issue`/`Project`/`WorkflowState`/`IssueComment`/`ConnectionStatus` types)
- `backend-go/services/issue-tracking-service/internal/usecase/ports.go` (extend `IssueTrackerProvider`, `ConnectionRepository`)
- `backend-go/services/issue-tracking-service/internal/usecase/*.go` (new use cases, one per RPC)
- `backend-go/services/issue-tracking-service/internal/adapter/postgres/*.go` (`issuetracking_connections` repository)
- `backend-go/services/issue-tracking-service/internal/adapter/external/jira/client.go` (extend beyond `ListIssues`/`CreateIssue`)
- `backend-go/services/issue-tracking-service/internal/adapter/credential/client.go` (real `credential-broker-service` client, replacing the local-dev env-var stub)
- `backend-go/services/issue-tracking-service/migrations/*.sql` (extend `issuetracking_connections`)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/issue_tracking_routes.go` (extend `/v1/issues` for the new fields; `LinkIssue` → `LinkIssueToTask` rename)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_jira.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_jira_test.go` (new file)
- `backend-go/services/api-gateway/cmd/server/main.go` (pass `issueTrackingClient` into `RegisterRealChannels`)
**Status:** ✅ Implemented — all 6 task(s) (TASK-096–101) DONE; see each task file's own Status/Verify section for evidence.

---

## The RPC surface is already designed — this is a gap-closing task, not a new design

`specs/backend-go/tdd/services/issue-tracking-service.md` §3 already sketches a
provider-agnostic `IssueTrackingService` with 21 RPCs covering connection
management, issue querying/mutation, comments, and project/workflow
metadata — `provider` (`JIRA`/`LINEAR`) is a field on shared requests, not a
per-provider service split. What actually shipped
(`backend-go/proto/orca/issuetracking/v1/issuetracking.proto:9-13`) is 3 of
those 21: `ListIssues`, `CreateIssue`, `LinkIssue` (renamed from the TDD's
`LinkIssueToTask`), each with a thin generic `Issue{id, title, state, url}`
message. BUG-015 found this gap independently from the WS-channel side; the
TDD confirms the target design already answers "what should these RPCs look
like," just not "have they been built yet."

Mapping BUG-015's 19 `jira.*` methods onto the TDD's already-sketched RPCs:

| `jira.*` method | TDD RPC (`issue-tracking-service.md` §3) | Notes |
|---|---|---|
| `status` | `GetConnectionStatus` | |
| `connect` | `Connect` | |
| `disconnect` | `Disconnect` | |
| `selectSite` | `SelectWorkspace` | TDD's own comment: `// Jira "site" / Linear "workspace"` |
| `testConnection` | `TestConnection` | |
| `searchIssues` | `SearchIssues` | |
| `listIssues` | `ListIssues` | partial backing today — thin `Issue`, no `filter`/`limit`/`siteId` |
| `getIssue` | `GetIssue` | |
| `createIssue` | `CreateIssue` | partial backing today — no `issueType`/`assignee`/`priority`/custom fields |
| `updateIssue` | `UpdateIssue` | |
| `addIssueComment` | `AddIssueComment` | |
| `issueComments` | `ListIssueComments` | |
| `listProjects` | `ListProjects` | |
| `listIssueTypes` | `ListIssueTypes` | TDD marks this "Jira only" |
| `listCreateFields` | `ListCreateFields` | |
| `listAssignableUsers` | `ListAssignableUsers` | |
| `listPriorities` | *(none)* | **scope addition beyond the TDD** — see below |
| `listTransitions` | *(none — `TransitionIssue` is the mutation, not a list)* | **scope addition beyond the TDD** |
| `getProjectStatusOrder` | *(none)* | **scope addition beyond the TDD** |

Three methods have no TDD-sketched RPC at all: `listPriorities`,
`listTransitions` (the TDD has `TransitionIssue` — the *mutation* — but no
read-only "what transitions are available from this issue's current
status" RPC), and `getProjectStatusOrder` (Jira's per-project Kanban column
ordering — a Jira-specific concept the TDD's `WorkflowState`/`ListWorkflowStates`
pair doesn't cover: column *grouping*, not just per-state category). Flag
these as scope additions to propose alongside the RPC list, the same way
SOL-001 flagged `GetAdminStats` — not something to silently skip because
the TDD doesn't already name it.

`issue-tracking-service/internal/adapter/external/jira/client.go`'s own
`CreateIssue` doc comment already anticipates this exact gap: `// TODO:
thread a caller-requested issue-type name through CreateIssueRequest once
the design doc's ListIssueTypes RPC (§3) is exposed over gRPC.` — written
before this task, independent confirmation the target shape was already
known.

---

## Design — Proto additions (`issuetracking.proto`)

Additive only (new RPCs, new optional fields on existing messages) — no
existing field is removed or renumbered, so this passes `buf breaking`
cleanly per `08-inter-service-communication.md`'s gRPC conventions. `Issue`
grows from `{id, title, state, url}` into the richer provider-agnostic
shape `issue-tracking-service.md` §4 describes (title/description
normalized to Markdown, workflow state, assignee, labels, provider URL, raw
provider ID for round-tripping mutations); Jira-only concerns (issue type,
custom fields, status category/column) ride along as fields that are simply
empty for Linear, rather than a second parallel message — matches §4's "the
same adapter-per-provider pattern `scm-integration-service` uses" framing.

```protobuf
service IssueTrackingService {
  // ── existing, kept for compatibility ──────────────────────────────
  rpc ListIssues(ListIssuesRequest) returns (ListIssuesResponse);   // fields extended below
  rpc CreateIssue(CreateIssueRequest) returns (CreateIssueResponse); // fields extended below
  rpc LinkIssue(LinkIssueRequest) returns (LinkIssueResponse);       // == TDD's LinkIssueToTask

  // ── connection mgmt (new) ──────────────────────────────────────────
  rpc Connect(ConnectRequest) returns (ConnectionStatus);
  rpc Disconnect(DisconnectRequest) returns (google.protobuf.Empty);
  rpc SelectWorkspace(SelectWorkspaceRequest) returns (ConnectionStatus);
  rpc GetConnectionStatus(GetConnectionStatusRequest) returns (ConnectionStatus);
  rpc TestConnection(TestConnectionRequest) returns (TestConnectionResult);

  // ── issue querying/mutation beyond ListIssues/CreateIssue (new) ────
  rpc SearchIssues(SearchIssuesRequest) returns (SearchIssuesResponse);
  rpc GetIssue(GetIssueRequest) returns (Issue);
  rpc UpdateIssue(UpdateIssueRequest) returns (Issue);
  rpc AddIssueComment(AddIssueCommentRequest) returns (IssueComment);
  rpc ListIssueComments(ListIssueCommentsRequest) returns (ListIssueCommentsResponse);

  // ── project/workflow metadata (new) ─────────────────────────────────
  rpc ListProjects(ListProjectsRequest) returns (ListProjectsResponse);
  rpc ListIssueTypes(ListIssueTypesRequest) returns (ListIssueTypesResponse); // Jira only
  rpc ListCreateFields(ListCreateFieldsRequest) returns (ListCreateFieldsResponse);
  rpc ListAssignableUsers(ListAssignableUsersRequest) returns (ListAssignableUsersResponse);

  // ── scope additions beyond the TDD (flagged, not silently skipped) ──
  rpc ListPriorities(ListPrioritiesRequest) returns (ListPrioritiesResponse);
  rpc ListTransitions(ListTransitionsRequest) returns (ListTransitionsResponse);
  rpc GetProjectStatusOrder(GetProjectStatusOrderRequest) returns (GetProjectStatusOrderResponse);
}

message Workspace { // "site" for Jira, "workspace" for Linear — TDD's own unification
  string id = 1;
  string name = 2;
  string url = 3; // Jira site base URL; empty for Linear
}

message ConnectionStatus {
  bool connected = 1;
  string viewer_id = 2;
  string viewer_display_name = 3;
  string viewer_email = 4;
  repeated Workspace workspaces = 5;
  string active_workspace_id = 6;
  string selected_workspace_id = 7; // "" | "all" | a specific id — see JiraSiteSelection
  string credential_error = 8;      // set when a stored credential exists but resolution/decrypt failed
}

message Issue {
  string id = 1;               // provider-agnostic id (was already here)
  string provider_issue_id = 2; // raw provider id/key, for round-tripping mutations
  string key = 3;               // "PROJ-123" (Jira) / "ENG-42" (Linear identifier)
  string title = 4;
  string description_markdown = 5;
  string state = 6;             // kept for back-compat with existing callers
  WorkflowState workflow_state = 7;
  string url = 8;
  Project project = 9;
  IssueType issue_type = 10;    // Jira only; unset for Linear
  repeated string labels = 11;
  UserRef assignee = 12;
  UserRef reporter = 13;        // Jira only
  Priority priority = 14;
  string custom_fields_json = 15; // JSON-encoded map — Jira create/edit custom fields
  google.protobuf.Timestamp created_at = 16;
  google.protobuf.Timestamp updated_at = 17;
}

message Project { string id = 1; string key = 2; string name = 3; string workspace_id = 4; }
message IssueType { string id = 1; string name = 2; bool subtask = 3; }
message WorkflowState { string id = 1; string name = 2; string category = 3; } // todo|in_progress|done|cancelled
message UserRef { string id = 1; string display_name = 2; string email = 3; string avatar_url = 4; }
message Priority { string id = 1; string name = 2; }
message IssueComment { string id = 1; string body_markdown = 2; UserRef author = 3; google.protobuf.Timestamp created_at = 4; google.protobuf.Timestamp updated_at = 5; }

message ListIssuesRequest {
  string tenant_id = 1;
  IssueProvider provider = 2;
  string project_key = 3;
  // New, additive:
  string filter_json = 4; // JSON-encoded JiraIssueFilter-shaped object
  int32 limit = 5;
  string workspace_id = 6; // site/workspace selector
}

message CreateIssueRequest {
  string tenant_id = 1;
  IssueProvider provider = 2;
  string project_key = 3;
  string title = 4;
  string description = 5;
  // New, additive:
  string issue_type_id = 6;   // Jira
  string assignee_id = 7;
  string priority_id = 8;
  repeated string label_ids = 9;
  string parent_issue_id = 10; // Linear sub-issue
  string custom_fields_json = 11; // arbitrary Jira create-field bag, keyed by field key
  string workspace_id = 12;
}
```

`buf breaking` note: `ListIssuesRequest.filter_json`/`limit`/`workspace_id`
and `CreateIssueRequest`'s new fields are appended at unused field numbers —
no renumbering of `tenant_id`/`provider`/`project_key`/`title`/
`description`, so existing callers (the `/v1/issues` REST proxy) keep
compiling unmodified until they're updated to pass the new fields.

---

## Design — Credential model: per-tenant-per-user, not per-tenant-per-provider

`06-secrets-vault-architecture.md`'s "Integration OAuth tokens" row and
`issue-tracking-service.md` §9 both specify Vault KV v2 credentials, "one
path per `(tenant, service, user)`," mediated by `credential-broker-service`
— explicitly the successor to the old TS `WebCredentialStore` per-user
encrypted files, **not** a return to local encrypted files (BUG-015's own
note; this solution does not reintroduce file-based storage anywhere).

There's a real nuance worth surfacing rather than silently picking a side:
`credentialbroker.proto`'s `ResolveCredentialByOwner` doc comment
(`credentialbroker.proto:30-41`) names `issue-tracking-service` as a caller
with `owner_id = provider name, e.g. "jira"` — i.e. **one credential per
tenant per provider**, keyed by service name, not by user. That contradicts
`issue-tracking-service.md` §5's own `issuetracking_connections` table,
which is `UNIQUE (tenant_id, user_id, provider)` — **one credential per
tenant per user per provider**, matching Jira/Linear's actual per-user API
token model (§9: "TS already scopes credentials per-user... Go keeps that
isolation exactly"). The per-user table is the authoritative design (§9 is
explicit and specific about preserving per-user isolation; the broker
proto's comment reads as a simplified example, not a hard constraint — nothing
in `credentialbroker.proto` prevents a composite `owner_id`). Propose
resolving the tension by using a **composite `owner_id`**, `"<user_id>:jira"`
(or `"<user_id>:jira:<site_id>"` once multi-site credentials are needed —
see below), rather than the bare `"jira"` the doc comment's example implies.
Flag this reconciliation explicitly in the `credentialbroker.proto` doc
comment when this solution lands, so the next reader doesn't hit the same
ambiguity.

**Multi-site schema note.** `JiraConnectionStatus` carries `sites?:
JiraSite[]` (`frontend/src/shared/jira-types.ts:15-24`) — a user may be
connected to more than one Jira Cloud site under one Atlassian identity.
`issuetracking_connections`'s current `UNIQUE (tenant_id, user_id,
provider)` with a single `external_site_id` column only models **one**
active site per user, not a list. Propose widening the uniqueness to
`UNIQUE (tenant_id, user_id, provider, external_site_id)` (one row per
connected site, each with its own `credential_id`) plus a small
`is_selected BOOLEAN` column recording which row `selectSite` most recently
chose — `GetConnectionStatus` then returns `workspaces` as every row for
`(tenant_id, user_id, provider)` and `selected_workspace_id` as the
`is_selected` row's `external_site_id`. Add a `credential_id UUID` column
(the id `WriteCredential` returned) so `ResolveCredential(credential_id)` —
not `ResolveCredentialByOwner` — is the per-request read path once a
connection row exists; `ResolveCredentialByOwner` is only needed for the
one-time bootstrap "does *some* Jira credential already exist for this
composite owner_id" check `Connect` needs to decide between create-new vs.
already-connected.

```sql
-- migration: widen issuetracking_connections for multi-site + credential pointer
ALTER TABLE issuetracking_connections
  DROP CONSTRAINT issuetracking_connections_tenant_id_user_id_provider_key,
  ADD CONSTRAINT issuetracking_connections_site_key
    UNIQUE (tenant_id, user_id, provider, external_site_id),
  ADD COLUMN credential_id UUID NOT NULL,
  ADD COLUMN is_selected BOOLEAN NOT NULL DEFAULT true;
```

---

## Design — `usecase/` layer

One usecase per RPC per `03-clean-architecture-guidelines.md` ("mirrors the
granularity of today's RPC methods"); grouped here by shared pattern rather
than 19 individual sketches.

### Connection/auth group (`Connect`/`Disconnect`/`SelectWorkspace`/`GetConnectionStatus`/`TestConnection`)

```go
// internal/usecase/connect.go
type ConnectInput struct {
    TenantID, UserID string
    Provider         domain.Provider
    SiteURL, Email, Token string // Jira; Token only for Linear
}

func (uc *Connect) Execute(ctx context.Context, in ConnectInput) (domain.ConnectionStatus, error) {
    provider, err := uc.registry.Resolve(in.Provider)
    if err != nil {
        return domain.ConnectionStatus{}, err
    }
    cred := usecase.Credential{BaseURL: in.SiteURL, Email: in.Email, Token: in.Token}
    // Verify before persisting anything — an invalid token must not create
    // a "connected" row a later call then fails against.
    viewer, err := provider.Whoami(ctx, cred)
    if err != nil {
        return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_AUTH_FAILED", "could not authenticate", err)
    }
    ownerID := fmt.Sprintf("%s:%s", in.UserID, in.Provider) // see credential-model note above
    credID, err := uc.credentialWriter.Write(ctx, in.TenantID, ownerID, credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH, cred)
    if err != nil {
        return domain.ConnectionStatus{}, err
    }
    return uc.connections.Upsert(ctx, in.TenantID, in.UserID, in.Provider, viewer, credID)
}
```

### Issue querying/mutation group (`SearchIssues`/`ListIssues`/`GetIssue`/`CreateIssue`/`UpdateIssue`)

```go
// internal/usecase/create_issue.go
func (uc *CreateIssue) Execute(ctx context.Context, in CreateIssueInput) (domain.Issue, error) {
    cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.UserID, in.Provider, in.WorkspaceID)
    if err != nil {
        return domain.Issue{}, err // ISSUETRACKING_NOT_CONNECTED, mapped at grpc boundary
    }
    provider, err := uc.registry.Resolve(in.Provider)
    if err != nil {
        return domain.Issue{}, err
    }
    return provider.CreateIssue(ctx, cred, domain.NewIssue{
        ProjectKey: in.ProjectKey, Title: in.Title, Description: in.Description,
        IssueTypeID: in.IssueTypeID, AssigneeID: in.AssigneeID, PriorityID: in.PriorityID,
        LabelIDs: in.LabelIDs, ParentIssueID: in.ParentIssueID, CustomFields: in.CustomFields,
    })
}
```

`IssueTrackerProvider` (`ports.go`) grows from its current 2 methods
(`ListIssues`, `CreateIssue`) to one method per usecase in the mapping
table — `jira/client.go` and `linear/client.go` each implement the full
port; `CreateIssue`'s existing `TODO` comment (cited above) is the concrete
signal this was anticipated, not new scope invented here.

### Comments group (`AddIssueComment`/`ListIssueComments`)

Same shape as issue querying/mutation — `provider.AddComment`/
`provider.ListComments`, credential-resolve-then-call. No new pattern.

### Metadata lookups group (`ListProjects`/`ListIssueTypes`/`ListCreateFields`/`ListAssignableUsers`/`ListPriorities`/`ListTransitions`/`GetProjectStatusOrder`)

```go
// internal/usecase/list_create_fields.go — representative of the group;
// the Jira-only concept `jira/client.go`'s CreateIssue TODO was written
// against.
func (uc *ListCreateFields) Execute(ctx context.Context, in ListCreateFieldsInput) ([]domain.CreateField, error) {
    cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.UserID, domain.ProviderJira, in.WorkspaceID)
    if err != nil {
        return nil, err
    }
    provider, err := uc.registry.Resolve(domain.ProviderJira)
    if err != nil {
        return nil, err
    }
    return provider.ListCreateFields(ctx, cred, in.ProjectIDOrKey, in.IssueTypeID)
}
```

`ListPriorities`/`ListTransitions`/`GetProjectStatusOrder` follow the
identical shape — new `IssueTrackerProvider` methods, no credential/registry
pattern change, just three more provider calls.

---

## Design — `wscompat` wiring (`channels_jira.go`)

New file, following `channels.go`'s established per-namespace
`register<Namespace>Channels(r, client)` convention (`registerDevServerChannels`,
`registerGitChannels`) and the same `rpcTimeout`-scoped, `decodeArg`-based
handler shape:

```go
package wscompat

import (
    "context"
    "encoding/json"

    issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

func registerJiraChannels(r *Registry, client issuetrackingv1.IssueTrackingServiceClient) {
    r.Register("jira.status", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetConnectionStatus(rpcCtx, &issuetrackingv1.GetConnectionStatusRequest{
            Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
        })
        if err != nil {
            return nil, err
        }
        return toJiraConnectionStatusView(resp), nil // wire-shape translation lives here, not in domain/
    })

    r.Register("jira.createIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type createArgs struct {
            ProjectKey string `json:"projectKey"`
            Title      string `json:"title"`
            IssueType  string `json:"issueType"`
            SiteID     string `json:"siteId"`
            // ... rest of JiraCreateIssueArgs
        }
        in, err := decodeArg[createArgs](args, 0)
        if err != nil {
            return nil, err
        }
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.CreateIssue(rpcCtx, &issuetrackingv1.CreateIssueRequest{
            Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA,
            ProjectKey: in.ProjectKey, Title: in.Title, IssueTypeId: in.IssueType,
            WorkspaceId: in.SiteID,
        })
        if err != nil {
            return nil, err
        }
        return toJiraIssueView(resp), nil
    })

    // jira.disconnect, .selectSite, .testConnection, .searchIssues,
    // .listIssues, .getIssue, .updateIssue, .addIssueComment,
    // .issueComments, .listProjects, .listIssueTypes, .listCreateFields,
    // .listPriorities, .listAssignableUsers, .listTransitions,
    // .getProjectStatusOrder — same decode-args -> typed RPC call ->
    // view-translate shape, one handler each.
}
```

`RegisterRealChannels` gains an `issueTrackingClient
issuetrackingv1.IssueTrackingServiceClient` parameter and a
`registerJiraChannels(r, issueTrackingClient)` call — `main.go` already
dials `issueTrackingClient` for the `/v1/issues` REST routes
(`main.go:271`), so no new dial, just threading the existing client into
`RegisterRealChannels`'s call at `main.go:241`.

`toJiraConnectionStatusView`/`toJiraIssueView` etc. translate the
provider-agnostic proto response into the exact field names
`frontend/src/shared/jira-types.ts` expects (`siteUrl` not `url`, `key` not
`provider_issue_id`, etc.) — this translation belongs at the `wscompat`
adapter boundary per `03-clean-architecture-guidelines.md`'s "adapter
translates wire format ↔ usecase" rule, not inside `issue-tracking-service`
itself (which stays provider-agnostic and namespace-agnostic — it has no
concept of "jira.*" as a wire shape, only `IssueProvider_ISSUE_PROVIDER_JIRA`
as a request field).

---

## Test plan

- `services/issue-tracking-service/internal/usecase/connect_test.go` — verifies before persisting; a failing `Whoami` never reaches `ConnectionRepository.Upsert`.
- `services/issue-tracking-service/internal/usecase/create_issue_test.go` — credential-not-found short-circuits before the provider call; custom fields pass through to `IssueTrackerProvider.CreateIssue` verbatim.
- `services/issue-tracking-service/internal/adapter/postgres/repository_test.go` (testcontainers) — multi-site upsert: connecting a second site for the same `(tenant, user, provider)` adds a row, doesn't overwrite the first; `is_selected` moves on `SelectWorkspace`.
- `services/api-gateway/internal/adapter/wscompat/channels_jira_test.go` — one test per channel mirroring `channels_test.go`'s existing `TestDevServerListChannel_Success`/`_PropagatesError` shape (fake `IssueTrackingServiceClient`, assert decoded args map onto the right request fields and the response view matches `jira-types.ts`'s field names).
- Contract test: round-trip a `CreateIssue` call through both the extended `/v1/issues` REST route and the new `jira.createIssue` WS channel against the same fake client — same request fields reach the RPC, guarding against the two surfaces drifting the way BUG-015 found `/v1/issues` and `jira.*` already had before this fix.

## References

- `specs/backend-go/bugs/missing-v1/BUG-015-jira-channels-not-implemented.md` — the gap this solution closes
- `specs/backend-go/tdd/services/issue-tracking-service.md` §3 (RPC surface), §4 (domain model), §5 (data model), §9 (credential/security notes) — the target design this solution implements
- `specs/backend-go/tdd/architecture/06-secrets-vault-architecture.md` — "Integration OAuth tokens" row; per-`(tenant,service,user)` Vault KV v2 path
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/port layering, wire-translation-at-the-adapter-boundary rule
- `backend-go/proto/orca/issuetracking/v1/issuetracking.proto:1-58` — current 3-RPC surface to extend
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto:30-41` — `ResolveCredentialByOwner`'s `owner_id` doc comment, the source of the per-provider-vs-per-user tension flagged above
- `backend-go/services/issue-tracking-service/internal/usecase/ports.go:1-77` — `IssueTrackerProvider`/`CredentialResolver` ports to extend
- `backend-go/services/issue-tracking-service/internal/adapter/jira/client.go:22-31` — `CreateIssue`'s own `TODO` anticipating `ListIssueTypes`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:221-235,390-406` — `registerGitChannels`/`registerDevServerChannels`, the pattern this solution's `registerJiraChannels` mirrors
- `backend-go/services/api-gateway/cmd/server/main.go:241,271` — existing `issueTrackingClient` dial and `RegisterRealChannels` call site to extend
- `frontend/src/shared/jira-types.ts:1-111` — wire shapes `wscompat`'s view-translation functions must match
- `frontend/src/renderer/src/runtime/runtime-jira-client.ts:51-297` — all 19 frontend call sites and their exact arg shapes
