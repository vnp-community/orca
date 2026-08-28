package usecase

import (
	"context"
	"errors"
	"sync"
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

// UpdateUser mirrors the postgres adapter's COALESCE semantics — a nil
// field leaves the existing value unchanged.
func (f *fakeUserRepository) UpdateUser(ctx context.Context, userID string, email, name *string, role *domain.Role) (domain.User, error) {
	u, ok := f.byID[userID]
	if !ok {
		return domain.User{}, ErrUserNotFound
	}
	oldEmail := u.Email
	if email != nil {
		u.Email = *email
	}
	if name != nil {
		u.Name = *name
	}
	if role != nil {
		u.Role = *role
	}
	f.byID[userID] = u
	delete(f.byEmail, oldEmail)
	f.byEmail[u.Email] = u
	return u, nil
}

// fakeSessionRepository is an in-memory SessionRepository. Guarded by mu
// because ValidateSession's touchBestEffort calls TouchLastSeen from a
// background goroutine (see validate_session.go) — a test asserting on
// touchCalls races the fake's map/slice mutations otherwise.
type fakeSessionRepository struct {
	mu     sync.Mutex
	byHash map[string]domain.Session

	touchCalls           []string
	touchErr             error
	deleteExpiredCutoffs []time.Time
	deleteExpiredErr     error
	listForTenantErr     error
	lastTenantID         string
	userEmails           map[string]string // userID -> email, for ListForTenant's join
}

func newFakeSessionRepository() *fakeSessionRepository {
	return &fakeSessionRepository{byHash: make(map[string]domain.Session)}
}

func (f *fakeSessionRepository) CreateSession(ctx context.Context, session domain.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byHash[session.TokenHash] = session
	return nil
}

func (f *fakeSessionRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byHash[tokenHash]
	if !ok {
		return domain.Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeSessionRepository) RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byHash[tokenHash]
	if !ok {
		return ErrSessionNotFound
	}
	s.RevokedAt = &revokedAt
	f.byHash[tokenHash] = s
	return nil
}

func (f *fakeSessionRepository) ListForUser(ctx context.Context, userID string) ([]domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Session
	for _, s := range f.byHash {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSessionRepository) RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) (int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int32
	for _, s := range f.byHash {
		if s.IsValid(now) {
			n++
		}
	}
	return n, nil
}

// touchErr/deleteExpiredErr/deleteExpiredN let a test force TouchLastSeen or
// DeleteExpiredBefore to fail, or control DeleteExpiredBefore's return
// count, without needing a real Postgres cutoff comparison against byHash.
func (f *fakeSessionRepository) TouchLastSeen(ctx context.Context, tokenHash string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.touchErr != nil {
		return f.touchErr
	}
	f.touchCalls = append(f.touchCalls, tokenHash)
	s, ok := f.byHash[tokenHash]
	if !ok {
		return nil // no-op for an unknown token hash — see interface doc comment
	}
	s.LastSeenAt = &now
	f.byHash[tokenHash] = s
	return nil
}

// touchCallCount reports how many times TouchLastSeen has been called so
// far — safe to poll from a test racing ValidateSession's background touch
// goroutine.
func (f *fakeSessionRepository) touchCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.touchCalls)
}

// ListForTenant mirrors the postgres adapter's tenant-scoping + email join
// (via userEmails, populated by seedActiveUser/users.seed's callers when a
// test needs it) and lastTenantID records what tenantID it was actually
// called with, so a test can assert ListSessions never leaks a
// caller-supplied tenant_id through.
func (f *fakeSessionRepository) ListForTenant(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.SessionWithUser, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastTenantID = tenantID
	if f.listForTenantErr != nil {
		return nil, "", f.listForTenantErr
	}
	var out []domain.SessionWithUser
	for _, s := range f.byHash {
		if s.TenantID == tenantID {
			out = append(out, domain.SessionWithUser{Session: s, UserEmail: f.userEmails[s.UserID]})
		}
	}
	return out, "", nil
}

