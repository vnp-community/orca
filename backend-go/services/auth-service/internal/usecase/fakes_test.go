package usecase

import (
	"context"
	"errors"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// fakeUserRepository is an in-memory UserRepository — the "test against
// fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeUserRepository struct {
	byID    map[string]domain.User
	byEmail map[string]domain.User
	hashes  map[string]string // keyed by user ID

	createErr error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		byID:    make(map[string]domain.User),
		byEmail: make(map[string]domain.User),
		hashes:  make(map[string]string),
	}
}

func (f *fakeUserRepository) seed(u domain.User, passwordHash string) {
	f.byID[u.ID] = u
	f.byEmail[u.Email] = u
	f.hashes[u.ID] = passwordHash
}

func (f *fakeUserRepository) HasAnyUsers(ctx context.Context) (bool, error) {
	return len(f.byID) > 0, nil
}

func (f *fakeUserRepository) CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	if f.createErr != nil {
		return domain.User{}, f.createErr
	}
	if _, exists := f.byEmail[user.Email]; exists {
		return domain.User{}, ErrUserAlreadyExists
	}
	f.seed(user, passwordHash)
	return user, nil
}

func (f *fakeUserRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return domain.User{}, "", ErrUserNotFound
	}
	return u, f.hashes[u.ID], nil
}

func (f *fakeUserRepository) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	u, ok := f.byID[userID]
	if !ok {
		return domain.User{}, ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) ListUsers(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.User, string, error) {
	var out []domain.User
	for _, u := range f.byID {
		if u.TenantID == tenantID {
			out = append(out, u)
		}
	}
	return out, "", nil
}

func (f *fakeUserRepository) UpdateUserRole(ctx context.Context, userID string, role domain.Role) (domain.User, error) {
	u, ok := f.byID[userID]
	if !ok {
		return domain.User{}, ErrUserNotFound
	}
	u.Role = role
	f.byID[userID] = u
	f.byEmail[u.Email] = u
	return u, nil
}

func (f *fakeUserRepository) SetActive(ctx context.Context, userID string, active bool) error {
	u, ok := f.byID[userID]
	if !ok {
		// Matches the real postgres adapter: affecting 0 rows is not an
		// error — the caller re-reads the user afterward and surfaces
		// ErrUserNotFound from that read instead.
		return nil
	}
	u.IsActive = active
	f.byID[userID] = u
	f.byEmail[u.Email] = u
	return nil
}

func (f *fakeUserRepository) Count(ctx context.Context) (int32, error) {
	return int32(len(f.byID)), nil
}

// fakeSessionRepository is an in-memory SessionRepository.
type fakeSessionRepository struct {
	byHash map[string]domain.Session
}

func newFakeSessionRepository() *fakeSessionRepository {
	return &fakeSessionRepository{byHash: make(map[string]domain.Session)}
}

func (f *fakeSessionRepository) CreateSession(ctx context.Context, session domain.Session) error {
	f.byHash[session.TokenHash] = session
	return nil
}

func (f *fakeSessionRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	s, ok := f.byHash[tokenHash]
	if !ok {
		return domain.Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeSessionRepository) RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	s, ok := f.byHash[tokenHash]
	if !ok {
		return ErrSessionNotFound
	}
	s.RevokedAt = &revokedAt
	f.byHash[tokenHash] = s
	return nil
}

func (f *fakeSessionRepository) ListForUser(ctx context.Context, userID string) ([]domain.Session, error) {
	var out []domain.Session
	for _, s := range f.byHash {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSessionRepository) RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) (int32, error) {
	var n int32
	for hash, s := range f.byHash {
		if s.UserID == userID && s.RevokedAt == nil {
			s.RevokedAt = &revokedAt
			f.byHash[hash] = s
			n++
		}
	}
	return n, nil
}

func (f *fakeSessionRepository) CountActive(ctx context.Context, now time.Time) (int32, error) {
	var n int32
	for _, s := range f.byHash {
		if s.IsValid(now) {
			n++
		}
	}
	return n, nil
}

// fakeAccessPolicyRepository is an in-memory AccessPolicyRepository —
// stores every version row (append-only, matching the real postgres
// adapter's (id, version) primary key), keyed by id then version.
type fakeAccessPolicyRepository struct {
	versions map[string]map[int32]domain.AccessPolicy

	insertErr error
}

func newFakeAccessPolicyRepository() *fakeAccessPolicyRepository {
	return &fakeAccessPolicyRepository{versions: make(map[string]map[int32]domain.AccessPolicy)}
}

