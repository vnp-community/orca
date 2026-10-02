package usecase

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

const (
	userAlice = "bbbbbbbb-0000-4000-8000-000000000001"
	userBob   = "bbbbbbbb-0000-4000-8000-000000000002"
	tenantB   = "aaaaaaaa-0000-4000-8000-000000000002"
	redirect  = "https://client.example.com/cb?keep=1"
	issuer    = "https://orca.example.com"
	resource  = "https://orca.example.com/mcp"
)

type authzHarness struct {
	t        *testing.T
	repo     *fakeAuthzRepo
	as       *fakeAuthServer
	settings *fakeRepo
	outbox   *fakeOutbox
	clock    *fixedClock
	cfg      ConsentConfig

	create  *CreateConsentRequest
	get     *GetConsentRequest
	decide  *DecideConsent
	list    *ListGrants
	revoke  *RevokeGrant
	recon   *ReconcileGrantRevocations
	clients *ListOAuthClients
	setSt   *SetOAuthClientStatus
}

func newAuthzHarness(t *testing.T) *authzHarness {
	t.Helper()
	h := &authzHarness{
		t: t, repo: newFakeAuthzRepo(), settings: newFakeRepo(), outbox: &fakeOutbox{},
		clock: &fixedClock{now: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)},
		cfg:   ConsentConfig{ConsentTTL: 10 * time.Minute, Issuer: issuer},
		as: &fakeAuthServer{info: AuthorizeInfo{
			ClientID: "app-1", ClientName: "Cool App", RegisteredVia: "dcr", Scopes: []string{"orca:read", "orca:write"},
		}},
	}
	defaults := Defaults{TenantEnabled: true, MaxTokenDays: 90}
	// DCR on so a first-seen client starts allowed.
	h.settings.rows[tenantA] = domain.TenantSettings{TenantID: tenantA, Enabled: true, DCREnabled: true, MaxTokenDays: 90, ApprovalTTLSeconds: 600}
	h.create = NewCreateConsentRequest(h.repo, h.settings, h.as, h.clock, defaults, h.cfg)
	h.get = NewGetConsentRequest(h.repo, h.clock)
	h.decide = NewDecideConsent(h.repo, h.as, h.clock, h.cfg)
	h.list = NewListGrants(h.repo, h.as)
	h.revoke = NewRevokeGrant(h.repo, h.as, h.clock)
	h.recon = NewReconcileGrantRevocations(h.repo, h.as, h.clock)
	h.clients = NewListOAuthClients(h.repo, h.as)
	h.setSt = NewSetOAuthClientStatus(h.repo, h.as, h.outbox, h.clock)
	return h
}

func as(tenantID, userID, role string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ctx = tenant.WithUserID(ctx, userID)
	if role != "" {
		ctx = tenant.WithRole(ctx, role)
	}
	return ctx
}

func (h *authzHarness) in(state string) CreateConsentRequestInput {
	return CreateConsentRequestInput{
		AuthorizeParams: AuthorizeParams{
			ResponseType: "code", ClientID: "app-1", RedirectURI: redirect, Scope: "orca:read orca:write",
			CodeChallenge: strings.Repeat("c", 43), CodeChallengeMethod: "S256", Resource: resource,
		},
		State: state,
	}
}

func (h *authzHarness) pending(ctx context.Context) string {
	h.t.Helper()
	out, err := h.create.Execute(ctx, h.in("st"))
	if err != nil || out.RequestID == "" {
		h.t.Fatalf("create consent: %+v %v", out, err)
	}
	return out.RequestID
}

func (h *authzHarness) approve(ctx context.Context, id string, scopes ...string) DecideConsentOutput {
	h.t.Helper()
	out, err := h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "approve", Scopes: scopes})
	if err != nil {
		h.t.Fatalf("approve: %v", err)
	}
	return out
}

