package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func webSub(id, user string) domain.PushSubscription {
	p, a := "p256dh", "auth"
	return domain.PushSubscription{ID: id, TenantID: "t1", UserID: user, Channel: domain.ChannelWeb, Endpoint: "https://push.example/" + id, P256dhKey: &p, AuthKey: &a}
}

func TestDeliverPush_WebPayloadIsExactlyTitleBodyDeepLinkTag(t *testing.T) {
	subs := &fakeDeliverPushSubscriptionRepository{subs: []domain.PushSubscription{webSub("s1", "u1")}}
	wp := &fakeWebPushClient{}
	uc := NewDeliverPush(subs, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, &fakeVaultSigner{}, wp, &fakeBufferedNotificationRepository{}, &fakeNotificationPreferenceRepository{}, nil, nil, nil)

	ev := testEvent("t1", "u1")
	ev.Body = "done"
	ev.DeepLink = "/?section=tasks&task=1" // json escapes & as \u0026, still valid JSON
	_ = uc.Execute(context.Background(), ev)

	if len(wp.calls) != 1 {
		t.Fatalf("calls = %d", len(wp.calls))
	}
	call := wp.calls[0]
	const golden = `{"title":"Task completed","body":"done","deepLink":"/?section=tasks\u0026task=1","tag":"task_completed:ne-1"}`
	if string(call.ciphertext) != golden {
		t.Errorf("payload = %s\nwant      %s", call.ciphertext, golden)
	}
	if call.nonce != nil {
		t.Errorf("standard web push must be unframed/unsealed, nonce=%v", call.nonce)
	}
	if call.vapidAuth != "vapid t=signed-jwt, k=pub" {
		t.Errorf("vapidAuth = %q", call.vapidAuth)
	}
	if call.opts.TTLSeconds != 86400 || call.opts.Urgency != "normal" {
		t.Errorf("opts = %+v", call.opts)
	}
}