func (f *fakeSessionRepository) DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteExpiredErr != nil {
		return 0, f.deleteExpiredErr
	}
	f.deleteExpiredCutoffs = append(f.deleteExpiredCutoffs, cutoff)
	var n int64
	for hash, s := range f.byHash {
		expired := s.ExpiresAt.Before(cutoff)
		revoked := s.RevokedAt != nil && s.RevokedAt.Before(cutoff)
		if expired || revoked {
			delete(f.byHash, hash)
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
	entries   []domain.AuditEntry
	appendErr error // used by handle_ssh_connected_event_test.go to simulate a write failure
}

func (f *fakeAuditRepository) Append(ctx context.Context, entry domain.AuditEntry) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeAuditRepository) Query(ctx context.Context, filter AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	var out []domain.AuditEntry
	for _, e := range f.entries {
		if e.TenantID != filter.TenantID {
			continue
		}
		if !filter.Since.IsZero() && e.OccurredAt.Before(filter.Since) {
			continue
		}
		if !filter.To.IsZero() && e.OccurredAt.After(filter.To) {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.ActorID != "" && e.ActorID != filter.ActorID {
			continue
		}
		out = append(out, e)
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

// fakePairingSessionRepository is an in-memory PairingSessionRepository.
type fakePairingSessionRepository struct {
	byID map[string]domain.PairingSession

	saveErr error
	// getAndConsumeErr, if set, is returned by GetAndConsume regardless of
	// map state — lets a test simulate an already-consumed/never-existed
	// token without needing two real calls.
	getAndConsumeErr error
}

func newFakePairingSessionRepository() *fakePairingSessionRepository {
	return &fakePairingSessionRepository{byID: make(map[string]domain.PairingSession)}
}

func (f *fakePairingSessionRepository) Save(ctx context.Context, session domain.PairingSession) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.byID[session.ID] = session
	return nil
}

func (f *fakePairingSessionRepository) GetAndConsume(ctx context.Context, id string) (domain.PairingSession, error) {
	if f.getAndConsumeErr != nil {
		return domain.PairingSession{}, f.getAndConsumeErr
	}
	session, ok := f.byID[id]
	if !ok || session.Consumed() {
		return domain.PairingSession{}, domain.ErrPairingTokenNotFound
	}
	now := session.ExpiresAt // arbitrary consumed-at stamp; tests don't assert on it
	session.ConsumedAt = &now
	f.byID[id] = session
	return session, nil
}

// fakePairedDeviceRepository is an in-memory PairedDeviceRepository.
type fakePairedDeviceRepository struct {
	byID map[string]domain.PairedDevice

	saveErr        error
	countActiveErr error
	revokeErr      error
	touchErr       error
	touchCalled    bool
}

func newFakePairedDeviceRepository() *fakePairedDeviceRepository {
	return &fakePairedDeviceRepository{byID: make(map[string]domain.PairedDevice)}
}

func (f *fakePairedDeviceRepository) Save(ctx context.Context, device domain.PairedDevice) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.byID[device.ID] = device
	return nil
}

func (f *fakePairedDeviceRepository) CountActive(ctx context.Context, tenantID, userID string) (int, error) {
	if f.countActiveErr != nil {
		return 0, f.countActiveErr
	}
	n := 0
	for _, d := range f.byID {
		if d.TenantID == tenantID && d.UserID == userID && d.Status == domain.DeviceActive {
			n++
		}
	}
	return n, nil
}

func (f *fakePairedDeviceRepository) Get(ctx context.Context, id string) (domain.PairedDevice, error) {
	d, ok := f.byID[id]
	if !ok {
		return domain.PairedDevice{}, domain.ErrDeviceNotFound
	}
	return d, nil
}

func (f *fakePairedDeviceRepository) List(ctx context.Context, tenantID, userID string) ([]domain.PairedDevice, error) {
	var out []domain.PairedDevice
	for _, d := range f.byID {
		if d.TenantID == tenantID && d.UserID == userID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakePairedDeviceRepository) RevokeAndWipeSecret(ctx context.Context, id string) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	d, ok := f.byID[id]
	if !ok {
		return domain.ErrDeviceNotFound
	}
	d.Status = domain.DeviceRevoked
	d.SharedSecretCiphertext = nil
	d.VaultKeyRef = ""
	f.byID[id] = d
	return nil
}

func (f *fakePairedDeviceRepository) Touch(ctx context.Context, id string, now time.Time) error {
	f.touchCalled = true
	if f.touchErr != nil {
		return f.touchErr
	}
	d, ok := f.byID[id]
	if !ok {
		return domain.ErrDeviceNotFound
	}
	d.LastUsedAt = now
	f.byID[id] = d
	return nil
}

// fakeDeviceKeyExchanger is an in-memory DeviceKeyExchanger — deterministic,
// no real X25519 math, so tests can assert on exactly what
// InitiateDevicePairing/CompleteDevicePairing pass through it.
type fakeDeviceKeyExchanger struct {
	pub, priv []byte
	shared    []byte

	genErr    error
	sharedErr error
}

func (f *fakeDeviceKeyExchanger) GenerateEphemeralKeypair() (pub, priv []byte, err error) {
	if f.genErr != nil {
		return nil, nil, f.genErr
	}
	if f.pub == nil {
		f.pub = []byte("fake-desktop-pub-key-32-bytes!!")
	}
	if f.priv == nil {
		f.priv = []byte("fake-desktop-priv-key-32-bytes!")
	}
	return f.pub, f.priv, nil
}

func (f *fakeDeviceKeyExchanger) SharedSecret(priv, peerPub []byte) ([]byte, error) {
	if f.sharedErr != nil {
		return nil, f.sharedErr
	}
	if f.shared == nil {
		f.shared = []byte("fake-shared-secret-32-bytes!!!!")
	}
	return f.shared, nil
}

// fakeSharedSecretSealer is an in-memory SharedSecretSealer — Encrypt is a
// no-op passthrough tagged with a fixed key ref, Decrypt reverses it. Real
// enough for usecase tests, no Vault Transit round trip.
type fakeSharedSecretSealer struct {
	encryptErr error
	decryptErr error
	// decryptCalled lets a test assert Decrypt was (or wasn't) invoked —
	// e.g. ResolveDeviceSharedSecret must never call it once a device's
	// ciphertext has been wiped (BR-MB-04's "no oracle" guarantee).
	decryptCalled bool
}

func (f *fakeSharedSecretSealer) Encrypt(ctx context.Context, plaintext []byte) ([]byte, string, error) {
	if f.encryptErr != nil {
		return nil, "", f.encryptErr
	}
	sealed := append([]byte("sealed:"), plaintext...)
	return sealed, "fake-key-ref", nil
}

func (f *fakeSharedSecretSealer) Decrypt(ctx context.Context, ciphertext []byte, keyRef string) ([]byte, error) {
	f.decryptCalled = true
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	const prefix = "sealed:"
	if len(ciphertext) < len(prefix) {
		return nil, errors.New("fakeSharedSecretSealer: not sealed by this fake")
	}
	return ciphertext[len(prefix):], nil
}
