package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type sessionTestClock struct{ t time.Time }

func (c sessionTestClock) Now() time.Time { return c.t }

type fakeSessionRepo struct {
	rows   map[string]domain.Session
	events []domain.OutboxRecord
	opens  int
}

func (f *fakeSessionRepo) CreateSession(_ context.Context, s domain.Session) (domain.Session, error) {
	f.rows[s.ID] = s
	return s, nil
}
func (f *fakeSessionRepo) GetSessionBySecretHash(_ context.Context, _ string, h []byte) (domain.Session, error) {
	for _, s := range f.rows {
		if string(s.SecretHash) == string(h) {
			return s, nil
		}
	}
	return domain.Session{}, domain.ErrNotFound()
}
func (f *fakeSessionRepo) GetSession(_ context.Context, _, id string) (domain.Session, error) {
	if s, ok := f.rows[id]; ok {
		return s, nil
	}
	return domain.Session{}, domain.ErrNotFound()
}
func (f *fakeSessionRepo) TouchSession(context.Context, string, string, bool, int64, string, time.Time) (string, error) {
	return domain.SessionReady, nil
}
func (f *fakeSessionRepo) CloseSession(_ context.Context, _, id, reason string, _ time.Time, ev []domain.OutboxRecord) (bool, error) {
	s := f.rows[id]
	s.State, s.CloseReason = domain.SessionClosed, reason
	f.rows[id] = s
	f.events = append(f.events, ev...)
	return true, nil
}
func (f *fakeSessionRepo) ListSessions(_ context.Context, _, user string, _ time.Time) ([]domain.Session, error) {
	var out []domain.Session
	for _, s := range f.rows {
		if s.State != domain.SessionClosed && (user == "" || s.UserID == user) {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeSessionRepo) OpenStream(context.Context, string, string, string, string, string, string, int, int, time.Time) error {
	f.opens++
	return nil
}
func (f *fakeSessionRepo) HeartbeatStream(context.Context, string, string, time.Time) error {
	return nil
}
func (f *fakeSessionRepo) CloseStream(context.Context, string, string) error { return nil }
func (f *fakeSessionRepo) ReapIdle(_ context.Context, _ time.Time, _ int, _ time.Time, mk func(domain.ClosedSession) (domain.OutboxRecord, error)) ([]domain.ClosedSession, error) {
	return nil, nil
}

func sessCtx(user, role string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), "t1")
	ctx = tenant.WithUserID(ctx, user)
	return tenant.WithRole(ctx, role)
}

func TestSessions_OwnershipAndAdmin(t *testing.T) {
	repo := &fakeSessionRepo{rows: map[string]domain.Session{}}
	uc := NewSessions(repo, sessionTestClock{time.Unix(1000, 0).UTC()})
	hash := func(b byte) []byte { h := make([]byte, 32); h[0] = b; return h }

	a, err := uc.Create(sessCtx("alice", "user"), CreateSessionInput{SecretHash: hash(1), ProtoVer: "2025-06-18", ClientName: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Create(sessCtx("alice", "user"), CreateSessionInput{SecretHash: []byte("short"), ProtoVer: "x"}); err == nil {
		t.Fatal("short hash must be rejected")
	}
	b, _ := uc.Create(sessCtx("bob", "user"), CreateSessionInput{SecretHash: hash(2), ProtoVer: "2025-06-18"})

	if rows, _ := uc.List(sessCtx("alice", "user"), false); len(rows) != 1 || rows[0].ID != a.ID {
		t.Fatalf("own list: %+v", rows)
	}
	if _, err := uc.List(sessCtx("alice", "user"), true); err == nil {
		t.Fatal("non-admin must not list the tenant")
	}
	if rows, _ := uc.List(sessCtx("root", "admin"), true); len(rows) != 2 {
		t.Fatalf("admin list: %d", len(rows))
	}

	// Closing someone else's session is indistinguishable from an unknown one.
	_, err = uc.Close(sessCtx("alice", "user"), CloseSessionInput{SessionID: b.ID, Reason: domain.CloseReasonUser})
	var ae *apperrors.AppError
	if !sessAppErr(err, &ae) || ae.Code != domain.CodeNotFound {
		t.Fatalf("want MCP_NOT_FOUND, got %v", err)
	}
	if _, err := uc.Close(sessCtx("alice", "user"), CloseSessionInput{SessionID: a.ID, Reason: domain.CloseReasonUser}); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 1 || repo.events[0].Subject != domain.SubjectSessionClosed {
		t.Fatalf("close must enqueue orca.mcp.session.closed: %+v", repo.events)
	}
	// Idempotent: closing a closed session does not enqueue again.
	if _, err := uc.Close(sessCtx("alice", "user"), CloseSessionInput{SessionID: a.ID, Reason: domain.CloseReasonUser}); err != nil || len(repo.events) != 1 {
		t.Fatalf("idempotent close: %v events=%d", err, len(repo.events))
	}
	if _, err := uc.Close(sessCtx("root", "admin"), CloseSessionInput{SessionID: b.ID, Reason: domain.CloseReasonAdmin}); err != nil {
		t.Fatalf("admin close: %v", err)
	}
	if _, err := uc.Close(sessCtx("root", "admin"), CloseSessionInput{SessionID: b.ID, Reason: "whatever"}); err == nil {
		t.Fatal("unknown reason must be rejected")
	}
}

func sessAppErr(err error, target **apperrors.AppError) bool { return errors.As(err, target) }
