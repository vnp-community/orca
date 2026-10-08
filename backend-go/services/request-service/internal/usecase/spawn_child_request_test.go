package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func newLcSpawn(s *lcStore) *SpawnChildRequest {
	return NewSpawnChildRequest(s, lcLinks{s}, NewIdempotentChildCreator(s, lcIdempotency{s}, s), s)
}

func spikeParent(s *lcStore) domain.Request {
	return s.seed(func(r *domain.Request) { r.Type, r.Status = domain.RequestTypeSpike, domain.RequestStatusCompleted })
}

func spawnIn(parent domain.Request) SpawnInput {
	return SpawnInput{
		ParentRequestID: parent.ID, LinkReason: domain.LinkReasonSpawnedBySpike, Title: "child", TypeHint: domain.RequestTypeTask,
		ClientRequestID: "c1", ActorID: "11111111-1111-1111-1111-111111111111", Provider: domain.SourceProviderManual,
	}
}

func TestSpawn_CreatesChildWithLinkAndPayload(t *testing.T) {
	s := newLcStore()
	parent := spikeParent(s)
	res, err := newLcSpawn(s).Execute(lcCtx(), spawnIn(parent))
	if err != nil || !res.Created {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	child := res.Child
	if child.Status != domain.RequestStatusNew || child.ProjectID != parent.ProjectID || child.Number == 0 {
		t.Fatalf("child %+v", child)
	}
	if len(s.links) != 1 || s.links[0].ParentRequestID != parent.ID || s.links[0].ChildRequestID != child.ID || s.links[0].Reason != domain.LinkReasonSpawnedBySpike {
		t.Fatalf("links %+v", s.links)
	}
	if len(s.events) != 1 || s.events[0].Subject != "orca.request.request.created" {
		t.Fatalf("events %v", s.subjects())
	}
	var p ChildRequestPayload
	_ = jsonUnmarshal(s.events[0].Payload, &p)
	if p.ParentRequestID != parent.ID || p.LinkReason != "spawned_by_spike" || p.TypeHint != "task" || p.RequestID != child.ID {
		t.Fatalf("payload %+v", p)
	}
}

func TestSpawn_IdempotentSameClientRequestID(t *testing.T) {
	s := newLcStore()
	parent := spikeParent(s)
	uc := newLcSpawn(s)
	first, _ := uc.Execute(lcCtx(), spawnIn(parent))
	second, err := uc.Execute(lcCtx(), spawnIn(parent))
	if err != nil || second.Created || second.Child.ID != first.Child.ID {
		t.Fatalf("second %+v %v", second, err)
	}
	if len(s.links) != 1 || len(s.events) != 1 {
		t.Fatalf("links=%d events=%d", len(s.links), len(s.events))
	}
}

func TestSpawn_SameClientIDDifferentParents_TwoChildren(t *testing.T) {
	s := newLcStore()
	uc := newLcSpawn(s)
	a, _ := uc.Execute(lcCtx(), spawnIn(spikeParent(s)))
	b, err := uc.Execute(lcCtx(), spawnIn(spikeParent(s)))
	if err != nil || !b.Created || a.Child.ID == b.Child.ID {
		t.Fatalf("a=%+v b=%+v err=%v", a, b, err)
	}
}

func TestSpawn_ChildLimit50(t *testing.T) {
	s := newLcStore()
	parent := spikeParent(s)
	for i := 0; i < domain.MaxChildrenPerParent; i++ {
		s.links = append(s.links, domain.RequestLink{ParentRequestID: parent.ID, ChildRequestID: s.seed(nil).ID, Reason: domain.LinkReasonSpawnedBySpike})
	}
	if _, err := newLcSpawn(s).Execute(lcCtx(), spawnIn(parent)); codeOf(err) != "REQUEST_CHILD_LIMIT" {
		t.Fatalf("got %v", err)
	}
}

// chain builds a root-to-leaf escalation chain of n requests and returns the leaf.
func chain(s *lcStore, n int) domain.Request {
	cur := s.seed(func(r *domain.Request) { r.Type, r.Status = domain.RequestTypeBug, domain.RequestStatusAnalyzing })
	for i := 1; i < n; i++ {
		next := s.seed(func(r *domain.Request) { r.Type, r.Status = domain.RequestTypeBug, domain.RequestStatusAnalyzing })
		s.links = append(s.links, domain.RequestLink{ParentRequestID: cur.ID, ChildRequestID: next.ID, Reason: domain.LinkReasonEscalation})
		cur = next
	}
	return cur
}

func TestSpawn_DepthFiveOK_SixExceeded(t *testing.T) {
	s := newLcStore()
	in := func(p domain.Request) SpawnInput {
		x := spawnIn(p)
		x.LinkReason, x.TypeHint = domain.LinkReasonEscalation, ""
		return x
	}
	if res, err := newLcSpawn(s).Execute(lcCtx(), in(chain(s, 4))); err != nil || !res.Created {
		t.Fatalf("child at level 5 must be allowed: %v", err)
	}
	if _, err := newLcSpawn(s).Execute(lcCtx(), in(chain(s, 5))); codeOf(err) != "REQUEST_CHILD_DEPTH_EXCEEDED" {
		t.Fatalf("child at level 6: %v", err)
	}
}

func TestSpawn_Validation(t *testing.T) {
	s := newLcStore()
	uc := newLcSpawn(s)
	parent := spikeParent(s)
	in := spawnIn(parent)
	in.ClientRequestID = ""
	if _, err := uc.Execute(lcCtx(), in); codeOf(err) != "REQUEST_CLIENT_REQUEST_ID_REQUIRED" {
		t.Errorf("client id: %v", err)
	}
	in = spawnIn(parent)
	in.ParentRequestID = "00000000-0000-0000-0000-000000000000"
	if _, err := uc.Execute(lcCtx(), in); codeOf(err) != "REQUEST_PARENT_NOT_FOUND" {
		t.Errorf("parent: %v", err)
	}
	in = spawnIn(parent)
	in.TypeHint = domain.RequestTypeBug
	if _, err := uc.Execute(lcCtx(), in); codeOf(err) != "REQUEST_CHILD_NOT_ALLOWED" {
		t.Errorf("hint: %v", err)
	}
	in = spawnIn(parent)
	in.Provider = domain.SourceProviderJira
	if _, err := uc.Execute(lcCtx(), in); codeOf(err) != "REQUEST_INVALID_SOURCE_PROVIDER" {
		t.Errorf("provider: %v", err)
	}
}

func TestSpawn_RollbackLeavesNoLinkNoChild(t *testing.T) {
	s := newLcStore()
	parent := spikeParent(s)
	before := len(s.requests)
	s.failOutbox = errBoom
	if _, err := newLcSpawn(s).Execute(lcCtx(), spawnIn(parent)); err == nil {
		t.Fatal("want error")
	}
	if len(s.links) != 0 || len(s.requests) != before || len(s.claims) != 0 {
		t.Fatalf("leaked: links=%d requests=%d claims=%d", len(s.links), len(s.requests), len(s.claims))
	}
}

func TestListRequestLinks_ParentsAndChildren(t *testing.T) {
	s := newLcStore()
	parent := spikeParent(s)
	res, _ := newLcSpawn(s).Execute(lcCtx(), spawnIn(parent))
	uc := NewListRequestLinks(s, lcLinks{s})
	v, err := uc.Execute(lcCtx(), parent.ID)
	if err != nil || len(v.Children) != 1 || len(v.Parents) != 0 {
		t.Fatalf("parent view %+v %v", v, err)
	}
	v, err = uc.Execute(lcCtx(), res.Child.ID)
	if err != nil || len(v.Parents) != 1 || len(v.Children) != 0 {
		t.Fatalf("child view %+v %v", v, err)
	}
	if _, err := uc.Execute(lcCtx(), "00000000-0000-0000-0000-000000000000"); codeOf(err) != "REQUEST_NOT_FOUND" {
		t.Fatalf("missing: %v", err)
	}
}
