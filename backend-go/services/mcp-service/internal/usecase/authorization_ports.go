package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

var (
	ErrConsentRequestNotFound = errors.New("usecase: consent request not found")
	// ErrConsentNotDecidable: no pending, unexpired request of this user matched.
	ErrConsentNotDecidable = errors.New("usecase: consent request is not decidable")
	ErrGrantNotFound       = errors.New("usecase: grant not found")
)

// Clock abstracts time for expiry logic.
type Clock interface{ Now() time.Time }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// ApproveConsentInput carries what the atomic approve needs. The event ids are
// pre-generated so the repository can write the outbox row in the same
// transaction without owning id generation.
type ApproveConsentInput struct {
	TenantID, UserID, RequestID string
	Scopes                      []string
	Now                         time.Time
	GrantID                     string // used only when a new grant row is inserted
	EventID                     string
}

type ApproveConsentResult struct {
	Request domain.ConsentRequest
	Grant   domain.Grant
	Created bool // false = an existing active grant was updated
}

// AuthorizationRepository persists consent requests and grants. Every
// state change that emits an event writes its outbox row in the same
// transaction. All methods are tenant-scoped (RLS plus explicit tenant_id).
type AuthorizationRepository interface {
	CreateConsentRequest(ctx context.Context, r domain.ConsentRequest) error
	// GetConsentRequest returns the request only if it belongs to userID in
	// tenantID (decided or not); otherwise ErrConsentRequestNotFound.
	GetConsentRequest(ctx context.Context, tenantID, userID, requestID string) (domain.ConsentRequest, error)
	// ApproveConsent atomically marks the request approved (only if pending,
	// unexpired and owned by the user), upserts the user's active grant with
	// exactly in.Scopes, and enqueues orca.mcp.grant.created|updated.
	// ErrConsentNotDecidable if the conditional update matched nothing.
	ApproveConsent(ctx context.Context, in ApproveConsentInput) (ApproveConsentResult, error)
	// DenyConsent marks the request denied under the same conditions.
	DenyConsent(ctx context.Context, tenantID, userID, requestID string, now time.Time) (domain.ConsentRequest, error)

	GetActiveGrant(ctx context.Context, tenantID, userID, clientID string) (domain.Grant, error)
	// CountGrantsForClient counts every grant (any status) of the client in the tenant.
	CountGrantsForClient(ctx context.Context, tenantID, clientID string) (int, error)
	// ListActiveGrants lists active grants; userID == "" means every user in the tenant.
	ListActiveGrants(ctx context.Context, tenantID, userID string) ([]domain.Grant, error)
	CountActiveGrantsByClient(ctx context.Context, tenantID string) (map[string]int, error)

	// RevokeGrant marks the grant revoked and enqueues orca.mcp.grant.revoked
	// in one transaction. ownerUserID != "" restricts to that user's grants.
	// ErrGrantNotFound if no such grant is visible. Revoking an already
	// revoked grant succeeds without a second event (newlyRevoked=false).
	RevokeGrant(ctx context.Context, tenantID, grantID, ownerUserID, revokedBy string, now time.Time, eventID string) (g domain.Grant, newlyRevoked bool, err error)
	MarkRevocationPropagated(ctx context.Context, tenantID, grantID string, now time.Time) error
	// ListUnpropagatedRevocations scans across tenants (reconcile job only).
	ListUnpropagatedRevocations(ctx context.Context, limit int) ([]domain.Grant, error)
}

// AuthorizeInfo is auth-service's verdict on an authorize request.
type AuthorizeInfo struct {
	ClientID, ClientName, ClientURI, RegisteredVia string
	Scopes                                         []string
	RedirectURI, Resource                          string
}

// AuthorizeParams are the raw /authorize parameters forwarded by the gateway.
type AuthorizeParams struct {
	ResponseType, ClientID, RedirectURI, Scope, CodeChallenge, CodeChallengeMethod, Resource string
}

// OAuthClientView is a client plus its standing in the calling tenant.
type OAuthClientView struct {
	ClientID, Name, ClientURI, RegisteredVia, Status string
	RedirectURIs                                     []string
	CreatedAt                                        time.Time
	LastUsedAt                                       *time.Time
}

// IssueCodeInput asks auth-service for an authorization code. Tenant and user
// travel in the call context (forwarded as gRPC metadata), never here.
type IssueCodeInput struct {
	ClientID, RedirectURI, CodeChallenge, Resource, GrantID string
	Scopes                                                  []string
}

// AuthorizationServer is the port to auth-service's OAuth* RPCs. The
// dependency direction is mcp-service -> auth-service only. Implementations
// must put the ctx tenant/user on the wire and bound each call by a deadline.
type AuthorizationServer interface {
	ValidateAuthorizeRequest(ctx context.Context, p AuthorizeParams) (AuthorizeInfo, error)
	EnsureClientForTenant(ctx context.Context, clientID string, dcrEnabled bool) (OAuthClientView, error)
	IssueAuthCode(ctx context.Context, in IssueCodeInput) (code string, err error)
	// RevokeGrant is idempotent on the auth-service side.
	RevokeGrant(ctx context.Context, grantID, reason string) error
	ListClientsForTenant(ctx context.Context) ([]OAuthClientView, error)
	SetClientStatus(ctx context.Context, clientID, status string) (OAuthClientView, error)
	// MemberNames maps user id to display name (best effort, tenant directory).
	MemberNames(ctx context.Context) (map[string]string, error)
}

// ConsentConfig is the consent flow's runtime configuration.
type ConsentConfig struct {
	ConsentTTL time.Duration
	// Issuer is the public base URL echoed as `iss` (RFC 9207). Taken from
	// configuration, never from a request.
	Issuer string
}

const DefaultConsentTTL = 10 * time.Minute
