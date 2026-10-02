package usecase

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type fakeAuthzRepo struct {
	mu       sync.Mutex
	consents map[string]domain.ConsentRequest
	grants   map[string]domain.Grant
	events   []domain.OutboxRecord
}

func newFakeAuthzRepo() *fakeAuthzRepo {
	return &fakeAuthzRepo{consents: map[string]domain.ConsentRequest{}, grants: map[string]domain.Grant{}}
}

func (f *fakeAuthzRepo) CreateConsentRequest(_ context.Context, c domain.ConsentRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consents[c.ID] = c
	return nil
}

func (f *fakeAuthzRepo) GetConsentRequest(_ context.Context, tenantID, userID, id string) (domain.ConsentRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.consents[id]
	if !ok || c.TenantID != tenantID || c.UserID != userID {
		return domain.ConsentRequest{}, ErrConsentRequestNotFound
	}
	return c, nil
}

func (f *fakeAuthzRepo) decide(tenantID, userID, id, decision string, now time.Time) (domain.ConsentRequest, error) {
	c, ok := f.consents[id]
	if !ok || c.TenantID != tenantID || c.UserID != userID || c.DecidedAt != nil || !now.Before(c.ExpiresAt) {
		return domain.ConsentRequest{}, ErrConsentNotDecidable
	}
	c.DecidedAt, c.Decision = &now, decision
	f.consents[id] = c
	return c, nil
}

func (f *fakeAuthzRepo) DenyConsent(_ context.Context, tenantID, userID, id string, now time.Time) (domain.ConsentRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.decide(tenantID, userID, id, domain.DecisionDeny, now)
}

func (f *fakeAuthzRepo) ApproveConsent(_ context.Context, in ApproveConsentInput) (ApproveConsentResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	req, err := f.decide(in.TenantID, in.UserID, in.RequestID, domain.DecisionApprove, in.Now)
	if err != nil {
		return ApproveConsentResult{}, err
	}
	subject := domain.SubjectGrantUpdated
	for id, g := range f.grants {
		if g.TenantID == in.TenantID && g.UserID == in.UserID && g.ClientID == req.ClientID && g.Status == domain.GrantActive {
			g.Scopes, g.UpdatedAt = in.Scopes, in.Now
			f.grants[id] = g
			f.addEvent(in.EventID, subject, in.TenantID, in.Now)
			return ApproveConsentResult{Request: req, Grant: g}, nil
		}
	}
	g := domain.Grant{ID: in.GrantID, TenantID: in.TenantID, UserID: in.UserID, ClientID: req.ClientID, ClientName: req.ClientName,
		Scopes: in.Scopes, Status: domain.GrantActive, CreatedAt: in.Now, UpdatedAt: in.Now}
	f.grants[g.ID] = g
	f.addEvent(in.EventID, domain.SubjectGrantCreated, in.TenantID, in.Now)
	return ApproveConsentResult{Request: req, Grant: g, Created: true}, nil
}

func (f *fakeAuthzRepo) addEvent(id, subject, tenantID string, at time.Time) {
	ev, _ := domain.NewOutboxEvent(id, subject, tenantID, at, nil)
	f.events = append(f.events, ev)
}

func (f *fakeAuthzRepo) GetActiveGrant(_ context.Context, tenantID, userID, clientID string) (domain.Grant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, g := range f.grants {
		if g.TenantID == tenantID && g.UserID == userID && g.ClientID == clientID && g.Status == domain.GrantActive {
			return g, nil
		}
	}
	return domain.Grant{}, ErrGrantNotFound
}

func (f *fakeAuthzRepo) CountGrantsForClient(_ context.Context, tenantID, clientID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, g := range f.grants {
		if g.TenantID == tenantID && g.ClientID == clientID {
			n++
		}
	}
	return n, nil
}

func (f *fakeAuthzRepo) ListActiveGrants(_ context.Context, tenantID, userID string) ([]domain.Grant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Grant
	for _, g := range f.grants {
		if g.TenantID == tenantID && g.Status == domain.GrantActive && (userID == "" || g.UserID == userID) {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeAuthzRepo) CountActiveGrantsByClient(_ context.Context, tenantID string) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]int{}
	for _, g := range f.grants {
		if g.TenantID == tenantID && g.Status == domain.GrantActive {
			out[g.ClientID]++
		}
	}
	return out, nil
}