func codeOf(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func wantErr(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || codeOf(err) != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func query(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

// --- CreateConsentRequest ---

func TestCreateConsent_StoresPendingRequestForNewClient(t *testing.T) {
	h := newAuthzHarness(t)
	id := h.pending(as(tenantA, userAlice, "user"))
	c := h.repo.consents[id]
	if !c.IsNewClient || !c.RegisteredViaDCR || c.UserID != userAlice || c.State != "st" || c.ClientName != "Cool App" {
		t.Fatalf("request = %+v", c)
	}
	if got := c.ExpiresAt.Sub(c.CreatedAt); got != 10*time.Minute {
		t.Fatalf("ttl = %v", got)
	}
}

func TestCreateConsent_Guards(t *testing.T) {
	cases := []struct {
		name string
		prep func(*authzHarness) context.Context
		in   func(*CreateConsentRequestInput)
		code string
	}{
		{"client blocked in tenant", func(h *authzHarness) context.Context {
			h.as.clientState = map[string]string{"app-1": "blocked"}
			return as(tenantA, userAlice, "user")
		}, nil, domain.CodeClientNotAllowed},
		{"client pending (tenant has DCR off)", func(h *authzHarness) context.Context {
			h.settings.rows[tenantA] = domain.TenantSettings{TenantID: tenantA, Enabled: true, DCREnabled: false, MaxTokenDays: 90, ApprovalTTLSeconds: 600}
			return as(tenantA, userAlice, "user")
		}, nil, domain.CodeClientNotAllowed},
		{"tenant MCP disabled", func(h *authzHarness) context.Context {
			h.settings.rows[tenantA] = domain.TenantSettings{TenantID: tenantA, Enabled: false, MaxTokenDays: 90, ApprovalTTLSeconds: 600}
			return as(tenantA, userAlice, "user")
		}, nil, domain.CodeDisabled},
		{"state too long", func(*authzHarness) context.Context { return as(tenantA, userAlice, "user") },
			func(in *CreateConsentRequestInput) { in.State = strings.Repeat("s", 513) }, domain.CodeInvalidArgument},
		{"only admin scope asked by plain user", func(h *authzHarness) context.Context {
			h.as.info.Scopes = []string{"orca:admin"}
			return as(tenantA, userAlice, "user")
		}, nil, domain.CodeScopeNotAllowed},
		{"unknown role gets nothing", func(*authzHarness) context.Context { return as(tenantA, userAlice, "") }, nil, domain.CodeScopeNotAllowed},
		{"auth-service rejects request", func(h *authzHarness) context.Context {
			h.as.validateErr = appErr(apperrors.KindInvalidArgument, "OAUTH_INVALID_REDIRECT_URI")
			return as(tenantA, userAlice, "user")
		}, nil, "OAUTH_INVALID_REDIRECT_URI"},
		{"no user in context", func(*authzHarness) context.Context { return tenant.WithTenantID(context.Background(), tenantA) }, nil, domain.CodeNoTenant},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newAuthzHarness(t)
			ctx := tc.prep(h)
			in := h.in("")
			if tc.in != nil {
				tc.in(&in)
			}
			_, err := h.create.Execute(ctx, in)
			wantErr(t, err, tc.code)
			if len(h.repo.consents) != 0 {
				t.Fatal("a rejected request must not leave a consent request behind")
			}
		})
	}
}

func TestCreateConsent_RoleCapsRequestedScopes(t *testing.T) {
	h := newAuthzHarness(t)
	h.as.info.Scopes = []string{"orca:read", "orca:admin"}
	id := h.pending(as(tenantA, userAlice, "user"))
	if got := h.repo.consents[id].Scopes; len(got) != 1 || got[0] != "orca:read" {
		t.Fatalf("scopes = %v, want admin scope dropped for a plain user", got)
	}
}

func TestCreateConsent_AutoApproveOnlyWhenGrantCoversEverything(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	h.approve(ctx, h.pending(ctx), "orca:read", "orca:write")

	out, err := h.create.Execute(ctx, h.in("xyz"))
	if err != nil || out.RedirectURL == "" || out.RequestID != "" {
		t.Fatalf("expected auto-approve, got %+v %v", out, err)
	}
	q := query(t, out.RedirectURL)
	if q.Get("code") == "" || q.Get("state") != "xyz" || q.Get("iss") != issuer || q.Get("keep") != "1" {
		t.Fatalf("redirect params = %v", q)
	}
	last := h.as.issued[len(h.as.issued)-1]
	if last.GrantID == "" || last.Resource != resource || last.CodeChallenge == "" {
		t.Fatalf("issue call = %+v", last)
	}

	// Asking for more than was granted must prompt again.
	h.as.info.Scopes = []string{"orca:read", "orca:exec"}
	in := h.in("")
	in.Scope = "orca:read orca:exec"
	out, err = h.create.Execute(ctx, in)
	if err != nil || out.RequestID == "" {
		t.Fatalf("scope expansion must re-prompt: %+v %v", out, err)
	}
}

func TestCreateConsent_GrantIsPerClientNeverShared(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	h.approve(ctx, h.pending(ctx), "orca:read", "orca:write")

	in := h.in("")
	in.ClientID = "app-2" // a different client asks for the same scopes
	out, err := h.create.Execute(ctx, in)
	if err != nil || out.RedirectURL != "" || out.RequestID == "" {
		t.Fatalf("consent for app-1 must not satisfy app-2: %+v %v", out, err)
	}
}

func TestCreateConsent_GrantIsPerUser(t *testing.T) {
	h := newAuthzHarness(t)
	h.approve(as(tenantA, userAlice, "user"), h.pending(as(tenantA, userAlice, "user")), "orca:read", "orca:write")
	out, err := h.create.Execute(as(tenantA, userBob, "user"), h.in(""))
	if err != nil || out.RequestID == "" {
		t.Fatalf("alice's consent must not auto-approve bob: %+v %v", out, err)
	}
}

// --- GetConsentRequest ---

func TestGetConsent_VisibleOnlyToOwner(t *testing.T) {
	h := newAuthzHarness(t)
	id := h.pending(as(tenantA, userAlice, "user"))

	v, err := h.get.Execute(as(tenantA, userAlice, "user"), id)
	if err != nil {
		t.Fatal(err)
	}
	if v.RedirectHost != "client.example.com" || strings.Contains(v.RedirectHost, "/") || len(v.Scopes) != 2 || v.Scopes[0].Label == "" {
		t.Fatalf("view = %+v", v)
	}
	for name, ctx := range map[string]context.Context{
		"other user":   as(tenantA, userBob, "user"),
		"other tenant": as(tenantB, userAlice, "user"),
	} {
		_, err := h.get.Execute(ctx, id)
		if codeOf(err) != domain.CodeConsentNotFound {
			t.Errorf("%s: got %v, want MCP_CONSENT_NOT_FOUND", name, err)
		}
	}
	for _, bad := range []string{"", "not-a-uuid", newID()} {
		if _, err := h.get.Execute(as(tenantA, userAlice, "user"), bad); codeOf(err) != domain.CodeConsentNotFound {
			t.Errorf("id %q: %v", bad, err)
		}
	}
}

func TestGetConsent_ExpiredAndDecided(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	id := h.pending(ctx)
	h.clock.now = h.clock.now.Add(11 * time.Minute)
	_, err := h.get.Execute(ctx, id)
	wantErr(t, err, domain.CodeConsentExpired)
	// An expired request that belongs to someone else stays indistinguishable from "missing".
	_, err = h.get.Execute(as(tenantA, userBob, "user"), id)
	wantErr(t, err, domain.CodeConsentNotFound)

	h.clock.now = h.clock.now.Add(-11 * time.Minute)
	h.approve(ctx, id, "orca:read")
	_, err = h.get.Execute(ctx, id)
	wantErr(t, err, domain.CodeConsentNotFound)
}

func TestGetConsent_ReportsAlreadyGranted(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	h.approve(ctx, h.pending(ctx), "orca:read")
	h.as.info.Scopes = []string{"orca:read", "orca:write"}
	id := h.pending(ctx)
	v, err := h.get.Execute(ctx, id)
	if err != nil || len(v.AlreadyGranted) != 1 || v.AlreadyGranted[0] != "orca:read" || v.Request.IsNewClient {
		t.Fatalf("view = %+v err = %v", v, err)
	}
}

// --- DecideConsent ---

func TestDecide_ApproveCreatesGrantWithChosenScopesAndIssuesCode(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	id := h.pending(ctx)
	out := h.approve(ctx, id, "orca:read") // user narrows what the app asked for

	q := query(t, out.RedirectURL)
	if q.Get("code") != "code-1" || q.Get("state") != "st" || q.Get("iss") != issuer || q.Get("keep") != "1" {
		t.Fatalf("redirect = %s", out.RedirectURL)
	}
	if !strings.HasPrefix(out.RedirectURL, "https://client.example.com/cb?") {
		t.Fatalf("redirect must target the registered redirect_uri: %s", out.RedirectURL)
	}
	grants, _ := h.repo.ListActiveGrants(ctx, tenantA, userAlice)
	if len(grants) != 1 || len(grants[0].Scopes) != 1 || grants[0].Scopes[0] != "orca:read" {
		t.Fatalf("grants = %+v", grants)
	}
	if got := h.as.issued[0]; len(got.Scopes) != 1 || got.GrantID != grants[0].ID || got.ClientID != "app-1" || got.RedirectURI != redirect {
		t.Fatalf("issued = %+v", got)
	}
	if len(h.repo.events) != 1 || h.repo.events[0].Subject != domain.SubjectGrantCreated {
		t.Fatalf("events = %+v", h.repo.events)
	}
}

func TestDecide_SecondApprovalReplacesScopesAndEmitsUpdated(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	h.approve(ctx, h.pending(ctx), "orca:read", "orca:write")
	h.as.info.Scopes = []string{"orca:read", "orca:exec"}
	in := h.in("")
	in.Scope = "orca:read orca:exec"
	r, _ := h.create.Execute(ctx, in)
	h.approve(ctx, r.RequestID, "orca:exec")

	grants, _ := h.repo.ListActiveGrants(ctx, tenantA, userAlice)
	if len(grants) != 1 || len(grants[0].Scopes) != 1 || grants[0].Scopes[0] != "orca:exec" {
		t.Fatalf("grants = %+v (exactly the last chosen set, one active grant per client)", grants)
	}
	if last := h.repo.events[len(h.repo.events)-1]; last.Subject != domain.SubjectGrantUpdated {
		t.Fatalf("last event = %s", last.Subject)
	}
}

func TestDecide_DenyRedirectsWithAccessDeniedAndCreatesNothing(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	id := h.pending(ctx)
	out, err := h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "deny"})
	if err != nil {
		t.Fatal(err)
	}
	q := query(t, out.RedirectURL)
	if q.Get("error") != "access_denied" || q.Get("state") != "st" || q.Get("iss") != issuer || q.Get("code") != "" {
		t.Fatalf("redirect = %s", out.RedirectURL)
	}
	if len(h.repo.grants) != 0 || len(h.as.issued) != 0 {
		t.Fatal("deny must create no grant and issue no code")
	}
}

