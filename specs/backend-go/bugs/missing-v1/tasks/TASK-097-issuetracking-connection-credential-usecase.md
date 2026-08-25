# TASK-097: Connection/credential usecase group (`Connect`/`Disconnect`/`SelectWorkspace`/`GetConnectionStatus`/`TestConnection`)

**From Solution:** SOL-015
**Priority:** P0
**Service:** `issue-tracking-service`
**File:** `services/issue-tracking-service/internal/{domain,usecase,adapter/postgres,adapter/credential,adapter/jira,adapter/grpc}/*.go`, `migrations/0002_connections.{up,down}.sql`
**Depends on:** TASK-096
**Status:** `[ ]` TODO

---

## Context

SOL-015 resolves the credential model as **per-tenant-per-user-per-provider**
(`issuetracking_connections` is `UNIQUE (tenant_id, user_id, provider)`
today — this task widens it to `UNIQUE (tenant_id, user_id, provider,
external_workspace_id)` for multi-site support), using a **composite
`owner_id`** `"<user_id>:<provider>"` when writing to
`credential-broker-service`, and a `credential_id` pointer column so the
per-request read path is `ResolveCredential(credential_id)`, not
`ResolveCredentialByOwner` (that stays reserved for `Connect`'s one-time
"does a credential already exist" bootstrap check). See SOL-015's
"Design — Credential model" section for the full rationale — this task
implements it.

This group is **provider-agnostic** — SOL-016 (TASK-103) reuses every
usecase/port here unchanged for `linear.*`, only adding `linear/client.go`'s
`Whoami` implementation and registering it. Do not add any Jira-specific
branching into these usecases beyond what's already parameterized by
`domain.Provider`.

## Changes to make

### 1. `internal/domain/connection.go` (new)

```go
package domain

import "time"

// Viewer is who a Connect/TestConnection call authenticated as — the
// provider account backing a connection.
type Viewer struct {
	ID          string
	DisplayName string
	Email       string
}

// Workspace is a Jira "site" or Linear "workspace" — the provider-agnostic
// name issue-tracking-service.md §3 unifies both under.
type Workspace struct {
	ID   string
	Name string
	URL  string // Jira site base URL; empty for Linear
}

// Connection is one connected (tenant, user, provider, workspace) row —
// issuetracking_connections' domain shape. A user may have more than one
// connected Workspace per provider (multi-site Jira); each gets its own
// row and its own CredentialID.
type Connection struct {
	TenantID       string
	UserID         string
	Provider       Provider
	Workspace      Workspace
	Viewer         Viewer
	CredentialID   string
	IsSelected     bool
	CredentialError string // set when CredentialID no longer resolves (revoked/decrypt failure)
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ConnectionStatus is GetConnectionStatus's return shape — every connected
// workspace for (tenant, user, provider), plus which one is selected.
type ConnectionStatus struct {
	Connected           bool
	Viewer              Viewer
	Workspaces          []Workspace
	ActiveWorkspaceID   string
	SelectedWorkspaceID string // "" | "all" | a specific workspace id
	CredentialError     string
}
```

### 2. `internal/usecase/ports.go` — extend `IssueTrackerProvider`, replace `CredentialResolver`, add `ConnectionRepository`

Add `Whoami` to `IssueTrackerProvider` (every provider adapter must
authenticate before anything else can happen):

```go
type IssueTrackerProvider interface {
	Whoami(ctx context.Context, cred Credential) (domain.Viewer, error)
	ListIssues(ctx context.Context, cred Credential, projectKey string) ([]domain.Issue, error)
	CreateIssue(ctx context.Context, cred Credential, projectKey, title, description string) (domain.Issue, error)
}
```

Replace `CredentialResolver` with a resolve-by-connection-key +
write-and-return-id pair, and add `ConnectionRepository`:

```go
// CredentialResolver resolves the per-(tenant,user,provider,workspace)
// credential before a provider call, and writes new credential material
// during Connect. Resolve is keyed by the same 4-tuple ConnectionRepository
// uses, not by an opaque credential_id directly — the usecase layer never
// needs to know a credential_id exists; the adapter looks it up via
// ConnectionRepository internally. See TASK-097/SOL-015's credential-model
// design.
type CredentialResolver interface {
	// Resolve looks up the connection row for
	// (tenantID, userID, provider, workspaceID) and calls
	// credential-broker-service's ResolveCredential(credential_id) — the
	// per-request read path. workspaceID may be "" to mean "the currently
	// selected workspace."
	Resolve(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (Credential, error)

	// Write encrypts and persists cred under a composite owner_id
	// ("<userID>:<provider>") via credential-broker-service.WriteCredential,
	// returning the opaque credential_id the caller must store on the
	// connection row it creates/updates.
	Write(ctx context.Context, tenantID, userID string, provider domain.Provider, cred Credential) (credentialID string, err error)

	// ExistingCredentialID checks whether a credential already exists for
	// this composite owner_id via ResolveCredentialByOwner — used only by
	// Connect's create-new-vs-already-connected bootstrap decision, never
	// the per-request read path.
	ExistingCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider) (credentialID string, found bool, err error)
}

// ConnectionRepository is the persistence port for issuetracking_connections
// — one row per connected (tenant, user, provider, workspace).
type ConnectionRepository interface {
	// Upsert inserts or updates the row for
	// (tenantID, userID, provider, workspace.ID). A first connect for a
	// (tenant,user,provider) also sets is_selected=true; connecting an
	// additional workspace under an already-connected provider defaults
	// is_selected=false (SelectWorkspace changes it explicitly).
	Upsert(ctx context.Context, tenantID, userID string, provider domain.Provider, workspace domain.Workspace, viewer domain.Viewer, credentialID string) (domain.ConnectionStatus, error)

	// Delete removes the row for workspaceID, or every row for
	// (tenantID, userID, provider) when workspaceID is "".
	Delete(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) error

	// GetStatus returns every connected workspace for
	// (tenantID, userID, provider) plus which one is_selected — the
	// GetConnectionStatus/Connect/SelectWorkspace return shape.
	GetStatus(ctx context.Context, tenantID, userID string, provider domain.Provider) (domain.ConnectionStatus, error)

	// SelectWorkspace flips is_selected to workspaceID's row (or, when
	// workspaceID == "all", clears any single selection — JiraSiteSelection
	// allows "all" as a valid value) and returns the updated status.
	SelectWorkspace(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (domain.ConnectionStatus, error)

	// GetCredentialID resolves which credential_id backs
	// (tenantID, userID, provider, workspaceID) — workspaceID == "" means
	// "the selected workspace." Returns ErrConnectionNotFound if none.
	GetCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (credentialID string, err error)
}

// ErrConnectionNotFound is returned by ConnectionRepository methods (and
// surfaced as ISSUETRACKING_NOT_CONNECTED at the usecase layer) when no
// connection row exists for the requested key.
var ErrConnectionNotFound = errors.New("usecase: no issue-tracking connection found")
```

Remove the old `CredentialResolver` interface and its "STUB PORT" doc
comment block — this task replaces the stub with a real implementation (see
`credential/client.go` below), so the warning no longer applies. Add
`"errors"` to the file's import block for `ErrConnectionNotFound`.

### 3. `internal/usecase/connect.go` (new)

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// ConnectInput mirrors ConnectRequest 1:1.
type ConnectInput struct {
	Provider domain.Provider
	SiteURL  string // Jira only
	Email    string // Jira only
	Token    string
}

// Connect verifies cred against the provider BEFORE persisting anything —
// an invalid token must not create a "connected" row a later call then
// fails against (SOL-015's own design note).
type Connect struct {
	registry    ProviderRegistry
	credentials CredentialResolver
	connections ConnectionRepository
}

func NewConnect(registry ProviderRegistry, credentials CredentialResolver, connections ConnectionRepository) *Connect {
	return &Connect{registry: registry, credentials: credentials, connections: connections}
}

