package eventbus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

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

// perApprovalApprovers fails the approver lookup of chosen approvals only.
type perApprovalApprovers struct{ errs map[string]error }

func (perApprovalApprovers) InsertSnapshot(context.Context, string, string, []domain.Principal) error {
	return nil
}
func (p perApprovalApprovers) ListForApproval(_ context.Context, _, approvalID string) ([]domain.Principal, error) {
	return []domain.Principal{{Kind: domain.PrincipalKindUser, ID: "u9"}}, p.errs[approvalID]
}

func approvalRec(id, subject, approvalID string) outbox.Record {
	return rec(id, subject, `{"approval_id":"`+approvalID+`","request_id":"r","subject_type":"plan","self_approval_allowed":true}`)
}

func storeWith(inner *memOutbox, errs map[string]error) *ApprovalNotificationStore {
	n := &usecase.PublishApprovalNotifications{Expander: &usecase.ExpandApprovalRecipients{ApproverRepo: perApprovalApprovers{errs}}}
	return &ApprovalNotificationStore{Store: inner, Notifier: n}
}

func ids(rs []outbox.Record) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func TestApprovalNotificationStore_NotRetryableRowDoesNotFreezeOtherEvents(t *testing.T) {
	permanent := usecase.MarkNotRetryable(errors.New("rpc error: code = PermissionDenied"))
	inner := &memOutbox{recs: []outbox.Record{
		approvalRec("1", "orca.request.approval.requested", "a-bad"),
		rec("2", "orca.request.request.status_changed", `{}`),
		approvalRec("3", "orca.request.approval.requested", "a-good"),
		approvalRec("4", "orca.request.approval.decided", "a-bad"),
		approvalRec("5", "orca.request.approval.decided", "a-good"),
	}}
	got, err := storeWith(inner, map[string]error{"a-bad": permanent}).FetchUnpublished(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	// Row 1 stays in the outbox; row 4 (same approval) waits behind it; everything else is delivered.
	if want := []string{"2", "3", "5"}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("published %v, want %v", ids(got), want)
	}
	var m map[string]any
	_ = json.Unmarshal(got[1].Event.Payload, &m)
	if m["user_ids"] == nil {
		t.Fatalf("rows that did enrich must carry recipients: %v", m)
	}
}

func TestApprovalNotificationStore_NotRetryableRowIsRetriedOnceTheCauseIsFixed(t *testing.T) {
	permanent := usecase.MarkNotRetryable(errors.New("denied"))
	errs := map[string]error{"a": permanent}
	inner := &memOutbox{recs: []outbox.Record{approvalRec("1", "orca.request.approval.requested", "a")}}
	st := storeWith(inner, errs)
	if got, _ := st.FetchUnpublished(context.Background(), 10); len(got) != 0 {
		t.Fatalf("published %v while the directory still refuses", ids(got))
	}
	delete(errs, "a") // configuration fixed
	if got, _ := st.FetchUnpublished(context.Background(), 10); len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("the stranded event must be delivered after the fix, got %v", ids(got))
	}
}

func TestApprovalNotificationStore_TransientFailureStillKeepsGlobalOrder(t *testing.T) {
	inner := &memOutbox{recs: []outbox.Record{
		rec("1", "orca.request.request.status_changed", `{}`),
		approvalRec("2", "orca.request.approval.requested", "a-down"),
		approvalRec("3", "orca.request.approval.requested", "a-fine"),
		rec("4", "orca.request.request.status_changed", `{}`),
	}}
	// A plain directory-unavailable error (e.g. auth-service restarting) is temporary.
	transient := domain.ErrApprovalDirectoryUnavailable
	got, err := storeWith(inner, map[string]error{"a-down": transient}).FetchUnpublished(context.Background(), 10)
	if err != nil || !reflect.DeepEqual(ids(got), []string{"1"}) {
		t.Fatalf("published %v %v, want only the row before the failing one", ids(got), err)
	}
}

func TestApprovalNotificationStore_NotRetryableFailureIsLoggedOncePerMinutePerRow(t *testing.T) {
	var buf bytes.Buffer
	now := time.Unix(1000, 0)
	inner := &memOutbox{recs: []outbox.Record{approvalRec("1", "orca.request.approval.requested", "a")}}
	st := storeWith(inner, map[string]error{"a": usecase.MarkNotRetryable(errors.New("denied"))})
	st.Log = slog.New(slog.NewTextHandler(&buf, nil))
	st.now = func() time.Time { return now }
	for i := 0; i < 5; i++ { // five polls two seconds apart
		_, _ = st.FetchUnpublished(context.Background(), 10)
		now = now.Add(2 * time.Second)
	}
	if n := strings.Count(buf.String(), "cannot be enriched"); n != 1 {
		t.Fatalf("logged %d times in 10s, want 1\n%s", n, buf.String())
	}
	now = now.Add(time.Minute)
	_, _ = st.FetchUnpublished(context.Background(), 10)
	if n := strings.Count(buf.String(), "cannot be enriched"); n != 2 {
		t.Fatalf("logged %d times after a minute, want 2", n)
	}
}
