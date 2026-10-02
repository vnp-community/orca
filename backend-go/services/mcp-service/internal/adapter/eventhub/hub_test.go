package eventhub

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type fakeApprovals map[string]domain.Approval

func (f fakeApprovals) GetApproval(_ context.Context, tenantID, id string) (domain.Approval, error) {
	a, ok := f[id]
	if !ok || a.TenantID != tenantID {
		return domain.Approval{}, domain.ErrNotFound()
	}
	return a, nil
}

func ev(tenantID string, payload map[string]any, at time.Time) eventbus.Event {
	b, _ := json.Marshal(payload)
	return eventbus.Event{ID: "e", TenantID: tenantID, OccurredAt: at, Version: 1, Payload: b}
}

func newHub(t *testing.T, a fakeApprovals) (*Hub, *[]string) {
	t.Helper()
	var invalidated []string
	h := New(a, func(tenantID string) { invalidated = append(invalidated, tenantID) })
	return h, &invalidated
}

func TestApprovalEventsReachOnlyTheOwner(t *testing.T) {
	a := domain.Approval{ID: "a1", TenantID: "t1", UserID: "owner", ToolName: "terminal_send", Status: "pending", ExpiresAt: time.Now().Add(time.Minute), ParamsHash: "sha256:x", ArgsPreview: "{}"}
	h, _ := newHub(t, fakeApprovals{"a1": a})
	owner, cancelO, _ := h.Subscribe("t1", "owner")
	other, cancelX, _ := h.Subscribe("t1", "someone-else")
	otherTenant, cancelT, _ := h.Subscribe("t2", "owner")
	defer cancelO()
	defer cancelX()
	defer cancelT()
	now := time.Now()
	if err := h.HandleApprovalRequested(context.Background(), ev("t1", map[string]any{"approval_id": "a1"}, now)); err != nil {
		t.Fatal(err)
	}
	_ = h.HandleApprovalResolved(context.Background(), ev("t1", map[string]any{"approval_id": "a1", "user_id": "owner", "status": "approved"}, now))
	got := <-owner
	if got.GetType() != "approval.requested" || got.GetApproval().GetParamsHash() != "sha256:x" {
		t.Fatalf("%+v", got)
	}
	if got := <-owner; got.GetType() != "approval.resolved" || got.GetId() != "a1" || got.GetStatus() != "approved" {
		t.Fatalf("%+v", got)
	}
	select {
	case e := <-other:
		t.Fatalf("another user received %+v", e)
	case e := <-otherTenant:
		t.Fatalf("another tenant received %+v", e)
	default:
	}
}

func TestKillSwitchBroadcastsTenantWideAndInvalidates(t *testing.T) {
	h, inv := newHub(t, nil)
	a, c1, _ := h.Subscribe("t1", "u1")
	b, c2, _ := h.Subscribe("t1", "u2")
	defer c1()
	defer c2()
	_ = h.HandleKillSwitchChanged(context.Background(), ev("t1", map[string]any{"scope": "tenant", "active": true, "reason": "incident"}, time.Now()))
	_ = h.HandleKillSwitchChanged(context.Background(), ev("t1", map[string]any{"scope": "client", "active": true, "reason": "x"}, time.Now()))
	e1, e2 := <-a, <-b
	if e1.GetType() != "killswitch.changed" || !e1.GetActive() || e2.GetReason() != "incident" {
		t.Fatalf("%+v %+v", e1, e2)
	}
	select {
	case e := <-a:
		t.Fatalf("non-tenant switch must not be broadcast: %+v", e)
	default:
	}
	if len(*inv) != 2 {
		t.Fatalf("caches must be invalidated for every kill event: %v", *inv)
	}
}

func TestStaleReplayIsIgnoredPoisonIsDroppedAndLimitsHold(t *testing.T) {
	h, _ := newHub(t, fakeApprovals{})
	ch, cancel, _ := h.Subscribe("t1", "u1")
	defer cancel()
	old := time.Now().Add(-time.Hour)
	_ = h.HandleApprovalResolved(context.Background(), ev("t1", map[string]any{"approval_id": "a", "user_id": "u1", "status": "denied"}, old))
	if err := h.HandleApprovalResolved(context.Background(), eventbus.Event{TenantID: "t1", OccurredAt: time.Now(), Payload: []byte("{not json")}); err != nil {
		t.Fatalf("poison payload must be acked, got %v", err)
	}
	select {
	case e := <-ch:
		t.Fatalf("unexpected %+v", e)
	default:
	}
	var cancels []func()
	for i := 0; i < MaxStreamsPerUser-1; i++ {
		_, c, err := h.Subscribe("t1", "u1")
		if err != nil {
			t.Fatal(err)
		}
		cancels = append(cancels, c)
	}
	if _, _, err := h.Subscribe("t1", "u1"); err == nil {
		t.Fatal("the sixth stream must be refused")
	}
	for _, c := range cancels {
		c()
	}
}

func TestSlowSubscriberIsClosedNotBlocked(t *testing.T) {
	h, _ := newHub(t, nil)
	ch, cancel, _ := h.Subscribe("t1", "u1")
	defer cancel()
	for i := 0; i < subscriberBuffer+5; i++ {
		_ = h.HandleGrantRevoked(context.Background(), ev("t1", map[string]any{"grant_id": "g", "user_id": "u1"}, time.Now()))
	}
	n := 0
	for range ch {
		n++
	}
	if n != subscriberBuffer {
		t.Fatalf("buffer drained %d events then the stream must close", n)
	}
}