func (uc *Connect) Execute(ctx context.Context, in ConnectInput) (domain.ConnectionStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if in.Provider == domain.ProviderJira && in.SiteURL == "" {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_SITE_URL", "site_url is required for jira", nil)
	}
	if in.Token == "" {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_TOKEN", "token is required", nil)
	}

	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}

	cred := Credential{BaseURL: in.SiteURL, Email: in.Email, Token: in.Token}
	viewer, err := provider.Whoami(ctx, cred)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_AUTH_FAILED", "could not authenticate with the provided credential", err)
	}

	credID, err := uc.credentials.Write(ctx, tenantID, userID, in.Provider, cred)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREDENTIAL_WRITE_FAILED", "failed to store credential", err)
	}

	// workspace.ID: Jira sites/Linear workspaces resolve their own real id
	// via a provider-specific "which site/workspace am I" call in
	// TASK-098/TASK-104's provider client extensions; Connect's minimal
	// contract here only needs an id to upsert against, so it derives one
	// deterministically from the credential when the provider adapter's
	// Whoami doesn't yet resolve a workspace id (Jira's /myself response
	// carries no site id — the site IS the base URL).
	workspace := domain.Workspace{ID: workspaceIDFor(in.Provider, in.SiteURL, viewer.ID), Name: in.SiteURL, URL: in.SiteURL}

	status, err := uc.connections.Upsert(ctx, tenantID, userID, in.Provider, workspace, viewer, credID)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CONNECTION_UPSERT_FAILED", "failed to persist connection", err)
	}
	return status, nil
}

// workspaceIDFor derives a stable workspace id. Jira: the site base URL IS
// the natural unique key (one Atlassian site = one base URL). Linear has a
// single implicit workspace per token in this scaffold — see TASK-104 for
// a real multi-workspace Linear lookup if that becomes necessary.
func workspaceIDFor(provider domain.Provider, siteURL, viewerID string) string {
	if siteURL != "" {
		return siteURL
	}
	return fmt.Sprintf("%s:%s", provider, viewerID)
}
```

### 4. `internal/usecase/disconnect.go`, `select_workspace.go`, `get_connection_status.go`, `test_connection.go` (new)

Same tenant/user/provider-validation shape as `Connect.Execute` above, each
a thin wrapper over `ConnectionRepository`/`CredentialResolver`/
`ProviderRegistry`:

```go
// disconnect.go
type DisconnectInput struct {
	Provider    domain.Provider
	WorkspaceID string // "" = disconnect every workspace for this provider
}

type Disconnect struct{ connections ConnectionRepository }

func NewDisconnect(connections ConnectionRepository) *Disconnect {
	return &Disconnect{connections: connections}
}

func (uc *Disconnect) Execute(ctx context.Context, in DisconnectInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	if !in.Provider.Valid() {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_INVALID_PROVIDER", "provider must be jira or linear", domain.ErrInvalidProvider)
	}
	if err := uc.connections.Delete(ctx, tenantID, userID, in.Provider, in.WorkspaceID); err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_DISCONNECT_FAILED", "failed to disconnect", err)
	}
	return nil
}
```

```go
// select_workspace.go
type SelectWorkspaceInput struct {
	Provider    domain.Provider
	WorkspaceID string // "" | "all" | a specific workspace id
}

type SelectWorkspace struct{ connections ConnectionRepository }

func NewSelectWorkspace(connections ConnectionRepository) *SelectWorkspace {
	return &SelectWorkspace{connections: connections}
}

func (uc *SelectWorkspace) Execute(ctx context.Context, in SelectWorkspaceInput) (domain.ConnectionStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	status, err := uc.connections.SelectWorkspace(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_SELECT_WORKSPACE_FAILED", "failed to select workspace", err)
	}
	return status, nil
}
```

```go
// get_connection_status.go
type GetConnectionStatusInput struct {
	Provider domain.Provider
}

type GetConnectionStatus struct{ connections ConnectionRepository }

func NewGetConnectionStatus(connections ConnectionRepository) *GetConnectionStatus {
	return &GetConnectionStatus{connections: connections}
}