func TestDecide_ScopeValidation(t *testing.T) {
	cases := []struct {
		name   string
		role   string
		scopes []string
		code   string
	}{
		{"nothing selected", "user", nil, domain.CodeScopeInvalid},
		{"unknown scope", "user", []string{"orca:nuke"}, domain.CodeScopeInvalid},
		{"scope the app did not ask for", "user", []string{"orca:exec"}, domain.CodeScopeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newAuthzHarness(t)
			ctx := as(tenantA, userAlice, tc.role)
			id := h.pending(ctx)
			_, err := h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "approve", Scopes: tc.scopes})
			wantErr(t, err, tc.code)
			if c := h.repo.consents[id]; c.DecidedAt != nil {
				t.Fatal("an invalid decision must not consume the request")
			}
		})
	}
}

func TestDecide_ScopeAboveRoleCeilingIsNotAllowed(t *testing.T) {
	h := newAuthzHarness(t)
	id := newID()
	now := h.clock.now
	// A request that (somehow) carries admin, decided by a plain user.
	h.repo.consents[id] = domain.ConsentRequest{ID: id, TenantID: tenantA, UserID: userAlice, ClientID: "app-1", RedirectURI: redirect,
		Scopes: []string{"orca:read", "orca:admin"}, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	_, err := h.decide.Execute(as(tenantA, userAlice, "user"), DecideConsentInput{RequestID: id, Decision: "approve", Scopes: []string{"orca:admin"}})
	wantErr(t, err, domain.CodeScopeNotAllowed)
	if len(h.repo.grants) != 0 {
		t.Fatal("no grant may be created")
	}
}

func TestDecide_ReplayAndOwnership(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	id := h.pending(ctx)

	_, err := h.decide.Execute(as(tenantA, userBob, "user"), DecideConsentInput{RequestID: id, Decision: "approve", Scopes: []string{"orca:read"}})
	wantErr(t, err, domain.CodeConsentNotFound)
	_, err = h.decide.Execute(as(tenantB, userAlice, "user"), DecideConsentInput{RequestID: id, Decision: "approve", Scopes: []string{"orca:read"}})
	wantErr(t, err, domain.CodeConsentNotFound)

	h.approve(ctx, id, "orca:read")
	_, err = h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "approve", Scopes: []string{"orca:read"}})
	wantErr(t, err, domain.CodeConsentNotFound)
	_, err = h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "deny"})
	wantErr(t, err, domain.CodeConsentNotFound)
	if len(h.as.issued) != 1 {
		t.Fatalf("code issued %d times, want exactly once", len(h.as.issued))
	}
}

func TestDecide_ExpiredAndBadDecision(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	id := h.pending(ctx)
	_, err := h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "maybe"})
	wantErr(t, err, domain.CodeInvalidArgument)
	h.clock.now = h.clock.now.Add(11 * time.Minute)
	_, err = h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "approve", Scopes: []string{"orca:read"}})
	wantErr(t, err, domain.CodeConsentExpired)
}

func TestDecide_ConcurrentDecisionsOnlyOneWins(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	id := h.pending(ctx)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "approve", Scopes: []string{"orca:read"}}); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("%d decisions won, want exactly 1", wins)
	}
}

func TestDecide_AuthServiceFailureSurfacesCode(t *testing.T) {
	h := newAuthzHarness(t)
	ctx := as(tenantA, userAlice, "user")
	id := h.pending(ctx)
	h.as.issueErr = appErr(apperrors.KindPermissionDenied, "OAUTH_UNAUTHORIZED_CLIENT")
	_, err := h.decide.Execute(ctx, DecideConsentInput{RequestID: id, Decision: "approve", Scopes: []string{"orca:read"}})
	wantErr(t, err, "OAUTH_UNAUTHORIZED_CLIENT")
}
