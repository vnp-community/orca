package eventbus

import (
	"context"
	"encoding/json"
	"testing"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type memOutbox struct{ recs []outbox.Record }

func (m *memOutbox) FetchUnpublished(context.Context, int) ([]outbox.Record, error) {
	return append([]outbox.Record(nil), m.recs...), nil
}
func (m *memOutbox) MarkPublished(context.Context, []string) error { return nil }

type memApprovers struct{ err error }

func (m memApprovers) InsertSnapshot(context.Context, string, string, []domain.Principal) error {
	return nil
}
func (m memApprovers) ListForApproval(ctx context.Context, tenantID, _ string) ([]domain.Principal, error) {
	if got, _ := tenant.TenantID(ctx); got != tenantID {
		panic("enrichment must run under the event's tenant")
	}
	return []domain.Principal{{Kind: domain.PrincipalKindUser, ID: "u9"}}, m.err
}

func rec(id, subject, payload string) outbox.Record {
	return outbox.Record{ID: id, Subject: subject, Event: commoneventbus.Event{ID: id, TenantID: "t1", Payload: json.RawMessage(payload)}}
}

func newStore(inner *memOutbox, apprErr error) *ApprovalNotificationStore {
	n := &usecase.PublishApprovalNotifications{Expander: &usecase.ExpandApprovalRecipients{ApproverRepo: memApprovers{apprErr}}}
	return &ApprovalNotificationStore{Store: inner, Notifier: n}
}

func TestApprovalNotificationStore_EnrichesOnlyApprovalSubjects(t *testing.T) {
	inner := &memOutbox{recs: []outbox.Record{
		rec("1", "orca.request.request.status_changed", `{"request_id":"r"}`),
		rec("2", "orca.request.approval.requested", `{"approval_id":"a","request_id":"r","subject_type":"plan","self_approval_allowed":true}`),
	}}
	got, err := newStore(inner, nil).FetchUnpublished(context.Background(), 10)
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	if string(got[0].Event.Payload) != `{"request_id":"r"}` {
		t.Fatalf("unrelated events must pass through untouched: %s", got[0].Event.Payload)
	}
	var m map[string]any
	_ = json.Unmarshal(got[1].Event.Payload, &m)
	if m["title"] != "Approval needed" || m["user_ids"].([]any)[0] != "u9" || got[1].ID != "2" || got[1].Event.ID != "2" {
		t.Fatalf("enriched = %v (event id must stay stable for dedupe)", m)
	}
}

func TestApprovalNotificationStore_LookupFailureHoldsBackThatEventAndLaterOnes(t *testing.T) {
	inner := &memOutbox{recs: []outbox.Record{
		rec("1", "orca.request.request.status_changed", `{}`),
		rec("2", "orca.request.approval.requested", `{"approval_id":"a","request_id":"r","subject_type":"plan","self_approval_allowed":true}`),
		rec("3", "orca.request.request.status_changed", `{}`),
	}}
	got, err := newStore(inner, domain.ErrApprovalDirectoryUnavailable).FetchUnpublished(context.Background(), 10)
	if err != nil || len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("batch must stop before the failing event to keep order: %v %v", got, err)
	}
}

func TestApprovalNotificationStore_MalformedPayloadIsPublishedAsIs(t *testing.T) {
	inner := &memOutbox{recs: []outbox.Record{rec("1", "orca.request.approval.decided", `not json`)}}
	got, err := newStore(inner, nil).FetchUnpublished(context.Background(), 10)
	if err != nil || len(got) != 1 || string(got[0].Event.Payload) != `not json` {
		t.Fatalf("%v %v", got, err)
	}
}