func (uc *GetConnectionStatus) Execute(ctx context.Context, in GetConnectionStatusInput) (domain.ConnectionStatus, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	// Not-connected is a legitimate, non-error state — GetStatus returns a
	// zero-value ConnectionStatus{Connected:false} rather than an error
	// when no rows exist, mirroring SOL-018's "not found is a valid read
	// result" convention.
	status, err := uc.connections.GetStatus(ctx, tenantID, userID, in.Provider)
	if err != nil {
		return domain.ConnectionStatus{}, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_GET_STATUS_FAILED", "failed to read connection status", err)
	}
	return status, nil
}
```

```go
// test_connection.go
type TestConnectionInput struct {
	Provider    domain.Provider
	WorkspaceID string
}

// TestConnectionResult never errors on an auth failure — false + a message
// IS the answer, per TestConnectionResult's proto shape ({ok, error}).
type TestConnectionResult struct {
	OK    bool
	Error string
}

type TestConnection struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewTestConnection(registry ProviderRegistry, credentials CredentialResolver) *TestConnection {
	return &TestConnection{registry: registry, credentials: credentials}
}

func (uc *TestConnection) Execute(ctx context.Context, in TestConnectionInput) (TestConnectionResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return TestConnectionResult{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return TestConnectionResult{}, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(in.Provider)
	if err != nil {
		return TestConnectionResult{}, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for provider", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, in.Provider, in.WorkspaceID)
	if err != nil {
		return TestConnectionResult{OK: false, Error: "not connected"}, nil
	}
	if _, err := provider.Whoami(ctx, cred); err != nil {
		return TestConnectionResult{OK: false, Error: err.Error()}, nil
	}
	return TestConnectionResult{OK: true}, nil
}
```

### 5. `migrations/0002_connections.up.sql` / `0002_connections.down.sql` (new)

```sql
-- 0002_connections.up.sql
CREATE TABLE IF NOT EXISTS issuetracking.connections (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             TEXT NOT NULL,
    user_id               TEXT NOT NULL,
    provider              TEXT NOT NULL,
    external_workspace_id TEXT NOT NULL,
    workspace_name        TEXT NOT NULL DEFAULT '',
    workspace_url         TEXT NOT NULL DEFAULT '',
    viewer_id             TEXT NOT NULL DEFAULT '',
    viewer_display_name   TEXT NOT NULL DEFAULT '',
    viewer_email          TEXT NOT NULL DEFAULT '',
    credential_id         UUID NOT NULL,
    is_selected           BOOLEAN NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT issuetracking_connections_site_key
        UNIQUE (tenant_id, user_id, provider, external_workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_issuetracking_connections_lookup
    ON issuetracking.connections (tenant_id, user_id, provider);
```

```sql
-- 0002_connections.down.sql
DROP TABLE IF EXISTS issuetracking.connections;
```

### 6. `internal/adapter/postgres/connections.go` (new)

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

var _ usecase.ConnectionRepository = (*Repository)(nil)

func (r *Repository) Upsert(ctx context.Context, tenantID, userID string, provider domain.Provider, workspace domain.Workspace, viewer domain.Viewer, credentialID string) (domain.ConnectionStatus, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO issuetracking.connections
			(tenant_id, user_id, provider, external_workspace_id, workspace_name, workspace_url,
			 viewer_id, viewer_display_name, viewer_email, credential_id, is_selected, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			NOT EXISTS (SELECT 1 FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3),
			now())
		ON CONFLICT (tenant_id, user_id, provider, external_workspace_id)
		DO UPDATE SET workspace_name = EXCLUDED.workspace_name, workspace_url = EXCLUDED.workspace_url,
			viewer_id = EXCLUDED.viewer_id, viewer_display_name = EXCLUDED.viewer_display_name,
			viewer_email = EXCLUDED.viewer_email, credential_id = EXCLUDED.credential_id, updated_at = now()
	`, tenantID, userID, string(provider), workspace.ID, workspace.Name, workspace.URL,
		viewer.ID, viewer.DisplayName, viewer.Email, credentialID)
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: upsert connection: %w", err)
	}
	return r.GetStatus(ctx, tenantID, userID, provider)
}

func (r *Repository) Delete(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) error {
	if workspaceID == "" {
		_, err := r.pool.Exec(ctx, `DELETE FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3`, tenantID, userID, string(provider))
		if err != nil {
			return fmt.Errorf("postgres: delete all connections: %w", err)
		}
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND external_workspace_id=$4`,
		tenantID, userID, string(provider), workspaceID)
	if err != nil {
		return fmt.Errorf("postgres: delete connection: %w", err)
	}
	return nil
}

func (r *Repository) GetStatus(ctx context.Context, tenantID, userID string, provider domain.Provider) (domain.ConnectionStatus, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT external_workspace_id, workspace_name, workspace_url,
		       viewer_id, viewer_display_name, viewer_email, is_selected
		FROM issuetracking.connections
		WHERE tenant_id=$1 AND user_id=$2 AND provider=$3
		ORDER BY created_at
	`, tenantID, userID, string(provider))
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: query connection status: %w", err)
	}
	defer rows.Close()

	var status domain.ConnectionStatus
	for rows.Next() {
		var ws domain.Workspace
		var viewer domain.Viewer
		var selected bool
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.URL, &viewer.ID, &viewer.DisplayName, &viewer.Email, &selected); err != nil {
			return domain.ConnectionStatus{}, fmt.Errorf("postgres: scan connection row: %w", err)
		}
		status.Workspaces = append(status.Workspaces, ws)
		if selected {
			status.SelectedWorkspaceID = ws.ID
			status.ActiveWorkspaceID = ws.ID
			status.Viewer = viewer
		}
	}
	if err := rows.Err(); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: iterate connection rows: %w", err)
	}
	status.Connected = len(status.Workspaces) > 0
	return status, nil
}