func (f *fakeAccessPolicyRepository) InsertPolicyVersion(ctx context.Context, p domain.AccessPolicy) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	if f.versions[p.ID] == nil {
		f.versions[p.ID] = make(map[int32]domain.AccessPolicy)
	}
	f.versions[p.ID][p.Version] = p
	return nil
}

func (f *fakeAccessPolicyRepository) GetLatestPolicy(ctx context.Context, id string) (domain.AccessPolicy, error) {
	versions, ok := f.versions[id]
	if !ok || len(versions) == 0 {
		return domain.AccessPolicy{}, ErrPolicyNotFound
	}
	var latest domain.AccessPolicy
	for _, p := range versions {
		if p.Version > latest.Version {
			latest = p
		}
	}
	return latest, nil
}

func (f *fakeAccessPolicyRepository) ListLatestPolicies(ctx context.Context, pageToken string, pageSize int32) ([]domain.AccessPolicy, string, error) {
	var out []domain.AccessPolicy
	for id := range f.versions {
		latest, err := f.GetLatestPolicy(ctx, id)
		if err == nil {
			out = append(out, latest)
		}
	}
	return out, "", nil
}

func (f *fakeAccessPolicyRepository) DeletePolicy(ctx context.Context, id string) error {
	delete(f.versions, id)
	return nil
}

func (f *fakeAccessPolicyRepository) CountDistinctIDs(ctx context.Context) (int32, error) {
	return int32(len(f.versions)), nil
}

// fakePolicyPublisher is an in-memory PolicyDataPublisher — records every
// call so tests can assert UpdateAccessPolicy actually invoked it.
type fakePolicyPublisher struct {
	published  []domain.AccessPolicy
	publishErr error
}

func (f *fakePolicyPublisher) PublishPolicyChange(ctx context.Context, policy domain.AccessPolicy) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.published = append(f.published, policy)
	return nil
}

// fakeAuditRepository is an in-memory AuditRepository.
type fakeAuditRepository struct {
	entries []domain.AuditEntry
}

func (f *fakeAuditRepository) Append(ctx context.Context, entry domain.AuditEntry) error {
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeAuditRepository) Query(ctx context.Context, tenantID string, since time.Time, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	var out []domain.AuditEntry
	for _, e := range f.entries {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, "", nil
}

// fakeHasher is a PasswordHasher that just prefixes with "hashed:" —
// deterministic and fast for unit tests, no real bcrypt cost.
type fakeHasher struct{}

func (fakeHasher) Hash(password string) (string, error) {
	return "hashed:" + password, nil
}

func (fakeHasher) Compare(hash, password string) error {
	if hash != "hashed:"+password {
		return errors.New("fakeHasher: mismatch")
	}
	return nil
}

// fakeClock is a Clock returning a fixed instant, advanceable by tests.
type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

// fakeTokenSigner is an in-memory TokenSigner — records the last Sign call
// so tests can assert on the claims IssueServiceToken built, without a real
// Vault Transit round trip.
type fakeTokenSigner struct {
	signErr  error
	jwksErr  error
	lastCall jwtauth.Claims
	token    string
	jwks     jose.JSONWebKeySet
}

func (f *fakeTokenSigner) Sign(ctx context.Context, claims jwtauth.Claims) (string, error) {
	if f.signErr != nil {
		return "", f.signErr
	}
	f.lastCall = claims
	if f.token != "" {
		return f.token, nil
	}
	return "fake-signed-token", nil
}

func (f *fakeTokenSigner) PublicJWKS(ctx context.Context) (jose.JSONWebKeySet, error) {
	if f.jwksErr != nil {
		return jose.JSONWebKeySet{}, f.jwksErr
	}
	return f.jwks, nil
}

// fakeOPAClient backs requireAdminActor's tests without loading the real
// Rego bundle — allow/decisionErr let a test force either branch of
// Execute's fail-closed check; lastActor records the actor Decision was
// last called with, so a test can assert the OPA input requireAdminActor
// built. Mirrors task-service's own fakeOPAClient shape.
type fakeOPAClient struct {
	allow       bool
	decisionErr error
	lastActor   domain.User
	called      bool
}

func (f *fakeOPAClient) Decision(ctx context.Context, actor domain.User) (bool, error) {
	f.called = true
	f.lastActor = actor
	if f.decisionErr != nil {
		return false, f.decisionErr
	}
	return f.allow, nil
}
