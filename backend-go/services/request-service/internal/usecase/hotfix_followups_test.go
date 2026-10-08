package usecase

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func completedHotfix(s *lcStore) domain.Request {
	return s.seed(func(r *domain.Request) {
		r.Type, r.Status, r.Title = domain.RequestTypeHotfix, domain.RequestStatusCompleted, "login 500"
	})
}

func followUpsOf(t *testing.T, req domain.Request) []domain.FollowUp {
	t.Helper()
	fus, err := domain.NewPolicyRegistry(domain.PolicyDeps{}).PolicyFor(domain.RequestTypeHotfix).OnCompleted(req)
	if err != nil {
		t.Fatal(err)
	}
	return fus
}

func TestHotfixOnCompleted_TwoFollowUps_Idempotent(t *testing.T) {
	s := newLcStore()
	parent := completedHotfix(s)
	uc := &SpawnFollowUps{Spawn: newLcSpawn(s)}
	for i := 0; i < 2; i++ {
		if err := uc.ExecuteFollowUps(lcCtx(), parent, followUpsOf(t, parent)); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if len(s.links) != 2 {
		t.Fatalf("two follow-up requests, once, got %d links", len(s.links))
	}
	types := map[string]bool{}
	for _, l := range s.links {
		if l.Reason != domain.LinkReasonFollowupHotfix || l.ParentRequestID != parent.ID {
			t.Errorf("link %+v", l)
		}
		types[string(s.requests[l.ChildRequestID].Title)] = true
	}
	if !types["Nguyên nhân gốc và test hồi quy cho login 500"] || !types["Review sau hotfix: login 500"] {
		t.Fatalf("children: %v", types)
	}
}

func TestExecuteFollowUps_AlreadyExists_Success(t *testing.T) {
	s := newLcStore()
	parent := completedHotfix(s)
	uc := &SpawnFollowUps{Spawn: newLcSpawn(s)}
	fus := followUpsOf(t, parent)
	if err := uc.ExecuteFollowUps(lcCtx(), parent, fus[:1]); err != nil {
		t.Fatal(err)
	}
	if err := uc.ExecuteFollowUps(lcCtx(), parent, fus); err != nil {
		t.Fatalf("a follow-up that already exists is success: %v", err)
	}
	if len(s.links) != 2 {
		t.Fatalf("got %d links", len(s.links))
	}
}

func TestExecuteFollowUps_TransientError_Retried(t *testing.T) {
	s := newLcStore()
	parent := completedHotfix(s)
	s.failOutbox = errors.New("database is down")
	uc := &SpawnFollowUps{Spawn: newLcSpawn(s)}
	if err := uc.ExecuteFollowUps(lcCtx(), parent, followUpsOf(t, parent)); err == nil {
		t.Fatal("an infrastructure error must be returned so the caller can retry")
	}
	s.failOutbox = nil
	if err := uc.ExecuteFollowUps(lcCtx(), parent, followUpsOf(t, parent)); err != nil || len(s.links) != 2 {
		t.Fatalf("the retry succeeds: %v links=%d", err, len(s.links))
	}
}

func TestExecuteFollowUps_PermanentError_DoesNotBlockCompletion(t *testing.T) {
	s := newLcStore()
	// A cancelled parent can never spawn children: dropped, not returned.
	parent := s.seed(func(r *domain.Request) { r.Type, r.Status = domain.RequestTypeHotfix, domain.RequestStatusCancelled })
	uc := &SpawnFollowUps{Spawn: newLcSpawn(s)}
	if err := uc.ExecuteFollowUps(lcCtx(), parent, followUpsOf(t, parent)); err != nil {
		t.Fatalf("a permanent failure must not surface: %v", err)
	}
	if len(s.links) != 0 {
		t.Fatalf("no child expected, got %d", len(s.links))
	}
}

func TestIsPermanentFollowUpError(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"invalid":     {apperrors.New(apperrors.KindInvalidArgument, "X", "x", nil), true},
		"precond":     {apperrors.New(apperrors.KindFailedPrecondition, "X", "x", nil), true},
		"not found":   {apperrors.New(apperrors.KindNotFound, "X", "x", nil), true},
		"internal":    {apperrors.New(apperrors.KindInternal, "X", "x", nil), false},
		"unavailable": {apperrors.New(apperrors.KindUnavailable, "X", "x", nil), false},
		"plain":       {errors.New("boom"), false},
	}
	for name, tc := range cases {
		if got := isPermanentFollowUpError(tc.err); got != tc.want {
			t.Errorf("%s: got %v", name, got)
		}
	}
}

func TestExecutionCompletion_HotfixFollowUpsUseRealSpawn(t *testing.T) {
	r := newExRig(t)
	r.evaluate.FollowUps = &SpawnFollowUps{Spawn: newLcSpawn(r.store)}
	req := r.request(domain.RequestTypeHotfix, domain.RequestSizeS, domain.RequestStatusExecuting)
	task := r.leaf(req, domain.TaskView{}, "fix", domain.TaskStatusOpen)
	advance(t, r, req, "")
	r.finishRun(req, task, true, "")
	if len(r.store.links) != 2 {
		t.Fatalf("completing a hotfix files its two follow-up requests, got %d", len(r.store.links))
	}
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil || len(r.store.links) != 2 {
		t.Fatalf("a repeat changes nothing: %v %d", err, len(r.store.links))
	}
}