func (r *Repository) SelectWorkspace(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (domain.ConnectionStatus, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: begin select-workspace tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE issuetracking.connections SET is_selected=false WHERE tenant_id=$1 AND user_id=$2 AND provider=$3`,
		tenantID, userID, string(provider)); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: clear selection: %w", err)
	}
	if workspaceID != "" && workspaceID != "all" {
		if _, err := tx.Exec(ctx, `UPDATE issuetracking.connections SET is_selected=true WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND external_workspace_id=$4`,
			tenantID, userID, string(provider), workspaceID); err != nil {
			return domain.ConnectionStatus{}, fmt.Errorf("postgres: set selection: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ConnectionStatus{}, fmt.Errorf("postgres: commit select-workspace tx: %w", err)
	}
	return r.GetStatus(ctx, tenantID, userID, provider)
}

func (r *Repository) GetCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (string, error) {
	var query string
	var args []any
	if workspaceID == "" {
		query = `SELECT credential_id FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND is_selected=true LIMIT 1`
		args = []any{tenantID, userID, string(provider)}
	} else {
		query = `SELECT credential_id FROM issuetracking.connections WHERE tenant_id=$1 AND user_id=$2 AND provider=$3 AND external_workspace_id=$4 LIMIT 1`
		args = []any{tenantID, userID, string(provider), workspaceID}
	}
	var credID string
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&credID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", usecase.ErrConnectionNotFound
		}
		return "", fmt.Errorf("postgres: get credential id: %w", err)
	}
	return credID, nil
}
```

### 7. `internal/adapter/credential/client.go` — rewrite to implement the new `CredentialResolver`

```go
package credential

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// Resolver implements usecase.CredentialResolver against a real
// credential-broker-service connection, keyed through ConnectionRepository
// per TASK-097/SOL-015's credential-model design: Resolve looks up
// credential_id via connections, then calls ResolveCredential(credential_id)
// — the per-request read path. ResolveCredentialByOwner (owner_id =
// "<userID>:<provider>") is used only by Write/ExistingCredentialID for
// Connect's create-vs-already-connected bootstrap.
type Resolver struct {
	client      credentialbrokerv1.CredentialBrokerServiceClient
	connections usecase.ConnectionRepository
}

func New(conn grpc.ClientConnInterface, connections usecase.ConnectionRepository) *Resolver {
	return &Resolver{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn), connections: connections}
}

var _ usecase.CredentialResolver = (*Resolver)(nil)

// credentialEnvelope is the JSON shape written to/read from
// credential-broker-service's plaintext value — see the previous version of
// this file's doc comment for the original convention; unchanged here.
type credentialEnvelope struct {
	BaseURL string `json:"baseUrl"`
	Email   string `json:"email"`
	Token   string `json:"token"`
}

func ownerID(userID string, provider domain.Provider) string {
	return fmt.Sprintf("%s:%s", userID, provider)
}

func (r *Resolver) Resolve(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (usecase.Credential, error) {
	credID, err := r.connections.GetCredentialID(ctx, tenantID, userID, provider, workspaceID)
	if err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: resolving connection for %s: %w", provider, err)
	}
	resp, err := r.client.ResolveCredential(ctx, &credentialbrokerv1.ResolveCredentialRequest{CredentialId: credID})
	if err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: resolving %s credential: %w", provider, err)
	}
	var envelope credentialEnvelope
	if err := json.Unmarshal(resp.GetValue(), &envelope); err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: decoding %s credential envelope: %w", provider, err)
	}
	return usecase.Credential{BaseURL: envelope.BaseURL, Email: envelope.Email, Token: envelope.Token}, nil
}

func (r *Resolver) Write(ctx context.Context, tenantID, userID string, provider domain.Provider, cred usecase.Credential) (string, error) {
	envelope, err := json.Marshal(credentialEnvelope{BaseURL: cred.BaseURL, Email: cred.Email, Token: cred.Token})
	if err != nil {
		return "", fmt.Errorf("credential: encoding %s credential envelope: %w", provider, err)
	}
	resp, err := r.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId:          tenantID,
		OwnerId:            ownerID(userID, provider),
		Category:           credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
		EncryptedEnvelope:  envelope, // see WriteCredentialRequest's doc comment — broker re-encrypts via Vault Transit
	})
	if err != nil {
		return "", fmt.Errorf("credential: writing %s credential: %w", provider, err)
	}
	return resp.GetMetadata().GetId(), nil
}

func (r *Resolver) ExistingCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider) (string, bool, error) {
	resp, err := r.client.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
		OwnerId:  ownerID(userID, provider),
	})
	if err != nil {
		return "", false, nil // not found is not an error here — Connect treats it as "create new"
	}
	_ = resp
	return "", true, nil
}
```

⚠️ `WriteCredentialRequest.encrypted_envelope` is documented as a
client-side-encrypted envelope (`credentialbroker.proto`'s doc comment on
`WriteCredentialRequest`) — this scaffold, like `scm-integration-service`'s
existing caller, writes the plaintext JSON envelope directly for now
(matches this service's pre-existing convention; no client-side encryption
layer exists anywhere in `backend-go` yet). Flagged here rather than
silently deviating without a comment — do not treat this as solved.

### 8. `internal/adapter/jira/client.go` — add `Whoami`

```go
// jiraMyselfResponse mirrors GET /rest/api/3/myself's JSON shape.
type jiraMyselfResponse struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// Whoami calls Jira's /rest/api/3/myself to verify cred and identify the
// authenticated account — the first call Connect makes, before anything is
// persisted.
func (c *Client) Whoami(ctx context.Context, cred usecase.Credential) (domain.Viewer, error) {
	if cred.BaseURL == "" {
		return domain.Viewer{}, fmt.Errorf("jira: credential is missing a site base URL")
	}
	u := strings.TrimRight(cred.BaseURL, "/") + "/rest/api/3/myself"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.Viewer{}, fmt.Errorf("jira: building whoami request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cred.Email, cred.Token))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Viewer{}, fmt.Errorf("jira: whoami request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.Viewer{}, jiraStatusError("whoami", resp)
	}
	var parsed jiraMyselfResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.Viewer{}, fmt.Errorf("jira: decoding whoami response: %w", err)
	}
	return domain.Viewer{ID: parsed.AccountID, DisplayName: parsed.DisplayName, Email: parsed.EmailAddress}, nil
}
```

### 9. `internal/adapter/grpc/server.go` — wire the 5 new RPCs

Extend `Server` with the 5 new usecases and add handlers following the
existing `ListIssues`/`CreateIssue` translation shape:

```go
type Server struct {
	issuetrackingv1.UnimplementedIssueTrackingServiceServer

	listIssues          *usecase.ListIssues
	createIssue         *usecase.CreateIssue
	linkIssue           *usecase.LinkIssue
	connect             *usecase.Connect
	disconnect          *usecase.Disconnect
	selectWorkspace     *usecase.SelectWorkspace
	getConnectionStatus *usecase.GetConnectionStatus
	testConnection      *usecase.TestConnection
}

func (s *Server) Connect(ctx context.Context, req *issuetrackingv1.ConnectRequest) (*issuetrackingv1.ConnectionStatus, error) {
	status, err := s.connect.Execute(ctx, usecase.ConnectInput{
		Provider: toDomainProvider(req.GetProvider()),
		SiteURL:  req.GetSiteUrl(),
		Email:    req.GetEmail(),
		Token:    req.GetToken(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoConnectionStatus(status), nil
}

func (s *Server) Disconnect(ctx context.Context, req *issuetrackingv1.DisconnectRequest) (*emptypb.Empty, error) {
	err := s.disconnect.Execute(ctx, usecase.DisconnectInput{
		Provider:    toDomainProvider(req.GetProvider()),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// SelectWorkspace, GetConnectionStatus, TestConnection follow the exact
// same shape — decode request, call usecase.Execute, apperrors.ToGRPCStatus
// on failure, translate the domain result to its proto message.

func toProtoConnectionStatus(s domain.ConnectionStatus) *issuetrackingv1.ConnectionStatus {
	workspaces := make([]*issuetrackingv1.Workspace, 0, len(s.Workspaces))
	for _, w := range s.Workspaces {
		workspaces = append(workspaces, &issuetrackingv1.Workspace{Id: w.ID, Name: w.Name, Url: w.URL})
	}
	return &issuetrackingv1.ConnectionStatus{
		Connected:           s.Connected,
		ViewerId:            s.Viewer.ID,
		ViewerDisplayName:   s.Viewer.DisplayName,
		ViewerEmail:         s.Viewer.Email,
		Workspaces:          workspaces,
		ActiveWorkspaceId:   s.ActiveWorkspaceID,
		SelectedWorkspaceId: s.SelectedWorkspaceID,
		CredentialError:     s.CredentialError,
	}
}
```

Add `google.golang.org/protobuf/types/known/emptypb` to the import block.
Update `New(...)` to accept and store the 5 new usecase pointers.

### 10. `cmd/server/main.go` — wire the new dependencies

```go
connectionRepo := issuetrackingpostgres.New(pool) // same Repository, connections.go adds the methods
credentialResolver := credential.New(brokerConn, connectionRepo) // was credential.New(brokerConn)

connectUC := usecase.NewConnect(registry, credentialResolver, connectionRepo)
disconnectUC := usecase.NewDisconnect(connectionRepo)
selectWorkspaceUC := usecase.NewSelectWorkspace(connectionRepo)
getConnectionStatusUC := usecase.NewGetConnectionStatus(connectionRepo)
testConnectionUC := usecase.NewTestConnection(registry, credentialResolver)

server := issuetrackinggrpc.New(listIssuesUC, createIssueUC, linkIssueUC,
	connectUC, disconnectUC, selectWorkspaceUC, getConnectionStatusUC, testConnectionUC)
```

`credentialResolver` is now used by `ListIssues`/`CreateIssue` too (TASK-098
updates their `Resolve` call sites to the new 4-arg signature) — this task
only changes the constructor call; TASK-098 updates the call sites.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/issue-tracking-service/...
go vet ./services/issue-tracking-service/...
```

Expected: build fails at `usecase/create_issue.go`/`list_issues.go`'s
`credentials.Resolve(ctx, tenantID, in.Provider)` call sites (3-arg, old
signature) — this is expected and resolved by TASK-098, which updates them
to the new 4-arg `Resolve(ctx, tenantID, userID, provider, workspaceID)`
signature. Everything else in this task (domain, ports, connect.go,
disconnect.go, select_workspace.go, get_connection_status.go,
test_connection.go, postgres/connections.go, credential/client.go,
jira/client.go's `Whoami`, grpc/server.go's 5 new handlers) must compile
clean on its own.
