package grpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/adapter/bcrypt"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeOPAClient is a minimal in-memory usecase.OPAClient — every
// admin-console usecase's requireAdminActor gate needs one.
type fakeOPAClient struct{ allow bool }

func (f *fakeOPAClient) Decision(ctx context.Context, actor domain.User) (bool, error) {
	return f.allow, nil
}

func withActor(ctx context.Context, tenantID, userID string) context.Context {
	ctx = tenant.WithTenantID(ctx, tenantID)
	return tenant.WithUserID(ctx, userID)
}

// fakeUserRepository is a minimal in-memory usecase.UserRepository, local
// to this adapter test — the usecase package's own test fakes are
// unexported and not reusable from here.
type fakeUserRepository struct {
	byEmail map[string]domain.User
	hashes  map[string]string
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{byEmail: map[string]domain.User{}, hashes: map[string]string{}}
}

func (r *fakeUserRepository) seed(u domain.User, hash string) {
	r.byEmail[u.Email] = u
	r.hashes[u.Email] = hash
}

func (r *fakeUserRepository) CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	r.seed(user, passwordHash)
	return user, nil
}
func (r *fakeUserRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	u, ok := r.byEmail[email]
	if !ok {
		return domain.User{}, "", usecase.ErrUserNotFound
	}
	return u, r.hashes[email], nil
}
func (r *fakeUserRepository) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	for _, u := range r.byEmail {
		if u.ID == userID {
			return u, nil
		}
	}
	return domain.User{}, usecase.ErrUserNotFound
}
func (r *fakeUserRepository) ListUsers(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.User, string, error) {
	return nil, "", nil
}
func (r *fakeUserRepository) UpdateUserRole(ctx context.Context, userID string, role domain.Role) (domain.User, error) {
	return domain.User{}, nil
}
func (r *fakeUserRepository) SetActive(ctx context.Context, userID string, active bool) error {
	return nil
}
func (r *fakeUserRepository) HasAnyUsers(ctx context.Context) (bool, error) { return true, nil }
func (r *fakeUserRepository) Count(ctx context.Context) (int32, error)     { return int32(len(r.byEmail)), nil }
func (r *fakeUserRepository) UpdateUser(ctx context.Context, userID string, email, name *string, role *domain.Role) (domain.User, error) {
	u, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	oldEmail := u.Email
	hash := r.hashes[oldEmail]
	if email != nil {
		u.Email = *email
	}
	if name != nil {
		u.Name = *name
	}
	if role != nil {
		u.Role = *role
	}
	if u.Email != oldEmail {
		delete(r.byEmail, oldEmail)
		delete(r.hashes, oldEmail)
	}
	r.seed(u, hash)
	return u, nil
}

// fakeSessionRepository is a minimal in-memory usecase.SessionRepository.
type fakeSessionRepository struct {
	byHash     map[string]domain.Session
	userEmails map[string]string // userID -> email, for ListForTenant's join
}

func newFakeSessionRepository() *fakeSessionRepository {
	return &fakeSessionRepository{byHash: map[string]domain.Session{}}
}
func (r *fakeSessionRepository) CreateSession(ctx context.Context, session domain.Session) error {
	r.byHash[session.TokenHash] = session
	return nil
}
func (r *fakeSessionRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	s, ok := r.byHash[tokenHash]
	if !ok {
		return domain.Session{}, usecase.ErrSessionNotFound
	}
	return s, nil
}
func (r *fakeSessionRepository) RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	return nil
}
func (r *fakeSessionRepository) ListForUser(ctx context.Context, userID string) ([]domain.Session, error) {
	var out []domain.Session
	for _, s := range r.byHash {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (r *fakeSessionRepository) RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) (int32, error) {
	return 0, nil
}
func (r *fakeSessionRepository) CountActive(ctx context.Context, now time.Time) (int32, error) {
	return int32(len(r.byHash)), nil
}
func (r *fakeSessionRepository) TouchLastSeen(ctx context.Context, tokenHash string, now time.Time) error {
	return nil
}
func (r *fakeSessionRepository) DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeSessionRepository) ListForTenant(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.SessionWithUser, string, error) {
	var out []domain.SessionWithUser
	for _, s := range r.byHash {
		if s.TenantID == tenantID {
			out = append(out, domain.SessionWithUser{Session: s, UserEmail: r.userEmails[s.UserID]})
		}
	}
	return out, "", nil
}