func TestDeliverPush_MCPApproval_BodyNeverCarriesArguments(t *testing.T) {
	subs := &fakeDeliverPushSubscriptionRepository{subs: []domain.PushSubscription{webSub("s1", "u1")}}
	wp := &fakeWebPushClient{}
	uc := NewDeliverPush(subs, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, &fakeVaultSigner{}, wp, &fakeBufferedNotificationRepository{}, &fakeNotificationPreferenceRepository{}, nil, nil, nil)

	ev := testEvent("t1", "u1")
	ev.Type = domain.MCPApprovalType
	ev.Severity = domain.SeverityWarning
	ev.Title = "Approval needed"
	ev.Body = `claude / run_command {"cmd":"rm -rf /","token":"SECRET-ARG"}`
	ev.DeepLink = "/?section=mcp&tab=approvals&approval=a1"
	_ = uc.Execute(context.Background(), ev)

	if len(wp.calls) != 1 {
		t.Fatalf("calls = %d", len(wp.calls))
	}
	raw := string(wp.calls[0].ciphertext)
	if strings.Contains(raw, "SECRET-ARG") || strings.Contains(raw, "rm -rf") {
		t.Fatalf("push payload leaked tool arguments: %s", raw)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil || len(m) != 4 {
		t.Fatalf("payload must have exactly 4 keys: %v %s", err, raw)
	}
	if m["deepLink"] != ev.DeepLink {
		t.Errorf("deepLink = %q", m["deepLink"])
	}
	if o := wp.calls[0].opts; o.Urgency != "high" || o.TTLSeconds != 600 {
		t.Errorf("approval opts = %+v", o)
	}
}

func TestDeliverPush_PartialFailures_AreIndependent(t *testing.T) {
	var all []domain.PushSubscription
	for i := 0; i < 6; i++ {
		all = append(all, webSub(fmt.Sprintf("s%d", i), "u1"))
	}
	subs := &fakeDeliverPushSubscriptionRepository{subs: all}
	wp := &fakeWebPushClient{errFor: map[string]error{
		"https://push.example/s1": fmt.Errorf("gone: %w", ErrDeviceTokenInvalid),
		"https://push.example/s3": errors.New("503"),
	}}
	signer := &fakeVaultSigner{failFor: map[string]bool{"https://push.example/s4": true}}
	buffer := &fakeBufferedNotificationRepository{}
	uc := NewDeliverPush(subs, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, signer, wp, buffer, &fakeNotificationPreferenceRepository{}, nil, nil, nil)

	if err := uc.Execute(context.Background(), testEvent("t1", "u1")); err != nil {
		t.Fatalf("Execute must not error: %v", err)
	}
	if signer.calls != 6 {
		t.Errorf("signer calls = %d, want 6 (one per subscription)", signer.calls)
	}
	if len(wp.calls) != 5 { // s4 failed signing, never reached send
		t.Errorf("send calls = %d, want 5", len(wp.calls))
	}
	if len(subs.expiredEndpoints) != 1 || subs.expiredEndpoints[0] != "https://push.example/s1" {
		t.Errorf("expired = %v", subs.expiredEndpoints)
	}
	if len(buffer.enqueued) != 3 { // s1 (dead), s3 (503), s4 (signer)
		t.Errorf("buffered = %v", buffer.enqueued)
	}
}

type recordingObserver struct{ outcomes atomic.Int64 }

func (r *recordingObserver) ObserveDelivery(channel, outcome string, _ time.Duration) {
	r.outcomes.Add(1)
}

type blockingWebPush struct {
	inflight, peak atomic.Int64
}

func (b *blockingWebPush) Send(ctx context.Context, _, _, _ string, _, _ []byte, _ string, _ WebPushOptions) error {
	n := b.inflight.Add(1)
	for {
		p := b.peak.Load()
		if n <= p || b.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(10 * time.Millisecond)
	b.inflight.Add(-1)
	return nil
}

func TestDeliverPush_ConcurrencyIsBounded(t *testing.T) {
	var all []domain.PushSubscription
	for i := 0; i < 20; i++ {
		all = append(all, webSub(fmt.Sprintf("s%d", i), "u1"))
	}
	bw := &blockingWebPush{}
	obs := &recordingObserver{}
	uc := NewDeliverPush(&fakeDeliverPushSubscriptionRepository{subs: all}, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, &fakeVaultSigner{}, bw, &fakeBufferedNotificationRepository{}, &fakeNotificationPreferenceRepository{}, nil, nil, nil).
		Configure(DeliverPushConfig{Concurrency: 3, Observer: obs})
	_ = uc.Execute(context.Background(), testEvent("t1", "u1"))
	if bw.peak.Load() > 3 || bw.peak.Load() < 2 {
		t.Errorf("peak concurrency = %d, want 2..3", bw.peak.Load())
	}
	if obs.outcomes.Load() != 20 {
		t.Errorf("observed = %d", obs.outcomes.Load())
	}
}

type hangingWebPush struct{}

func (hangingWebPush) Send(ctx context.Context, _, _, _ string, _, _ []byte, _ string, _ WebPushOptions) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestDeliverPush_EachDeliveryHasDeadline(t *testing.T) {
	uc := NewDeliverPush(&fakeDeliverPushSubscriptionRepository{subs: []domain.PushSubscription{webSub("s1", "u1")}}, &fakeDeviceSecretResolver{}, &fakeE2ESealer{}, &fakeVaultSigner{}, hangingWebPush{}, &fakeBufferedNotificationRepository{}, &fakeNotificationPreferenceRepository{}, nil, nil, nil).
		Configure(DeliverPushConfig{Timeout: 50 * time.Millisecond})
	done := make(chan struct{})
	go func() { _ = uc.Execute(context.Background(), testEvent("t1", "u1")); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Execute hung: outbound call has no deadline")
	}
}

func TestEndpointHost_RedactsPathAndQuery(t *testing.T) {
	if got := endpointHost("https://fcm.googleapis.com/fcm/send/SECRET?x=1"); got != "fcm.googleapis.com" {
		t.Errorf("got %q", got)
	}
	if got := endpointHost("::bad"); got != "unknown" {
		t.Errorf("got %q", got)
	}
}
