package usecase

import (
	"context"
	"errors"
	"time"

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