// fakeAuditRepository is a minimal in-memory usecase.AuditRepository that
// records every entry appended, so a test can assert what Login wrote.
type fakeAuditRepository struct {
	entries []domain.AuditEntry
}

func (r *fakeAuditRepository) Append(ctx context.Context, entry domain.AuditEntry) error {
	r.entries = append(r.entries, entry)
	return nil
}
func (r *fakeAuditRepository) Query(ctx context.Context, filter usecase.AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	return nil, "", nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestServer_Login(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	audit := &fakeAuditRepository{}
	hasher := bcrypt.New(bcrypt.MinCost)
	clock := fixedClock{now: time.Now()}

	u, err := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, clock.now)
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	hash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	users.seed(u, hash)

	login := usecase.NewLogin(users, sessions, audit, hasher, clock, time.Hour)
	s := New(login, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := s.Login(context.Background(), &authv1.LoginRequest{
		Email:     "alice@example.com",
		Password:  "correct-password",
		Ip:        "203.0.113.7",
		UserAgent: "test-agent/1.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSessionToken() == "" {
		t.Fatal("expected a non-empty session token")
	}
	if resp.GetUser().GetId() != "u1" {
		t.Errorf("expected user u1, got %s", resp.GetUser().GetId())
	}
}

func TestServer_Login_WrongPasswordWritesFailureAudit(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	audit := &fakeAuditRepository{}
	hasher := bcrypt.New(bcrypt.MinCost)
	clock := fixedClock{now: time.Now()}

	u, err := domain.NewUser("u1", "t1", "alice@example.com", "Alice", domain.RoleUser, true, clock.now)
	if err != nil {
		t.Fatalf("building user: %v", err)
	}
	hash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	users.seed(u, hash)

	login := usecase.NewLogin(users, sessions, audit, hasher, clock, time.Hour)
	s := New(login, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	_, err = s.Login(context.Background(), &authv1.LoginRequest{
		Email:     "alice@example.com",
		Password:  "wrong-password",
		Ip:        "203.0.113.7",
		UserAgent: "test-agent/1.0",
	})
	if err == nil {
		t.Fatal("expected an error for a wrong password")
	}
	if len(audit.entries) != 1 || audit.entries[0].Action != "login.fail" {
		t.Errorf("expected one login.fail audit entry, got %+v", audit.entries)
	}
}

func TestServer_ListSessionsForUser(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	opa := &fakeOPAClient{allow: true}

	admin, err := domain.NewUser("admin1", "t1", "admin@example.com", "Admin", domain.RoleAdmin, true, time.Now())
	if err != nil {
		t.Fatalf("building admin: %v", err)
	}
	users.seed(admin, "hash")
	member, err := domain.NewUser("u2", "t1", "member@example.com", "Member", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building member: %v", err)
	}
	users.seed(member, "hash")

	now := time.Now().UTC().Truncate(time.Second)
	touched, err := domain.NewSession("hash-touched", "u2", "t1", now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building touched session: %v", err)
	}
	touched = touched.WithClientInfo("203.0.113.7", "test-agent/1.0")
	lastSeen := now.Add(-5 * time.Minute)
	touched.LastSeenAt = &lastSeen
	if err := sessions.CreateSession(context.Background(), touched); err != nil {
		t.Fatalf("create touched session: %v", err)
	}

	untouched, err := domain.NewSession("hash-untouched", "u2", "t1", now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building untouched session: %v", err)
	}
	if err := sessions.CreateSession(context.Background(), untouched); err != nil {
		t.Fatalf("create untouched session: %v", err)
	}

	listSessionsForUser := usecase.NewListSessionsForUser(users, sessions, opa)
	s := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, listSessionsForUser, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	ctx := withActor(context.Background(), "t1", "admin1")
	resp, err := s.ListSessionsForUser(ctx, &authv1.ListSessionsForUserRequest{UserId: "u2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetSessions()) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(resp.GetSessions()))
	}

	var gotTouched, gotUntouched *authv1.Session
	for _, sess := range resp.GetSessions() {
		switch sess.GetId() {
		case "hash-touched":
			gotTouched = sess
		case "hash-untouched":
			gotUntouched = sess
		}
	}
	if gotTouched == nil || gotUntouched == nil {
		t.Fatalf("expected both sessions in response, got %+v", resp.GetSessions())
	}

	if gotTouched.GetIp() != "203.0.113.7" || gotTouched.GetUserAgent() != "test-agent/1.0" {
		t.Errorf("expected IP/UserAgent to carry through, got ip=%q ua=%q", gotTouched.GetIp(), gotTouched.GetUserAgent())
	}
	if gotTouched.GetLastSeenAt() == nil || !gotTouched.GetLastSeenAt().AsTime().Equal(lastSeen) {
		t.Errorf("expected LastSeenAt = %v, got %v", lastSeen, gotTouched.GetLastSeenAt())
	}

	if gotUntouched.GetLastSeenAt() != nil {
		t.Errorf("expected a never-touched session's LastSeenAt to be unset, got %v", gotUntouched.GetLastSeenAt())
	}
}

func TestServer_ListSessions(t *testing.T) {
	users := newFakeUserRepository()
	sessions := newFakeSessionRepository()
	sessions.userEmails = map[string]string{"u2": "member@example.com"}
	opa := &fakeOPAClient{allow: true}

	admin, err := domain.NewUser("admin1", "t1", "admin@example.com", "Admin", domain.RoleAdmin, true, time.Now())
	if err != nil {
		t.Fatalf("building admin: %v", err)
	}
	users.seed(admin, "hash")

	now := time.Now().UTC().Truncate(time.Second)
	session, err := domain.NewSession("hash1", "u2", "t1", now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("building session: %v", err)
	}
	if err := sessions.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	listSessions := usecase.NewListSessions(users, sessions, opa)
	s := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, listSessions, nil, nil, nil, nil, nil, nil)

	ctx := withActor(context.Background(), "t1", "admin1")
	resp, err := s.ListSessions(ctx, &authv1.ListSessionsRequest{PageSize: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetSessions()) != 1 {
		t.Fatalf("expected 1 session, got %d", len(resp.GetSessions()))
	}
	if resp.GetSessions()[0].GetUserEmail() != "member@example.com" {
		t.Errorf("expected the joined user email, got %q", resp.GetSessions()[0].GetUserEmail())
	}
	if resp.GetSessions()[0].GetSession().GetId() != "hash1" {
		t.Errorf("expected the session to carry through, got %+v", resp.GetSessions()[0].GetSession())
	}
}

func TestServer_UpdateUser_PartialRequestOnlySetsGivenFields(t *testing.T) {
	users := newFakeUserRepository()
	admin, err := domain.NewUser("admin1", "t1", "admin@example.com", "Admin", domain.RoleAdmin, true, time.Now())
	if err != nil {
		t.Fatalf("building admin: %v", err)
	}
	users.seed(admin, "hash")
	member, err := domain.NewUser("u2", "t1", "member@example.com", "Member", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatalf("building member: %v", err)
	}
	users.seed(member, "hash")

	updateUser := usecase.NewUpdateUser(users, &fakeAuditRepository{}, fixedClock{now: time.Now()}, &fakeOPAClient{allow: true})
	s := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, updateUser, nil, nil, nil, nil, nil)

	ctx := withActor(context.Background(), "t1", "admin1")
	resp, err := s.UpdateUser(ctx, &authv1.UpdateUserRequest{
		UserId: "u2",
		Email:  wrapperspb.String("member-new@example.com"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetUser().GetEmail() != "member-new@example.com" {
		t.Errorf("expected email to be updated, got %q", resp.GetUser().GetEmail())
	}
	if resp.GetUser().GetName() != "Member" {
		t.Errorf("expected name to be left unchanged (Name/Role omitted from request), got %q", resp.GetUser().GetName())
	}
	if resp.GetUser().GetRole() != authv1.Role_ROLE_USER {
		t.Errorf("expected role to be left unchanged, got %v", resp.GetUser().GetRole())
	}
}