func (f *fakeAuthzRepo) RevokeGrant(_ context.Context, tenantID, grantID, owner, by string, now time.Time, eventID string) (domain.Grant, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.grants[grantID]
	if !ok || g.TenantID != tenantID || (owner != "" && g.UserID != owner) {
		return domain.Grant{}, false, ErrGrantNotFound
	}
	if g.Status == domain.GrantRevoked {
		return g, false, nil
	}
	g.Status, g.RevokedAt, g.RevokedBy = domain.GrantRevoked, &now, by
	f.grants[grantID] = g
	f.addEvent(eventID, domain.SubjectGrantRevoked, tenantID, now)
	return g, true, nil
}

func (f *fakeAuthzRepo) MarkRevocationPropagated(_ context.Context, tenantID, grantID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	g := f.grants[grantID]
	if g.TenantID == tenantID {
		g.RevocationPropagatedAt = &now
		f.grants[grantID] = g
	}
	return nil
}

func (f *fakeAuthzRepo) ListUnpropagatedRevocations(_ context.Context, limit int) ([]domain.Grant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Grant
	for _, g := range f.grants {
		if g.Status == domain.GrantRevoked && g.RevocationPropagatedAt == nil {
			out = append(out, g)
		}
	}
	return out, nil
}

// fakeOutbox records enqueued events.
type fakeOutbox struct{ records []domain.OutboxRecord }

func (f *fakeOutbox) EnqueueOutbox(_ context.Context, _ string, rec domain.OutboxRecord) error {
	f.records = append(f.records, rec)
	return nil
}

// fakeAuthServer is the auth-service port.
type fakeAuthServer struct {
	info        AuthorizeInfo
	validateErr error
	clientState map[string]string // clientID -> status in this tenant
	issueErr    error
	issued      []IssueCodeInput
	revokeErr   error
	revoked     []string // "grantID:reason"
	clients     []OAuthClientView
	setErr      error
	names       map[string]string
}

func (f *fakeAuthServer) ValidateAuthorizeRequest(_ context.Context, p AuthorizeParams) (AuthorizeInfo, error) {
	if f.validateErr != nil {
		return AuthorizeInfo{}, f.validateErr
	}
	info := f.info
	info.ClientID, info.RedirectURI, info.Resource = p.ClientID, p.RedirectURI, p.Resource
	return info, nil
}

func (f *fakeAuthServer) EnsureClientForTenant(_ context.Context, clientID string, dcr bool) (OAuthClientView, error) {
	if f.clientState == nil {
		f.clientState = map[string]string{}
	}
	if _, ok := f.clientState[clientID]; !ok {
		f.clientState[clientID] = "pending"
		if dcr {
			f.clientState[clientID] = "allowed"
		}
	}
	return OAuthClientView{ClientID: clientID, Status: f.clientState[clientID]}, nil
}

func (f *fakeAuthServer) IssueAuthCode(_ context.Context, in IssueCodeInput) (string, error) {
	if f.issueErr != nil {
		return "", f.issueErr
	}
	f.issued = append(f.issued, in)
	return fmt.Sprintf("code-%d", len(f.issued)), nil
}

func (f *fakeAuthServer) RevokeGrant(_ context.Context, grantID, reason string) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	f.revoked = append(f.revoked, grantID+":"+reason)
	return nil
}

func (f *fakeAuthServer) ListClientsForTenant(context.Context) ([]OAuthClientView, error) {
	return f.clients, nil
}

func (f *fakeAuthServer) SetClientStatus(_ context.Context, clientID, status string) (OAuthClientView, error) {
	if f.setErr != nil {
		return OAuthClientView{}, f.setErr
	}
	return OAuthClientView{ClientID: clientID, Status: status}, nil
}

func (f *fakeAuthServer) MemberNames(context.Context) (map[string]string, error) {
	return f.names, nil
}

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

func appErr(kind apperrors.Kind, code string) error { return apperrors.New(kind, code, "x", nil) }

func newID() string { return uuid.NewString() }
